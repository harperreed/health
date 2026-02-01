// ABOUTME: SQLite schema definition and initialization.
// ABOUTME: Defines tables for metrics, workouts, and workout_metrics.
package storage

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

	_, err := d.db.Exec(schema)
	return err
}
