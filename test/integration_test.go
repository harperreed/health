// ABOUTME: Integration tests for health CLI with SQLite backend.
// ABOUTME: Tests full workflow from CLI commands.
package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestFullWorkflow tests the complete health CLI workflow.
func TestFullWorkflow(t *testing.T) {
	// Create temp dir for test database
	tmpDir, err := os.MkdirTemp("", "health-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set XDG_DATA_HOME to temp dir so tests don't affect real data
	os.Setenv("XDG_DATA_HOME", tmpDir)
	defer os.Unsetenv("XDG_DATA_HOME")

	// Build the binary
	projectRoot, _ := filepath.Abs("..")
	healthBinary := filepath.Join(projectRoot, "health")

	buildCmd := exec.Command("go", "build", "-o", healthBinary, "./cmd/health")
	buildCmd.Dir = projectRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\n%s", err, output)
	}
	defer os.Remove(healthBinary)

	run := func(args ...string) (string, error) {
		cmd := exec.Command(healthBinary, args...)
		cmd.Env = append(os.Environ(), "XDG_DATA_HOME="+tmpDir)
		output, err := cmd.CombinedOutput()
		return string(output), err
	}

	// Test adding metrics
	output, err := run("add", "weight", "82.5")
	if err != nil {
		t.Fatalf("Failed to add weight: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Added weight") {
		t.Errorf("Expected 'Added weight' in output, got: %s", output)
	}

	// Test blood pressure
	output, err = run("add", "bp", "120", "80")
	if err != nil {
		t.Fatalf("Failed to add bp: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Added blood pressure") {
		t.Errorf("Expected 'Added blood pressure' in output, got: %s", output)
	}

	// Test listing
	output, err = run("list")
	if err != nil {
		t.Fatalf("Failed to list: %v\n%s", err, output)
	}
	if !strings.Contains(output, "weight") {
		t.Errorf("Expected 'weight' in list output, got: %s", output)
	}

	// Test workout add
	output, err = run("workout", "add", "run", "--duration", "45")
	if err != nil {
		t.Fatalf("Failed to add workout: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Added run workout") {
		t.Errorf("Expected 'Added run workout' in output, got: %s", output)
	}

	// Test workout list
	output, err = run("workout", "list")
	if err != nil {
		t.Fatalf("Failed to list workouts: %v\n%s", err, output)
	}
	if !strings.Contains(output, "run") {
		t.Errorf("Expected 'run' in workout list, got: %s", output)
	}

	// Test export json
	output, err = run("export", "json")
	if err != nil {
		t.Fatalf("Failed to export json: %v\n%s", err, output)
	}
	if !strings.Contains(output, "\"version\": \"1.0\"") {
		t.Errorf("Expected version in JSON export, got: %s", output)
	}

	// Test export yaml
	output, err = run("export", "yaml")
	if err != nil {
		t.Fatalf("Failed to export yaml: %v\n%s", err, output)
	}
	if !strings.Contains(output, "version:") {
		t.Errorf("Expected version in YAML export, got: %s", output)
	}

	// Test export markdown
	output, err = run("export", "markdown")
	if err != nil {
		t.Fatalf("Failed to export markdown: %v\n%s", err, output)
	}
	if !strings.Contains(output, "# Health Export") {
		t.Errorf("Expected markdown header, got: %s", output)
	}
}
