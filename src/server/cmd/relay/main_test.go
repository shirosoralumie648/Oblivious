package main

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/config"
	"oblivious/server/internal/relay"
	"oblivious/server/internal/relay/ratelimit"
)

func TestBuildStandaloneRelayRateLimiterRedisTransportContract(t *testing.T) {
	for _, test := range []struct {
		name      string
		tls       bool
		wantFirst byte
	}{
		{name: "plain Redis sends RESP", wantFirst: '*'},
		{name: "rediss sends TLS ClientHello", tls: true, wantFirst: 0x16},
	} {
		t.Run(test.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			firstByte := make(chan byte, 1)
			go func() {
				connection, acceptErr := listener.Accept()
				if acceptErr != nil {
					return
				}
				defer connection.Close()
				buffer := []byte{0}
				if _, readErr := connection.Read(buffer); readErr == nil {
					firstByte <- buffer[0]
				}
			}()

			limiter, closeLimiter, err := buildRelayRateLimiter(config.Config{
				RelayRateLimitBackend: "redis",
				RedisAddr:             listener.Addr().String(),
				RedisTLS:              test.tls,
			})
			if err != nil {
				t.Fatal(err)
			}
			if closeLimiter == nil {
				t.Fatal("Redis limiter did not return a close function")
			}
			defer closeLimiter()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			result := make(chan error, 1)
			go func() {
				result <- limiter.Allow(ctx, ratelimit.Key{ChannelID: "transport"}, ratelimit.Limits{RPM: 1}, ratelimit.Usage{})
			}()
			select {
			case got := <-firstByte:
				if got != test.wantFirst {
					t.Fatalf("first transport byte = 0x%02x, want 0x%02x", got, test.wantFirst)
				}
			case <-time.After(time.Second):
				t.Fatal("Redis limiter did not connect")
			}
			cancel()
			select {
			case <-result:
			case <-time.After(time.Second):
				t.Fatal("Redis limiter did not stop after cancellation")
			}
		})
	}

	for _, tlsEnabled := range []bool{false, true} {
		for _, malformed := range []string{
			" redis-secret.internal:6379",
			"redis\x01-secret.internal:6379",
			"redis-secret.internal/path:6379",
			"redis-secret.internal?db=1:6379",
			"user@redis-secret.internal:6379",
			"redis-secret..internal:6379",
			"-redis-secret.internal:6379",
			"redis-secret.invalid-address",
			"redis-secret.invalid:invalid-port",
			"redis-secret.invalid:0",
			"redis-secret.invalid:65536",
		} {
			limiter, closeLimiter, err := buildRelayRateLimiter(config.Config{RelayRateLimitBackend: "redis", RedisAddr: malformed, RedisTLS: tlsEnabled})
			if err == nil || limiter != nil || closeLimiter != nil || strings.Contains(err.Error(), malformed) || strings.Contains(err.Error(), "redis-secret") {
				t.Fatalf("invalid Redis limiter result tls=%t limiter=%T close=%v error=%v", tlsEnabled, limiter, closeLimiter != nil, err)
			}
		}
	}
}

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

func TestStartStandaloneRelayQuotaCompensationWorker(t *testing.T) {
	factory := &recordingStandaloneRelayQuotaCompensationWorkerFactory{}
	store := &recordingStandaloneRelayQuotaCompensationStore{}
	coordinator := relay.NewQuotaCompensationCoordinator(store, nil, nil)

	cancel, started := startStandaloneRelayQuotaCompensationWorker(
		store, coordinator,
		relay.QuotaCompensationWorkerConfig{OnError: func(error) {}},
		factory.newWorker,
	)
	if !started {
		t.Fatal("expected quota compensation worker to start")
	}
	if cancel == nil {
		t.Fatal("expected shutdown cancel function")
	}
	if factory.worker == nil {
		t.Fatal("expected factory to construct a worker")
	}
	select {
	case <-factory.worker.startedCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for compensation worker to start")
	}
	cancel()
	select {
	case <-factory.worker.cancelledCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for compensation worker shutdown")
	}
}

func TestStartStandaloneRelayQuotaCompensationWorkerSkipsWhenNilStore(t *testing.T) {
	factory := &recordingStandaloneRelayQuotaCompensationWorkerFactory{}
	cancel, started := startStandaloneRelayQuotaCompensationWorker(nil, nil, relay.QuotaCompensationWorkerConfig{}, factory.newWorker)
	if started {
		t.Fatal("expected worker to stay stopped without a store")
	}
	if cancel != nil {
		t.Fatal("expected no cancel without a store")
	}
	if factory.worker != nil {
		t.Fatal("worker should not be constructed without a store")
	}
}

type recordingStandaloneRelayQuotaCompensationWorkerFactory struct {
	config relay.QuotaCompensationWorkerConfig
	worker *recordingStandaloneRelayQuotaCompensationWorker
}

func (f *recordingStandaloneRelayQuotaCompensationWorkerFactory) newWorker(_ relay.QuotaCompensationStore, _ *relay.QuotaCompensationCoordinator, config relay.QuotaCompensationWorkerConfig) relayQuotaCompensationWorkerRunner {
	f.config = config
	f.worker = &recordingStandaloneRelayQuotaCompensationWorker{
		startedCh:   make(chan struct{}),
		cancelledCh: make(chan struct{}),
	}
	return f.worker
}

type recordingStandaloneRelayQuotaCompensationWorker struct {
	startedCh   chan struct{}
	cancelledCh chan struct{}
}

func (w *recordingStandaloneRelayQuotaCompensationWorker) Run(ctx context.Context) {
	close(w.startedCh)
	<-ctx.Done()
	close(w.cancelledCh)
}

type recordingStandaloneRelayQuotaCompensationStore struct{}

func (s *recordingStandaloneRelayQuotaCompensationStore) ArmQuotaCompensation(_ context.Context, _ relay.QuotaCompensationRequest) (relay.QuotaCompensationJob, error) {
	return relay.QuotaCompensationJob{}, nil
}

func (s *recordingStandaloneRelayQuotaCompensationStore) ClaimQuotaCompensationJobs(_ context.Context, _ time.Time, _ int, _ string) ([]relay.QuotaCompensationJob, error) {
	return nil, nil
}

func (s *recordingStandaloneRelayQuotaCompensationStore) MarkQuotaCompensationScopeSucceeded(_ context.Context, _, _, _ string, _ time.Time) error {
	return nil
}

func (s *recordingStandaloneRelayQuotaCompensationStore) MarkQuotaCompensationScopeFailed(_ context.Context, _, _, _, _ string, _ time.Time) error {
	return nil
}

func (s *recordingStandaloneRelayQuotaCompensationStore) RecordAPITokenQuotaRefundReceipt(_ context.Context, _, _ string, _ float64, _ time.Time) (bool, error) {
	return false, nil
}
