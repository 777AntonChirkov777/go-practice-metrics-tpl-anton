package filestore

import (
	"context"
	"practice/internal/logger"
	model "practice/internal/model"
	"practice/internal/storage"

	"go.uber.org/zap"
)

type SyncStorage struct {
	store storage.MetricStorage
	fs    *FileStore
}

var _ storage.MetricStorage = (*SyncStorage)(nil)

func NewSyncStorage(store storage.MetricStorage, fs *FileStore) *SyncStorage {
	return &SyncStorage{store: store, fs: fs}
}

func (s *SyncStorage) Save(ctx context.Context, m *model.Metric) error {
	if err := s.store.Save(ctx, m); err != nil {
		return err
	}
	if err := s.fs.SaveFrom(ctx, s.store); err != nil {
		logger.Log.Info("synchronous dump failed", zap.Error(err))
	}
	return nil
}

func (s *SyncStorage) SaveBatch(ctx context.Context, metrics []*model.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	if err := s.store.SaveBatch(ctx, metrics); err != nil {
		return err
	}
	if err := s.fs.SaveFrom(ctx, s.store); err != nil {
		logger.Log.Info("synchronous batch dump failed", zap.Error(err))
	}
	return nil
}

func (s *SyncStorage) Get(ctx context.Context, mtype model.MetricType, name string) (*model.Metric, error) {
	return s.store.Get(ctx, mtype, name)
}

func (s *SyncStorage) GetAll(ctx context.Context) ([]*model.Metric, error) {
	return s.store.GetAll(ctx)
}
