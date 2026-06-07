package channel

import (
	"context"
	"time"
)

const defaultRetryWorkerInterval = time.Minute

type RetryWorkerConfig struct {
	Interval time.Duration
	Limit    int
	Now      func() time.Time
	Ticks    <-chan time.Time
	OnError  func(error)
}

type RetryWorker struct {
	service  *Service
	store    RetryWorkerStore
	interval time.Duration
	limit    int
	now      func() time.Time
	ticks    <-chan time.Time
	onError  func(error)
}

func NewRetryWorker(service *Service, store RetryWorkerStore, config RetryWorkerConfig) *RetryWorker {
	interval := config.Interval
	if interval <= 0 {
		interval = defaultRetryWorkerInterval
	}
	now := config.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &RetryWorker{
		service:  service,
		store:    store,
		interval: interval,
		limit:    config.Limit,
		now:      now,
		ticks:    config.Ticks,
		onError:  config.OnError,
	}
}

func (w *RetryWorker) Run(ctx context.Context) {
	if w == nil || w.service == nil || w.store == nil {
		return
	}

	ticks := w.ticks
	var ticker *time.Ticker
	if ticks == nil {
		ticker = time.NewTicker(w.interval)
		defer ticker.Stop()
		ticks = ticker.C
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			w.runOnce(ctx)
		}
	}
}

func (w *RetryWorker) runOnce(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}
	if _, err := w.service.ProcessDueRetryMessages(ctx, w.store, ClaimDueRetryMessagesInput{
		Now:   w.now(),
		Limit: w.limit,
	}); err != nil && w.onError != nil {
		w.onError(err)
	}
}
