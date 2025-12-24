// ABOUTME: Charm KV client wrapper using transactional Do API
// ABOUTME: Short-lived connections to avoid lock contention with other MCP servers

package charm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/charm/client"
	"github.com/charmbracelet/charm/kv"
	charmproto "github.com/charmbracelet/charm/proto"
)

const (
	// DBName is the name of the charm kv database for health.
	DBName = "health"

	MetricPrefix        = "metric:"
	WorkoutPrefix       = "workout:"
	WorkoutMetricPrefix = "workout_metric:"
)

// Client holds configuration for KV operations.
// Unlike the previous implementation, it does NOT hold a persistent connection.
// Each operation opens the database, performs the operation, and closes it.
type Client struct {
	dbName   string
	autoSync bool
}

// Option configures a Client.
type Option func(*Client)

// WithDBName sets the database name.
func WithDBName(name string) Option {
	return func(c *Client) {
		c.dbName = name
	}
}

// WithAutoSync enables or disables auto-sync after writes.
func WithAutoSync(enabled bool) Option {
	return func(c *Client) {
		c.autoSync = enabled
	}
}

// NewClient creates a new client with the given options.
func NewClient(opts ...Option) (*Client, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	// Set charm host if configured
	if cfg.CharmHost != "" {
		if err := os.Setenv("CHARM_HOST", cfg.CharmHost); err != nil {
			return nil, err
		}
	}

	c := &Client{
		dbName:   DBName,
		autoSync: cfg.AutoSync,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// DoReadOnly executes a function with read-only database access.
// Use this for batch read operations that need multiple Gets.
func (c *Client) DoReadOnly(fn func(k *kv.KV) error) error {
	return kv.DoReadOnly(c.dbName, fn)
}

// Do executes a function with write access to the database.
// Use this for batch write operations.
func (c *Client) Do(fn func(k *kv.KV) error) error {
	return kv.Do(c.dbName, func(k *kv.KV) error {
		if err := fn(k); err != nil {
			return err
		}
		if c.autoSync {
			return k.Sync()
		}
		return nil
	})
}

// Sync triggers a manual sync with the charm server.
func (c *Client) Sync() error {
	return kv.Do(c.dbName, func(k *kv.KV) error {
		return k.Sync()
	})
}

// Reset clears all data (nuclear option).
func (c *Client) Reset() error {
	return kv.Do(c.dbName, func(k *kv.KV) error {
		return k.Reset()
	})
}

// ID returns the charm user ID for this device.
func (c *Client) ID() (string, error) {
	cc, err := client.NewClientWithDefaults()
	if err != nil {
		return "", err
	}
	return cc.ID()
}

// User returns the current charm user information.
func (c *Client) User() (*charmproto.User, error) {
	cc, err := client.NewClientWithDefaults()
	if err != nil {
		return nil, err
	}
	return cc.Bio()
}

// SetAutoSync enables or disables automatic sync after writes.
func (c *Client) SetAutoSync(enabled bool) {
	c.autoSync = enabled
}

// Config returns the current configuration.
func (c *Client) Config() *Config {
	cfg, _ := LoadConfig()
	return cfg
}

// set stores a value with the given key.
func (c *Client) set(key string, data []byte) error {
	return kv.Do(c.dbName, func(k *kv.KV) error {
		if err := k.Set([]byte(key), data); err != nil {
			return err
		}
		if c.autoSync {
			return k.Sync()
		}
		return nil
	})
}

// delete removes a key.
func (c *Client) delete(key string) error {
	return kv.Do(c.dbName, func(k *kv.KV) error {
		if err := k.Delete([]byte(key)); err != nil {
			return err
		}
		if c.autoSync {
			return k.Sync()
		}
		return nil
	})
}

// listByPrefix returns all values with keys matching the given prefix.
func (c *Client) listByPrefix(prefix string) ([][]byte, error) {
	var results [][]byte
	err := kv.DoReadOnly(c.dbName, func(k *kv.KV) error {
		prefixBytes := []byte(prefix)

		// Get all keys from the database
		keys, err := k.Keys()
		if err != nil {
			return err
		}

		// Filter keys by prefix and retrieve their values
		for _, key := range keys {
			if bytes.HasPrefix(key, prefixBytes) {
				val, err := k.Get(key)
				if err != nil {
					return err
				}
				results = append(results, val)
			}
		}
		return nil
	})
	return results, err
}

// getByIDPrefix retrieves a single value by ID prefix match.
// Returns error if no match or multiple matches found.
func (c *Client) getByIDPrefix(typePrefix, idPrefix string) ([]byte, error) {
	var matches [][]byte
	err := kv.DoReadOnly(c.dbName, func(k *kv.KV) error {
		searchPrefix := []byte(typePrefix + idPrefix)

		// Get all keys from the database
		keys, err := k.Keys()
		if err != nil {
			return err
		}

		// Find keys matching the search prefix
		for _, key := range keys {
			if bytes.HasPrefix(key, searchPrefix) {
				val, err := k.Get(key)
				if err != nil {
					return err
				}
				matches = append(matches, val)
				if len(matches) > 1 {
					break
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("not found: %s", idPrefix)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("ambiguous prefix %s: matches multiple records", idPrefix)
	}

	return matches[0], nil
}

// deleteByIDPrefix deletes a record by ID prefix match.
func (c *Client) deleteByIDPrefix(typePrefix, idPrefix string) error {
	return kv.Do(c.dbName, func(k *kv.KV) error {
		// First find the full key
		var fullKey []byte
		searchPrefix := []byte(typePrefix + idPrefix)

		// Get all keys from the database
		keys, err := k.Keys()
		if err != nil {
			return err
		}

		// Find keys matching the search prefix
		var matches [][]byte
		for _, key := range keys {
			if bytes.HasPrefix(key, searchPrefix) {
				matches = append(matches, key)
				if len(matches) > 1 {
					return fmt.Errorf("ambiguous prefix %s: matches multiple records", idPrefix)
				}
			}
		}

		if len(matches) == 0 {
			return fmt.Errorf("not found: %s", idPrefix)
		}
		fullKey = matches[0]

		if err := k.Delete(fullKey); err != nil {
			return err
		}
		if c.autoSync {
			return k.Sync()
		}
		return nil
	})
}

// unmarshalJSON is a helper to unmarshal JSON data.
func unmarshalJSON[T any](data []byte) (*T, error) {
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// marshalJSON is a helper to marshal data to JSON.
func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

// extractID extracts the ID portion from a prefixed key.
func extractID(key, prefix string) string {
	return strings.TrimPrefix(key, prefix)
}

// --- Legacy compatibility layer ---
// These functions maintain backwards compatibility with existing code.

var globalClient *Client

// InitClient initializes the global charm client.
// With the new architecture, this just creates a Client instance.
func InitClient() (*Client, error) {
	if globalClient != nil {
		return globalClient, nil
	}
	var err error
	globalClient, err = NewClient()
	return globalClient, err
}

// GetClient returns the global client, initializing if needed.
func GetClient() (*Client, error) {
	if globalClient != nil {
		return globalClient, nil
	}
	return InitClient()
}

// Close is a no-op for backwards compatibility.
// With Do API, connections are automatically closed after each operation.
func (c *Client) Close() error {
	return nil
}
