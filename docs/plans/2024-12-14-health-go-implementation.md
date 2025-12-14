# Health Metrics Store (Go) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a SQLite-backed health metrics store with CLI and MCP server, following toki/chronicle patterns.

**Architecture:** Cobra CLI with Persistent{Pre,Post}RunE for DB lifecycle. Internal packages for db, models, mcp, ui. Pure Go SQLite (modernc.org/sqlite) for no CGO dependency. MCP server via stdio transport.

**Tech Stack:** Go 1.21+, Cobra CLI, MCP Go SDK, modernc.org/sqlite, google/uuid, fatih/color

---

## Task 1: Initialize Go Module

**Files:**
- Create: `go.mod`
- Create: `cmd/health/main.go`

**Step 1: Initialize module**

```bash
cd /Users/harper/Public/src/personal/suite/health
go mod init github.com/harperreed/health
```

**Step 2: Create minimal main.go**

Create `cmd/health/main.go`:
```go
// ABOUTME: Entry point for health CLI.
// ABOUTME: Invokes the root Cobra command.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Execute is a placeholder until root.go is created
func Execute() error {
	fmt.Println("health CLI - not yet implemented")
	return nil
}
```

**Step 3: Verify it builds**

```bash
go build -o health ./cmd/health
./health
```
Expected: "health CLI - not yet implemented"

**Step 4: Commit**

```bash
git add go.mod cmd/
git commit -m "feat: initialize Go module and entry point"
```

---

## Task 2: Add Dependencies

**Files:**
- Modify: `go.mod`

**Step 1: Add all dependencies**

```bash
go get github.com/spf13/cobra@latest
go get github.com/google/uuid@latest
go get github.com/fatih/color@latest
go get modernc.org/sqlite@latest
go get github.com/modelcontextprotocol/go-sdk@latest
```

**Step 2: Tidy**

```bash
go mod tidy
```

**Step 3: Verify**

```bash
cat go.mod | grep require -A 10
```
Expected: Should show cobra, uuid, color, sqlite, go-sdk

**Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "feat: add dependencies"
```

---

## Task 3: Create MetricType and Metric Model

**Files:**
- Create: `internal/models/metric.go`
- Create: `internal/models/metric_test.go`

**Step 1: Write the failing test**

Create `internal/models/metric_test.go`:
```go
// ABOUTME: Tests for Metric model and MetricType.
// ABOUTME: Validates type constants, units mapping, and constructor.
package models

import (
	"testing"
	"time"
)

func TestMetricTypeUnit(t *testing.T) {
	tests := []struct {
		metricType MetricType
		wantUnit   string
	}{
		{MetricWeight, "kg"},
		{MetricHRV, "ms"},
		{MetricMood, "scale"},
		{MetricCalories, "kcal"},
	}

	for _, tt := range tests {
		t.Run(string(tt.metricType), func(t *testing.T) {
			got := MetricUnits[tt.metricType]
			if got != tt.wantUnit {
				t.Errorf("MetricUnits[%s] = %s, want %s", tt.metricType, got, tt.wantUnit)
			}
		})
	}
}

func TestNewMetric(t *testing.T) {
	m := NewMetric(MetricWeight, 82.5)

	if m.ID.String() == "" {
		t.Error("expected UUID to be set")
	}
	if m.MetricType != MetricWeight {
		t.Errorf("MetricType = %s, want weight", m.MetricType)
	}
	if m.Value != 82.5 {
		t.Errorf("Value = %f, want 82.5", m.Value)
	}
	if m.Unit != "kg" {
		t.Errorf("Unit = %s, want kg", m.Unit)
	}
	if m.RecordedAt.IsZero() {
		t.Error("expected RecordedAt to be set")
	}
}

