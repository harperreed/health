// ABOUTME: Repository interface for health data storage.
// ABOUTME: Defines contract for metrics and workouts CRUD operations.
package storage

import (
	"github.com/google/uuid"
	"github.com/harperreed/health/internal/models"
)

// Repository defines the storage interface for health data.
// This interface allows swapping implementations (e.g., for testing).
type Repository interface {
	// Metric operations
	CreateMetric(m *models.Metric) error
	GetMetric(idOrPrefix string) (*models.Metric, error)
	ListMetrics(metricType *models.MetricType, limit int) ([]*models.Metric, error)
	DeleteMetric(idOrPrefix string) error
	GetLatestMetric(metricType models.MetricType) (*models.Metric, error)

	// Workout operations
	CreateWorkout(w *models.Workout) error
	GetWorkout(idOrPrefix string) (*models.Workout, error)
	GetWorkoutWithMetrics(idOrPrefix string) (*models.Workout, error)
	ListWorkouts(workoutType *string, limit int) ([]*models.Workout, error)
	DeleteWorkout(idOrPrefix string) error

	// Workout metric operations
	AddWorkoutMetric(wm *models.WorkoutMetric) error
	GetWorkoutMetric(idOrPrefix string) (*models.WorkoutMetric, error)
	ListWorkoutMetrics(workoutID uuid.UUID) ([]*models.WorkoutMetric, error)
	DeleteWorkoutMetric(idOrPrefix string) error

	// Export/Import
	GetAllData() (*ExportData, error)
	ImportData(data *ExportData) error

	// Lifecycle
	Close() error
}
