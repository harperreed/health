// ABOUTME: Tests for vault sync integration in health.
// ABOUTME: Verifies change queuing, syncing, and applying changes for metrics/workouts.

package sync

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/harperreed/health/internal/models"
	"github.com/harperreed/sweet/vault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSyncer(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test app database
	appDB := setupTestDB(t, tmpDir)
	defer func() { _ = appDB.Close() }()

	// Create seed and derive key
	seed, phrase, err := vault.NewSeedPhrase()
	require.NoError(t, err)

	cfg := &Config{
		Server:     "https://test.example.com",
		UserID:     "test-user",
		Token:      "test-token",
		DerivedKey: phrase,
		DeviceID:   "test-device",
		VaultDB:    filepath.Join(tmpDir, "vault.db"),
		AutoSync:   false,
	}

	syncer, err := NewSyncer(cfg, appDB)
	require.NoError(t, err)
	require.NotNil(t, syncer)
	defer func() { _ = syncer.Close() }()

	assert.Equal(t, cfg, syncer.config)
	assert.NotNil(t, syncer.store)
	assert.NotNil(t, syncer.client)
	assert.NotNil(t, syncer.keys)

	// Verify keys were derived correctly
	expectedKeys, err := vault.DeriveKeys(seed, "", vault.DefaultKDFParams())
	require.NoError(t, err)
	assert.Equal(t, expectedKeys.EncKey, syncer.keys.EncKey)
}

func TestNewSyncerNoDerivedKey(t *testing.T) {
	tmpDir := t.TempDir()

	appDB := setupTestDB(t, tmpDir)
	defer func() { _ = appDB.Close() }()

	cfg := &Config{
		Server:   "https://test.example.com",
		DeviceID: "test-device",
		VaultDB:  filepath.Join(tmpDir, "vault.db"),
	}

	_, err := NewSyncer(cfg, appDB)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "derived key not configured")
}

func TestNewSyncerInvalidDerivedKey(t *testing.T) {
	tmpDir := t.TempDir()

	appDB := setupTestDB(t, tmpDir)
	defer func() { _ = appDB.Close() }()

	cfg := &Config{
		Server:     "https://test.example.com",
		UserID:     "test-user",
		Token:      "test-token",
		DerivedKey: "invalid-key-format",
		DeviceID:   "test-device",
		VaultDB:    filepath.Join(tmpDir, "vault.db"),
	}

	_, err := NewSyncer(cfg, appDB)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid derived key")
}

