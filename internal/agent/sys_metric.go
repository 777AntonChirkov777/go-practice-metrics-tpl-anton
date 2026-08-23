package agent

import (
	"context"
	"practice/internal/logger"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"

	"go.uber.org/zap"
)

func (a *Agent) runSystemCollector(ctx context.Context) {
	defer a.wg.Done()

	a.collectSystemMetrics(ctx)

	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.collectSystemMetrics(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (a *Agent) collectSystemMetrics(ctx context.Context) {
	vm, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		logger.Log.Info("collect memory metrics failed", zap.Error(err))
		return
	}

	utilization, err := cpu.PercentWithContext(ctx, 0, true)
	if err != nil {
		logger.Log.Info("collect cpu metrics failed", zap.Error(err))
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.gauges["TotalMemory"] = float64(vm.Total)
	a.gauges["FreeMemory"] = float64(vm.Free)
	for i, percent := range utilization {
		a.gauges["CPUutilization"+strconv.Itoa(i+1)] = percent
	}
}
