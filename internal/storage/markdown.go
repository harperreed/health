// ABOUTME: Core MarkdownStore struct and helpers for file-based health data storage.
// ABOUTME: Provides constructor, path helpers, and frontmatter types via mdstore library.

package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/harper/suite/mdstore"
	"github.com/harperreed/health/internal/models"
	"gopkg.in/yaml.v3"
)

// MarkdownStore provides file-based storage for health data using markdown files.
type MarkdownStore struct {
	dataDir string
}

// Compile-time check that MarkdownStore implements Repository.
var _ Repository = (*MarkdownStore)(nil)

// NewMarkdownStore creates a new markdown-backed store rooted at dataDir.
func NewMarkdownStore(dataDir string) (*MarkdownStore, error) {
	if err := mdstore.EnsureDir(dataDir); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	return &MarkdownStore{dataDir: dataDir}, nil
}

// Close releases resources. For MarkdownStore this is a no-op.
func (s *MarkdownStore) Close() error {
	return nil
}

// metricsDir returns the path to the metrics directory.
func (s *MarkdownStore) metricsDir() string {
	return filepath.Join(s.dataDir, "metrics")
}

// workoutsDir returns the path to the workouts directory.
func (s *MarkdownStore) workoutsDir() string {
	return filepath.Join(s.dataDir, "workouts")
}

// metricFilePath returns the path for a metric file based on date and type.
// Format: metrics/YYYY/MM/YYYY-MM-DD-<type>-<id_prefix>.md.
func (s *MarkdownStore) metricFilePath(recordedAt time.Time, metricType models.MetricType, id uuid.UUID) string {
	year := recordedAt.Format("2006")
	month := recordedAt.Format("01")
	date := recordedAt.Format("2006-01-02")
	return filepath.Join(s.metricsDir(), year, month,
		fmt.Sprintf("%s-%s-%s.md", date, string(metricType), id.String()[:8]))
}

// workoutFilePath returns the path for a workout file based on date and type.
// Format: workouts/YYYY/MM/YYYY-MM-DD-<type>-<id_prefix>.md.
func (s *MarkdownStore) workoutFilePath(startedAt time.Time, workoutType string, id uuid.UUID) string {
	year := startedAt.Format("2006")
	month := startedAt.Format("01")
	date := startedAt.Format("2006-01-02")
	slug := mdstore.Slugify(workoutType)
	return filepath.Join(s.workoutsDir(), year, month,
		fmt.Sprintf("%s-%s-%s.md", date, slug, id.String()[:8]))
}

// metricFrontmatter holds the YAML frontmatter of a metric file.
type metricFrontmatter struct {
	ID         string  `yaml:"id"`
	MetricType string  `yaml:"metric_type"`
	Value      float64 `yaml:"value"`
	Unit       string  `yaml:"unit"`
	RecordedAt string  `yaml:"recorded_at"`
	CreatedAt  string  `yaml:"created_at"`
}

// workoutFrontmatter holds the YAML frontmatter of a workout file.
type workoutFrontmatter struct {
	ID              string                     `yaml:"id"`
	WorkoutType     string                     `yaml:"workout_type"`
	StartedAt       string                     `yaml:"started_at"`
	DurationMinutes *int                       `yaml:"duration_minutes,omitempty"`
	CreatedAt       string                     `yaml:"created_at"`
	Metrics         []workoutMetricFrontmatter `yaml:"metrics,omitempty"`
}

// workoutMetricFrontmatter holds workout metric data in frontmatter.
type workoutMetricFrontmatter struct {
	ID         string  `yaml:"id"`
	MetricName string  `yaml:"metric_name"`
	Value      float64 `yaml:"value"`
	Unit       string  `yaml:"unit,omitempty"`
	CreatedAt  string  `yaml:"created_at"`
}

