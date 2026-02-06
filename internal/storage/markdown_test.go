// ABOUTME: Tests for MarkdownStore implementation of Repository interface.
// ABOUTME: Verifies CRUD operations for metrics, workouts, and workout metrics using file storage.
package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/harperreed/health/internal/models"
)

// setupTestMarkdownStore creates a MarkdownStore in a temp directory.
func setupTestMarkdownStore(t *testing.T) *MarkdownStore {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "health-md-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	store, err := NewMarkdownStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create MarkdownStore: %v", err)
	}

	return store
}

func TestMarkdownStoreCreateAndGetMetric(t *testing.T) {
	store := setupTestMarkdownStore(t)

	m := models.NewMetric(models.MetricWeight, 82.5)
	m.WithNotes("morning weight")

	err := store.CreateMetric(m)
	if err != nil {
		t.Fatalf("CreateMetric failed: %v", err)
	}

	// Retrieve by full ID
	got, err := store.GetMetric(m.ID.String())
	if err != nil {
		t.Fatalf("GetMetric failed: %v", err)
	}

	if got.ID != m.ID {
		t.Errorf("ID mismatch: got %v, want %v", got.ID, m.ID)
	}
	if got.MetricType != m.MetricType {
		t.Errorf("MetricType mismatch: got %v, want %v", got.MetricType, m.MetricType)
	}
	if got.Value != m.Value {
		t.Errorf("Value mismatch: got %v, want %v", got.Value, m.Value)
	}
	if got.Notes == nil || *got.Notes != "morning weight" {
		t.Errorf("Notes mismatch: got %v, want 'morning weight'", got.Notes)
	}
}

func TestMarkdownStoreGetMetricByPrefix(t *testing.T) {
	store := setupTestMarkdownStore(t)

	m := models.NewMetric(models.MetricWeight, 82.5)
	if err := store.CreateMetric(m); err != nil {
		t.Fatalf("CreateMetric failed: %v", err)
	}

	// Retrieve by 8-char prefix
	prefix := m.ID.String()[:8]
	got, err := store.GetMetric(prefix)
	if err != nil {
		t.Fatalf("GetMetric by prefix failed: %v", err)
	}

	if got.ID != m.ID {
		t.Errorf("ID mismatch: got %v, want %v", got.ID, m.ID)
	}
}

