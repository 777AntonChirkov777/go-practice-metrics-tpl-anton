package storage

import (
	"context"
	"fmt"
	"sync"
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

func TestMemStorage_SaveBatchMixedTypes(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()

	err := s.SaveBatch(ctx, []*model.Metric{
		model.NewGaugeMetric("cpu", 0.5),
		model.NewCountMetric("reqs", 3),
	})
	if err != nil {
		t.Fatal(err)
	}

	gauge, err := s.Get(ctx, model.Gauge, "cpu")
	if err != nil || gauge.Value != 0.5 {
		t.Fatalf("gauge not stored: %+v err=%v", gauge, err)
	}
	counter, err := s.Get(ctx, model.Counter, "reqs")
	if err != nil || counter.Delta != 3 {
		t.Fatalf("counter not stored: %+v err=%v", counter, err)
	}
}

func TestMemStorage_SaveBatchCounterAccumulates(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()

	batch := []*model.Metric{model.NewCountMetric("reqs", 4)}
	if err := s.SaveBatch(ctx, batch); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveBatch(ctx, []*model.Metric{model.NewCountMetric("reqs", 6)}); err != nil {
		t.Fatal(err)
	}

	m, _ := s.Get(ctx, model.Counter, "reqs")
	if m.Delta != 10 {
		t.Errorf("counter delta = %d, want 10", m.Delta)
	}
}

func TestMemStorage_SaveBatchDuplicateNames(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()

	err := s.SaveBatch(ctx, []*model.Metric{
		model.NewCountMetric("reqs", 1),
		model.NewCountMetric("reqs", 2),
		model.NewGaugeMetric("cpu", 1.5),
		model.NewGaugeMetric("cpu", 2.5),
	})
	if err != nil {
		t.Fatal(err)
	}

	counter, _ := s.Get(ctx, model.Counter, "reqs")
	if counter.Delta != 3 {
		t.Errorf("counter delta = %d, want 3", counter.Delta)
	}
	gauge, _ := s.Get(ctx, model.Gauge, "cpu")
	if gauge.Value != 2.5 {
		t.Errorf("gauge value = %v, want 2.5", gauge.Value)
	}
}

func TestMemStorage_SaveBatchEmptyIsNoop(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()

	if err := s.Save(ctx, model.NewGaugeMetric("cpu", 0.5)); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveBatch(ctx, nil); err != nil {
		t.Fatalf("empty batch must not fail: %v", err)
	}
	if err := s.SaveBatch(ctx, []*model.Metric{}); err != nil {
		t.Fatalf("empty batch must not fail: %v", err)
	}

	all, err := s.GetAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Errorf("storage changed after empty batch: %d metrics", len(all))
	}
}

func TestMemStorage_SaveBatchNeverVisibleHalfApplied(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()

	const (
		batchSize = 50
		rounds    = 100
	)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Каждый раунд добавляет batchSize НОВЫХ имён, поэтому размер хранилища
	// обязан быть кратен batchSize: любое другое число — наполовину
	// применённый батч.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}

			all, err := s.GetAll(ctx)
			if err != nil {
				t.Errorf("GetAll error: %v", err)
				return
			}
			if n := len(all); n%batchSize != 0 {
				t.Errorf("reader saw %d metrics, want a multiple of %d: batch is visible half-applied", n, batchSize)
				return
			}
		}
	}()

	for round := 0; round < rounds; round++ {
		batch := make([]*model.Metric, 0, batchSize)
		for i := 0; i < batchSize; i++ {
			batch = append(batch, model.NewGaugeMetric(fmt.Sprintf("r%03d-g%02d", round, i), float64(i)))
		}
		if err := s.SaveBatch(ctx, batch); err != nil {
			t.Errorf("SaveBatch error: %v", err)
			break
		}
	}

	close(stop)
	wg.Wait()
}
