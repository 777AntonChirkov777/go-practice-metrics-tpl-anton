package storage

import (
	"context"
	model "practice/internal/model"
	"sync"
)

type MemStorage struct {
	mu      sync.RWMutex
	gauges  map[string]*model.Metric
	counter map[string]*model.Metric
}

var _ MetricStorage = (*MemStorage)(nil)

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:  make(map[string]*model.Metric),
		counter: make(map[string]*model.Metric),
	}
}

// LoadAll заполняет хранилище переданными метриками с семантикой «замены»
// (set), а не накопления: используется при восстановлении из файла на старте.
func (s *MemStorage) LoadAll(metrics []*model.Metric) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range metrics {
		cp := *m
		switch model.MetricType(cp.MType) {
		case model.Gauge:
			s.gauges[cp.ID] = &cp
		case model.Counter:
			s.counter[cp.ID] = &cp
		}
	}
}

func (s *MemStorage) Save(_ context.Context, m *model.Metric) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.saveLocked(m)

	return nil
}

func (s *MemStorage) SaveBatch(_ context.Context, metrics []*model.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range metrics {
		s.saveLocked(m)
	}

	return nil
}

func (s *MemStorage) saveLocked(m *model.Metric) {
	switch model.MetricType(m.MType) {
	case model.Gauge:
		// если уже есть — обновляем Value
		if existing, ok := s.gauges[m.ID]; ok {
			existing.Value = m.Value
			existing.Hash = m.Hash
		} else {
			s.gauges[m.ID] = m
		}
	case model.Counter:
		// счётчик накапливает: прибавляем Delta к уже существующему
		if existing, ok := s.counter[m.ID]; ok {
			existing.Delta += m.Delta
			existing.Hash = m.Hash
		} else {
			s.counter[m.ID] = m
		}
	}
}

func (s *MemStorage) Get(_ context.Context, mtype model.MetricType, name string) (*model.Metric, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch mtype {
	case model.Gauge:
		if m, ok := s.gauges[name]; ok {
			cp := *m
			return &cp, nil
		}
	case model.Counter:
		if m, ok := s.counter[name]; ok {
			cp := *m
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func (s *MemStorage) GetAll(_ context.Context) ([]*model.Metric, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Копии по той же причине, что и в Get.
	out := make([]*model.Metric, 0, len(s.gauges)+len(s.counter))
	for _, m := range s.gauges {
		cp := *m
		out = append(out, &cp)
	}
	for _, m := range s.counter {
		cp := *m
		out = append(out, &cp)
	}
	return out, nil
}