func TestAllMetricTypesHaveUnits(t *testing.T) {
	types := []MetricType{
		MetricWeight, MetricBodyFat, MetricBPSys, MetricBPDia,
		MetricHeartRate, MetricHRV, MetricTemperature,
		MetricSteps, MetricSleepHours, MetricActiveCalories,
		MetricWater, MetricCalories, MetricProtein, MetricCarbs, MetricFat,
		MetricMood, MetricEnergy, MetricStress, MetricAnxiety, MetricFocus, MetricMeditation,
	}

	for _, mt := range types {
		if _, ok := MetricUnits[mt]; !ok {
			t.Errorf("MetricType %s has no unit defined", mt)
		}
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/models/... -v
```
Expected: FAIL - package doesn't exist

**Step 3: Write implementation**

Create `internal/models/metric.go`:
```go
// ABOUTME: Metric model and MetricType enum for health data.
// ABOUTME: Defines 22 metric types across biometrics, activity, nutrition, mental health.
package models

import (
	"time"

	"github.com/google/uuid"
)

// MetricType represents the type of health metric being recorded.
type MetricType string

const (
	// Biometrics
	MetricWeight      MetricType = "weight"
	MetricBodyFat     MetricType = "body_fat"
	MetricBPSys       MetricType = "bp_sys"
	MetricBPDia       MetricType = "bp_dia"
	MetricHeartRate   MetricType = "heart_rate"
	MetricHRV         MetricType = "hrv"
	MetricTemperature MetricType = "temperature"

	// Activity
	MetricSteps          MetricType = "steps"
	MetricSleepHours     MetricType = "sleep_hours"
	MetricActiveCalories MetricType = "active_calories"

	// Nutrition
	MetricWater    MetricType = "water"
	MetricCalories MetricType = "calories"
	MetricProtein  MetricType = "protein"
	MetricCarbs    MetricType = "carbs"
	MetricFat      MetricType = "fat"

	// Mental Health
	MetricMood       MetricType = "mood"
	MetricEnergy     MetricType = "energy"
	MetricStress     MetricType = "stress"
	MetricAnxiety    MetricType = "anxiety"
	MetricFocus      MetricType = "focus"
	MetricMeditation MetricType = "meditation"
)

// MetricUnits maps metric types to their display units.
var MetricUnits = map[MetricType]string{
	MetricWeight:         "kg",
	MetricBodyFat:        "%",
	MetricBPSys:          "mmHg",
	MetricBPDia:          "mmHg",
	MetricHeartRate:      "bpm",
	MetricHRV:            "ms",
	MetricTemperature:    "°C",
	MetricSteps:          "steps",
	MetricSleepHours:     "hours",
	MetricActiveCalories: "kcal",
	MetricWater:          "ml",
	MetricCalories:       "kcal",
	MetricProtein:        "g",
	MetricCarbs:          "g",
	MetricFat:            "g",
	MetricMood:           "scale",
	MetricEnergy:         "scale",
	MetricStress:         "scale",
	MetricAnxiety:        "scale",
	MetricFocus:          "scale",
	MetricMeditation:     "min",
}

// AllMetricTypes returns all valid metric types.
var AllMetricTypes = []MetricType{
	MetricWeight, MetricBodyFat, MetricBPSys, MetricBPDia,
	MetricHeartRate, MetricHRV, MetricTemperature,
	MetricSteps, MetricSleepHours, MetricActiveCalories,
	MetricWater, MetricCalories, MetricProtein, MetricCarbs, MetricFat,
	MetricMood, MetricEnergy, MetricStress, MetricAnxiety, MetricFocus, MetricMeditation,
}

// IsValidMetricType checks if a string is a valid metric type.
func IsValidMetricType(s string) bool {
	for _, mt := range AllMetricTypes {
		if string(mt) == s {
			return true
		}
	}
	return false
}

// Metric represents a single health metric entry.
type Metric struct {
	ID         uuid.UUID
	MetricType MetricType
	Value      float64
	Unit       string
	RecordedAt time.Time
	Notes      *string
	CreatedAt  time.Time
}

// NewMetric creates a new Metric with generated UUID and current timestamp.
func NewMetric(metricType MetricType, value float64) *Metric {
	now := time.Now()
	return &Metric{
		ID:         uuid.New(),
		MetricType: metricType,
		Value:      value,
		Unit:       MetricUnits[metricType],
		RecordedAt: now,
		CreatedAt:  now,
	}
}

// WithRecordedAt sets a custom recorded_at timestamp.
func (m *Metric) WithRecordedAt(t time.Time) *Metric {
	m.RecordedAt = t
	return m
}

// WithNotes sets notes on the metric.
func (m *Metric) WithNotes(notes string) *Metric {
	m.Notes = &notes
	return m
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/models/... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/models/
git commit -m "feat: add Metric model and MetricType enum"
```

---

## Task 4: Create Workout Models

**Files:**
- Create: `internal/models/workout.go`
- Create: `internal/models/workout_test.go`

**Step 1: Write the failing test**

Create `internal/models/workout_test.go`:
```go
// ABOUTME: Tests for Workout and WorkoutMetric models.
// ABOUTME: Validates constructors and builder methods.
package models

import (
	"testing"
)

func TestNewWorkout(t *testing.T) {
	w := NewWorkout("run")

	if w.ID.String() == "" {
		t.Error("expected UUID to be set")
	}
	if w.WorkoutType != "run" {
		t.Errorf("WorkoutType = %s, want run", w.WorkoutType)
	}
	if w.StartedAt.IsZero() {
		t.Error("expected StartedAt to be set")
	}
}

func TestWorkoutWithDuration(t *testing.T) {
	w := NewWorkout("lift").WithDuration(45)

	if w.DurationMinutes == nil || *w.DurationMinutes != 45 {
		t.Error("expected DurationMinutes to be 45")
	}
}

func TestNewWorkoutMetric(t *testing.T) {
	w := NewWorkout("run")
	wm := NewWorkoutMetric(w.ID, "distance", 5.2, "km")

	if wm.WorkoutID != w.ID {
		t.Error("expected WorkoutID to match")
	}
	if wm.MetricName != "distance" {
		t.Errorf("MetricName = %s, want distance", wm.MetricName)
	}
	if wm.Value != 5.2 {
		t.Errorf("Value = %f, want 5.2", wm.Value)
	}
	if wm.Unit == nil || *wm.Unit != "km" {
		t.Error("expected Unit to be km")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/models/... -v
```
Expected: FAIL - Workout not defined

**Step 3: Write implementation**

Create `internal/models/workout.go`:
```go
// ABOUTME: Workout and WorkoutMetric models for exercise tracking.
// ABOUTME: Workouts contain sub-metrics like distance, pace, sets, reps.
package models

import (
	"time"

	"github.com/google/uuid"
)

// Workout represents an exercise session.
type Workout struct {
	ID              uuid.UUID
	WorkoutType     string
	StartedAt       time.Time
	DurationMinutes *int
	Notes           *string
	CreatedAt       time.Time
	Metrics         []WorkoutMetric // Populated when fetching full workout
}

// NewWorkout creates a new Workout with generated UUID and current timestamp.
func NewWorkout(workoutType string) *Workout {
	now := time.Now()
	return &Workout{
		ID:          uuid.New(),
		WorkoutType: workoutType,
		StartedAt:   now,
		CreatedAt:   now,
	}
}

// WithDuration sets the workout duration in minutes.
func (w *Workout) WithDuration(minutes int) *Workout {
	w.DurationMinutes = &minutes
	return w
}

// WithNotes sets notes on the workout.
func (w *Workout) WithNotes(notes string) *Workout {
	w.Notes = &notes
	return w
}

// WithStartedAt sets a custom start timestamp.
func (w *Workout) WithStartedAt(t time.Time) *Workout {
	w.StartedAt = t
	return w
}

// WorkoutMetric represents a measurement within a workout.
type WorkoutMetric struct {
	ID        uuid.UUID
	WorkoutID uuid.UUID
	MetricName string
	Value      float64
	Unit       *string
	CreatedAt  time.Time
}

// NewWorkoutMetric creates a new WorkoutMetric.
func NewWorkoutMetric(workoutID uuid.UUID, name string, value float64, unit string) *WorkoutMetric {
	var unitPtr *string
	if unit != "" {
		unitPtr = &unit
	}
	return &WorkoutMetric{
		ID:         uuid.New(),
		WorkoutID:  workoutID,
		MetricName: name,
		Value:      value,
		Unit:       unitPtr,
		CreatedAt:  time.Now(),
	}
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/models/... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/models/workout*.go
git commit -m "feat: add Workout and WorkoutMetric models"
```

---

## Task 5: Database Connection and Schema

**Files:**
- Create: `internal/db/db.go`
- Create: `internal/db/schema.go`
- Create: `internal/db/db_test.go`

**Step 1: Write the failing test**

Create `internal/db/db_test.go`:
```go
// ABOUTME: Tests for database initialization and connection.
// ABOUTME: Verifies schema creation and XDG path handling.
package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Verify tables exist
	tables := []string{"metrics", "workouts", "workout_metrics"}
	for _, table := range tables {
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
			table).Scan(&count)
		if err != nil {
			t.Errorf("Error checking table %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("Table %s does not exist", table)
		}
	}
}

func TestGetDefaultDBPath(t *testing.T) {
	// Test with XDG_DATA_HOME set
	tmpDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tmpDir)
	defer os.Unsetenv("XDG_DATA_HOME")

	path := GetDefaultDBPath()
	expected := filepath.Join(tmpDir, "health", "health.db")
	if path != expected {
		t.Errorf("GetDefaultDBPath() = %s, want %s", path, expected)
	}
}

func TestInitDBCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "subdir", "nested", "test.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Verify directory was created
	dir := filepath.Dir(dbPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("expected directory to be created")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/db/... -v
```
Expected: FAIL - package doesn't exist

**Step 3: Write schema.go**

Create `internal/db/schema.go`:
```go
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
```

**Step 4: Write db.go**

Create `internal/db/db.go`:
```go
// ABOUTME: Database connection management for health metrics store.
// ABOUTME: Handles initialization, XDG paths, and SQLite pragmas.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// GetDefaultDBPath returns the default database path following XDG spec.
func GetDefaultDBPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		dataDir = filepath.Join(homeDir, ".local", "share")
	}

	return filepath.Join(dataDir, "health", "health.db")
}

// InitDB initializes the database connection and creates schema.
func InitDB(dbPath string) (*sql.DB, error) {
	// Create directory if needed
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open connection
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set pragmas
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("failed to set pragma: %w", err)
		}
	}

	// Run schema
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return db, nil
}
```

**Step 5: Run test to verify it passes**

```bash
go test ./internal/db/... -v
```
Expected: PASS

**Step 6: Commit**

```bash
git add internal/db/
git commit -m "feat: add database connection with XDG paths and schema"
```

---

## Task 6: Metrics CRUD Operations

**Files:**
- Create: `internal/db/metrics.go`
- Create: `internal/db/metrics_test.go`

**Step 1: Write the failing test**

Create `internal/db/metrics_test.go`:
```go
// ABOUTME: Tests for metrics CRUD operations.
// ABOUTME: Validates create, get, list, and delete functions.
package db

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/harperreed/health/internal/models"
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

func TestCreateAndGetMetric(t *testing.T) {
	db := setupTestDB(t)

	m := models.NewMetric(models.MetricWeight, 82.5)
	if err := CreateMetric(db, m); err != nil {
		t.Fatalf("CreateMetric failed: %v", err)
	}

	got, err := GetMetric(db, m.ID.String())
	if err != nil {
		t.Fatalf("GetMetric failed: %v", err)
	}

	if got.ID != m.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, m.ID)
	}
	if got.Value != 82.5 {
		t.Errorf("Value mismatch: got %f, want 82.5", got.Value)
	}
}

func TestListMetrics(t *testing.T) {
	db := setupTestDB(t)

	// Create some metrics
	m1 := models.NewMetric(models.MetricWeight, 82.5)
	m2 := models.NewMetric(models.MetricWeight, 82.0)
	m3 := models.NewMetric(models.MetricHRV, 45)

	CreateMetric(db, m1)
	CreateMetric(db, m2)
	CreateMetric(db, m3)

	// List all
	metrics, err := ListMetrics(db, nil, 10)
	if err != nil {
		t.Fatalf("ListMetrics failed: %v", err)
	}
	if len(metrics) != 3 {
		t.Errorf("expected 3 metrics, got %d", len(metrics))
	}

	// List by type
	weightType := models.MetricWeight
	metrics, err = ListMetrics(db, &weightType, 10)
	if err != nil {
		t.Fatalf("ListMetrics by type failed: %v", err)
	}
	if len(metrics) != 2 {
		t.Errorf("expected 2 weight metrics, got %d", len(metrics))
	}
}

