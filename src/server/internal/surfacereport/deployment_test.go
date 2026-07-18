package surfacereport

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/releasecontract"
)

func TestDeploymentSurfaceContract(t *testing.T) {
	repoRoot, profile, identity := deploymentReportAuthority(t)
	identities := deploymentIdentityProvider{identity: identity}
	profiles := deploymentProfileResolver{profile: profile}
	passing := Outcome{Result: ResultPass, ErrorCodes: []string{}, SkippedChecks: []string{}}
	details := DeploymentDetails{
		Profile:           "monolith",
		CanonicalWorkload: "deploy/kubernetes/app-deployment.yaml",
		StartupEndpoint:   "/livez",
		LivenessEndpoint:  "/livez",
		ReadinessEndpoint: "/readyz",
		AuditStorage:      "emptyDir",
		MigrationState:    "applied_and_validated",
		HarnessResult:     "passed",
	}

	t.Run("accepts the exact typed deployment details", func(t *testing.T) {
		if got := len([]string{details.Profile, details.CanonicalWorkload, details.StartupEndpoint, details.LivenessEndpoint, details.ReadinessEndpoint, details.AuditStorage, details.MigrationState, details.HarnessResult}); got != 8 {
			t.Fatalf("deployment details field count = %d, want 8", got)
		}
		report, err := NewDeploymentReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", details, passing)
		if err != nil {
			t.Fatalf("construct deployment report: %v", err)
		}
		if report.SurfaceIdentity.Surface != DeploymentSurfaceID || report.ReleaseIdentity.DeploymentProfile != "monolith" {
			t.Fatalf("report identity = %#v / %#v", report.SurfaceIdentity, report.ReleaseIdentity)
		}
		if report.SurfaceIdentity.Surface == ReadinessSurfaceID {
			t.Fatal("deployment report folded into readiness surface")
		}
		var decoded map[string]any
		if err := json.Unmarshal(report.Evidence.Details, &decoded); err != nil {
			t.Fatalf("decode deployment details: %v", err)
		}
		for _, forbidden := range []string{"generation", "checkedAt", "validUntil", "readinessGeneration"} {
			if _, ok := decoded[forbidden]; ok {
				t.Fatalf("deployment details contain readiness field %q: %#v", forbidden, decoded)
			}
		}
		if err := Validate(report, NewDetailsRegistry()); err != nil {
			t.Fatalf("validate deployment report: %v", err)
		}
	})

	t.Run("registry rejects unknown fields and arbitrary maps", func(t *testing.T) {
		registry := NewDetailsRegistry()
		raw, err := registry.MarshalDetails(DeploymentSurfaceID, details)
		if err != nil {
			t.Fatalf("marshal details: %v", err)
		}
		if err := registry.ValidateDetails(DeploymentSurfaceID, raw); err != nil {
			t.Fatalf("validate details: %v", err)
		}
		unknown := append([]byte(nil), raw[:len(raw)-1]...)
		unknown = append(unknown, []byte(`,"generation":1}`)...)
		if err := registry.ValidateDetails(DeploymentSurfaceID, unknown); !IsCode(err, ErrorSurfaceSchemaInvalid) {
			t.Fatalf("unknown deployment field error = %v", err)
		}
		if _, err := registry.MarshalDetails(DeploymentSurfaceID, map[string]any{"profile": "monolith"}); !IsCode(err, ErrorSurfaceSchemaInvalid) {
			t.Fatalf("arbitrary deployment map error = %v", err)
		}
	})

	t.Run("rejects profile, probe, workload, result, identity and claim mutations", func(t *testing.T) {
		mutations := []struct {
			name string
			edit func(*DeploymentDetails)
		}{
			{"profile", func(value *DeploymentDetails) { value.Profile = "microservices" }},
			{"canonical workload", func(value *DeploymentDetails) { value.CanonicalWorkload = "deploy/kubernetes/server.yaml" }},
			{"startup probe", func(value *DeploymentDetails) { value.StartupEndpoint = "/readyz" }},
			{"liveness probe", func(value *DeploymentDetails) { value.LivenessEndpoint = "/readyz" }},
			{"readiness probe", func(value *DeploymentDetails) { value.ReadinessEndpoint = "/livez" }},
			{"audit storage", func(value *DeploymentDetails) { value.AuditStorage = "/var/lib/readiness.json" }},
			{"migration", func(value *DeploymentDetails) { value.MigrationState = "pending" }},
			{"harness", func(value *DeploymentDetails) { value.HarnessResult = "failed" }},
		}
		for _, mutation := range mutations {
			t.Run(mutation.name, func(t *testing.T) {
				candidate := details
				mutation.edit(&candidate)
				if _, err := NewDeploymentReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", candidate, passing); err == nil {
					t.Fatalf("mutation %q was accepted", mutation.name)
				}
			})
		}
		withSkip := passing
		withSkip.SkippedChecks = []string{"docker"}
		if _, err := NewDeploymentReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", details, withSkip); err == nil {
			t.Fatal("deployment report accepted a committed skip")
		}
		for _, claim := range []string{"target-environment", "same-commit-release"} {
			forged := identity
			forged.EvidenceClass = claim
			if _, err := NewDeploymentReport(context.Background(), deploymentIdentityProvider{identity: forged}, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", details, passing); err == nil {
				t.Fatalf("deployment report accepted claim class %q", claim)
			}
		}
		forged := identity
		forged.Dirty = true
		if _, err := NewDeploymentReport(context.Background(), deploymentIdentityProvider{identity: forged}, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", details, passing); err == nil {
			t.Fatal("deployment report accepted dirty identity")
		}
	})
}

type deploymentIdentityProvider struct {
	identity buildinfo.BuildIdentityV1
	err      error
}

func (p deploymentIdentityProvider) Resolve(context.Context, string, string, string) (buildinfo.BuildIdentityV1, error) {
	return p.identity, p.err
}

type deploymentProfileResolver struct {
	profile releasecontract.DeploymentProfile
	err     error
}

func (r deploymentProfileResolver) ResolveCommittedProfile(context.Context, string, string, string, string) (releasecontract.DeploymentProfile, error) {
	return r.profile, r.err
}

func deploymentReportAuthority(t *testing.T) (string, releasecontract.DeploymentProfile, buildinfo.BuildIdentityV1) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve deployment report source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "../../../.."))
	contract, err := releasecontract.Load(context.Background(), repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json")
	if err != nil {
		t.Fatalf("load release contract: %v", err)
	}
	var profile releasecontract.DeploymentProfile
	for _, candidate := range contract.Profiles {
		if candidate.ID == "monolith" {
			profile = candidate
		}
	}
	digest, err := releasecontract.Digest(contract)
	if err != nil {
		t.Fatalf("digest release contract: %v", err)
	}
	return repoRoot, profile, buildinfo.BuildIdentityV1{
		SchemaVersion: buildinfo.BuildIdentitySchemaV1,
		ReleaseCommit: strings.Repeat("a", 40), SourceTree: strings.Repeat("b", 40),
		ContractDigest: digest, Dirty: false, EvidenceClass: buildinfo.EvidenceRepositoryLocal,
	}
}
