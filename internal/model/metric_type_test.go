package models

import (
	"testing"
)

func TestMetricType_String(t *testing.T) {
	tests := []struct {
		m        MetricType
		expected string
	}{
		{Unknown, "Unknown"},
		{Gauge, "Gauge"},
		{Counter, "Counter"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.m.String(); got != tt.expected {
				t.Errorf("String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetTypeMetric(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected MetricType
	}{
		{"lowercase gauge", "gauge", Gauge},
		{"mixed case gauge", "GauGe", Gauge},
		{"lowercase counter", "counter", Counter},
		{"mixed case counter", "CounTER", Counter},
		{"unknown empty", "", Unknown},
		{"unknown gibberish", "xyz", Unknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetTypeMetric(tt.input); got != tt.expected {
				t.Errorf("GetTypeMetric(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsParseMetricValue(t *testing.T) {
	tests := []struct {
		name     string
		typeM    MetricType
		value    string
		expected bool
	}{
		// Gauge valid
		{"gauge integer", Gauge, "42", true},
		{"gauge float", Gauge, "3.14", true},
		{"gauge scientific", Gauge, "1e2", true},
		{"gauge negative", Gauge, "-0.5", true},
		{"gauge invalid", Gauge, "abc", false},
		{"gauge empty", Gauge, "", false},
		// Counter valid
		{"counter positive", Counter, "100", true},
		{"counter negative", Counter, "-1", true},
		{"counter zero", Counter, "0", true},
		{"counter float", Counter, "1.5", false}, // ParseInt rejects decimal
		{"counter text", Counter, "100a", false}, // trailing characters invalid
		{"counter empty", Counter, "", false},
		// Unknown always false
		{"unknown any", Unknown, "123", false},
		{"unknown empty", Unknown, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsParseMetricValue(tt.typeM, tt.value); got != tt.expected {
				t.Errorf("IsParseMetricValue(%v, %q) = %v, want %v", tt.typeM, tt.value, got, tt.expected)
			}
		})
	}
}

func TestParseGauge(t *testing.T) {
	tests := []struct {
		name     string
		typeM    MetricType // ignored but present for signature
		value    string
		expected float64
		wantErr  bool
	}{
		{"valid int", Gauge, "42", 42.0, false},
		{"valid float", Gauge, "3.14", 3.14, false},
		{"valid negative", Counter, "-10.5", -10.5, false}, // uses Counter type but still works
		{"invalid", Unknown, "abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseGauge(tt.typeM, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGauge() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("ParseGauge() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseCounter(t *testing.T) {
	tests := []struct {
		name     string
		typeM    MetricType // ignored
		value    string
		expected int64
		wantErr  bool
	}{
		{"positive", Gauge, "123", 123, false},
		{"negative", Gauge, "-1", -1, false},
		{"zero", Unknown, "0", 0, false},
		{"float string", Counter, "2.5", 0, true}, // not a valid int64
		{"invalid", Counter, "xyz", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCounter(tt.typeM, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCounter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("ParseCounter() = %v, want %v", got, tt.expected)
			}
		})
	}
}
