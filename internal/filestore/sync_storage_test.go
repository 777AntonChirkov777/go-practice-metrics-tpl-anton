package filestore

import (
	"context"
	"fmt"
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
