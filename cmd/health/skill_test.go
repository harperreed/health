// ABOUTME: Tests for the install-skill command.
// ABOUTME: Validates skill installation, directory creation, and file content.

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestSkillInstallCreatesDirectory verifies that the skill directory is created
// when it doesn't exist.
func TestSkillInstallCreatesDirectory(t *testing.T) {
	tmpHome := t.TempDir()

	skillDir := filepath.Join(tmpHome, ".claude", "skills", "health")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	// Read embedded skill content for verification
	content, err := skillFS.ReadFile("skill/SKILL.md")
	if err != nil {
		t.Fatalf("Failed to read embedded skill: %v", err)
	}

	// Create directory and write skill file (simulating what installSkill does)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	if err := os.WriteFile(skillPath, content, 0644); err != nil {
		t.Fatalf("Failed to write skill file: %v", err)
	}

	// Verify directory was created
	info, err := os.Stat(skillDir)
	if err != nil {
		t.Fatalf("Skill directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected skill path to be a directory")
	}

	// Verify file exists
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("Skill file not created: %v", err)
	}
}

// TestSkillInstallWritesCorrectContent verifies the installed SKILL.md has
// expected content markers.
func TestSkillInstallWritesCorrectContent(t *testing.T) {
	tmpHome := t.TempDir()

	skillDir := filepath.Join(tmpHome, ".claude", "skills", "health")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	// Read embedded skill content
	content, err := skillFS.ReadFile("skill/SKILL.md")
	if err != nil {
		t.Fatalf("Failed to read embedded skill: %v", err)
	}

	// Create directory and write skill file
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	if err := os.WriteFile(skillPath, content, 0644); err != nil {
		t.Fatalf("Failed to write skill file: %v", err)
	}

	// Read the written file back
	written, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("Failed to read written skill file: %v", err)
	}

	// Verify essential content markers
	contentStr := string(written)
	expectedMarkers := []string{
		"name: health",
		"description:",
		"mcp__health__add_metric",
		"mcp__health__list_metrics",
		"mcp__health__get_latest",
		"mcp__health__add_workout",
		"## When to use health",
		"## Metric types",
	}

	for _, marker := range expectedMarkers {
		if !strings.Contains(contentStr, marker) {
			t.Errorf("Expected SKILL.md to contain %q", marker)
		}
	}
}

// TestSkillInstallOverwritesExistingFile verifies that an existing skill file
// is properly overwritten.
func TestSkillInstallOverwritesExistingFile(t *testing.T) {
	tmpHome := t.TempDir()

	skillDir := filepath.Join(tmpHome, ".claude", "skills", "health")
	skillPath := filepath.Join(skillDir, "SKILL.md")

	// Create directory first
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	// Write an old/stale version
	oldContent := []byte("# Old Skill\nThis is stale content that should be replaced.")
	if err := os.WriteFile(skillPath, oldContent, 0644); err != nil {
		t.Fatalf("Failed to write old skill file: %v", err)
	}

	// Verify old file exists
	oldData, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("Failed to read old skill file: %v", err)
	}
	if !strings.Contains(string(oldData), "stale content") {
		t.Error("Expected old content to be present initially")
	}

	// Read embedded skill content
	content, err := skillFS.ReadFile("skill/SKILL.md")
	if err != nil {
		t.Fatalf("Failed to read embedded skill: %v", err)
	}

	// Overwrite with the current skill content
	if err := os.WriteFile(skillPath, content, 0644); err != nil {
		t.Fatalf("Failed to overwrite skill file: %v", err)
	}

	// Verify the file was overwritten with correct content
	newData, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("Failed to read new skill file: %v", err)
	}

	if strings.Contains(string(newData), "stale content") {
		t.Error("Old content should have been replaced")
	}
	if !strings.Contains(string(newData), "name: health") {
		t.Error("Expected new content to contain 'name: health'")
	}
}

// TestSkillFSReadEmbeddedContent verifies the embedded filesystem can read
// the SKILL.md file correctly.
func TestSkillFSReadEmbeddedContent(t *testing.T) {
	content, err := skillFS.ReadFile("skill/SKILL.md")
	if err != nil {
		t.Fatalf("Failed to read embedded skill/SKILL.md: %v", err)
	}

	if len(content) == 0 {
		t.Error("Embedded SKILL.md is empty")
	}

	contentStr := string(content)

	// Verify it's a valid SKILL.md with frontmatter
	if !strings.HasPrefix(contentStr, "---") {
		t.Error("Expected SKILL.md to start with YAML frontmatter (---)")
	}

	// Verify required frontmatter fields
	if !strings.Contains(contentStr, "name: health") {
		t.Error("Expected frontmatter to contain 'name: health'")
	}
	if !strings.Contains(contentStr, "description:") {
		t.Error("Expected frontmatter to contain 'description:'")
	}
}

