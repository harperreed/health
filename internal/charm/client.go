// ABOUTME: Charm KV client wrapper for health metrics storage.
// ABOUTME: Provides thread-safe initialization and automatic cloud sync.
package charm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/charm/client"
	"github.com/charmbracelet/charm/kv"
)

const (
	dbName    = "health"
	charmHost = "charm.2389.dev"

	MetricPrefix        = "metric:"
	WorkoutPrefix       = "workout:"
	WorkoutMetricPrefix = "workout_metric:"
)

var (
	globalClient *Client
	clientOnce   sync.Once
	clientErr    error
)

type Client struct {
	kv       *kv.KV
	autoSync bool
	mu       sync.RWMutex
}

// InitClient initializes the global Charm client.
// Thread-safe; can be called multiple times.
func InitClient() (*Client, error) {
	clientOnce.Do(func() {
		// Set server before opening KV
		if err := os.Setenv("CHARM_HOST", charmHost); err != nil {
			clientErr = err
			return
		}

		db, err := kv.OpenWithDefaultsFallback(dbName)
		if err != nil {
			clientErr = err
			return
		}

		globalClient = &Client{
			kv:       db,
			autoSync: true,
		}

		// Pull remote data on startup (skip in read-only mode)
		if !db.IsReadOnly() {
			_ = db.Sync()
		}
	})

	return globalClient, clientErr
}

// GetClient returns the global client, initializing if needed.
func GetClient() (*Client, error) {
	return InitClient()
}

// Close closes the KV database connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.kv != nil {
		return c.kv.Close()
	}
	return nil
}

// IsReadOnly returns true if the database is open in read-only mode.
// This happens when another process (like an MCP server) holds the lock.
func (c *Client) IsReadOnly() bool {
	return c.kv.IsReadOnly()
}

// Sync synchronizes local state with Charm Cloud.
func (c *Client) Sync() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.kv.IsReadOnly() {
		return nil
	}
	return c.kv.Sync()
}

// syncIfEnabled calls Sync if autoSync is enabled.
func (c *Client) syncIfEnabled() {
	if c.autoSync && !c.kv.IsReadOnly() {
		_ = c.kv.Sync()
	}
}

// SetAutoSync enables or disables automatic sync after writes.
func (c *Client) SetAutoSync(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.autoSync = enabled
}

// ID returns the Charm user ID for the current account.
func (c *Client) ID() (string, error) {
	cc, err := client.NewClientWithDefaults()
	if err != nil {
		return "", fmt.Errorf("create charm client: %w", err)
	}
	return cc.ID()
}

// Reset wipes local data and rebuilds from Charm Cloud.
func (c *Client) Reset() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.kv.Reset()
}

// set stores a value with the given key.
func (c *Client) set(key string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.kv.IsReadOnly() {
		return fmt.Errorf("cannot write: database is locked by another process (MCP server?)")
	}

	if err := c.kv.Set([]byte(key), data); err != nil {
		return err
	}
	c.syncIfEnabled()
	return nil
}

// delete removes a key.
func (c *Client) delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.kv.IsReadOnly() {
		return fmt.Errorf("cannot write: database is locked by another process (MCP server?)")
	}

	if err := c.kv.Delete([]byte(key)); err != nil {
		return err
	}
	c.syncIfEnabled()
	return nil
}

// listByPrefix returns all values with keys matching the given prefix.
func (c *Client) listByPrefix(prefix string) ([][]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var results [][]byte
	prefixBytes := []byte(prefix)

	// Get all keys from the database
	keys, err := c.kv.Keys()
	if err != nil {
		return nil, err
	}

	// Filter keys by prefix and retrieve their values
	for _, key := range keys {
		if bytes.HasPrefix(key, prefixBytes) {
			val, err := c.kv.Get(key)
			if err != nil {
				return nil, err
			}
			results = append(results, val)
		}
	}

	return results, nil
}

// getByIDPrefix retrieves a single value by ID prefix match.
// Returns error if no match or multiple matches found.
func (c *Client) getByIDPrefix(typePrefix, idPrefix string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var matches [][]byte
	searchPrefix := []byte(typePrefix + idPrefix)

	// Get all keys from the database
	keys, err := c.kv.Keys()
	if err != nil {
		return nil, err
	}

	// Find keys matching the search prefix
	for _, key := range keys {
		if bytes.HasPrefix(key, searchPrefix) {
			val, err := c.kv.Get(key)
			if err != nil {
				return nil, err
			}
			matches = append(matches, val)
			if len(matches) > 1 {
				break
			}
		}
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
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.kv.IsReadOnly() {
		return fmt.Errorf("cannot write: database is locked by another process (MCP server?)")
	}

	// First find the full key
	var fullKey []byte
	searchPrefix := []byte(typePrefix + idPrefix)

	// Get all keys from the database
	keys, err := c.kv.Keys()
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

	if err := c.kv.Delete(fullKey); err != nil {
		return err
	}
	c.syncIfEnabled()
	return nil
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
