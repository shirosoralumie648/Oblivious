package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	stdhttp "net/http"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/config"
	serverhttp "oblivious/server/internal/http"
	"oblivious/server/internal/migrations"
	"oblivious/server/internal/releasecontract"

	_ "github.com/lib/pq"
)

const (
	packagedRepoRoot     = "/app"
	packagedContractPath = "config/release/contract.v1.json"
	packagedSchemaPath   = "config/release/contract.schema.json"
)

type inspectionDependencies struct {
	provider buildinfo.IdentityProvider
	stdout   io.Writer
	stderr   io.Writer
	repoRoot string
	contract string
	schema   string
}

func main() {
	if err := config.RunEntrypoint(context.Background(), releasecontract.EntrypointID("server"), config.EntrypointPreflightOptions{
		RepoRoot: packagedRepoRoot, ContractPath: packagedContractPath, SchemaPath: packagedSchemaPath,
		ProfileID: os.Getenv("OBLIVIOUS_DEPLOYMENT_PROFILE"), Contracts: config.FileContractLoader{},
		Identities: buildinfo.NewEmbeddedProvider(), Profiles: releasecontract.NewFileProfileResolver(),
	}, func(ctx context.Context, inputs config.ResolvedEntrypointInputs) error {
		exitCode := runMainWithInputs(ctx, os.Args[1:], inspectionDependencies{
			provider: buildinfo.NewEmbeddedProvider(), stdout: os.Stdout, stderr: os.Stderr,
			repoRoot: packagedRepoRoot, contract: packagedContractPath, schema: packagedSchemaPath,
		}, inputs)
		if exitCode != 0 {
			return fmt.Errorf("server exited with code %d", exitCode)
		}
		return nil
	}); err != nil {
		log.Fatalf("server preflight failed: %v", err)
	}
}

type startupDependencies struct {
	loadConfig      func() (config.Config, error)
	openDatabase    func(string) (*sql.DB, error)
	pingDatabase    func(context.Context, *sql.DB) error
	applyMigrations func(context.Context, *sql.DB, string) (migrations.Result, error)
	newManager      func(context.Context, config.ResolvedEntrypointInputs, string) (releasecontract.ReadinessManager, error)
	newAuthorities  func(releasecontract.AuthoredContractV1, releasecontract.DeploymentProfile, releasecontract.Guard) (releasecontract.RuntimeAuthorities, error)
	buildRuntime    func(config.Config, *sql.DB, serverhttp.RuntimeOptions) (*serverhttp.Runtime, error)
	listen          func(string, string) (net.Listener, error)
	serve           func(*stdhttp.Server, net.Listener) error
	serveStarted    chan struct{}
}

func defaultStartupDependencies() startupDependencies {
	return startupDependencies{
		loadConfig: config.Load,
		openDatabase: func(databaseURL string) (*sql.DB, error) {
			return sql.Open("postgres", databaseURL)
		},
		pingDatabase:    func(ctx context.Context, database *sql.DB) error { return database.PingContext(ctx) },
		applyMigrations: migrations.Apply,
		newManager: func(ctx context.Context, inputs config.ResolvedEntrypointInputs, auditPath string) (releasecontract.ReadinessManager, error) {
			return newStartupReadinessManager(ctx, inputs, auditPath)
		},
		newAuthorities: releasecontract.NewRuntimeAuthorities,
		buildRuntime: func(cfg config.Config, database *sql.DB, options serverhttp.RuntimeOptions) (*serverhttp.Runtime, error) {
			return serverhttp.BuildRuntime(cfg, database, options)
		},
		listen: net.Listen,
		serve:  func(server *stdhttp.Server, listener net.Listener) error { return server.Serve(listener) },
	}
}

