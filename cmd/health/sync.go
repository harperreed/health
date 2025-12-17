// ABOUTME: CLI commands for E2E encrypted sync with vault.
// ABOUTME: Supports register, login, status, now (manual sync), and logout.
package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/harperreed/health/internal/sync"
	"github.com/harperreed/sweet/vault"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	syncServer string
)

var syncCmd = &cobra.Command{
	Use:     "sync",
	Aliases: []string{"s"},
	Short:   "Sync health data across devices",
	Long: `E2E encrypted sync for health metrics across devices.

Your data is encrypted locally before upload using XChaCha20-Poly1305.
The server never sees your unencrypted health data.

GETTING STARTED:

  1. Register a new account:
     health sync register

  2. Save your recovery phrase! You'll need it to sync on other devices.

  3. On other devices, login with your recovery phrase:
     health sync login

COMMANDS:

  register    Create a new account (generates recovery phrase)
  login       Login with existing account and recovery phrase
  status      Show sync status and pending changes
  now         Manually sync (push local changes, pull remote)
  logout      Clear sync credentials

AUTO-SYNC:

  When auto-sync is enabled (default after login), changes sync immediately
  after each add/delete operation. Disable with 'health sync auto off'.`,
}

var syncRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Create a new sync account",
	Long: `Register a new account on the sync server.

This generates a 24-word recovery phrase that you MUST save securely.
You'll need this phrase to sync on other devices.

Example:
  health sync register`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Prompt for email
		fmt.Print("Email: ")
		var email string
		fmt.Scanln(&email)
		email = strings.TrimSpace(email)
		if email == "" {
			return fmt.Errorf("email is required")
		}

		// Prompt for password
		fmt.Print("Password: ")
		passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("read password: %w", err)
		}
		password := strings.TrimSpace(string(passwordBytes))
		if password == "" {
			return fmt.Errorf("password is required")
		}

		// Generate device ID before registration (required by v0.3.0)
		deviceID := sync.GenerateDeviceID()

		// Register with device ID
		ctx := context.Background()
		authClient := vault.NewPBAuthClient(syncServer)
		result, err := authClient.Register(ctx, email, password, deviceID)
		if err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}

		// Convert mnemonic to hex seed for storage
		seed, err := vault.ParseSeedPhrase(result.Mnemonic)
		if err != nil {
			return fmt.Errorf("parse mnemonic: %w", err)
		}
		derivedKeyHex := hex.EncodeToString(seed.Raw)

		// Save config
		cfg := &sync.Config{
			Server:       syncServer,
			UserID:       result.UserID,
			Token:        result.Token.Token,
			TokenExpires: result.Token.Expires.Format(time.RFC3339),
			DerivedKey:   derivedKeyHex,
			DeviceID:     deviceID,
			VaultDB:      sync.VaultDBPath(),
			AutoSync:     true,
		}

		if err := sync.SaveConfig(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		// Ensure vault DB directory exists
		if err := os.MkdirAll(filepath.Dir(cfg.VaultDB), 0750); err != nil {
			return fmt.Errorf("create vault db directory: %w", err)
		}

		color.Green("✓ Registration successful!")
		fmt.Println()
		color.Yellow("⚠️  SAVE YOUR RECOVERY PHRASE - you'll need it to sync on other devices:")
		fmt.Println()
		color.Cyan("  " + result.Mnemonic)
		fmt.Println()
		fmt.Println("Config saved to:", sync.ConfigPath())
		fmt.Println("Auto-sync: enabled")

		return nil
	},
}

var syncLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to sync server",
	Long: `Login to sync service with your credentials and recovery phrase.

Your recovery phrase is used to derive encryption keys - the server
never sees your data in plaintext.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)

		// Get email
		fmt.Print("Email: ")
		email, _ := reader.ReadString('\n')
		email = strings.TrimSpace(email)
		if email == "" {
			return fmt.Errorf("email required")
		}

		// Get password
		fmt.Print("Password: ")
		passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("read password: %w", err)
		}
		password := string(passwordBytes)
		if password == "" {
			return fmt.Errorf("password cannot be empty")
		}

		// Get mnemonic
		fmt.Print("Recovery phrase (12 or 24 words): ")
		mnemonic, _ := reader.ReadString('\n')
		mnemonic = strings.TrimSpace(mnemonic)

		// Validate mnemonic
		parsed, err := vault.ParseMnemonic(mnemonic)
		if err != nil {
			return fmt.Errorf("invalid recovery phrase: must be 12 or 24 words")
		}
		// Verify it's actually 12 or 24 words
		wordCount := len(strings.Fields(mnemonic))
		if wordCount != 12 && wordCount != 24 {
			return fmt.Errorf("invalid recovery phrase: must be 12 or 24 words")
		}
		_ = parsed

		// Generate device ID before login (required by v0.3.0)
		deviceID := sync.GenerateDeviceID()

		// Login to server
		fmt.Printf("\nLogging in to %s...\n", syncServer)
		ctx := context.Background()
		authClient := vault.NewPBAuthClient(syncServer)
		result, err := authClient.Login(ctx, email, password, deviceID)
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		// Derive key from mnemonic
		seed, err := vault.ParseSeedPhrase(mnemonic)
		if err != nil {
			return fmt.Errorf("parse mnemonic: %w", err)
		}
		derivedKeyHex := hex.EncodeToString(seed.Raw)

		// Save config
		cfg := &sync.Config{
			Server:       syncServer,
			UserID:       result.UserID,
			Token:        result.Token.Token,
			RefreshToken: result.RefreshToken,
			TokenExpires: result.Token.Expires.Format(time.RFC3339),
			DerivedKey:   derivedKeyHex,
			DeviceID:     deviceID,
			VaultDB:      sync.VaultDBPath(),
			AutoSync:     true,
		}

		if err := sync.SaveConfig(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		// Ensure vault DB directory exists
		if err := os.MkdirAll(filepath.Dir(cfg.VaultDB), 0750); err != nil {
			return fmt.Errorf("create vault db directory: %w", err)
		}

		color.Green("\n✓ Logged in successfully")
		fmt.Printf("  User ID: %s\n", cfg.UserID)
		fmt.Printf("  Device: %s\n", cfg.DeviceID[:8]+"...")
		fmt.Printf("  Token expires: %s\n", result.Token.Expires.Format(time.RFC3339))
		fmt.Printf("\nRun 'health sync now' to sync your data.\n")

		return nil
	},
}

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status",
	Long: `Show current sync status including:
