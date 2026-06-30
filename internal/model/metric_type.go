package models

import (
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
	return [...]string{"Unknown", "Gauge", "Counter"}[m]
}

func GetTypeMetric(typeMetric string) MetricType {
	switch strings.ToLower(typeMetric) {
	case strings.ToLower(Gauge.String()):
		return Gauge
	case strings.ToLower(Counter.String()):
		return Counter
	default:
		return Unknown
	}
}

func IsParseMetricValue(typeMetric MetricType, valueStr string) bool {
	switch typeMetric {
	case Gauge:
		if _, err := strconv.ParseFloat(valueStr, 64); err == nil {
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
	return strconv.ParseFloat(valueStr, 64)
}

func ParseCounter(typeMetric MetricType, valueStr string) (int64, error) {
	return strconv.ParseInt(valueStr, 10, 64)
}
