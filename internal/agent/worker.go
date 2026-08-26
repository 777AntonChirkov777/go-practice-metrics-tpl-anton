package agent

import (
	"context"
	"practice/internal/logger"
	"time"

	"go.uber.org/zap"
)

func (a *Agent) runDispatcher(ctx context.Context) {
	defer a.wg.Done()
	defer close(a.jobs)

	ticker := time.NewTicker(a.reportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.dispatch(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (a *Agent) dispatch(ctx context.Context) bool {
	batch := a.snapshot()
	if len(batch) == 0 {
		return false
	}

	select {
	case a.jobs <- batch:
		return true
	default:
	}

	logger.Log.Info("report waiting: all workers are busy",
		zap.Int("metrics", len(batch)),
		zap.Int("rate_limit", a.rateLimit))

	select {
	case a.jobs <- batch:
		return true
	case <-ctx.Done():
		return false
	}
}

func (a *Agent) runWorker(ctx context.Context) {
	defer a.wg.Done()

	for {
		select {
		case batch, ok := <-a.jobs:
			if !ok {
				return
			}
			a.sendBatch(ctx, batch)
		case <-ctx.Done():
			return
		}
	}
}