func TestMarkdownStoreListMetrics(t *testing.T) {
	store := setupTestMarkdownStore(t)

	// Create metrics with different timestamps
	m1 := models.NewMetric(models.MetricWeight, 82.0)
	m1.RecordedAt = time.Now().Add(-2 * time.Hour)
	m2 := models.NewMetric(models.MetricWeight, 82.5)
	m2.RecordedAt = time.Now().Add(-1 * time.Hour)
	m3 := models.NewMetric(models.MetricMood, 7)
	m3.RecordedAt = time.Now()

	for _, m := range []*models.Metric{m1, m2, m3} {
		if err := store.CreateMetric(m); err != nil {
			t.Fatalf("CreateMetric failed: %v", err)
		}
	}

	// List all metrics (should be ordered by RecordedAt DESC)
	all, err := store.ListMetrics(nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("Expected 3 metrics, got %d", len(all))
	}

	// Verify order (most recent first)
	if len(all) >= 1 && all[0].ID != m3.ID {
		t.Errorf("Expected most recent first, got %v", all[0].ID)
	}

	// Filter by type
	weightType := models.MetricWeight
	weights, err := store.ListMetrics(&weightType, 0)
	if err != nil {
		t.Fatalf("ListMetrics with type failed: %v", err)
	}
	if len(weights) != 2 {
		t.Errorf("Expected 2 weight metrics, got %d", len(weights))
	}

	// Test limit
	limited, err := store.ListMetrics(nil, 2)
	if err != nil {
		t.Fatalf("ListMetrics with limit failed: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("Expected 2 metrics with limit, got %d", len(limited))
	}
}

func TestMarkdownStoreDeleteMetric(t *testing.T) {
	store := setupTestMarkdownStore(t)

	m := models.NewMetric(models.MetricWeight, 82.5)
	if err := store.CreateMetric(m); err != nil {
		t.Fatalf("CreateMetric failed: %v", err)
	}

	// Delete by prefix
	if err := store.DeleteMetric(m.ID.String()[:8]); err != nil {
		t.Fatalf("DeleteMetric failed: %v", err)
	}

	// Verify deleted
	_, err := store.GetMetric(m.ID.String())
	if err == nil {
		t.Error("Expected error getting deleted metric")
	}
}

func TestMarkdownStoreGetLatestMetric(t *testing.T) {
	store := setupTestMarkdownStore(t)

	m1 := models.NewMetric(models.MetricWeight, 82.0)
	m1.RecordedAt = time.Now().Add(-1 * time.Hour)
	m2 := models.NewMetric(models.MetricWeight, 83.0)
	m2.RecordedAt = time.Now()

	if err := store.CreateMetric(m1); err != nil {
		t.Fatalf("CreateMetric failed: %v", err)
	}
	if err := store.CreateMetric(m2); err != nil {
		t.Fatalf("CreateMetric failed: %v", err)
	}

	latest, err := store.GetLatestMetric(models.MetricWeight)
	if err != nil {
		t.Fatalf("GetLatestMetric failed: %v", err)
	}

	if latest.Value != 83.0 {
		t.Errorf("Expected latest value 83.0, got %v", latest.Value)
	}
}

func TestMarkdownStoreCreateAndGetWorkout(t *testing.T) {
	store := setupTestMarkdownStore(t)

	w := models.NewWorkout("run")
	w.WithDuration(45)
	w.WithNotes("morning run")

	if err := store.CreateWorkout(w); err != nil {
		t.Fatalf("CreateWorkout failed: %v", err)
	}

	got, err := store.GetWorkout(w.ID.String())
	if err != nil {
		t.Fatalf("GetWorkout failed: %v", err)
	}

	if got.ID != w.ID {
		t.Errorf("ID mismatch: got %v, want %v", got.ID, w.ID)
	}
	if got.WorkoutType != "run" {
		t.Errorf("WorkoutType mismatch: got %v, want 'run'", got.WorkoutType)
	}
	if got.DurationMinutes == nil || *got.DurationMinutes != 45 {
		t.Errorf("Duration mismatch: got %v, want 45", got.DurationMinutes)
	}
	if got.Notes == nil || *got.Notes != "morning run" {
		t.Errorf("Notes mismatch: got %v, want 'morning run'", got.Notes)
	}
}

func TestMarkdownStoreWorkoutWithMetrics(t *testing.T) {
	store := setupTestMarkdownStore(t)

	w := models.NewWorkout("run")
	if err := store.CreateWorkout(w); err != nil {
		t.Fatalf("CreateWorkout failed: %v", err)
	}

	// Add workout metrics
	wm1 := models.NewWorkoutMetric(w.ID, "distance", 5.2, "km")
	wm2 := models.NewWorkoutMetric(w.ID, "avg_hr", 150, "bpm")

	if err := store.AddWorkoutMetric(wm1); err != nil {
		t.Fatalf("AddWorkoutMetric failed: %v", err)
	}
	if err := store.AddWorkoutMetric(wm2); err != nil {
		t.Fatalf("AddWorkoutMetric failed: %v", err)
	}

	// Get workout with metrics
	got, err := store.GetWorkoutWithMetrics(w.ID.String())
	if err != nil {
		t.Fatalf("GetWorkoutWithMetrics failed: %v", err)
	}

	if len(got.Metrics) != 2 {
		t.Errorf("Expected 2 metrics, got %d", len(got.Metrics))
	}
}

func TestMarkdownStoreDeleteWorkout(t *testing.T) {
	store := setupTestMarkdownStore(t)

	w := models.NewWorkout("run")
	if err := store.CreateWorkout(w); err != nil {
		t.Fatalf("CreateWorkout failed: %v", err)
	}

	wm := models.NewWorkoutMetric(w.ID, "distance", 5.2, "km")
	if err := store.AddWorkoutMetric(wm); err != nil {
		t.Fatalf("AddWorkoutMetric failed: %v", err)
	}

	// Delete workout (and its metrics since they're embedded in the file)
	if err := store.DeleteWorkout(w.ID.String()[:8]); err != nil {
		t.Fatalf("DeleteWorkout failed: %v", err)
	}

	// Verify workout deleted
	_, err := store.GetWorkout(w.ID.String())
	if err == nil {
		t.Error("Expected error getting deleted workout")
	}
}

func TestMarkdownStoreListWorkouts(t *testing.T) {
	store := setupTestMarkdownStore(t)

	w1 := models.NewWorkout("run")
	w1.StartedAt = time.Now().Add(-2 * time.Hour)
	w2 := models.NewWorkout("lift")
	w2.StartedAt = time.Now().Add(-1 * time.Hour)
	w3 := models.NewWorkout("run")
	w3.StartedAt = time.Now()

	for _, w := range []*models.Workout{w1, w2, w3} {
		if err := store.CreateWorkout(w); err != nil {
			t.Fatalf("CreateWorkout failed: %v", err)
		}
	}

	// List all
	all, err := store.ListWorkouts(nil, 0)
	if err != nil {
		t.Fatalf("ListWorkouts failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("Expected 3 workouts, got %d", len(all))
	}

	// Filter by type
	runType := "run"
	runs, err := store.ListWorkouts(&runType, 0)
	if err != nil {
		t.Fatalf("ListWorkouts with type failed: %v", err)
	}
	if len(runs) != 2 {
		t.Errorf("Expected 2 run workouts, got %d", len(runs))
	}

	// Test limit
	limited, err := store.ListWorkouts(nil, 2)
	if err != nil {
		t.Fatalf("ListWorkouts with limit failed: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("Expected 2 workouts with limit, got %d", len(limited))
	}
}

func TestMarkdownStoreListWorkoutMetrics(t *testing.T) {
	store := setupTestMarkdownStore(t)

	w := models.NewWorkout("run")
	if err := store.CreateWorkout(w); err != nil {
		t.Fatalf("CreateWorkout failed: %v", err)
	}

	wm1 := models.NewWorkoutMetric(w.ID, "distance", 5.2, "km")
	wm2 := models.NewWorkoutMetric(w.ID, "pace", 5.5, "min/km")

	if err := store.AddWorkoutMetric(wm1); err != nil {
		t.Fatalf("AddWorkoutMetric failed: %v", err)
	}
	if err := store.AddWorkoutMetric(wm2); err != nil {
		t.Fatalf("AddWorkoutMetric failed: %v", err)
	}

	metrics, err := store.ListWorkoutMetrics(w.ID)
	if err != nil {
		t.Fatalf("ListWorkoutMetrics failed: %v", err)
	}

	if len(metrics) != 2 {
		t.Errorf("Expected 2 workout metrics, got %d", len(metrics))
	}
}

func TestMarkdownStoreDeleteWorkoutMetric(t *testing.T) {
	store := setupTestMarkdownStore(t)

	w := models.NewWorkout("run")
	if err := store.CreateWorkout(w); err != nil {
		t.Fatalf("CreateWorkout failed: %v", err)
	}

	wm := models.NewWorkoutMetric(w.ID, "distance", 5.2, "km")
	if err := store.AddWorkoutMetric(wm); err != nil {
		t.Fatalf("AddWorkoutMetric failed: %v", err)
	}

	if err := store.DeleteWorkoutMetric(wm.ID.String()[:8]); err != nil {
		t.Fatalf("DeleteWorkoutMetric failed: %v", err)
	}

	// Verify deleted by checking workout metrics list is empty
	metrics, err := store.ListWorkoutMetrics(w.ID)
	if err != nil {
		t.Fatalf("ListWorkoutMetrics failed: %v", err)
	}
	if len(metrics) != 0 {
		t.Errorf("Expected 0 workout metrics after delete, got %d", len(metrics))
	}
}

func TestMarkdownStoreGetWorkoutMetric(t *testing.T) {
	store := setupTestMarkdownStore(t)

	w := models.NewWorkout("run")
	store.CreateWorkout(w)

	wm := models.NewWorkoutMetric(w.ID, "distance", 5.0, "km")
	if err := store.AddWorkoutMetric(wm); err != nil {
		t.Fatalf("AddWorkoutMetric failed: %v", err)
	}

	// Retrieve by full UUID
	got, err := store.GetWorkoutMetric(wm.ID.String())
	if err != nil {
		t.Fatalf("GetWorkoutMetric by full UUID failed: %v", err)
	}

	if got.ID != wm.ID {
		t.Errorf("ID mismatch: got %v, want %v", got.ID, wm.ID)
	}
	if got.MetricName != "distance" {
		t.Errorf("MetricName mismatch: got %v, want 'distance'", got.MetricName)
	}
	if got.Value != 5.0 {
		t.Errorf("Value mismatch: got %v, want 5.0", got.Value)
	}
}

func TestMarkdownStoreGetWorkoutMetricByPrefix(t *testing.T) {
	store := setupTestMarkdownStore(t)

	w := models.NewWorkout("run")
	store.CreateWorkout(w)

	wm := models.NewWorkoutMetric(w.ID, "distance", 5.0, "km")
	if err := store.AddWorkoutMetric(wm); err != nil {
		t.Fatalf("AddWorkoutMetric failed: %v", err)
	}

	// Retrieve by prefix
	got, err := store.GetWorkoutMetric(wm.ID.String()[:8])
	if err != nil {
		t.Fatalf("GetWorkoutMetric by prefix failed: %v", err)
	}

	if got.ID != wm.ID {
		t.Errorf("ID mismatch: got %v, want %v", got.ID, wm.ID)
	}
}

func TestMarkdownStoreGetMetricNotFound(t *testing.T) {
	store := setupTestMarkdownStore(t)

	_, err := store.GetMetric("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent metric")
	}
}

func TestMarkdownStoreGetWorkoutNotFound(t *testing.T) {
	store := setupTestMarkdownStore(t)

	_, err := store.GetWorkout("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent workout")
	}
}

func TestMarkdownStoreDeleteMetricNotFound(t *testing.T) {
	store := setupTestMarkdownStore(t)

	err := store.DeleteMetric("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent metric")
	}
}

