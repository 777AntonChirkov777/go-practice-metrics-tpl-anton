package services

import (
	models "practice/internal/model"
	repositories "practice/internal/repository"
	"strings"
)

func SaveMetric(typeMetric models.MetricType, name, value string) error {
	if typeMetric != models.Gauge && typeMetric != models.Counter {
		panic("invalid metric type: " + typeMetric.String())
	}

	if strings.TrimSpace(name) == "" {
		panic("invalid name! Name cannot be empty")
	}

	if strings.TrimSpace(value) == "" {
		panic("invalid name! Name cannot be empty")
	}

	typeMetricSave(typeMetric, name, value)

	return nil
}

func typeMetricSave(typeMetric models.MetricType, name, value string) error {

	metricFind, _ := repositories.FindMetric(name, int8(typeMetric))

	switch typeMetric {
	case models.Gauge:
		parsVal, _ := models.ParseGauge(typeMetric, value)
		gauge(name, parsVal, metricFind)
	case models.Counter:
		parsVal, _ := models.ParseCounter(typeMetric, value)
		counter(name, parsVal, metricFind)
	}

	return nil
}

func gauge(name string, value float64, metricFind *models.Metric) {
	if metricFind != nil {
		if (metricFind).Value == value {
			return
		}
		metricFind.Value = value
		repositories.SaveMetric(*metricFind)
	}

	metric := models.NewGaugeMetric(name, value)
	repositories.SaveMetric(*metric)
}

func counter(name string, value int64, metricFind *models.Metric) {
	if metricFind != nil {
		metricFind.Delta = metricFind.Delta + 1
		repositories.SaveMetric(*metricFind)
		return
	}

	metric := models.NewCountMetric(name, value)
	repositories.SaveMetric(*metric)
}
