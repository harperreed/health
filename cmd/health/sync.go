// ABOUTME: CLI commands for Charm-based sync.
// ABOUTME: Supports link, unlink, status, and wipe operations.
package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:     "sync",
	Aliases: []string{"s"},
	Short:   "Sync health data across devices",
	Long: `Sync health data across devices using Charm Cloud.

Your data is E2E encrypted with your SSH key before upload.
The server never sees your unencrypted health data.

GETTING STARTED:

  1. Link your device (creates/uses SSH key automatically):
     health sync link

  2. On other devices, link with the same Charm account:
     health sync link

  3. Check sync status:
     health sync status

COMMANDS:

  link        Link this device to your Charm account
  unlink      Disconnect this device from Charm
  status      Show sync status and account info
  wipe        Reset local data from cloud (destructive)

Data syncs automatically after each add/delete operation.`,
}

var syncLinkCmd = &cobra.Command{
	Use:   "link",
	Short: "Link this device to Charm",
	Long: `Link this device to your Charm account.

If you don't have a Charm account, one will be created using your SSH key.
If you already have an account, you'll be prompted to link via charm.sh.

Example:
  health sync link`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Use charm CLI to link
		charmCmd := exec.Command("charm", "link")
		charmCmd.Stdin = os.Stdin
		charmCmd.Stdout = os.Stdout
		charmCmd.Stderr = os.Stderr

		if err := charmCmd.Run(); err != nil {
			return fmt.Errorf("failed to link: %w\n\nMake sure 'charm' CLI is installed: go install github.com/charmbracelet/charm@latest", err)
		}

		color.Green("\n✓ Device linked to Charm")
		fmt.Println("Your health data will now sync automatically across devices.")

		// Sync immediately after linking
		if charmClient != nil {
			if err := charmClient.Sync(); err != nil {
				color.Yellow("⚠ Initial sync failed: %v", err)
			} else {
				color.Green("✓ Initial sync complete")
			}
		}

		return nil
	},
}

var syncUnlinkCmd = &cobra.Command{
	Use:   "unlink",
	Short: "Disconnect from Charm",
	Long: `Disconnect this device from Charm.

This does not delete your local health data.
You can link again later with 'health sync link'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Use charm CLI to unlink
		charmCmd := exec.Command("charm", "unlink")
		charmCmd.Stdin = os.Stdin
		charmCmd.Stdout = os.Stdout
		charmCmd.Stderr = os.Stderr

		if err := charmCmd.Run(); err != nil {
			return fmt.Errorf("failed to unlink: %w", err)
		}

		color.Green("✓ Device unlinked from Charm")
		fmt.Println("Your local health data is preserved.")

		return nil
	},
}

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status",
	Long: `Show current sync status including:
- Charm account info
- Connection status
- Local data info`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get Charm ID
		if charmClient == nil {
			color.Yellow("Charm client not initialized")
			fmt.Println("\nRun 'health sync link' to connect to Charm.")
			return nil
		}

		id, err := charmClient.ID()
		if err != nil {
			color.Yellow("Not linked to Charm")
			fmt.Println("\nRun 'health sync link' to connect to Charm.")
			return nil
		}

		fmt.Println("Charm ID:", id)
		fmt.Println("Server: charm.2389.dev")
		fmt.Println()

		// Show local data counts
		metrics, _ := charmClient.ListMetrics(nil, 0)
		workouts, _ := charmClient.ListWorkouts(nil, 0)

		color.Green("✓ Connected to Charm")
		fmt.Printf("  Metrics: %d\n", len(metrics))
		fmt.Printf("  Workouts: %d\n", len(workouts))

		return nil
	},
}

var syncWipeCmd = &cobra.Command{
	Use:   "wipe",
	Short: "Reset local data from cloud",
	Long: `Delete all local data and restore from Charm Cloud.

This is a destructive operation. All local data will be lost.
Use this to:
- Fix sync conflicts
- Reset a device to cloud state
- Start fresh on a device`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if charmClient == nil {
			return fmt.Errorf("charm client not initialized")
		}

		// Confirm
		fmt.Println("This will DELETE all local health data and restore from cloud.")
		fmt.Print("Type 'WIPE' to confirm: ")
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "WIPE" {
			fmt.Println("Canceled.")
			return nil
		}

		if err := charmClient.Reset(); err != nil {
			return fmt.Errorf("wipe failed: %w", err)
		}

		color.Green("✓ Local data wiped and restored from cloud")

		return nil
	},
}

func init() {
	syncCmd.AddCommand(syncLinkCmd)
	syncCmd.AddCommand(syncUnlinkCmd)
	syncCmd.AddCommand(syncStatusCmd)
	syncCmd.AddCommand(syncWipeCmd)

	rootCmd.AddCommand(syncCmd)
}