func TestDeleteMetric(t *testing.T) {
	db := setupTestDB(t)

	m := models.NewMetric(models.MetricWeight, 82.5)
	CreateMetric(db, m)

	if err := DeleteMetric(db, m.ID.String()); err != nil {
		t.Fatalf("DeleteMetric failed: %v", err)
	}

	_, err := GetMetric(db, m.ID.String())
	if err == nil {
		t.Error("expected error getting deleted metric")
	}
}

func TestGetMetricByPrefix(t *testing.T) {
	db := setupTestDB(t)

	m := models.NewMetric(models.MetricWeight, 82.5)
	CreateMetric(db, m)

	// Get by first 8 chars of UUID
	prefix := m.ID.String()[:8]
	got, err := GetMetric(db, prefix)
	if err != nil {
		t.Fatalf("GetMetric by prefix failed: %v", err)
	}

	if got.ID != m.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, m.ID)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/db/... -v
```
Expected: FAIL - CreateMetric undefined

**Step 3: Write implementation**

Create `internal/db/metrics.go`:
```go
// ABOUTME: Metrics CRUD operations for the health database.
// ABOUTME: Supports create, get (with prefix matching), list, and delete.
package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/harperreed/health/internal/models"
)

// CreateMetric inserts a new metric into the database.
func CreateMetric(db *sql.DB, m *models.Metric) error {
	_, err := db.Exec(`
		INSERT INTO metrics (id, metric_type, value, unit, recorded_at, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.ID.String(), string(m.MetricType), m.Value, m.Unit,
		m.RecordedAt.Format(time.RFC3339), m.Notes,
		m.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("failed to create metric: %w", err)
	}
	return nil
}

// GetMetric retrieves a metric by ID or ID prefix.
func GetMetric(db *sql.DB, idOrPrefix string) (*models.Metric, error) {
	var row *sql.Row
	if len(idOrPrefix) < 36 {
		// Prefix match
		row = db.QueryRow(`
			SELECT id, metric_type, value, unit, recorded_at, notes, created_at
			FROM metrics WHERE id LIKE ? LIMIT 1`, idOrPrefix+"%")
	} else {
		row = db.QueryRow(`
			SELECT id, metric_type, value, unit, recorded_at, notes, created_at
			FROM metrics WHERE id = ?`, idOrPrefix)
	}

	return scanMetric(row)
}

// ListMetrics retrieves recent metrics, optionally filtered by type.
func ListMetrics(db *sql.DB, metricType *models.MetricType, limit int) ([]*models.Metric, error) {
	var rows *sql.Rows
	var err error

	if metricType != nil {
		rows, err = db.Query(`
			SELECT id, metric_type, value, unit, recorded_at, notes, created_at
			FROM metrics WHERE metric_type = ?
			ORDER BY recorded_at DESC LIMIT ?`, string(*metricType), limit)
	} else {
		rows, err = db.Query(`
			SELECT id, metric_type, value, unit, recorded_at, notes, created_at
			FROM metrics ORDER BY recorded_at DESC LIMIT ?`, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list metrics: %w", err)
	}
	defer rows.Close()

	var metrics []*models.Metric
	for rows.Next() {
		m, err := scanMetricRows(rows)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}

	return metrics, rows.Err()
}

// DeleteMetric removes a metric by ID or prefix.
func DeleteMetric(db *sql.DB, idOrPrefix string) error {
	var result sql.Result
	var err error

	if len(idOrPrefix) < 36 {
		result, err = db.Exec("DELETE FROM metrics WHERE id LIKE ?", idOrPrefix+"%")
	} else {
		result, err = db.Exec("DELETE FROM metrics WHERE id = ?", idOrPrefix)
	}

	if err != nil {
		return fmt.Errorf("failed to delete metric: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("metric not found: %s", idOrPrefix)
	}

	return nil
}

func scanMetric(row *sql.Row) (*models.Metric, error) {
	var m models.Metric
	var idStr, metricType string
	var recordedAt, createdAt string

	err := row.Scan(&idStr, &metricType, &m.Value, &m.Unit, &recordedAt, &m.Notes, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to scan metric: %w", err)
	}

	m.ID, _ = uuid.Parse(idStr)
	m.MetricType = models.MetricType(metricType)
	m.RecordedAt, _ = time.Parse(time.RFC3339, recordedAt)
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

	return &m, nil
}

func scanMetricRows(rows *sql.Rows) (*models.Metric, error) {
	var m models.Metric
	var idStr, metricType string
	var recordedAt, createdAt string

	err := rows.Scan(&idStr, &metricType, &m.Value, &m.Unit, &recordedAt, &m.Notes, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to scan metric: %w", err)
	}

	m.ID, _ = uuid.Parse(idStr)
	m.MetricType = models.MetricType(metricType)
	m.RecordedAt, _ = time.Parse(time.RFC3339, recordedAt)
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

	return &m, nil
}
```

**Step 4: Fix test imports**

Update `internal/db/metrics_test.go` to add missing import:
```go
import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/harperreed/health/internal/models"
)
```

**Step 5: Run test to verify it passes**

```bash
go test ./internal/db/... -v
```
Expected: PASS

**Step 6: Commit**

```bash
git add internal/db/metrics*.go
git commit -m "feat: add metrics CRUD operations"
```

---

## Task 7: Workouts CRUD Operations

**Files:**
- Create: `internal/db/workouts.go`
- Create: `internal/db/workouts_test.go`

**Step 1: Write the failing test**

Create `internal/db/workouts_test.go`:
```go
// ABOUTME: Tests for workout CRUD operations.
// ABOUTME: Validates workout and workout_metrics functions.
package db

import (
	"testing"

	"github.com/harperreed/health/internal/models"
)

func TestCreateAndGetWorkout(t *testing.T) {
	db := setupTestDB(t)

	w := models.NewWorkout("run").WithDuration(45)
	if err := CreateWorkout(db, w); err != nil {
		t.Fatalf("CreateWorkout failed: %v", err)
	}

	got, err := GetWorkout(db, w.ID.String())
	if err != nil {
		t.Fatalf("GetWorkout failed: %v", err)
	}

	if got.ID != w.ID {
		t.Errorf("ID mismatch")
	}
	if got.WorkoutType != "run" {
		t.Errorf("WorkoutType = %s, want run", got.WorkoutType)
	}
	if got.DurationMinutes == nil || *got.DurationMinutes != 45 {
		t.Error("DurationMinutes mismatch")
	}
}

func TestAddWorkoutMetric(t *testing.T) {
	db := setupTestDB(t)

	w := models.NewWorkout("run")
	CreateWorkout(db, w)

	wm := models.NewWorkoutMetric(w.ID, "distance", 5.2, "km")
	if err := AddWorkoutMetric(db, wm); err != nil {
		t.Fatalf("AddWorkoutMetric failed: %v", err)
	}

	// Get workout with metrics
	got, err := GetWorkoutWithMetrics(db, w.ID.String())
	if err != nil {
		t.Fatalf("GetWorkoutWithMetrics failed: %v", err)
	}

	if len(got.Metrics) != 1 {
		t.Errorf("expected 1 metric, got %d", len(got.Metrics))
	}
	if got.Metrics[0].MetricName != "distance" {
		t.Errorf("MetricName = %s, want distance", got.Metrics[0].MetricName)
	}
}

func TestListWorkouts(t *testing.T) {
	db := setupTestDB(t)

	w1 := models.NewWorkout("run")
	w2 := models.NewWorkout("lift")
	CreateWorkout(db, w1)
	CreateWorkout(db, w2)

	workouts, err := ListWorkouts(db, nil, 10)
	if err != nil {
		t.Fatalf("ListWorkouts failed: %v", err)
	}
	if len(workouts) != 2 {
		t.Errorf("expected 2 workouts, got %d", len(workouts))
	}

	// Filter by type
	runType := "run"
	workouts, err = ListWorkouts(db, &runType, 10)
	if err != nil {
		t.Fatalf("ListWorkouts by type failed: %v", err)
	}
	if len(workouts) != 1 {
		t.Errorf("expected 1 run workout, got %d", len(workouts))
	}
}

func TestDeleteWorkoutCascades(t *testing.T) {
	db := setupTestDB(t)

	w := models.NewWorkout("run")
	CreateWorkout(db, w)

	wm := models.NewWorkoutMetric(w.ID, "distance", 5.2, "km")
	AddWorkoutMetric(db, wm)

	if err := DeleteWorkout(db, w.ID.String()); err != nil {
		t.Fatalf("DeleteWorkout failed: %v", err)
	}

	// Verify cascade delete
	var count int
	db.QueryRow("SELECT COUNT(*) FROM workout_metrics WHERE workout_id = ?", w.ID.String()).Scan(&count)
	if count != 0 {
		t.Error("expected workout_metrics to be cascade deleted")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/db/... -v
```
Expected: FAIL - CreateWorkout undefined

**Step 3: Write implementation**

Create `internal/db/workouts.go`:
```go
// ABOUTME: Workout CRUD operations for the health database.
// ABOUTME: Supports workouts with sub-metrics, cascade delete.
package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/harperreed/health/internal/models"
)

// CreateWorkout inserts a new workout into the database.
func CreateWorkout(db *sql.DB, w *models.Workout) error {
	_, err := db.Exec(`
		INSERT INTO workouts (id, workout_type, started_at, duration_minutes, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		w.ID.String(), w.WorkoutType, w.StartedAt.Format(time.RFC3339),
		w.DurationMinutes, w.Notes, w.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("failed to create workout: %w", err)
	}
	return nil
}

// GetWorkout retrieves a workout by ID or prefix (without metrics).
func GetWorkout(db *sql.DB, idOrPrefix string) (*models.Workout, error) {
	var row *sql.Row
	if len(idOrPrefix) < 36 {
		row = db.QueryRow(`
			SELECT id, workout_type, started_at, duration_minutes, notes, created_at
			FROM workouts WHERE id LIKE ? LIMIT 1`, idOrPrefix+"%")
	} else {
		row = db.QueryRow(`
			SELECT id, workout_type, started_at, duration_minutes, notes, created_at
			FROM workouts WHERE id = ?`, idOrPrefix)
	}
	return scanWorkout(row)
}

// GetWorkoutWithMetrics retrieves a workout with all its metrics.
func GetWorkoutWithMetrics(db *sql.DB, idOrPrefix string) (*models.Workout, error) {
	w, err := GetWorkout(db, idOrPrefix)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT id, workout_id, metric_name, value, unit, created_at
		FROM workout_metrics WHERE workout_id = ?`, w.ID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get workout metrics: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		wm, err := scanWorkoutMetric(rows)
		if err != nil {
			return nil, err
		}
		w.Metrics = append(w.Metrics, *wm)
	}

	return w, rows.Err()
}

// AddWorkoutMetric adds a metric to an existing workout.
func AddWorkoutMetric(db *sql.DB, wm *models.WorkoutMetric) error {
	_, err := db.Exec(`
		INSERT INTO workout_metrics (id, workout_id, metric_name, value, unit, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		wm.ID.String(), wm.WorkoutID.String(), wm.MetricName,
		wm.Value, wm.Unit, wm.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("failed to add workout metric: %w", err)
	}
	return nil
}

// ListWorkouts retrieves recent workouts, optionally filtered by type.
func ListWorkouts(db *sql.DB, workoutType *string, limit int) ([]*models.Workout, error) {
	var rows *sql.Rows
	var err error

	if workoutType != nil {
		rows, err = db.Query(`
			SELECT id, workout_type, started_at, duration_minutes, notes, created_at
			FROM workouts WHERE workout_type = ?
			ORDER BY started_at DESC LIMIT ?`, *workoutType, limit)
	} else {
		rows, err = db.Query(`
			SELECT id, workout_type, started_at, duration_minutes, notes, created_at
			FROM workouts ORDER BY started_at DESC LIMIT ?`, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list workouts: %w", err)
	}
	defer rows.Close()

	var workouts []*models.Workout
	for rows.Next() {
		w, err := scanWorkoutRows(rows)
		if err != nil {
			return nil, err
		}
		workouts = append(workouts, w)
	}

	return workouts, rows.Err()
}

// DeleteWorkout removes a workout and its metrics (cascade).
func DeleteWorkout(db *sql.DB, idOrPrefix string) error {
	var result sql.Result
	var err error

	if len(idOrPrefix) < 36 {
		result, err = db.Exec("DELETE FROM workouts WHERE id LIKE ?", idOrPrefix+"%")
	} else {
		result, err = db.Exec("DELETE FROM workouts WHERE id = ?", idOrPrefix)
	}

	if err != nil {
		return fmt.Errorf("failed to delete workout: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("workout not found: %s", idOrPrefix)
	}

	return nil
}

func scanWorkout(row *sql.Row) (*models.Workout, error) {
	var w models.Workout
	var idStr, startedAt, createdAt string

	err := row.Scan(&idStr, &w.WorkoutType, &startedAt, &w.DurationMinutes, &w.Notes, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to scan workout: %w", err)
	}

	w.ID, _ = uuid.Parse(idStr)
	w.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	w.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

	return &w, nil
}

func scanWorkoutRows(rows *sql.Rows) (*models.Workout, error) {
	var w models.Workout
	var idStr, startedAt, createdAt string

	err := rows.Scan(&idStr, &w.WorkoutType, &startedAt, &w.DurationMinutes, &w.Notes, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to scan workout: %w", err)
	}

	w.ID, _ = uuid.Parse(idStr)
	w.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	w.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

	return &w, nil
}

func scanWorkoutMetric(rows *sql.Rows) (*models.WorkoutMetric, error) {
	var wm models.WorkoutMetric
	var idStr, workoutIDStr, createdAt string

	err := rows.Scan(&idStr, &workoutIDStr, &wm.MetricName, &wm.Value, &wm.Unit, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to scan workout metric: %w", err)
	}

	wm.ID, _ = uuid.Parse(idStr)
	wm.WorkoutID, _ = uuid.Parse(workoutIDStr)
	wm.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

	return &wm, nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/db/... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/db/workouts*.go
git commit -m "feat: add workout CRUD operations"
```

---

## Task 8: CLI Root Command

**Files:**
- Create: `cmd/health/root.go`
- Modify: `cmd/health/main.go`

**Step 1: Create root.go**

Create `cmd/health/root.go`:
```go
// ABOUTME: Root Cobra command for health CLI.
// ABOUTME: Handles database lifecycle via PersistentPre/PostRunE.
package main

import (
	"database/sql"
	"fmt"

	"github.com/harperreed/health/internal/db"
	"github.com/spf13/cobra"
)

var (
	dbPath string
	dbConn *sql.DB
)

var rootCmd = &cobra.Command{
	Use:   "health",
	Short: "Health metrics tracker",
	Long:  `A CLI tool for tracking health metrics including biometrics, activity, nutrition, and mental health.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip DB init for commands that don't need it
		if cmd.Name() == "version" || cmd.Name() == "help" {
			return nil
		}

		var err error
		dbConn, err = db.InitDB(dbPath)
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if dbConn != nil {
			return dbConn.Close()
		}
		return nil
	},
}

func init() {
	defaultPath := db.GetDefaultDBPath()
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", defaultPath, "database file path")
}
```

**Step 2: Update main.go**

Update `cmd/health/main.go`:
```go
// ABOUTME: Entry point for health CLI.
// ABOUTME: Invokes the root Cobra command.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

**Step 3: Verify it builds and runs**

```bash
go build -o health ./cmd/health
./health --help
```
Expected: Shows help with --db flag

**Step 4: Commit**

```bash
git add cmd/health/
git commit -m "feat: add CLI root command with database lifecycle"
```

---

## Task 9: CLI Add Command

**Files:**
- Create: `cmd/health/add.go`

**Step 1: Create add.go**

Create `cmd/health/add.go`:
```go
// ABOUTME: CLI command for adding health metrics.
// ABOUTME: Handles single metrics and blood pressure special case.
package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/fatih/color"
	"github.com/harperreed/health/internal/db"
	"github.com/harperreed/health/internal/models"
	"github.com/spf13/cobra"
)

var (
	addAt    string
	addNotes string
)

var addCmd = &cobra.Command{
	Use:     "add <type> <value> [value2]",
	Aliases: []string{"a"},
	Short:   "Add a health metric",
	Long: `Add a health metric. For blood pressure, provide both systolic and diastolic values.

Examples:
  health add weight 82.5
  health add hrv 48 --at "2024-12-14 07:00"
  health add bp 120 80
  health add mood 7 --notes "Good day"`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		metricType := args[0]

		// Handle blood pressure special case
		if metricType == "bp" {
			if len(args) < 3 {
				return fmt.Errorf("blood pressure requires two values: systolic and diastolic")
			}
			return addBloodPressure(args[1], args[2])
		}

		// Validate metric type
		if !models.IsValidMetricType(metricType) {
			return fmt.Errorf("unknown metric type: %s\nValid types: weight, body_fat, bp_sys, bp_dia, heart_rate, hrv, temperature, steps, sleep_hours, active_calories, water, calories, protein, carbs, fat, mood, energy, stress, anxiety, focus, meditation", metricType)
		}

		value, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return fmt.Errorf("invalid value: %s", args[1])
		}

		m := models.NewMetric(models.MetricType(metricType), value)

		// Handle --at flag
		if addAt != "" {
			t, err := parseTime(addAt)
			if err != nil {
				return fmt.Errorf("invalid timestamp: %s", addAt)
			}
			m.WithRecordedAt(t)
		}

		// Handle --notes flag
		if addNotes != "" {
			m.WithNotes(addNotes)
		}

		if err := db.CreateMetric(dbConn, m); err != nil {
			return fmt.Errorf("failed to create metric: %w", err)
		}

		color.Green("✓ Added %s", metricType)
		fmt.Printf("  %s %.2f %s\n",
			color.New(color.Faint).Sprint(m.ID.String()[:8]),
			m.Value, m.Unit)

		return nil
	},
}

func addBloodPressure(sysStr, diaStr string) error {
	sys, err := strconv.ParseFloat(sysStr, 64)
	if err != nil {
		return fmt.Errorf("invalid systolic value: %s", sysStr)
	}
	dia, err := strconv.ParseFloat(diaStr, 64)
	if err != nil {
		return fmt.Errorf("invalid diastolic value: %s", diaStr)
	}

	// Use same timestamp for both
	var recordedAt time.Time
	if addAt != "" {
		var err error
		recordedAt, err = parseTime(addAt)
		if err != nil {
			return fmt.Errorf("invalid timestamp: %s", addAt)
		}
	} else {
		recordedAt = time.Now()
	}

	mSys := models.NewMetric(models.MetricBPSys, sys).WithRecordedAt(recordedAt)
	mDia := models.NewMetric(models.MetricBPDia, dia).WithRecordedAt(recordedAt)

	if addNotes != "" {
		mSys.WithNotes(addNotes)
		mDia.WithNotes(addNotes)
	}

	if err := db.CreateMetric(dbConn, mSys); err != nil {
		return fmt.Errorf("failed to create bp_sys: %w", err)
	}
	if err := db.CreateMetric(dbConn, mDia); err != nil {
		return fmt.Errorf("failed to create bp_dia: %w", err)
	}

	color.Green("✓ Added blood pressure")
	fmt.Printf("  %s %.0f/%.0f mmHg\n",
		color.New(color.Faint).Sprint(mSys.ID.String()[:8]),
		sys, dia)

	return nil
}

func parseTime(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02 15:04",
		"2006-01-02T15:04",
		"2006-01-02",
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time format")
}

func init() {
	addCmd.Flags().StringVar(&addAt, "at", "", "timestamp (YYYY-MM-DD HH:MM)")
	addCmd.Flags().StringVar(&addNotes, "notes", "", "notes for the metric")
	rootCmd.AddCommand(addCmd)
}
```

**Step 2: Verify it builds and test manually**

```bash
go build -o health ./cmd/health
./health add --help
```
Expected: Shows add command help

**Step 3: Commit**

```bash
git add cmd/health/add.go
git commit -m "feat: add CLI add command with blood pressure support"
```

---

## Task 10: CLI List Command

**Files:**
- Create: `cmd/health/list.go`

**Step 1: Create list.go**

Create `cmd/health/list.go`:
```go
// ABOUTME: CLI command for listing health metrics.
// ABOUTME: Supports filtering by type and limiting results.
package main

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/harperreed/health/internal/db"
	"github.com/harperreed/health/internal/models"
	"github.com/spf13/cobra"
)

var (
	listType  string
	listLimit int
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "l"},
	Short:   "List health metrics",
	Long: `List recent health metrics, optionally filtered by type.

Examples:
  health list
  health list --type weight
  health list --type mood --limit 20`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var metricType *models.MetricType
		if listType != "" {
			if !models.IsValidMetricType(listType) {
				return fmt.Errorf("unknown metric type: %s", listType)
			}
			mt := models.MetricType(listType)
			metricType = &mt
		}

		metrics, err := db.ListMetrics(dbConn, metricType, listLimit)
		if err != nil {
			return fmt.Errorf("failed to list metrics: %w", err)
		}

		if len(metrics) == 0 {
			fmt.Println("No metrics found.")
			return nil
		}

		faint := color.New(color.Faint)
		for _, m := range metrics {
			notes := ""
			if m.Notes != nil && *m.Notes != "" {
				notes = faint.Sprintf(" (%s)", truncate(*m.Notes, 30))
			}
			fmt.Printf("%s %s %s %.2f %s%s\n",
				faint.Sprint(m.ID.String()[:8]),
				faint.Sprint(m.RecordedAt.Format("2006-01-02 15:04")),
				padRight(string(m.MetricType), 16),
				m.Value,
				m.Unit,
				notes)
		}

		return nil
	},
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func padRight(s string, length int) string {
	if len(s) >= length {
		return s
	}
	return s + strings.Repeat(" ", length-len(s))
}