// metricFromFrontmatter converts frontmatter to a models.Metric.
func metricFromFrontmatter(fm *metricFrontmatter, notes string) (*models.Metric, error) {
	id, err := uuid.Parse(fm.ID)
	if err != nil {
		return nil, fmt.Errorf("parse metric ID %q: %w", fm.ID, err)
	}
	recordedAt, err := mdstore.ParseTime(fm.RecordedAt)
	if err != nil {
		return nil, fmt.Errorf("parse recorded_at %q: %w", fm.RecordedAt, err)
	}
	createdAt, err := mdstore.ParseTime(fm.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", fm.CreatedAt, err)
	}

	m := &models.Metric{
		ID:         id,
		MetricType: models.MetricType(fm.MetricType),
		Value:      fm.Value,
		Unit:       fm.Unit,
		RecordedAt: recordedAt,
		CreatedAt:  createdAt,
	}
	if notes != "" {
		m.Notes = &notes
	}
	return m, nil
}

// metricToFrontmatter converts a models.Metric to frontmatter.
func metricToFrontmatter(m *models.Metric) metricFrontmatter {
	return metricFrontmatter{
		ID:         m.ID.String(),
		MetricType: string(m.MetricType),
		Value:      m.Value,
		Unit:       m.Unit,
		RecordedAt: mdstore.FormatTime(m.RecordedAt.UTC()),
		CreatedAt:  mdstore.FormatTime(m.CreatedAt.UTC()),
	}
}

// workoutFromFrontmatter converts frontmatter to a models.Workout.
func workoutFromFrontmatter(fm *workoutFrontmatter, notes string) (*models.Workout, error) {
	id, err := uuid.Parse(fm.ID)
	if err != nil {
		return nil, fmt.Errorf("parse workout ID %q: %w", fm.ID, err)
	}
	startedAt, err := mdstore.ParseTime(fm.StartedAt)
	if err != nil {
		return nil, fmt.Errorf("parse started_at %q: %w", fm.StartedAt, err)
	}
	createdAt, err := mdstore.ParseTime(fm.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", fm.CreatedAt, err)
	}

	w := &models.Workout{
		ID:              id,
		WorkoutType:     fm.WorkoutType,
		StartedAt:       startedAt,
		DurationMinutes: fm.DurationMinutes,
		CreatedAt:       createdAt,
	}
	if notes != "" {
		w.Notes = &notes
	}
	return w, nil
}

// workoutToFrontmatter converts a models.Workout to frontmatter.
func workoutToFrontmatter(w *models.Workout) workoutFrontmatter {
	return workoutFrontmatter{
		ID:              w.ID.String(),
		WorkoutType:     w.WorkoutType,
		StartedAt:       mdstore.FormatTime(w.StartedAt.UTC()),
		DurationMinutes: w.DurationMinutes,
		CreatedAt:       mdstore.FormatTime(w.CreatedAt.UTC()),
	}
}

// workoutMetricFromFrontmatter converts frontmatter to a models.WorkoutMetric.
func workoutMetricFromFrontmatter(wmf *workoutMetricFrontmatter, workoutID uuid.UUID) (*models.WorkoutMetric, error) {
	id, err := uuid.Parse(wmf.ID)
	if err != nil {
		return nil, fmt.Errorf("parse workout metric ID %q: %w", wmf.ID, err)
	}
	createdAt, err := mdstore.ParseTime(wmf.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", wmf.CreatedAt, err)
	}

	wm := &models.WorkoutMetric{
		ID:         id,
		WorkoutID:  workoutID,
		MetricName: wmf.MetricName,
		Value:      wmf.Value,
		CreatedAt:  createdAt,
	}
	if wmf.Unit != "" {
		wm.Unit = &wmf.Unit
	}
	return wm, nil
}

// workoutMetricToFrontmatter converts a models.WorkoutMetric to frontmatter.
func workoutMetricToFrontmatter(wm *models.WorkoutMetric) workoutMetricFrontmatter {
	unit := ""
	if wm.Unit != nil {
		unit = *wm.Unit
	}
	return workoutMetricFrontmatter{
		ID:         wm.ID.String(),
		MetricName: wm.MetricName,
		Value:      wm.Value,
		Unit:       unit,
		CreatedAt:  mdstore.FormatTime(wm.CreatedAt.UTC()),
	}
}