func runMainWithInputs(ctx context.Context, args []string, deps inspectionDependencies, inputs config.ResolvedEntrypointInputs) int {
	if len(args) == 0 || args[0] != buildinfo.InspectionFlag {
		if err := runServerWithInputs(ctx, inputs, defaultStartupDependencies()); err != nil {
			if deps.stderr != nil {
				_, _ = fmt.Fprintf(deps.stderr, "server startup failed: %v\n", err)
			}
			return 1
		}
		return 0
	}
	if len(args) != 1 || deps.stdout == nil || deps.stderr == nil {
		return 2
	}
	encoded, err := buildinfo.MarshalIdentity(inputs.Identity())
	if err != nil {
		_, _ = fmt.Fprintf(deps.stderr, "build identity inspection failed: %v\n", err)
		return 1
	}
	if _, err := deps.stdout.Write(append(encoded, '\n')); err != nil {
		return 1
	}
	return 0
}

func runMain(ctx context.Context, args []string, deps inspectionDependencies, normalStartup func()) int {
	handled, exitCode := buildinfo.HandleInspection(ctx, args, deps.stdout, deps.stderr, deps.provider, deps.repoRoot, deps.contract, deps.schema)
	if handled {
		return exitCode
	}
	if normalStartup != nil {
		normalStartup()
	}
	return 0
}

func runServerWithInputs(ctx context.Context, inputs config.ResolvedEntrypointInputs, deps startupDependencies) error {
	if ctx == nil {
		return fmt.Errorf("server startup: context is required")
	}
	if deps.loadConfig == nil || deps.openDatabase == nil || deps.pingDatabase == nil || deps.applyMigrations == nil || deps.newManager == nil || deps.newAuthorities == nil || deps.buildRuntime == nil || deps.listen == nil || deps.serve == nil {
		return fmt.Errorf("server startup: incomplete dependencies")
	}
	cfg, err := deps.loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	database, err := deps.openDatabase(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	pingCtx, cancelPing := context.WithTimeout(ctx, 30*time.Second)
	err = deps.pingDatabase(pingCtx, database)
	cancelPing()
	if err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	migrationCtx, cancelMigrations := context.WithTimeout(ctx, 10*time.Minute)
	migrationResult, err := deps.applyMigrations(migrationCtx, database, "migrations")
	cancelMigrations()
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	log.Printf("migrations ready: applied=%d skipped=%d", migrationResult.Applied, migrationResult.Skipped)

	auditPath := os.Getenv("OBLIVIOUS_READINESS_AUDIT_PATH")
	manager, err := deps.newManager(ctx, inputs, auditPath)
	if err != nil {
		return fmt.Errorf("construct readiness manager: %w", err)
	}
	if err := manager.Bootstrap(ctx); err != nil {
		return fmt.Errorf("bootstrap readiness generation 1: %w", err)
	}
	guard := releasecontract.ManagerGuard{Manager: manager}
	authorities, err := deps.newAuthorities(inputs.Contract(), inputs.Profile(), guard)
	if err != nil {
		return fmt.Errorf("construct runtime authorities: %w", err)
	}
	effects := newRuntimeEffectRegistry()
	runtime, err := deps.buildRuntime(cfg, database, serverhttp.RuntimeOptions{
		Readiness: manager, Guard: guard, Effects: effects, Authorities: authorities,
	})
	if err != nil {
		return fmt.Errorf("build runtime: %w", err)
	}
	if runtime == nil || runtime.Server == nil || runtime.StartBackground == nil || runtime.Close == nil {
		return fmt.Errorf("build runtime: incomplete runtime")
	}
	listener, err := deps.listen("tcp", runtime.Server.Addr)
	if err != nil {
		_ = runtime.Close(context.Background())
		return fmt.Errorf("listen: %w", err)
	}
	serverErrors := make(chan error, 1)

	go func() {
		if deps.serveStarted != nil {
			close(deps.serveStarted)
		}
		log.Printf("server listening on %s env=%s", runtime.Server.Addr, cfg.Env)
		serverErrors <- deps.serve(runtime.Server, listener)
	}()
	if deps.serveStarted != nil {
		<-deps.serveStarted
	}

	rootCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	manager.StartRefresh(rootCtx)
	if err := runtime.StartBackground(rootCtx); err != nil {
		_ = runtime.Close(context.Background())
		return fmt.Errorf("start background: %w", err)
	}

	runtimeClosed := false
	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
			_ = runtime.Close(context.Background())
			return fmt.Errorf("server failed: %w", err)
		}
		if err := runtime.Close(context.Background()); err != nil {
			return fmt.Errorf("server shutdown: %w", err)
		}
		runtimeClosed = true
	case <-rootCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := runtime.Close(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown: %w", err)
		}
		runtimeClosed = true
	}
	if !runtimeClosed {
		if err := runtime.Close(context.Background()); err != nil {
			return fmt.Errorf("server shutdown: %w", err)
		}
	}
	return nil
}

