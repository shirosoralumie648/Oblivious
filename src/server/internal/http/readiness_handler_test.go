package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/releasecontract"
)

type controlPlaneReadinessManager struct{ evaluation releasecontract.Evaluation }

func (m *controlPlaneReadinessManager) Bootstrap(context.Context) error { return nil }
func (m *controlPlaneReadinessManager) StartRefresh(context.Context)    {}
func (m *controlPlaneReadinessManager) Require(capability string) error {
	item, ok := m.evaluation.Capabilities[capability]
	if !ok {
		return &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityUnknown}
	}
	if item.Availability == releasecontract.AvailabilityBlocked {
		return &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityBlocked}
	}
	return nil
}
func (m *controlPlaneReadinessManager) Evaluate() releasecontract.Evaluation { return m.evaluation }
func (m *controlPlaneReadinessManager) ExportAudit(string) error             { return nil }

type controlPlaneGuard struct{}

func (controlPlaneGuard) Require(context.Context, string, releasecontract.Boundary) error { return nil }

func TestReadinessControlPlaneContract(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", ".."))
	contract, err := releasecontract.Load(context.Background(), repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json")
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}
	profile, err := releasecontract.NewFileProfileResolver().ResolveCommittedProfile(context.Background(), repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith")
	if err != nil {
		t.Fatalf("resolve profile: %v", err)
	}
	digest, err := releasecontract.Digest(contract)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	identity := buildinfo.BuildIdentityV1{SchemaVersion: buildinfo.BuildIdentitySchemaV1, ReleaseCommit: strings.Repeat("a", 40), SourceTree: strings.Repeat("b", 40), ContractDigest: digest, EvidenceClass: buildinfo.EvidenceRepositoryLocal}
	authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, controlPlaneGuard{})
	if err != nil {
		t.Fatalf("authorities: %v", err)
	}
	evaluation := releasecontract.Evaluation{
		Identity: identity, Profile: profile.ID, Generation: 7,
		CheckedAt: time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC), ValidUntil: time.Date(2026, 7, 18, 0, 2, 0, 0, time.UTC),
		Capabilities: map[string]releasecontract.CapabilityEvaluation{
			"required":    {CapabilityID: "required", Commitment: releasecontract.CommitmentCommitted, Availability: releasecontract.AvailabilityBlocked, ReasonCode: "dependency_unavailable"},
			"conditional": {CapabilityID: "conditional", Commitment: releasecontract.CommitmentConditional, Availability: releasecontract.AvailabilityDisabled, ReasonCode: "not_configured"},
			"excluded":    {CapabilityID: "excluded", Commitment: releasecontract.CommitmentExcluded, Availability: releasecontract.AvailabilityDisabled, ReasonCode: "profile_parity_unproven"},
		},
	}
	manager := &controlPlaneReadinessManager{evaluation: evaluation}
	handlers := NewReadinessHandlers(ReadinessHandlerOptions{Readiness: manager, Authorities: authorities})

	livez := httptest.NewRecorder()
	handlers.Livez(livez, httptest.NewRequest(http.MethodGet, "/livez", nil))
	if livez.Code != http.StatusOK {
		t.Fatalf("livez status = %d", livez.Code)
	}
	readyz := httptest.NewRecorder()
	handlers.Readyz(readyz, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if readyz.Code != http.StatusServiceUnavailable || !strings.Contains(readyz.Body.String(), string(releasecontract.CodeCapabilityBlocked)) {
		t.Fatalf("readyz = %d %s", readyz.Code, readyz.Body.String())
	}
	admin := httptest.NewRecorder()
	handlers.Admin(admin, httptest.NewRequest(http.MethodGet, "/api/v1/admin/readiness", nil))
	if admin.Code != http.StatusOK || !strings.Contains(admin.Body.String(), "excluded") || !strings.Contains(admin.Body.String(), "evidenceRefs") {
		t.Fatalf("admin = %d %s", admin.Code, admin.Body.String())
	}
	app := httptest.NewRecorder()
	handlers.App(app, httptest.NewRequest(http.MethodGet, "/api/v1/app/readiness/capabilities", nil))
	if app.Code != http.StatusOK || strings.Contains(app.Body.String(), "excluded") || strings.Contains(app.Body.String(), "evidenceRefs") {
		t.Fatalf("app = %d %s", app.Code, app.Body.String())
	}
}

