// ABOUTME: MCP tool implementations for health metrics.
// ABOUTME: Provides CRUD operations for metrics and workouts.
package mcp

import (
	"context"
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
	}, s.handleAddMetric)

	// list_metrics
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_metrics",
		Description: "List recent health metrics, optionally filtered by type",
	}, s.handleListMetrics)

	// delete_metric
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "delete_metric",
		Description: "Delete a metric by ID or ID prefix",
	}, s.handleDeleteMetric)

	// add_workout
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "add_workout",
		Description: "Create a new workout session",
	}, s.handleAddWorkout)

	// add_workout_metric
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "add_workout_metric",
		Description: "Add a metric to an existing workout",
	}, s.handleAddWorkoutMetric)

	// list_workouts
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "list_workouts",
		Description: "List recent workouts, optionally filtered by type",
	}, s.handleListWorkouts)

	// get_workout
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_workout",
		Description: "Get a workout with all its metrics",
	}, s.handleGetWorkout)

	// delete_workout
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "delete_workout",
		Description: "Delete a workout and its metrics",
	}, s.handleDeleteWorkout)

	// get_latest
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_latest",
		Description: "Get the most recent value for one or more metric types",
	}, s.handleGetLatest)
}

// Tool input/output types

type addMetricInput struct {
	MetricType string  `json:"metric_type" jsonschema:"description=Type of metric (weight, body_fat, bp_sys, bp_dia, heart_rate, hrv, temperature, steps, sleep_hours, active_calories, water, calories, protein, carbs, fat, mood, energy, stress, anxiety, focus, meditation),required"`
	Value      float64 `json:"value" jsonschema:"description=The metric value,required"`
	RecordedAt string  `json:"recorded_at,omitempty" jsonschema:"description=Timestamp (ISO 8601), defaults to now"`
	Notes      string  `json:"notes,omitempty" jsonschema:"description=Optional notes"`
}

type metricOutput struct {
	ID         string  `json:"id"`
	MetricType string  `json:"metric_type"`
	Value      float64 `json:"value"`
	Unit       string  `json:"unit"`
	Message    string  `json:"message"`
}

type listMetricsInput struct {
	MetricType string `json:"metric_type,omitempty" jsonschema:"description=Filter by metric type"`
	Limit      int    `json:"limit,omitempty" jsonschema:"description=Max results (default 20)"`
}

type deleteMetricInput struct {
	ID string `json:"id" jsonschema:"description=Metric ID or prefix,required"`
}

type simpleOutput struct {
	Message string `json:"message"`
}

type addWorkoutInput struct {
	WorkoutType     string `json:"workout_type" jsonschema:"description=Type of workout (run, lift, cycle, swim, etc.),required"`
	DurationMinutes int    `json:"duration_minutes,omitempty" jsonschema:"description=Duration in minutes"`
	Notes           string `json:"notes,omitempty" jsonschema:"description=Workout notes"`
}

type workoutOutput struct {
	ID          string `json:"id"`
	WorkoutType string `json:"workout_type"`
	Message     string `json:"message"`
}

type addWorkoutMetricInput struct {
	WorkoutID  string  `json:"workout_id" jsonschema:"description=Workout ID or prefix,required"`
	MetricName string  `json:"metric_name" jsonschema:"description=Name of the metric (distance, pace, sets, reps, etc.),required"`
	Value      float64 `json:"value" jsonschema:"description=The value,required"`
	Unit       string  `json:"unit,omitempty" jsonschema:"description=Unit of measurement"`
}

type listWorkoutsInput struct {
	WorkoutType string `json:"workout_type,omitempty" jsonschema:"description=Filter by workout type"`
	Limit       int    `json:"limit,omitempty" jsonschema:"description=Max results (default 20)"`
}

type getWorkoutInput struct {
	ID string `json:"id" jsonschema:"description=Workout ID or prefix,required"`
}

type getLatestInput struct {
	MetricTypes []string `json:"metric_types,omitempty" jsonschema:"description=List of metric types to get latest values for"`
}

// Tool handlers

func (s *Server) handleAddMetric(ctx context.Context, req *mcp.CallToolRequest, input addMetricInput) (*mcp.CallToolResult, metricOutput, error) {
	if !models.IsValidMetricType(input.MetricType) {
		return nil, metricOutput{}, fmt.Errorf("unknown metric type: %s", input.MetricType)
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
		return nil, metricOutput{}, fmt.Errorf("failed to create metric: %w", err)
	}

	return nil, metricOutput{
		ID:         m.ID.String()[:8],
		MetricType: input.MetricType,
		Value:      m.Value,
		Unit:       m.Unit,
		Message:    fmt.Sprintf("Added %s: %.2f %s (ID: %s)", input.MetricType, m.Value, m.Unit, m.ID.String()[:8]),
	}, nil
}

