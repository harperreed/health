// ABOUTME: Sync configuration for vault E2E encrypted sync.
// ABOUTME: Stores server, auth tokens, and derived key (hex-encoded seed).
package sync

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/oklog/ulid/v2"
)

// Config stores sync settings.
type Config struct {
	Server       string `json:"server"`
	UserID       string `json:"user_id"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenExpires string `json:"token_expires,omitempty"`
	DerivedKey   string `json:"derived_key"` // hex-encoded seed, NOT the mnemonic
	DeviceID     string `json:"device_id"`
	VaultDB      string `json:"vault_db"`
}

// ConfigDir returns the XDG config directory for health sync.
func ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "health")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "health")
}

// ConfigPath returns the path to the sync config file.
func ConfigPath() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "health", "sync.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "health", "sync.json")
}

// VaultDBPath returns the default path for the vault database.
func VaultDBPath() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "health", "vault.db")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "health", "vault.db")
}

// LoadConfig loads sync config from disk.
func LoadConfig() (*Config, error) {
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{VaultDB: VaultDBPath()}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.VaultDB == "" {
		cfg.VaultDB = VaultDBPath()
	}
	return &cfg, nil
}

// SaveConfig persists sync config to disk.
func SaveConfig(cfg *Config) error {
	if err := os.MkdirAll(ConfigDir(), 0750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(), data, 0600)
}

// IsConfigured returns true if sync is fully configured.
func (c *Config) IsConfigured() bool {
	return c.DerivedKey != "" && c.Token != "" && c.Server != "" && c.UserID != ""
}

// GenerateDeviceID creates a new unique device ID.
func GenerateDeviceID() string {
	return ulid.Make().String()
}

// ClearConfig removes sync config file.
func ClearConfig() error {
	path := ConfigPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(path)
}
