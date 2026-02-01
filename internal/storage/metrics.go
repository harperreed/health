// ABOUTME: Metric CRUD operations for SQLite storage.
// ABOUTME: Implements Repository interface methods for metrics.
package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/harperreed/health/internal/models"
)

// CreateMetric stores a new metric in the database.
func (d *DB) CreateMetric(m *models.Metric) error {
	query := `
		INSERT INTO metrics (id, metric_type, value, unit, recorded_at, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := d.db.Exec(query,
		m.ID.String(),
		string(m.MetricType),
		m.Value,
		m.Unit,
		m.RecordedAt.Format(time.RFC3339),
		m.Notes,
		m.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create metric: %w", err)
	}
	return nil
}

// GetMetric retrieves a metric by ID or ID prefix.
func (d *DB) GetMetric(idOrPrefix string) (*models.Metric, error) {
	id, err := d.resolveMetricID(idOrPrefix)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, metric_type, value, unit, recorded_at, notes, created_at
		FROM metrics
		WHERE id = ?
	`
	return d.scanMetric(d.db.QueryRow(query, id))
}

// ListMetrics retrieves metrics with optional filtering by type.
// Results are sorted by RecordedAt descending (most recent first).
func (d *DB) ListMetrics(metricType *models.MetricType, limit int) ([]*models.Metric, error) {
	var query string
	var args []interface{}

	if metricType != nil {
		query = `
			SELECT id, metric_type, value, unit, recorded_at, notes, created_at
			FROM metrics
			WHERE metric_type = ?
			ORDER BY recorded_at DESC
		`
		args = append(args, string(*metricType))
	} else {
		query = `
			SELECT id, metric_type, value, unit, recorded_at, notes, created_at
			FROM metrics
			ORDER BY recorded_at DESC
		`
	}

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list metrics: %w", err)
	}
	defer rows.Close()

	return d.scanMetrics(rows)
}

// DeleteMetric removes a metric by ID or prefix.
func (d *DB) DeleteMetric(idOrPrefix string) error {
	id, err := d.resolveMetricID(idOrPrefix)
	if err != nil {
		return fmt.Errorf("delete metric: %w", err)
	}

	result, err := d.db.Exec("DELETE FROM metrics WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete metric: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete metric: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("not found: %s", idOrPrefix)
	}

	return nil
}

// GetLatestMetric returns the most recent metric of a specific type.
func (d *DB) GetLatestMetric(metricType models.MetricType) (*models.Metric, error) {
	query := `
		SELECT id, metric_type, value, unit, recorded_at, notes, created_at
		FROM metrics
		WHERE metric_type = ?
		ORDER BY recorded_at DESC
		LIMIT 1
	`
	m, err := d.scanMetric(d.db.QueryRow(query, string(metricType)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no metrics of type %s found", metricType)
		}
		return nil, err
	}
	return m, nil
}

// resolveMetricID finds the full ID from a prefix.
func (d *DB) resolveMetricID(idOrPrefix string) (string, error) {
	// If it looks like a full UUID, use it directly
	if len(idOrPrefix) == 36 && strings.Count(idOrPrefix, "-") == 4 {
		return idOrPrefix, nil
	}

	// Search by prefix
	query := `SELECT id FROM metrics WHERE id LIKE ? || '%'`
	rows, err := d.db.Query(query, idOrPrefix)
	if err != nil {
		return "", fmt.Errorf("resolve metric ID: %w", err)
	}
	defer rows.Close()

	var matches []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", fmt.Errorf("scan metric ID: %w", err)
		}
		matches = append(matches, id)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("not found: %s", idOrPrefix)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous prefix %s: matches multiple records", idOrPrefix)
	}

	return matches[0], nil
}

// scanMetric scans a single row into a Metric struct.
func (d *DB) scanMetric(row *sql.Row) (*models.Metric, error) {
	var m models.Metric
	var idStr, metricType, recordedAt, createdAt string
	var notes sql.NullString

	err := row.Scan(&idStr, &metricType, &m.Value, &m.Unit, &recordedAt, &notes, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("not found")
		}
		return nil, fmt.Errorf("scan metric: %w", err)
	}

	m.ID, _ = uuid.Parse(idStr)
	m.MetricType = models.MetricType(metricType)
	m.RecordedAt, _ = time.Parse(time.RFC3339, recordedAt)
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if notes.Valid {
		m.Notes = &notes.String
	}

	return &m, nil
}

// scanMetrics scans multiple rows into a slice of Metrics.
func (d *DB) scanMetrics(rows *sql.Rows) ([]*models.Metric, error) {
	var metrics []*models.Metric

	for rows.Next() {
		var m models.Metric
		var idStr, metricType, recordedAt, createdAt string
		var notes sql.NullString

		err := rows.Scan(&idStr, &metricType, &m.Value, &m.Unit, &recordedAt, &notes, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("scan metric: %w", err)
		}

		m.ID, _ = uuid.Parse(idStr)
		m.MetricType = models.MetricType(metricType)
		m.RecordedAt, _ = time.Parse(time.RFC3339, recordedAt)
		m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if notes.Valid {
			m.Notes = &notes.String
		}

		metrics = append(metrics, &m)
	}

	return metrics, rows.Err()
}
