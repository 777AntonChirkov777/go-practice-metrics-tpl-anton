package models

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

const (
	FileName = "metricData.jsonl"
)

// NOTE: Не усложняем пример, вводя иерархическую вложенность структур.
// Органичиваясь плоской моделью.
// Delta и Value объявлены через указатели,
// что бы отличать значение "0", от не заданного значения
// и соответственно не кодировать в структуру.
type Metric struct {
	ID    string  `json:"id"`
	MType int8    `json:"type"`
	Delta int64   `json:"delta,omitempty"`
	Value float64 `json:"value,omitempty"`
	Hash  string  `json:"hash,omitempty"`
}

func (m *Metric) CalculateHash() error {
	// Временная структура без поля Hash, чтобы исключить его из хеширования
	temp := struct {
		ID    string
		MType int8
		Delta int64
		Value float64
	}{
		ID:    m.ID,
		MType: m.MType,
		Delta: m.Delta,
		Value: m.Value,
	}

	data, err := json.Marshal(temp)
	if err != nil {
		return fmt.Errorf("failed to marshal metric for hash: %w", err)
	}

	hash := sha256.Sum256(data)
	m.Hash = fmt.Sprintf("%x", hash)
	return nil
}

func NewGaugeMetric(id string, value float64) *Metric {
	return &Metric{
		ID:    id,
		MType: int8(Gauge),
		Value: value,
	}
}

func NewCountMetric(id string, value int64) *Metric {
	return &Metric{
		ID:    id,
		MType: int8(Counter),
		Delta: value,
	}
}
