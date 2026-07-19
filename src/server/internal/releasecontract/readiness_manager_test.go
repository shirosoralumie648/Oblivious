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

func TestReadinessManagerBootstrapNilContextContract(t *testing.T) {
	contract, profile, identity := loadReadinessTestAuthority(t)
	probes := managerTestProbes(t, contract, profile)
	writer := &recordingSnapshotWriter{}
	manager := newTestManager(
		t,
		contract,
		profile,
		identity,
		newManagerTestClock(time.Date(2026, time.July, 19, 5, 0, 0, 0, time.UTC)),
		probes,
		25*time.Millisecond,
		writer,
		filepath.Join(t.TempDir(), "readiness.json"),
	)

	for attempt := 1; attempt <= 2; attempt++ {
		err := manager.Bootstrap(nil)
		var readinessErr *ReadinessError
		if !errors.As(err, &readinessErr) {
			t.Fatalf("nil-context bootstrap %d error = %T %v, want ReadinessError", attempt, err, err)
		}
		if readinessErr.Code != CodeReadinessUnavailable || readinessErr.Field != "context" {
			t.Fatalf("nil-context bootstrap %d error = code %q field %q, want %q and context", attempt, readinessErr.Code, readinessErr.Field, CodeReadinessUnavailable)
		}
		if got := string(readinessErr.Code); got != "readiness_unavailable" {
			t.Fatalf("nil-context bootstrap %d code = %q, want readiness_unavailable", attempt, got)
		}
	}

	for _, probe := range probes {
		fixed, ok := probe.(*fixedManagerProbe)
		if !ok {
			t.Fatalf("probe %s has unsupported test type %T", probe.DependencyID(), probe)
		}
		if got := fixed.calls.Load(); got != 0 {
			t.Fatalf("nil-context bootstrap probe %s calls = %d, want zero", probe.DependencyID(), got)
		}
	}
	if got := writer.snapshotCount(); got != 0 {
		t.Fatalf("nil-context bootstrap audit writes = %d, want zero", got)
	}
	if manager.current.Load() != nil {
		t.Fatal("nil-context bootstrap published a readiness generation")
	}
	if got := manager.Evaluate(); got.Generation != 0 || got.ErrorCode != CodeReadinessUnavailable {
		t.Fatalf("nil-context bootstrap evaluation = generation %d error %q, want zero and readiness_unavailable", got.Generation, got.ErrorCode)
	}
}

func TestReadinessManagerBootstrapConcurrencyContract(t *testing.T) {
	contract, profile, identity := loadReadinessTestAuthority(t)
	clock := newManagerTestClock(time.Date(2026, time.July, 19, 0, 0, 0, 0, time.UTC))
	probes := managerTestProbes(t, contract, profile)
	release := make(chan struct{})
	started := make(chan struct{})
	gated := &gatedManagerProbe{
		id:         "probe.postgres",
		dependency: "postgres",
		started:    started,
		release:    release,
	}
	probes = replaceManagerProbe(probes, "postgres", gated)
	writer := &recordingSnapshotWriter{}
	manager := newTestManager(t, contract, profile, identity, clock, probes, 500*time.Millisecond, writer, filepath.Join(t.TempDir(), "readiness.json"))

	const callers = 64
	start := make(chan struct{})
	results := make(chan error, callers)
	var ready sync.WaitGroup
	ready.Add(callers)
	for range callers {
		go func() {
			ready.Done()
			<-start
			results <- manager.Bootstrap(context.Background())
		}()
	}
	ready.Wait()
	close(start)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("no bootstrap probe started")
	}
	// Keep generation one unpublished long enough for every released caller to
	// reach the bootstrap transition.
	time.Sleep(50 * time.Millisecond)
	close(release)

	succeeded := 0
	failed := 0
	failureText := ""
	for range callers {
		err := <-results
		if err == nil {
			succeeded++
			continue
		}
		if !IsReadinessCode(err, CodeReadinessUnavailable) {
			t.Fatalf("bootstrap loser error = %T %v, want readiness_unavailable", err, err)
		}
		if failureText == "" {
			failureText = err.Error()
		} else if err.Error() != failureText {
			t.Fatalf("bootstrap loser error = %q, want stable %q", err.Error(), failureText)
		}
		failed++
	}
	if succeeded != 1 || failed != callers-1 {
		t.Fatalf("bootstrap outcomes = %d succeeded, %d failed; want 1 and %d", succeeded, failed, callers-1)
	}
	if got := manager.Evaluate().Generation; got != 1 {
		t.Fatalf("bootstrap generation = %d, want 1", got)
	}
	for _, probe := range probes {
		calls := int64(0)
		switch typed := probe.(type) {
		case *fixedManagerProbe:
			calls = typed.calls.Load()
		case *gatedManagerProbe:
			calls = typed.calls.Load()
		default:
			t.Fatalf("probe %s has unsupported test type %T", probe.DependencyID(), probe)
		}
		if calls != 1 {
			t.Fatalf("probe %s calls = %d, want 1", probe.DependencyID(), calls)
		}
	}
	if got := writer.snapshotCount(); got != 1 {
		t.Fatalf("audit writes = %d, want 1", got)
	}

	before := marshalCurrentGeneration(t, manager)
	if err := manager.Bootstrap(context.Background()); !IsReadinessCode(err, CodeReadinessUnavailable) || err.Error() != failureText {
		t.Fatalf("second bootstrap error = %T %v, want stable readiness_unavailable", err, err)
	}
	after := marshalCurrentGeneration(t, manager)
	if string(after) != string(before) {
		t.Fatalf("second bootstrap changed published snapshot\nbefore=%s\nafter=%s", before, after)
	}
	if got := writer.snapshotCount(); got != 1 {
		t.Fatalf("second bootstrap audit writes = %d, want 1", got)
	}
}

