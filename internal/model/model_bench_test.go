package models

import "testing"

var (
	benchDomainSink *Metric
	benchWireSink   Metrics
	benchTypeSink   MetricType
	benchStringSink string
)

func BenchmarkMetricCalculateHash(b *testing.B) {
	m := NewGaugeMetric("HeapAlloc", 123456.75)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := m.CalculateHash(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMetricsToDomainGauge(b *testing.B) {
	in := NewGaugeDTO("HeapAlloc", 123456.75)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m, err := in.ToDomain()
		if err != nil {
			b.Fatal(err)
		}
		benchDomainSink = m
	}
}

func BenchmarkMetricsToDomainCounter(b *testing.B) {
	in := NewCounterDTO("PollCount", 42)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m, err := in.ToDomain()
		if err != nil {
			b.Fatal(err)
		}
		benchDomainSink = m
	}
}

func BenchmarkFromDomainGauge(b *testing.B) {
	m := NewGaugeMetric("HeapAlloc", 123456.75)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchWireSink = FromDomain(m)
	}
}

func BenchmarkFromDomainCounter(b *testing.B) {
	m := NewCountMetric("PollCount", 42)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchWireSink = FromDomain(m)
	}
}

func BenchmarkGetTypeMetricLowerCase(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchTypeSink = GetTypeMetric("counter")
	}
}

func BenchmarkGetTypeMetricMixedCase(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchTypeSink = GetTypeMetric("Gauge")
	}
}

func BenchmarkGetTypeMetricUnknown(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchTypeSink = GetTypeMetric("histogram")
	}
}

func BenchmarkMetricTypeString(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchStringSink = Gauge.String()
	}
}
