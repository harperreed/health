// ABOUTME: Export and import functionality for health data.
// ABOUTME: Supports JSON, YAML, and Markdown export formats.
package storage

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/harperreed/health/internal/models"
	"gopkg.in/yaml.v3"
)

// ExportData represents the full export format for health data.
type ExportData struct {
	Version    string            `json:"version" yaml:"version"`
	ExportedAt time.Time         `json:"exported_at" yaml:"exported_at"`
	Tool       string            `json:"tool" yaml:"tool"`
	Metrics    []*models.Metric  `json:"metrics" yaml:"metrics"`
	Workouts   []*models.Workout `json:"workouts" yaml:"workouts"`
}

// GetAllData retrieves all data for export.
func (d *DB) GetAllData() (*ExportData, error) {
	metrics, err := d.ListMetrics(nil, 0)
	if err != nil {
		return nil, fmt.Errorf("list metrics: %w", err)
	}

	workouts, err := d.ListWorkouts(nil, 0)
	if err != nil {
		return nil, fmt.Errorf("list workouts: %w", err)
	}

	// Populate workout metrics
	for _, w := range workouts {
		wMetrics, err := d.ListWorkoutMetrics(w.ID)
		if err != nil {
			return nil, fmt.Errorf("list workout metrics: %w", err)
		}
		for _, wm := range wMetrics {
			w.Metrics = append(w.Metrics, *wm)
		}
	}

	return &ExportData{
		Version:    "1.0",
		ExportedAt: time.Now(),
		Tool:       "health",
		Metrics:    metrics,
		Workouts:   workouts,
	}, nil
}

// ImportData imports data from an export file.
func (d *DB) ImportData(data *ExportData) error {
	// Import metrics
	for _, m := range data.Metrics {
		if err := d.CreateMetric(m); err != nil {
			return fmt.Errorf("import metric: %w", err)
		}
	}

	// Import workouts and their metrics
	for _, w := range data.Workouts {
		if err := d.CreateWorkout(w); err != nil {
			return fmt.Errorf("import workout: %w", err)
		}
		for _, wm := range w.Metrics {
			wm.WorkoutID = w.ID
			if err := d.AddWorkoutMetric(&wm); err != nil {
				return fmt.Errorf("import workout metric: %w", err)
			}
		}
	}

	return nil
}

// ExportJSON exports all data as JSON.
func (d *DB) ExportJSON() ([]byte, error) {
	data, err := d.GetAllData()
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(data, "", "  ")
}

// ExportYAML exports all data as YAML.
func (d *DB) ExportYAML() ([]byte, error) {
	data, err := d.GetAllData()
	if err != nil {
		return nil, err
	}

	// Convert to YAML-friendly format with metrics grouped by type
	yamlData := struct {
		Version    string                  `yaml:"version"`
		ExportedAt string                  `yaml:"exported_at"`
		Tool       string                  `yaml:"tool"`
		Metrics    map[string][]yamlMetric `yaml:"metrics"`
		Workouts   []yamlWorkout           `yaml:"workouts"`
	}{
		Version:    data.Version,
		ExportedAt: data.ExportedAt.Format(time.RFC3339),
		Tool:       data.Tool,
		Metrics:    make(map[string][]yamlMetric),
		Workouts:   make([]yamlWorkout, 0, len(data.Workouts)),
	}

	// Group metrics by type
	for _, m := range data.Metrics {
		mt := string(m.MetricType)
		ym := yamlMetric{
			ID:         m.ID.String()[:8],
			Value:      m.Value,
			Unit:       m.Unit,
			RecordedAt: m.RecordedAt.Format(time.RFC3339),
		}
		if m.Notes != nil {
			ym.Notes = *m.Notes
		}
		yamlData.Metrics[mt] = append(yamlData.Metrics[mt], ym)
	}

	// Convert workouts
	for _, w := range data.Workouts {
		yw := yamlWorkout{
			ID:        w.ID.String()[:8],
			Type:      w.WorkoutType,
			StartedAt: w.StartedAt.Format(time.RFC3339),
		}
		if w.DurationMinutes != nil {
			yw.DurationMinutes = *w.DurationMinutes
		}
		if w.Notes != nil {
			yw.Notes = *w.Notes
		}
		for _, wm := range w.Metrics {
			ywm := yamlWorkoutMetric{
				Name:  wm.MetricName,
				Value: wm.Value,
			}
			if wm.Unit != nil {
				ywm.Unit = *wm.Unit
			}
			yw.Metrics = append(yw.Metrics, ywm)
		}
		yamlData.Workouts = append(yamlData.Workouts, yw)
	}

	return yaml.Marshal(yamlData)
}