func TestReadinessManagerCanceledRefreshDoesNotStartProbeContract(t *testing.T) {
	contract, profile, identity := loadReadinessTestAuthority(t)
	release := make(chan struct{})
	postgres := &cancellationIgnoringHealthyManagerProbe{
		id:         "probe.postgres",
		dependency: "postgres",
		release:    release,
	}
	probes := replaceManagerProbe(managerTestProbes(t, contract, profile), "postgres", postgres)
	manager := newTestManager(
		t,
		contract,
		profile,
		identity,
		newManagerTestClock(time.Date(2026, time.July, 19, 3, 0, 0, 0, time.UTC)),
		probes,
		20*time.Millisecond,
		&recordingSnapshotWriter{},
		filepath.Join(t.TempDir(), "readiness.json"),
	)
	defer close(release)
	if err := manager.Bootstrap(context.Background()); err != nil {
		t.Fatalf("healthy bootstrap: %v", err)
	}
	initialCalls := postgres.calls.Load()

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for attempt := 0; attempt < 256; attempt++ {
		if err := manager.refresh(canceled); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled refresh %d error = %v, want context canceled", attempt+1, err)
		}
	}
	if got := postgres.calls.Load(); got != initialCalls {
		t.Fatalf("canceled refreshes started %d new postgres probes, want zero", got-initialCalls)
	}
	if got := manager.Evaluate().Generation; got != 1 {
		t.Fatalf("canceled refreshes changed generation to %d, want 1", got)
	}

	if err := manager.refresh(context.Background()); err != nil {
		t.Fatalf("healthy lane reuse refresh: %v", err)
	}
	if got := postgres.calls.Load(); got != initialCalls+1 {
		t.Fatalf("healthy lane reuse calls = %d, want %d", got, initialCalls+1)
	}
}

func TestReadinessManagerBootstrapOwnsInitialGenerationContract(t *testing.T) {
	contract, profile, identity := loadReadinessTestAuthority(t)
	release := make(chan struct{})
	started := make(chan struct{})
	probes := replaceManagerProbe(
		managerTestProbes(t, contract, profile),
		"postgres",
		&gatedManagerProbe{id: "probe.postgres", dependency: "postgres", started: started, release: release},
	)
	writer := &recordingSnapshotWriter{}
	manager := newTestManager(
		t,
		contract,
		profile,
		identity,
		newManagerTestClock(time.Date(2026, time.July, 19, 4, 0, 0, 0, time.UTC)),
		probes,
		500*time.Millisecond,
		writer,
		filepath.Join(t.TempDir(), "readiness.json"),
	)
	var releaseOnce sync.Once
	releaseProbe := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseProbe()

	bootstrapResult := make(chan error, 1)
	go func() { bootstrapResult <- manager.Bootstrap(context.Background()) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("bootstrap probe did not start")
	}

	refreshErr := manager.refresh(context.Background())
	if !IsReadinessCode(refreshErr, CodeReadinessUnavailable) {
		t.Fatalf("racing ordinary refresh error = %v, want readiness_unavailable", refreshErr)
	}
	if got := manager.Evaluate().Generation; got != 0 {
		t.Fatalf("racing ordinary refresh published generation %d, want zero", got)
	}
	if got := writer.snapshotCount(); got != 0 {
		t.Fatalf("racing ordinary refresh audit writes = %d, want zero", got)
	}

	releaseProbe()
	if err := <-bootstrapResult; err != nil {
		t.Fatalf("bootstrap after racing refresh: %v", err)
	}
	if got := manager.Evaluate().Generation; got != 1 {
		t.Fatalf("bootstrap generation = %d, want 1", got)
	}
	if got := writer.snapshotCount(); got != 1 {
		t.Fatalf("bootstrap audit writes = %d, want 1", got)
	}
}