// readMetricFile reads a metric from a markdown file.
func readMetricFile(path string) (*models.Metric, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	yamlStr, body := mdstore.ParseFrontmatter(string(data))
	if yamlStr == "" {
		return nil, fmt.Errorf("no frontmatter in %s", path)
	}

	var fm metricFrontmatter
	if err := yaml.Unmarshal([]byte(yamlStr), &fm); err != nil {
		return nil, fmt.Errorf("parse frontmatter in %s: %w", path, err)
	}

	notes := strings.TrimSpace(body)
	return metricFromFrontmatter(&fm, notes)
}

// writeMetricFile writes a metric to a markdown file.
func (s *MarkdownStore) writeMetricFile(m *models.Metric) error {
	fm := metricToFrontmatter(m)
	path := s.metricFilePath(m.RecordedAt, m.MetricType, m.ID)

	body := ""
	if m.Notes != nil && *m.Notes != "" {
		body = "\n" + *m.Notes + "\n"
	}

	content, err := mdstore.RenderFrontmatter(&fm, body)
	if err != nil {
		return fmt.Errorf("render metric file: %w", err)
	}

	return mdstore.AtomicWrite(path, []byte(content))
}

// readWorkoutFile reads a workout from a markdown file.
func readWorkoutFile(path string) (*models.Workout, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	yamlStr, body := mdstore.ParseFrontmatter(string(data))
	if yamlStr == "" {
		return nil, fmt.Errorf("no frontmatter in %s", path)
	}

	var fm workoutFrontmatter
	if err := yaml.Unmarshal([]byte(yamlStr), &fm); err != nil {
		return nil, fmt.Errorf("parse frontmatter in %s: %w", path, err)
	}

	notes := strings.TrimSpace(body)
	w, err := workoutFromFrontmatter(&fm, notes)
	if err != nil {
		return nil, err
	}

	// Parse embedded metrics from frontmatter
	for _, wmf := range fm.Metrics {
		wm, err := workoutMetricFromFrontmatter(&wmf, w.ID)
		if err != nil {
			continue
		}
		w.Metrics = append(w.Metrics, *wm)
	}

	return w, nil
}

// writeWorkoutFile writes a workout (with its metrics) to a markdown file.
func (s *MarkdownStore) writeWorkoutFile(w *models.Workout) error {
	fm := workoutToFrontmatter(w)

	// Include workout metrics in frontmatter
	for _, wm := range w.Metrics {
		fm.Metrics = append(fm.Metrics, workoutMetricToFrontmatter(&wm))
	}

	path := s.workoutFilePath(w.StartedAt, w.WorkoutType, w.ID)

	body := ""
	if w.Notes != nil && *w.Notes != "" {
		body = "\n" + *w.Notes + "\n"
	}

	content, err := mdstore.RenderFrontmatter(&fm, body)
	if err != nil {
		return fmt.Errorf("render workout file: %w", err)
	}

	return mdstore.AtomicWrite(path, []byte(content))
}

// walkMetricFiles walks all metric markdown files and calls fn for each.
func (s *MarkdownStore) walkMetricFiles(fn func(path string, m *models.Metric) error) error {
	metricsDir := s.metricsDir()
	if _, err := os.Stat(metricsDir); os.IsNotExist(err) {
		return nil
	}

	return filepath.Walk(metricsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		m, err := readMetricFile(path)
		if err != nil {
			return fmt.Errorf("read metric file %s: %w", path, err)
		}

		return fn(path, m)
	})
}

// walkWorkoutFiles walks all workout markdown files and calls fn for each.
func (s *MarkdownStore) walkWorkoutFiles(fn func(path string, w *models.Workout) error) error {
	workoutsDir := s.workoutsDir()
	if _, err := os.Stat(workoutsDir); os.IsNotExist(err) {
		return nil
	}

	return filepath.Walk(workoutsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		w, err := readWorkoutFile(path)
		if err != nil {
			return fmt.Errorf("read workout file %s: %w", path, err)
		}

		return fn(path, w)
	})
}

