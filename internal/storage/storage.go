package storage

import (
	"context"
	"errors"

	models "practice/internal/model"
)

var ErrNotFound = errors.New("metric not found")

// MetricStorage — контракт, через который handlers работают с хранилищем.
// Это позволяет подменить in-memory реализацию на БД без переписывания handlers.
type MetricStorage interface {
	Save(ctx context.Context, m *models.Metric) error
	SaveBatch(ctx context.Context, metrics []*models.Metric) error
	Get(ctx context.Context, mtype models.MetricType, name string) (*models.Metric, error)
	GetAll(ctx context.Context) ([]*models.Metric, error)
}
