package storage

import (
	"testing"

	model "practice/internal/model"
)

func TestMemStorage_SaveGetGauge(t *testing.T) {
	s := NewMemStorage()
	if err := s.Save(model.NewGaugeMetric("cpu", 0.5)); err != nil {
		t.Fatal(err)
	}
	m, ok := s.Get(model.Gauge, "cpu")
	if !ok || m.Value != 0.5 {
		t.Fatalf("gauge not stored: %+v ok=%v", m, ok)
	}
	// перезапись значения
	if err := s.Save(model.NewGaugeMetric("cpu", 0.9)); err != nil {
		t.Fatal(err)
	}
	m, _ = s.Get(model.Gauge, "cpu")
	if m.Value != 0.9 {
		t.Errorf("gauge not overwritten: %v", m.Value)
	}
}

func TestMemStorage_CounterAccumulates(t *testing.T) {
	s := NewMemStorage()
	_ = s.Save(model.NewCountMetric("reqs", 3))
	_ = s.Save(model.NewCountMetric("reqs", 4))
	m, ok := s.Get(model.Counter, "reqs")
	if !ok || m.Delta != 7 {
		t.Fatalf("counter should accumulate to 7, got %+v ok=%v", m, ok)
	}
}

// LoadAll имеет семантику замены: восстановленный counter не накапливается
// поверх, а задаёт значение как есть.
func TestMemStorage_LoadAllSetSemantics(t *testing.T) {
	s := NewMemStorage()
	s.LoadAll([]*model.Metric{
		model.NewGaugeMetric("Alloc", 12.5),
		model.NewCountMetric("PollCount", 100),
	})

	g, ok := s.Get(model.Gauge, "Alloc")
	if !ok || g.Value != 12.5 {
		t.Errorf("gauge not restored: %+v ok=%v", g, ok)
	}
	c, ok := s.Get(model.Counter, "PollCount")
	if !ok || c.Delta != 100 {
		t.Errorf("counter not restored as-is: %+v ok=%v", c, ok)
	}

	if got := len(s.GetAll()); got != 2 {
		t.Errorf("GetAll len = %d, want 2", got)
	}
}

// Get возвращает копию: мутация результата не меняет хранилище.
func TestMemStorage_GetReturnsCopy(t *testing.T) {
	s := NewMemStorage()
	_ = s.Save(model.NewGaugeMetric("x", 1))
	m, _ := s.Get(model.Gauge, "x")
	m.Value = 999
	again, _ := s.Get(model.Gauge, "x")
	if again.Value != 1 {
		t.Errorf("storage mutated through returned copy: %v", again.Value)
	}
}
