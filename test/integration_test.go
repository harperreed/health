// ABOUTME: Integration tests for health CLI with Charm KV backend.
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
// NOTE: This test requires a configured Charm account.
// Skip if SKIP_CHARM_TESTS is set.
func TestFullWorkflow(t *testing.T) {
	if os.Getenv("SKIP_CHARM_TESTS") != "" {
		t.Skip("Skipping Charm integration tests (SKIP_CHARM_TESTS set)")
	}

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
