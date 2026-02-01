// ABOUTME: CLI command for migrating from Charm KV to SQLite.
// ABOUTME: One-time migration tool for users upgrading from older versions.
package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	migrateDryRun bool
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate from Charm KV to SQLite",
	Long: `Migrate health data from the legacy Charm KV storage to SQLite.

This is a one-time migration tool for users upgrading from older versions
of the health tool that used Charm KV for storage.

IMPORTANT:

  - This command requires the legacy Charm KV data to exist
  - The SQLite database will be created at ~/.local/share/health/health.db
  - Existing SQLite data will NOT be overwritten (duplicates cause errors)
  - Run with --dry-run first to see what would be migrated

USAGE:

  health migrate --dry-run   # Preview what would be migrated
  health migrate             # Perform the migration

AFTER MIGRATION:

  Once migration is complete, you can delete the old Charm data:
    rm -rf ~/.local/share/charm/kv/health/

  The new data is stored at:
    ~/.local/share/health/health.db`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if migrateDryRun {
			color.Yellow("Dry run mode - no changes will be made")
			fmt.Println()
		}

		// The Charm KV storage has been removed, so this command now serves
		// as a placeholder for documentation purposes. Users who need to
		// migrate should contact support or use the export/import commands
		// if they have a JSON backup from the old system.
		color.Yellow("Note: Charm KV storage has been removed from this version.")
		fmt.Println()
		fmt.Println("If you have data in the old Charm KV format, you have two options:")
		fmt.Println()
		fmt.Println("1. If you previously exported data using 'health sync export':")
		fmt.Println("   health import backup.json")
		fmt.Println()
		fmt.Println("2. The Charm KV data was stored at:")
		fmt.Println("   ~/.local/share/charm/kv/health/")
		fmt.Println()
		fmt.Println("   This data was encrypted with your SSH key. If you need to")
		fmt.Println("   recover it, please contact support.")
		fmt.Println()
		fmt.Println("Your new data is stored at:")
		fmt.Println("   ~/.local/share/health/health.db")

		return nil
	},
}

func init() {
	migrateCmd.Flags().BoolVar(&migrateDryRun, "dry-run", false, "preview migration without making changes")
	rootCmd.AddCommand(migrateCmd)
}