type runtimeEffectRegistry struct {
	mu          sync.Mutex
	descriptors map[string]releasecontract.EffectDescriptor
}

func newRuntimeEffectRegistry() *runtimeEffectRegistry {
	return &runtimeEffectRegistry{descriptors: make(map[string]releasecontract.EffectDescriptor)}
}

func (r *runtimeEffectRegistry) Register(descriptor releasecontract.EffectDescriptor) error {
	if r == nil || descriptor.ID == "" || descriptor.CapabilityID == "" {
		return fmt.Errorf("effect descriptor is incomplete")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.descriptors[descriptor.ID]; exists {
		return fmt.Errorf("duplicate effect descriptor %q", descriptor.ID)
	}
	r.descriptors[descriptor.ID] = descriptor
	return nil
}

type startupBlockedProbe struct {
	dependencyID  string
	capabilityIDs []string
}

func (p startupBlockedProbe) ID() string           { return "startup." + p.dependencyID }
func (p startupBlockedProbe) DependencyID() string { return p.dependencyID }
func (p startupBlockedProbe) Run(context.Context) (releasecontract.Observation, error) {
	return releasecontract.Observation{Availability: releasecontract.AvailabilityBlocked, ReasonCode: "dependency_unproven", CapabilityIDs: append([]string(nil), p.capabilityIDs...)}, nil
}

func newStartupReadinessManager(_ context.Context, inputs config.ResolvedEntrypointInputs, auditPath string) (releasecontract.ReadinessManager, error) {
	if strings.TrimSpace(auditPath) == "" {
		return nil, fmt.Errorf("OBLIVIOUS_READINESS_AUDIT_PATH is required")
	}
	contract, profile := inputs.Contract(), inputs.Profile()
	capabilitiesByDependency := make(map[string][]string)
	for _, requirementID := range profile.ReadinessRequirementIDs {
		for _, requirement := range contract.ReadinessRequirements {
			if requirement.ID == requirementID {
				for _, dependencyID := range requirement.DependencyIDs {
					capabilitiesByDependency[dependencyID] = append(capabilitiesByDependency[dependencyID], requirement.CapabilityIDs...)
				}
			}
		}
	}
	probes := make([]releasecontract.Probe, 0, len(profile.Dependencies))
	probeBaseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("OBLIVIOUS_READINESS_PROBE_BASE_URL")), "/")
	for _, dependency := range profile.Dependencies {
		if probeBaseURL == "" {
			probes = append(probes, startupBlockedProbe{dependencyID: dependency.ID, capabilityIDs: capabilitiesByDependency[dependency.ID]})
			continue
		}
		probe, err := releasecontract.NewHTTPDependencyProbe(
			"runtime."+dependency.ID,
			dependency.ID,
			probeBaseURL+"/"+dependency.ID,
			&stdhttp.Client{Timeout: 5 * time.Second},
		)
		if err != nil {
			return nil, fmt.Errorf("construct readiness probe %s: %w", dependency.ID, err)
		}
		probes = append(probes, probe)
	}
	return releasecontract.NewManager(contract, inputs.Identity(), profile, releasecontract.NewEvaluator(), releasecontract.NewSystemClock(), probes, 5*time.Second, releasecontract.NewAtomicReadinessSnapshotWriter(), auditPath)
}