func TestReadinessProjectionEvaluationErrorContract(t *testing.T) {
	_, _, _, authorities := readinessProjectionTestAuthority(t)
	const (
		rawEndpoint = "https://readiness.internal.example/probe?credential=operator"
		rawSecret   = "sk-readiness-projection-secret"
	)

	for _, errorCode := range []releasecontract.ReadinessCode{
		releasecontract.CodeReadinessStale,
		releasecontract.CodeReadinessUnavailable,
		releasecontract.CodeBuildIdentityMismatch,
	} {
		t.Run(string(errorCode), func(t *testing.T) {
			evaluation := releasecontract.Evaluation{
				Identity: releasecontract.BuildIdentityV1{SourceTree: rawEndpoint, ContractDigest: rawSecret},
				Profile:  rawSecret, Generation: 9, ErrorCode: errorCode,
				CheckedAt:  time.Date(2026, time.July, 19, 0, 0, 0, 0, time.UTC),
				ValidUntil: time.Date(2026, time.July, 19, 0, 2, 0, 0, time.UTC),
				Capabilities: map[string]releasecontract.CapabilityEvaluation{
					"leaky.enabled": {
						CapabilityID: "leaky.enabled", Commitment: releasecontract.CommitmentCommitted,
						Availability: releasecontract.AvailabilityEnabled, ReasonCode: rawSecret,
						Dependencies: []string{rawEndpoint},
					},
				},
			}
			handlers := NewReadinessHandlers(ReadinessHandlerOptions{
				Readiness:   &controlPlaneReadinessManager{evaluation: evaluation},
				Authorities: authorities,
			})

			for _, endpoint := range []struct {
				name string
				path string
				call func(http.ResponseWriter, *http.Request)
			}{
				{name: "admin", path: "/api/v1/admin/readiness", call: handlers.Admin},
				{name: "app", path: "/api/v1/app/readiness/capabilities", call: handlers.App},
			} {
				t.Run(endpoint.name, func(t *testing.T) {
					response := httptest.NewRecorder()
					endpoint.call(response, httptest.NewRequest(http.MethodGet, endpoint.path, nil))
					body := response.Body.String()
					if response.Code != http.StatusServiceUnavailable {
						t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusServiceUnavailable, body)
					}
					if !strings.Contains(body, string(errorCode)) {
						t.Fatalf("body = %s, want stable code %q", body, errorCode)
					}
					for _, forbidden := range []string{"leaky.enabled", `"enabled":true`, rawEndpoint, rawSecret} {
						if strings.Contains(body, forbidden) {
							t.Fatalf("body leaked %q: %s", forbidden, body)
						}
					}
				})
			}
		})
	}
}

func TestReadinessProjectionFreshnessBoundaryContract(t *testing.T) {
	contract, profile, identity, authorities := readinessProjectionTestAuthority(t)
	validUntil := time.Date(2026, time.July, 19, 8, 30, 0, 123, time.UTC)
	observedAt := validUntil.Add(-time.Duration(profile.MaxAgeSeconds) * time.Second)
	observations := readinessProjectionTestObservations(t, contract, profile, observedAt)
	evaluator := releasecontract.NewEvaluator()

	fresh, err := evaluator.Evaluate(contract, identity, profile, 11, observations, validUntil)
	if err != nil {
		t.Fatalf("evaluate at exact validUntil: %v", err)
	}
	if fresh.ValidUntil.UnixNano() != validUntil.UnixNano() {
		t.Fatalf("validUntil = %s, want %s", fresh.ValidUntil, validUntil)
	}
	assertReadinessProjectionResponse(t, fresh, authorities, http.StatusOK, "")

	_, err = evaluator.Evaluate(contract, identity, profile, 11, observations, validUntil.Add(time.Nanosecond))
	if !releasecontract.IsReadinessCode(err, releasecontract.CodeReadinessStale) {
		t.Fatalf("evaluate at validUntil + 1ns error = %T %v, want %q", err, err, releasecontract.CodeReadinessStale)
	}
	stale := fresh
	stale.ErrorCode = releasecontract.CodeReadinessStale
	assertReadinessProjectionResponse(t, stale, authorities, http.StatusServiceUnavailable, releasecontract.CodeReadinessStale)
}

