package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	stdhttp "net/http"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"oblivious/server/internal/buildinfo"
	serverconfig "oblivious/server/internal/config"
	serverhttp "oblivious/server/internal/http"
	"oblivious/server/internal/migrations"
	"oblivious/server/internal/releasecontract"
	sharedconfig "oblivious/server/pkg/config"
)

func TestStandaloneProfilePreflightGroupOneContract(t *testing.T) {
	profile := releasecontract.DeploymentProfile{
		ID: "monolith", Commitment: releasecontract.CommitmentCommitted,
		RefreshIntervalSeconds: 30, MaxAgeSeconds: 120, AllowedFutureSkewSeconds: 30,
		Topology:    releasecontract.Topology{Kind: releasecontract.TopologyMonolith, Components: []string{"server"}},
		Entrypoints: []string{"server", "agent", "billing", "channel", "chat"},
	}
	contract := releasecontract.AuthoredContractV1{SchemaVersion: releasecontract.SchemaVersionV1, Profiles: []releasecontract.DeploymentProfile{profile}}
	digest, err := releasecontract.Digest(contract)
	if err != nil {
		t.Fatal(err)
	}
	identity := testIdentity()
	identity.ContractDigest = digest

	for _, root := range []string{"server", "agent", "billing", "channel", "chat"} {
		t.Run(root+" success freezes one exact handoff", func(t *testing.T) {
			spies := &entrypointPreflightSpies{contract: contract, profile: profile, identity: identity}
			err := sharedconfig.RunEntrypoint(context.Background(), releasecontract.EntrypointID(root), spies.options("monolith"), func(_ context.Context, inputs sharedconfig.ResolvedEntrypointInputs) error {
				spies.continuations++
				spies.databaseOpens++
				spies.listeners++
				spies.providerCalls++
				if !reflect.DeepEqual(inputs.Contract(), contract) || !reflect.DeepEqual(inputs.Profile(), profile) || inputs.Identity() != identity {
					t.Fatalf("continuation received a different preflight handoff")
				}
				contractCopy := inputs.Contract()
				contractCopy.Profiles[0].Entrypoints[0] = "mutated"
				profileCopy := inputs.Profile()
				profileCopy.Entrypoints[0] = "mutated"
				if inputs.Contract().Profiles[0].Entrypoints[0] == "mutated" || inputs.Profile().Entrypoints[0] == "mutated" {
					t.Fatal("entrypoint handoff accessors retained mutable aliases")
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			spies.assertCounts(t, 1, 1)
		})
	}

	for _, test := range []struct {
		name      string
		profileID string
		mutate    func(*entrypointPreflightSpies)
	}{
		{name: "omitted profile", profileID: ""},
		{name: "unknown profile", profileID: "unknown", mutate: func(s *entrypointPreflightSpies) { s.profileErr = errors.New("profile_unknown") }},
		{name: "conditional profile", profileID: "monolith", mutate: func(s *entrypointPreflightSpies) {
			s.profile.Commitment = releasecontract.CommitmentConditional
			s.contract.Profiles[0] = s.profile
		}},
		{name: "excluded profile", profileID: "monolith", mutate: func(s *entrypointPreflightSpies) {
			s.profile.Commitment = releasecontract.CommitmentExcluded
			s.contract.Profiles[0] = s.profile
		}},
		{name: "entrypoint missing", profileID: "monolith", mutate: func(s *entrypointPreflightSpies) {
			s.profile.Entrypoints = []string{"server"}
			s.contract.Profiles[0] = s.profile
		}},
		{name: "identity mismatched", profileID: "monolith", mutate: func(s *entrypointPreflightSpies) {
			s.identity.ContractDigest = "sha256:" + strings.Repeat("d", 64)
		}},
	} {
		for _, root := range []string{"server", "agent", "billing", "channel", "chat"} {
			t.Run(root+" "+test.name+" stops before mutable dependencies", func(t *testing.T) {
				spies := &entrypointPreflightSpies{contract: contract, profile: profile, identity: identity}
				if test.mutate != nil {
					test.mutate(spies)
				}
				err := sharedconfig.RunEntrypoint(context.Background(), releasecontract.EntrypointID(root), spies.options(test.profileID), func(context.Context, sharedconfig.ResolvedEntrypointInputs) error {
					spies.continuations++
					spies.databaseOpens++
					spies.listeners++
					spies.providerCalls++
					return nil
				})
				if err == nil {
					t.Fatal("preflight unexpectedly authorized startup")
				}
				if spies.continuations != 0 || spies.databaseOpens != 0 || spies.listeners != 0 || spies.providerCalls != 0 {
					t.Fatalf("startup effects after denial: continuation=%d db=%d listener=%d provider=%d", spies.continuations, spies.databaseOpens, spies.listeners, spies.providerCalls)
				}
			})
		}
	}

	for root, source := range map[string]string{
		"server": "main.go", "agent": "../agent/main.go", "billing": "../billing/main.go",
		"channel": "../channel/main.go", "chat": "../chat/main.go",
	} {
		t.Run(root+" source-authored first call", func(t *testing.T) {
			assertEntrypointFirstCall(t, source, root)
		})
	}
}

func TestRuntimeBuildAndBackgroundLifecycleContract(t *testing.T) {
	root, err := filepath.Abs("../../../../")
	if err != nil {
		t.Fatal(err)
	}
	contract, err := releasecontract.Load(context.Background(), root, "config/release/contract.v1.json", "config/release/contract.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var profile releasecontract.DeploymentProfile
	for _, candidate := range contract.Profiles {
		if candidate.ID == "monolith" {
			profile = candidate
			break
		}
	}
	guard := &runtimeGuardSpy{}
	authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
	if err != nil {
		t.Fatal(err)
	}
	effects := &runtimeEffectRegistrarSpy{}
	manager := runtimeReadinessManagerStub{}
	database, err := sql.Open("postgres", "postgres://unused")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	runtime, err := serverhttp.BuildRuntime(serverconfig.Config{
		Port: 0, Env: "test", ModelDefaultName: "demo-reply", RelayDefaultModel: "gpt-4o-mini", DatabaseURL: "postgres://unused",
	}, database, serverhttp.RuntimeOptions{Readiness: manager, Guard: guard, Effects: effects, Authorities: authorities})
	if err != nil {
		t.Fatalf("BuildRuntime: %v", err)
	}
	if len(effects.descriptors) == 0 {
		t.Fatal("strict runtime constructors did not register any effect descriptors")
	}
	if len(guard.calls) != 0 {
		t.Fatalf("BuildRuntime called readiness guard %d times", len(guard.calls))
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := runtime.StartBackground(ctx); err != nil {
		t.Fatalf("StartBackground: %v", err)
	}
	if err := runtime.StartBackground(ctx); err == nil {
		t.Fatal("duplicate StartBackground unexpectedly succeeded")
	}
	if err := runtime.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := runtime.Close(context.Background()); err != nil {
		t.Fatalf("idempotent Close: %v", err)
	}
	if err := runtime.StartBackground(context.Background()); err == nil {
		t.Fatal("StartBackground after Close unexpectedly succeeded")
	}
	if len(guard.calls) != 0 {
		t.Fatalf("lifecycle test observed readiness effects despite disabled workers: %d", len(guard.calls))
	}
	if _, err := serverhttp.BuildRuntime(serverconfig.Config{Env: "test"}, database, serverhttp.RuntimeOptions{Readiness: manager, Guard: guard, Effects: effects}); err == nil {
		t.Fatal("zero RuntimeAuthorities unexpectedly accepted")
	}
}

func TestRuntimeEffectRegistryAllowsExactSharedDescriptor(t *testing.T) {
	registry := newRuntimeEffectRegistry()
	descriptor := releasecontract.EffectDescriptor{
		ID: "chat.provider.dispatch", CapabilityID: "relay.provider_inference",
		Boundary: releasecontract.BoundaryOutbound, Owner: "chat.RelayGateway",
	}
	if err := registry.Register(descriptor); err != nil {
		t.Fatalf("register first descriptor: %v", err)
	}
	if err := registry.Register(descriptor); err != nil {
		t.Fatalf("exact shared descriptor should be idempotent: %v", err)
	}
	conflict := descriptor
	conflict.Owner = "chat.CompositeGateway"
	if err := registry.Register(conflict); err == nil {
		t.Fatal("conflicting descriptor unexpectedly passed")
	}
}

func TestServerStartupOrderContract(t *testing.T) {
	profile := releasecontract.DeploymentProfile{
		ID: "monolith", Commitment: releasecontract.CommitmentCommitted,
		RefreshIntervalSeconds: 30, MaxAgeSeconds: 120, AllowedFutureSkewSeconds: 30,
		Topology:    releasecontract.Topology{Kind: releasecontract.TopologyMonolith, Components: []string{"server"}},
		Entrypoints: []string{"server"},
	}
	contract := releasecontract.AuthoredContractV1{SchemaVersion: releasecontract.SchemaVersionV1, Profiles: []releasecontract.DeploymentProfile{profile}}
	digest, err := releasecontract.Digest(contract)
	if err != nil {
		t.Fatal(err)
	}
	identity := testIdentity()
	identity.ContractDigest = digest

	t.Run("success orders listener before background", func(t *testing.T) {
		events := []string{}
		spies := &entrypointPreflightSpies{contract: contract, profile: profile, identity: identity}
		deps := startupContractDependencies(&events, "", nil, nil, nil)
		if err := sharedconfig.RunEntrypoint(context.Background(), "server", spies.options("monolith"), func(ctx context.Context, inputs sharedconfig.ResolvedEntrypointInputs) error {
			return runServerWithInputs(ctx, inputs, deps)
		}); err != nil {
			t.Fatal(err)
		}
		want := []string{"config", "db_open", "db_ping", "migrations", "manager_construct", "bootstrap", "authorities", "build_runtime", "listen", "serve", "refresh", "background", "close"}
		got := snapshotStartupEvents(events)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("startup events = %#v, want %#v", got, want)
		}
	})

	for _, test := range []struct {
		name string
		fail string
		want []string
	}{
		{name: "database open", fail: "db_open", want: []string{"config", "db_open"}},
		{name: "database ping", fail: "db_ping", want: []string{"config", "db_open", "db_ping"}},
		{name: "migration", fail: "migrations", want: []string{"config", "db_open", "db_ping", "migrations"}},
		{name: "authority", fail: "authorities", want: []string{"config", "db_open", "db_ping", "migrations", "manager_construct", "bootstrap", "authorities"}},
		{name: "runtime", fail: "build_runtime", want: []string{"config", "db_open", "db_ping", "migrations", "manager_construct", "bootstrap", "authorities", "build_runtime"}},
		{name: "bind", fail: "listen", want: []string{"config", "db_open", "db_ping", "migrations", "manager_construct", "bootstrap", "authorities", "build_runtime", "listen", "close"}},
	} {
		t.Run(test.name+" fails closed", func(t *testing.T) {
			events := []string{}
			spies := &entrypointPreflightSpies{contract: contract, profile: profile, identity: identity}
			deps := startupContractDependencies(&events, test.fail, nil, nil, nil)
			if err := sharedconfig.RunEntrypoint(context.Background(), "server", spies.options("monolith"), func(ctx context.Context, inputs sharedconfig.ResolvedEntrypointInputs) error {
				return runServerWithInputs(ctx, inputs, deps)
			}); err == nil {
				t.Fatal("startup unexpectedly succeeded")
			}
			got := snapshotStartupEvents(events)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("failure events = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestDeploymentOwnedReadinessProbeContract(t *testing.T) {
	database, err := sql.Open("postgres", "postgres://unused")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	probes, err := newDeploymentReadinessProbes(serverconfig.Config{
		RedisAddr:        "redis.internal:6379",
		QdrantURL:        "http://qdrant.internal:6333",
		ClickHouseDSN:    "tcp://clickhouse.internal:9000?database=oblivious",
		ClickHouseDriver: "clickhouse",
		KafkaBrokers:     []string{"kafka.internal:9092"},
	}, database)
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]string, len(probes))
	for _, probe := range probes {
		got[probe.DependencyID()] = probe.ID()
	}
	want := map[string]string{
		"postgres":   "runtime.postgres",
		"redis":      "runtime.redis",
		"qdrant":     "runtime.qdrant",
		"clickhouse": "runtime.clickhouse",
		"kafka":      "runtime.kafka",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deployment probes = %#v, want %#v", got, want)
	}
	for _, broker := range []string{
		" kafka.internal:9092",
		"kafka .internal:9092",
		"kafka.internal/path:9092",
		"kafka..internal:9092",
		"-kafka.internal:9092",
		strings.Repeat("a", 64) + ".internal:9092",
	} {
		cfg := serverconfig.Config{KafkaBrokers: []string{broker}}
		_, err := newDeploymentReadinessProbes(cfg, database)
		if err == nil || !strings.Contains(err.Error(), "construct kafka readiness probe: invalid configuration") {
			t.Fatalf("programmatic broker %q error = %v", broker, err)
		}
		if strings.Contains(err.Error(), broker) {
			t.Fatalf("factory error leaked raw broker %q: %v", broker, err)
		}
	}

	configured := serverconfig.Config{
		RedisAddr:        "redis.internal:6379",
		QdrantURL:        "http://qdrant.internal:6333",
		ClickHouseDSN:    "tcp://user:dsn-secret@clickhouse.internal:9000?database=oblivious",
		ClickHouseDriver: "clickhouse",
		KafkaBrokers:     []string{"kafka.internal:9092"},
	}
	newOperations := func(t *testing.T, failDependency string, calls map[string]int) deploymentReadinessOperations {
		t.Helper()
		call := func(ctx context.Context, dependencyID string) error {
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > deploymentReadinessProbeTimeout+time.Second {
				t.Fatalf("%s probe did not receive bounded context", dependencyID)
			}
			calls[dependencyID]++
			if dependencyID == failDependency {
				return errors.New("dsn-secret endpoint.internal response-body-secret")
			}
			return nil
		}
		return deploymentReadinessOperations{
			postgres:   func(ctx context.Context, _ *sql.DB) error { return call(ctx, "postgres") },
			redis:      func(ctx context.Context, _ serverconfig.Config) error { return call(ctx, "redis") },
			qdrant:     func(ctx context.Context, _ serverconfig.Config, _ string) error { return call(ctx, "qdrant") },
			clickhouse: func(ctx context.Context, _ serverconfig.Config) error { return call(ctx, "clickhouse") },
			kafka:      func(ctx context.Context, _ []string) error { return call(ctx, "kafka") },
		}
	}

	t.Run("all typed operations are bounded and enabled only on success", func(t *testing.T) {
		calls := map[string]int{}
		probes, err := newDeploymentReadinessProbesWithOperations(configured, database, newOperations(t, "", calls))
		if err != nil {
			t.Fatal(err)
		}
		for _, probe := range probes {
			observation, err := probe.Run(context.Background())
			if err != nil || observation.Availability != releasecontract.AvailabilityEnabled || observation.ReasonCode != "" {
				t.Fatalf("%s success observation = %#v, error=%v", probe.DependencyID(), observation, err)
			}
		}
		for dependencyID := range want {
			if calls[dependencyID] != 1 {
				t.Fatalf("%s calls = %d, want 1", dependencyID, calls[dependencyID])
			}
		}
	})

	for dependencyID := range want {
		t.Run(dependencyID+" failure is blocked and sanitized", func(t *testing.T) {
			calls := map[string]int{}
			probes, err := newDeploymentReadinessProbesWithOperations(configured, database, newOperations(t, dependencyID, calls))
			if err != nil {
				t.Fatal(err)
			}
			for _, probe := range probes {
				if probe.DependencyID() != dependencyID {
					continue
				}
				observation, runErr := probe.Run(context.Background())
				if runErr != nil || observation.Availability != releasecontract.AvailabilityBlocked || observation.ReasonCode != "dependency_unproven" || observation.Detail != nil {
					t.Fatalf("failure observation = %#v, error=%v", observation, runErr)
				}
				encoded, _ := json.Marshal(observation)
				for _, forbidden := range []string{"dsn-secret", "endpoint.internal", "response-body-secret"} {
					if strings.Contains(string(encoded), forbidden) {
						t.Fatalf("sanitized observation leaked %q: %s", forbidden, encoded)
					}
				}
			}
			if calls[dependencyID] != 1 {
				t.Fatalf("%s calls = %d, want 1", dependencyID, calls[dependencyID])
			}
		})
	}

	t.Run("missing deployment config stays blocked and ignores synthetic authority", func(t *testing.T) {
		t.Setenv("OBLIVIOUS_READINESS_PROBE_BASE_URL", "http://synthetic-authority.invalid")
		probes, err := newDeploymentReadinessProbes(serverconfig.Config{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, probe := range probes {
			observation, runErr := probe.Run(context.Background())
			if runErr != nil || observation.Availability != releasecontract.AvailabilityBlocked || observation.ReasonCode != "dependency_unproven" {
				t.Fatalf("%s missing-config observation = %#v, error=%v", probe.DependencyID(), observation, runErr)
			}
		}
	})

	t.Run("parent deadline bounds context-honoring work", func(t *testing.T) {
		probe := deploymentReadinessProbe{id: "runtime.redis", dependencyID: "redis", run: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		started := time.Now()
		observation, err := probe.Run(ctx)
		if err != nil || observation.Availability != releasecontract.AvailabilityBlocked || time.Since(started) > time.Second {
			t.Fatalf("deadline observation=%#v error=%v duration=%s", observation, err, time.Since(started))
		}
	})
}

func startupContractDependencies(events *[]string, fail string, _ *sql.DB, _ net.Listener, _ *stdhttp.Server) startupDependencies {
	appendEvent := func(event string) {
		startupEventsMu.Lock()
		defer startupEventsMu.Unlock()
		*events = append(*events, event)
	}
	manager := &startupManagerSpy{events: events}
	return startupDependencies{
		loadConfig: func() (serverconfig.Config, error) {
			appendEvent("config")
			if fail == "config" {
				return serverconfig.Config{}, errors.New("config failure")
			}
			return serverconfig.Config{Env: "test", DatabaseURL: "unused", Port: 0}, nil
		},
		openDatabase: func(string) (*sql.DB, error) {
			appendEvent("db_open")
			if fail == "db_open" {
				return nil, errors.New("db open failure")
			}
			database, _ := sql.Open("postgres", "postgres://unused")
			return database, nil
		},
		pingDatabase: func(context.Context, *sql.DB) error {
			appendEvent("db_ping")
			if fail == "db_ping" {
				return errors.New("db ping failure")
			}
			return nil
		},
		applyMigrations: func(context.Context, *sql.DB, string) (migrations.Result, error) {
			appendEvent("migrations")
			if fail == "migrations" {
				return migrations.Result{}, errors.New("migration failure")
			}
			return migrations.Result{}, nil
		},
		newManager: func(context.Context, serverconfig.ResolvedEntrypointInputs, serverconfig.Config, *sql.DB, string) (releasecontract.ReadinessManager, error) {
			appendEvent("manager_construct")
			if fail == "manager_construct" {
				return nil, errors.New("manager failure")
			}
			return manager, nil
		},
		newAuthorities: func(releasecontract.AuthoredContractV1, releasecontract.DeploymentProfile, releasecontract.Guard) (releasecontract.RuntimeAuthorities, error) {
			appendEvent("authorities")
			if fail == "authorities" {
				return releasecontract.RuntimeAuthorities{}, errors.New("authority failure")
			}
			return releasecontract.RuntimeAuthorities{}, nil
		},
		buildRuntime: func(serverconfig.Config, *sql.DB, serverhttp.RuntimeOptions) (*serverhttp.Runtime, error) {
			appendEvent("build_runtime")
			if fail == "build_runtime" {
				return nil, errors.New("runtime failure")
			}
			runtime := &serverhttp.Runtime{Server: &stdhttp.Server{Addr: ":0"}}
			runtime.StartBackground = func(context.Context) error {
				appendEvent("background")
				return nil
			}
			runtime.Close = func(context.Context) error {
				appendEvent("close")
				return nil
			}
			return runtime, nil
		},
		listen: func(string, string) (net.Listener, error) {
			appendEvent("listen")
			if fail == "listen" {
				return nil, errors.New("bind failure")
			}
			return startupListener{}, nil
		},
		serve: func(*stdhttp.Server, net.Listener) error {
			appendEvent("serve")
			return stdhttp.ErrServerClosed
		},
		serveStarted: make(chan struct{}),
	}
}

type startupManagerSpy struct{ events *[]string }

var startupEventsMu sync.Mutex

func snapshotStartupEvents(events []string) []string {
	startupEventsMu.Lock()
	defer startupEventsMu.Unlock()
	return append([]string(nil), events...)
}

func (m *startupManagerSpy) Bootstrap(context.Context) error {
	startupEventsMu.Lock()
	defer startupEventsMu.Unlock()
	*m.events = append(*m.events, "bootstrap")
	return nil
}
func (m *startupManagerSpy) StartRefresh(context.Context) {
	startupEventsMu.Lock()
	defer startupEventsMu.Unlock()
	*m.events = append(*m.events, "refresh")
}
func (m *startupManagerSpy) Require(string) error { return nil }
func (m *startupManagerSpy) Evaluate() releasecontract.Evaluation {
	return releasecontract.Evaluation{}
}
func (m *startupManagerSpy) ExportAudit(string) error { return nil }

type startupListener struct{}

func (startupListener) Accept() (net.Conn, error) { return nil, errors.New("closed") }
func (startupListener) Close() error              { return nil }
func (startupListener) Addr() net.Addr            { return startupAddr{} }

type startupAddr struct{}

func (startupAddr) Network() string { return "tcp" }
func (startupAddr) String() string  { return ":0" }

type runtimeGuardSpy struct{ calls []string }

func (g *runtimeGuardSpy) Require(_ context.Context, capabilityID string, _ releasecontract.Boundary) error {
	g.calls = append(g.calls, capabilityID)
	return nil
}

type runtimeEffectRegistrarSpy struct {
	descriptors []releasecontract.EffectDescriptor
}

func (r *runtimeEffectRegistrarSpy) Register(descriptor releasecontract.EffectDescriptor) error {
	r.descriptors = append(r.descriptors, descriptor)
	return nil
}

type runtimeReadinessManagerStub struct{}

func (runtimeReadinessManagerStub) Bootstrap(context.Context) error { return nil }
func (runtimeReadinessManagerStub) StartRefresh(context.Context)    {}
func (runtimeReadinessManagerStub) Require(string) error            { return nil }
func (runtimeReadinessManagerStub) Evaluate() releasecontract.Evaluation {
	return releasecontract.Evaluation{}
}
func (runtimeReadinessManagerStub) ExportAudit(string) error { return nil }

type entrypointPreflightSpies struct {
	contract releasecontract.AuthoredContractV1
	profile  releasecontract.DeploymentProfile
	identity buildinfo.BuildIdentityV1

	profileErr                                             error
	contractLoads, profileResolves, identityResolves       int
	continuations, databaseOpens, listeners, providerCalls int
}

func (s *entrypointPreflightSpies) options(profileID string) sharedconfig.EntrypointPreflightOptions {
	return sharedconfig.EntrypointPreflightOptions{
		RepoRoot: "/app", ContractPath: "contract.json", SchemaPath: "schema.json", ProfileID: profileID,
		Contracts: entrypointContractLoader{s}, Profiles: entrypointProfileResolver{s}, Identities: entrypointIdentityProvider{s},
	}
}

func (s *entrypointPreflightSpies) assertCounts(t *testing.T, resolverCalls, continuationCalls int) {
	t.Helper()
	if s.contractLoads != resolverCalls || s.profileResolves != resolverCalls || s.identityResolves != resolverCalls || s.continuations != continuationCalls {
		t.Fatalf("calls contract/profile/identity/continuation = %d/%d/%d/%d, want %d/%d/%d/%d", s.contractLoads, s.profileResolves, s.identityResolves, s.continuations, resolverCalls, resolverCalls, resolverCalls, continuationCalls)
	}
}

type entrypointContractLoader struct{ spies *entrypointPreflightSpies }

func (l entrypointContractLoader) Load(context.Context, string, string, string) (releasecontract.AuthoredContractV1, error) {
	l.spies.contractLoads++
	return l.spies.contract, nil
}

type entrypointProfileResolver struct{ spies *entrypointPreflightSpies }

func (r entrypointProfileResolver) ResolveCommittedProfile(context.Context, string, string, string, string) (releasecontract.DeploymentProfile, error) {
	r.spies.profileResolves++
	return r.spies.profile, r.spies.profileErr
}

type entrypointIdentityProvider struct{ spies *entrypointPreflightSpies }

func (p entrypointIdentityProvider) Resolve(context.Context, string, string, string) (buildinfo.BuildIdentityV1, error) {
	p.spies.identityResolves++
	return p.spies.identity, nil
}

func assertEntrypointFirstCall(t *testing.T, source, entrypoint string) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), source, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var mainDecl *ast.FuncDecl
	for _, declaration := range file.Decls {
		if candidate, ok := declaration.(*ast.FuncDecl); ok && candidate.Name.Name == "main" {
			mainDecl = candidate
			break
		}
	}
	if mainDecl == nil || len(mainDecl.Body.List) == 0 {
		t.Fatal("main function has no first statement")
	}
	statement, ok := mainDecl.Body.List[0].(*ast.IfStmt)
	if !ok || statement.Init == nil {
		t.Fatalf("first statement is %T, want preflight if statement", mainDecl.Body.List[0])
	}
	assignment, ok := statement.Init.(*ast.AssignStmt)
	if !ok || len(assignment.Rhs) != 1 {
		t.Fatal("first statement does not assign RunEntrypoint result")
	}
	call, ok := assignment.Rhs[0].(*ast.CallExpr)
	if !ok || len(call.Args) < 2 {
		t.Fatal("first statement is not a RunEntrypoint call")
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "RunEntrypoint" {
		t.Fatal("first fallible call is not RunEntrypoint")
	}
	idCall, ok := call.Args[1].(*ast.CallExpr)
	if !ok || len(idCall.Args) != 1 {
		t.Fatal("entrypoint ID is not an explicit source conversion")
	}
	literal, ok := idCall.Args[0].(*ast.BasicLit)
	if !ok || literal.Value != `"`+entrypoint+`"` {
		t.Fatalf("entrypoint literal = %v, want %q", call.Args[1], entrypoint)
	}
}

func TestIdentityInspectionPrecedesRuntimeSideEffects(t *testing.T) {
	identity := testIdentity()
	for _, test := range []struct {
		name     string
		args     []string
		provider buildinfo.IdentityProvider
		wantCode int
		wantRuns int
	}{
		{name: "inspection success", args: []string{buildinfo.InspectionFlag}, provider: staticInspectionProvider{identity: identity}, wantCode: 0},
		{name: "inspection failure", args: []string{buildinfo.InspectionFlag}, provider: staticInspectionProvider{err: &buildinfo.IdentityError{Code: buildinfo.ErrorBuildIdentityMismatch, Field: "releaseCommit"}}, wantCode: 1},
		{name: "normal startup", args: []string{"--serve"}, provider: staticInspectionProvider{identity: identity}, wantCode: 0, wantRuns: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			configLoads, databaseOpens, migrations, listeners := 0, 0, 0, 0
			exitCode := runMain(context.Background(), test.args, inspectionDependencies{
				provider: test.provider, stdout: &stdout, stderr: &stderr,
				repoRoot: "/app", contract: packagedContractPath, schema: packagedSchemaPath,
			}, func() {
				configLoads++
				databaseOpens++
				migrations++
				listeners++
			})
			if exitCode != test.wantCode {
				t.Fatalf("exit code = %d, want %d; stderr=%s", exitCode, test.wantCode, stderr.String())
			}
			for name, calls := range map[string]int{"config": configLoads, "database": databaseOpens, "migrations": migrations, "listener": listeners} {
				if calls != test.wantRuns {
					t.Fatalf("%s calls = %d, want %d", name, calls, test.wantRuns)
				}
			}
			switch test.name {
			case "inspection success":
				var got buildinfo.BuildIdentityV1
				if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || got != identity {
					t.Fatalf("inspection output = %q, identity=%#v, error=%v", stdout.String(), got, err)
				}
			case "inspection failure":
				if !strings.Contains(stderr.String(), string(buildinfo.ErrorBuildIdentityMismatch)) {
					t.Fatalf("inspection error = %q", stderr.String())
				}
			}
		})
	}
}

type staticInspectionProvider struct {
	identity buildinfo.BuildIdentityV1
	err      error
}

func (p staticInspectionProvider) Resolve(context.Context, string, string, string) (buildinfo.BuildIdentityV1, error) {
	if p.err != nil {
		return buildinfo.BuildIdentityV1{}, p.err
	}
	return p.identity, nil
}

func testIdentity() buildinfo.BuildIdentityV1 {
	return buildinfo.BuildIdentityV1{
		SchemaVersion: buildinfo.BuildIdentitySchemaV1, ReleaseCommit: strings.Repeat("a", 40),
		SourceTree: strings.Repeat("b", 40), ContractDigest: "sha256:" + strings.Repeat("c", 64),
		Dirty: false, EvidenceClass: buildinfo.EvidenceRepositoryLocal,
	}
}
