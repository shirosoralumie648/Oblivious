package releasecontract

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestReadinessManagerLifecycleContract(t *testing.T) {
	contract, profile, identity := loadReadinessTestAuthority(t)
	clock := newManagerTestClock(time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC))
	probes := managerTestProbes(t, contract, profile)
	auditPath := filepath.Join(t.TempDir(), "audit", "readiness.json")
	if err := os.MkdirAll(filepath.Dir(auditPath), 0o755); err != nil {
		t.Fatalf("create seeded audit parent: %v", err)
	}
	if err := os.WriteFile(auditPath, []byte(`{"generation":999,"capabilities":{"caller":"enabled"}}`), 0o600); err != nil {
		t.Fatalf("seed audit file: %v", err)
	}
	excluded := &fixedManagerProbe{id: "probe.excluded", dependency: "candidate-excluded"}
	allProbes := append(append([]Probe(nil), probes...), excluded)
	manager := newTestManager(t, contract, profile, identity, clock, allProbes, 25*time.Millisecond, NewAtomicReadinessSnapshotWriter(), auditPath)

	t.Run("bootstrap publishes generation one and ignores seeded audit", func(t *testing.T) {
		if err := manager.Bootstrap(context.Background()); err != nil {
			t.Fatalf("bootstrap manager: %v", err)
		}
		if got := manager.Evaluate(); got.Generation != 1 || got.ErrorCode != "" {
			t.Fatalf("bootstrap evaluation = generation %d error %q, want generation 1", got.Generation, got.ErrorCode)
		}
		if excluded.calls.Load() != 0 {
			t.Fatalf("excluded candidate probe calls = %d, want zero", excluded.calls.Load())
		}
		content, err := os.ReadFile(auditPath)
		if err != nil {
			t.Fatalf("read exported audit: %v", err)
		}
		var snapshot ReadinessSnapshotV1
		if err := json.Unmarshal(content, &snapshot); err != nil {
			t.Fatalf("decode exported audit: %v", err)
		}
		if snapshot.Generation != 1 || snapshot.SchemaVersion != ReadinessSnapshotSchemaV1 {
			t.Fatalf("exported snapshot = schema %q generation %d", snapshot.SchemaVersion, snapshot.Generation)
		}
	})

	t.Run("complete refresh increments once and returned maps are defensive", func(t *testing.T) {
		if err := manager.refresh(context.Background()); err != nil {
			t.Fatalf("refresh manager: %v", err)
		}
		evaluation := manager.Evaluate()
		if evaluation.Generation != 2 {
			t.Fatalf("generation = %d, want 2", evaluation.Generation)
		}
		evaluation.Capabilities["identity.account_session"] = CapabilityEvaluation{Availability: AvailabilityBlocked}
		if got := manager.Evaluate().Capabilities["identity.account_session"].Availability; got != AvailabilityEnabled {
			t.Fatalf("caller map mutation changed manager state: %q", got)
		}
	})

	t.Run("structural failure preserves generation and validity byte for byte", func(t *testing.T) {
		before := marshalCurrentGeneration(t, manager)
		postgres := findFixedProbe(t, probes, "postgres")
		postgres.set(Observation{Availability: Availability("malformed"), ReasonCode: "caller"}, nil)
		if err := manager.refresh(context.Background()); !IsReadinessCode(err, CodeReadinessUnavailable) {
			t.Fatalf("structural refresh error = %T %v, want readiness_unavailable", err, err)
		}
		after := marshalCurrentGeneration(t, manager)
		if string(after) != string(before) {
			t.Fatalf("structural refresh changed published generation\nbefore=%s\nafter=%s", before, after)
		}
		postgres.set(Observation{Availability: AvailabilityEnabled}, nil)
	})

	t.Run("dependency error publishes blocked generation", func(t *testing.T) {
		postgres := findFixedProbe(t, probes, "postgres")
		postgres.set(Observation{}, errors.New("dependency down"))
		previous := manager.Evaluate().Generation
		if err := manager.refresh(context.Background()); err != nil {
			t.Fatalf("dependency failure refresh: %v", err)
		}
		if got := manager.Evaluate().Generation; got != previous+1 {
			t.Fatalf("blocked generation = %d, want %d", got, previous+1)
		}
		assertReadinessCode(t, manager.Require("identity.account_session"), CodeCapabilityBlocked)
		postgres.set(Observation{Availability: AvailabilityEnabled}, nil)
	})

	t.Run("retained generation becomes stale without validity extension", func(t *testing.T) {
		if err := manager.refresh(context.Background()); err != nil {
			t.Fatalf("restore enabled generation: %v", err)
		}
		before := manager.Evaluate()
		findFixedProbe(t, probes, "postgres").set(Observation{Availability: Availability("malformed")}, nil)
		clock.Advance(121 * time.Second)
		if err := manager.refresh(context.Background()); !IsReadinessCode(err, CodeReadinessUnavailable) {
			t.Fatalf("structural refresh after advance = %v", err)
		}
		after := manager.Evaluate()
		if after.Generation != before.Generation || after.ValidUntil.UnixNano() != before.ValidUntil.UnixNano() {
			t.Fatalf("retained generation was extended: before=%+v after=%+v", before, after)
		}
		if after.ErrorCode != CodeReadinessStale {
			t.Fatalf("retained evaluation error = %q, want readiness_stale", after.ErrorCode)
		}
		assertReadinessCode(t, manager.Require("identity.account_session"), CodeReadinessStale)
		findFixedProbe(t, probes, "postgres").set(Observation{Availability: AvailabilityEnabled}, nil)
	})

	t.Run("fake clock refreshes only on authored cadence", func(t *testing.T) {
		freshClock := newManagerTestClock(time.Date(2026, time.July, 18, 0, 0, 0, 0, time.UTC))
		freshProbes := managerTestProbes(t, contract, profile)
		fresh := newTestManager(t, contract, profile, identity, freshClock, freshProbes, 25*time.Millisecond, &recordingSnapshotWriter{}, filepath.Join(t.TempDir(), "readiness.json"))
		if err := fresh.Bootstrap(context.Background()); err != nil {
			t.Fatalf("bootstrap cadence manager: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		fresh.StartRefresh(ctx)
		fresh.StartRefresh(ctx)
		if got := freshClock.lastInterval(); got != 30*time.Second {
			t.Fatalf("ticker interval = %s, want authored 30s", got)
		}
		freshClock.Advance(29 * time.Second)
		if got := fresh.Evaluate().Generation; got != 1 {
			t.Fatalf("early generation = %d, want 1", got)
		}
		freshClock.Advance(time.Second)
		waitForGeneration(t, fresh, 2)
		freshClock.Advance(30 * time.Second)
		waitForGeneration(t, fresh, 3)
		cancel()
	})

	t.Run("audit write failure cannot change memory authorization", func(t *testing.T) {
		writer := &recordingSnapshotWriter{err: errors.New("disk full")}
		failed := newTestManager(t, contract, profile, identity, newManagerTestClock(clock.Now()), managerTestProbes(t, contract, profile), 25*time.Millisecond, writer, filepath.Join(t.TempDir(), "readiness.json"))
		err := failed.Bootstrap(context.Background())
		assertReadinessCode(t, err, CodeReportOutputUnwritable)
		if got := failed.Evaluate().Generation; got != 1 {
			t.Fatalf("audit failure discarded in-memory generation: %d", got)
		}
		if err := failed.Require("identity.account_session"); err != nil {
			t.Fatalf("audit failure changed manager authorization: %v", err)
		}
	})

	t.Run("cancellation ignoring probe remains deadline bounded and is released", func(t *testing.T) {
		release := make(chan struct{})
		blocking := &blockingManagerProbe{id: "probe.postgres", dependency: "postgres", release: release}
		boundedProbes := managerTestProbes(t, contract, profile)
		for i, probe := range boundedProbes {
			if probe.DependencyID() == "postgres" {
				boundedProbes[i] = blocking
			}
		}
		bounded := newTestManager(t, contract, profile, identity, newManagerTestClock(clock.Now()), boundedProbes, 20*time.Millisecond, &recordingSnapshotWriter{}, filepath.Join(t.TempDir(), "readiness.json"))
		started := time.Now()
		if err := bounded.Bootstrap(context.Background()); err != nil {
			t.Fatalf("bounded bootstrap: %v", err)
		}
		if elapsed := time.Since(started); elapsed > 200*time.Millisecond {
			t.Fatalf("cancellation-ignoring probe held bootstrap for %s", elapsed)
		}
		assertReadinessCode(t, bounded.Require("identity.account_session"), CodeCapabilityBlocked)
		close(release)
	})

	t.Run("http transport failure publishes blocked but malformed success preserves", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"availability":"caller_value","reasonCode":"caller_reason"}`))
		}))
		malformedProbe, err := NewHTTPDependencyProbe("probe.postgres", "postgres", server.URL, server.Client())
		if err != nil {
			t.Fatalf("construct HTTP dependency probe: %v", err)
		}
		httpProbes := replaceManagerProbe(managerTestProbes(t, contract, profile), "postgres", malformedProbe)
		httpManager := newTestManager(t, contract, profile, identity, newManagerTestClock(clock.Now()), httpProbes, 100*time.Millisecond, &recordingSnapshotWriter{}, filepath.Join(t.TempDir(), "readiness.json"))
		if err := httpManager.Bootstrap(context.Background()); !IsReadinessCode(err, CodeReadinessUnavailable) {
			t.Fatalf("malformed HTTP bootstrap error = %v", err)
		}
		if got := httpManager.Evaluate().Generation; got != 0 {
			t.Fatalf("malformed HTTP success published generation %d", got)
		}

		server.Close()
		if err := httpManager.Bootstrap(context.Background()); err != nil {
			t.Fatalf("transport failure should publish blocked generation: %v", err)
		}
		if got := httpManager.Evaluate().Generation; got != 1 {
			t.Fatalf("transport failure generation = %d, want 1", got)
		}
		assertReadinessCode(t, httpManager.Require("identity.account_session"), CodeCapabilityBlocked)
	})
}

func TestReadinessManagerPublicationRace(t *testing.T) {
	contract, profile, identity := loadReadinessTestAuthority(t)
	clock := newManagerTestClock(time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC))
	manager := newTestManager(t, contract, profile, identity, clock, managerTestProbes(t, contract, profile), 100*time.Millisecond, &recordingSnapshotWriter{}, filepath.Join(t.TempDir(), "readiness.json"))
	if err := manager.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap race manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var readers sync.WaitGroup
	var failed atomic.Bool
	for range 12 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			last := uint64(0)
			for ctx.Err() == nil {
				evaluation := manager.Evaluate()
				if evaluation.Generation < last || evaluation.Generation == 0 || len(evaluation.Capabilities) == 0 {
					failed.Store(true)
					return
				}
				last = evaluation.Generation
				_ = manager.Require("identity.account_session")
				evaluation.Capabilities["identity.account_session"] = CapabilityEvaluation{}
			}
		}()
	}
	for range 80 {
		clock.Advance(time.Millisecond)
		if err := manager.refresh(context.Background()); err != nil {
			t.Fatalf("race refresh: %v", err)
		}
	}
	cancel()
	readers.Wait()
	if failed.Load() {
		t.Fatal("reader observed a partial or decreasing generation")
	}
	if got := manager.Evaluate().Generation; got != 81 {
		t.Fatalf("final generation = %d, want 81", got)
	}
}

type fixedManagerProbe struct {
	id         string
	dependency string
	calls      atomic.Int64
	mu         sync.RWMutex
	result     Observation
	err        error
}

func (p *fixedManagerProbe) ID() string           { return p.id }
func (p *fixedManagerProbe) DependencyID() string { return p.dependency }
func (p *fixedManagerProbe) Run(context.Context) (Observation, error) {
	p.calls.Add(1)
	p.mu.RLock()
	defer p.mu.RUnlock()
	return cloneObservation(p.result), p.err
}
func (p *fixedManagerProbe) set(result Observation, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.result = cloneObservation(result)
	p.err = err
}

type blockingManagerProbe struct {
	id         string
	dependency string
	release    <-chan struct{}
}

func (p *blockingManagerProbe) ID() string           { return p.id }
func (p *blockingManagerProbe) DependencyID() string { return p.dependency }
func (p *blockingManagerProbe) Run(context.Context) (Observation, error) {
	<-p.release
	return Observation{Availability: AvailabilityEnabled}, nil
}

type recordingSnapshotWriter struct {
	mu        sync.Mutex
	snapshots []ReadinessSnapshotV1
	err       error
}

func (w *recordingSnapshotWriter) Write(_ context.Context, _ string, snapshot ReadinessSnapshotV1) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.snapshots = append(w.snapshots, snapshot)
	return w.err
}

type managerTestClock struct {
	mu       sync.Mutex
	now      time.Time
	tickers  []*managerTestTicker
	interval time.Duration
}

func newManagerTestClock(now time.Time) *managerTestClock {
	return &managerTestClock{now: normalizeTime(now)}
}

func (c *managerTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *managerTestClock) NewTicker(interval time.Duration) Ticker {
	c.mu.Lock()
	defer c.mu.Unlock()
	ticker := &managerTestTicker{clock: c, interval: interval, next: c.now.Add(interval), channel: make(chan time.Time, 16)}
	c.tickers = append(c.tickers, ticker)
	c.interval = interval
	return ticker
}

func (c *managerTestClock) Advance(duration time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(duration)
	now := c.now
	for _, ticker := range c.tickers {
		if ticker.stopped {
			continue
		}
		for !ticker.next.After(now) {
			ticker.channel <- ticker.next
			ticker.next = ticker.next.Add(ticker.interval)
		}
	}
	c.mu.Unlock()
}

func (c *managerTestClock) lastInterval() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.interval
}

type managerTestTicker struct {
	clock    *managerTestClock
	interval time.Duration
	next     time.Time
	channel  chan time.Time
	stopped  bool
}

func (t *managerTestTicker) C() <-chan time.Time { return t.channel }
func (t *managerTestTicker) Stop() {
	t.clock.mu.Lock()
	t.stopped = true
	t.clock.mu.Unlock()
}

func managerTestProbes(t *testing.T, contract AuthoredContractV1, profile DeploymentProfile) []Probe {
	t.Helper()
	_, dependencies, err := applicableCapabilityPolicy(contract, profile)
	if err != nil {
		t.Fatalf("derive manager test dependencies: %v", err)
	}
	result := make([]Probe, 0, len(dependencies))
	for dependency := range dependencies {
		result = append(result, &fixedManagerProbe{id: "probe." + dependency, dependency: dependency, result: Observation{Availability: AvailabilityEnabled}})
	}
	return result
}

func newTestManager(t *testing.T, contract AuthoredContractV1, profile DeploymentProfile, identity BuildIdentityV1, clock Clock, probes []Probe, deadline time.Duration, writer ReadinessSnapshotWriter, auditPath string) *Manager {
	t.Helper()
	manager, err := NewManager(contract, identity, profile, NewEvaluator(), clock, probes, deadline, writer, auditPath)
	if err != nil {
		t.Fatalf("construct manager: %v", err)
	}
	return manager
}

func findFixedProbe(t *testing.T, probes []Probe, dependency string) *fixedManagerProbe {
	t.Helper()
	for _, probe := range probes {
		if probe.DependencyID() == dependency {
			fixed, ok := probe.(*fixedManagerProbe)
			if !ok {
				t.Fatalf("probe %s is %T, want fixedManagerProbe", dependency, probe)
			}
			return fixed
		}
	}
	t.Fatalf("probe for dependency %s not found", dependency)
	return nil
}

func replaceManagerProbe(probes []Probe, dependency string, replacement Probe) []Probe {
	result := append([]Probe(nil), probes...)
	for i, probe := range result {
		if probe.DependencyID() == dependency {
			result[i] = replacement
		}
	}
	return result
}

func marshalCurrentGeneration(t *testing.T, manager *Manager) []byte {
	t.Helper()
	content, err := json.Marshal(snapshotFromGeneration(manager.current.Load()))
	if err != nil {
		t.Fatalf("marshal current generation: %v", err)
	}
	return content
}

func waitForGeneration(t *testing.T, manager *Manager, generation uint64) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if manager.Evaluate().Generation >= generation {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("manager did not reach generation %d; current=%d", generation, manager.Evaluate().Generation)
}
