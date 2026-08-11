package filestore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	model "practice/internal/model"
	"practice/internal/storage"
)

// В синхронном режиме каждое Save немедленно отражается на диске.
func TestSyncStorage_DumpsOnEachSave(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "metrics-db.json")
	fs := New(path)
	st := NewSyncStorage(storage.NewMemStorage(), fs)

	if err := st.Save(ctx, model.NewGaugeMetric("cpu", 0.5)); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := fs.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(loaded) != 1 || loaded[0].ID != "cpu" || loaded[0].Value != 0.5 {
		t.Fatalf("first save not persisted: %+v", loaded)
	}

	// Второе обновление счётчика — на диске накопленное значение.
	if err := st.Save(ctx, model.NewCountMetric("reqs", 3)); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	if err := st.Save(ctx, model.NewCountMetric("reqs", 4)); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err = fs.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	byID := map[string]*model.Metric{}
	for _, m := range loaded {
		byID[m.ID] = m
	}
	if byID["reqs"] == nil || byID["reqs"].Delta != 7 {
		t.Errorf("counter not accumulated on disk: %+v", byID["reqs"])
	}
	if byID["cpu"] == nil || byID["cpu"].Value != 0.5 {
		t.Errorf("gauge missing from disk snapshot: %+v", byID["cpu"])
	}
}

func TestSyncStorage_ConcurrentSavesAllPersisted(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "metrics-db.json")
	fs := New(path)
	st := NewSyncStorage(storage.NewMemStorage(), fs)

	const n = 50

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// t.Errorf, а не t.Fatalf: Fatal из не-тестовой горутины запрещён.
			if err := st.Save(ctx, model.NewGaugeMetric(fmt.Sprintf("g%02d", i), float64(i))); err != nil {
				t.Errorf("Save error: %v", err)
			}
		}(i)
	}
	wg.Wait()

	loaded, err := fs.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(loaded) != n {
		t.Fatalf("expected %d metrics on disk, got %d", n, len(loaded))
	}

	seen := make(map[string]bool, n)
	for _, m := range loaded {
		seen[m.ID] = true
	}
	for i := 0; i < n; i++ {
		if id := fmt.Sprintf("g%02d", i); !seen[id] {
			t.Errorf("metric %s lost", id)
		}
	}
}

type countingStorage struct {
	inner       storage.MetricStorage
	getAllCalls int
}

func (c *countingStorage) Save(ctx context.Context, m *model.Metric) error {
	return c.inner.Save(ctx, m)
}

func (c *countingStorage) SaveBatch(ctx context.Context, metrics []*model.Metric) error {
	return c.inner.SaveBatch(ctx, metrics)
}

func (c *countingStorage) Get(ctx context.Context, mtype model.MetricType, name string) (*model.Metric, error) {
	return c.inner.Get(ctx, mtype, name)
}

func (c *countingStorage) GetAll(ctx context.Context) ([]*model.Metric, error) {
	c.getAllCalls++
	return c.inner.GetAll(ctx)
}

func TestSyncStorage_BatchDumpsOnce(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "metrics-db.json")
	fs := New(path)
	counting := &countingStorage{inner: storage.NewMemStorage()}
	st := NewSyncStorage(counting, fs)

	batch := []*model.Metric{
		model.NewGaugeMetric("cpu", 0.5),
		model.NewGaugeMetric("mem", 1.5),
		model.NewCountMetric("reqs", 3),
		model.NewCountMetric("errs", 1),
		model.NewGaugeMetric("disk", 2.5),
	}
	if err := st.SaveBatch(ctx, batch); err != nil {
		t.Fatalf("SaveBatch error: %v", err)
	}

	// Один дамп читает хранилище ровно один раз, N дампов читали бы N раз.
	if counting.getAllCalls != 1 {
		t.Errorf("dumps = %d, want exactly 1 for a batch of %d", counting.getAllCalls, len(batch))
	}

	loaded, err := fs.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(loaded) != len(batch) {
		t.Fatalf("persisted %d metrics, want %d", len(loaded), len(batch))
	}

	byID := map[string]*model.Metric{}
	for _, m := range loaded {
		byID[m.ID] = m
	}
	if byID["cpu"] == nil || byID["cpu"].Value != 0.5 {
		t.Errorf("gauge cpu not persisted: %+v", byID["cpu"])
	}
	if byID["reqs"] == nil || byID["reqs"].Delta != 3 {
		t.Errorf("counter reqs not persisted: %+v", byID["reqs"])
	}
}

func TestSyncStorage_EmptyBatchDoesNotDump(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "metrics-db.json")
	fs := New(path)
	counting := &countingStorage{inner: storage.NewMemStorage()}
	st := NewSyncStorage(counting, fs)

	if err := st.SaveBatch(ctx, nil); err != nil {
		t.Fatalf("SaveBatch error: %v", err)
	}
	if counting.getAllCalls != 0 {
		t.Errorf("dumps = %d, want 0 for an empty batch", counting.getAllCalls)
	}
}

func TestSyncStorage_DumpErrorDoesNotFailBatch(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	// Каталог для дампа занят обычным файлом, поэтому MkdirAll внутри
	// FileStore обязан упасть — это и есть отказ записи на диск.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("setup error: %v", err)
	}
	dumpPath := filepath.Join(blocker, "metrics-db.json")

	inner := storage.NewMemStorage()
	st := NewSyncStorage(inner, New(dumpPath))

	batch := []*model.Metric{
		model.NewGaugeMetric("cpu", 0.5),
		model.NewCountMetric("reqs", 3),
	}
	if err := st.SaveBatch(ctx, batch); err != nil {
		t.Fatalf("SaveBatch must stay successful when the dump fails: %v", err)
	}

	gauge, err := inner.Get(ctx, model.Gauge, "cpu")
	if err != nil || gauge.Value != 0.5 {
		t.Errorf("batch must still be applied in memory: %+v err=%v", gauge, err)
	}
	counter, err := inner.Get(ctx, model.Counter, "reqs")
	if err != nil || counter.Delta != 3 {
		t.Errorf("batch must still be applied in memory: %+v err=%v", counter, err)
	}

	// Любая ошибка Stat означает, что файла по пути нет: на Linux это ENOTDIR,
	// на Windows — ENOENT. Успешный Stat означал бы, что дамп всё-таки записан.
	if _, err := os.Stat(dumpPath); err == nil {
		t.Error("dump file must not exist, but it does")
	}
}