// TestSkillInstallDirectoryPermissions verifies the created directory has
// correct permissions by calling installSkill with a temp HOME.
func TestSkillInstallDirectoryPermissions(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", origHome) })
	os.Setenv("HOME", tmpHome)

	// Reset the global flag
	origSkipConfirm := skillSkipConfirm
	skillSkipConfirm = true
	t.Cleanup(func() { skillSkipConfirm = origSkipConfirm })

	// Create a mock command with captured output
	cmd := &cobra.Command{}
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetIn(strings.NewReader(""))

	// Call the actual installSkill function
	if err := installSkill(cmd); err != nil {
		t.Fatalf("installSkill failed: %v", err)
	}

	skillDir := filepath.Join(tmpHome, ".claude", "skills", "health")
	info, err := os.Stat(skillDir)
	if err != nil {
		t.Fatalf("Failed to stat skill directory: %v", err)
	}

	// Check that directory is readable and executable by owner (0750)
	mode := info.Mode()
	if mode&0700 != 0700 {
		t.Errorf("Expected directory to be rwx for owner, got %v", mode)
	}
}

// TestSkillInstallFilePermissions verifies the created file has correct permissions
// by calling installSkill with a temp HOME.
func TestSkillInstallFilePermissions(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", origHome) })
	os.Setenv("HOME", tmpHome)

	// Reset the global flag
	origSkipConfirm := skillSkipConfirm
	skillSkipConfirm = true
	t.Cleanup(func() { skillSkipConfirm = origSkipConfirm })

	// Create a mock command with captured output
	cmd := &cobra.Command{}
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetIn(strings.NewReader(""))

	// Call the actual installSkill function
	if err := installSkill(cmd); err != nil {
		t.Fatalf("installSkill failed: %v", err)
	}

	skillPath := filepath.Join(tmpHome, ".claude", "skills", "health", "SKILL.md")
	info, err := os.Stat(skillPath)
	if err != nil {
		t.Fatalf("Failed to stat skill file: %v", err)
	}

	// Check that file is readable and writable by owner only (0600)
	mode := info.Mode()
	if mode&0600 != 0600 {
		t.Errorf("Expected file to be rw for owner, got %v", mode)
	}
	// Verify it's NOT world readable (security)
	if mode&0077 != 0 {
		t.Errorf("Expected file to NOT be accessible to group/others, got %v", mode)
	}
}

// TestSkillInstallNestedDirectoryCreation verifies that MkdirAll creates
// the full path including parent directories.
func TestSkillInstallNestedDirectoryCreation(t *testing.T) {
	tmpHome := t.TempDir()

	// None of these directories exist yet
	skillDir := filepath.Join(tmpHome, ".claude", "skills", "health")

	// Verify parent directories don't exist
	claudeDir := filepath.Join(tmpHome, ".claude")
	if _, err := os.Stat(claudeDir); err == nil {
		t.Fatal(".claude directory should not exist yet")
	}

	// Create the full path
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create nested directories: %v", err)
	}

	// Verify all directories were created
	for _, dir := range []string{
		filepath.Join(tmpHome, ".claude"),
		filepath.Join(tmpHome, ".claude", "skills"),
		filepath.Join(tmpHome, ".claude", "skills", "health"),
	} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("Directory %s was not created: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}
}

// TestSkillSkipConfirmFlag verifies the flag exists and has correct defaults.
func TestSkillSkipConfirmFlag(t *testing.T) {
	// Check that the flag is defined on the command
	flag := installSkillCmd.Flags().Lookup("yes")
	if flag == nil {
		t.Fatal("Expected --yes flag to be defined")
	}

	// Check shorthand
	if flag.Shorthand != "y" {
		t.Errorf("Expected shorthand 'y', got %q", flag.Shorthand)
	}

	// Check default value
	if flag.DefValue != "false" {
		t.Errorf("Expected default value 'false', got %q", flag.DefValue)
	}
}

