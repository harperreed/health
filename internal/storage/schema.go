// ABOUTME: SQLite schema definition and initialization.
// ABOUTME: Defines tables for metrics, workouts, and workout_metrics.
package storage

import (
	"database/sql"
	"fmt"
)

// initSchema creates or updates the database schema.
func (d *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS metrics (
		id TEXT PRIMARY KEY,
		metric_type TEXT NOT NULL,
		value REAL NOT NULL,
		unit TEXT NOT NULL,
		recorded_at DATETIME NOT NULL,
		notes TEXT,
		source TEXT NOT NULL DEFAULT 'manual',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS workouts (
		id TEXT PRIMARY KEY,
		workout_type TEXT NOT NULL,
		started_at DATETIME NOT NULL,
		duration_minutes INTEGER,
		notes TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS workout_metrics (
		id TEXT PRIMARY KEY,
		workout_id TEXT NOT NULL,
		metric_name TEXT NOT NULL,
		value REAL NOT NULL,
		unit TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (workout_id) REFERENCES workouts(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_metrics_type ON metrics(metric_type);
	CREATE INDEX IF NOT EXISTS idx_metrics_recorded ON metrics(recorded_at DESC);
	CREATE INDEX IF NOT EXISTS idx_metrics_type_recorded ON metrics(metric_type, recorded_at DESC);
	CREATE INDEX IF NOT EXISTS idx_workouts_started ON workouts(started_at DESC);
	CREATE INDEX IF NOT EXISTS idx_workout_metrics_workout ON workout_metrics(workout_id);
	`

	if _, err := d.db.Exec(schema); err != nil {
		return err
	}
	if err := d.ensureMetricSourceColumn(); err != nil {
		return err
	}
	// Source indexes must come after the column migration so pre-source
	// databases have the column before indexing it.
	sourceIndexes := `
	CREATE INDEX IF NOT EXISTS idx_metrics_source ON metrics(source);
	CREATE INDEX IF NOT EXISTS idx_metrics_dedupe ON metrics(source, metric_type, recorded_at);
	`
	_, err := d.db.Exec(sourceIndexes)
	return err
}

// ensureMetricSourceColumn adds the source column to databases created
// before the column existed. Existing rows read back as 'manual'.
func (d *DB) ensureMetricSourceColumn() error {
	rows, err := d.db.Query(`PRAGMA table_info(metrics)`)
	if err != nil {
		return fmt.Errorf("inspect metrics schema: %w", err)
	}
	defer rows.Close()

	hasSource := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("scan metrics schema: %w", err)
		}
		if name == "source" {
			hasSource = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if hasSource {
		return nil
	}
	if _, err := d.db.Exec(`ALTER TABLE metrics ADD COLUMN source TEXT NOT NULL DEFAULT 'manual'`); err != nil {
		return fmt.Errorf("add source column: %w", err)
	}
	return nil
}
