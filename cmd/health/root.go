// ABOUTME: Root Cobra command for health CLI.
// ABOUTME: Handles database lifecycle via PersistentPre/PostRunE.
package main

import (
	"database/sql"
	"fmt"

	"github.com/harperreed/health/internal/db"
	"github.com/spf13/cobra"
)

var (
	dbPath string
	dbConn *sql.DB
)

var rootCmd = &cobra.Command{
	Use:   "health",
	Short: "Health metrics tracker",
	Long:  `A CLI tool for tracking health metrics including biometrics, activity, nutrition, and mental health.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip DB init for commands that don't need it
		if cmd.Name() == "version" || cmd.Name() == "help" {
			return nil
		}

		var err error
		dbConn, err = db.InitDB(dbPath)
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if dbConn != nil {
			return dbConn.Close()
		}
		return nil
	},
}

func init() {
	defaultPath := db.GetDefaultDBPath()
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", defaultPath, "database file path")
}
