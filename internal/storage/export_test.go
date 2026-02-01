// ABOUTME: Tests for export and import functionality.
// ABOUTME: Verifies JSON, YAML, and Markdown export formats.
package storage

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/harperreed/health/internal/models"
	"gopkg.in/yaml.v3"
)

func TestExportJSON(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add test data
	m := models.NewMetric(models.MetricWeight, 82.5)
	m.WithNotes("test note")
	db.CreateMetric(m)

	w := models.NewWorkout("run")
	w.WithDuration(30)
	db.CreateWorkout(w)

	wm := models.NewWorkoutMetric(w.ID, "distance", 5.0, "km")
	db.AddWorkoutMetric(wm)

	// Export
	data, err := db.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	// Parse and verify
	var export ExportData
	if err := json.Unmarshal(data, &export); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if export.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", export.Version)
	}
	if export.Tool != "health" {
		t.Errorf("Expected tool health, got %s", export.Tool)
	}
	if len(export.Metrics) != 1 {
		t.Errorf("Expected 1 metric, got %d", len(export.Metrics))
	}
	if len(export.Workouts) != 1 {
		t.Errorf("Expected 1 workout, got %d", len(export.Workouts))
	}
}

func TestExportYAML(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add test data
	m := models.NewMetric(models.MetricWeight, 82.5)
	db.CreateMetric(m)

	// Export
	data, err := db.ExportYAML()
	if err != nil {
		t.Fatalf("ExportYAML failed: %v", err)
	}

	// Verify it's valid YAML
	var yamlData map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	if yamlData["version"] != "1.0" {
		t.Errorf("Expected version 1.0, got %v", yamlData["version"])
	}
	if yamlData["tool"] != "health" {
		t.Errorf("Expected tool health, got %v", yamlData["tool"])
	}

	metrics, ok := yamlData["metrics"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected metrics to be a map")
	}
	if _, ok := metrics["weight"]; !ok {
		t.Error("Expected weight in metrics")
	}
}

func TestExportMarkdown(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add test data
	m := models.NewMetric(models.MetricWeight, 82.5)
	m.WithNotes("morning")
	db.CreateMetric(m)

	// Export all
	md, err := db.ExportMarkdown(nil, nil)
	if err != nil {
		t.Fatalf("ExportMarkdown failed: %v", err)
	}

	if !strings.Contains(md, "# Health Export") {
		t.Error("Expected markdown header")
	}
	if !strings.Contains(md, "## weight") {
		t.Error("Expected weight section")
	}
	if !strings.Contains(md, "82.50") {
		t.Error("Expected value in table")
	}

	// Export filtered by type
	weightType := models.MetricWeight
	md, err = db.ExportMarkdown(&weightType, nil)
	if err != nil {
		t.Fatalf("ExportMarkdown with type failed: %v", err)
	}

	if !strings.Contains(md, "## weight") {
		t.Error("Expected weight section in filtered export")
	}
}

func TestExportMarkdownWithSince(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add old and new metrics
	oldMetric := models.NewMetric(models.MetricWeight, 80.0)
	oldMetric.RecordedAt = time.Now().Add(-30 * 24 * time.Hour) // 30 days ago
	db.CreateMetric(oldMetric)

	newMetric := models.NewMetric(models.MetricWeight, 82.5)
	newMetric.RecordedAt = time.Now()
	db.CreateMetric(newMetric)

	// Export with since filter
	since := time.Now().Add(-7 * 24 * time.Hour) // 7 days ago
	md, err := db.ExportMarkdown(nil, &since)
	if err != nil {
		t.Fatalf("ExportMarkdown with since failed: %v", err)
	}

	// Should only contain the new metric
	if !strings.Contains(md, "82.50") {
		t.Error("Expected new metric value")
	}
	if strings.Contains(md, "80.00") {
		t.Error("Should not contain old metric value")
	}
}

