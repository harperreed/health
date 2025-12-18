// ABOUTME: Tests for sync configuration management.
// ABOUTME: Verifies LoadConfig, SaveConfig, IsConfigured, and device ID generation.

package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigNoFile(t *testing.T) {
	// Setup temp directory for config
	tmpDir := t.TempDir()
	origXDGConfig := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		if origXDGConfig != "" {
			_ = os.Setenv("XDG_CONFIG_HOME", origXDGConfig)
		} else {
			_ = os.Unsetenv("XDG_CONFIG_HOME")
		}
	})
	_ = os.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg, err := LoadConfig()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Should return defaults when no file exists
	assert.Equal(t, "", cfg.Server)
	assert.Equal(t, "", cfg.UserID)
	assert.Equal(t, "", cfg.Token)
	assert.Equal(t, "", cfg.DerivedKey)
	assert.NotEmpty(t, cfg.VaultDB) // Should have default path
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	origXDGConfig := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		if origXDGConfig != "" {
			_ = os.Setenv("XDG_CONFIG_HOME", origXDGConfig)
		} else {
			_ = os.Unsetenv("XDG_CONFIG_HOME")
		}
	})
	_ = os.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create test config
	cfg := &Config{
		Server:       "https://test.example.com",
		UserID:       "test-user-123",
		Token:        "test-token-abc",
		RefreshToken: "refresh-token-xyz",
		TokenExpires: "2025-12-31T23:59:59Z",
		DerivedKey:   "test-derived-key-hex",
		DeviceID:     "device-123",
		VaultDB:      filepath.Join(tmpDir, "vault.db"),
	}

	// Save config
	err := SaveConfig(cfg)
	require.NoError(t, err)

	// Verify config file was created
	configPath := ConfigPath()
	assert.FileExists(t, configPath)

	// Load config and verify round-trip
	loaded, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, cfg.Server, loaded.Server)
	assert.Equal(t, cfg.UserID, loaded.UserID)
	assert.Equal(t, cfg.Token, loaded.Token)
	assert.Equal(t, cfg.RefreshToken, loaded.RefreshToken)
	assert.Equal(t, cfg.TokenExpires, loaded.TokenExpires)
	assert.Equal(t, cfg.DerivedKey, loaded.DerivedKey)
	assert.Equal(t, cfg.DeviceID, loaded.DeviceID)
	assert.Equal(t, cfg.VaultDB, loaded.VaultDB)
}

func TestConfigDirXDG(t *testing.T) {
	tmpDir := t.TempDir()
	origXDGConfig := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		if origXDGConfig != "" {
			_ = os.Setenv("XDG_CONFIG_HOME", origXDGConfig)
		} else {
			_ = os.Unsetenv("XDG_CONFIG_HOME")
		}
	})
	_ = os.Setenv("XDG_CONFIG_HOME", tmpDir)

	configDir := ConfigDir()
	assert.Equal(t, filepath.Join(tmpDir, "health"), configDir)
}

func TestConfigDirFallback(t *testing.T) {
	origXDGConfig := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		if origXDGConfig != "" {
			_ = os.Setenv("XDG_CONFIG_HOME", origXDGConfig)
		} else {
			_ = os.Unsetenv("XDG_CONFIG_HOME")
		}
	})
	_ = os.Unsetenv("XDG_CONFIG_HOME")

	configDir := ConfigDir()
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "health")
	assert.Equal(t, expected, configDir)
}

func TestVaultDBPathXDG(t *testing.T) {
	tmpDir := t.TempDir()
	origXDGData := os.Getenv("XDG_DATA_HOME")
	t.Cleanup(func() {
		if origXDGData != "" {
			_ = os.Setenv("XDG_DATA_HOME", origXDGData)
		} else {
			_ = os.Unsetenv("XDG_DATA_HOME")
		}
	})
	_ = os.Setenv("XDG_DATA_HOME", tmpDir)

	vaultPath := VaultDBPath()
	assert.Equal(t, filepath.Join(tmpDir, "health", "vault.db"), vaultPath)
}

func TestVaultDBPathFallback(t *testing.T) {
	origXDGData := os.Getenv("XDG_DATA_HOME")
	t.Cleanup(func() {
		if origXDGData != "" {
			_ = os.Setenv("XDG_DATA_HOME", origXDGData)
		} else {
			_ = os.Unsetenv("XDG_DATA_HOME")
		}
	})
	_ = os.Unsetenv("XDG_DATA_HOME")

	vaultPath := VaultDBPath()
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".local", "share", "health", "vault.db")
	assert.Equal(t, expected, vaultPath)
}

