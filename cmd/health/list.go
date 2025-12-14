// ABOUTME: CLI command for listing health metrics.
// ABOUTME: Supports filtering by type and limiting results.
package main

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/harperreed/health/internal/db"
	"github.com/harperreed/health/internal/models"
	"github.com/spf13/cobra"
)

var (
	listType  string
	listLimit int
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "l"},
	Short:   "List health metrics",
	Long: `List recent health metrics, optionally filtered by type.

Examples:
  health list
  health list --type weight
  health list --type mood --limit 20`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var metricType *models.MetricType
		if listType != "" {
			if !models.IsValidMetricType(listType) {
				return fmt.Errorf("unknown metric type: %s", listType)
			}
			mt := models.MetricType(listType)
			metricType = &mt
		}

		metrics, err := db.ListMetrics(dbConn, metricType, listLimit)
		if err != nil {
			return fmt.Errorf("failed to list metrics: %w", err)
		}

		if len(metrics) == 0 {
			fmt.Println("No metrics found.")
			return nil
		}

		faint := color.New(color.Faint)
		for _, m := range metrics {
			notes := ""
			if m.Notes != nil && *m.Notes != "" {
				notes = faint.Sprintf(" (%s)", truncate(*m.Notes, 30))
			}
			fmt.Printf("%s %s %s %.2f %s%s\n",
				faint.Sprint(m.ID.String()[:8]),
				faint.Sprint(m.RecordedAt.Format("2006-01-02 15:04")),
				padRight(string(m.MetricType), 16),
				m.Value,
				m.Unit,
				notes)
		}

		return nil
	},
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func padRight(s string, length int) string {
	if len(s) >= length {
		return s
	}
	return s + strings.Repeat(" ", length-len(s))
}

func init() {
	listCmd.Flags().StringVarP(&listType, "type", "t", "", "filter by metric type")
	listCmd.Flags().IntVarP(&listLimit, "limit", "n", 20, "max number of results")
	rootCmd.AddCommand(listCmd)
}