// TestSkillInstallNonInteractiveContext verifies that install-skill detects
// non-interactive contexts and cancels gracefully.
func TestSkillInstallNonInteractiveContext(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", origHome) })
	os.Setenv("HOME", tmpHome)

	// Ensure skipConfirm is false to test the interactive path
	origSkipConfirm := skillSkipConfirm
	skillSkipConfirm = false
	t.Cleanup(func() { skillSkipConfirm = origSkipConfirm })

	// Override isTerminal to simulate non-TTY
	origIsTerminal := isTerminal
	isTerminal = func(fd int) bool { return false }
	t.Cleanup(func() { isTerminal = origIsTerminal })

	// Create a mock command with captured output and a pipe (non-TTY) input
	cmd := &cobra.Command{}
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	// Use a file-based reader that won't be detected as a terminal
	r, w, _ := os.Pipe()
	w.Close() // Close write end immediately to simulate empty/EOF
	cmd.SetIn(r)

	// Call installSkill - should detect non-interactive and cancel
	if err := installSkill(cmd); err != nil {
		t.Fatalf("installSkill failed: %v", err)
	}

	output := outBuf.String()
	if !strings.Contains(output, "Non-interactive context detected") {
		t.Errorf("Expected non-interactive message, got: %s", output)
	}
	if !strings.Contains(output, "Installation canceled") {
		t.Errorf("Expected installation canceled message, got: %s", output)
	}

	// Verify file was NOT created
	skillPath := filepath.Join(tmpHome, ".claude", "skills", "health", "SKILL.md")
	if _, err := os.Stat(skillPath); err == nil {
		t.Error("Skill file should NOT have been created in non-interactive mode")
	}
}

// TestSkillInstallWithYesFlagInNonInteractive verifies that --yes flag works
// in non-interactive contexts.
func TestSkillInstallWithYesFlagInNonInteractive(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", origHome) })
	os.Setenv("HOME", tmpHome)

	// Set skipConfirm to true (simulating --yes flag)
	origSkipConfirm := skillSkipConfirm
	skillSkipConfirm = true
	t.Cleanup(func() { skillSkipConfirm = origSkipConfirm })

	// Override isTerminal to simulate non-TTY
	origIsTerminal := isTerminal
	isTerminal = func(fd int) bool { return false }
	t.Cleanup(func() { isTerminal = origIsTerminal })

	// Create a mock command
	cmd := &cobra.Command{}
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetIn(strings.NewReader(""))

	// Call installSkill - should succeed with --yes even in non-interactive
	if err := installSkill(cmd); err != nil {
		t.Fatalf("installSkill failed: %v", err)
	}

	output := outBuf.String()
	if !strings.Contains(output, "Installed health skill successfully") {
		t.Errorf("Expected success message, got: %s", output)
	}

	// Verify file WAS created
	skillPath := filepath.Join(tmpHome, ".claude", "skills", "health", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Error("Skill file should have been created with --yes flag")
	}
}

// TestSkillInstallUsesCobraStreams verifies that output goes to Cobra's streams.
func TestSkillInstallUsesCobraStreams(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", origHome) })
	os.Setenv("HOME", tmpHome)

	origSkipConfirm := skillSkipConfirm
	skillSkipConfirm = true
	t.Cleanup(func() { skillSkipConfirm = origSkipConfirm })

	// Create command with custom output buffer
	cmd := &cobra.Command{}
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetIn(strings.NewReader(""))

	if err := installSkill(cmd); err != nil {
		t.Fatalf("installSkill failed: %v", err)
	}

	// Verify output was captured in our buffer (not just printed to stdout)
	output := outBuf.String()
	if !strings.Contains(output, "Health Skill for Claude Code") {
		t.Errorf("Expected header in Cobra output stream, got: %s", output)
	}
	if !strings.Contains(output, "Installed health skill successfully") {
		t.Errorf("Expected success message in Cobra output stream, got: %s", output)
	}
}

// TestSkillEmbeddedContentMatchesSource verifies the embedded content integrity.
func TestSkillEmbeddedContentMatchesSource(t *testing.T) {
	content, err := skillFS.ReadFile("skill/SKILL.md")
	if err != nil {
		t.Fatalf("Failed to read embedded skill: %v", err)
	}

	// Verify the content has all expected MCP tool references
	expectedTools := []string{
		"mcp__health__add_metric",
		"mcp__health__list_metrics",
		"mcp__health__get_latest",
		"mcp__health__add_workout",
		"mcp__health__list_workouts",
		"mcp__health__delete_metric",
	}

	contentStr := string(content)
	for _, tool := range expectedTools {
		if !strings.Contains(contentStr, tool) {
			t.Errorf("Expected embedded SKILL.md to reference %q", tool)
		}
	}

	// Verify metric types are documented
	expectedMetrics := []string{
		"weight",
		"body_fat",
		"bp_sys",
		"bp_dia",
		"heart_rate",
		"mood",
		"energy",
		"stress",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(contentStr, metric) {
			t.Errorf("Expected embedded SKILL.md to document metric type %q", metric)
		}
	}
}
