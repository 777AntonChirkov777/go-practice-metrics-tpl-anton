package retry

import (
	"context"
	"time"

	"practice/internal/logger"

	"go.uber.org/zap"
)

// DefaultDelays - интервалы между повторами: 1s, 3s, 5s
var DefaultDelays = []time.Duration{time.Second, 3 * time.Second, 5 * time.Second}

func Do(
	ctx context.Context,
	op string,
	delays []time.Duration,
	retriable func(error) bool,
	fn func(context.Context) error,
) error {
	err := fn(ctx)

	for i, delay := range delays {
		if err == nil || !retriable(err) {
			return err
		}

		logger.Log.Info("retrying after error",
			zap.String("operation", op),
			zap.Int("attempt", i+1),
			zap.Duration("delay", delay),
			zap.Error(err))

		if !wait(ctx, delay) {
			return err
		}

		if ctx.Err() != nil {
			return err
		}

		err = fn(ctx)
	}

	return err
}

func wait(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
