// ABOUTME: CLI command for deleting health metrics.
// ABOUTME: Supports deletion by full ID or ID prefix.
package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"del", "rm"},
	Short:   "Delete a health metric",
	Long: `Delete a health metric by its ID or ID prefix.

You can use either the full UUID or just the first few characters (prefix).
The ID prefix is shown in the first column of 'health list' output.

EXAMPLES:

  health delete abc12345                    # Delete by 8-char prefix
  health delete abc12345-1234-1234-...     # Delete by full UUID
  health rm abc1                            # Short prefix (if unique)

CAUTION:

  This permanently deletes the metric. There is no undo.
  If the prefix matches multiple metrics, an error is returned.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		idOrPrefix := args[0]

		// First, try to get the metric to show what we're deleting
		metric, err := charmClient.GetMetric(idOrPrefix)
		if err != nil {
			return fmt.Errorf("metric not found: %s", idOrPrefix)
		}

		if err := charmClient.DeleteMetric(idOrPrefix); err != nil {
			return fmt.Errorf("failed to delete metric: %w", err)
		}

		color.Yellow("✗ Deleted %s", metric.MetricType)
		fmt.Printf("  %s %.2f %s\n",
			color.New(color.Faint).Sprint(metric.ID.String()[:8]),
			metric.Value, metric.Unit)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
