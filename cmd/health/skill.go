// ABOUTME: Install Claude Code skill for health
// ABOUTME: Embeds and installs the skill definition to ~/.claude/skills/

package main

import (
	"bufio"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
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
		return installSkill(cmd)
	},
}

func init() {
	installSkillCmd.Flags().BoolVarP(&skillSkipConfirm, "yes", "y", false, "Skip confirmation prompt")
	rootCmd.AddCommand(installSkillCmd)
}

// isTerminal checks if the given file descriptor is a terminal.
// This is used to detect non-interactive contexts.
// Defined as a variable to allow testing with mock implementations.
var isTerminal = term.IsTerminal

func installSkill(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()
	in := cmd.InOrStdin()

	// Determine destination
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	skillDir := filepath.Join(home, ".claude", "skills", "health")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	// Show explanation (output errors ignored for display text)
	_, _ = fmt.Fprintln(out, "┌─────────────────────────────────────────────────────────────┐")
	_, _ = fmt.Fprintln(out, "│             Health Skill for Claude Code                    │")
	_, _ = fmt.Fprintln(out, "└─────────────────────────────────────────────────────────────┘")
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "This will install the health skill, enabling Claude Code to:")
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "  • Log weight, exercise, and vitals")
	_, _ = fmt.Fprintln(out, "  • Track nutrition and mood")
	_, _ = fmt.Fprintln(out, "  • View health trends over time")
	_, _ = fmt.Fprintln(out, "  • Use the /health slash command")
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "Destination:")
	_, _ = fmt.Fprintf(out, "  %s\n", skillPath)
	_, _ = fmt.Fprintln(out)

	// Check if already installed
	if _, err := os.Stat(skillPath); err == nil {
		_, _ = fmt.Fprintln(out, "Note: A skill file already exists and will be overwritten.")
		_, _ = fmt.Fprintln(out)
	}

	// Ask for confirmation unless --yes flag is set
	if !skillSkipConfirm {
		// Check if stdin is a terminal - if not, treat as non-interactive
		inFile, isFile := in.(*os.File)
		if !isFile || !isTerminal(int(inFile.Fd())) {
			_, _ = fmt.Fprintln(out, "Non-interactive context detected. Use --yes to confirm installation.")
			_, _ = fmt.Fprintln(out, "Installation canceled.")
			return nil
		}

		_, _ = fmt.Fprint(out, "Install the health skill? [y/N] ")
		reader := bufio.NewReader(in)
		response, err := reader.ReadString('\n')
		if err != nil {
			// Treat EOF as "no" - user didn't provide input
			if err == io.EOF {
				_, _ = fmt.Fprintln(out)
				_, _ = fmt.Fprintln(out, "Installation canceled.")
				return nil
			}
			return fmt.Errorf("failed to read response: %w", err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			_, _ = fmt.Fprintln(out, "Installation canceled.")
			return nil
		}
		_, _ = fmt.Fprintln(out)
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

	_, _ = fmt.Fprintln(out, "✓ Installed health skill successfully!")
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, "Claude Code will now recognize /health commands.")
	_, _ = fmt.Fprintln(out, "Try asking Claude: \"Log my weight as 175 lbs\" or \"Show my health trends\"")
	return nil
}