func init() {
	listCmd.Flags().StringVarP(&listType, "type", "t", "", "filter by metric type")
	listCmd.Flags().IntVarP(&listLimit, "limit", "n", 20, "max number of results")
	rootCmd.AddCommand(listCmd)
}
```

**Step 2: Verify it builds**

```bash
go build -o health ./cmd/health
./health list --help
```

**Step 3: Commit**

```bash
git add cmd/health/list.go
git commit -m "feat: add CLI list command"
```

---

## Task 11: CLI Workout Commands

**Files:**
- Create: `cmd/health/workout.go`

**Step 1: Create workout.go**

Create `cmd/health/workout.go`:
```go
// ABOUTME: CLI commands for managing workouts.
// ABOUTME: Supports add, list, show, and metric subcommands.
package main

import (
	"fmt"
	"strconv"

	"github.com/fatih/color"
	"github.com/harperreed/health/internal/db"
	"github.com/harperreed/health/internal/models"
	"github.com/spf13/cobra"
)

var (
	workoutDuration int
	workoutNotes    string
	workoutType     string
	workoutLimit    int
)

var workoutCmd = &cobra.Command{
	Use:     "workout",
	Aliases: []string{"w"},
	Short:   "Manage workouts",
}

var workoutAddCmd = &cobra.Command{
	Use:   "add <type>",
	Short: "Add a new workout",
	Long: `Add a new workout session.

Examples:
  health workout add run --duration 45
  health workout add lift --notes "Leg day"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workoutType := args[0]

		w := models.NewWorkout(workoutType)
		if workoutDuration > 0 {
			w.WithDuration(workoutDuration)
		}
		if workoutNotes != "" {
			w.WithNotes(workoutNotes)
		}

		if err := db.CreateWorkout(dbConn, w); err != nil {
			return fmt.Errorf("failed to create workout: %w", err)
		}

		color.Green("✓ Added %s workout", workoutType)
		fmt.Printf("  ID: %s\n", w.ID.String()[:8])
		if w.DurationMinutes != nil {
			fmt.Printf("  Duration: %d min\n", *w.DurationMinutes)
		}

		return nil
	},
}

var workoutListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List workouts",
	RunE: func(cmd *cobra.Command, args []string) error {
		var wType *string
		if workoutType != "" {
			wType = &workoutType
		}

		workouts, err := db.ListWorkouts(dbConn, wType, workoutLimit)
		if err != nil {
			return fmt.Errorf("failed to list workouts: %w", err)
		}

		if len(workouts) == 0 {
			fmt.Println("No workouts found.")
			return nil
		}

		faint := color.New(color.Faint)
		for _, w := range workouts {
			duration := ""
			if w.DurationMinutes != nil {
				duration = fmt.Sprintf("%d min", *w.DurationMinutes)
			}
			fmt.Printf("%s %s %s %s\n",
				faint.Sprint(w.ID.String()[:8]),
				faint.Sprint(w.StartedAt.Format("2006-01-02 15:04")),
				padRight(w.WorkoutType, 12),
				duration)
		}

		return nil
	},
}

var workoutShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show workout details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		w, err := db.GetWorkoutWithMetrics(dbConn, args[0])
		if err != nil {
			return fmt.Errorf("failed to get workout: %w", err)
		}

		fmt.Printf("Workout: %s\n", w.ID.String()[:8])
		fmt.Printf("Type: %s\n", w.WorkoutType)
		fmt.Printf("Started: %s\n", w.StartedAt.Format("2006-01-02 15:04"))
		if w.DurationMinutes != nil {
			fmt.Printf("Duration: %d min\n", *w.DurationMinutes)
		}
		if w.Notes != nil {
			fmt.Printf("Notes: %s\n", *w.Notes)
		}

		if len(w.Metrics) > 0 {
			fmt.Println("\nMetrics:")
			for _, m := range w.Metrics {
				unit := ""
				if m.Unit != nil {
					unit = *m.Unit
				}
				fmt.Printf("  %s: %.2f %s\n", m.MetricName, m.Value, unit)
			}
		}

		return nil
	},
}

var workoutMetricCmd = &cobra.Command{
	Use:   "metric <workout-id> <name> <value> [unit]",
	Short: "Add a metric to a workout",
	Long: `Add a metric to an existing workout.

Examples:
  health workout metric abc123 distance 5.2 km
  health workout metric abc123 avg_hr 145 bpm
  health workout metric abc123 sets 4`,
	Args: cobra.MinimumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		workoutID := args[0]
		metricName := args[1]
		value, err := strconv.ParseFloat(args[2], 64)
		if err != nil {
			return fmt.Errorf("invalid value: %s", args[2])
		}

		unit := ""
		if len(args) > 3 {
			unit = args[3]
		}

		// Verify workout exists
		w, err := db.GetWorkout(dbConn, workoutID)
		if err != nil {
			return fmt.Errorf("workout not found: %s", workoutID)
		}

		wm := models.NewWorkoutMetric(w.ID, metricName, value, unit)
		if err := db.AddWorkoutMetric(dbConn, wm); err != nil {
			return fmt.Errorf("failed to add workout metric: %w", err)
		}

		color.Green("✓ Added %s to workout", metricName)
		fmt.Printf("  %.2f %s\n", value, unit)

		return nil
	},
}

func init() {
	workoutAddCmd.Flags().IntVarP(&workoutDuration, "duration", "d", 0, "duration in minutes")
	workoutAddCmd.Flags().StringVarP(&workoutNotes, "notes", "n", "", "workout notes")

	workoutListCmd.Flags().StringVarP(&workoutType, "type", "t", "", "filter by workout type")
	workoutListCmd.Flags().IntVarP(&workoutLimit, "limit", "n", 20, "max number of results")

	workoutCmd.AddCommand(workoutAddCmd)
	workoutCmd.AddCommand(workoutListCmd)
	workoutCmd.AddCommand(workoutShowCmd)
	workoutCmd.AddCommand(workoutMetricCmd)
	rootCmd.AddCommand(workoutCmd)
}
```

**Step 2: Verify it builds**

```bash
go build -o health ./cmd/health
./health workout --help
./health workout add --help
```

**Step 3: Commit**

```bash
git add cmd/health/workout.go
git commit -m "feat: add CLI workout commands"
```

---

## Task 12: MCP Server Setup

**Files:**
- Create: `internal/mcp/server.go`
- Create: `cmd/health/mcp.go`

**Step 1: Create server.go**

Create `internal/mcp/server.go`:
```go
// ABOUTME: MCP server setup for health metrics store.
// ABOUTME: Wraps MCP server with database connection.
package mcp

import (
	"context"
	"database/sql"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP server with database access.
type Server struct {
	mcpServer *mcp.Server
	db        *sql.DB
}

// NewServer creates a new MCP server with the given database connection.
func NewServer(db *sql.DB) (*Server, error) {
	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    "health",
			Version: "1.0.0",
		},
		nil,
	)

	s := &Server{
		mcpServer: mcpServer,
		db:        db,
	}

	s.registerTools()
	s.registerResources()

	return s, nil
}

// Serve starts the MCP server using stdio transport.
func (s *Server) Serve(ctx context.Context) error {
	return s.mcpServer.Run(ctx, mcp.NewStdioTransport())
}

// registerTools registers all MCP tools.
func (s *Server) registerTools() {
	// Will be implemented in Task 13
}

// registerResources registers all MCP resources.
func (s *Server) registerResources() {
	// Will be implemented in Task 14
}
```

**Step 2: Create cmd/health/mcp.go**

Create `cmd/health/mcp.go`:
```go
// ABOUTME: CLI command for starting MCP server.
// ABOUTME: Runs stdio-based MCP server for Claude integration.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/harperreed/health/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server",
	Long:  `Start the Model Context Protocol server for integration with Claude and other MCP clients.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		server, err := mcp.NewServer(dbConn)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle shutdown signals
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			cancel()
		}()

		return server.Serve(ctx)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
