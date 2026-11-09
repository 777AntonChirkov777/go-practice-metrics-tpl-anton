package agent

import (
	"encoding/json"
	"testing"
	"time"

	models "practice/internal/model"
)

const benchAgentKey = "benchmark-signing-key"

var (
	benchSnapshotSink []models.Metrics
	benchGzipSink     []byte
)

func benchAgent() *Agent {
	a := NewAgent(time.Second, time.Second, "http://127.0.0.1:8080", benchAgentKey, 1)
	a.CollectMetrics()
	return a
}

func BenchmarkAgentCollectMetrics(b *testing.B) {
	a := NewAgent(time.Second, time.Second, "http://127.0.0.1:8080", benchAgentKey, 1)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.CollectMetrics()
	}
}

func BenchmarkAgentSnapshot(b *testing.B) {
	a := benchAgent()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchSnapshotSink = a.snapshot()
	}
}

func BenchmarkAgentGzipJSON(b *testing.B) {
	payload, err := json.Marshal(benchAgent().snapshot())
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, err := gzipJSON(payload)
		if err != nil {
			b.Fatal(err)
		}
		benchGzipSink = out
	}
}
