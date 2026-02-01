// ABOUTME: Install Claude Code skill for health
// ABOUTME: Embeds and installs the skill definition to ~/.claude/skills/

package main

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed skill/SKILL.md
var skillFS embed.FS

var skillSkipConfirm bool

var installSkillCmd = &cobra.Command{
	Use:   "install-skill",
	Short: "Install Claude Code skill",
	Long: `Install the health skill for Claude Code.

This copies the skill definition to ~/.claude/skills/health/
so Claude Code can use health commands contextually.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return installSkill()
	},
}

func init() {
	installSkillCmd.Flags().BoolVarP(&skillSkipConfirm, "yes", "y", false, "Skip confirmation prompt")
	rootCmd.AddCommand(installSkillCmd)
}

func installSkill() error {
	// Determine destination
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	skillDir := filepath.Join(home, ".claude", "skills", "health")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	// Show explanation
	fmt.Println("┌─────────────────────────────────────────────────────────────┐")
	fmt.Println("│             Health Skill for Claude Code                    │")
	fmt.Println("└─────────────────────────────────────────────────────────────┘")
	fmt.Println()
	fmt.Println("This will install the health skill, enabling Claude Code to:")
	fmt.Println()
	fmt.Println("  • Log weight, exercise, and vitals")
	fmt.Println("  • Track nutrition and mood")
	fmt.Println("  • View health trends over time")
	fmt.Println("  • Use the /health slash command")
	fmt.Println()
	fmt.Println("Destination:")
	fmt.Printf("  %s\n", skillPath)
	fmt.Println()

	// Check if already installed
	if _, err := os.Stat(skillPath); err == nil {
		fmt.Println("Note: A skill file already exists and will be overwritten.")
		fmt.Println()
	}

	// Ask for confirmation unless --yes flag is set
	if !skillSkipConfirm {
		fmt.Print("Install the health skill? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Installation canceled.")
			return nil
		}
		fmt.Println()
	}

	// Read embedded skill file
	content, err := skillFS.ReadFile("skill/SKILL.md")
	if err != nil {
		return fmt.Errorf("failed to read embedded skill: %w", err)
	}

	// Create directory
	if err := os.MkdirAll(skillDir, 0750); err != nil {
		return fmt.Errorf("failed to create skill directory: %w", err)
	}

	// Write skill file
	if err := os.WriteFile(skillPath, content, 0600); err != nil {
		return fmt.Errorf("failed to write skill file: %w", err)
	}

	fmt.Println("✓ Installed health skill successfully!")
	fmt.Println()
	fmt.Println("Claude Code will now recognize /health commands.")
	fmt.Println("Try asking Claude: \"Log my weight as 175 lbs\" or \"Show my health trends\"")
	return nil
}
