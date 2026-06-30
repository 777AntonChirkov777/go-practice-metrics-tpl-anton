package storage

import models "practice/internal/model"

// MetricStorage — контракт, через который handlers работают с хранилищем.
// Это позволяет подменить in-memory реализацию на БД без переписывания handlers.
type MetricStorage interface {
	Save(m *models.Metric) error
	Get(mtype models.MetricType, name string) (*models.Metric, bool)
	GetAll() []*models.Metric
}
