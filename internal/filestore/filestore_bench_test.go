package filestore

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"

	model "practice/internal/model"
	"practice/internal/storage"
)

const benchMetricCount = 100

var benchLoadSink []*model.Metric

func benchSnapshotSource(n int) *storage.MemStorage {
	metrics := make([]*model.Metric, 0, n+1)
	for i := 0; i < n; i++ {
		metrics = append(metrics, model.NewGaugeMetric("gauge"+strconv.Itoa(i), float64(i)+0.5))
	}
	metrics = append(metrics, model.NewCountMetric("PollCount", int64(n)))

	s := storage.NewMemStorage()
	s.LoadAll(metrics)

	return s
}

func BenchmarkFileStoreSaveFrom(b *testing.B) {
	ctx := context.Background()
	src := benchSnapshotSource(benchMetricCount)
	fs := New(filepath.Join(b.TempDir(), "metrics-db.json"))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := fs.SaveFrom(ctx, src); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFileStoreLoad(b *testing.B) {
	fs := New(filepath.Join(b.TempDir(), "metrics-db.json"))
	if err := fs.SaveFrom(context.Background(), benchSnapshotSource(benchMetricCount)); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, err := fs.Load()
		if err != nil {
			b.Fatal(err)
		}
		benchLoadSink = out
	}
}