type yamlMetric struct {
	ID         string  `yaml:"id"`
	Value      float64 `yaml:"value"`
	Unit       string  `yaml:"unit"`
	RecordedAt string  `yaml:"recorded_at"`
	Notes      string  `yaml:"notes,omitempty"`
}

type yamlWorkout struct {
	ID              string              `yaml:"id"`
	Type            string              `yaml:"type"`
	StartedAt       string              `yaml:"started_at"`
	DurationMinutes int                 `yaml:"duration_minutes,omitempty"`
	Notes           string              `yaml:"notes,omitempty"`
	Metrics         []yamlWorkoutMetric `yaml:"metrics,omitempty"`
}

type yamlWorkoutMetric struct {
	Name  string  `yaml:"name"`
	Value float64 `yaml:"value"`
	Unit  string  `yaml:"unit,omitempty"`
}

// ExportMarkdown exports data as Markdown.
//
//nolint:gocognit,nestif,gocyclo // This function has clear, linear logic despite complexity metrics.
func (d *DB) ExportMarkdown(metricType *models.MetricType, since *time.Time) (string, error) {
	var metrics []*models.Metric
	var err error

	metrics, err = d.ListMetrics(metricType, 0)
	if err != nil {
		return "", err
	}

	// Filter by since date if provided
	if since != nil {
		var filtered []*models.Metric
		for _, m := range metrics {
			if m.RecordedAt.After(*since) || m.RecordedAt.Equal(*since) {
				filtered = append(filtered, m)
			}
		}
		metrics = filtered
	}

	var sb strings.Builder
	now := time.Now()

	sb.WriteString(fmt.Sprintf("# Health Export - %s\n\n", now.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", now.Format(time.RFC3339)))

	if metricType != nil {
		sb.WriteString(fmt.Sprintf("## %s\n\n", *metricType))
		sb.WriteString("| Date | Value | Notes |\n")
		sb.WriteString("|------|-------|-------|\n")
		for _, m := range metrics {
			notes := ""
			if m.Notes != nil {
				notes = *m.Notes
			}
			sb.WriteString(fmt.Sprintf("| %s | %.2f %s | %s |\n",
				m.RecordedAt.Format("2006-01-02 15:04"),
				m.Value, m.Unit, notes))
		}
	} else {
		// Group by metric type
		grouped := make(map[models.MetricType][]*models.Metric)
		for _, m := range metrics {
			grouped[m.MetricType] = append(grouped[m.MetricType], m)
		}

		// Sort types for consistent output
		var types []models.MetricType
		for t := range grouped {
			types = append(types, t)
		}
		sort.Slice(types, func(i, j int) bool {
			return string(types[i]) < string(types[j])
		})

		for _, t := range types {
			sb.WriteString(fmt.Sprintf("## %s\n\n", t))
			sb.WriteString("| Date | Value | Notes |\n")
			sb.WriteString("|------|-------|-------|\n")
			for _, m := range grouped[t] {
				notes := ""
				if m.Notes != nil {
					notes = *m.Notes
				}
				sb.WriteString(fmt.Sprintf("| %s | %.2f %s | %s |\n",
					m.RecordedAt.Format("2006-01-02 15:04"),
					m.Value, m.Unit, notes))
			}
			sb.WriteString("\n")
		}

		// Add workouts section
		workouts, err := d.ListWorkouts(nil, 0)
		if err == nil && len(workouts) > 0 {
			// Filter by since if provided
			if since != nil {
				var filtered []*models.Workout
				for _, w := range workouts {
					if w.StartedAt.After(*since) || w.StartedAt.Equal(*since) {
						filtered = append(filtered, w)
					}
				}
				workouts = filtered
			}

			if len(workouts) > 0 {
				sb.WriteString("## Workouts\n\n")
				sb.WriteString("| Date | Type | Duration | Notes |\n")
				sb.WriteString("|------|------|----------|-------|\n")
				for _, w := range workouts {
					duration := ""
					if w.DurationMinutes != nil {
						duration = fmt.Sprintf("%d min", *w.DurationMinutes)
					}
					notes := ""
					if w.Notes != nil {
						notes = *w.Notes
					}
					sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
						w.StartedAt.Format("2006-01-02 15:04"),
						w.WorkoutType, duration, notes))
				}
			}
		}
	}

	return sb.String(), nil
}

// ImportJSON imports data from JSON bytes.
func (d *DB) ImportJSON(data []byte) error {
	var exportData ExportData
	if err := json.Unmarshal(data, &exportData); err != nil {
		return fmt.Errorf("unmarshal JSON: %w", err)
	}
	return d.ImportData(&exportData)
}