// findMetricFile finds the file path for a metric by ID or prefix.
func (s *MarkdownStore) findMetricFile(idOrPrefix string) (string, *models.Metric, error) {
	isFullUUID := len(idOrPrefix) == 36 && strings.Count(idOrPrefix, "-") == 4

	var foundPath string
	var foundMetric *models.Metric
	matchCount := 0

	err := s.walkMetricFiles(func(path string, m *models.Metric) error {
		idStr := m.ID.String()
		if isFullUUID {
			if idStr == idOrPrefix {
				foundPath = path
				foundMetric = m
				matchCount = 1
				return filepath.SkipAll
			}
		} else {
			if strings.HasPrefix(idStr, idOrPrefix) {
				foundPath = path
				foundMetric = m
				matchCount++
			}
		}
		return nil
	})
	if err != nil {
		return "", nil, err
	}

	if matchCount == 0 {
		return "", nil, fmt.Errorf("not found: %s", idOrPrefix)
	}
	if matchCount > 1 {
		return "", nil, fmt.Errorf("ambiguous prefix %s: matches multiple records", idOrPrefix)
	}

	return foundPath, foundMetric, nil
}

// findWorkoutFile finds the file path for a workout by ID or prefix.
func (s *MarkdownStore) findWorkoutFile(idOrPrefix string) (string, *models.Workout, error) {
	isFullUUID := len(idOrPrefix) == 36 && strings.Count(idOrPrefix, "-") == 4

	var foundPath string
	var foundWorkout *models.Workout
	matchCount := 0

	err := s.walkWorkoutFiles(func(path string, w *models.Workout) error {
		idStr := w.ID.String()
		if isFullUUID {
			if idStr == idOrPrefix {
				foundPath = path
				foundWorkout = w
				matchCount = 1
				return filepath.SkipAll
			}
		} else {
			if strings.HasPrefix(idStr, idOrPrefix) {
				foundPath = path
				foundWorkout = w
				matchCount++
			}
		}
		return nil
	})
	if err != nil {
		return "", nil, err
	}

	if matchCount == 0 {
		return "", nil, fmt.Errorf("not found: %s", idOrPrefix)
	}
	if matchCount > 1 {
		return "", nil, fmt.Errorf("ambiguous prefix %s: matches multiple records", idOrPrefix)
	}

	return foundPath, foundWorkout, nil
}

// --- Repository interface methods ---

// CreateMetric stores a new metric as a markdown file.
func (s *MarkdownStore) CreateMetric(m *models.Metric) error {
	return s.writeMetricFile(m)
}

// GetMetric retrieves a metric by ID or ID prefix.
func (s *MarkdownStore) GetMetric(idOrPrefix string) (*models.Metric, error) {
	_, m, err := s.findMetricFile(idOrPrefix)
	return m, err
}

// ListMetrics retrieves metrics with optional filtering by type.
// Results are sorted by RecordedAt descending (most recent first).
func (s *MarkdownStore) ListMetrics(metricType *models.MetricType, limit int) ([]*models.Metric, error) {
	var metrics []*models.Metric

	err := s.walkMetricFiles(func(path string, m *models.Metric) error {
		if metricType != nil && m.MetricType != *metricType {
			return nil
		}
		metrics = append(metrics, m)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list metrics: %w", err)
	}

	// Sort by RecordedAt descending
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].RecordedAt.After(metrics[j].RecordedAt)
	})

	if limit > 0 && len(metrics) > limit {
		metrics = metrics[:limit]
	}

	return metrics, nil
}

// DeleteMetric removes a metric file by ID or prefix.
func (s *MarkdownStore) DeleteMetric(idOrPrefix string) error {
	path, _, err := s.findMetricFile(idOrPrefix)
	if err != nil {
		return fmt.Errorf("delete metric: %w", err)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete metric file: %w", err)
	}
	return nil
}

// GetLatestMetric returns the most recent metric of a specific type.
func (s *MarkdownStore) GetLatestMetric(metricType models.MetricType) (*models.Metric, error) {
	mt := metricType
	metrics, err := s.ListMetrics(&mt, 1)
	if err != nil {
		return nil, err
	}
	if len(metrics) == 0 {
		return nil, fmt.Errorf("no metrics of type %s found", metricType)
	}
	return metrics[0], nil
}

