// ABOUTME: Tests for metrics CRUD operations.
// ABOUTME: Validates create, get, list, and delete functions.
package db

import (
	"testing"

	"github.com/harperreed/health/internal/models"
)

func TestCreateAndGetMetric(t *testing.T) {
	db := setupTestDB(t)

	m := models.NewMetric(models.MetricWeight, 82.5)
	if err := CreateMetric(db, m); err != nil {
		t.Fatalf("CreateMetric failed: %v", err)
	}

	got, err := GetMetric(db, m.ID.String())
	if err != nil {
		t.Fatalf("GetMetric failed: %v", err)
	}

	if got.ID != m.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, m.ID)
	}
	if got.Value != 82.5 {
		t.Errorf("Value mismatch: got %f, want 82.5", got.Value)
	}
}

func TestListMetrics(t *testing.T) {
	db := setupTestDB(t)

	// Create some metrics
	m1 := models.NewMetric(models.MetricWeight, 82.5)
	m2 := models.NewMetric(models.MetricWeight, 82.0)
	m3 := models.NewMetric(models.MetricHRV, 45)

	CreateMetric(db, m1)
	CreateMetric(db, m2)
	CreateMetric(db, m3)

	// List all
	metrics, err := ListMetrics(db, nil, 10)
	if err != nil {
		t.Fatalf("ListMetrics failed: %v", err)
	}
	if len(metrics) != 3 {
		t.Errorf("expected 3 metrics, got %d", len(metrics))
	}

	// List by type
	weightType := models.MetricWeight
	metrics, err = ListMetrics(db, &weightType, 10)
	if err != nil {
		t.Fatalf("ListMetrics by type failed: %v", err)
	}
	if len(metrics) != 2 {
		t.Errorf("expected 2 weight metrics, got %d", len(metrics))
	}
}

func TestDeleteMetric(t *testing.T) {
	db := setupTestDB(t)

	m := models.NewMetric(models.MetricWeight, 82.5)
	CreateMetric(db, m)

	if err := DeleteMetric(db, m.ID.String()); err != nil {
		t.Fatalf("DeleteMetric failed: %v", err)
	}

	_, err := GetMetric(db, m.ID.String())
	if err == nil {
		t.Error("expected error getting deleted metric")
	}
}
