// ABOUTME: Integration tests for health CLI.
// ABOUTME: Tests full workflow from CLI commands.
package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFullWorkflow(t *testing.T) {
	// Build the binary
	projectRoot, _ := filepath.Abs("..")
	healthBinary := filepath.Join(projectRoot, "health")

	buildCmd := exec.Command("go", "build", "-o", healthBinary, "./cmd/health")
	buildCmd.Dir = projectRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\n%s", err, output)
	}
	defer os.Remove(healthBinary)

	// Use temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	run := func(args ...string) (string, error) {
		fullArgs := append([]string{"--db", dbPath}, args...)
		cmd := exec.Command(healthBinary, fullArgs...)
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
}