// CreateWorkout stores a new workout as a markdown file.
func (s *MarkdownStore) CreateWorkout(w *models.Workout) error {
	return s.writeWorkoutFile(w)
}

// GetWorkout retrieves a workout by ID or ID prefix (without metrics).
func (s *MarkdownStore) GetWorkout(idOrPrefix string) (*models.Workout, error) {
	_, w, err := s.findWorkoutFile(idOrPrefix)
	if err != nil {
		return nil, err
	}
	// Clear metrics for plain GetWorkout
	w.Metrics = nil
	return w, nil
}

// GetWorkoutWithMetrics retrieves a workout with all its associated metrics.
func (s *MarkdownStore) GetWorkoutWithMetrics(idOrPrefix string) (*models.Workout, error) {
	_, w, err := s.findWorkoutFile(idOrPrefix)
	return w, err
}

// ListWorkouts retrieves workouts with optional filtering by type.
// Results are sorted by StartedAt descending (most recent first).
func (s *MarkdownStore) ListWorkouts(workoutType *string, limit int) ([]*models.Workout, error) {
	var workouts []*models.Workout

	err := s.walkWorkoutFiles(func(path string, w *models.Workout) error {
		if workoutType != nil && !strings.EqualFold(w.WorkoutType, *workoutType) {
			return nil
		}
		// Clear metrics for list view
		w.Metrics = nil
		workouts = append(workouts, w)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list workouts: %w", err)
	}

	// Sort by StartedAt descending
	sort.Slice(workouts, func(i, j int) bool {
		return workouts[i].StartedAt.After(workouts[j].StartedAt)
	})

	if limit > 0 && len(workouts) > limit {
		workouts = workouts[:limit]
	}

	return workouts, nil
}

// DeleteWorkout removes a workout file by ID or prefix (cascade deletes metrics).
func (s *MarkdownStore) DeleteWorkout(idOrPrefix string) error {
	path, _, err := s.findWorkoutFile(idOrPrefix)
	if err != nil {
		return fmt.Errorf("delete workout: %w", err)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete workout file: %w", err)
	}
	return nil
}

// AddWorkoutMetric adds a metric to an existing workout by re-writing the workout file.
func (s *MarkdownStore) AddWorkoutMetric(wm *models.WorkoutMetric) error {
	path, w, err := s.findWorkoutFile(wm.WorkoutID.String())
	if err != nil {
		return fmt.Errorf("add workout metric: workout not found: %w", err)
	}

	// Add the new metric to the workout
	w.Metrics = append(w.Metrics, *wm)

	// Rewrite the file
	fm := workoutToFrontmatter(w)
	for _, existing := range w.Metrics {
		fm.Metrics = append(fm.Metrics, workoutMetricToFrontmatter(&existing))
	}

	body := ""
	if w.Notes != nil && *w.Notes != "" {
		body = "\n" + *w.Notes + "\n"
	}

	content, err := mdstore.RenderFrontmatter(&fm, body)
	if err != nil {
		return fmt.Errorf("render workout file: %w", err)
	}

	return mdstore.AtomicWrite(path, []byte(content))
}

// GetWorkoutMetric retrieves a workout metric by ID or ID prefix.
func (s *MarkdownStore) GetWorkoutMetric(idOrPrefix string) (*models.WorkoutMetric, error) {
	isFullUUID := len(idOrPrefix) == 36 && strings.Count(idOrPrefix, "-") == 4

	var found *models.WorkoutMetric
	matchCount := 0

	err := s.walkWorkoutFiles(func(path string, w *models.Workout) error {
		for i := range w.Metrics {
			wm := &w.Metrics[i]
			idStr := wm.ID.String()
			if isFullUUID {
				if idStr == idOrPrefix {
					found = wm
					matchCount = 1
					return filepath.SkipAll
				}
			} else {
				if strings.HasPrefix(idStr, idOrPrefix) {
					found = wm
					matchCount++
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if matchCount == 0 {
		return nil, fmt.Errorf("not found: %s", idOrPrefix)
	}
	if matchCount > 1 {
		return nil, fmt.Errorf("ambiguous prefix %s: matches multiple records", idOrPrefix)
	}

	return found, nil
}

// ListWorkoutMetrics retrieves all workout metrics for a specific workout.
func (s *MarkdownStore) ListWorkoutMetrics(workoutID uuid.UUID) ([]*models.WorkoutMetric, error) {
	_, w, err := s.findWorkoutFile(workoutID.String())
	if err != nil {
		return nil, fmt.Errorf("list workout metrics: %w", err)
	}

	var metrics []*models.WorkoutMetric
	for i := range w.Metrics {
		metrics = append(metrics, &w.Metrics[i])
	}

	// Sort by CreatedAt ascending
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].CreatedAt.Before(metrics[j].CreatedAt)
	})

	return metrics, nil
}

// DeleteWorkoutMetric removes a workout metric by re-writing the workout file.
func (s *MarkdownStore) DeleteWorkoutMetric(idOrPrefix string) error {
	isFullUUID := len(idOrPrefix) == 36 && strings.Count(idOrPrefix, "-") == 4

	var targetPath string
	var targetWorkout *models.Workout
	var targetIndex = -1
	matchCount := 0

	err := s.walkWorkoutFiles(func(path string, w *models.Workout) error {
		for i := range w.Metrics {
			wm := &w.Metrics[i]
			idStr := wm.ID.String()
			if isFullUUID {
				if idStr == idOrPrefix {
					targetPath = path
					targetWorkout = w
					targetIndex = i
					matchCount = 1
					return filepath.SkipAll
				}
			} else {
				if strings.HasPrefix(idStr, idOrPrefix) {
					targetPath = path
					targetWorkout = w
					targetIndex = i
					matchCount++
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	if matchCount == 0 {
		return fmt.Errorf("not found: %s", idOrPrefix)
	}
	if matchCount > 1 {
		return fmt.Errorf("ambiguous prefix %s: matches multiple records", idOrPrefix)
	}

	// Remove the metric from the slice
	targetWorkout.Metrics = append(targetWorkout.Metrics[:targetIndex], targetWorkout.Metrics[targetIndex+1:]...)

	// Rewrite the file
	fm := workoutToFrontmatter(targetWorkout)
	for _, wm := range targetWorkout.Metrics {
		fm.Metrics = append(fm.Metrics, workoutMetricToFrontmatter(&wm))
	}

	body := ""
	if targetWorkout.Notes != nil && *targetWorkout.Notes != "" {
		body = "\n" + *targetWorkout.Notes + "\n"
	}

	content, err := mdstore.RenderFrontmatter(&fm, body)
	if err != nil {
		return fmt.Errorf("render workout file: %w", err)
	}

	return mdstore.AtomicWrite(targetPath, []byte(content))
}

// GetAllData retrieves all data for export.
func (s *MarkdownStore) GetAllData() (*ExportData, error) {
	metrics, err := s.ListMetrics(nil, 0)
	if err != nil {
		return nil, fmt.Errorf("list metrics: %w", err)
	}

	// Get workouts with their metrics
	var workouts []*models.Workout
	err = s.walkWorkoutFiles(func(path string, w *models.Workout) error {
		workouts = append(workouts, w)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list workouts: %w", err)
	}

	return &ExportData{
		Version:    "1.0",
		ExportedAt: time.Now(),
		Tool:       "health",
		Metrics:    metrics,
		Workouts:   workouts,
	}, nil
}

// ImportData imports data from an export format.
func (s *MarkdownStore) ImportData(data *ExportData) error {
	// Import metrics
	for _, m := range data.Metrics {
		if err := s.CreateMetric(m); err != nil {
			return fmt.Errorf("import metric: %w", err)
		}
	}

	// Import workouts and their metrics
	for _, w := range data.Workouts {
		if err := s.CreateWorkout(w); err != nil {
			return fmt.Errorf("import workout: %w", err)
		}
		for _, wm := range w.Metrics {
			wm.WorkoutID = w.ID
			if err := s.AddWorkoutMetric(&wm); err != nil {
				return fmt.Errorf("import workout metric: %w", err)
			}
		}
	}

	return nil
}