- Server connectivity
- Pending changes
- Last sync sequence
- Auto-sync setting`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := sync.LoadConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if !cfg.IsConfigured() {
			color.Yellow("Sync not configured")
			fmt.Println()
			fmt.Println("Run 'health sync register' or 'health sync login' to set up sync.")
			return nil
		}

		fmt.Println("Server:", cfg.Server)
		fmt.Println("User ID:", cfg.UserID)
		fmt.Println("Device ID:", cfg.DeviceID)
		fmt.Println("Auto-sync:", cfg.AutoSync)
		fmt.Println()

		// Create syncer to check status
		syncer, err := sync.NewSyncer(cfg, dbConn)
		if err != nil {
			color.Red("Error: %v", err)
			return nil
		}
		defer syncer.Close()

		ctx := context.Background()

		// Check server health
		health := syncer.Health(ctx)
		if health.OK {
			color.Green("Server: online (latency: %v)", health.Latency)
		} else {
			color.Red("Server: offline or unreachable")
		}

		if health.TokenValid {
			fmt.Println("Token: valid (expires", health.TokenExpires.Format("2006-01-02 15:04"), ")")
		} else {
			color.Yellow("Token: expired - run 'health sync login' to refresh")
		}

		// Check local status
		status, err := syncer.Status(ctx)
		if err != nil {
			color.Red("Error getting status: %v", err)
			return nil
		}

		fmt.Println()
		if status.PendingChanges > 0 {
			color.Yellow("Pending changes: %d", status.PendingChanges)
		} else {
			color.Green("Pending changes: 0 (all synced)")
		}
		fmt.Println("Last pulled sequence:", status.LastPulledSeq)

		return nil
	},
}

var syncNowCmd = &cobra.Command{
	Use:   "now",
	Short: "Sync now",
	Long: `Manually trigger a sync operation.

This pushes any pending local changes to the server and pulls
any new changes from other devices.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := sync.LoadConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if !cfg.IsConfigured() {
			return fmt.Errorf("sync not configured - run 'health sync login' first")
		}

		syncer, err := sync.NewSyncer(cfg, dbConn)
		if err != nil {
			return fmt.Errorf("create syncer: %w", err)
		}
		defer syncer.Close()

		ctx := context.Background()

		fmt.Println("Syncing...")

		events := &vault.SyncEvents{
			OnStart: func() {
				// Already printed "Syncing..."
			},
			OnPush: func(pushed, remaining int) {
				fmt.Printf("  Pushed %d changes (%d remaining)\n", pushed, remaining)
			},
			OnPull: func(pulled int) {
				if pulled > 0 {
					fmt.Printf("  Pulled %d changes\n", pulled)
				}
			},
			OnComplete: func(pushed, pulled int) {
				color.Green("✓ Sync complete (pushed: %d, pulled: %d)", pushed, pulled)
			},
		}

		if err := syncer.SyncWithEvents(ctx, events); err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}

		return nil
	},
}

var syncLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear sync credentials",
	Long: `Remove sync configuration and credentials.

This does not delete your local health data or the vault database.
You can login again with your recovery phrase.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := sync.ClearConfig(); err != nil {
			return fmt.Errorf("clear config: %w", err)
		}

		color.Green("✓ Logged out")
		fmt.Println("Sync configuration removed.")
		fmt.Println("Your local health data is preserved.")

		return nil
	},
}

var syncAutoCmd = &cobra.Command{
	Use:   "auto [on|off]",
	Short: "Enable or disable auto-sync",
	Long: `Enable or disable automatic sync after each mutation.

With auto-sync on, changes sync immediately after each add/delete.
With auto-sync off, you need to manually run 'health sync now'.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := sync.LoadConfig()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		switch strings.ToLower(args[0]) {
		case "on", "true", "1":
			cfg.AutoSync = true
		case "off", "false", "0":
			cfg.AutoSync = false
		default:
			return fmt.Errorf("invalid value: use 'on' or 'off'")
		}

		if err := sync.SaveConfig(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		if cfg.AutoSync {
			color.Green("✓ Auto-sync enabled")
		} else {
			color.Yellow("✓ Auto-sync disabled")
			fmt.Println("Run 'health sync now' to sync manually.")
		}

		return nil
	},
}

const defaultSyncServer = "https://api.storeusa.org"

func init() {
	syncCmd.PersistentFlags().StringVar(&syncServer, "server", defaultSyncServer, "sync server URL")

	syncCmd.AddCommand(syncRegisterCmd)
	syncCmd.AddCommand(syncLoginCmd)
	syncCmd.AddCommand(syncStatusCmd)
	syncCmd.AddCommand(syncNowCmd)
	syncCmd.AddCommand(syncLogoutCmd)
	syncCmd.AddCommand(syncAutoCmd)

	rootCmd.AddCommand(syncCmd)
}
