package services_test

import (
	"os"
	"testing"

	models "practice/internal/model"
	repositories "practice/internal/repository"
	services "practice/internal/service"
)

// setupTempDir создаёт временную директорию и переходит в неё.
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

// --------------- Панические случаи ---------------

func TestSaveMetric_InvalidTypePanic(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid metric type")
		}
	}()
	services.SaveMetric(models.Unknown, "name", "1.0")
}

func TestSaveMetric_EmptyNamePanic(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for empty name")
		}
	}()
	// имя состоит из пробелов
	services.SaveMetric(models.Gauge, "  ", "1.0")
}

func TestSaveMetric_EmptyValuePanic(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for empty value")
		}
	}()
	// значение состоит из пробелов
	services.SaveMetric(models.Gauge, "name", "  ")
}

// --------------- Gauge метрики ---------------

func TestSaveMetric_NewGauge(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	err := services.SaveMetric(models.Gauge, "cpu", "0.5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m, err := repositories.FindMetric("cpu", int8(models.Gauge))
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("metric not found")
	}
	if m.Value != 0.5 {
		t.Errorf("expected value 0.5, got %v", m.Value)
	}
}

func TestSaveMetric_UpdateGaugeDifferentValue(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	// Начальное сохранение
	err := services.SaveMetric(models.Gauge, "temp", "10.0")
	if err != nil {
		t.Fatal(err)
	}
	// Обновляем другим значением
	err = services.SaveMetric(models.Gauge, "temp", "20.0")
	if err != nil {
		t.Fatal(err)
	}

	m, err := repositories.FindMetric("temp", int8(models.Gauge))
	if err != nil {
		t.Fatal(err)
	}
	if m.Value != 20.0 {
		t.Errorf("expected updated value 20.0, got %v", m.Value)
	}
}

func TestSaveMetric_UpdateGaugeSameValue(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	err := services.SaveMetric(models.Gauge, "pressure", "100.0")
	if err != nil {
		t.Fatal(err)
	}
	// Передаём то же значение — обновления не происходит
	err = services.SaveMetric(models.Gauge, "pressure", "100.0")
	if err != nil {
		t.Fatal(err)
	}

	m, err := repositories.FindMetric("pressure", int8(models.Gauge))
	if err != nil {
		t.Fatal(err)
	}
	if m.Value != 100.0 {
		t.Errorf("value should remain 100.0, got %v", m.Value)
	}
}

// --------------- Counter метрики ---------------

func TestSaveMetric_NewCounter(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	err := services.SaveMetric(models.Counter, "requests", "5")
	if err != nil {
		t.Fatal(err)
	}

	m, err := repositories.FindMetric("requests", int8(models.Counter))
	if err != nil {
		t.Fatal(err)
	}
	if m.Delta != 5 {
		t.Errorf("expected delta 5, got %d", m.Delta)
	}
}

func TestSaveMetric_UpdateCounter(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	// Начальное значение
	err := services.SaveMetric(models.Counter, "clicks", "10")
	if err != nil {
		t.Fatal(err)
	}
	// При обновлении counter инкрементируется, переданное value игнорируется
	err = services.SaveMetric(models.Counter, "clicks", "0")
	if err != nil {
		t.Fatal(err)
	}

	m, err := repositories.FindMetric("clicks", int8(models.Counter))
	if err != nil {
		t.Fatal(err)
	}
	if m.Delta != 11 {
		t.Errorf("expected delta 11 after increment, got %d", m.Delta)
	}

	// Ещё одно обновление
	err = services.SaveMetric(models.Counter, "clicks", "100")
	if err != nil {
		t.Fatal(err)
	}
	m, _ = repositories.FindMetric("clicks", int8(models.Counter))
	if m.Delta != 12 {
		t.Errorf("expected delta 12 after second increment, got %d", m.Delta)
	}
}

// --------------- Проверка обработки некорректных значений ---------------

func TestSaveMetric_InvalidValueParsedAsZero(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	// Некорректное число для Gauge – ParseGauge вернёт 0
	err := services.SaveMetric(models.Gauge, "invalid", "not-a-number")
	if err != nil {
		t.Fatal(err)
	}

	m, err := repositories.FindMetric("invalid", int8(models.Gauge))
	if err != nil {
		t.Fatal(err)
	}
	if m.Value != 0.0 {
		t.Errorf("expected 0 for invalid parse, got %v", m.Value)
	}
}
