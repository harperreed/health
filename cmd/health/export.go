// ABOUTME: CLI commands for exporting and importing health data.
// ABOUTME: Supports JSON, YAML, and Markdown export formats.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/harperreed/health/internal/models"
	"github.com/spf13/cobra"
)

var (
	exportOutput string
	exportType   string
	exportSince  string
)

var exportCmd = &cobra.Command{
	Use:   "export <format>",
	Short: "Export health data",
	Long: `Export health data in various formats.

FORMATS:

  json       Full JSON export (suitable for backup/restore)
  yaml       YAML export (human-readable)
  markdown   Markdown tables (for documentation/sharing)

OPTIONS:

  --output, -o   Write to file instead of stdout
  --type, -t     Filter by metric type (markdown only)
  --since        Only include data since this date (YYYY-MM-DD)

EXAMPLES:

  health export json                        # Export all data as JSON
  health export json -o backup.json         # Save to file
  health export yaml                        # Export as YAML
  health export markdown --type weight      # Export weight as Markdown
  health export markdown --since 2024-01-01 # Export data from 2024 onward`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"json", "yaml", "markdown"},
	RunE: func(cmd *cobra.Command, args []string) error {
		format := args[0]

		var data []byte
		var err error

		switch format {
		case "json":
			data, err = db.ExportJSON()
		case "yaml":
			data, err = db.ExportYAML()
		case "markdown":
			var metricType *models.MetricType
			if exportType != "" {
				mt := models.MetricType(exportType)
				metricType = &mt
			}
			var since *time.Time
			if exportSince != "" {
				t, err := time.Parse("2006-01-02", exportSince)
				if err != nil {
					return fmt.Errorf("invalid date format: %s (use YYYY-MM-DD)", exportSince)
				}
				since = &t
			}
			md, err := db.ExportMarkdown(metricType, since)
			if err != nil {
				return err
			}
			data = []byte(md)
		default:
			return fmt.Errorf("unknown format: %s (use json, yaml, or markdown)", format)
		}

		if err != nil {
			return fmt.Errorf("export failed: %w", err)
		}

		if exportOutput != "" {
			if err := os.WriteFile(exportOutput, data, 0600); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			color.Green("✓ Exported to %s", exportOutput)
		} else {
			fmt.Println(string(data))
		}

		return nil
	},
}

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import health data from JSON",
	Long: `Import health data from a JSON backup file.

This imports metrics and workouts from a previously exported JSON file.
Duplicate entries (same ID) will cause an error.

EXAMPLES:

  health import backup.json               # Import from file`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := args[0]

		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		if err := db.ImportJSON(data); err != nil {
			return fmt.Errorf("import failed: %w", err)
		}

		color.Green("✓ Imported from %s", filename)
		return nil
	},
}

func init() {
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "output file (default: stdout)")
	exportCmd.Flags().StringVarP(&exportType, "type", "t", "", "filter by metric type (markdown only)")
	exportCmd.Flags().StringVar(&exportSince, "since", "", "only include data since date (YYYY-MM-DD)")

	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(importCmd)
}
