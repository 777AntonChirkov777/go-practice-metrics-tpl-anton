package audit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingSink struct {
	events []Event
	err    error
	closed bool
}

func (s *recordingSink) Notify(_ context.Context, e Event) error {
	if s.err != nil {
		return s.err
	}

	s.events = append(s.events, e)
	return nil
}

func (s *recordingSink) Close() error {
	s.closed = true
	return nil
}

func TestNotifier_DeliversToEveryObserver(t *testing.T) {
	first, second := &recordingSink{}, &recordingSink{}

	n := NewNotifier()
	n.Subscribe(first)
	n.Subscribe(second)
	require.True(t, n.Enabled())
	n.Start()

	before := time.Now().Unix()
	n.Record([]string{"Alloc", "PollCount"}, "192.168.0.42")
	n.Close()
	after := time.Now().Unix()

	require.Len(t, first.events, 1)
	require.Len(t, second.events, 1)

	assert.Equal(t, []string{"Alloc", "PollCount"}, first.events[0].Metrics)
	assert.Equal(t, "192.168.0.42", first.events[0].IPAddress)
	assert.GreaterOrEqual(t, first.events[0].Ts, before)
	assert.LessOrEqual(t, first.events[0].Ts, after)
	assert.Equal(t, first.events[0], second.events[0])

	assert.True(t, first.closed)
	assert.True(t, second.closed)
}

func TestNotifier_FailingObserverDoesNotStopTheRest(t *testing.T) {
	failing := &recordingSink{err: errors.New("sink is down")}
	healthy := &recordingSink{}

	n := NewNotifier()
	n.Subscribe(failing)
	n.Subscribe(healthy)
	n.Start()

	n.Record([]string{"Alloc"}, "10.0.0.7")
	n.Close()

	require.Len(t, healthy.events, 1)
	assert.Empty(t, failing.events)
}

func TestNotifier_RecordKeepsCallerSliceIntact(t *testing.T) {
	sink := &recordingSink{}

	n := NewNotifier()
	n.Subscribe(sink)
	n.Start()

	names := []string{"Alloc", "Frees"}
	n.Record(names, "10.0.0.7")
	names[0] = "Mutated"
	n.Close()

	require.Len(t, sink.events, 1)
	assert.Equal(t, []string{"Alloc", "Frees"}, sink.events[0].Metrics)
}

func TestNotifier_DropsEventWhenQueueIsFull(t *testing.T) {
	n := NewNotifier()
	n.Subscribe(&recordingSink{})

	for i := 0; i < queueCapacity; i++ {
		n.Record([]string{"Alloc"}, "10.0.0.7")
	}
	require.Equal(t, queueCapacity, len(n.events))

	n.Record([]string{"Overflow"}, "10.0.0.7")
	assert.Equal(t, queueCapacity, len(n.events))
}

func TestNotifier_CloseDrainsPendingEvents(t *testing.T) {
	const total = 50

	sink := &recordingSink{}

	n := NewNotifier()
	n.Subscribe(sink)
	n.Start()

	for i := 0; i < total; i++ {
		n.Record([]string{"Alloc"}, "10.0.0.7")
	}
	n.Close()

	assert.Len(t, sink.events, total)
	assert.True(t, sink.closed)
}

func TestNotifier_CloseIsIdempotent(t *testing.T) {
	sink := &recordingSink{}

	n := NewNotifier()
	n.Subscribe(sink)
	n.Start()

	n.Record([]string{"Alloc"}, "10.0.0.7")
	n.Close()
	n.Close()

	assert.Len(t, sink.events, 1)
}

type blockingSink struct {
	entered chan struct{}
	release chan struct{}
}

func (s *blockingSink) Notify(_ context.Context, _ Event) error {
	select {
	case s.entered <- struct{}{}:
	default:
	}

	<-s.release
	return nil
}

func TestNotifier_SlowObserverDoesNotHoldClose(t *testing.T) {
	sink := &blockingSink{
		entered: make(chan struct{}, 1),
		release: make(chan struct{}),
	}

	n := NewNotifier()
	n.Subscribe(sink)
	n.drainWait = 100 * time.Millisecond
	n.Start()

	n.Record([]string{"Alloc"}, "10.0.0.7")
	<-sink.entered

	start := time.Now()
	n.Close()
	elapsed := time.Since(start)

	close(sink.release)

	assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond)
	assert.Less(t, elapsed, time.Second, "Close обязан вернуться по таймауту дренажа, не дожидаясь приёмника")
}

func TestNotifier_WithoutObserversDoesNothing(t *testing.T) {
	n := NewNotifier()
	n.Start()

	assert.False(t, n.Enabled())

	n.Record([]string{"Alloc"}, "10.0.0.7")
	assert.Equal(t, 0, len(n.events))

	start := time.Now()
	n.Close()
	assert.Less(t, time.Since(start), drainTimeout)
}