func TestReadinessManagerNonCooperativeProbeBoundContract(t *testing.T) {
	contract, profile, identity := loadReadinessTestAuthority(t)
	clock := newManagerTestClock(time.Date(2026, time.July, 19, 1, 0, 0, 0, time.UTC))
	probes := managerTestProbes(t, contract, profile)
	release := make(chan struct{})
	started := make(chan struct{})
	exited := make(chan struct{})
	blocking := &countedBlockingManagerProbe{
		id:         "probe.postgres",
		dependency: "postgres",
		started:    started,
		exited:     exited,
		release:    release,
	}
	probes = replaceManagerProbe(probes, "postgres", blocking)
	writer := &recordingSnapshotWriter{}
	const probeDeadline = 20 * time.Millisecond
	manager := newTestManager(t, contract, profile, identity, clock, probes, probeDeadline, writer, filepath.Join(t.TempDir(), "readiness.json"))

	startedAt := time.Now()
	if err := manager.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap with non-cooperative probe: %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > 10*probeDeadline {
		t.Fatalf("bootstrap elapsed = %s, want at most %s", elapsed, 10*probeDeadline)
	}
	select {
	case <-started:
	default:
		t.Fatal("non-cooperative probe did not start")
	}
	assertBlockedDependencyObservation(t, manager.current.Load(), "postgres", len(probes))

	for attempt := 1; attempt <= 5; attempt++ {
		startedAt = time.Now()
		if err := manager.refresh(context.Background()); err != nil {
			t.Fatalf("occupied-lane refresh %d: %v", attempt, err)
		}
		if elapsed := time.Since(startedAt); elapsed > 10*probeDeadline {
			t.Fatalf("occupied-lane refresh %d elapsed = %s, want at most %s", attempt, elapsed, 10*probeDeadline)
		}
		assertBlockedDependencyObservation(t, manager.current.Load(), "postgres", len(probes))
	}
	if got := blocking.calls.Load(); got != 1 {
		t.Fatalf("non-cooperative probe calls before release = %d, want 1", got)
	}
	for _, probe := range probes {
		fixed, ok := probe.(*fixedManagerProbe)
		if !ok {
			continue
		}
		if got := fixed.calls.Load(); got != 6 {
			t.Fatalf("other probe %s calls = %d, want 6", fixed.DependencyID(), got)
		}
	}
	for index, snapshot := range writer.snapshotCopy() {
		if len(snapshot.Observations) != len(probes) {
			t.Fatalf("snapshot %d observations = %d, want %d", index+1, len(snapshot.Observations), len(probes))
		}
	}

	blockedGeneration := manager.Evaluate().Generation
	blockedSnapshot := marshalCurrentGeneration(t, manager)
	close(release)
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("released non-cooperative probe did not exit")
	}
	waitForManagedProbeLaneEmpty(t, manager, "postgres")
	if got := manager.Evaluate().Generation; got != blockedGeneration {
		t.Fatalf("late result changed generation to %d, want %d", got, blockedGeneration)
	}
	if got := marshalCurrentGeneration(t, manager); string(got) != string(blockedSnapshot) {
		t.Fatalf("late result changed blocked snapshot\nbefore=%s\nafter=%s", blockedSnapshot, got)
	}

	if err := manager.refresh(context.Background()); err != nil {
		t.Fatalf("reuse refresh: %v", err)
	}
	if got := blocking.calls.Load(); got != 2 {
		t.Fatalf("non-cooperative probe calls after release = %d, want 2", got)
	}
	if got := manager.Evaluate(); got.Generation <= blockedGeneration || got.ErrorCode != "" {
		t.Fatalf("reused lane evaluation = generation %d error %q", got.Generation, got.ErrorCode)
	}
	if err := manager.Require("identity.account_session"); err != nil {
		t.Fatalf("reused lane did not restore enabled capability: %v", err)
	}
}

