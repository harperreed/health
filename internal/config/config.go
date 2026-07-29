// ABOUTME: Health configuration management with backend selection.
// ABOUTME: Handles settings, preferences, and storage backend factory function.

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/harperreed/health/internal/storage"
	"github.com/harperreed/mdstore"
)

// SyncConfig holds native provider sync settings.
type SyncConfig struct {
	Whoop    OAuthProviderConfig `json:"whoop,omitempty"`
	Withings OAuthProviderConfig `json:"withings,omitempty"`
	Emfit    EmfitConfig         `json:"emfit,omitempty"`
}

// OAuthProviderConfig holds OAuth2 client credentials for a provider.
type OAuthProviderConfig struct {
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	RedirectURI  string `json:"redirect_uri,omitempty"`
}

// EmfitConfig holds credentials for the Emfit QS API.
// If Token is set it is used directly (no login). Otherwise Username+Password
// are used with POST /api/v1/login. DeviceID is always required.
type EmfitConfig struct {
	Token    string `json:"token,omitempty"`
	DeviceID string `json:"device_id,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// Config stores health tool configuration.
type Config struct {
	// Backend selects the storage backend: "sqlite" (default) or "markdown".
	Backend string `json:"backend,omitempty"`

	// DataDir is the root directory for data storage.
	// SQLite puts health.db here. Markdown puts metrics/ and workouts/ folders here.
	// Supports ~ expansion for home directory. Defaults to ~/.local/share/health.
	DataDir string `json:"data_dir,omitempty"`

	// Sync holds native provider sync configuration.
	Sync SyncConfig `json:"sync,omitempty"`
}

// defaultDBFilename is the SQLite database filename used for existing-user detection.
const defaultDBFilename = "health.db"

// defaultFirstRunConfig returns the appropriate default config for first-time runs.
// If an existing SQLite database is found, it preserves SQLite as the backend.
// Otherwise, it defaults to markdown for new users.
func defaultFirstRunConfig() *Config {
	dbPath := filepath.Join(storage.DataDir(), defaultDBFilename)
	_, err := os.Stat(dbPath)
	switch {
	case err == nil:
		return &Config{Backend: "sqlite"}
	case !os.IsNotExist(err):
		fmt.Fprintf(os.Stderr, "warning: could not check for existing database: %v\n", err)
	}
	return &Config{Backend: "markdown"}
}

// GetBackend returns the configured backend, defaulting to "sqlite".
func (c *Config) GetBackend() string {
	if c.Backend == "" {
		return "sqlite"
	}
	return c.Backend
}

// GetDataDir returns the configured data directory with ~ expanded,
// defaulting to the standard XDG data directory.
func (c *Config) GetDataDir() string {
	if c.DataDir == "" {
		return storage.DataDir()
	}
	return ExpandPath(c.DataDir)
}

// ExpandPath expands a leading ~ to the user's home directory.
func ExpandPath(path string) string {
	if path == "" {
		return ""
	}
	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// OpenStorage creates a Repository implementation based on the configured backend.
func (c *Config) OpenStorage() (storage.Repository, error) {
	backend := c.GetBackend()
	dataDir := c.GetDataDir()

	switch backend {
	case "sqlite":
		dbPath := filepath.Join(dataDir, "health.db")
		return storage.Open(dbPath)
	case "markdown":
		return storage.NewMarkdownStore(dataDir)
	default:
		return nil, fmt.Errorf("unknown backend: %q", backend)
	}
}

// GetConfigPath returns the config file path.
func GetConfigPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		homeDir, _ := os.UserHomeDir()
		configDir = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(configDir, "health", "config.json")
}

// Load reads config from disk.
func Load() (*Config, error) {
	path := GetConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := defaultFirstRunConfig()
			if saveErr := cfg.Save(); saveErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not save default config: %v\n", saveErr)
			}
			return cfg, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes config to disk.
func (c *Config) Save() error {
	path := GetConfigPath()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return mdstore.AtomicWrite(path, data)
}
