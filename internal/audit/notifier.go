package audit

import (
	"context"
	"io"
	"sync"
	"time"

	"practice/internal/logger"

	"go.uber.org/zap"
)

const (
	queueCapacity = 1024
	drainTimeout  = 5 * time.Second
	notifyTimeout = 5 * time.Second
)

type Notifier struct {
	observers []Observer
	events    chan Event
	quit      chan struct{}
	done      chan struct{}
	drainWait time.Duration
	started   bool
	closeOnce sync.Once
}

func NewNotifier() *Notifier {
	return &Notifier{
		events:    make(chan Event, queueCapacity),
		quit:      make(chan struct{}),
		done:      make(chan struct{}),
		drainWait: drainTimeout,
	}
}

func (n *Notifier) Subscribe(o Observer) {
	n.observers = append(n.observers, o)
}

func (n *Notifier) Enabled() bool {
	return len(n.observers) > 0
}

func (n *Notifier) Start() {
	if !n.Enabled() || n.started {
		return
	}

	n.started = true
	go n.run()
}

func (n *Notifier) Record(metrics []string, ip string) {
	if !n.Enabled() {
		return
	}

	e := Event{
		Timestamp: time.Now().Unix(),
		Metrics:   append([]string(nil), metrics...),
		IPAddress: ip,
	}

	select {
	case n.events <- e:
	default:
		logger.Log.Info("audit event dropped: queue is full",
			zap.Int("metrics", len(e.Metrics)),
			zap.String("ip", e.IPAddress))
	}
}

func (n *Notifier) Close() {
	n.closeOnce.Do(func() {
		if n.started {
			close(n.quit)

			select {
			case <-n.done:
			case <-time.After(n.drainWait):
				logger.Log.Info("audit drain timed out",
					zap.Duration("timeout", n.drainWait),
					zap.Int("pending", len(n.events)))
			}
		}

		n.closeObservers()
	})
}

func (n *Notifier) run() {
	defer close(n.done)

	for {
		select {
		case e := <-n.events:
			n.deliver(e)
		case <-n.quit:
			n.drain()
			return
		}
	}
}

func (n *Notifier) drain() {
	for {
		select {
		case e := <-n.events:
			n.deliver(e)
		default:
			return
		}
	}
}

func (n *Notifier) deliver(e Event) {
	ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
	defer cancel()

	for _, o := range n.observers {
		if err := o.Notify(ctx, e); err != nil {
			logger.Log.Info("audit delivery failed", zap.Error(err))
		}
	}
}

func (n *Notifier) closeObservers() {
	for _, o := range n.observers {
		c, ok := o.(io.Closer)
		if !ok {
			continue
		}
		if err := c.Close(); err != nil {
			logger.Log.Info("audit sink close failed", zap.Error(err))
		}
	}
}
