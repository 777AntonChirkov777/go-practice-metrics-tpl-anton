package models

import (
	"encoding/json"
	"testing"
)

func TestNewGaugeMetric(t *testing.T) {
	id := "cpu_usage"
	value := 75.5
	m := NewGaugeMetric(id, value)

	if m.ID != id {
		t.Errorf("ID = %q, want %q", m.ID, id)
	}
	if m.MType != int8(Gauge) {
		t.Errorf("MType = %d, want %d", m.MType, Gauge)
	}
	if m.Value != value {
		t.Errorf("Value = %v, want %v", m.Value, value)
	}
	if m.Delta != 0 {
		t.Errorf("Delta = %d, want 0", m.Delta)
	}
	if m.Hash != "" {
		t.Errorf("Hash = %q, want empty", m.Hash)
	}
}

func TestNewCountMetric(t *testing.T) {
	id := "requests"
	delta := int64(42)
	m := NewCountMetric(id, delta)

	if m.ID != id {
		t.Errorf("ID = %q, want %q", m.ID, id)
	}
	if m.MType != int8(Counter) {
		t.Errorf("MType = %d, want %d", m.MType, Counter)
	}
	if m.Delta != delta {
		t.Errorf("Delta = %d, want %d", m.Delta, delta)
	}
	if m.Value != 0 {
		t.Errorf("Value = %v, want 0", m.Value)
	}
	if m.Hash != "" {
		t.Errorf("Hash = %q, want empty", m.Hash)
	}
}

func TestMetric_CalculateHash(t *testing.T) {
	// Создаём gauge‑метрику
	m1 := NewGaugeMetric("temp", 23.5)
	// Первый вызов CalculateHash
	err := m1.CalculateHash()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hash1 := m1.Hash
	if hash1 == "" {
		t.Error("Hash should not be empty after CalculateHash")
	}

	// Повторный вызов для того же объекта – хеш должен остаться тем же
	err = m1.CalculateHash()
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if m1.Hash != hash1 {
		t.Error("hash changed on identical metric")
	}

	// Такая же метрика (те же значения) даст тот же хеш
	m2 := NewGaugeMetric("temp", 23.5)
	err = m2.CalculateHash()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m2.Hash != hash1 {
		t.Error("identical metric produced different hash")
	}

	// Изменение поля Value должно изменить хеш
	m3 := NewGaugeMetric("temp", 99.9)
	err = m3.CalculateHash()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m3.Hash == hash1 {
		t.Error("hash should change when value changes")
	}

	// Изменение ID
	m4 := NewGaugeMetric("other", 23.5)
	err = m4.CalculateHash()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m4.Hash == hash1 {
		t.Error("hash should change when ID changes")
	}

	// Предварительно установленный Hash игнорируется при расчёте
	m5 := NewGaugeMetric("temp", 23.5)
	m5.Hash = "some_old_hash"
	err = m5.CalculateHash()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m5.Hash != hash1 {
		t.Errorf("pre‑existing hash should be replaced; got %q, want %q", m5.Hash, hash1)
	}
}

func TestMetric_JSONOmitEmpty(t *testing.T) {
	// Проверяем, что поля Value и Delta не выводятся, если равны нулю
	g := NewGaugeMetric("mem", 0.0)
	data, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var generic map[string]interface{}
	if err := json.Unmarshal(data, &generic); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if _, exists := generic["value"]; exists {
		t.Error("zero Value should be omitted")
	}
	if _, exists := generic["delta"]; exists {
		t.Error("zero Delta should be omitted for gauge metric")
	}

	c := NewCountMetric("req", 0)
	data, err = json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	generic = nil
	if err := json.Unmarshal(data, &generic); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if _, exists := generic["delta"]; exists {
		t.Error("zero Delta should be omitted")
	}
	if _, exists := generic["value"]; exists {
		t.Error("zero Value should be omitted for counter metric")
	}

	// Если значения ненулевые – они присутствуют
	g.Value = 1.23
	data, _ = json.Marshal(g)
	json.Unmarshal(data, &generic)
	if _, exists := generic["value"]; !exists {
		t.Error("non‑zero Value should be present")
	}
}
