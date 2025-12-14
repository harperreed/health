// ABOUTME: Tests for database initialization and connection.
// ABOUTME: Verifies schema creation and XDG path handling.
package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Verify tables exist
	tables := []string{"metrics", "workouts", "workout_metrics"}
	for _, table := range tables {
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
			table).Scan(&count)
		if err != nil {
			t.Errorf("Error checking table %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("Table %s does not exist", table)
		}
	}
}

func TestGetDefaultDBPath(t *testing.T) {
	// Test with XDG_DATA_HOME set
	tmpDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tmpDir)
	defer os.Unsetenv("XDG_DATA_HOME")

	path := GetDefaultDBPath()
	expected := filepath.Join(tmpDir, "health", "health.db")
	if path != expected {
		t.Errorf("GetDefaultDBPath() = %s, want %s", path, expected)
	}
}

func TestInitDBCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "subdir", "nested", "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Verify directory was created
	dir := filepath.Dir(dbPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("expected directory to be created")
	}
}
