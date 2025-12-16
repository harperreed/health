// ABOUTME: Syncer wraps vault operations for health metrics sync.
// ABOUTME: Handles encryption, queueing, and applying changes for metrics/workouts.
package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/harperreed/health/internal/models"
	"suitesync/vault"
)

// Syncer manages sync operations for health data.
type Syncer struct {
	config *Config
	store  *vault.Store
	keys   vault.Keys
	client *vault.Client
	appDB  *sql.DB
}

// MetricPayload is the sync payload for a health metric.
type MetricPayload struct {
	ID         string  `json:"id"`
	MetricType string  `json:"metric_type"`
	Value      float64 `json:"value"`
	Unit       string  `json:"unit"`
	RecordedAt string  `json:"recorded_at"`
	Notes      *string `json:"notes,omitempty"`
}

// WorkoutPayload is the sync payload for a workout.
type WorkoutPayload struct {
	ID              string  `json:"id"`
	WorkoutType     string  `json:"workout_type"`
	StartedAt       string  `json:"started_at"`
	DurationMinutes *int    `json:"duration_minutes,omitempty"`
	Notes           *string `json:"notes,omitempty"`
}

// WorkoutMetricPayload is the sync payload for a workout metric.
type WorkoutMetricPayload struct {
	ID         string  `json:"id"`
	WorkoutID  string  `json:"workout_id"`
	MetricName string  `json:"metric_name"`
	Value      float64 `json:"value"`
	Unit       *string `json:"unit,omitempty"`
}

// NewSyncer creates a new Syncer with the given config and app database.
func NewSyncer(cfg *Config, appDB *sql.DB) (*Syncer, error) {
	if cfg.DerivedKey == "" {
		return nil, fmt.Errorf("derived key not configured - run 'health sync login'")
	}

	// DerivedKey is stored as hex-encoded seed or mnemonic
	seed, err := vault.ParseSeedPhrase(cfg.DerivedKey)
	if err != nil {
		return nil, fmt.Errorf("invalid derived key: %w", err)
	}

	keys, err := vault.DeriveKeys(seed, "", vault.DefaultKDFParams())
	if err != nil {
		return nil, fmt.Errorf("derive keys: %w", err)
	}

	// Ensure vault DB directory exists
	if err := os.MkdirAll(filepath.Dir(cfg.VaultDB), 0750); err != nil {
		return nil, fmt.Errorf("create vault db directory: %w", err)
	}

	store, err := vault.OpenStore(cfg.VaultDB)
	if err != nil {
		return nil, fmt.Errorf("open vault store: %w", err)
	}

	var tokenExpires time.Time
	if cfg.TokenExpires != "" {
		tokenExpires, _ = time.Parse(time.RFC3339, cfg.TokenExpires)
	}

	client := vault.NewClient(vault.SyncConfig{
		BaseURL:      cfg.Server,
		DeviceID:     cfg.DeviceID,
		AuthToken:    cfg.Token,
		RefreshToken: cfg.RefreshToken,
		TokenExpires: tokenExpires,
		OnTokenRefresh: func(token, refreshToken string, expires time.Time) {
			// Update config with refreshed tokens
			cfg.Token = token
			cfg.RefreshToken = refreshToken
			cfg.TokenExpires = expires.Format(time.RFC3339)
			_ = SaveConfig(cfg)
		},
	})

	return &Syncer{
		config: cfg,
		store:  store,
		keys:   keys,
		client: client,
		appDB:  appDB,
	}, nil
}

// Close closes the vault store.
func (s *Syncer) Close() error {
	return s.store.Close()
}

