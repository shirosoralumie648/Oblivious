package releasecontract

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Ticker interface {
	C() <-chan time.Time
	Stop()
}

type Clock interface {
	Now() time.Time
	NewTicker(time.Duration) Ticker
}

type systemClock struct{}

func NewSystemClock() Clock { return systemClock{} }

func (systemClock) Now() time.Time { return time.Now().UTC() }

func (systemClock) NewTicker(interval time.Duration) Ticker {
	return systemTicker{ticker: time.NewTicker(interval)}
}

type systemTicker struct{ ticker *time.Ticker }

func (t systemTicker) C() <-chan time.Time { return t.ticker.C }
func (t systemTicker) Stop()               { t.ticker.Stop() }

type Probe interface {
	ID() string
	DependencyID() string
	Run(context.Context) (Observation, error)
}

type ReadinessSnapshotWriter interface {
	Write(context.Context, string, ReadinessSnapshotV1) error
}

type AtomicReadinessSnapshotWriter struct{}

func NewAtomicReadinessSnapshotWriter() *AtomicReadinessSnapshotWriter {
	return &AtomicReadinessSnapshotWriter{}
}

func (w *AtomicReadinessSnapshotWriter) Write(ctx context.Context, destination string, snapshot ReadinessSnapshotV1) error {
	if w == nil || ctx == nil || strings.TrimSpace(destination) == "" {
		return readinessError(CodeReportOutputUnwritable, "destination", nil)
	}
	if err := ctx.Err(); err != nil {
		return readinessError(CodeReportOutputUnwritable, "context", err)
	}
	content, err := json.Marshal(snapshot)
	if err != nil {
		return readinessError(CodeReportOutputUnwritable, "snapshot", err)
	}
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return readinessError(CodeReportOutputUnwritable, "parent", err)
	}
	prior, priorExists, err := readReadinessDestination(destination)
	if err != nil {
		return readinessError(CodeReportOutputUnwritable, "destination", err)
	}
	staging, err := os.CreateTemp(parent, ".readiness-snapshot-staging-*")
	if err != nil {
		return readinessError(CodeReportOutputUnwritable, "staging", err)
	}
	stagingPath := staging.Name()
	defer func() {
		_ = staging.Close()
		_ = os.Remove(stagingPath)
	}()
	if _, err := staging.Write(content); err != nil {
		return readinessError(CodeReportOutputUnwritable, "write", err)
	}
	if err := staging.Sync(); err != nil {
		return readinessError(CodeReportOutputUnwritable, "file-sync", err)
	}
	if err := staging.Close(); err != nil {
		return readinessError(CodeReportOutputUnwritable, "close", err)
	}
	if err := ctx.Err(); err != nil {
		return readinessError(CodeReportOutputUnwritable, "context", err)
	}
	if err := os.Rename(stagingPath, destination); err != nil {
		return readinessError(CodeReportOutputUnwritable, "rename", err)
	}
	stagingPath = ""
	if err := syncReadinessDirectory(parent); err != nil {
		if restoreErr := restoreReadinessDestination(parent, destination, prior, priorExists); restoreErr != nil {
			return readinessError(CodeReportOutputUnwritable, "rollback-verification", errors.Join(err, restoreErr))
		}
		return readinessError(CodeReportOutputUnwritable, "parent-sync", err)
	}
	return nil
}

func readReadinessDestination(path string) ([]byte, bool, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return content, true, nil
}