func TestQueueMetricChange(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)

	metric := models.NewMetric(models.MetricWeight, 75.5)
	metric.Notes = strPtr("Morning weigh-in")

	// Queue metric create
	err := syncer.QueueMetricChange(ctx, metric, vault.OpUpsert)
	require.NoError(t, err)

	// Verify change was queued
	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestQueueMetricDelete(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)

	metric := models.NewMetric(models.MetricWeight, 75.5)

	// Queue metric delete
	err := syncer.QueueMetricChange(ctx, metric, vault.OpDelete)
	require.NoError(t, err)

	// Verify change was queued
	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestQueueWorkoutChange(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)

	workout := models.NewWorkout("running")
	workout.DurationMinutes = intPtr(30)
	workout.Notes = strPtr("Morning run")

	// Queue workout create
	err := syncer.QueueWorkoutChange(ctx, workout, vault.OpUpsert)
	require.NoError(t, err)

	// Verify change was queued
	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestQueueWorkoutDelete(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)

	workout := models.NewWorkout("running")

	// Queue workout delete
	err := syncer.QueueWorkoutChange(ctx, workout, vault.OpDelete)
	require.NoError(t, err)

	// Verify change was queued
	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestQueueWorkoutMetricChange(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)

	workoutID := uuid.New()
	workoutMetric := models.NewWorkoutMetric(workoutID, "distance", 5.2, "km")

	// Queue workout metric create
	err := syncer.QueueWorkoutMetricChange(ctx, workoutMetric, vault.OpUpsert)
	require.NoError(t, err)

	// Verify change was queued
	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestQueueWorkoutMetricDelete(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)

	workoutID := uuid.New()
	workoutMetric := models.NewWorkoutMetric(workoutID, "distance", 5.2, "km")

	// Queue workout metric delete
	err := syncer.QueueWorkoutMetricChange(ctx, workoutMetric, vault.OpDelete)
	require.NoError(t, err)

	// Verify change was queued
	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestMultipleChanges(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)

	// Queue multiple different entity types
	metric := models.NewMetric(models.MetricWeight, 75.5)
	err := syncer.QueueMetricChange(ctx, metric, vault.OpUpsert)
	require.NoError(t, err)

	workout := models.NewWorkout("running")
	err = syncer.QueueWorkoutChange(ctx, workout, vault.OpUpsert)
	require.NoError(t, err)

	workoutMetric := models.NewWorkoutMetric(workout.ID, "distance", 5.2, "km")
	err = syncer.QueueWorkoutMetricChange(ctx, workoutMetric, vault.OpUpsert)
	require.NoError(t, err)

	// Verify all changes were queued
	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestAutoSyncDisabled(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)

	// AutoSync is disabled by default in test setup
	assert.False(t, syncer.config.AutoSync)

	metric := models.NewMetric(models.MetricWeight, 75.5)
	err := syncer.QueueMetricChange(ctx, metric, vault.OpUpsert)
	require.NoError(t, err)

	// Change should be queued but not synced (still pending)
	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestApplyMetricChangeUpsert(t *testing.T) {
	ctx := context.Background()
	syncer, appDB := setupTestSyncerWithDB(t)

	// Create metric payload
	metric := models.NewMetric(models.MetricWeight, 75.5)
	metric.Notes = strPtr("Morning weigh-in")

	payload := MetricPayload{
		ID:         metric.ID.String(),
		MetricType: string(metric.MetricType),
		Value:      metric.Value,
		Unit:       metric.Unit,
		RecordedAt: metric.RecordedAt.Format(time.RFC3339),
		Notes:      metric.Notes,
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	change := vault.Change{
		Entity:   "metric",
		EntityID: metric.ID.String(),
		Op:       vault.OpUpsert,
		Payload:  payloadBytes,
	}

	// Apply change
	err = syncer.applyMetricChange(ctx, change)
	require.NoError(t, err)

	// Verify metric was inserted
	var count int
	err = appDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM metrics WHERE id = ?", metric.ID.String()).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify metric data
	var (
		metricType string
		value      float64
		unit       string
		notes      *string
	)
	err = appDB.QueryRowContext(ctx,
		"SELECT metric_type, value, unit, notes FROM metrics WHERE id = ?",
		metric.ID.String()).Scan(&metricType, &value, &unit, &notes)
	require.NoError(t, err)
	assert.Equal(t, string(models.MetricWeight), metricType)
	assert.Equal(t, 75.5, value)
	assert.Equal(t, "kg", unit)
	require.NotNil(t, notes)
	assert.Equal(t, "Morning weigh-in", *notes)
}

func TestApplyMetricChangeUpdate(t *testing.T) {
	ctx := context.Background()
	syncer, appDB := setupTestSyncerWithDB(t)

	// Insert initial metric
	metric := models.NewMetric(models.MetricWeight, 75.5)
	_, err := appDB.ExecContext(ctx, `
		INSERT INTO metrics (id, metric_type, value, unit, recorded_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		metric.ID.String(), metric.MetricType, metric.Value, metric.Unit,
		metric.RecordedAt.Format(time.RFC3339), time.Now().Format(time.RFC3339))
	require.NoError(t, err)

	// Update with different value and notes
	metric.Value = 76.0
	metric.Notes = strPtr("Evening weigh-in")

	payload := MetricPayload{
		ID:         metric.ID.String(),
		MetricType: string(metric.MetricType),
		Value:      metric.Value,
		Unit:       metric.Unit,
		RecordedAt: metric.RecordedAt.Format(time.RFC3339),
		Notes:      metric.Notes,
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	change := vault.Change{
		Entity:   "metric",
		EntityID: metric.ID.String(),
		Op:       vault.OpUpsert,
		Payload:  payloadBytes,
	}

	// Apply update
	err = syncer.applyMetricChange(ctx, change)
	require.NoError(t, err)

	// Verify only one metric exists
	var count int
	err = appDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM metrics WHERE id = ?", metric.ID.String()).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify updated values
	var value float64
	var notes *string
	err = appDB.QueryRowContext(ctx,
		"SELECT value, notes FROM metrics WHERE id = ?",
		metric.ID.String()).Scan(&value, &notes)
	require.NoError(t, err)
	assert.Equal(t, 76.0, value)
	require.NotNil(t, notes)
	assert.Equal(t, "Evening weigh-in", *notes)
}

func TestApplyMetricChangeDelete(t *testing.T) {
	ctx := context.Background()
	syncer, appDB := setupTestSyncerWithDB(t)

	// Insert metric
	metric := models.NewMetric(models.MetricWeight, 75.5)
	_, err := appDB.ExecContext(ctx, `
		INSERT INTO metrics (id, metric_type, value, unit, recorded_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		metric.ID.String(), metric.MetricType, metric.Value, metric.Unit,
		metric.RecordedAt.Format(time.RFC3339), time.Now().Format(time.RFC3339))
	require.NoError(t, err)

	// Verify it exists
	var count int
	err = appDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM metrics WHERE id = ?", metric.ID.String()).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Delete metric
	payload := MetricPayload{ID: metric.ID.String()}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	change := vault.Change{
		Entity:   "metric",
		EntityID: metric.ID.String(),
		Op:       vault.OpDelete,
		Payload:  payloadBytes,
	}

	err = syncer.applyMetricChange(ctx, change)
	require.NoError(t, err)

	// Verify deletion
	err = appDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM metrics WHERE id = ?", metric.ID.String()).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestApplyWorkoutChangeUpsert(t *testing.T) {
	ctx := context.Background()
	syncer, appDB := setupTestSyncerWithDB(t)

	// Create workout payload
	workout := models.NewWorkout("running")
	workout.DurationMinutes = intPtr(30)
	workout.Notes = strPtr("Morning run")

	payload := WorkoutPayload{
		ID:              workout.ID.String(),
		WorkoutType:     workout.WorkoutType,
		StartedAt:       workout.StartedAt.Format(time.RFC3339),
		DurationMinutes: workout.DurationMinutes,
		Notes:           workout.Notes,
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	change := vault.Change{
		Entity:   "workout",
		EntityID: workout.ID.String(),
		Op:       vault.OpUpsert,
		Payload:  payloadBytes,
	}

	// Apply change
	err = syncer.applyWorkoutChange(ctx, change)
	require.NoError(t, err)

	// Verify workout was inserted
	var count int
	err = appDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM workouts WHERE id = ?", workout.ID.String()).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify workout data
	var (
		workoutType     string
		durationMinutes *int
		notes           *string
	)
	err = appDB.QueryRowContext(ctx,
		"SELECT workout_type, duration_minutes, notes FROM workouts WHERE id = ?",
		workout.ID.String()).Scan(&workoutType, &durationMinutes, &notes)
	require.NoError(t, err)
	assert.Equal(t, "running", workoutType)
	require.NotNil(t, durationMinutes)
	assert.Equal(t, 30, *durationMinutes)
	require.NotNil(t, notes)
	assert.Equal(t, "Morning run", *notes)
}

func TestApplyWorkoutChangeDelete(t *testing.T) {
	ctx := context.Background()
	syncer, appDB := setupTestSyncerWithDB(t)

	// Insert workout
	workout := models.NewWorkout("running")
	_, err := appDB.ExecContext(ctx, `
		INSERT INTO workouts (id, workout_type, started_at, created_at)
		VALUES (?, ?, ?, ?)`,
		workout.ID.String(), workout.WorkoutType,
		workout.StartedAt.Format(time.RFC3339), time.Now().Format(time.RFC3339))
	require.NoError(t, err)

	// Insert workout metric
	workoutMetric := models.NewWorkoutMetric(workout.ID, "distance", 5.2, "km")
	_, err = appDB.ExecContext(ctx, `
		INSERT INTO workout_metrics (id, workout_id, metric_name, value, unit, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		workoutMetric.ID.String(), workoutMetric.WorkoutID.String(),
		workoutMetric.MetricName, workoutMetric.Value, workoutMetric.Unit,
		time.Now().Format(time.RFC3339))
	require.NoError(t, err)

	// Delete workout
	payload := WorkoutPayload{ID: workout.ID.String()}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	change := vault.Change{
		Entity:   "workout",
		EntityID: workout.ID.String(),
		Op:       vault.OpDelete,
		Payload:  payloadBytes,
	}

	err = syncer.applyWorkoutChange(ctx, change)
	require.NoError(t, err)

	// Verify workout was deleted
	var count int
	err = appDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM workouts WHERE id = ?", workout.ID.String()).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Verify workout metrics were deleted (cascade)
	err = appDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM workout_metrics WHERE workout_id = ?", workout.ID.String()).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestApplyWorkoutMetricChangeUpsert(t *testing.T) {
	ctx := context.Background()
	syncer, appDB := setupTestSyncerWithDB(t)

	// Insert workout first
	workout := models.NewWorkout("running")
	_, err := appDB.ExecContext(ctx, `
		INSERT INTO workouts (id, workout_type, started_at, created_at)
		VALUES (?, ?, ?, ?)`,
		workout.ID.String(), workout.WorkoutType,
		workout.StartedAt.Format(time.RFC3339), time.Now().Format(time.RFC3339))
	require.NoError(t, err)

	// Create workout metric payload
	workoutMetric := models.NewWorkoutMetric(workout.ID, "distance", 5.2, "km")

	payload := WorkoutMetricPayload{
		ID:         workoutMetric.ID.String(),
		WorkoutID:  workoutMetric.WorkoutID.String(),
		MetricName: workoutMetric.MetricName,
		Value:      workoutMetric.Value,
		Unit:       workoutMetric.Unit,
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	change := vault.Change{
		Entity:   "workout_metric",
		EntityID: workoutMetric.ID.String(),
		Op:       vault.OpUpsert,
		Payload:  payloadBytes,
	}

	// Apply change
	err = syncer.applyWorkoutMetricChange(ctx, change)
	require.NoError(t, err)

	// Verify workout metric was inserted
	var count int
	err = appDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM workout_metrics WHERE id = ?", workoutMetric.ID.String()).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify workout metric data
	var (
		metricName string
		value      float64
		unit       *string
	)
	err = appDB.QueryRowContext(ctx,
		"SELECT metric_name, value, unit FROM workout_metrics WHERE id = ?",
		workoutMetric.ID.String()).Scan(&metricName, &value, &unit)
	require.NoError(t, err)
	assert.Equal(t, "distance", metricName)
	assert.Equal(t, 5.2, value)
	require.NotNil(t, unit)
	assert.Equal(t, "km", *unit)
}

func TestApplyWorkoutMetricChangeNoWorkout(t *testing.T) {
	ctx := context.Background()
	syncer, appDB := setupTestSyncerWithDB(t)

	// Create workout metric without inserting workout
	workoutID := uuid.New()
	workoutMetric := models.NewWorkoutMetric(workoutID, "distance", 5.2, "km")

	payload := WorkoutMetricPayload{
		ID:         workoutMetric.ID.String(),
		WorkoutID:  workoutMetric.WorkoutID.String(),
		MetricName: workoutMetric.MetricName,
		Value:      workoutMetric.Value,
		Unit:       workoutMetric.Unit,
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	change := vault.Change{
		Entity:   "workout_metric",
		EntityID: workoutMetric.ID.String(),
		Op:       vault.OpUpsert,
		Payload:  payloadBytes,
	}

	// Apply change - should not error, but also not insert
	err = syncer.applyWorkoutMetricChange(ctx, change)
	require.NoError(t, err)

	// Verify workout metric was NOT inserted
	var count int
	err = appDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM workout_metrics WHERE id = ?", workoutMetric.ID.String()).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestApplyWorkoutMetricChangeDelete(t *testing.T) {
	ctx := context.Background()
	syncer, appDB := setupTestSyncerWithDB(t)

	// Insert workout
	workout := models.NewWorkout("running")
	_, err := appDB.ExecContext(ctx, `
		INSERT INTO workouts (id, workout_type, started_at, created_at)
		VALUES (?, ?, ?, ?)`,
		workout.ID.String(), workout.WorkoutType,
		workout.StartedAt.Format(time.RFC3339), time.Now().Format(time.RFC3339))
	require.NoError(t, err)

	// Insert workout metric
	workoutMetric := models.NewWorkoutMetric(workout.ID, "distance", 5.2, "km")
	_, err = appDB.ExecContext(ctx, `
		INSERT INTO workout_metrics (id, workout_id, metric_name, value, unit, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		workoutMetric.ID.String(), workoutMetric.WorkoutID.String(),
		workoutMetric.MetricName, workoutMetric.Value, workoutMetric.Unit,
		time.Now().Format(time.RFC3339))
	require.NoError(t, err)

	// Delete workout metric
	payload := WorkoutMetricPayload{ID: workoutMetric.ID.String()}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	change := vault.Change{
		Entity:   "workout_metric",
		EntityID: workoutMetric.ID.String(),
		Op:       vault.OpDelete,
		Payload:  payloadBytes,
	}

	err = syncer.applyWorkoutMetricChange(ctx, change)
	require.NoError(t, err)

	// Verify deletion
	var count int
	err = appDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM workout_metrics WHERE id = ?", workoutMetric.ID.String()).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestApplyChangeUnknownEntity(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)

	// Create change with unknown entity type
	change := vault.Change{
		Entity:   "unknown_entity",
		EntityID: uuid.New().String(),
		Op:       vault.OpUpsert,
		Payload:  []byte(`{"test":"data"}`),
	}

	// Should not error (forward compatibility)
	err := syncer.applyChange(ctx, change)
	require.NoError(t, err)
}

func TestStatusInitial(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)

	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)

	assert.Equal(t, 0, count)
}

func TestStatusAfterQueuing(t *testing.T) {
	ctx := context.Background()
	syncer := setupTestSyncer(t)

	// Queue some changes
	metric := models.NewMetric(models.MetricWeight, 75.5)
	err := syncer.QueueMetricChange(ctx, metric, vault.OpUpsert)
	require.NoError(t, err)

	workout := models.NewWorkout("running")
	err = syncer.QueueWorkoutChange(ctx, workout, vault.OpUpsert)
	require.NoError(t, err)

	// Check pending count
	count, err := syncer.PendingCount(ctx)
	require.NoError(t, err)

	assert.Equal(t, 2, count)
}

func TestDifferentMetricTypes(t *testing.T) {
	ctx := context.Background()
	syncer, appDB := setupTestSyncerWithDB(t)

	testCases := []struct {
		name       string
		metricType models.MetricType
		value      float64
		unit       string
	}{
		{"Weight", models.MetricWeight, 75.5, "kg"},
		{"Body Fat", models.MetricBodyFat, 18.5, "%"},
		{"Blood Pressure Sys", models.MetricBPSys, 120.0, "mmHg"},
		{"Heart Rate", models.MetricHeartRate, 72.0, "bpm"},
		{"Steps", models.MetricSteps, 10000.0, "steps"},
		{"Sleep Hours", models.MetricSleepHours, 7.5, "hours"},
		{"Water", models.MetricWater, 2000.0, "ml"},
		{"Mood", models.MetricMood, 8.0, "scale"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metric := models.NewMetric(tc.metricType, tc.value)

			payload := MetricPayload{
				ID:         metric.ID.String(),
				MetricType: string(metric.MetricType),
				Value:      metric.Value,
				Unit:       metric.Unit,
				RecordedAt: metric.RecordedAt.Format(time.RFC3339),
			}

			payloadBytes, err := json.Marshal(payload)
			require.NoError(t, err)

			change := vault.Change{
				Entity:   "metric",
				EntityID: metric.ID.String(),
				Op:       vault.OpUpsert,
				Payload:  payloadBytes,
			}

			// Apply change
			err = syncer.applyMetricChange(ctx, change)
			require.NoError(t, err)

			// Verify metric
			var metricType, unit string
			var value float64
			err = appDB.QueryRowContext(ctx,
				"SELECT metric_type, value, unit FROM metrics WHERE id = ?",
				metric.ID.String()).Scan(&metricType, &value, &unit)
			require.NoError(t, err)
			assert.Equal(t, string(tc.metricType), metricType)
			assert.Equal(t, tc.value, value)
			assert.Equal(t, tc.unit, unit)
		})
	}
}
