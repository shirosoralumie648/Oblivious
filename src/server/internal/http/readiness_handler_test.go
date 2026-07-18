package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
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