// QueueMetricChange queues a metric change for sync.
func (s *Syncer) QueueMetricChange(ctx context.Context, m *models.Metric, op vault.Op) error {
	payload := MetricPayload{
		ID:         m.ID.String(),
		MetricType: string(m.MetricType),
		Value:      m.Value,
		Unit:       m.Unit,
		RecordedAt: m.RecordedAt.Format(time.RFC3339),
		Notes:      m.Notes,
	}

	change, err := vault.NewChange("metric", m.ID.String(), op, payload)
	if err != nil {
		return fmt.Errorf("create change: %w", err)
	}

	return s.enqueueChange(ctx, change)
}

// QueueWorkoutChange queues a workout change for sync.
func (s *Syncer) QueueWorkoutChange(ctx context.Context, w *models.Workout, op vault.Op) error {
	payload := WorkoutPayload{
		ID:              w.ID.String(),
		WorkoutType:     w.WorkoutType,
		StartedAt:       w.StartedAt.Format(time.RFC3339),
		DurationMinutes: w.DurationMinutes,
		Notes:           w.Notes,
	}

	change, err := vault.NewChange("workout", w.ID.String(), op, payload)
	if err != nil {
		return fmt.Errorf("create change: %w", err)
	}

	return s.enqueueChange(ctx, change)
}

// QueueWorkoutMetricChange queues a workout metric change for sync.
func (s *Syncer) QueueWorkoutMetricChange(ctx context.Context, wm *models.WorkoutMetric, op vault.Op) error {
	payload := WorkoutMetricPayload{
		ID:         wm.ID.String(),
		WorkoutID:  wm.WorkoutID.String(),
		MetricName: wm.MetricName,
		Value:      wm.Value,
		Unit:       wm.Unit,
	}

	change, err := vault.NewChange("workout_metric", wm.ID.String(), op, payload)
	if err != nil {
		return fmt.Errorf("create change: %w", err)
	}

	return s.enqueueChange(ctx, change)
}

// enqueueChange encrypts and queues a change.
func (s *Syncer) enqueueChange(ctx context.Context, change vault.Change) error {
	userID := s.config.UserID
	aad := change.AAD(userID, s.config.DeviceID)
	plaintext, err := json.Marshal(change)
	if err != nil {
		return fmt.Errorf("marshal change: %w", err)
	}

	env, err := vault.Encrypt(s.keys.EncKey, plaintext, aad)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	if err := s.store.EnqueueEncryptedChange(ctx, change, userID, s.config.DeviceID, env); err != nil {
		return err
	}

	// Auto-sync if enabled
	if s.config.AutoSync {
		return s.Sync(ctx)
	}
	return nil
}

// Sync pushes local changes and pulls remote updates.
func (s *Syncer) Sync(ctx context.Context) error {
	// Ensure token is valid before syncing
	if err := s.client.EnsureValidToken(ctx); err != nil {
		return fmt.Errorf("token expired - run 'health sync login': %w", err)
	}

	return vault.Sync(ctx, s.store, s.client, s.keys, s.config.UserID, s.applyChange)
}

// SyncWithEvents syncs with progress callbacks.
func (s *Syncer) SyncWithEvents(ctx context.Context, events *vault.SyncEvents) error {
	if err := s.client.EnsureValidToken(ctx); err != nil {
		return fmt.Errorf("token expired - run 'health sync login': %w", err)
	}

	return vault.Sync(ctx, s.store, s.client, s.keys, s.config.UserID, s.applyChange, events)
}

// Status returns the current sync status.
func (s *Syncer) Status(ctx context.Context) (vault.SyncStatus, error) {
	return s.store.SyncStatus(ctx)
}

// Health checks server connectivity.
func (s *Syncer) Health(ctx context.Context) vault.HealthStatus {
	return s.client.Health(ctx)
}

// applyChange applies a decrypted change to the local database.
func (s *Syncer) applyChange(ctx context.Context, c vault.Change) error {
	switch c.Entity {
	case "metric":
		return s.applyMetricChange(ctx, c)
	case "workout":
		return s.applyWorkoutChange(ctx, c)
	case "workout_metric":
		return s.applyWorkoutMetricChange(ctx, c)
	default:
		// Unknown entity - skip for forward compatibility
		return nil
	}
}