func TestReadinessManagerProbeDeadlinePrecedenceContract(t *testing.T) {
	t.Run("already canceled context discards buffered enabled completion", func(t *testing.T) {
		probeCtx, cancel := context.WithCancel(context.Background())
		cancel()
		completed := make(chan probeResult, 1)
		completed <- probeResult{
			index:       7,
			observation: Observation{Availability: AvailabilityEnabled},
		}

		result := awaitProbeResult(probeCtx, 7, completed)
		if result.index != 7 {
			t.Fatalf("arbitrated probe index = %d, want 7", result.index)
		}
		if !errors.Is(result.err, context.Canceled) {
			t.Fatalf("arbitrated probe error = %v, want context canceled", result.err)
		}
		if result.observation.Availability != "" {
			t.Fatalf("expired completion availability = %q, want discarded", result.observation.Availability)
		}
	})

	t.Run("manager blocks enabled result returned after probe deadline", func(t *testing.T) {
		contract, profile, identity := loadReadinessTestAuthority(t)
		probes := replaceManagerProbe(
			managerTestProbes(t, contract, profile),
			"postgres",
			&enabledAfterCancellationManagerProbe{id: "probe.postgres", dependency: "postgres"},
		)
		manager := newTestManager(
			t,
			contract,
			profile,
			identity,
			newManagerTestClock(time.Date(2026, time.July, 19, 2, 0, 0, 0, time.UTC)),
			probes,
			5*time.Millisecond,
			&recordingSnapshotWriter{},
			filepath.Join(t.TempDir(), "readiness.json"),
		)

		if err := manager.Bootstrap(context.Background()); err != nil {
			t.Fatalf("bootstrap with post-deadline enabled result: %v", err)
		}
		assertBlockedDependencyObservation(t, manager.current.Load(), "postgres", len(probes))
		assertReadinessCode(t, manager.Require("identity.account_session"), CodeCapabilityBlocked)
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

type cancellationIgnoringHealthyManagerProbe struct {
	id         string
	dependency string
	calls      atomic.Int64
	release    <-chan struct{}
}

func (p *cancellationIgnoringHealthyManagerProbe) ID() string           { return p.id }
func (p *cancellationIgnoringHealthyManagerProbe) DependencyID() string { return p.dependency }
func (p *cancellationIgnoringHealthyManagerProbe) Run(ctx context.Context) (Observation, error) {
	p.calls.Add(1)
	if ctx.Err() != nil {
		<-p.release
	}
	return Observation{Availability: AvailabilityEnabled}, nil
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

type gatedManagerProbe struct {
	id          string
	dependency  string
	calls       atomic.Int64
	startedOnce sync.Once
	started     chan<- struct{}
	release     <-chan struct{}
}

func (p *gatedManagerProbe) ID() string           { return p.id }
func (p *gatedManagerProbe) DependencyID() string { return p.dependency }
func (p *gatedManagerProbe) Run(context.Context) (Observation, error) {
	p.calls.Add(1)
	p.startedOnce.Do(func() { close(p.started) })
	<-p.release
	return Observation{Availability: AvailabilityEnabled}, nil
}

type countedBlockingManagerProbe struct {
	id          string
	dependency  string
	calls       atomic.Int64
	startedOnce sync.Once
	exitedOnce  sync.Once
	started     chan<- struct{}
	exited      chan<- struct{}
	release     <-chan struct{}
}

func (p *countedBlockingManagerProbe) ID() string           { return p.id }
func (p *countedBlockingManagerProbe) DependencyID() string { return p.dependency }
func (p *countedBlockingManagerProbe) Run(context.Context) (Observation, error) {
	p.calls.Add(1)
	p.startedOnce.Do(func() { close(p.started) })
	<-p.release
	p.exitedOnce.Do(func() { close(p.exited) })
	return Observation{Availability: AvailabilityEnabled}, nil
}

type enabledAfterCancellationManagerProbe struct {
	id         string
	dependency string
}

func (p *enabledAfterCancellationManagerProbe) ID() string           { return p.id }
func (p *enabledAfterCancellationManagerProbe) DependencyID() string { return p.dependency }
func (p *enabledAfterCancellationManagerProbe) Run(ctx context.Context) (Observation, error) {
	<-ctx.Done()
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

func (w *recordingSnapshotWriter) snapshotCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.snapshots)
}

func (w *recordingSnapshotWriter) snapshotCopy() []ReadinessSnapshotV1 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]ReadinessSnapshotV1(nil), w.snapshots...)
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

func assertBlockedDependencyObservation(t *testing.T, generation *readinessGeneration, dependency string, observationCount int) {
	t.Helper()
	if generation == nil {
		t.Fatal("readiness generation is nil")
	}
	if len(generation.observations) != observationCount {
		t.Fatalf("generation observations = %d, want %d", len(generation.observations), observationCount)
	}
	for _, observation := range generation.observations {
		if observation.DependencyID != dependency {
			continue
		}
		if observation.Availability != AvailabilityBlocked || observation.ReasonCode != "dependency_unproven" {
			t.Fatalf("dependency %s observation = availability %q reason %q", dependency, observation.Availability, observation.ReasonCode)
		}
		return
	}
	t.Fatalf("dependency %s observation not found", dependency)
}

func waitForManagedProbeLaneEmpty(t *testing.T, manager *Manager, dependency string) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		for _, managed := range manager.probes {
			if managed.probe.DependencyID() == dependency {
				if len(managed.inFlight) == 0 {
					return
				}
				break
			}
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatalf("probe lane %s did not become empty", dependency)
		}
	}
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
