package filestore

import (
	"path/filepath"
	"testing"

	model "practice/internal/model"
	"practice/internal/storage"
)

// В синхронном режиме каждое Save немедленно отражается на диске.
func TestSyncStorage_DumpsOnEachSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics-db.json")
	fs := New(path)
	sync := NewSyncStorage(storage.NewMemStorage(), fs)

	if err := sync.Save(model.NewGaugeMetric("cpu", 0.5)); err != nil {
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
	if err := sync.Save(model.NewCountMetric("reqs", 3)); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	if err := sync.Save(model.NewCountMetric("reqs", 4)); err != nil {
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
