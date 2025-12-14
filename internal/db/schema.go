// ABOUTME: SQL schema definition for health metrics database.
// ABOUTME: Defines metrics, workouts, and workout_metrics tables.
package db

const schema = `
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
    workout_id TEXT NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    metric_name TEXT NOT NULL,
    value REAL NOT NULL,
    unit TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_metrics_type_date ON metrics(metric_type, recorded_at);
CREATE INDEX IF NOT EXISTS idx_workouts_type_date ON workouts(workout_type, started_at);
CREATE INDEX IF NOT EXISTS idx_workout_metrics_workout ON workout_metrics(workout_id);
`
