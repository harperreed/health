// ABOUTME: SQLite database connection and lifecycle management.
// ABOUTME: Uses modernc.org/sqlite (pure Go, no CGO required).
package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite database connection.
type DB struct {
	db     *sql.DB
	dbPath string
}

// Open opens or creates a SQLite database at the given path.
func Open(dbPath string) (*DB, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Set file permissions
	if err := os.Chmod(dbPath, 0600); err != nil && !os.IsNotExist(err) {
		_ = db.Close()
		return nil, fmt.Errorf("set database permissions: %w", err)
	}

	d := &DB{db: db, dbPath: dbPath}

	// Configure pragmas for better performance
	if err := d.configurePragmas(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configure pragmas: %w", err)
	}

	// Initialize schema
	if err := d.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize schema: %w", err)
	}

	return d, nil
}

// OpenDefault opens the database at the default XDG data path.
func OpenDefault() (*DB, error) {
	return Open(DefaultDBPath())
}

// DataDir returns the default data directory following XDG spec.
func DataDir() string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, _ := os.UserHomeDir()
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "health")
}

// DefaultDBPath returns the default database path following XDG spec.
func DefaultDBPath() string {
	return filepath.Join(DataDir(), "health.db")
}

// Close closes the database connection.
func (d *DB) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// configurePragmas sets up SQLite for optimal performance.
func (d *DB) configurePragmas() error {
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA synchronous = NORMAL",
	}
	for _, pragma := range pragmas {
		if _, err := d.db.Exec(pragma); err != nil {
			return fmt.Errorf("execute %s: %w", pragma, err)
		}
	}
	return nil
}
