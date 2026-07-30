package filestore

import (
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

func (s *SyncStorage) Save(m *model.Metric) error {
	if err := s.store.Save(m); err != nil {
		return err
	}
	if err := s.fs.SaveFrom(s.store); err != nil {
		logger.Log.Info("synchronous dump failed", zap.Error(err))
	}
	return nil
}

func (s *SyncStorage) Get(mtype model.MetricType, name string) (*model.Metric, bool) {
	return s.store.Get(mtype, name)
}

func (s *SyncStorage) GetAll() []*model.Metric {
	return s.store.GetAll()
}