func TestImportJSON(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	jsonData := `{
		"version": "1.0",
		"exported_at": "2026-01-31T12:00:00Z",
		"tool": "health",
		"metrics": [
			{
				"ID": "11111111-1111-1111-1111-111111111111",
				"MetricType": "weight",
				"Value": 82.5,
				"Unit": "kg",
				"RecordedAt": "2026-01-31T08:00:00Z",
				"CreatedAt": "2026-01-31T08:00:00Z"
			}
		],
		"workouts": [
			{
				"ID": "22222222-2222-2222-2222-222222222222",
				"WorkoutType": "run",
				"StartedAt": "2026-01-31T07:00:00Z",
				"CreatedAt": "2026-01-31T07:00:00Z",
				"Metrics": [
					{
						"ID": "33333333-3333-3333-3333-333333333333",
						"MetricName": "distance",
						"Value": 5.2,
						"CreatedAt": "2026-01-31T07:30:00Z"
					}
				]
			}
		]
	}`

	if err := db.ImportJSON([]byte(jsonData)); err != nil {
		t.Fatalf("ImportJSON failed: %v", err)
	}

	// Verify imported data
	metrics, err := db.ListMetrics(nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics failed: %v", err)
	}
	if len(metrics) != 1 {
		t.Errorf("Expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].Value != 82.5 {
		t.Errorf("Expected value 82.5, got %v", metrics[0].Value)
	}

	workouts, err := db.ListWorkouts(nil, 0)
	if err != nil {
		t.Fatalf("ListWorkouts failed: %v", err)
	}
	if len(workouts) != 1 {
		t.Errorf("Expected 1 workout, got %d", len(workouts))
	}
}

func TestExportMarkdownWithWorkouts(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add workout
	w := models.NewWorkout("run")
	w.WithDuration(45)
	w.WithNotes("Morning jog")
	db.CreateWorkout(w)

	// Export
	md, err := db.ExportMarkdown(nil, nil)
	if err != nil {
		t.Fatalf("ExportMarkdown failed: %v", err)
	}

	if !strings.Contains(md, "## Workouts") {
		t.Error("Expected Workouts section")
	}
	if !strings.Contains(md, "run") {
		t.Error("Expected workout type in table")
	}
	if !strings.Contains(md, "45 min") {
		t.Error("Expected duration in table")
	}
}

func TestExportYAMLWithAllOptionalFields(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add metric with notes
	m := models.NewMetric(models.MetricWeight, 82.5)
	m.WithNotes("morning weight")
	db.CreateMetric(m)

	// Add workout with all optional fields
	w := models.NewWorkout("run")
	w.WithDuration(30)
	w.WithNotes("Easy run")
	db.CreateWorkout(w)

	// Add workout metric with unit
	wm := models.NewWorkoutMetric(w.ID, "distance", 5.0, "km")
	db.AddWorkoutMetric(wm)

	// Export
	data, err := db.ExportYAML()
	if err != nil {
		t.Fatalf("ExportYAML failed: %v", err)
	}

	// Verify it's valid YAML
	var yamlData map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	// Verify workouts section
	workouts, ok := yamlData["workouts"].([]interface{})
	if !ok {
		t.Fatalf("Expected workouts to be an array")
	}
	if len(workouts) != 1 {
		t.Errorf("Expected 1 workout, got %d", len(workouts))
	}

	// Check workout has duration and notes
	workout := workouts[0].(map[string]interface{})
	if _, ok := workout["duration_minutes"]; !ok {
		t.Error("Expected workout to have duration_minutes")
	}
	if _, ok := workout["notes"]; !ok {
		t.Error("Expected workout to have notes")
	}
	if _, ok := workout["metrics"]; !ok {
		t.Error("Expected workout to have metrics")
	}
}

func TestExportYAMLWithWorkoutMetricNoUnit(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add workout
	w := models.NewWorkout("lift")
	db.CreateWorkout(w)

	// Add workout metric without unit
	wm := models.NewWorkoutMetric(w.ID, "sets", 4, "")
	db.AddWorkoutMetric(wm)

	// Export
	data, err := db.ExportYAML()
	if err != nil {
		t.Fatalf("ExportYAML failed: %v", err)
	}

	// Verify it's valid YAML
	var yamlData map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	// Verify workouts section has the metric
	workouts := yamlData["workouts"].([]interface{})
	workout := workouts[0].(map[string]interface{})
	metrics := workout["metrics"].([]interface{})
	if len(metrics) != 1 {
		t.Errorf("Expected 1 workout metric, got %d", len(metrics))
	}
}

func TestExportYAMLWithNullableWorkoutFields(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add workout without duration or notes
	w := models.NewWorkout("swim")
	db.CreateWorkout(w)

	// Export
	data, err := db.ExportYAML()
	if err != nil {
		t.Fatalf("ExportYAML failed: %v", err)
	}

	// Verify it's valid YAML
	var yamlData map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	// Verify workout exists
	workouts := yamlData["workouts"].([]interface{})
	if len(workouts) != 1 {
		t.Errorf("Expected 1 workout, got %d", len(workouts))
	}
}

func TestExportMarkdownWithSinceAndType(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add metrics
	m := models.NewMetric(models.MetricWeight, 82.5)
	db.CreateMetric(m)

	// Export with type filter and since
	weightType := models.MetricWeight
	since := time.Now().Add(-24 * time.Hour)
	md, err := db.ExportMarkdown(&weightType, &since)
	if err != nil {
		t.Fatalf("ExportMarkdown failed: %v", err)
	}

	if !strings.Contains(md, "## weight") {
		t.Error("Expected weight section in filtered export")
	}
}

func TestExportMarkdownWorkoutsWithSince(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add old workout
	oldWorkout := models.NewWorkout("run")
	oldWorkout.StartedAt = time.Now().Add(-30 * 24 * time.Hour)
	db.CreateWorkout(oldWorkout)

	// Add new workout
	newWorkout := models.NewWorkout("lift")
	newWorkout.WithDuration(45)
	db.CreateWorkout(newWorkout)

	// Export with since filter
	since := time.Now().Add(-7 * 24 * time.Hour)
	md, err := db.ExportMarkdown(nil, &since)
	if err != nil {
		t.Fatalf("ExportMarkdown failed: %v", err)
	}

	// Should contain only the new workout
	if !strings.Contains(md, "lift") {
		t.Error("Expected new workout in export")
	}
}

func TestExportMarkdownWorkoutWithoutDuration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add workout without duration
	w := models.NewWorkout("yoga")
	db.CreateWorkout(w)

	// Export
	md, err := db.ExportMarkdown(nil, nil)
	if err != nil {
		t.Fatalf("ExportMarkdown failed: %v", err)
	}

	if !strings.Contains(md, "yoga") {
		t.Error("Expected workout type in export")
	}
}