```

**Step 3: Verify it builds**

```bash
go build -o health ./cmd/health
./health mcp --help
```

**Step 4: Commit**

```bash
git add internal/mcp/ cmd/health/mcp.go
git commit -m "feat: add MCP server skeleton"
```

---

## Task 13: MCP CRUD Tools

**Files:**
- Create: `internal/mcp/tools.go`

**Step 1: Create tools.go with CRUD operations**

Create `internal/mcp/tools.go`:
```go
// ABOUTME: MCP tool implementations for health metrics.
// ABOUTME: Provides CRUD operations for metrics and workouts.
package mcp

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/harperreed/health/internal/db"
	"github.com/harperreed/health/internal/models"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerTools() {
	// add_metric
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "add_metric",
		Description: "Record a health metric (weight, hrv, mood, etc.)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"metric_type": map[string]interface{}{
					"type":        "string",
					"description": "Type of metric (weight, body_fat, bp_sys, bp_dia, heart_rate, hrv, temperature, steps, sleep_hours, active_calories, water, calories, protein, carbs, fat, mood, energy, stress, anxiety, focus, meditation)",
				},
				"value": map[string]interface{}{
					"type":        "number",
					"description": "The metric value",
				},
				"recorded_at": map[string]interface{}{
					"type":        "string",
					"description": "Timestamp (ISO 8601), defaults to now",
				},
				"notes": map[string]interface{}{
					"type":        "string",
					"description": "Optional notes",
				},
			},
			"required": []string{"metric_type", "value"},
		},
	}, s.handleAddMetric)

	// list_metrics
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_metrics",
		Description: "List recent health metrics, optionally filtered by type",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"metric_type": map[string]interface{}{
					"type":        "string",
					"description": "Filter by metric type (optional)",
				},
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Max results (default 20)",
				},
			},
		},
	}, s.handleListMetrics)

	// delete_metric
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "delete_metric",
		Description: "Delete a metric by ID or ID prefix",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "string",
					"description": "Metric ID or prefix",
				},
			},
			"required": []string{"id"},
		},
	}, s.handleDeleteMetric)

	// add_workout
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "add_workout",
		Description: "Create a new workout session",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workout_type": map[string]interface{}{
					"type":        "string",
					"description": "Type of workout (run, lift, cycle, swim, etc.)",
				},
				"duration_minutes": map[string]interface{}{
					"type":        "number",
					"description": "Duration in minutes",
				},
				"notes": map[string]interface{}{
					"type":        "string",
					"description": "Workout notes",
				},
			},
			"required": []string{"workout_type"},
		},
	}, s.handleAddWorkout)

	// add_workout_metric
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "add_workout_metric",
		Description: "Add a metric to an existing workout",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workout_id": map[string]interface{}{
					"type":        "string",
					"description": "Workout ID or prefix",
				},
				"metric_name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the metric (distance, pace, sets, reps, etc.)",
				},
				"value": map[string]interface{}{
					"type":        "number",
					"description": "The value",
				},
				"unit": map[string]interface{}{
					"type":        "string",
					"description": "Unit of measurement",
				},
			},
			"required": []string{"workout_id", "metric_name", "value"},
		},
	}, s.handleAddWorkoutMetric)

	// list_workouts
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_workouts",
		Description: "List recent workouts, optionally filtered by type",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workout_type": map[string]interface{}{
					"type":        "string",
					"description": "Filter by workout type",
				},
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Max results (default 20)",
				},
			},
		},
	}, s.handleListWorkouts)

	// get_workout
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_workout",
		Description: "Get a workout with all its metrics",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "string",
					"description": "Workout ID or prefix",
				},
			},
			"required": []string{"id"},
		},
	}, s.handleGetWorkout)

	// delete_workout
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "delete_workout",
		Description: "Delete a workout and its metrics",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "string",
					"description": "Workout ID or prefix",
				},
			},
			"required": []string{"id"},
		},
	}, s.handleDeleteWorkout)

	// get_latest
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_latest",
		Description: "Get the most recent value for one or more metric types",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"metric_types": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "List of metric types to get latest values for",
				},
			},
		},
	}, s.handleGetLatest)
}