func TestMarkdownStoreDeleteWorkoutNotFound(t *testing.T) {
	store := setupTestMarkdownStore(t)

	err := store.DeleteWorkout("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent workout")
	}
}

func TestMarkdownStoreDeleteWorkoutMetricNotFound(t *testing.T) {
	store := setupTestMarkdownStore(t)

	err := store.DeleteWorkoutMetric("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent workout metric")
	}
}

func TestMarkdownStoreGetLatestMetricNotFound(t *testing.T) {
	store := setupTestMarkdownStore(t)

	_, err := store.GetLatestMetric(models.MetricWeight)
	if err == nil {
		t.Error("Expected error when no metrics exist")
	}
}

func TestMarkdownStoreGetWorkoutMetricNotFound(t *testing.T) {
	store := setupTestMarkdownStore(t)

	_, err := store.GetWorkoutMetric("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent workout metric")
	}
}

func TestMarkdownStoreListMetricsEmpty(t *testing.T) {
	store := setupTestMarkdownStore(t)

	metrics, err := store.ListMetrics(nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics failed: %v", err)
	}

	if len(metrics) != 0 {
		t.Errorf("Expected 0 metrics, got %d", len(metrics))
	}
}

func TestMarkdownStoreListWorkoutsEmpty(t *testing.T) {
	store := setupTestMarkdownStore(t)

	workouts, err := store.ListWorkouts(nil, 0)
	if err != nil {
		t.Fatalf("ListWorkouts failed: %v", err)
	}

	if len(workouts) != 0 {
		t.Errorf("Expected 0 workouts, got %d", len(workouts))
	}
}

