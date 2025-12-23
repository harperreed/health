// ABOUTME: CLI commands for Charm-based sync.
// ABOUTME: Supports link, unlink, status, repair, reset, and wipe operations.
package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/charmbracelet/charm/kv"
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
  repair      Repair database corruption (checkpoints WAL, removes SHM, vacuums)
  reset       Reset local data and restore from cloud (destructive)
  wipe        Delete cloud and local data (destructive)

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
	Short: "Delete all cloud and local data",
	Long: `Delete all cloud backups and local data.

This is a DESTRUCTIVE operation. ALL data will be permanently deleted.
Use this to:
- Completely remove all health data
- Start completely fresh`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Confirm
		fmt.Println("This will PERMANENTLY DELETE all cloud backups and local health data.")
		fmt.Print("Type 'wipe' to confirm: ")
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "wipe" {
			fmt.Println("Canceled.")
			return nil
		}

		result, err := kv.Wipe("health")
		if err != nil {
			return fmt.Errorf("wipe failed: %w", err)
		}

		color.Green("✓ Data wiped successfully")
		fmt.Printf("  Cloud backups deleted: %d\n", result.CloudBackupsDeleted)
		fmt.Printf("  Local files deleted: %d\n", result.LocalFilesDeleted)

		return nil
	},
}

var syncRepairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Repair database corruption",
	Long: `Repair database corruption by checkpointing WAL, removing SHM files, checking integrity, and vacuuming.

Use this when you encounter database lock errors or corruption.
Run with --force to attempt recovery even if integrity checks fail.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")

		fmt.Println("Repairing health database...")
		result, err := kv.Repair("health", force)

		// Show what happened
		if result.WalCheckpointed {
			color.Green("  ✓ WAL checkpointed")
		}
		if result.ShmRemoved {
			color.Green("  ✓ SHM file removed")
		}
		if result.IntegrityOK {
			color.Green("  ✓ Integrity check passed")
		} else {
			color.Red("  ✗ Integrity check failed")
		}
		if result.Vacuumed {
			color.Green("  ✓ Database vacuumed")
		}

		if err != nil {
			if !force {
				color.Yellow("\nRun with --force to attempt recovery.")
			}
			return fmt.Errorf("repair failed: %w", err)
		}

		color.Green("\n✓ Repair complete")
		return nil
	},
}

var syncResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset local data and restore from cloud",
	Long: `Delete all local data and restore from Charm Cloud.

This is a destructive operation. All local data will be lost and restored from cloud.
Use this to:
- Fix sync conflicts
- Reset a device to cloud state
- Start fresh on a device`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Confirm
		fmt.Println("This will DELETE all local health data and restore from cloud.")
		fmt.Print("Continue? [y/N]: ")
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Canceled.")
			return nil
		}

		err := kv.Reset("health")
		if err != nil {
			return fmt.Errorf("reset failed: %w", err)
		}

		color.Green("✓ Local data reset and restored from cloud")

		return nil
	},
}

func init() {
	syncCmd.AddCommand(syncLinkCmd)
	syncCmd.AddCommand(syncUnlinkCmd)
	syncCmd.AddCommand(syncStatusCmd)
	syncCmd.AddCommand(syncRepairCmd)
	syncCmd.AddCommand(syncResetCmd)
	syncCmd.AddCommand(syncWipeCmd)

	// Add --force flag to repair command
	syncRepairCmd.Flags().Bool("force", false, "Attempt recovery even if integrity checks fail")

	rootCmd.AddCommand(syncCmd)
}