// Tool handlers

type addMetricInput struct {
	MetricType string  `json:"metric_type"`
	Value      float64 `json:"value"`
	RecordedAt string  `json:"recorded_at,omitempty"`
	Notes      string  `json:"notes,omitempty"`
}

func (s *Server) handleAddMetric(args json.RawMessage) (*mcp.CallToolResult, error) {
	var input addMetricInput
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if !models.IsValidMetricType(input.MetricType) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Unknown metric type: %s", input.MetricType)}},
			IsError: true,
		}, nil
	}

	m := models.NewMetric(models.MetricType(input.MetricType), input.Value)

	if input.RecordedAt != "" {
		t, err := time.Parse(time.RFC3339, input.RecordedAt)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04", input.RecordedAt)
		}
		if err == nil {
			m.WithRecordedAt(t)
		}
	}

	if input.Notes != "" {
		m.WithNotes(input.Notes)
	}

	if err := db.CreateMetric(s.db, m); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Failed to create metric: %v", err)}},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Added %s: %.2f %s (ID: %s)", input.MetricType, m.Value, m.Unit, m.ID.String()[:8])}},
	}, nil
}

type listMetricsInput struct {
	MetricType string `json:"metric_type,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

func (s *Server) handleListMetrics(args json.RawMessage) (*mcp.CallToolResult, error) {
	var input listMetricsInput
	json.Unmarshal(args, &input)

	if input.Limit <= 0 {
		input.Limit = 20
	}

	var metricType *models.MetricType
	if input.MetricType != "" {
		mt := models.MetricType(input.MetricType)
		metricType = &mt
	}

	metrics, err := db.ListMetrics(s.db, metricType, input.Limit)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Failed to list metrics: %v", err)}},
			IsError: true,
		}, nil
	}

	if len(metrics) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{{Type: "text", Text: "No metrics found."}},
		}, nil
	}

	result, _ := json.MarshalIndent(metrics, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{{Type: "text", Text: string(result)}},
	}, nil
}

type deleteMetricInput struct {
	ID string `json:"id"`
}

func (s *Server) handleDeleteMetric(args json.RawMessage) (*mcp.CallToolResult, error) {
	var input deleteMetricInput
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if err := db.DeleteMetric(s.db, input.ID); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Failed to delete metric: %v", err)}},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Deleted metric: %s", input.ID)}},
	}, nil
}

type addWorkoutInput struct {
	WorkoutType     string `json:"workout_type"`
	DurationMinutes int    `json:"duration_minutes,omitempty"`
	Notes           string `json:"notes,omitempty"`
}

func (s *Server) handleAddWorkout(args json.RawMessage) (*mcp.CallToolResult, error) {
	var input addWorkoutInput
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	w := models.NewWorkout(input.WorkoutType)
	if input.DurationMinutes > 0 {
		w.WithDuration(input.DurationMinutes)
	}
	if input.Notes != "" {
		w.WithNotes(input.Notes)
	}

	if err := db.CreateWorkout(s.db, w); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Failed to create workout: %v", err)}},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Added %s workout (ID: %s)", input.WorkoutType, w.ID.String()[:8])}},
	}, nil
}

type addWorkoutMetricInput struct {
	WorkoutID  string  `json:"workout_id"`
	MetricName string  `json:"metric_name"`
	Value      float64 `json:"value"`
	Unit       string  `json:"unit,omitempty"`
}

func (s *Server) handleAddWorkoutMetric(args json.RawMessage) (*mcp.CallToolResult, error) {
	var input addWorkoutMetricInput
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	w, err := db.GetWorkout(s.db, input.WorkoutID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Workout not found: %s", input.WorkoutID)}},
			IsError: true,
		}, nil
	}

	wm := models.NewWorkoutMetric(w.ID, input.MetricName, input.Value, input.Unit)
	if err := db.AddWorkoutMetric(s.db, wm); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Failed to add workout metric: %v", err)}},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Added %s: %.2f %s to workout", input.MetricName, input.Value, input.Unit)}},
	}, nil
}

type listWorkoutsInput struct {
	WorkoutType string `json:"workout_type,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

func (s *Server) handleListWorkouts(args json.RawMessage) (*mcp.CallToolResult, error) {
	var input listWorkoutsInput
	json.Unmarshal(args, &input)

	if input.Limit <= 0 {
		input.Limit = 20
	}

	var workoutType *string
	if input.WorkoutType != "" {
		workoutType = &input.WorkoutType
	}

	workouts, err := db.ListWorkouts(s.db, workoutType, input.Limit)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Failed to list workouts: %v", err)}},
			IsError: true,
		}, nil
	}

	if len(workouts) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{{Type: "text", Text: "No workouts found."}},
		}, nil
	}

	result, _ := json.MarshalIndent(workouts, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{{Type: "text", Text: string(result)}},
	}, nil
}

