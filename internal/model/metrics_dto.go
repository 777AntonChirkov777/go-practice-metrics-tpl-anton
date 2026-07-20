package models

import "errors"

// Ошибки конвертации wire -> domain. Хендлер маппит их в 4xx.
var (
	ErrUnknownType   = errors.New("invalid metric type")
	ErrValueRequired = errors.New("value is required for gauge")
	ErrDeltaRequired = errors.New("delta is required for counter")
)

// Metrics — формат обмена по HTTP (инкремент 7). Форма зафиксирована заданием
// дословно и намеренно отличается от внутренней Metric.
//
// Delta и Value — указатели, потому что omitempty на обычном поле выкидывает
// любое нулевое значение: легитимные Lookups=0, NumForcedGC=0 уехали бы без
// ключа "value", а получатель обязан видеть ровно одно из delta/value.
type Metrics struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
}

// Type разбирает поле MType (GetTypeMetric регистронезависим).
func (w Metrics) Type() MetricType { return GetTypeMetric(w.MType) }

// ToDomain конвертирует тело запроса во внутреннюю модель.
// Возвращает ошибку, а не панику: невалидное тело — это 400, а не 500.
func (w Metrics) ToDomain() (*Metric, error) {
	switch w.Type() {
	case Gauge:
		if w.Value == nil {
			// Отсутствующий value != value:0, молча подставлять 0 нельзя.
			return nil, ErrValueRequired
		}
		return NewGaugeMetric(w.ID, *w.Value), nil
	case Counter:
		if w.Delta == nil {
			return nil, ErrDeltaRequired
		}
		return NewCountMetric(w.ID, *w.Delta), nil
	default:
		return nil, ErrUnknownType
	}
}

// FromDomain строит тело ответа из внутренней модели.
//
// Указатели берутся от локальных копий (v := m.Value; &v), а не от полей m:
// иначе DTO алиасил бы объект хранилища.
func FromDomain(m *Metric) Metrics {
	out := Metrics{ID: m.ID, MType: MetricType(m.MType).WireName()}
	switch MetricType(m.MType) {
	case Gauge:
		v := m.Value
		out.Value = &v // всегда не-nil: даже 0 обязан попасть в ответ
	case Counter:
		d := m.Delta
		out.Delta = &d
	}
	return out
}

// NewGaugeDTO собирает тело запроса для gauge-метрики.
func NewGaugeDTO(id string, value float64) Metrics {
	v := value
	return Metrics{ID: id, MType: Gauge.WireName(), Value: &v}
}

// NewCounterDTO собирает тело запроса для counter-метрики.
func NewCounterDTO(id string, delta int64) Metrics {
	d := delta
	return Metrics{ID: id, MType: Counter.WireName(), Delta: &d}
}
