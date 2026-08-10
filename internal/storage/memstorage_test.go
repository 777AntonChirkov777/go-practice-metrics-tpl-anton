package storage

import (
	"context"
	"testing"

	model "practice/internal/model"
)

func TestMemStorage_SaveGetGauge(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()
	if err := s.Save(ctx, model.NewGaugeMetric("cpu", 0.5)); err != nil {
		t.Fatal(err)
	}
	m, err := s.Get(ctx, model.Gauge, "cpu")
	if err != nil || m.Value != 0.5 {
		t.Fatalf("gauge not stored: %+v err=%v", m, err)
	}
	// перезапись значения
	if err := s.Save(ctx, model.NewGaugeMetric("cpu", 0.9)); err != nil {
		t.Fatal(err)
	}
	m, _ = s.Get(ctx, model.Gauge, "cpu")
	if m.Value != 0.9 {
		t.Errorf("gauge not overwritten: %v", m.Value)
	}
}

func TestMemStorage_CounterAccumulates(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()
	_ = s.Save(ctx, model.NewCountMetric("reqs", 3))
	_ = s.Save(ctx, model.NewCountMetric("reqs", 4))
	m, err := s.Get(ctx, model.Counter, "reqs")
	if err != nil || m.Delta != 7 {
		t.Fatalf("counter should accumulate to 7, got %+v err=%v", m, err)
	}
}

// LoadAll имеет семантику замены: восстановленный counter не накапливается
// поверх, а задаёт значение как есть.
func TestMemStorage_LoadAllSetSemantics(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()
	s.LoadAll([]*model.Metric{
		model.NewGaugeMetric("Alloc", 12.5),
		model.NewCountMetric("PollCount", 100),
	})

	g, err := s.Get(ctx, model.Gauge, "Alloc")
	if err != nil || g.Value != 12.5 {
		t.Errorf("gauge not restored: %+v err=%v", g, err)
	}
	c, err := s.Get(ctx, model.Counter, "PollCount")
	if err != nil || c.Delta != 100 {
		t.Errorf("counter not restored as-is: %+v err=%v", c, err)
	}

	all, err := s.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll error: %v", err)
	}
	if got := len(all); got != 2 {
		t.Errorf("GetAll len = %d, want 2", got)
	}
}

// Get возвращает копию: мутация результата не меняет хранилище.
func TestMemStorage_GetReturnsCopy(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()
	_ = s.Save(ctx, model.NewGaugeMetric("x", 1))
	m, _ := s.Get(ctx, model.Gauge, "x")
	m.Value = 999
	again, _ := s.Get(ctx, model.Gauge, "x")
	if again.Value != 1 {
		t.Errorf("storage mutated through returned copy: %v", again.Value)
	}
}
