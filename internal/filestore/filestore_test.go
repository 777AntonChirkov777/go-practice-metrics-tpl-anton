package filestore

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	model "practice/internal/model"
	"practice/internal/storage"
)

func tmpPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "metrics-db.json")
}

// Формат файла должен совпадать с примером из задания: JSON-массив, тип строкой
// "gauge"/"counter", значение в value / delta.
func TestSave_FileFormat(t *testing.T) {
	path := tmpPath(t)
	fs := New(path)

	metrics := []*model.Metric{
		model.NewGaugeMetric("LastGC", 1257894000000000000),
		model.NewCountMetric("NumGC", 42),
	}
	if err := fs.save(metrics); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("file is not a JSON array: %v\n%s", err, data)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(arr))
	}

	byID := map[string]map[string]any{}
	for _, e := range arr {
		id, _ := e["id"].(string)
		byID[id] = e
	}

	g := byID["LastGC"]
	if g["type"] != "gauge" {
		t.Errorf("gauge type = %v, want \"gauge\"", g["type"])
	}
	if _, ok := g["value"]; !ok {
		t.Errorf("gauge must carry a value field: %v", g)
	}
	if _, ok := g["delta"]; ok {
		t.Errorf("gauge must not carry delta: %v", g)
	}

	c := byID["NumGC"]
	if c["type"] != "counter" {
		t.Errorf("counter type = %v, want \"counter\"", c["type"])
	}
	if c["delta"].(float64) != 42 {
		t.Errorf("counter delta = %v, want 42", c["delta"])
	}
	if _, ok := c["value"]; ok {
		t.Errorf("counter must not carry value: %v", c)
	}
}

// Значение gauge, равное нулю, обязано сохраняться (0 — валидное значение).
func TestSave_ZeroGaugePresent(t *testing.T) {
	path := tmpPath(t)
	fs := New(path)

	if err := fs.save([]*model.Metric{model.NewGaugeMetric("Zero", 0)}); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := fs.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(loaded))
	}
	if loaded[0].ID != "Zero" || loaded[0].MType != int8(model.Gauge) || loaded[0].Value != 0 {
		t.Errorf("zero gauge not round-tripped: %+v", loaded[0])
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	path := tmpPath(t)
	fs := New(path)

	in := []*model.Metric{
		model.NewGaugeMetric("Alloc", 12.5),
		model.NewCountMetric("PollCount", 7),
	}
	if err := fs.save(in); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	out, err := fs.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(out))
	}

	got := map[string]*model.Metric{}
	for _, m := range out {
		got[m.ID] = m
	}
	if got["Alloc"].Value != 12.5 {
		t.Errorf("Alloc value = %v, want 12.5", got["Alloc"].Value)
	}
	if got["PollCount"].Delta != 7 {
		t.Errorf("PollCount delta = %v, want 7", got["PollCount"].Delta)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	fs := New(filepath.Join(t.TempDir(), "does-not-exist.json"))
	out, err := fs.Load()
	if err != nil {
		t.Fatalf("missing file must not error, got %v", err)
	}
	if out != nil {
		t.Errorf("expected nil, got %v", out)
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	path := tmpPath(t)
	if err := os.WriteFile(path, []byte("   \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := New(path).Load()
	if err != nil {
		t.Fatalf("empty file must not error, got %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected no metrics, got %v", out)
	}
}

func TestLoad_BadJSON(t *testing.T) {
	path := tmpPath(t)
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(path).Load(); err == nil {
		t.Error("expected error for malformed JSON")
	}
}

// Некорректные записи (например, unknown type) пропускаются, валидные грузятся.
func TestLoad_SkipsInvalidEntries(t *testing.T) {
	path := tmpPath(t)
	content := `[
	  {"id":"ok","type":"gauge","value":1.5},
	  {"id":"bad","type":"weird","value":2}
	]`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := New(path).Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(out) != 1 || out[0].ID != "ok" {
		t.Errorf("expected only the valid entry, got %+v", out)
	}
}

func TestSave_EmptyPathIsNoop(t *testing.T) {
	fs := New("")
	if err := fs.save([]*model.Metric{model.NewGaugeMetric("x", 1)}); err != nil {
		t.Errorf("empty path Save should be no-op, got %v", err)
	}
	out, err := fs.Load()
	if err != nil || out != nil {
		t.Errorf("empty path Load should return (nil,nil), got %v, %v", out, err)
	}
}

func TestSaveFrom_KeepsOrderWhenWriterIsDelayed(t *testing.T) {
	ctx := context.Background()
	path := tmpPath(t)
	fs := New(path)
	store := storage.NewMemStorage()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := store.Save(ctx, model.NewGaugeMetric("first", 1)); err != nil {
			t.Errorf("store.Save: %v", err)
			return
		}
		time.Sleep(100 * time.Millisecond)
		if err := fs.SaveFrom(ctx, store); err != nil {
			t.Errorf("SaveFrom: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		time.Sleep(20 * time.Millisecond)
		if err := store.Save(ctx, model.NewGaugeMetric("second", 2)); err != nil {
			t.Errorf("store.Save: %v", err)
			return
		}
		if err := fs.SaveFrom(ctx, store); err != nil {
			t.Errorf("SaveFrom: %v", err)
		}
	}()

	wg.Wait()

	loaded, err := fs.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	ids := map[string]bool{}
	for _, m := range loaded {
		ids[m.ID] = true
	}
	if !ids["first"] || !ids["second"] {
		t.Errorf("устаревший снимок затёр более свежий, на диске %v", ids)
	}
}

// Save создаёт промежуточные директории пути.
func TestSave_CreatesDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "metrics.json")
	if err := New(path).save([]*model.Metric{model.NewGaugeMetric("x", 1)}); err != nil {
		t.Fatalf("Save error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created in nested dir: %v", err)
	}
}
