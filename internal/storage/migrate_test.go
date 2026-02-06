// ABOUTME: Tests for data migration between storage backends.
// ABOUTME: Covers sqlite-to-markdown, markdown-to-sqlite, and round-trip migration.
package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/harperreed/health/internal/models"
)

func TestMigrateDataSQLiteToMarkdown(t *testing.T) {
	// Set up source (SQLite)
	srcDir, err := os.MkdirTemp("", "health-migrate-src-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(srcDir)

	srcDB, err := Open(filepath.Join(srcDir, "health.db"))
	if err != nil {
		t.Fatalf("Failed to open source DB: %v", err)
	}
	defer srcDB.Close()

	// Populate source with test data
	m1 := models.NewMetric(models.MetricWeight, 82.5)
	m1.WithNotes("morning weight")
	m2 := models.NewMetric(models.MetricMood, 7)
	srcDB.CreateMetric(m1)
	srcDB.CreateMetric(m2)

	w := models.NewWorkout("run")
	w.WithDuration(45)
	w.WithNotes("morning run")
	srcDB.CreateWorkout(w)

	wm := models.NewWorkoutMetric(w.ID, "distance", 5.2, "km")
	srcDB.AddWorkoutMetric(wm)

	// Set up destination (Markdown)
	dstDir, err := os.MkdirTemp("", "health-migrate-dst-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dstDir)

	dstStore, err := NewMarkdownStore(dstDir)
	if err != nil {
		t.Fatalf("Failed to create MarkdownStore: %v", err)
	}

	// Run migration
	summary, err := MigrateData(srcDB, dstStore)
	if err != nil {
		t.Fatalf("MigrateData failed: %v", err)
	}

	// Verify summary
	if summary.Metrics != 2 {
		t.Errorf("Expected 2 migrated metrics, got %d", summary.Metrics)
	}
	if summary.Workouts != 1 {
		t.Errorf("Expected 1 migrated workout, got %d", summary.Workouts)
	}
	if summary.WorkoutMetrics != 1 {
		t.Errorf("Expected 1 migrated workout metric, got %d", summary.WorkoutMetrics)
	}

	// Verify data in destination
	metrics, err := dstStore.ListMetrics(nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics from dst failed: %v", err)
	}
	if len(metrics) != 2 {
		t.Errorf("Expected 2 metrics in dst, got %d", len(metrics))
	}

	workouts, err := dstStore.ListWorkouts(nil, 0)
	if err != nil {
		t.Fatalf("ListWorkouts from dst failed: %v", err)
	}
	if len(workouts) != 1 {
		t.Errorf("Expected 1 workout in dst, got %d", len(workouts))
	}

	// Verify workout has metrics
	dstWorkout, err := dstStore.GetWorkoutWithMetrics(w.ID.String())
	if err != nil {
		t.Fatalf("GetWorkoutWithMetrics from dst failed: %v", err)
	}
	if len(dstWorkout.Metrics) != 1 {
		t.Errorf("Expected 1 workout metric in dst, got %d", len(dstWorkout.Metrics))
	}
}

func TestMigrateDataMarkdownToSQLite(t *testing.T) {
	// Set up source (Markdown)
	srcDir, err := os.MkdirTemp("", "health-migrate-src-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(srcDir)

	srcStore, err := NewMarkdownStore(srcDir)
	if err != nil {
		t.Fatalf("Failed to create MarkdownStore: %v", err)
	}

	// Populate source
	m1 := models.NewMetric(models.MetricWeight, 82.5)
	m1.WithNotes("morning weight")
	m2 := models.NewMetric(models.MetricMood, 7)
	srcStore.CreateMetric(m1)
	srcStore.CreateMetric(m2)

	w := models.NewWorkout("run")
	w.WithDuration(45)
	w.WithNotes("morning run")
	srcStore.CreateWorkout(w)

	wm := models.NewWorkoutMetric(w.ID, "distance", 5.2, "km")
	srcStore.AddWorkoutMetric(wm)

	// Set up destination (SQLite)
	dstDir, err := os.MkdirTemp("", "health-migrate-dst-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dstDir)

	dstDB, err := Open(filepath.Join(dstDir, "health.db"))
	if err != nil {
		t.Fatalf("Failed to open destination DB: %v", err)
	}
	defer dstDB.Close()

	// Run migration
	summary, err := MigrateData(srcStore, dstDB)
	if err != nil {
		t.Fatalf("MigrateData failed: %v", err)
	}

	// Verify summary
	if summary.Metrics != 2 {
		t.Errorf("Expected 2 migrated metrics, got %d", summary.Metrics)
	}
	if summary.Workouts != 1 {
		t.Errorf("Expected 1 migrated workout, got %d", summary.Workouts)
	}
	if summary.WorkoutMetrics != 1 {
		t.Errorf("Expected 1 migrated workout metric, got %d", summary.WorkoutMetrics)
	}

	// Verify data in destination
	metrics, err := dstDB.ListMetrics(nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics from dst failed: %v", err)
	}
	if len(metrics) != 2 {
		t.Errorf("Expected 2 metrics in dst, got %d", len(metrics))
	}
}

func TestMigrateDataEmptySource(t *testing.T) {
	// Set up empty source (SQLite)
	srcDir, err := os.MkdirTemp("", "health-migrate-src-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(srcDir)

	srcDB, err := Open(filepath.Join(srcDir, "health.db"))
	if err != nil {
		t.Fatalf("Failed to open source DB: %v", err)
	}
	defer srcDB.Close()

	// Set up destination (Markdown)
	dstDir, err := os.MkdirTemp("", "health-migrate-dst-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dstDir)

	dstStore, err := NewMarkdownStore(dstDir)
	if err != nil {
		t.Fatalf("Failed to create MarkdownStore: %v", err)
	}

	// Run migration with no data
	summary, err := MigrateData(srcDB, dstStore)
	if err != nil {
		t.Fatalf("MigrateData with empty source failed: %v", err)
	}

	if summary.Metrics != 0 {
		t.Errorf("Expected 0 migrated metrics, got %d", summary.Metrics)
	}
	if summary.Workouts != 0 {
		t.Errorf("Expected 0 migrated workouts, got %d", summary.Workouts)
	}
	if summary.WorkoutMetrics != 0 {
		t.Errorf("Expected 0 migrated workout metrics, got %d", summary.WorkoutMetrics)
	}
}

func TestMigrateDataRoundTrip(t *testing.T) {
	// Create data in SQLite, migrate to Markdown, migrate back to SQLite
	// and verify all data is preserved.

	// Step 1: Populate SQLite source
	srcDir, err := os.MkdirTemp("", "health-migrate-rt-src-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(srcDir)

	srcDB, err := Open(filepath.Join(srcDir, "health.db"))
	if err != nil {
		t.Fatalf("Failed to open source DB: %v", err)
	}
	defer srcDB.Close()

	m1 := models.NewMetric(models.MetricWeight, 82.5)
	m1.WithNotes("test note")
	m1.RecordedAt = time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	srcDB.CreateMetric(m1)

	w := models.NewWorkout("run")
	w.WithDuration(30)
	w.WithNotes("test run")
	srcDB.CreateWorkout(w)

	wm := models.NewWorkoutMetric(w.ID, "distance", 5.0, "km")
	srcDB.AddWorkoutMetric(wm)

	// Step 2: Migrate SQLite -> Markdown
	mdDir, err := os.MkdirTemp("", "health-migrate-rt-md-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(mdDir)

	mdStore, err := NewMarkdownStore(mdDir)
	if err != nil {
		t.Fatalf("Failed to create MarkdownStore: %v", err)
	}

	_, err = MigrateData(srcDB, mdStore)
	if err != nil {
		t.Fatalf("MigrateData (sqlite->md) failed: %v", err)
	}

	// Step 3: Migrate Markdown -> new SQLite
	dstDir, err := os.MkdirTemp("", "health-migrate-rt-dst-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dstDir)

	dstDB, err := Open(filepath.Join(dstDir, "health.db"))
	if err != nil {
		t.Fatalf("Failed to open destination DB: %v", err)
	}
	defer dstDB.Close()

	_, err = MigrateData(mdStore, dstDB)
	if err != nil {
		t.Fatalf("MigrateData (md->sqlite) failed: %v", err)
	}

	// Step 4: Verify data round-tripped correctly
	gotMetric, err := dstDB.GetMetric(m1.ID.String())
	if err != nil {
		t.Fatalf("GetMetric after round-trip failed: %v", err)
	}
	if gotMetric.Value != 82.5 {
		t.Errorf("Metric value mismatch after round-trip: got %v, want 82.5", gotMetric.Value)
	}
	if gotMetric.Notes == nil || *gotMetric.Notes != "test note" {
		t.Error("Metric notes lost in round-trip")
	}

	gotWorkout, err := dstDB.GetWorkoutWithMetrics(w.ID.String())
	if err != nil {
		t.Fatalf("GetWorkoutWithMetrics after round-trip failed: %v", err)
	}
	if gotWorkout.WorkoutType != "run" {
		t.Errorf("Workout type mismatch: got %v, want 'run'", gotWorkout.WorkoutType)
	}
	if gotWorkout.DurationMinutes == nil || *gotWorkout.DurationMinutes != 30 {
		t.Error("Workout duration lost in round-trip")
	}
	if gotWorkout.Notes == nil || *gotWorkout.Notes != "test run" {
		t.Error("Workout notes lost in round-trip")
	}
	if len(gotWorkout.Metrics) != 1 {
		t.Errorf("Expected 1 workout metric after round-trip, got %d", len(gotWorkout.Metrics))
	}
}

func TestIsDirNonEmpty(t *testing.T) {
	// Empty directory
	emptyDir, err := os.MkdirTemp("", "health-empty-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(emptyDir)

	nonEmpty, err := IsDirNonEmpty(emptyDir)
	if err != nil {
		t.Fatalf("IsDirNonEmpty failed: %v", err)
	}
	if nonEmpty {
		t.Error("Expected empty directory to return false")
	}

	// Non-empty directory
	if err := os.WriteFile(filepath.Join(emptyDir, "test.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	nonEmpty, err = IsDirNonEmpty(emptyDir)
	if err != nil {
		t.Fatalf("IsDirNonEmpty failed: %v", err)
	}
	if !nonEmpty {
		t.Error("Expected non-empty directory to return true")
	}

	// Non-existent directory
	nonEmpty, err = IsDirNonEmpty("/nonexistent/path")
	if err != nil {
		t.Fatalf("IsDirNonEmpty for nonexistent should not error: %v", err)
	}
	if nonEmpty {
		t.Error("Expected non-existent directory to return false")
	}
}

func TestMigrateDataPreservesMetricDetails(t *testing.T) {
	// Verify that specific field values are preserved during migration
	srcDir, err := os.MkdirTemp("", "health-migrate-detail-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(srcDir)

	srcDB, err := Open(filepath.Join(srcDir, "health.db"))
	if err != nil {
		t.Fatalf("Failed to open source DB: %v", err)
	}
	defer srcDB.Close()

	// Create diverse metrics
	metrics := []*models.Metric{
		func() *models.Metric {
			m := models.NewMetric(models.MetricWeight, 82.5)
			m.WithNotes("morning")
			return m
		}(),
		func() *models.Metric {
			m := models.NewMetric(models.MetricBPSys, 120)
			return m
		}(),
		func() *models.Metric {
			m := models.NewMetric(models.MetricBPDia, 80)
			return m
		}(),
		func() *models.Metric {
			m := models.NewMetric(models.MetricMood, 7)
			m.WithNotes("great day!")
			return m
		}(),
	}

	for _, m := range metrics {
		srcDB.CreateMetric(m)
	}

	dstDir, err := os.MkdirTemp("", "health-migrate-detail-dst-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dstDir)

	dstStore, err := NewMarkdownStore(dstDir)
	if err != nil {
		t.Fatalf("Failed to create MarkdownStore: %v", err)
	}

	summary, err := MigrateData(srcDB, dstStore)
	if err != nil {
		t.Fatalf("MigrateData failed: %v", err)
	}

	if summary.Metrics != 4 {
		t.Errorf("Expected 4 migrated metrics, got %d", summary.Metrics)
	}

	// Verify each metric was preserved
	for _, original := range metrics {
		got, err := dstStore.GetMetric(original.ID.String())
		if err != nil {
			t.Fatalf("GetMetric(%s) from dst failed: %v", original.ID, err)
		}
		if got.MetricType != original.MetricType {
			t.Errorf("MetricType mismatch for %s: got %v, want %v", original.ID, got.MetricType, original.MetricType)
		}
		if got.Value != original.Value {
			t.Errorf("Value mismatch for %s: got %v, want %v", original.ID, got.Value, original.Value)
		}
	}
}