type getWorkoutInput struct {
	ID string `json:"id"`
}

func (s *Server) handleGetWorkout(args json.RawMessage) (*mcp.CallToolResult, error) {
	var input getWorkoutInput
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	w, err := db.GetWorkoutWithMetrics(s.db, input.ID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Workout not found: %s", input.ID)}},
			IsError: true,
		}, nil
	}

	result, _ := json.MarshalIndent(w, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{{Type: "text", Text: string(result)}},
	}, nil
}

func (s *Server) handleDeleteWorkout(args json.RawMessage) (*mcp.CallToolResult, error) {
	var input getWorkoutInput
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if err := db.DeleteWorkout(s.db, input.ID); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Failed to delete workout: %v", err)}},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Deleted workout: %s", input.ID)}},
	}, nil
}

type getLatestInput struct {
	MetricTypes []string `json:"metric_types,omitempty"`
}

func (s *Server) handleGetLatest(args json.RawMessage) (*mcp.CallToolResult, error) {
	var input getLatestInput
	json.Unmarshal(args, &input)

	// If no types specified, get all
	types := input.MetricTypes
	if len(types) == 0 {
		for _, mt := range models.AllMetricTypes {
			types = append(types, string(mt))
		}
	}

	results := make(map[string]interface{})
	for _, t := range types {
		mt := models.MetricType(t)
		metrics, err := db.ListMetrics(s.db, &mt, 1)
		if err == nil && len(metrics) > 0 {
			results[t] = map[string]interface{}{
				"value":       metrics[0].Value,
				"unit":        metrics[0].Unit,
				"recorded_at": metrics[0].RecordedAt,
			}
		}
	}

	result, _ := json.MarshalIndent(results, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{{Type: "text", Text: string(result)}},
	}, nil
}
```

**Step 2: Verify it builds**

```bash
go build -o health ./cmd/health
```

**Step 3: Commit**

```bash
git add internal/mcp/tools.go
git commit -m "feat: add MCP CRUD tools"
```

---

## Task 14: MCP Resources

**Files:**
- Create: `internal/mcp/resources.go`

**Step 1: Create resources.go**

Create `internal/mcp/resources.go`:
```go
// ABOUTME: MCP resource implementations for health metrics.
// ABOUTME: Provides read-only views of health data.
package mcp

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/harperreed/health/internal/db"
	"github.com/harperreed/health/internal/models"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerResources() {
	// health://recent
	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "health://recent",
		Name:        "Recent Metrics",
		Description: "Last 10 entries across all metrics",
		MimeType:    "application/json",
	}, s.handleRecentResource)

	// health://today
	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "health://today",
		Name:        "Today's Metrics",
		Description: "All metrics logged today",
		MimeType:    "application/json",
	}, s.handleTodayResource)

	// health://summary
	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "health://summary",
		Name:        "Health Summary",
		Description: "Dashboard with latest of each metric type",
		MimeType:    "application/json",
	}, s.handleSummaryResource)
}

