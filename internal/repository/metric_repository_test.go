package repositories

import (
	"encoding/json"
	"os"
	"testing"

	models "practice/internal/model"
)

// Вспомогательная функция для подготовки изолированной файловой системы.
// Возвращает функцию восстановления исходной рабочей директории.
func setupTempDir(t *testing.T) (restore func()) {
	t.Helper()
	tmpDir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to tmp: %v", err)
	}
	return func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("failed to restore working dir: %v", err)
		}
	}
}

// Записывает слайс метрик в файл models.FileName (формат JSON Lines).
func writeMetricFile(t *testing.T, metrics []models.Metric) {
	t.Helper()
	f, err := os.Create(models.FileName)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, m := range metrics {
		if err := enc.Encode(m); err != nil {
			t.Fatalf("failed to encode metric: %v", err)
		}
	}
}

// Читает все метрики из файла models.FileName.
func readAllMetrics(t *testing.T) []models.Metric {
	t.Helper()
	f, err := os.Open(models.FileName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("failed to open test file: %v", err)
	}
	defer f.Close()
	var result []models.Metric
	dec := json.NewDecoder(f)
	for dec.More() {
		var m models.Metric
		if err := dec.Decode(&m); err != nil {
			continue // битые строки пропускаем, как и в оригинале
		}
		result = append(result, m)
	}
	return result
}

func TestSaveMetric_NewFile(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	m := models.NewGaugeMetric("cpu", 0.75)
	if err := SaveMetric(*m); err != nil {
		t.Fatalf("SaveMetric error: %v", err)
	}

	metrics := readAllMetrics(t)
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	got := metrics[0]
	if got.ID != "cpu" || got.MType != 1 || got.Value != 0.75 {
		t.Errorf("metric mismatch: %+v", got)
	}
}

func TestSaveMetric_UpdateExisting(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	// Сначала сохраняем gauge
	old := models.NewGaugeMetric("mem", 100.0)
	if err := SaveMetric(*old); err != nil {
		t.Fatalf("first SaveMetric error: %v", err)
	}

	// Обновляем тем же ID и MType
	new := models.NewGaugeMetric("mem", 200.0)
	if err := SaveMetric(*new); err != nil {
		t.Fatalf("second SaveMetric error: %v", err)
	}

	metrics := readAllMetrics(t)
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric after update, got %d", len(metrics))
	}
	if metrics[0].Value != 200.0 {
		t.Errorf("expected value 200, got %v", metrics[0].Value)
	}
}

func TestSaveMetric_AddDifferent(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	gauge := models.NewGaugeMetric("temp", 36.6)
	counter := models.NewCountMetric("req", 5)

	if err := SaveMetric(*gauge); err != nil {
		t.Fatal(err)
	}
	if err := SaveMetric(*counter); err != nil {
		t.Fatal(err)
	}

	metrics := readAllMetrics(t)
	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(metrics))
	}
	// Проверяем наличие обоих
	found := map[string]bool{}
	for _, m := range metrics {
		if m.ID == "temp" && m.MType == 1 {
			found["gauge"] = true
		}
		if m.ID == "req" && m.MType == 2 {
			found["counter"] = true
		}
	}
	if !found["gauge"] || !found["counter"] {
		t.Errorf("not all metrics present: %+v", metrics)
	}
}

func TestSaveMetric_InvalidJSONLinesIgnored(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	// Создаём файл с "битой" строкой вручную
	f, err := os.Create(models.FileName)
	if err != nil {
		t.Fatal(err)
	}
	// Пишем валидную метрику
	valid := models.NewGaugeMetric("valid", 1.0)
	enc := json.NewEncoder(f)
	if err := enc.Encode(valid); err != nil {
		f.Close()
		t.Fatal(err)
	}
	// Дописываем мусор
	f.Write([]byte("this is not json\n"))
	f.Close()

	// Сохраняем новую метрику – она должна заменить старую
	newM := models.NewGaugeMetric("valid", 2.0)
	if err := SaveMetric(*newM); err != nil {
		t.Fatalf("SaveMetric error: %v", err)
	}

	metrics := readAllMetrics(t)
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric after save, got %d", len(metrics))
	}
	if metrics[0].Value != 2.0 {
		t.Errorf("expected value 2.0, got %v", metrics[0].Value)
	}
}

func TestSaveMetric_AddNewWhenFileDoesNotExist(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	// Файла нет – создаётся новый
	m := models.NewCountMetric("clicks", 10)
	if err := SaveMetric(*m); err != nil {
		t.Fatal(err)
	}
	metrics := readAllMetrics(t)
	if len(metrics) != 1 || metrics[0].Delta != 10 {
		t.Error("metric not correctly saved into a new file")
	}
}

func TestFindMetric_Found(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	predefined := []models.Metric{
		*models.NewGaugeMetric("cpu", 0.5),
		*models.NewCountMetric("requests", 42),
		*models.NewGaugeMetric("mem", 99.9),
	}
	writeMetricFile(t, predefined)

	found, err := FindMetric("requests", 2)
	if err != nil {
		t.Fatal(err)
	}
	if found == nil {
		t.Fatal("expected metric not found")
	}
	if found.Delta != 42 {
		t.Errorf("wrong delta: %d", found.Delta)
	}
}

func TestFindMetric_NotFound(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	writeMetricFile(t, []models.Metric{
		*models.NewGaugeMetric("a", 1.0),
	})

	found, err := FindMetric("b", 1)
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Error("expected nil, got metric")
	}
}

func TestFindMetric_EmptyFile(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	// файл не существует, FindMetric создаст его пустым
	found, err := FindMetric("any", 1)
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Error("expected nil from empty/non-existent file")
	}
}

func TestFindMetric_DuplicatePanic(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	// Две одинаковые метрики
	m1 := models.NewGaugeMetric("panic", 1.0)
	m2 := models.NewGaugeMetric("panic", 2.0)
	writeMetricFile(t, []models.Metric{*m1, *m2})

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic but did not panic")
		}
	}()
	FindMetric("panic", 1) // должно паниковать
}
