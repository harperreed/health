// ABOUTME: Metrics CRUD operations for the health database.
// ABOUTME: Supports create, get (with prefix matching), list, and delete.
package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/harperreed/health/internal/models"
)

// CreateMetric inserts a new metric into the database.
func CreateMetric(db *sql.DB, m *models.Metric) error {
	_, err := db.Exec(`
		INSERT INTO metrics (id, metric_type, value, unit, recorded_at, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.ID.String(), string(m.MetricType), m.Value, m.Unit,
		m.RecordedAt.Format(time.RFC3339), m.Notes,
		m.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("failed to create metric: %w", err)
	}
	return nil
}

// CreateMetricTx inserts a new metric using an existing transaction.
func CreateMetricTx(tx *sql.Tx, m *models.Metric) error {
	_, err := tx.Exec(`
		INSERT INTO metrics (id, metric_type, value, unit, recorded_at, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.ID.String(), string(m.MetricType), m.Value, m.Unit,
		m.RecordedAt.Format(time.RFC3339), m.Notes,
		m.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("failed to create metric: %w", err)
	}
	return nil
}

// GetMetric retrieves a metric by ID or ID prefix.
func GetMetric(db *sql.DB, idOrPrefix string) (*models.Metric, error) {
	var row *sql.Row
	if len(idOrPrefix) < 36 {
		// Prefix match
		row = db.QueryRow(`
			SELECT id, metric_type, value, unit, recorded_at, notes, created_at
			FROM metrics WHERE id LIKE ? LIMIT 1`, idOrPrefix+"%")
	} else {
		row = db.QueryRow(`
			SELECT id, metric_type, value, unit, recorded_at, notes, created_at
			FROM metrics WHERE id = ?`, idOrPrefix)
	}

	return scanMetric(row)
}

// ListMetrics retrieves recent metrics, optionally filtered by type.
func ListMetrics(db *sql.DB, metricType *models.MetricType, limit int) ([]*models.Metric, error) {
	var rows *sql.Rows
	var err error

	if metricType != nil {
		rows, err = db.Query(`
			SELECT id, metric_type, value, unit, recorded_at, notes, created_at
			FROM metrics WHERE metric_type = ?
			ORDER BY recorded_at DESC LIMIT ?`, string(*metricType), limit)
	} else {
		rows, err = db.Query(`
			SELECT id, metric_type, value, unit, recorded_at, notes, created_at
			FROM metrics ORDER BY recorded_at DESC LIMIT ?`, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list metrics: %w", err)
	}
	defer rows.Close()

	var metrics []*models.Metric
	for rows.Next() {
		m, err := scanMetricRows(rows)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}

	return metrics, rows.Err()
}

// DeleteMetric removes a metric by ID or prefix.
func DeleteMetric(db *sql.DB, idOrPrefix string) error {
	var result sql.Result
	var err error

	if len(idOrPrefix) < 36 {
		result, err = db.Exec("DELETE FROM metrics WHERE id LIKE ?", idOrPrefix+"%")
	} else {
		result, err = db.Exec("DELETE FROM metrics WHERE id = ?", idOrPrefix)
	}

	if err != nil {
		return fmt.Errorf("failed to delete metric: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("metric not found: %s", idOrPrefix)
	}

	return nil
}

func scanMetric(row *sql.Row) (*models.Metric, error) {
	var m models.Metric
	var idStr, metricType string
	var recordedAt, createdAt string

	err := row.Scan(&idStr, &metricType, &m.Value, &m.Unit, &recordedAt, &m.Notes, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to scan metric: %w", err)
	}

	m.ID, err = uuid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("invalid metric ID in database: %w", err)
	}
	m.MetricType = models.MetricType(metricType)
	m.RecordedAt, err = time.Parse(time.RFC3339, recordedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid recorded_at timestamp: %w", err)
	}
	m.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("invalid created_at timestamp: %w", err)
	}

	return &m, nil
}

func scanMetricRows(rows *sql.Rows) (*models.Metric, error) {
	var m models.Metric
	var idStr, metricType string
	var recordedAt, createdAt string

	err := rows.Scan(&idStr, &metricType, &m.Value, &m.Unit, &recordedAt, &m.Notes, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to scan metric: %w", err)
	}

	m.ID, err = uuid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("invalid metric ID in database: %w", err)
	}
	m.MetricType = models.MetricType(metricType)
	m.RecordedAt, err = time.Parse(time.RFC3339, recordedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid recorded_at timestamp: %w", err)
	}
	m.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("invalid created_at timestamp: %w", err)
	}

	return &m, nil
}
