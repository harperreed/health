// ABOUTME: Tests for MCP server, tools, and resources.
// ABOUTME: Covers NewServer, tool handlers, and resource handlers.
package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/harperreed/health/internal/models"
	"github.com/harperreed/health/internal/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// setupTestDB creates a test database in a temp directory.
func setupTestDB(t *testing.T) *storage.DB {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "health-mcp-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	dbPath := filepath.Join(tmpDir, "health.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return db
}

func TestNewServer(t *testing.T) {
	db := setupTestDB(t)

	server, err := NewServer(db)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	if server == nil {
		t.Fatal("Expected non-nil server")
	}
	if server.mcpServer == nil {
		t.Error("Expected non-nil mcpServer")
	}
	if server.repo == nil {
		t.Error("Expected non-nil repo")
	}
}

func TestHandleAddMetric(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	tests := []struct {
		name      string
		input     addMetricInput
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid weight metric",
			input: addMetricInput{
				MetricType: "weight",
				Value:      82.5,
			},
			wantErr: false,
		},
		{
			name: "valid metric with notes",
			input: addMetricInput{
				MetricType: "mood",
				Value:      7,
				Notes:      "feeling good",
			},
			wantErr: false,
		},
		{
			name: "valid metric with RFC3339 timestamp",
			input: addMetricInput{
				MetricType: "hrv",
				Value:      48,
				RecordedAt: "2025-01-31T08:00:00Z",
			},
			wantErr: false,
		},
		{
			name: "valid metric with simple timestamp",
			input: addMetricInput{
				MetricType: "steps",
				Value:      10000,
				RecordedAt: "2025-01-31 08:00",
			},
			wantErr: false,
		},
		{
			name: "invalid metric type",
			input: addMetricInput{
				MetricType: "invalid_type",
				Value:      100,
			},
			wantErr:   true,
			errSubstr: "unknown metric type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, output, err := server.handleAddMetric(ctx, &mcp.CallToolRequest{}, tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errSubstr != "" && !contains(err.Error(), tt.errSubstr) {
					t.Errorf("Error %q should contain %q", err.Error(), tt.errSubstr)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if output.MetricType != tt.input.MetricType {
				t.Errorf("MetricType = %s, want %s", output.MetricType, tt.input.MetricType)
			}
			if output.Value != tt.input.Value {
				t.Errorf("Value = %f, want %f", output.Value, tt.input.Value)
			}
			if output.ID == "" {
				t.Error("Expected non-empty ID")
			}
			if output.Message == "" {
				t.Error("Expected non-empty Message")
			}
		})
	}
}

func TestHandleListMetrics(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Add some test metrics
	m1 := models.NewMetric(models.MetricWeight, 82.5)
	m2 := models.NewMetric(models.MetricMood, 7)
	db.CreateMetric(m1)
	db.CreateMetric(m2)

	tests := []struct {
		name     string
		input    listMetricsInput
		minCount int
	}{
		{
			name:     "list all metrics",
			input:    listMetricsInput{},
			minCount: 2,
		},
		{
			name:     "list with default limit",
			input:    listMetricsInput{Limit: 0},
			minCount: 2,
		},
		{
			name:     "list with limit 1",
			input:    listMetricsInput{Limit: 1},
			minCount: 1,
		},
		{
			name:     "filter by type",
			input:    listMetricsInput{MetricType: "weight"},
			minCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, output, err := server.handleListMetrics(ctx, &mcp.CallToolRequest{}, tt.input)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if output == nil {
				t.Error("Expected non-nil output")
			}
		})
	}
}

