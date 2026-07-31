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
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	// Also override XDG_DATA_HOME so no existing health.db is found
	t.Setenv("XDG_DATA_HOME", tmpDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with no config file should not error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// New users should get markdown backend
	if cfg.Backend != "markdown" {
		t.Errorf("Expected Backend %q for new user, got %q", "markdown", cfg.Backend)
	}

	// Config file should be auto-created
	configPath := GetConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Expected config file to be auto-created on first run")
	}
}

func TestLoadExistingSQLiteUser(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// Create a fake health.db to simulate existing SQLite user
	dataDir := filepath.Join(tmpDir, "health")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data dir: %v", err)
	}
	dbPath := filepath.Join(dataDir, "health.db")
	if err := os.WriteFile(dbPath, []byte("fake-sqlite-db"), 0600); err != nil {
		t.Fatalf("Failed to create fake health.db: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with existing health.db should not error: %v", err)
	}

	if cfg.Backend != "sqlite" {
		t.Errorf("Expected Backend %q for existing SQLite user, got %q", "sqlite", cfg.Backend)
	}
}

func TestAutoCreatedConfigContainsValidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// Trigger auto-creation by loading with no config
	_, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Read and validate the auto-created config file
	configPath := GetConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read auto-created config: %v", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Auto-created config is not valid JSON: %v", err)
	}

	if cfg.Backend != "markdown" {
		t.Errorf("Auto-created config backend = %q, want %q", cfg.Backend, "markdown")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

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
	tmpDir := t.TempDir()
	// Point to a non-existent subdirectory
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "nonexistent"))

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
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Write invalid JSON
	configDir := filepath.Join(tmpDir, "health")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "config.json"), []byte("invalid json"), 0600)

	_, err := Load()
	if err == nil {
		t.Error("Expected error for invalid JSON config")
	}
}

func TestGetConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	got := GetConfigPath()
	want := filepath.Join(tmpDir, "health", "config.json")
	if got != want {
		t.Errorf("GetConfigPath() = %q, want %q", got, want)
	}
}

func TestOpenStorageSQLite(t *testing.T) {
	tmpDir := t.TempDir()

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
	tmpDir := t.TempDir()

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

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	// Scalar string fields with omitempty are absent when empty.
	if _, ok := m["backend"]; ok {
		t.Error("Expected 'backend' key to be omitted when empty")
	}
	if _, ok := m["data_dir"]; ok {
		t.Error("Expected 'data_dir' key to be omitted when empty")
	}
}

func TestOpenStorageDefaultBackend(t *testing.T) {
	tmpDir := t.TempDir()

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

func TestSyncConfigRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := &Config{
		Backend: "sqlite",
		DataDir: "/tmp/health-data",
		Sync: SyncConfig{
			Whoop: OAuthProviderConfig{
				ClientID:     "whoop-id",
				ClientSecret: "whoop-secret",
				RedirectURI:  "http://localhost:8080/callback",
			},
			Withings: OAuthProviderConfig{
				ClientID:     "withings-id",
				ClientSecret: "withings-secret",
				RedirectURI:  "http://localhost:8080/callback",
			},
			Emfit: EmfitConfig{
				Token:    "emfit-token",
				DeviceID: "device-123",
			},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if loaded.Sync.Whoop.ClientID != "whoop-id" {
		t.Errorf("Whoop.ClientID = %q, want %q", loaded.Sync.Whoop.ClientID, "whoop-id")
	}
	if loaded.Sync.Whoop.ClientSecret != "whoop-secret" {
		t.Errorf("Whoop.ClientSecret = %q, want %q", loaded.Sync.Whoop.ClientSecret, "whoop-secret")
	}
	if loaded.Sync.Whoop.RedirectURI != "http://localhost:8080/callback" {
		t.Errorf("Whoop.RedirectURI = %q, want %q", loaded.Sync.Whoop.RedirectURI, "http://localhost:8080/callback")
	}
	if loaded.Sync.Withings.ClientID != "withings-id" {
		t.Errorf("Withings.ClientID = %q, want %q", loaded.Sync.Withings.ClientID, "withings-id")
	}
	if loaded.Sync.Emfit.Token != "emfit-token" {
		t.Errorf("Emfit.Token = %q, want %q", loaded.Sync.Emfit.Token, "emfit-token")
	}
	if loaded.Sync.Emfit.DeviceID != "device-123" {
		t.Errorf("Emfit.DeviceID = %q, want %q", loaded.Sync.Emfit.DeviceID, "device-123")
	}
}

func TestSyncConfigFilePerms(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := &Config{
		Backend: "sqlite",
		Sync: SyncConfig{
			Whoop: OAuthProviderConfig{
				ClientID:     "id",
				ClientSecret: "secret",
			},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	info, err := os.Stat(GetConfigPath())
	if err != nil {
		t.Fatalf("Stat config file: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("config file perms = %o, want 0600", perm)
	}
}