func readinessProjectionTestAuthority(t *testing.T) (releasecontract.AuthoredContractV1, releasecontract.DeploymentProfile, releasecontract.BuildIdentityV1, releasecontract.RuntimeAuthorities) {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", ".."))
	contract, err := releasecontract.Load(context.Background(), repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json")
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}
	profile, err := releasecontract.NewFileProfileResolver().ResolveCommittedProfile(context.Background(), repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith")
	if err != nil {
		t.Fatalf("resolve profile: %v", err)
	}
	digest, err := releasecontract.Digest(contract)
	if err != nil {
		t.Fatalf("digest contract: %v", err)
	}
	identity := releasecontract.BuildIdentityV1{
		SchemaVersion: "build-identity/v1", ReleaseCommit: strings.Repeat("a", 40), SourceTree: strings.Repeat("b", 40),
		ContractDigest: digest, EvidenceClass: "repository-local",
	}
	authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, controlPlaneGuard{})
	if err != nil {
		t.Fatalf("authorities: %v", err)
	}
	return contract, profile, identity, authorities
}

func readinessProjectionTestObservations(t *testing.T, contract releasecontract.AuthoredContractV1, profile releasecontract.DeploymentProfile, observedAt time.Time) []releasecontract.Observation {
	t.Helper()
	commitments := make(map[string]releasecontract.Commitment, len(contract.Capabilities))
	for _, capability := range contract.Capabilities {
		commitments[capability.ID] = capability.Commitment
	}
	for _, override := range profile.CapabilityOverrides {
		commitments[override.CapabilityID] = override.Commitment
	}
	requirements := make(map[string]releasecontract.ReadinessRequirement, len(contract.ReadinessRequirements))
	for _, requirement := range contract.ReadinessRequirements {
		requirements[requirement.ID] = requirement
	}
	dependencyCapabilities := map[string]map[string]struct{}{}
	for _, requirementID := range profile.ReadinessRequirementIDs {
		requirement, ok := requirements[requirementID]
		if !ok {
			t.Fatalf("profile references unknown readiness requirement %q", requirementID)
		}
		for _, capabilityID := range requirement.CapabilityIDs {
			if commitments[capabilityID] == releasecontract.CommitmentExcluded {
				continue
			}
			for _, dependencyID := range requirement.DependencyIDs {
				if dependencyCapabilities[dependencyID] == nil {
					dependencyCapabilities[dependencyID] = map[string]struct{}{}
				}
				dependencyCapabilities[dependencyID][capabilityID] = struct{}{}
			}
		}
	}
	dependencyIDs := make([]string, 0, len(dependencyCapabilities))
	for dependencyID := range dependencyCapabilities {
		dependencyIDs = append(dependencyIDs, dependencyID)
	}
	sort.Strings(dependencyIDs)
	observations := make([]releasecontract.Observation, 0, len(dependencyIDs))
	for _, dependencyID := range dependencyIDs {
		capabilityIDs := make([]string, 0, len(dependencyCapabilities[dependencyID]))
		for capabilityID := range dependencyCapabilities[dependencyID] {
			capabilityIDs = append(capabilityIDs, capabilityID)
		}
		sort.Strings(capabilityIDs)
		observations = append(observations, releasecontract.Observation{
			ProbeID: "probe." + dependencyID, DependencyID: dependencyID, CapabilityIDs: capabilityIDs,
			Availability: releasecontract.AvailabilityEnabled, ObservedAt: observedAt,
		})
	}
	return observations
}

func assertReadinessProjectionResponse(t *testing.T, evaluation releasecontract.Evaluation, authorities releasecontract.RuntimeAuthorities, wantStatus int, wantCode releasecontract.ReadinessCode) {
	t.Helper()
	handlers := NewReadinessHandlers(ReadinessHandlerOptions{Readiness: &controlPlaneReadinessManager{evaluation: evaluation}, Authorities: authorities})
	for _, endpoint := range []struct {
		name string
		path string
		call func(http.ResponseWriter, *http.Request)
	}{
		{name: "admin", path: "/api/v1/admin/readiness", call: handlers.Admin},
		{name: "app", path: "/api/v1/app/readiness/capabilities", call: handlers.App},
	} {
		t.Run(endpoint.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			endpoint.call(response, httptest.NewRequest(http.MethodGet, endpoint.path, nil))
			body := response.Body.String()
			if response.Code != wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, wantStatus, body)
			}
			if wantCode == "" {
				if !strings.Contains(body, `"enabled":true`) {
					t.Fatalf("fresh body has no enabled capability: %s", body)
				}
				return
			}
			if !strings.Contains(body, string(wantCode)) {
				t.Fatalf("body = %s, want stable code %q", body, wantCode)
			}
			if strings.Contains(body, `"enabled":true`) {
				t.Fatalf("stale body exposed enabled capability: %s", body)
			}
		})
	}
}