func restoreReadinessDestination(parent, destination string, prior []byte, priorExists bool) error {
	if !priorExists {
		if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else {
		rollback, err := os.CreateTemp(parent, ".readiness-snapshot-rollback-*")
		if err != nil {
			return err
		}
		rollbackPath := rollback.Name()
		defer os.Remove(rollbackPath)
		if _, err := rollback.Write(prior); err != nil {
			_ = rollback.Close()
			return err
		}
		if err := rollback.Sync(); err != nil {
			_ = rollback.Close()
			return err
		}
		if err := rollback.Close(); err != nil {
			return err
		}
		if err := os.Rename(rollbackPath, destination); err != nil {
			return err
		}
	}
	if err := syncReadinessDirectory(parent); err != nil {
		return err
	}
	restored, exists, err := readReadinessDestination(destination)
	if err != nil {
		return err
	}
	if exists != priorExists || (priorExists && !bytes.Equal(restored, prior)) {
		return errors.New("restored readiness destination differs from prior state")
	}
	return nil
}

func syncReadinessDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

type Manager struct {
	contract      AuthoredContractV1
	identity      BuildIdentityV1
	profile       DeploymentProfile
	evaluator     Evaluator
	clock         Clock
	probes        []managedProbe
	probeDeadline time.Duration
	auditWriter   ReadinessSnapshotWriter
	auditPath     string

	current        atomic.Pointer[readinessGeneration]
	bootstrapMu    sync.Mutex
	publicationMu  sync.Mutex
	refreshStarted atomic.Bool
}

type managedProbe struct {
	probe         Probe
	capabilityIDs []string
	inFlight      chan struct{}
}

type readinessGeneration struct {
	evaluation   Evaluation
	observations []Observation
}

func NewManager(contract AuthoredContractV1, identity BuildIdentityV1, profile DeploymentProfile, evaluator Evaluator, clock Clock, probes []Probe, probeDeadline time.Duration, auditWriter ReadinessSnapshotWriter, auditPath string) (*Manager, error) {
	if evaluator == nil || clock == nil || probeDeadline <= 0 || auditWriter == nil || strings.TrimSpace(auditPath) == "" {
		return nil, readinessError(CodeReadinessUnavailable, "manager.dependencies", nil)
	}
	clonedContract, err := cloneAuthoredContract(contract)
	if err != nil {
		return nil, readinessError(CodeReadinessUnavailable, "contract", err)
	}
	clonedProfile, err := cloneDeploymentProfile(profile)
	if err != nil {
		return nil, readinessError(CodeReadinessUnavailable, "profile", err)
	}
	if err := validateReadinessIdentity(clonedContract, identity); err != nil {
		return nil, err
	}
	if err := validateReadinessProfile(clonedContract, clonedProfile); err != nil {
		return nil, err
	}
	_, dependencies, err := applicableCapabilityPolicy(clonedContract, clonedProfile)
	if err != nil {
		return nil, err
	}
	managed, err := compileApplicableProbes(probes, dependencies)
	if err != nil {
		return nil, err
	}
	return &Manager{
		contract:      clonedContract,
		identity:      identity,
		profile:       clonedProfile,
		evaluator:     evaluator,
		clock:         clock,
		probes:        managed,
		probeDeadline: probeDeadline,
		auditWriter:   auditWriter,
		auditPath:     auditPath,
	}, nil
}

func compileApplicableProbes(probes []Probe, expected map[string][]string) ([]managedProbe, error) {
	byDependency := make(map[string]Probe, len(probes))
	for _, probe := range probes {
		if probe == nil || strings.TrimSpace(probe.ID()) == "" || strings.TrimSpace(probe.DependencyID()) == "" {
			return nil, readinessError(CodeReadinessUnavailable, "probes", nil)
		}
		if _, applicable := expected[probe.DependencyID()]; !applicable {
			continue
		}
		if _, duplicate := byDependency[probe.DependencyID()]; duplicate {
			return nil, readinessError(CodeReadinessUnavailable, "probes.dependencyId", nil)
		}
		byDependency[probe.DependencyID()] = probe
	}
	managed := make([]managedProbe, 0, len(expected))
	for dependencyID, capabilityIDs := range expected {
		probe, ok := byDependency[dependencyID]
		if !ok {
			return nil, readinessError(CodeReadinessUnavailable, "probes.missing", nil)
		}
		managed = append(managed, managedProbe{
			probe:         probe,
			capabilityIDs: append([]string(nil), capabilityIDs...),
			inFlight:      make(chan struct{}, 1),
		})
	}
	sort.Slice(managed, func(i, j int) bool { return managed[i].probe.DependencyID() < managed[j].probe.DependencyID() })
	return managed, nil
}

func (m *Manager) Bootstrap(ctx context.Context) error {
	if m == nil {
		return readinessError(CodeReadinessUnavailable, "manager", nil)
	}
	m.bootstrapMu.Lock()
	defer m.bootstrapMu.Unlock()
	if m.current.Load() != nil {
		return readinessError(CodeReadinessUnavailable, "bootstrap", nil)
	}
	return m.refresh(ctx)
}

func (m *Manager) StartRefresh(ctx context.Context) {
	if m == nil || ctx == nil || !m.refreshStarted.CompareAndSwap(false, true) {
		return
	}
	interval := time.Duration(m.profile.RefreshIntervalSeconds) * time.Second
	ticker := m.clock.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		defer m.refreshStarted.Store(false)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C():
				_ = m.refresh(ctx)
			}
		}
	}()
}