func TestIsConfiguredAllFields(t *testing.T) {
	cfg := &Config{
		Server:     "https://test.example.com",
		UserID:     "user-123",
		Token:      "token-abc",
		DerivedKey: "derived-key-hex",
		DeviceID:   "device-123",
	}

	assert.True(t, cfg.IsConfigured())
}

func TestIsConfiguredMissingDerivedKey(t *testing.T) {
	cfg := &Config{
		Server:   "https://test.example.com",
		UserID:   "user-123",
		Token:    "token-abc",
		DeviceID: "device-123",
	}

	assert.False(t, cfg.IsConfigured())
}

func TestIsConfiguredMissingToken(t *testing.T) {
	cfg := &Config{
		Server:     "https://test.example.com",
		UserID:     "user-123",
		DerivedKey: "derived-key-hex",
		DeviceID:   "device-123",
	}

	assert.False(t, cfg.IsConfigured())
}

func TestIsConfiguredMissingServer(t *testing.T) {
	cfg := &Config{
		UserID:     "user-123",
		Token:      "token-abc",
		DerivedKey: "derived-key-hex",
		DeviceID:   "device-123",
	}

	assert.False(t, cfg.IsConfigured())
}

func TestIsConfiguredMissingUserID(t *testing.T) {
	cfg := &Config{
		Server:     "https://test.example.com",
		Token:      "token-abc",
		DerivedKey: "derived-key-hex",
		DeviceID:   "device-123",
	}

	assert.False(t, cfg.IsConfigured())
}

func TestGenerateDeviceID(t *testing.T) {
	deviceID1 := GenerateDeviceID()
	deviceID2 := GenerateDeviceID()

	// Should generate non-empty IDs
	assert.NotEmpty(t, deviceID1)
	assert.NotEmpty(t, deviceID2)

	// Should generate unique IDs
	assert.NotEqual(t, deviceID1, deviceID2)

	// Should be ULID format (26 characters)
	assert.Len(t, deviceID1, 26)
	assert.Len(t, deviceID2, 26)
}

func TestClearConfig(t *testing.T) {
	tmpDir := t.TempDir()
	origXDGConfig := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		if origXDGConfig != "" {
			_ = os.Setenv("XDG_CONFIG_HOME", origXDGConfig)
		} else {
			_ = os.Unsetenv("XDG_CONFIG_HOME")
		}
	})
	_ = os.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Save a config
	cfg := &Config{
		Server:     "https://test.example.com",
		UserID:     "test-user",
		Token:      "test-token",
		DerivedKey: "test-key",
	}
	err := SaveConfig(cfg)
	require.NoError(t, err)

	// Verify it exists
	assert.FileExists(t, ConfigPath())

	// Clear config
	err = ClearConfig()
	require.NoError(t, err)

	// Verify it's gone
	assert.NoFileExists(t, ConfigPath())
}

func TestClearConfigNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	origXDGConfig := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		if origXDGConfig != "" {
			_ = os.Setenv("XDG_CONFIG_HOME", origXDGConfig)
		} else {
			_ = os.Unsetenv("XDG_CONFIG_HOME")
		}
	})
	_ = os.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Should not error when file doesn't exist
	err := ClearConfig()
	require.NoError(t, err)
}

func TestLoadConfigDefaultsVaultDB(t *testing.T) {
	tmpDir := t.TempDir()
	origXDGConfig := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		if origXDGConfig != "" {
			_ = os.Setenv("XDG_CONFIG_HOME", origXDGConfig)
		} else {
			_ = os.Unsetenv("XDG_CONFIG_HOME")
		}
	})
	_ = os.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Save config without VaultDB
	data := `{"server":"https://test.example.com","user_id":"test-user"}`
	err := os.MkdirAll(ConfigDir(), 0750)
	require.NoError(t, err)
	err = os.WriteFile(ConfigPath(), []byte(data), 0600)
	require.NoError(t, err)

	// Load config
	loaded, err := LoadConfig()
	require.NoError(t, err)

	// Should default VaultDB path
	assert.NotEmpty(t, loaded.VaultDB)
	assert.Equal(t, VaultDBPath(), loaded.VaultDB)
}
