package main

import (
	"context"
	"testing"
	"time"

	"oblivious/server/internal/config"
	"oblivious/server/internal/relay"
)

func TestBuildStandaloneRelayConfigWiresBatchCommercialLifecycle(t *testing.T) {
	store := &relay.RelayStore{}
	relayConfig := buildStandaloneRelayConfig(
		config.Config{
			Env:                                     "production",
			CORSAllowedOrigins:                      []string{"https://console.example.test"},
			RelayBatchCommercialLifecycleEnabled:    true,
			RelayRealtimeCommercialLifecycleEnabled: true,
		},
		relay.NewChannelPool(),
		relay.NewPricingStoreWithDefaults(),
		nil,
		nil,
		store,
		nil,
	)

	if !relayConfig.Production {
		t.Fatal("expected production relay config")
	}
	if !relayConfig.BatchCommercialLifecycleEnabled {
		t.Fatal("expected batch commercial lifecycle flag to be forwarded")
	}
	if !relayConfig.RealtimeCommercialLifecycleEnabled {
		t.Fatal("expected realtime commercial lifecycle flag to be forwarded")
	}
	if len(relayConfig.CORSAllowedOrigins) != 1 || relayConfig.CORSAllowedOrigins[0] != "https://console.example.test" {
		t.Fatalf("expected CORS origins to be forwarded to standalone relay config, got %v", relayConfig.CORSAllowedOrigins)
	}
	if relayConfig.BatchPollingRegistrar != store {
		t.Fatal("expected relay store to be wired as batch polling registrar")
	}
	if relayConfig.FilesMappingStore != store || relayConfig.ConversationAffinityStore != store {
		t.Fatal("expected relay store to back relay durable stores")
	}
}

func TestStartStandaloneRelayBatchPollingWorkerStartsConfiguredWorker(t *testing.T) {
	factory := &recordingStandaloneRelayBatchPollingWorkerFactory{}
	cfg := config.Config{
		RelayBatchPollingWorkerEnabled:    true,
		RelayBatchPollingWorkerIntervalMS: 750,
		RelayBatchPollingWorkerClaimLimit: 6,
	}
	finalizer := relay.NewBatchUsageFinalizer(&recordingStandaloneRelayUsageReplacer{}, relay.BatchUsageFinalizerConfig{
		PricingStore: relay.NewPricingStoreWithDefaults(),
	})

	cancel, started := startStandaloneRelayBatchPollingWorker(
		cfg,
		&recordingStandaloneRelayBatchPollingWorkerStore{},
		&recordingStandaloneRelayBatchStatusClient{},
		finalizer,
		finalizer,
		factory.newWorker,
	)
	if !started {
		t.Fatal("expected relay batch polling worker to start")
	}
	if cancel == nil {
		t.Fatal("expected shutdown cancel function")
	}
	if factory.worker == nil {
		t.Fatalf("expected worker factory to run worker, got %+v", factory.worker)
	}
	select {
	case <-factory.worker.startedCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for standalone relay batch polling worker to start")
	}
	if factory.config.Interval != 750*time.Millisecond || factory.config.Limit != 6 {
		t.Fatalf("worker config interval=%s limit=%d", factory.config.Interval, factory.config.Limit)
	}
	if factory.config.CompletionFinalizer == nil {
		t.Fatal("expected batch polling worker to receive completion finalizer")
	}
	if factory.config.FailureFinalizer == nil {
		t.Fatal("expected batch polling worker to receive failure finalizer")
	}
	cancel()
	select {
	case <-factory.worker.cancelledCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for standalone relay batch polling worker shutdown")
	}
}

func TestStartStandaloneRelayBatchPollingWorkerSkipsWhenDisabled(t *testing.T) {
	factory := &recordingStandaloneRelayBatchPollingWorkerFactory{}

	cancel, started := startStandaloneRelayBatchPollingWorker(
		config.Config{},
		&recordingStandaloneRelayBatchPollingWorkerStore{},
		&recordingStandaloneRelayBatchStatusClient{},
		nil,
		nil,
		factory.newWorker,
	)
	if started {
		t.Fatal("expected relay batch polling worker to stay stopped")
	}
	if cancel != nil {
		t.Fatal("expected no shutdown cancel function")
	}
	if factory.worker != nil {
		t.Fatalf("worker should not be constructed, got %+v", factory.worker)
	}
}

type recordingStandaloneRelayBatchPollingWorkerFactory struct {
	config relay.BatchPollingWorkerConfig
	worker *recordingStandaloneRelayBatchPollingWorker
}

func (f *recordingStandaloneRelayBatchPollingWorkerFactory) newWorker(_ relay.BatchPollingWorkerStore, _ relay.BatchStatusClient, config relay.BatchPollingWorkerConfig) relayBatchPollingWorkerRunner {
	f.config = config
	f.worker = &recordingStandaloneRelayBatchPollingWorker{
		startedCh:   make(chan struct{}),
		cancelledCh: make(chan struct{}),
	}
	return f.worker
}

type recordingStandaloneRelayBatchPollingWorker struct {
	startedCh   chan struct{}
	cancelledCh chan struct{}
}

func (w *recordingStandaloneRelayBatchPollingWorker) Run(ctx context.Context) {
	close(w.startedCh)
	<-ctx.Done()
	close(w.cancelledCh)
}

type recordingStandaloneRelayUsageReplacer struct{}

func (r *recordingStandaloneRelayUsageReplacer) RecordRelayUsage(_ context.Context, _ relay.RelayUsageLogRecord) error {
	return nil
}

func (r *recordingStandaloneRelayUsageReplacer) ReplaceRelayUsage(_ context.Context, _ relay.RelayUsageLogRecord) error {
	return nil
}

type recordingStandaloneRelayBatchPollingWorkerStore struct{}

func (s *recordingStandaloneRelayBatchPollingWorkerStore) ClaimBatchPollingJobs(context.Context, time.Time, int, string) ([]relay.RelayBatchPollingJob, error) {
	return nil, nil
}

func (s *recordingStandaloneRelayBatchPollingWorkerStore) MarkBatchPollingJobDeadLetter(context.Context, string, string, string, time.Time) error {
	return nil
}

func (s *recordingStandaloneRelayBatchPollingWorkerStore) MarkBatchPollingJobFailed(context.Context, string, string, string, time.Time) error {
	return nil
}

func (s *recordingStandaloneRelayBatchPollingWorkerStore) MarkBatchPollingJobSucceeded(context.Context, string, string, time.Time) error {
	return nil
}

type recordingStandaloneRelayBatchStatusClient struct{}

func (c *recordingStandaloneRelayBatchStatusClient) RetrieveBatch(context.Context, relay.RelayBatchPollingJob) (relay.BatchStatusResult, error) {
	return relay.BatchStatusResult{}, nil
}