func (s *Syncer) applyMetricChange(ctx context.Context, c vault.Change) error {
	var payload MetricPayload
	if err := json.Unmarshal(c.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal metric payload: %w", err)
	}

	switch c.Op {
	case vault.OpUpsert:
		recordedAt, err := time.Parse(time.RFC3339, payload.RecordedAt)
		if err != nil {
			return fmt.Errorf("parse recorded_at: %w", err)
		}

		_, err = s.appDB.ExecContext(ctx, `
			INSERT INTO metrics (id, metric_type, value, unit, recorded_at, notes, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				metric_type = excluded.metric_type,
				value = excluded.value,
				unit = excluded.unit,
				recorded_at = excluded.recorded_at,
				notes = excluded.notes`,
			payload.ID, payload.MetricType, payload.Value, payload.Unit,
			recordedAt.Format(time.RFC3339), payload.Notes, time.Now().Format(time.RFC3339),
		)
		return err

	case vault.OpDelete:
		_, err := s.appDB.ExecContext(ctx, `DELETE FROM metrics WHERE id = ?`, payload.ID)
		return err
	}

	return nil
}

func (s *Syncer) applyWorkoutChange(ctx context.Context, c vault.Change) error {
	var payload WorkoutPayload
	if err := json.Unmarshal(c.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal workout payload: %w", err)
	}

	switch c.Op {
	case vault.OpUpsert:
		startedAt, err := time.Parse(time.RFC3339, payload.StartedAt)
		if err != nil {
			return fmt.Errorf("parse started_at: %w", err)
		}

		_, err = s.appDB.ExecContext(ctx, `
			INSERT INTO workouts (id, workout_type, started_at, duration_minutes, notes, created_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				workout_type = excluded.workout_type,
				started_at = excluded.started_at,
				duration_minutes = excluded.duration_minutes,
				notes = excluded.notes`,
			payload.ID, payload.WorkoutType, startedAt.Format(time.RFC3339),
			payload.DurationMinutes, payload.Notes, time.Now().Format(time.RFC3339),
		)
		return err

	case vault.OpDelete:
		// Delete workout metrics first, then workout
		_, _ = s.appDB.ExecContext(ctx, `DELETE FROM workout_metrics WHERE workout_id = ?`, payload.ID)
		_, err := s.appDB.ExecContext(ctx, `DELETE FROM workouts WHERE id = ?`, payload.ID)
		return err
	}

	return nil
}

func (s *Syncer) applyWorkoutMetricChange(ctx context.Context, c vault.Change) error {
	var payload WorkoutMetricPayload
	if err := json.Unmarshal(c.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal workout_metric payload: %w", err)
	}

	// Verify workout exists
	var exists int
	err := s.appDB.QueryRowContext(ctx, `SELECT 1 FROM workouts WHERE id = ?`, payload.WorkoutID).Scan(&exists)
	if err == sql.ErrNoRows {
		// Workout doesn't exist yet - skip this metric (it will come with the workout)
		return nil
	}
	if err != nil {
		return err
	}

	switch c.Op {
	case vault.OpUpsert:
		wID, err := uuid.Parse(payload.WorkoutID)
		if err != nil {
			return fmt.Errorf("parse workout_id: %w", err)
		}

		_, err = s.appDB.ExecContext(ctx, `
			INSERT INTO workout_metrics (id, workout_id, metric_name, value, unit, created_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				metric_name = excluded.metric_name,
				value = excluded.value,
				unit = excluded.unit`,
			payload.ID, wID.String(), payload.MetricName, payload.Value, payload.Unit,
			time.Now().Format(time.RFC3339),
		)
		return err

	case vault.OpDelete:
		_, err := s.appDB.ExecContext(ctx, `DELETE FROM workout_metrics WHERE id = ?`, payload.ID)
		return err
	}

	return nil
}