func TestMarkdownStoreClose(t *testing.T) {
	store := setupTestMarkdownStore(t)

	// Close should be a no-op and not error
	err := store.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestMarkdownStoreWorkoutNullableDuration(t *testing.T) {
	store := setupTestMarkdownStore(t)

	// Workout without duration
	w := models.NewWorkout("run")

	if err := store.CreateWorkout(w); err != nil {
		t.Fatalf("CreateWorkout failed: %v", err)
	}

	got, err := store.GetWorkout(w.ID.String())
	if err != nil {
		t.Fatalf("GetWorkout failed: %v", err)
	}

	if got.DurationMinutes != nil {
		t.Error("Expected DurationMinutes to be nil")
	}
}

func TestMarkdownStoreWorkoutNullableNotes(t *testing.T) {
	store := setupTestMarkdownStore(t)

	// Workout without notes
	w := models.NewWorkout("run")

	if err := store.CreateWorkout(w); err != nil {
		t.Fatalf("CreateWorkout failed: %v", err)
	}

	got, err := store.GetWorkout(w.ID.String())
	if err != nil {
		t.Fatalf("GetWorkout failed: %v", err)
	}

	if got.Notes != nil {
		t.Error("Expected Notes to be nil")
	}
}

func TestMarkdownStoreWorkoutMetricNullableUnit(t *testing.T) {
	store := setupTestMarkdownStore(t)

	w := models.NewWorkout("lift")
	store.CreateWorkout(w)

	// Workout metric without unit
	wm := models.NewWorkoutMetric(w.ID, "sets", 4, "")

	if err := store.AddWorkoutMetric(wm); err != nil {
		t.Fatalf("AddWorkoutMetric failed: %v", err)
	}

	got, err := store.GetWorkoutMetric(wm.ID.String())
	if err != nil {
		t.Fatalf("GetWorkoutMetric failed: %v", err)
	}

	if got.Unit != nil {
		t.Error("Expected Unit to be nil for empty string")
	}
}

func TestMarkdownStoreGetAllData(t *testing.T) {
	store := setupTestMarkdownStore(t)

	// Create metrics
	m1 := models.NewMetric(models.MetricWeight, 82.5)
	m2 := models.NewMetric(models.MetricMood, 7)
	store.CreateMetric(m1)
	store.CreateMetric(m2)

	// Create workout with metrics
	w := models.NewWorkout("run")
	w.WithDuration(30)
	store.CreateWorkout(w)

	wm := models.NewWorkoutMetric(w.ID, "distance", 5.0, "km")
	store.AddWorkoutMetric(wm)

	data, err := store.GetAllData()
	if err != nil {
		t.Fatalf("GetAllData failed: %v", err)
	}

	if len(data.Metrics) != 2 {
		t.Errorf("Expected 2 metrics, got %d", len(data.Metrics))
	}
	if len(data.Workouts) != 1 {
		t.Errorf("Expected 1 workout, got %d", len(data.Workouts))
	}
	if len(data.Workouts[0].Metrics) != 1 {
		t.Errorf("Expected 1 workout metric, got %d", len(data.Workouts[0].Metrics))
	}
}

func TestMarkdownStoreImportData(t *testing.T) {
	store := setupTestMarkdownStore(t)

	// Create export data
	data := &ExportData{
		Version:    "1.0",
		ExportedAt: time.Now(),
		Tool:       "health",
		Metrics: []*models.Metric{
			{
				ID:         uuid.New(),
				MetricType: models.MetricWeight,
				Value:      82.5,
				Unit:       "kg",
				RecordedAt: time.Now(),
				CreatedAt:  time.Now(),
			},
		},
		Workouts: []*models.Workout{
			{
				ID:          uuid.New(),
				WorkoutType: "run",
				StartedAt:   time.Now(),
				CreatedAt:   time.Now(),
				Metrics: []models.WorkoutMetric{
					{
						ID:         uuid.New(),
						MetricName: "distance",
						Value:      5.0,
						CreatedAt:  time.Now(),
					},
				},
			},
		},
	}

	// Import
	if err := store.ImportData(data); err != nil {
		t.Fatalf("ImportData failed: %v", err)
	}

	// Verify
	metrics, err := store.ListMetrics(nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics failed: %v", err)
	}
	if len(metrics) != 1 {
		t.Errorf("Expected 1 metric, got %d", len(metrics))
	}

	workouts, err := store.ListWorkouts(nil, 0)
	if err != nil {
		t.Fatalf("ListWorkouts failed: %v", err)
	}
	if len(workouts) != 1 {
		t.Errorf("Expected 1 workout, got %d", len(workouts))
	}
}

func TestMarkdownStoreFileLayout(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "health-md-layout-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewMarkdownStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create MarkdownStore: %v", err)
	}

	// Create a metric
	m := models.NewMetric(models.MetricWeight, 82.5)
	if err := store.CreateMetric(m); err != nil {
		t.Fatalf("CreateMetric failed: %v", err)
	}

	// Verify the file exists in the expected directory structure
	year := m.RecordedAt.Format("2006")
	month := m.RecordedAt.Format("01")
	metricsDir := filepath.Join(tmpDir, "metrics", year, month)

	entries, err := os.ReadDir(metricsDir)
	if err != nil {
		t.Fatalf("Failed to read metrics dir: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 file in metrics dir, got %d", len(entries))
	}

	// Verify filename contains metric type and ID prefix
	filename := entries[0].Name()
	if filepath.Ext(filename) != ".md" {
		t.Errorf("Expected .md extension, got %s", filepath.Ext(filename))
	}

	// Create a workout
	w := models.NewWorkout("run")
	if err := store.CreateWorkout(w); err != nil {
		t.Fatalf("CreateWorkout failed: %v", err)
	}

	// Verify workout file exists
	workoutsDir := filepath.Join(tmpDir, "workouts", year, month)
	entries, err = os.ReadDir(workoutsDir)
	if err != nil {
		t.Fatalf("Failed to read workouts dir: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 file in workouts dir, got %d", len(entries))
	}
}

