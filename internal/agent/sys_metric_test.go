package agent

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
)

func TestCollectSystemMetrics(t *testing.T) {
	agent := NewAgent(time.Second, time.Second, "http://localhost", "", 1)
	agent.collectSystemMetrics(context.Background())

	gauges, _ := agent.GetMetrics()

	if gauges["TotalMemory"] <= 0 {
		t.Errorf("TotalMemory = %v, want > 0", gauges["TotalMemory"])
	}
	if gauges["FreeMemory"] <= 0 {
		t.Errorf("FreeMemory = %v, want > 0", gauges["FreeMemory"])
	}

	count, err := cpu.Counts(true)
	if err != nil {
		t.Fatalf("не удалось узнать число логических процессоров: %v", err)
	}
	if count < 1 {
		t.Fatalf("logical CPUs = %d, want >= 1", count)
	}

	for i := 1; i <= count; i++ {
		name := "CPUutilization" + strconv.Itoa(i)
		if _, ok := gauges[name]; !ok {
			t.Errorf("missing gauge metric: %s", name)
		}
	}

	if _, ok := gauges["CPUutilization0"]; ok {
		t.Error("нумерация CPUutilization начинается с единицы")
	}
	if _, ok := gauges["CPUutilization"+strconv.Itoa(count+1)]; ok {
		t.Errorf("метрик CPUutilization больше, чем логических процессоров (%d)", count)
	}
}

func TestCollectSystemMetricsGoesIntoReportBatch(t *testing.T) {
	agent := NewAgent(time.Second, time.Second, "http://localhost", "", 1)
	agent.CollectMetrics()
	agent.collectSystemMetrics(context.Background())

	batch := agent.snapshot()

	found := map[string]bool{}
	for _, m := range batch {
		found[m.ID] = true
	}

	for _, name := range []string{"TotalMemory", "FreeMemory", "CPUutilization1", "Alloc", "PollCount"} {
		if !found[name] {
			t.Errorf("метрика %s не попала в батч отчёта", name)
		}
	}
}
