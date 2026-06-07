package channel

import (
	"context"
	"sync"
	"testing"
	"time"
)

type retryWorkerTestStore struct {
	mu         sync.Mutex
	claimCalls int
	claimInput ClaimDueRetryMessagesInput
}

func (s *retryWorkerTestStore) ListDueRetryMessages(ctx context.Context, input ClaimDueRetryMessagesInput) ([]*ChannelMessageLog, error) {
	return nil, nil
}

func (s *retryWorkerTestStore) ClaimDueRetryMessages(ctx context.Context, input ClaimDueRetryMessagesInput) ([]*ChannelMessageLog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claimCalls++
	s.claimInput = input
	return nil, nil
}

func (s *retryWorkerTestStore) GetConfigByID(ctx context.Context, id string) (*ChannelConfig, error) {
	return nil, nil
}

func (s *retryWorkerTestStore) UpdateRetryMessageLog(ctx context.Context, log *ChannelMessageLog) (*ChannelMessageLog, error) {
	return log, nil
}

func (s *retryWorkerTestStore) UpdateConfigStatus(ctx context.Context, organizationID, id string, status ChannelStatus) (*ChannelConfig, error) {
	return nil, nil
}

func (s *retryWorkerTestStore) CountConsecutiveDeliveryFailures(ctx context.Context, channelID string, limit int) (int, error) {
	return 0, nil
}

func (s *retryWorkerTestStore) CountConsecutiveSuccessfulDeliveries(ctx context.Context, channelID string, limit int) (int, error) {
	return 0, nil
}

func (s *retryWorkerTestStore) snapshot() (int, ClaimDueRetryMessagesInput) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.claimCalls, s.claimInput
}

func TestChannelRetryWorkerProcessesDueMessagesOnTicker(t *testing.T) {
	now := time.Date(2026, time.June, 5, 10, 0, 0, 0, time.UTC)
	store := &retryWorkerTestStore{}
	ticks := make(chan time.Time)
	worker := NewRetryWorker(NewService(NewAdapterRegistry(nil)), store, RetryWorkerConfig{
		Ticks: ticks,
		Now:   func() time.Time { return now },
		Limit: 9,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()

	ticks <- now

	deadline := time.After(250 * time.Millisecond)
	for {
		calls, input := store.snapshot()
		if calls == 1 {
			if input.Limit != 9 || !input.Now.Equal(now) {
				t.Fatalf("expected worker to pass configured now/limit, got %+v", input)
			}
			cancel()
			select {
			case <-done:
			case <-time.After(250 * time.Millisecond):
				t.Fatal("expected worker to stop after context cancellation")
			}
			return
		}
		select {
		case <-deadline:
			t.Fatalf("expected worker to process one due retry batch after ticker, got %d calls", calls)
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func TestChannelRetryWorkerDoesNotBusyLoopBetweenTicks(t *testing.T) {
	store := &retryWorkerTestStore{}
	ticks := make(chan time.Time)
	worker := NewRetryWorker(NewService(NewAdapterRegistry(nil)), store, RetryWorkerConfig{Ticks: ticks})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()

	time.Sleep(25 * time.Millisecond)
	if calls, _ := store.snapshot(); calls != 0 {
		cancel()
		t.Fatalf("expected worker not to process without a tick, got %d calls", calls)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected worker to stop after context cancellation")
	}
}
