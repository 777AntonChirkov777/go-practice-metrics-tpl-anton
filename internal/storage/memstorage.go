package storage

import (
	model "practice/internal/model"
	repository "practice/internal/repository"
	"sync"
)

type MemStorage struct {
	mu      sync.RWMutex
	gauges  map[string]*model.Metric
	counter map[string]*model.Metric
}

func NewMemStorage() *MemStorage {
	s := &MemStorage{
		gauges:  make(map[string]*model.Metric),
		counter: make(map[string]*model.Metric),
	}
	s.loadFromFile()
	return s
}

// loadFromFile подтягивает уже сохранённые метрики из JSONL при старте.
func (s *MemStorage) loadFromFile() {
	all, err := repository.ReadAll()
	if err != nil {
		return
	}
	for _, m := range all {
		switch model.MetricType(m.MType) {
		case model.Gauge:
			s.gauges[m.ID] = m
		case model.Counter:
			s.counter[m.ID] = m
		}
	}
}

func (s *MemStorage) Save(m *model.Metric) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch model.MetricType(m.MType) {
	case model.Gauge:
		// если уже есть — обновляем Value
		if existing, ok := s.gauges[m.ID]; ok {
			existing.Value = m.Value
			existing.Hash = m.Hash
			m = existing
		} else {
			s.gauges[m.ID] = m
		}
	case model.Counter:
		// счётчик накапливает: прибавляем Delta к уже существующему
		if existing, ok := s.counter[m.ID]; ok {
			existing.Delta += m.Delta
			existing.Hash = m.Hash
			m = existing
		} else {
			s.counter[m.ID] = m
		}
	}

	// персистентность — пишем в файл через существующий репозиторий
	return repository.SaveMetric(*m)
}

func (s *MemStorage) Get(mtype model.MetricType, name string) (*model.Metric, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch mtype {
	case model.Gauge:
		m, ok := s.gauges[name]
		return m, ok
	case model.Counter:
		m, ok := s.counter[name]
		return m, ok
	}
	return nil, false
}

func (s *MemStorage) GetAll() []*model.Metric {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*model.Metric, 0, len(s.gauges)+len(s.counter))
	for _, m := range s.gauges {
		out = append(out, m)
	}
	for _, m := range s.counter {
		out = append(out, m)
	}
	return out
}