func TestExportMarkdownWorkoutWithoutNotes(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add workout without notes
	w := models.NewWorkout("cycling")
	w.WithDuration(60)
	db.CreateWorkout(w)

	// Export
	md, err := db.ExportMarkdown(nil, nil)
	if err != nil {
		t.Fatalf("ExportMarkdown failed: %v", err)
	}

	if !strings.Contains(md, "cycling") {
		t.Error("Expected workout type in export")
	}
}

func TestExportMarkdownWithTypeFilterOnlyNoNotes(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add metric without notes
	m := models.NewMetric(models.MetricMood, 7)
	db.CreateMetric(m)

	// Export with type filter
	moodType := models.MetricMood
	md, err := db.ExportMarkdown(&moodType, nil)
	if err != nil {
		t.Fatalf("ExportMarkdown failed: %v", err)
	}

	if !strings.Contains(md, "mood") {
		t.Error("Expected mood section")
	}
}

func TestImportJSONInvalid(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Invalid JSON
	err := db.ImportJSON([]byte("not valid json"))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestExportYAMLMultipleMetricTypes(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add multiple metric types
	db.CreateMetric(models.NewMetric(models.MetricWeight, 82.5))
	db.CreateMetric(models.NewMetric(models.MetricMood, 7))
	db.CreateMetric(models.NewMetric(models.MetricSteps, 10000))

	// Export
	data, err := db.ExportYAML()
	if err != nil {
		t.Fatalf("ExportYAML failed: %v", err)
	}

	// Verify it's valid YAML with multiple metric types
	var yamlData map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	metrics := yamlData["metrics"].(map[string]interface{})
	if len(metrics) != 3 {
		t.Errorf("Expected 3 metric types, got %d", len(metrics))
	}
}

func TestGetAllDataWithWorkoutMetrics(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create workout with metrics
	w := models.NewWorkout("run")
	db.CreateWorkout(w)

	wm1 := models.NewWorkoutMetric(w.ID, "distance", 5.0, "km")
	wm2 := models.NewWorkoutMetric(w.ID, "pace", 5.5, "min/km")
	db.AddWorkoutMetric(wm1)
	db.AddWorkoutMetric(wm2)

	data, err := db.GetAllData()
	if err != nil {
		t.Fatalf("GetAllData failed: %v", err)
	}

	if len(data.Workouts) != 1 {
		t.Errorf("Expected 1 workout, got %d", len(data.Workouts))
	}
	if len(data.Workouts[0].Metrics) != 2 {
		t.Errorf("Expected 2 workout metrics, got %d", len(data.Workouts[0].Metrics))
	}
}

func TestImportDataWithWorkoutMetrics(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	workoutID := uuid.New()
	data := &ExportData{
		Version:    "1.0",
		ExportedAt: time.Now(),
		Tool:       "health",
		Metrics:    []*models.Metric{},
		Workouts: []*models.Workout{
			{
				ID:          workoutID,
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
					{
						ID:         uuid.New(),
						MetricName: "pace",
						Value:      5.5,
						CreatedAt:  time.Now(),
					},
				},
			},
		},
	}

	if err := db.ImportData(data); err != nil {
		t.Fatalf("ImportData failed: %v", err)
	}

	// Verify workout was imported with metrics
	w, err := db.GetWorkoutWithMetrics(workoutID.String())
	if err != nil {
		t.Fatalf("GetWorkoutWithMetrics failed: %v", err)
	}

	if len(w.Metrics) != 2 {
		t.Errorf("Expected 2 workout metrics, got %d", len(w.Metrics))
	}
}