func (s *Server) handleRecentResource(uri string) (string, error) {
	metrics, err := db.ListMetrics(s.db, nil, 10)
	if err != nil {
		return "", fmt.Errorf("failed to get recent metrics: %w", err)
	}

	result, _ := json.MarshalIndent(metrics, "", "  ")
	return string(result), nil
}

func (s *Server) handleTodayResource(uri string) (string, error) {
	// Get today's metrics by listing and filtering
	metrics, err := db.ListMetrics(s.db, nil, 100)
	if err != nil {
		return "", fmt.Errorf("failed to get metrics: %w", err)
	}

	today := time.Now().Truncate(24 * time.Hour)
	var todayMetrics []*models.Metric
	for _, m := range metrics {
		if m.RecordedAt.After(today) || m.RecordedAt.Equal(today) {
			todayMetrics = append(todayMetrics, m)
		}
	}

	result, _ := json.MarshalIndent(todayMetrics, "", "  ")
	return string(result), nil
}

func (s *Server) handleSummaryResource(uri string) (string, error) {
	summary := make(map[string]interface{})

	// Get latest of each metric type
	latest := make(map[string]interface{})
	for _, mt := range models.AllMetricTypes {
		metrics, err := db.ListMetrics(s.db, &mt, 1)
		if err == nil && len(metrics) > 0 {
			latest[string(mt)] = map[string]interface{}{
				"value":       metrics[0].Value,
				"unit":        metrics[0].Unit,
				"recorded_at": metrics[0].RecordedAt.Format(time.RFC3339),
			}
		}
	}
	summary["latest"] = latest

	// Get recent workouts
	workouts, _ := db.ListWorkouts(s.db, nil, 5)
	summary["recent_workouts"] = workouts

	result, _ := json.MarshalIndent(summary, "", "  ")
	return string(result), nil
}
```

**Step 2: Verify it builds**

```bash
go build -o health ./cmd/health
```

**Step 3: Commit**

```bash
git add internal/mcp/resources.go
git commit -m "feat: add MCP resources"
```

---

## Task 15: Integration Test

**Files:**
- Create: `test/integration_test.go`

**Step 1: Create integration test**

Create `test/integration_test.go`:
```go
// ABOUTME: Integration tests for health CLI.
// ABOUTME: Tests full workflow from CLI commands.
package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFullWorkflow(t *testing.T) {
	// Build the binary
	projectRoot, _ := filepath.Abs("..")
	healthBinary := filepath.Join(projectRoot, "health")

	buildCmd := exec.Command("go", "build", "-o", healthBinary, "./cmd/health")
	buildCmd.Dir = projectRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build: %v\n%s", err, output)
	}
	defer os.Remove(healthBinary)

	// Use temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	run := func(args ...string) (string, error) {
		fullArgs := append([]string{"--db", dbPath}, args...)
		cmd := exec.Command(healthBinary, fullArgs...)
		output, err := cmd.CombinedOutput()
		return string(output), err
	}

	// Test adding metrics
	output, err := run("add", "weight", "82.5")
	if err != nil {
		t.Fatalf("Failed to add weight: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Added weight") {
		t.Errorf("Expected 'Added weight' in output, got: %s", output)
	}

	// Test blood pressure
	output, err = run("add", "bp", "120", "80")
	if err != nil {
		t.Fatalf("Failed to add bp: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Added blood pressure") {
		t.Errorf("Expected 'Added blood pressure' in output, got: %s", output)
	}

	// Test listing
	output, err = run("list")
	if err != nil {
		t.Fatalf("Failed to list: %v\n%s", err, output)
	}
	if !strings.Contains(output, "weight") {
		t.Errorf("Expected 'weight' in list output, got: %s", output)
	}

	// Test workout add
	output, err = run("workout", "add", "run", "--duration", "45")
	if err != nil {
		t.Fatalf("Failed to add workout: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Added run workout") {
		t.Errorf("Expected 'Added run workout' in output, got: %s", output)
	}

	// Test workout list
	output, err = run("workout", "list")
	if err != nil {
		t.Fatalf("Failed to list workouts: %v\n%s", err, output)
	}
	if !strings.Contains(output, "run") {
		t.Errorf("Expected 'run' in workout list, got: %s", output)
	}
}
```

**Step 2: Run integration test**

```bash
go test ./test/... -v
```
Expected: PASS

**Step 3: Commit**

```bash
git add test/
git commit -m "test: add integration tests"
```

---

## Task 16: Makefile

**Files:**
- Create: `Makefile`

**Step 1: Create Makefile**

Create `Makefile`:
```makefile
.PHONY: build test test-race test-coverage install clean run-mcp lint

build:
	go build -o health ./cmd/health

test:
	go test ./internal/... -v
	go test ./test/... -v

test-race:
	go test -race ./...

test-coverage:
	go test -coverprofile=coverage.out -covermode=atomic ./internal/...
	go tool cover -html=coverage.out -o coverage.html

install:
	go install ./cmd/health

clean:
	rm -f health coverage.out coverage.html
	go clean

run-mcp: build
	./health mcp

lint:
	golangci-lint run

.DEFAULT_GOAL := build
```

**Step 2: Verify make works**

```bash
make build
make test
```

**Step 3: Commit**

```bash
git add Makefile
git commit -m "build: add Makefile"
```

---

## Task 17: Final Verification

**Step 1: Run all tests**

```bash
make test
```
Expected: All tests pass

**Step 2: Manual smoke test**

```bash
make build
./health add weight 82.5
./health add bp 120 80
./health add mood 7 --notes "Good day"
./health list
./health workout add run --duration 45
./health workout list
```

**Step 3: Force push to reset origin**

```bash
git push --force origin main
```

---

## Summary

17 tasks total:
1. Initialize Go module
2. Add dependencies
3. Metric model
4. Workout models
5. Database connection + schema
6. Metrics CRUD
7. Workouts CRUD
8. CLI root command
9. CLI add command
10. CLI list command
11. CLI workout commands
12. MCP server setup
13. MCP CRUD tools
14. MCP resources
15. Integration test
16. Makefile
17. Final verification

Each task is atomic, testable, and committable.
