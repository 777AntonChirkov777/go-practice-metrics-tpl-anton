package model

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type MetricType int8

const (
	Unknown MetricType = iota // 0 - значение по умолчанию
	Gauge                     // 1
	Counter                   // 2
)

func (m MetricType) String() string {
	switch m {
	case Gauge:
		return "Gauge"
	case Counter:
		return "Counter"
	default:
		return "Unknown"
	}
}

func GetTypeMetric(typeMetric string) MetricType {
	switch {
	case strings.EqualFold(typeMetric, Gauge.WireName()):
		return Gauge
	case strings.EqualFold(typeMetric, Counter.WireName()):
		return Counter
	default:
		return Unknown
	}
}

func IsParseMetricValue(typeMetric MetricType, valueStr string) bool {
	switch typeMetric {
	case Gauge:
		if _, err := ParseGauge(typeMetric, valueStr); err == nil {
			return true
		} else {
			return false
		}
	case Counter:
		if _, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
			return true
		} else {
			return false
		}
	default:
		return false
	}
}

func ParseGauge(typeMetric MetricType, valueStr string) (float64, error) {
	v, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("gauge value %q is not a finite number", valueStr)
	}
	return v, nil
}

func ParseCounter(typeMetric MetricType, valueStr string) (int64, error) {
	return strconv.ParseInt(valueStr, 10, 64)
}

func (m MetricType) WireName() string {
	switch m {
	case Gauge:
		return "gauge"
	case Counter:
		return "counter"
	default:
		return "unknown"
	}
}
