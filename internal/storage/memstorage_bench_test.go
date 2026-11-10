package storage

import (
	"context"
	"strconv"
	"testing"

	model "practice/internal/model"
)

const benchStoreSize = 100

var (
	benchMetricSink  *model.Metric
	benchMetricsSink []model.Metric
)

func benchGauges(n int) []*model.Metric {
	out := make([]*model.Metric, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, model.NewGaugeMetric("gauge"+strconv.Itoa(i), float64(i)+0.5))
	}
	return out
}

func benchCounters(n int) []*model.Metric {
	out := make([]*model.Metric, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, model.NewCountMetric("counter"+strconv.Itoa(i), int64(i)))
	}
	return out
}

func benchGaugeNames(n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, "gauge"+strconv.Itoa(i))
	}
	return out
}

func benchPreloaded(gauges, counters int) *MemStorage {
	s := NewMemStorage()
	s.LoadAll(benchGauges(gauges))
	s.LoadAll(benchCounters(counters))
	return s
}

func BenchmarkMemStorageSaveExisting(b *testing.B) {
	ctx := context.Background()
	s := benchPreloaded(benchStoreSize, benchStoreSize)
	metrics := benchGauges(benchStoreSize)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := s.Save(ctx, metrics[i%len(metrics)]); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemStorageSaveNew(b *testing.B) {
	ctx := context.Background()
	metrics := benchGauges(1024)
	s := NewMemStorage()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := i % len(metrics)
		if idx == 0 {
			s = NewMemStorage()
		}
		if err := s.Save(ctx, metrics[idx]); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemStorageSaveBatch(b *testing.B) {
	ctx := context.Background()
	s := benchPreloaded(benchStoreSize, benchStoreSize)
	batch := append(benchGauges(28), model.NewCountMetric("counter0", 1))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := s.SaveBatch(ctx, batch); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemStorageGet(b *testing.B) {
	ctx := context.Background()
	s := benchPreloaded(benchStoreSize, benchStoreSize)
	names := benchGaugeNames(benchStoreSize)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m, err := s.Get(ctx, model.Gauge, names[i%len(names)])
		if err != nil {
			b.Fatal(err)
		}
		benchMetricSink = m
	}
}

func BenchmarkMemStorageGetAll(b *testing.B) {
	ctx := context.Background()
	s := benchPreloaded(benchStoreSize, benchStoreSize)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		all, err := s.GetAll(ctx)
		if err != nil {
			b.Fatal(err)
		}
		benchMetricsSink = all
	}
}