func TestExportJSONWithWorkoutMetrics(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create workout with metrics
	w := models.NewWorkout("lift")
	w.WithDuration(45)
	db.CreateWorkout(w)

	wm := models.NewWorkoutMetric(w.ID, "sets", 4, "")
	db.AddWorkoutMetric(wm)

	// Export
	data, err := db.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	// Parse and verify
	var export ExportData
	if err := json.Unmarshal(data, &export); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if len(export.Workouts) != 1 {
		t.Errorf("Expected 1 workout, got %d", len(export.Workouts))
	}
	if len(export.Workouts[0].Metrics) != 1 {
		t.Errorf("Expected 1 workout metric, got %d", len(export.Workouts[0].Metrics))
	}
}

func TestExportMarkdownEmptyDB(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Export with no data
	md, err := db.ExportMarkdown(nil, nil)
	if err != nil {
		t.Fatalf("ExportMarkdown failed: %v", err)
	}

	if !strings.Contains(md, "# Health Export") {
		t.Error("Expected markdown header")
	}
}

func TestExportYAMLEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Export with no data
	data, err := db.ExportYAML()
	if err != nil {
		t.Fatalf("ExportYAML failed: %v", err)
	}

	// Should still be valid YAML
	var yamlData map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	if yamlData["version"] != "1.0" {
		t.Errorf("Expected version 1.0, got %v", yamlData["version"])
	}
}

func TestExportJSONEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Export with no data
	data, err := db.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	// Should still be valid JSON
	var export ExportData
	if err := json.Unmarshal(data, &export); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if export.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", export.Version)
	}
	if len(export.Metrics) != 0 {
		t.Errorf("Expected 0 metrics, got %d", len(export.Metrics))
	}
	if len(export.Workouts) != 0 {
		t.Errorf("Expected 0 workouts, got %d", len(export.Workouts))
	}
}

func TestImportDataMultipleItems(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

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
			{
				ID:         uuid.New(),
				MetricType: models.MetricMood,
				Value:      7,
				Unit:       "scale",
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
			},
			{
				ID:          uuid.New(),
				WorkoutType: "lift",
				StartedAt:   time.Now(),
				CreatedAt:   time.Now(),
			},
		},
	}

	if err := db.ImportData(data); err != nil {
		t.Fatalf("ImportData failed: %v", err)
	}

	metrics, _ := db.ListMetrics(nil, 0)
	if len(metrics) != 2 {
		t.Errorf("Expected 2 metrics, got %d", len(metrics))
	}

	workouts, _ := db.ListWorkouts(nil, 0)
	if len(workouts) != 2 {
		t.Errorf("Expected 2 workouts, got %d", len(workouts))
	}
}
