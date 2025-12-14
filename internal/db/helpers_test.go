// ABOUTME: Shared test helpers for database tests.
// ABOUTME: Provides setupTestDB for creating isolated test database instances.
package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
