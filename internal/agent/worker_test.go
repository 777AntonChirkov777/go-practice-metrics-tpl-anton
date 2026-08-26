package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPoolLimitsConcurrentRequests(t *testing.T) {
	const (
		limit = 2
		jobs  = 6
	)

	var inFlight, maxInFlight atomic.Int64

	arrived := make(chan struct{}, jobs)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := inFlight.Add(1)
		for {
			peak := maxInFlight.Load()
			if current <= peak || maxInFlight.CompareAndSwap(peak, current) {
				break
			}
		}

		arrived <- struct{}{}
		<-release

		inFlight.Add(-1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	defer releaseAll()

	agent := NewAgent(time.Hour, time.Hour, srv.URL, "", limit)
	agent.CollectMetrics()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agent.wg.Add(limit)
	for i := 0; i < limit; i++ {
		go agent.runWorker(ctx)
	}

	sent := make(chan struct{})
	go func() {
		defer close(sent)
		for i := 0; i < jobs; i++ {
			select {
			case agent.jobs <- agent.snapshot():
			case <-ctx.Done():
				return
			}
		}
	}()

	for i := 0; i < limit; i++ {
		select {
		case <-arrived:
		case <-time.After(5 * time.Second):
			t.Fatal("воркеры не начали отправку")
		}
	}

	select {
	case <-arrived:
		t.Fatal("в полёте больше запросов, чем разрешает предел")
	case <-time.After(200 * time.Millisecond):
	}

	releaseAll()

	for i := limit; i < jobs; i++ {
		select {
		case <-arrived:
		case <-time.After(5 * time.Second):
			t.Fatal("не все задания дошли до сервера")
		}
	}

	select {
	case <-sent:
	case <-time.After(5 * time.Second):
		t.Fatal("часть заданий так и не роздана воркерам")
	}

	cancel()
	agent.Wait()

	if peak := maxInFlight.Load(); peak > limit {
		t.Errorf("максимум одновременных запросов = %d, want <= %d", peak, limit)
	}
}

func TestDispatchQueuesReportWhileWorkerIsBusy(t *testing.T) {
	var requests atomic.Int64

	arrived := make(chan struct{}, 4)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		arrived <- struct{}{}
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	defer releaseAll()

	agent := NewAgent(time.Hour, time.Hour, srv.URL, "", 1)
	agent.CollectMetrics()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agent.wg.Add(1)
	go agent.runWorker(ctx)

	agent.jobs <- agent.snapshot()

	select {
	case <-arrived:
	case <-time.After(5 * time.Second):
		t.Fatal("воркер не начал отправку")
	}

	if !agent.dispatch(ctx) {
		t.Fatal("отчёт отброшен, хотя в очереди есть место")
	}

	releaseAll()

	select {
	case <-arrived:
	case <-time.After(5 * time.Second):
		t.Fatal("отчёт из очереди не дошёл до сервера")
	}

	if got := requests.Load(); got != 2 {
		t.Errorf("requests = %d, want 2: отчёт из очереди обязан уйти на сервер", got)
	}

	gauges, pollCount := agent.GetMetrics()
	if pollCount < 1 {
		t.Errorf("pollCount = %d, want >= 1: постановка в очередь не должна терять состояние", pollCount)
	}
	if _, ok := gauges["Alloc"]; !ok {
		t.Error("постановка в очередь не должна терять собранные метрики")
	}

	cancel()
	agent.Wait()
}

func TestDispatchWaitsWhenQueueIsFull(t *testing.T) {
	arrived := make(chan struct{}, 4)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arrived <- struct{}{}
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	defer releaseAll()

	agent := NewAgent(time.Hour, time.Hour, srv.URL, "", 1)
	agent.CollectMetrics()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agent.wg.Add(1)
	go agent.runWorker(ctx)

	agent.jobs <- agent.snapshot()

	select {
	case <-arrived:
	case <-time.After(5 * time.Second):
		t.Fatal("воркер не начал отправку")
	}

	if !agent.dispatch(ctx) {
		t.Fatal("отчёт отброшен, хотя в очереди есть место")
	}

	queued := make(chan bool, 1)
	go func() { queued <- agent.dispatch(ctx) }()

	select {
	case <-queued:
		t.Fatal("dispatch вернулся, хотя очередь заполнена, а единственный воркер занят")
	case <-time.After(200 * time.Millisecond):
	}

	_, before := agent.GetMetrics()
	agent.CollectMetrics()
	_, after := agent.GetMetrics()
	if after <= before {
		t.Errorf("pollCount = %d, было %d: сбор обязан продолжаться, пока отправка ждёт воркера", after, before)
	}

	releaseAll()

	select {
	case ok := <-queued:
		if !ok {
			t.Error("dispatch вернул false, хотя воркер освободился")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("dispatch не разблокировался после освобождения воркера")
	}

	cancel()
	agent.Wait()
}

func TestDispatchStopsWaitingOnCancel(t *testing.T) {
	agent := NewAgent(time.Hour, time.Hour, "http://metrics.invalid", "", 1)
	agent.CollectMetrics()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agent.jobs <- agent.snapshot()

	queued := make(chan bool, 1)
	go func() { queued <- agent.dispatch(ctx) }()

	select {
	case <-queued:
		t.Fatal("dispatch вернулся, хотя очередь заполнена и свободных воркеров нет")
	case <-time.After(200 * time.Millisecond):
	}

	cancel()

	select {
	case ok := <-queued:
		if ok {
			t.Error("dispatch вернул true, хотя контекст отменён")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("dispatch не разблокировался после отмены контекста")
	}
}

func TestStartStopsAllGoroutines(t *testing.T) {
	arrived := make(chan struct{}, 64)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case arrived <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	agent := NewAgent(10*time.Millisecond, 10*time.Millisecond, srv.URL, "", 3)

	ctx, cancel := context.WithCancel(context.Background())
	agent.Start(ctx)

	select {
	case <-arrived:
	case <-time.After(5 * time.Second):
		cancel()
		agent.Wait()
		t.Fatal("агент не отправил ни одного отчёта")
	}

	cancel()

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		agent.Wait()
	}()

	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Wait() не вернулся после отмены контекста")
	}
}
