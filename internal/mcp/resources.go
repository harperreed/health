// ABOUTME: MCP resource implementations for health metrics.
// ABOUTME: Provides health://recent, health://today, and health://summary resources.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/harperreed/health/internal/models"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerResources() {
	// health://recent - Last 10 entries across all metrics
	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "health://recent",
		Name:        "Recent Health Entries",
		Description: "Last 10 health metrics and workouts",
		MIMEType:    "application/json",
	}, s.handleRecentResource)

	// health://today - All metrics logged today
	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "health://today",
		Name:        "Today's Health Data",
		Description: "All health metrics logged today",
		MIMEType:    "application/json",
	}, s.handleTodayResource)

	// health://summary - Dashboard with latest of each metric type + recent workouts
	s.mcpServer.AddResource(&mcp.Resource{
		URI:         "health://summary",
		Name:        "Health Summary Dashboard",
		Description: "Latest value for each metric type plus recent workouts",
		MIMEType:    "application/json",
	}, s.handleSummaryResource)
}

// Resource handlers

func (s *Server) handleRecentResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	// Get last 10 metrics
	metrics, err := s.client.ListMetrics(nil, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to list metrics: %w", err)
	}

	// Get last 5 workouts
	workouts, err := s.client.ListWorkouts(nil, 5)
	if err != nil {
		return nil, fmt.Errorf("failed to list workouts: %w", err)
	}

	result := map[string]interface{}{
		"metrics":  metrics,
		"workouts": workouts,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      "health://recent",
			MIMEType: "application/json",
			Text:     string(data),
		}},
	}, nil
}

func (s *Server) handleTodayResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	// Get today's start time (midnight)
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Get all metrics and filter by today
	metrics, err := s.client.ListMetrics(nil, 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to list metrics: %w", err)
	}

	var todayMetrics []*models.Metric
	for _, m := range metrics {
		if m.RecordedAt.After(todayStart) || m.RecordedAt.Equal(todayStart) {
			todayMetrics = append(todayMetrics, m)
		}
	}

	// Get all workouts and filter by today
	workouts, err := s.client.ListWorkouts(nil, 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to list workouts: %w", err)
	}

	var todayWorkouts []*models.Workout
	for _, w := range workouts {
		if w.StartedAt.After(todayStart) || w.StartedAt.Equal(todayStart) {
			todayWorkouts = append(todayWorkouts, w)
		}
	}

	result := map[string]interface{}{
		"date":     todayStart.Format("2006-01-02"),
		"metrics":  todayMetrics,
		"workouts": todayWorkouts,
		"counts": map[string]int{
			"metrics":  len(todayMetrics),
			"workouts": len(todayWorkouts),
		},
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      "health://today",
			MIMEType: "application/json",
			Text:     string(data),
		}},
	}, nil
}

func (s *Server) handleSummaryResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	// Get latest value for each metric type
	latestMetrics := make(map[string]interface{})
	for _, mt := range models.AllMetricTypes {
		metrics, err := s.client.ListMetrics(&mt, 1)
		if err == nil && len(metrics) > 0 {
			m := metrics[0]
			latestMetrics[string(mt)] = map[string]interface{}{
				"value":       m.Value,
				"unit":        m.Unit,
				"recorded_at": m.RecordedAt.Format(time.RFC3339),
				"notes":       m.Notes,
			}
		}
	}

	// Get recent workouts (last 10)
	workouts, err := s.client.ListWorkouts(nil, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to list workouts: %w", err)
	}

	// Organize metrics by category
	biometrics := make(map[string]interface{})
	activity := make(map[string]interface{})
	nutrition := make(map[string]interface{})
	mental := make(map[string]interface{})

	biometricTypes := []models.MetricType{
		models.MetricWeight, models.MetricBodyFat, models.MetricBPSys,
		models.MetricBPDia, models.MetricHeartRate, models.MetricHRV,
		models.MetricTemperature,
	}
	activityTypes := []models.MetricType{
		models.MetricSteps, models.MetricSleepHours, models.MetricActiveCalories,
	}
	nutritionTypes := []models.MetricType{
		models.MetricWater, models.MetricCalories, models.MetricProtein,
		models.MetricCarbs, models.MetricFat,
	}
	mentalTypes := []models.MetricType{
		models.MetricMood, models.MetricEnergy, models.MetricStress,
		models.MetricAnxiety, models.MetricFocus, models.MetricMeditation,
	}

	for _, mt := range biometricTypes {
		if val, ok := latestMetrics[string(mt)]; ok {
			biometrics[string(mt)] = val
		}
	}
	for _, mt := range activityTypes {
		if val, ok := latestMetrics[string(mt)]; ok {
			activity[string(mt)] = val
		}
	}
	for _, mt := range nutritionTypes {
		if val, ok := latestMetrics[string(mt)]; ok {
			nutrition[string(mt)] = val
		}
	}
	for _, mt := range mentalTypes {
		if val, ok := latestMetrics[string(mt)]; ok {
			mental[string(mt)] = val
		}
	}

	result := map[string]interface{}{
		"generated_at": time.Now().Format(time.RFC3339),
		"metrics": map[string]interface{}{
			"biometrics": biometrics,
			"activity":   activity,
			"nutrition":  nutrition,
			"mental":     mental,
		},
		"recent_workouts": workouts,
		"summary": map[string]int{
			"total_metric_types":   len(latestMetrics),
			"recent_workout_count": len(workouts),
		},
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      "health://summary",
			MIMEType: "application/json",
			Text:     string(data),
		}},
	}, nil
}
