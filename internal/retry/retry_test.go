package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

var errBoom = errors.New("boom")

func always(error) bool { return true }

func never(error) bool { return false }

func zeroDelays() []time.Duration { return []time.Duration{0, 0, 0} }

func TestDo_SuccessOnFirstAttempt(t *testing.T) {
	calls := 0

	err := Do(context.Background(), "op", zeroDelays(), always, func(context.Context) error {
		calls++
		return nil
	})

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestDo_SuccessOnThirdAttempt(t *testing.T) {
	calls := 0

	err := Do(context.Background(), "op", zeroDelays(), always, func(context.Context) error {
		calls++
		if calls < 3 {
			return errBoom
		}
		return nil
	})

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestDo_ExhaustsAttempts(t *testing.T) {
	calls := 0

	err := Do(context.Background(), "op", zeroDelays(), always, func(context.Context) error {
		calls++
		return errBoom
	})

	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want %v", err, errBoom)
	}
	if calls != 4 {
		t.Errorf("calls = %d, want 4 (первая попытка и три повтора)", calls)
	}
}

func TestDo_NonRetriableErrorStopsImmediately(t *testing.T) {
	calls := 0

	err := Do(context.Background(), "op", zeroDelays(), never, func(context.Context) error {
		calls++
		return errBoom
	})

	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want %v", err, errBoom)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestDo_EmptyDelaysMeansSingleAttempt(t *testing.T) {
	calls := 0

	err := Do(context.Background(), "op", nil, always, func(context.Context) error {
		calls++
		return errBoom
	})

	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want %v", err, errBoom)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestDo_CanceledContextBreaksSeries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	calls := 0
	delays := []time.Duration{time.Hour, time.Hour, time.Hour}

	err := Do(ctx, "op", delays, always, func(context.Context) error {
		calls++
		cancel()
		return errBoom
	})

	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v, want %v", err, errBoom)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1: отмена контекста обрывает серию до паузы", calls)
	}
}

func TestDefaultDelays(t *testing.T) {
	want := []time.Duration{time.Second, 3 * time.Second, 5 * time.Second}

	if len(DefaultDelays) != len(want) {
		t.Fatalf("DefaultDelays = %v, want %v", DefaultDelays, want)
	}
	for i := range want {
		if DefaultDelays[i] != want[i] {
			t.Errorf("DefaultDelays[%d] = %v, want %v", i, DefaultDelays[i], want[i])
		}
	}
}
