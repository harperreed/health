// ABOUTME: Install Claude Code skill for health
// ABOUTME: Embeds and installs the skill definition to ~/.claude/skills/

package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

//go:embed skill/SKILL.md
var skillFS embed.FS

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
	rootCmd.AddCommand(installSkillCmd)
}

func installSkill() error {
	// Read embedded skill file
	content, err := skillFS.ReadFile("skill/SKILL.md")
	if err != nil {
		return fmt.Errorf("failed to read embedded skill: %w", err)
	}

	// Determine destination
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	skillDir := filepath.Join(home, ".claude", "skills", "health")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	// Create directory
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("failed to create skill directory: %w", err)
	}

	// Write skill file
	if err := os.WriteFile(skillPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write skill file: %w", err)
	}

	fmt.Printf("Installed health skill to %s\n", skillPath)
	fmt.Println("Claude Code will now recognize /health commands.")
	return nil
}
