// ABOUTME: Tests for export and import functionality.
// ABOUTME: Verifies JSON, YAML, and Markdown export formats.
package storage

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

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
