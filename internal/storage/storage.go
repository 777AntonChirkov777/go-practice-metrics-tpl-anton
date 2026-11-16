package storage

import (
	"context"
	"errors"

	"practice/internal/model"
)

var ErrNotFound = errors.New("metric not found")

// MetricStorage — контракт, через который handlers работают с хранилищем.
// Это позволяет подменить in-memory реализацию на БД без переписывания handlers.
type MetricStorage interface {
	Save(ctx context.Context, m *model.Metric) error
	SaveBatch(ctx context.Context, metrics []*model.Metric) error
	Get(ctx context.Context, mtype model.MetricType, name string) (*model.Metric, error)
	GetAll(ctx context.Context) ([]model.Metric, error)
}
