// ABOUTME: Metric CRUD operations for Charm KV storage.
// ABOUTME: Uses type-prefixed keys and client-side filtering.
package charm

import (
	"fmt"
	"sort"

	"github.com/harperreed/health/internal/models"
)

// CreateMetric stores a new metric in the KV store.
func (c *Client) CreateMetric(m *models.Metric) error {
	key := MetricPrefix + m.ID.String()
	data, err := marshalJSON(m)
	if err != nil {
		return fmt.Errorf("marshal metric: %w", err)
	}
	return c.set(key, data)
}

// GetMetric retrieves a metric by ID or ID prefix.
func (c *Client) GetMetric(idOrPrefix string) (*models.Metric, error) {
	data, err := c.getByIDPrefix(MetricPrefix, idOrPrefix)
	if err != nil {
		return nil, fmt.Errorf("get metric: %w", err)
	}

	metric, err := unmarshalJSON[models.Metric](data)
	if err != nil {
		return nil, fmt.Errorf("unmarshal metric: %w", err)
	}

	return metric, nil
}

// ListMetrics retrieves metrics with optional filtering by type.
// Results are sorted by RecordedAt descending (most recent first).
func (c *Client) ListMetrics(metricType *models.MetricType, limit int) ([]*models.Metric, error) {
	allData, err := c.listByPrefix(MetricPrefix)
	if err != nil {
		return nil, fmt.Errorf("list metrics: %w", err)
	}

	var metrics []*models.Metric
	for _, data := range allData {
		m, err := unmarshalJSON[models.Metric](data)
		if err != nil {
			continue // Skip invalid entries
		}

		// Filter by type if specified
		if metricType != nil && m.MetricType != *metricType {
			continue
		}

		metrics = append(metrics, m)
	}

	// Sort by RecordedAt descending
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].RecordedAt.After(metrics[j].RecordedAt)
	})

	// Apply limit
	if limit > 0 && len(metrics) > limit {
		metrics = metrics[:limit]
	}

	return metrics, nil
}

// DeleteMetric removes a metric by ID or prefix.
func (c *Client) DeleteMetric(idOrPrefix string) error {
	if err := c.deleteByIDPrefix(MetricPrefix, idOrPrefix); err != nil {
		return fmt.Errorf("delete metric: %w", err)
	}
	return nil
}

// GetLatestMetric returns the most recent metric of a specific type.
func (c *Client) GetLatestMetric(metricType models.MetricType) (*models.Metric, error) {
	metrics, err := c.ListMetrics(&metricType, 1)
	if err != nil {
		return nil, err
	}
	if len(metrics) == 0 {
		return nil, fmt.Errorf("no metrics of type %s found", metricType)
	}
	return metrics[0], nil
}

// MetricFilter defines criteria for filtering metrics.
type MetricFilter struct {
	MetricType *models.MetricType
	Limit      int
}

// ListMetricsFiltered retrieves metrics matching the filter criteria.
func (c *Client) ListMetricsFiltered(filter *MetricFilter) ([]*models.Metric, error) {
	if filter == nil {
		return c.ListMetrics(nil, 0)
	}
	return c.ListMetrics(filter.MetricType, filter.Limit)
}