func TestHandleListMetricsEmpty(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	_, output, err := server.handleListMetrics(ctx, &mcp.CallToolRequest{}, listMetricsInput{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should return a message map for empty results
	if output == nil {
		t.Error("Expected non-nil output")
	}
}

func TestHandleDeleteMetric(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Create a metric to delete
	m := models.NewMetric(models.MetricWeight, 82.5)
	db.CreateMetric(m)

	// Delete by prefix
	_, output, err := server.handleDeleteMetric(ctx, &mcp.CallToolRequest{}, deleteMetricInput{
		ID: m.ID.String()[:8],
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if output.Message == "" {
		t.Error("Expected non-empty message")
	}

	// Verify deleted
	_, err = db.GetMetric(m.ID.String())
	if err == nil {
		t.Error("Expected metric to be deleted")
	}
}

func TestHandleDeleteMetricNotFound(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	_, _, err := server.handleDeleteMetric(ctx, &mcp.CallToolRequest{}, deleteMetricInput{
		ID: "nonexistent",
	})

	if err == nil {
		t.Error("Expected error for nonexistent metric")
	}
}

func TestHandleAddWorkout(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	tests := []struct {
		name  string
		input addWorkoutInput
	}{
		{
			name: "simple workout",
			input: addWorkoutInput{
				WorkoutType: "run",
			},
		},
		{
			name: "workout with duration",
			input: addWorkoutInput{
				WorkoutType:     "lift",
				DurationMinutes: 45,
			},
		},
		{
			name: "workout with notes",
			input: addWorkoutInput{
				WorkoutType: "swim",
				Notes:       "Pool workout",
			},
		},
		{
			name: "workout with all fields",
			input: addWorkoutInput{
				WorkoutType:     "yoga",
				DurationMinutes: 60,
				Notes:           "Morning session",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, output, err := server.handleAddWorkout(ctx, &mcp.CallToolRequest{}, tt.input)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if output.WorkoutType != tt.input.WorkoutType {
				t.Errorf("WorkoutType = %s, want %s", output.WorkoutType, tt.input.WorkoutType)
			}
			if output.ID == "" {
				t.Error("Expected non-empty ID")
			}
		})
	}
}

func TestHandleAddWorkoutMetric(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Create a workout first
	w := models.NewWorkout("run")
	db.CreateWorkout(w)

	tests := []struct {
		name  string
		input addWorkoutMetricInput
	}{
		{
			name: "distance metric",
			input: addWorkoutMetricInput{
				WorkoutID:  w.ID.String()[:8],
				MetricName: "distance",
				Value:      5.2,
				Unit:       "km",
			},
		},
		{
			name: "metric without unit",
			input: addWorkoutMetricInput{
				WorkoutID:  w.ID.String()[:8],
				MetricName: "sets",
				Value:      4,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, output, err := server.handleAddWorkoutMetric(ctx, &mcp.CallToolRequest{}, tt.input)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if output.Message == "" {
				t.Error("Expected non-empty message")
			}
		})
	}
}

func TestHandleAddWorkoutMetricNotFound(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	_, _, err := server.handleAddWorkoutMetric(ctx, &mcp.CallToolRequest{}, addWorkoutMetricInput{
		WorkoutID:  "nonexistent",
		MetricName: "distance",
		Value:      5.0,
	})

	if err == nil {
		t.Error("Expected error for nonexistent workout")
	}
}

func TestHandleListWorkouts(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Create test workouts
	w1 := models.NewWorkout("run")
	w2 := models.NewWorkout("lift")
	db.CreateWorkout(w1)
	db.CreateWorkout(w2)

	tests := []struct {
		name  string
		input listWorkoutsInput
	}{
		{
			name:  "list all workouts",
			input: listWorkoutsInput{},
		},
		{
			name:  "list with limit",
			input: listWorkoutsInput{Limit: 1},
		},
		{
			name:  "filter by type",
			input: listWorkoutsInput{WorkoutType: "run"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, output, err := server.handleListWorkouts(ctx, &mcp.CallToolRequest{}, tt.input)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if output == nil {
				t.Error("Expected non-nil output")
			}
		})
	}
}

func TestHandleListWorkoutsEmpty(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	_, output, err := server.handleListWorkouts(ctx, &mcp.CallToolRequest{}, listWorkoutsInput{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if output == nil {
		t.Error("Expected non-nil output")
	}
}

func TestHandleGetWorkout(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Create a workout with metrics
	w := models.NewWorkout("run")
	w.WithDuration(30)
	db.CreateWorkout(w)

	wm := models.NewWorkoutMetric(w.ID, "distance", 5.0, "km")
	db.AddWorkoutMetric(wm)

	_, output, err := server.handleGetWorkout(ctx, &mcp.CallToolRequest{}, getWorkoutInput{
		ID: w.ID.String()[:8],
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if output == nil {
		t.Error("Expected non-nil output")
	}
}

func TestHandleGetWorkoutNotFound(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	_, _, err := server.handleGetWorkout(ctx, &mcp.CallToolRequest{}, getWorkoutInput{
		ID: "nonexistent",
	})

	if err == nil {
		t.Error("Expected error for nonexistent workout")
	}
}

func TestHandleDeleteWorkout(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Create a workout
	w := models.NewWorkout("run")
	db.CreateWorkout(w)

	_, output, err := server.handleDeleteWorkout(ctx, &mcp.CallToolRequest{}, getWorkoutInput{
		ID: w.ID.String()[:8],
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if output.Message == "" {
		t.Error("Expected non-empty message")
	}

	// Verify deleted
	_, err = db.GetWorkout(w.ID.String())
	if err == nil {
		t.Error("Expected workout to be deleted")
	}
}

func TestHandleDeleteWorkoutNotFound(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	_, _, err := server.handleDeleteWorkout(ctx, &mcp.CallToolRequest{}, getWorkoutInput{
		ID: "nonexistent",
	})

	if err == nil {
		t.Error("Expected error for nonexistent workout")
	}
}

func TestHandleGetLatest(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Add metrics
	m1 := models.NewMetric(models.MetricWeight, 82.5)
	m2 := models.NewMetric(models.MetricMood, 7)
	db.CreateMetric(m1)
	db.CreateMetric(m2)

	tests := []struct {
		name  string
		input getLatestInput
	}{
		{
			name:  "get all latest",
			input: getLatestInput{},
		},
		{
			name:  "get specific types",
			input: getLatestInput{MetricTypes: []string{"weight", "mood"}},
		},
		{
			name:  "get single type",
			input: getLatestInput{MetricTypes: []string{"weight"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, output, err := server.handleGetLatest(ctx, &mcp.CallToolRequest{}, tt.input)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if output == nil {
				t.Error("Expected non-nil output")
			}
		})
	}
}

func TestHandleRecentResource(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Add some data
	m := models.NewMetric(models.MetricWeight, 82.5)
	db.CreateMetric(m)

	w := models.NewWorkout("run")
	db.CreateWorkout(w)

	result, err := server.handleRecentResource(ctx, &mcp.ReadResourceRequest{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if len(result.Contents) == 0 {
		t.Error("Expected non-empty contents")
	}
	if result.Contents[0].URI != "health://recent" {
		t.Errorf("URI = %s, want health://recent", result.Contents[0].URI)
	}
	if result.Contents[0].MIMEType != "application/json" {
		t.Errorf("MIMEType = %s, want application/json", result.Contents[0].MIMEType)
	}
	if result.Contents[0].Text == "" {
		t.Error("Expected non-empty Text")
	}
}

func TestHandleTodayResource(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Add a metric for today
	m := models.NewMetric(models.MetricWeight, 82.5)
	db.CreateMetric(m)

	// Add a workout for today
	w := models.NewWorkout("run")
	db.CreateWorkout(w)

	result, err := server.handleTodayResource(ctx, &mcp.ReadResourceRequest{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if len(result.Contents) == 0 {
		t.Error("Expected non-empty contents")
	}
	if result.Contents[0].URI != "health://today" {
		t.Errorf("URI = %s, want health://today", result.Contents[0].URI)
	}
}

func TestHandleTodayResourceFiltersOldData(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Add an old metric (yesterday)
	oldMetric := models.NewMetric(models.MetricWeight, 80.0)
	oldMetric.RecordedAt = time.Now().Add(-48 * time.Hour)
	db.CreateMetric(oldMetric)

	// Add a today metric
	todayMetric := models.NewMetric(models.MetricWeight, 82.5)
	db.CreateMetric(todayMetric)

	result, err := server.handleTodayResource(ctx, &mcp.ReadResourceRequest{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// The result should only include today's metric
	// (The old metric should be filtered out)
	if !contains(result.Contents[0].Text, "82.5") {
		t.Error("Expected today's metric in result")
	}
}

func TestHandleSummaryResource(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Add various metrics
	db.CreateMetric(models.NewMetric(models.MetricWeight, 82.5))
	db.CreateMetric(models.NewMetric(models.MetricMood, 7))
	db.CreateMetric(models.NewMetric(models.MetricSteps, 10000))
	db.CreateMetric(models.NewMetric(models.MetricCalories, 2000))

	// Add a workout
	w := models.NewWorkout("run")
	w.WithDuration(30)
	db.CreateWorkout(w)

	result, err := server.handleSummaryResource(ctx, &mcp.ReadResourceRequest{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if len(result.Contents) == 0 {
		t.Error("Expected non-empty contents")
	}
	if result.Contents[0].URI != "health://summary" {
		t.Errorf("URI = %s, want health://summary", result.Contents[0].URI)
	}

	// Verify the summary contains expected sections
	text := result.Contents[0].Text
	if !contains(text, "biometrics") {
		t.Error("Expected biometrics section")
	}
	if !contains(text, "activity") {
		t.Error("Expected activity section")
	}
	if !contains(text, "nutrition") {
		t.Error("Expected nutrition section")
	}
	if !contains(text, "mental") {
		t.Error("Expected mental section")
	}
}

func TestHandleSummaryResourceEmpty(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Don't add any data - test empty state
	result, err := server.handleSummaryResource(ctx, &mcp.ReadResourceRequest{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestHandleRecentResourceEmpty(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Don't add any data
	result, err := server.handleRecentResource(ctx, &mcp.ReadResourceRequest{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestHandleTodayResourceEmpty(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Don't add any data
	result, err := server.handleTodayResource(ctx, &mcp.ReadResourceRequest{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestHandleTodayResourceFiltersOldWorkouts(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Add an old workout (yesterday)
	oldWorkout := models.NewWorkout("run")
	oldWorkout.StartedAt = time.Now().Add(-48 * time.Hour)
	db.CreateWorkout(oldWorkout)

	// Add a today workout
	todayWorkout := models.NewWorkout("lift")
	db.CreateWorkout(todayWorkout)

	result, err := server.handleTodayResource(ctx, &mcp.ReadResourceRequest{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// The result should include today's workout
	if !contains(result.Contents[0].Text, "lift") {
		t.Error("Expected today's workout in result")
	}
}

func TestHandleAddMetricWithInvalidTimestamp(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Invalid timestamp format - should still work but use current time
	_, output, err := server.handleAddMetric(ctx, &mcp.CallToolRequest{}, addMetricInput{
		MetricType: "weight",
		Value:      82.5,
		RecordedAt: "invalid-timestamp",
	})

	// Should succeed, but with current time (invalid timestamp is ignored)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if output.ID == "" {
		t.Error("Expected non-empty ID")
	}
}

func TestHandleGetLatestEmpty(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Get latest with no data
	_, output, err := server.handleGetLatest(ctx, &mcp.CallToolRequest{}, getLatestInput{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should return empty results
	results, ok := output.(map[string]interface{})
	if !ok {
		t.Fatal("Expected map output")
	}
	if len(results) != 0 {
		t.Errorf("Expected empty results, got %d", len(results))
	}
}

func TestHandleSummaryResourceWithAllCategories(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Add metrics from all categories
	// Biometrics
	db.CreateMetric(models.NewMetric(models.MetricWeight, 82.5))
	db.CreateMetric(models.NewMetric(models.MetricBodyFat, 15))
	db.CreateMetric(models.NewMetric(models.MetricBPSys, 120))
	db.CreateMetric(models.NewMetric(models.MetricBPDia, 80))
	db.CreateMetric(models.NewMetric(models.MetricHeartRate, 65))
	db.CreateMetric(models.NewMetric(models.MetricHRV, 48))
	db.CreateMetric(models.NewMetric(models.MetricTemperature, 36.5))

	// Activity
	db.CreateMetric(models.NewMetric(models.MetricSteps, 10000))
	db.CreateMetric(models.NewMetric(models.MetricSleepHours, 7.5))
	db.CreateMetric(models.NewMetric(models.MetricActiveCalories, 500))

	// Nutrition
	db.CreateMetric(models.NewMetric(models.MetricWater, 2000))
	db.CreateMetric(models.NewMetric(models.MetricCalories, 2000))
	db.CreateMetric(models.NewMetric(models.MetricProtein, 100))
	db.CreateMetric(models.NewMetric(models.MetricCarbs, 250))
	db.CreateMetric(models.NewMetric(models.MetricFat, 70))

	// Mental
	db.CreateMetric(models.NewMetric(models.MetricMood, 7))
	db.CreateMetric(models.NewMetric(models.MetricEnergy, 6))
	db.CreateMetric(models.NewMetric(models.MetricStress, 3))
	db.CreateMetric(models.NewMetric(models.MetricAnxiety, 2))
	db.CreateMetric(models.NewMetric(models.MetricFocus, 8))
	db.CreateMetric(models.NewMetric(models.MetricMeditation, 15))

	result, err := server.handleSummaryResource(ctx, &mcp.ReadResourceRequest{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	text := result.Contents[0].Text

	// Verify all categories have data
	if !contains(text, "weight") {
		t.Error("Expected weight in biometrics")
	}
	if !contains(text, "steps") {
		t.Error("Expected steps in activity")
	}
	if !contains(text, "water") {
		t.Error("Expected water in nutrition")
	}
	if !contains(text, "mood") {
		t.Error("Expected mood in mental")
	}
}

func TestHandleAddWorkoutWithAllFields(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	_, output, err := server.handleAddWorkout(ctx, &mcp.CallToolRequest{}, addWorkoutInput{
		WorkoutType:     "hiit",
		DurationMinutes: 30,
		Notes:           "Intense session",
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if output.WorkoutType != "hiit" {
		t.Errorf("Expected WorkoutType 'hiit', got %s", output.WorkoutType)
	}
}

func TestHandleListMetricsWithInvalidType(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Invalid metric type - should still work but return empty
	_, output, err := server.handleListMetrics(ctx, &mcp.CallToolRequest{}, listMetricsInput{
		MetricType: "invalid_type",
		Limit:      10,
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should return message about no metrics found
	if output == nil {
		t.Error("Expected non-nil output")
	}
}

func TestHandleListWorkoutsWithFilter(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Create workouts of different types
	w1 := models.NewWorkout("run")
	w2 := models.NewWorkout("lift")
	w3 := models.NewWorkout("run")
	db.CreateWorkout(w1)
	db.CreateWorkout(w2)
	db.CreateWorkout(w3)

	// Filter by type
	_, output, err := server.handleListWorkouts(ctx, &mcp.CallToolRequest{}, listWorkoutsInput{
		WorkoutType: "run",
		Limit:       10,
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	workouts, ok := output.([]*models.Workout)
	if !ok {
		t.Fatal("Expected workout slice output")
	}
	if len(workouts) != 2 {
		t.Errorf("Expected 2 run workouts, got %d", len(workouts))
	}
}

func TestHandleRecentResourceWithData(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Add multiple metrics
	for i := 0; i < 15; i++ {
		m := models.NewMetric(models.MetricWeight, float64(80+i))
		db.CreateMetric(m)
	}

	// Add multiple workouts
	for i := 0; i < 8; i++ {
		w := models.NewWorkout("run")
		db.CreateWorkout(w)
	}

	result, err := server.handleRecentResource(ctx, &mcp.ReadResourceRequest{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if len(result.Contents) == 0 {
		t.Error("Expected non-empty contents")
	}
}

func TestHandleAddWorkoutMetricWithError(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Create a workout
	w := models.NewWorkout("run")
	db.CreateWorkout(w)

	// Try to add metric to a nonexistent workout prefix
	_, _, err := server.handleAddWorkoutMetric(ctx, &mcp.CallToolRequest{}, addWorkoutMetricInput{
		WorkoutID:  "nonexistent",
		MetricName: "distance",
		Value:      5.0,
		Unit:       "km",
	})

	if err == nil {
		t.Error("Expected error for nonexistent workout")
	}
}

func TestHandleGetLatestWithSpecificTypes(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Add metrics
	db.CreateMetric(models.NewMetric(models.MetricWeight, 82.5))
	db.CreateMetric(models.NewMetric(models.MetricMood, 7))
	db.CreateMetric(models.NewMetric(models.MetricSteps, 10000))

	// Get specific types only
	_, output, err := server.handleGetLatest(ctx, &mcp.CallToolRequest{}, getLatestInput{
		MetricTypes: []string{"weight", "nonexistent"},
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	results, ok := output.(map[string]interface{})
	if !ok {
		t.Fatal("Expected map output")
	}

	// Should have weight but not nonexistent
	if _, ok := results["weight"]; !ok {
		t.Error("Expected weight in results")
	}
	if _, ok := results["nonexistent"]; ok {
		t.Error("Should not have nonexistent in results")
	}
}

func TestHandleAddWorkoutZeroDuration(t *testing.T) {
	db := setupTestDB(t)
	server, _ := NewServer(db)
	ctx := context.Background()

	// Duration of 0 should not be set
	_, output, err := server.handleAddWorkout(ctx, &mcp.CallToolRequest{}, addWorkoutInput{
		WorkoutType:     "yoga",
		DurationMinutes: 0,
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if output.WorkoutType != "yoga" {
		t.Errorf("Expected WorkoutType 'yoga', got %s", output.WorkoutType)
	}
}

// Helper function.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