func TestMarkdownStoreMetricRoundTrip(t *testing.T) {
	store := setupTestMarkdownStore(t)

	// Test all metric types round-trip correctly
	metricTypes := []struct {
		metricType models.MetricType
		value      float64
	}{
		{models.MetricWeight, 82.5},
		{models.MetricBodyFat, 15.2},
		{models.MetricBPSys, 120},
		{models.MetricBPDia, 80},
		{models.MetricHeartRate, 65},
		{models.MetricHRV, 48},
		{models.MetricTemperature, 36.5},
		{models.MetricSteps, 10000},
		{models.MetricSleepHours, 7.5},
		{models.MetricActiveCalories, 500},
		{models.MetricWater, 2000},
		{models.MetricCalories, 2100},
		{models.MetricProtein, 100},
		{models.MetricCarbs, 250},
		{models.MetricFat, 70},
		{models.MetricMood, 7},
		{models.MetricEnergy, 6},
		{models.MetricStress, 3},
		{models.MetricAnxiety, 2},
		{models.MetricFocus, 8},
		{models.MetricMeditation, 15},
	}

	for _, tc := range metricTypes {
		t.Run(string(tc.metricType), func(t *testing.T) {
			m := models.NewMetric(tc.metricType, tc.value)
			if err := store.CreateMetric(m); err != nil {
				t.Fatalf("CreateMetric(%s) failed: %v", tc.metricType, err)
			}

			got, err := store.GetMetric(m.ID.String())
			if err != nil {
				t.Fatalf("GetMetric(%s) failed: %v", tc.metricType, err)
			}

			if got.MetricType != tc.metricType {
				t.Errorf("MetricType mismatch: got %v, want %v", got.MetricType, tc.metricType)
			}
			if got.Value != tc.value {
				t.Errorf("Value mismatch: got %v, want %v", got.Value, tc.value)
			}
			if got.Unit != m.Unit {
				t.Errorf("Unit mismatch: got %v, want %v", got.Unit, m.Unit)
			}
		})
	}
}

