// ABOUTME: Tests for health configuration management.
// ABOUTME: Covers load, save, defaults, backend selection, and path expansion.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGetBackendDefault(t *testing.T) {
	cfg := &Config{}
	if got := cfg.GetBackend(); got != "sqlite" {
		t.Errorf("GetBackend() = %q, want %q", got, "sqlite")
	}
}

func TestGetBackendExplicit(t *testing.T) {
	cfg := &Config{Backend: "markdown"}
	if got := cfg.GetBackend(); got != "markdown" {
		t.Errorf("GetBackend() = %q, want %q", got, "markdown")
	}
}

func TestGetDataDirDefault(t *testing.T) {
	cfg := &Config{}

	// GetDataDir with empty DataDir should return storage.DataDir()
	got := cfg.GetDataDir()
	if got == "" {
		t.Error("GetDataDir() returned empty string")
	}
}

func TestGetDataDirExplicit(t *testing.T) {
	cfg := &Config{DataDir: "/tmp/health-test"}
	if got := cfg.GetDataDir(); got != "/tmp/health-test" {
		t.Errorf("GetDataDir() = %q, want %q", got, "/tmp/health-test")
	}
}

func TestExpandPathEmpty(t *testing.T) {
	if got := ExpandPath(""); got != "" {
		t.Errorf("ExpandPath(\"\") = %q, want %q", got, "")
	}
}

func TestExpandPathAbsolute(t *testing.T) {
	if got := ExpandPath("/tmp/foo"); got != "/tmp/foo" {
		t.Errorf("ExpandPath(\"/tmp/foo\") = %q, want %q", got, "/tmp/foo")
	}
}

func TestExpandPathTilde(t *testing.T) {
	home, _ := os.UserHomeDir()

	got := ExpandPath("~")
	if got != home {
		t.Errorf("ExpandPath(\"~\") = %q, want %q", got, home)
	}
}

func TestExpandPathTildeSlash(t *testing.T) {
	home, _ := os.UserHomeDir()

	got := ExpandPath("~/data/health")
	want := filepath.Join(home, "data/health")
	if got != want {
		t.Errorf("ExpandPath(\"~/data/health\") = %q, want %q", got, want)
	}
}

func TestExpandPathRelative(t *testing.T) {
	if got := ExpandPath("data/health"); got != "data/health" {
		t.Errorf("ExpandPath(\"data/health\") = %q, want %q", got, "data/health")
	}
}

func TestGetDataDirExpandsTilde(t *testing.T) {
	home, _ := os.UserHomeDir()

	cfg := &Config{DataDir: "~/health-data"}
	got := cfg.GetDataDir()
	want := filepath.Join(home, "health-data")
	if got != want {
		t.Errorf("GetDataDir() = %q, want %q", got, want)
	}
}

func TestLoadNonExistentConfig(t *testing.T) {
	// Set XDG_CONFIG_HOME to a temp dir with no config file
	tmpDir, err := os.MkdirTemp("", "health-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", originalXDG)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with no config file should not error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// Should return defaults
	if cfg.Backend != "" {
		t.Errorf("Expected empty Backend, got %q", cfg.Backend)
	}
	if cfg.DataDir != "" {
		t.Errorf("Expected empty DataDir, got %q", cfg.DataDir)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "health-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", originalXDG)

	// Save config
	cfg := &Config{
		Backend: "markdown",
		DataDir: "/tmp/health-data",
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Load config
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if loaded.Backend != "markdown" {
		t.Errorf("Backend mismatch: got %q, want %q", loaded.Backend, "markdown")
	}
	if loaded.DataDir != "/tmp/health-data" {
		t.Errorf("DataDir mismatch: got %q, want %q", loaded.DataDir, "/tmp/health-data")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "health-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Point to a non-existent subdirectory
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "nonexistent"))
	defer os.Setenv("XDG_CONFIG_HOME", originalXDG)

	cfg := &Config{Backend: "sqlite"}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() should create directory: %v", err)
	}

	// Verify directory was created
	configDir := filepath.Join(tmpDir, "nonexistent", "health")
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Error("Expected config directory to be created")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "health-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", originalXDG)

	// Write invalid JSON
	configDir := filepath.Join(tmpDir, "health")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "config.json"), []byte("invalid json"), 0600)

	_, err = Load()
	if err == nil {
		t.Error("Expected error for invalid JSON config")
	}
}

func TestGetConfigPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "health-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", originalXDG)

	got := GetConfigPath()
	want := filepath.Join(tmpDir, "health", "config.json")
	if got != want {
		t.Errorf("GetConfigPath() = %q, want %q", got, want)
	}
}

func TestOpenStorageSQLite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "health-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &Config{
		Backend: "sqlite",
		DataDir: tmpDir,
	}

	repo, err := cfg.OpenStorage()
	if err != nil {
		t.Fatalf("OpenStorage() for sqlite failed: %v", err)
	}
	defer repo.Close()

	if repo == nil {
		t.Error("Expected non-nil repository")
	}

	// Verify database file was created
	dbPath := filepath.Join(tmpDir, "health.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Expected health.db to be created")
	}
}

func TestOpenStorageMarkdown(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "health-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &Config{
		Backend: "markdown",
		DataDir: tmpDir,
	}

	repo, err := cfg.OpenStorage()
	if err != nil {
		t.Fatalf("OpenStorage() for markdown failed: %v", err)
	}
	defer repo.Close()

	if repo == nil {
		t.Error("Expected non-nil repository")
	}
}

func TestOpenStorageInvalidBackend(t *testing.T) {
	cfg := &Config{
		Backend: "invalid",
		DataDir: "/tmp",
	}

	_, err := cfg.OpenStorage()
	if err == nil {
		t.Error("Expected error for invalid backend")
	}
}

func TestConfigJSONSerialization(t *testing.T) {
	cfg := &Config{
		Backend: "markdown",
		DataDir: "~/health-data",
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if loaded.Backend != cfg.Backend {
		t.Errorf("Backend mismatch: got %q, want %q", loaded.Backend, cfg.Backend)
	}
	if loaded.DataDir != cfg.DataDir {
		t.Errorf("DataDir mismatch: got %q, want %q", loaded.DataDir, cfg.DataDir)
	}
}

func TestConfigJSONOmitsEmpty(t *testing.T) {
	cfg := &Config{}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Empty config should result in "{}" since fields have omitempty
	if string(data) != "{}" {
		t.Errorf("Expected empty JSON object, got %s", string(data))
	}
}

func TestOpenStorageDefaultBackend(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "health-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Empty config should use sqlite backend by default
	cfg := &Config{
		DataDir: tmpDir,
	}

	repo, err := cfg.OpenStorage()
	if err != nil {
		t.Fatalf("OpenStorage() with default backend failed: %v", err)
	}
	defer repo.Close()

	if repo == nil {
		t.Error("Expected non-nil repository")
	}
}