func (m *Manager) refresh(ctx context.Context) error {
	if m == nil || ctx == nil {
		return readinessError(CodeReadinessUnavailable, "manager", nil)
	}
	observations, err := m.runBoundedProbes(ctx)
	if err != nil {
		return err
	}
	m.publicationMu.Lock()
	defer m.publicationMu.Unlock()

	nextGeneration := uint64(1)
	if current := m.current.Load(); current != nil {
		nextGeneration = current.evaluation.Generation + 1
		if nextGeneration == 0 {
			return readinessError(CodeReadinessUnavailable, "generation", nil)
		}
	}
	evaluation, err := m.evaluator.Evaluate(m.contract, m.identity, m.profile, nextGeneration, observations, m.clock.Now())
	if err != nil {
		return err
	}
	candidate := &readinessGeneration{
		evaluation:   cloneEvaluation(evaluation),
		observations: cloneObservationsForManager(observations),
	}
	m.current.Store(candidate)
	if err := m.auditWriter.Write(ctx, m.auditPath, snapshotFromGeneration(candidate)); err != nil {
		return readinessError(CodeReportOutputUnwritable, "audit", err)
	}
	return nil
}

type probeResult struct {
	index       int
	observation Observation
	err         error
}

func (m *Manager) runBoundedProbes(ctx context.Context) ([]Observation, error) {
	results := make(chan probeResult, len(m.probes))
	for index, managed := range m.probes {
		go func(index int, managed managedProbe) {
			select {
			case managed.inFlight <- struct{}{}:
			case <-ctx.Done():
				results <- probeResult{index: index, err: ctx.Err()}
				return
			default:
				results <- probeResult{index: index, err: context.DeadlineExceeded}
				return
			}
			probeCtx, cancel := context.WithTimeout(ctx, m.probeDeadline)
			defer cancel()
			completed := make(chan probeResult, 1)
			go func() {
				defer func() { <-managed.inFlight }()
				observation, err := managed.probe.Run(probeCtx)
				completed <- probeResult{index: index, observation: observation, err: err}
			}()
			select {
			case result := <-completed:
				results <- result
			case <-probeCtx.Done():
				if ctx.Err() != nil {
					results <- probeResult{index: index, err: ctx.Err()}
					return
				}
				results <- probeResult{index: index, err: context.DeadlineExceeded}
			}
		}(index, managed)
	}

	observations := make([]Observation, len(m.probes))
	for range m.probes {
		result := <-results
		if ctx.Err() != nil {
			return nil, fmt.Errorf("readiness refresh canceled: %w", ctx.Err())
		}
		managed := m.probes[result.index]
		observation := result.observation
		if result.err != nil {
			observation.Availability = AvailabilityBlocked
			observation.ReasonCode = "dependency_unproven"
			observation.Detail = nil
		}
		observation.ProbeID = managed.probe.ID()
		observation.DependencyID = managed.probe.DependencyID()
		observation.CapabilityIDs = append([]string(nil), managed.capabilityIDs...)
		observation.ObservedAt = normalizeTime(m.clock.Now())
		observations[result.index] = cloneObservation(observation)
	}
	return observations, nil
}

func (m *Manager) Require(capabilityID string) error {
	if m == nil || strings.TrimSpace(capabilityID) == "" {
		return readinessError(CodeCapabilityUnknown, "capabilityId", nil)
	}
	current := m.current.Load()
	if current == nil {
		return readinessError(CodeReadinessUnavailable, "generation", nil)
	}
	evaluation, err := m.evaluator.Evaluate(m.contract, m.identity, m.profile, current.evaluation.Generation, current.observations, m.clock.Now())
	if err != nil {
		return err
	}
	capability, ok := evaluation.Capabilities[capabilityID]
	if !ok {
		return readinessError(CodeCapabilityUnknown, "capabilityId", nil)
	}
	switch capability.Availability {
	case AvailabilityEnabled:
		return nil
	case AvailabilityDisabled:
		return readinessError(CodeCapabilityDisabled, "capabilityId", nil)
	case AvailabilityBlocked:
		return readinessError(CodeCapabilityBlocked, "capabilityId", nil)
	default:
		return readinessError(CodeReadinessUnavailable, "capability", nil)
	}
}