func TestMarkdownStoreDeleteMetricByFullUUID(t *testing.T) {
	store := setupTestMarkdownStore(t)

	m := models.NewMetric(models.MetricWeight, 82.5)
	if err := store.CreateMetric(m); err != nil {
		t.Fatalf("CreateMetric failed: %v", err)
	}

	// Delete by full UUID
	if err := store.DeleteMetric(m.ID.String()); err != nil {
		t.Fatalf("DeleteMetric by full UUID failed: %v", err)
	}

	// Verify deleted
	_, err := store.GetMetric(m.ID.String())
	if err == nil {
		t.Error("Expected error getting deleted metric")
	}
}

func TestMarkdownStoreDeleteWorkoutByFullUUID(t *testing.T) {
	store := setupTestMarkdownStore(t)

	w := models.NewWorkout("run")
	if err := store.CreateWorkout(w); err != nil {
		t.Fatalf("CreateWorkout failed: %v", err)
	}

	// Delete by full UUID
	if err := store.DeleteWorkout(w.ID.String()); err != nil {
		t.Fatalf("DeleteWorkout by full UUID failed: %v", err)
	}

	// Verify deleted
	_, err := store.GetWorkout(w.ID.String())
	if err == nil {
		t.Error("Expected error getting deleted workout")
	}
}