func (s *Server) handleListMetrics(ctx context.Context, req *mcp.CallToolRequest, input listMetricsInput) (*mcp.CallToolResult, any, error) {
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
		return nil, nil, fmt.Errorf("failed to list metrics: %w", err)
	}

	if len(metrics) == 0 {
		return nil, map[string]interface{}{"message": "No metrics found."}, nil
	}

	return nil, metrics, nil
}

func (s *Server) handleDeleteMetric(ctx context.Context, req *mcp.CallToolRequest, input deleteMetricInput) (*mcp.CallToolResult, simpleOutput, error) {
	if err := db.DeleteMetric(s.db, input.ID); err != nil {
		return nil, simpleOutput{}, fmt.Errorf("failed to delete metric: %w", err)
	}

	return nil, simpleOutput{
		Message: fmt.Sprintf("Deleted metric: %s", input.ID),
	}, nil
}

func (s *Server) handleAddWorkout(ctx context.Context, req *mcp.CallToolRequest, input addWorkoutInput) (*mcp.CallToolResult, workoutOutput, error) {
	w := models.NewWorkout(input.WorkoutType)
	if input.DurationMinutes > 0 {
		w.WithDuration(input.DurationMinutes)
	}
	if input.Notes != "" {
		w.WithNotes(input.Notes)
	}

	if err := db.CreateWorkout(s.db, w); err != nil {
		return nil, workoutOutput{}, fmt.Errorf("failed to create workout: %w", err)
	}

	return nil, workoutOutput{
		ID:          w.ID.String()[:8],
		WorkoutType: input.WorkoutType,
		Message:     fmt.Sprintf("Added %s workout (ID: %s)", input.WorkoutType, w.ID.String()[:8]),
	}, nil
}

func (s *Server) handleAddWorkoutMetric(ctx context.Context, req *mcp.CallToolRequest, input addWorkoutMetricInput) (*mcp.CallToolResult, simpleOutput, error) {
	w, err := db.GetWorkout(s.db, input.WorkoutID)
	if err != nil {
		return nil, simpleOutput{}, fmt.Errorf("workout not found: %s", input.WorkoutID)
	}

	wm := models.NewWorkoutMetric(w.ID, input.MetricName, input.Value, input.Unit)
	if err := db.AddWorkoutMetric(s.db, wm); err != nil {
		return nil, simpleOutput{}, fmt.Errorf("failed to add workout metric: %w", err)
	}

	return nil, simpleOutput{
		Message: fmt.Sprintf("Added %s: %.2f %s to workout", input.MetricName, input.Value, input.Unit),
	}, nil
}

func (s *Server) handleListWorkouts(ctx context.Context, req *mcp.CallToolRequest, input listWorkoutsInput) (*mcp.CallToolResult, any, error) {
	if input.Limit <= 0 {
		input.Limit = 20
	}

	var workoutType *string
	if input.WorkoutType != "" {
		workoutType = &input.WorkoutType
	}

	workouts, err := db.ListWorkouts(s.db, workoutType, input.Limit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list workouts: %w", err)
	}

	if len(workouts) == 0 {
		return nil, map[string]interface{}{"message": "No workouts found."}, nil
	}

	return nil, workouts, nil
}

func (s *Server) handleGetWorkout(ctx context.Context, req *mcp.CallToolRequest, input getWorkoutInput) (*mcp.CallToolResult, any, error) {
	w, err := db.GetWorkoutWithMetrics(s.db, input.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("workout not found: %s", input.ID)
	}

	return nil, w, nil
}

func (s *Server) handleDeleteWorkout(ctx context.Context, req *mcp.CallToolRequest, input getWorkoutInput) (*mcp.CallToolResult, simpleOutput, error) {
	if err := db.DeleteWorkout(s.db, input.ID); err != nil {
		return nil, simpleOutput{}, fmt.Errorf("failed to delete workout: %w", err)
	}

	return nil, simpleOutput{
		Message: fmt.Sprintf("Deleted workout: %s", input.ID),
	}, nil
}

func (s *Server) handleGetLatest(ctx context.Context, req *mcp.CallToolRequest, input getLatestInput) (*mcp.CallToolResult, any, error) {
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

	return nil, results, nil
}