func (m *Manager) Evaluate() Evaluation {
	if m == nil {
		return Evaluation{ErrorCode: CodeReadinessUnavailable}
	}
	current := m.current.Load()
	if current == nil {
		return Evaluation{ErrorCode: CodeReadinessUnavailable}
	}
	evaluation, err := m.evaluator.Evaluate(m.contract, m.identity, m.profile, current.evaluation.Generation, current.observations, m.clock.Now())
	if err == nil {
		return cloneEvaluation(evaluation)
	}
	result := cloneEvaluation(current.evaluation)
	var readinessErr *ReadinessError
	if errors.As(err, &readinessErr) {
		result.ErrorCode = readinessErr.Code
	} else {
		result.ErrorCode = CodeReadinessUnavailable
	}
	return result
}

func (m *Manager) ExportAudit(path string) error {
	if m == nil || strings.TrimSpace(path) == "" {
		return readinessError(CodeReportOutputUnwritable, "destination", nil)
	}
	current := m.current.Load()
	if current == nil {
		return readinessError(CodeReadinessUnavailable, "generation", nil)
	}
	if err := m.auditWriter.Write(context.Background(), path, snapshotFromGeneration(current)); err != nil {
		return readinessError(CodeReportOutputUnwritable, "audit", err)
	}
	return nil
}

func snapshotFromGeneration(generation *readinessGeneration) ReadinessSnapshotV1 {
	return ReadinessSnapshotV1{
		SchemaVersion: ReadinessSnapshotSchemaV1,
		Identity:      generation.evaluation.Identity,
		Profile:       generation.evaluation.Profile,
		Generation:    generation.evaluation.Generation,
		CheckedAt:     normalizeTime(generation.evaluation.CheckedAt),
		ValidUntil:    normalizeTime(generation.evaluation.ValidUntil),
		Observations:  cloneObservationsForManager(generation.observations),
		Capabilities:  cloneCapabilityEvaluations(generation.evaluation.Capabilities),
	}
}

func cloneObservationsForManager(source []Observation) []Observation {
	result := make([]Observation, len(source))
	for i := range source {
		result[i] = cloneObservation(source[i])
	}
	return result
}

func cloneAuthoredContract(contract AuthoredContractV1) (AuthoredContractV1, error) {
	content, err := json.Marshal(contract)
	if err != nil {
		return AuthoredContractV1{}, err
	}
	var cloned AuthoredContractV1
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cloned); err != nil {
		return AuthoredContractV1{}, err
	}
	return cloned, nil
}

func cloneDeploymentProfile(profile DeploymentProfile) (DeploymentProfile, error) {
	content, err := json.Marshal(profile)
	if err != nil {
		return DeploymentProfile{}, err
	}
	var cloned DeploymentProfile
	if err := json.Unmarshal(content, &cloned); err != nil {
		return DeploymentProfile{}, err
	}
	return cloned, nil
}

type HTTPDependencyProbe struct {
	probeID      string
	dependencyID string
	endpoint     *url.URL
	client       *http.Client
}

func NewHTTPDependencyProbe(probeID, dependencyID, endpoint string, client *http.Client) (*HTTPDependencyProbe, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || strings.TrimSpace(probeID) == "" || strings.TrimSpace(dependencyID) == "" || client == nil {
		return nil, readinessError(CodeReadinessUnavailable, "httpProbe", err)
	}
	return &HTTPDependencyProbe{probeID: probeID, dependencyID: dependencyID, endpoint: parsed, client: client}, nil
}

func (p *HTTPDependencyProbe) ID() string {
	if p == nil {
		return ""
	}
	return p.probeID
}

func (p *HTTPDependencyProbe) DependencyID() string {
	if p == nil {
		return ""
	}
	return p.dependencyID
}

func (p *HTTPDependencyProbe) Run(ctx context.Context) (Observation, error) {
	if p == nil || p.client == nil || p.endpoint == nil {
		return Observation{}, readinessError(CodeReadinessUnavailable, "httpProbe", nil)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint.String(), nil)
	if err != nil {
		return Observation{Availability: AvailabilityBlocked, ReasonCode: "dependency_unproven"}, nil
	}
	response, err := p.client.Do(request)
	if err != nil {
		return Observation{Availability: AvailabilityBlocked, ReasonCode: "dependency_unproven"}, nil
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return Observation{Availability: AvailabilityBlocked, ReasonCode: "dependency_unproven", Detail: map[string]string{"httpStatus": response.Status}}, nil
	}
	var body struct {
		Availability Availability `json:"availability"`
		ReasonCode   string       `json:"reasonCode,omitempty"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return Observation{Availability: Availability("malformed")}, nil
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Observation{Availability: Availability("malformed")}, nil
	}
	return Observation{Availability: body.Availability, ReasonCode: body.ReasonCode}, nil
}