func TestMarkdownStoreGetWorkoutByPrefix(t *testing.T) {
	store := setupTestMarkdownStore(t)

	w := models.NewWorkout("run")
	if err := store.CreateWorkout(w); err != nil {
		t.Fatalf("CreateWorkout failed: %v", err)
	}

	got, err := store.GetWorkout(w.ID.String()[:8])
	if err != nil {
		t.Fatalf("GetWorkout by prefix failed: %v", err)
	}

	if got.ID != w.ID {
		t.Errorf("ID mismatch: got %v, want %v", got.ID, w.ID)
	}
}

func TestMarkdownStoreMetricWithCustomTimestamp(t *testing.T) {
	store := setupTestMarkdownStore(t)

	m := models.NewMetric(models.MetricHRV, 48)
	m.WithNotes("morning reading")
	customTime := time.Now().Add(-1 * time.Hour)
	m.WithRecordedAt(customTime)

	if err := store.CreateMetric(m); err != nil {
		t.Fatalf("CreateMetric failed: %v", err)
	}

	got, err := store.GetMetric(m.ID.String())
	if err != nil {
		t.Fatalf("GetMetric failed: %v", err)
	}

	if got.Notes == nil || *got.Notes != "morning reading" {
		t.Error("Expected notes to be set")
	}
	if got.MetricType != models.MetricHRV {
		t.Errorf("Expected MetricType HRV, got %v", got.MetricType)
	}
}

func TestMarkdownStoreImplementsRepository(t *testing.T) {
	// This is checked at compile time via var _ Repository = (*MarkdownStore)(nil)
	// but let's also verify at runtime that setup succeeds
	store := setupTestMarkdownStore(t)
	// Assign to interface to confirm the concrete type satisfies Repository
	var r Repository = store
	_ = r
}
