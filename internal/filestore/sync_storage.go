package filestore

import (
	"practice/internal/logger"
	model "practice/internal/model"
	"practice/internal/storage"

	"go.uber.org/zap"
)

type SyncStorage struct {
	storage.MetricStorage
	fs *FileStore
}

func NewSyncStorage(store storage.MetricStorage, fs *FileStore) *SyncStorage {
	return &SyncStorage{MetricStorage: store, fs: fs}
}

func (s *SyncStorage) Save(m *model.Metric) error {
	if err := s.MetricStorage.Save(m); err != nil {
		return err
	}
	if err := s.fs.Save(s.MetricStorage.GetAll()); err != nil {
		logger.Log.Info("synchronous dump failed", zap.Error(err))
	}
	return nil
}
