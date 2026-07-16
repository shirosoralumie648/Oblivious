package surfacereport

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/releasecontract"
)

type testDetails struct {
	Observed string `json:"observed"`
}

func TestSurfaceReportV1NestedValidation(t *testing.T) {
	registry := testRegistry(t)
	report := validTestReport(t, registry)
	if err := Validate(report, registry); err != nil {
		t.Fatalf("validate report: %v", err)
	}
	encoded, err := Marshal(report, registry)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	decoded, err := Decode(encoded, registry)
	if err != nil {
		t.Fatalf("strict decode report: %v", err)
	}
	if decoded.ReleaseIdentity != report.ReleaseIdentity || decoded.SurfaceIdentity != report.SurfaceIdentity {
		t.Fatalf("decoded identities = %#v / %#v", decoded.ReleaseIdentity, decoded.SurfaceIdentity)
	}

	report.Outcome.SkippedChecks = []string{"database"}
	if err := Validate(report, registry); !IsCode(err, ErrorSurfaceSchemaInvalid) {
		t.Fatalf("pass with skip error = %v", err)
	}
	report = validTestReport(t, registry)
	report.Drift.Missing = nil
	if err := Validate(report, registry); !IsCode(err, ErrorSurfaceSchemaInvalid) {
		t.Fatalf("missing collection error = %v", err)
	}
}

func TestSurfaceReportV1RejectsFlatAndMisplacedFields(t *testing.T) {
	registry := testRegistry(t)
	base, err := Marshal(validTestReport(t, registry), registry)
	if err != nil {
		t.Fatalf("marshal baseline: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "flat identity", mutate: func(value map[string]any) { value["releaseCommit"] = strings.Repeat("a", 40) }},
		{name: "errors under evidence", mutate: func(value map[string]any) { value["evidence"].(map[string]any)["errorCodes"] = []any{} }},
		{name: "environment under drift", mutate: func(value map[string]any) { value["drift"].(map[string]any)["environment"] = "local" }},
		{name: "unknown nested", mutate: func(value map[string]any) { value["surfaceIdentity"].(map[string]any)["readiness"] = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var value map[string]any
			if err := json.Unmarshal(base, &value); err != nil {
				t.Fatalf("decode baseline: %v", err)
			}
			test.mutate(value)
			mutated, err := json.Marshal(value)
			if err != nil {
				t.Fatalf("marshal mutation: %v", err)
			}
			if _, err := Decode(mutated, registry); !IsCode(err, ErrorSurfaceSchemaInvalid) {
				t.Fatalf("mutation error = %v", err)
			}
		})
	}
}

func TestNewBuildIdentityReportUsesTrustedIdentityAndCommittedProfile(t *testing.T) {
	identity := validBuildIdentity()
	identities := &staticIdentityProvider{identity: identity}
	profiles := &staticProfileResolver{profile: releasecontract.DeploymentProfile{ID: "monolith", Commitment: releasecontract.CommitmentCommitted}}
	report, err := NewBuildIdentityReport(context.Background(), identities, profiles, "/repo", "contract.json", "schema.json", "monolith", validBuildDetails(identity), passingOutcome())
	if err != nil {
		t.Fatalf("construct build report: %v", err)
	}
	if report.ReleaseIdentity.ReleaseCommit != identity.ReleaseCommit || report.ReleaseIdentity.DeploymentProfile != profiles.profile.ID {
		t.Fatalf("untrusted report identity = %#v", report.ReleaseIdentity)
	}
	if identities.calls != 1 || profiles.calls != 1 || identities.paths != profiles.paths {
		t.Fatalf("resolver calls/paths = %d/%d %#v/%#v", identities.calls, profiles.calls, identities.paths, profiles.paths)
	}
}

func TestNewBuildIdentityReportRejectsUnknownExcludedAndConditionalProfile(t *testing.T) {
	identity := validBuildIdentity()
	for _, test := range []struct {
		name        string
		requested   string
		profile     releasecontract.DeploymentProfile
		resolverErr error
	}{
		{name: "unknown substitution", requested: "unknown", profile: releasecontract.DeploymentProfile{ID: "monolith", Commitment: releasecontract.CommitmentCommitted}},
		{name: "excluded", requested: "dual", profile: releasecontract.DeploymentProfile{ID: "dual", Commitment: releasecontract.CommitmentExcluded}},
		{name: "conditional", requested: "candidate", profile: releasecontract.DeploymentProfile{ID: "candidate", Commitment: releasecontract.CommitmentConditional}},
		{name: "resolver rejection", requested: "missing", resolverErr: errors.New("profile_unknown")},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewBuildIdentityReport(context.Background(), &staticIdentityProvider{identity: identity}, &staticProfileResolver{profile: test.profile, err: test.resolverErr}, "/repo", "contract.json", "schema.json", test.requested, validBuildDetails(identity), passingOutcome())
			if err == nil {
				t.Fatal("untrusted profile constructed report")
			}
		})
	}
}

func TestBuildIdentityReportRejectsCallerIdentityAndMismatch(t *testing.T) {
	identity := validBuildIdentity()
	details := validBuildDetails(identity)
	details.Binaries[0].Identity.ReleaseCommit = strings.Repeat("f", 40)
	_, err := NewBuildIdentityReport(context.Background(), &staticIdentityProvider{identity: identity}, &staticProfileResolver{profile: releasecontract.DeploymentProfile{ID: "monolith", Commitment: releasecontract.CommitmentCommitted}}, "/repo", "contract.json", "schema.json", "monolith", details, passingOutcome())
	if !IsCode(err, ErrorSurfaceSchemaInvalid) {
		t.Fatalf("component mismatch error = %v", err)
	}

	registry := NewDetailsRegistry()
	report := validTestReport(t, testRegistry(t))
	report.SurfaceIdentity.Surface = BuildIdentitySurfaceID
	report.Evidence.Details = []byte(`{"releaseIdentity":{"releaseCommit":"forged"}}`)
	if err := Validate(report, registry); !IsCode(err, ErrorSurfaceSchemaInvalid) {
		t.Fatalf("caller identity details error = %v", err)
	}
}

type resolverPaths struct {
	repo, contract, schema string
}

type staticIdentityProvider struct {
	identity buildinfo.BuildIdentityV1
	err      error
	calls    int
	paths    resolverPaths
}

func (p *staticIdentityProvider) Resolve(_ context.Context, repo, contract, schema string) (buildinfo.BuildIdentityV1, error) {
	p.calls++
	p.paths = resolverPaths{repo: repo, contract: contract, schema: schema}
	return p.identity, p.err
}

type staticProfileResolver struct {
	profile releasecontract.DeploymentProfile
	err     error
	calls   int
	paths   resolverPaths
}

func (r *staticProfileResolver) ResolveCommittedProfile(_ context.Context, repo, contract, schema, _ string) (releasecontract.DeploymentProfile, error) {
	r.calls++
	r.paths = resolverPaths{repo: repo, contract: contract, schema: schema}
	return r.profile, r.err
}

func validBuildIdentity() buildinfo.BuildIdentityV1 {
	return buildinfo.BuildIdentityV1{
		SchemaVersion: buildinfo.BuildIdentitySchemaV1, ReleaseCommit: strings.Repeat("a", 40),
		SourceTree: strings.Repeat("b", 40), ContractDigest: "sha256:" + strings.Repeat("c", 64),
		Dirty: false, EvidenceClass: buildinfo.EvidenceRepositoryLocal,
	}
}

func validBuildDetails(identity buildinfo.BuildIdentityV1) BuildIdentityDetails {
	digest := "sha256:" + strings.Repeat("d", 64)
	binaries := make([]BinaryInspection, 0, len(activeBinaryNames))
	for _, name := range activeBinaryNames {
		binaries = append(binaries, BinaryInspection{Name: name, Path: "/app/" + name, Digest: digest, Identity: identity, Matches: true})
	}
	return BuildIdentityDetails{
		Binaries:         binaries,
		OCI:              OCIInspection{Image: "oblivious:test", Digest: digest, Identity: identity, Matches: true},
		PackagedContract: PackagedContractInspection{Path: "/app/config/release/contract.v1.json", Digest: identity.ContractDigest, Identity: identity, Matches: true},
		ResidualRisks:    []string{"external target not inspected"},
	}
}

func passingOutcome() Outcome {
	return Outcome{Result: ResultPass, ErrorCodes: []string{}, SkippedChecks: []string{}}
}

func testRegistry(t *testing.T) *DetailsRegistry {
	t.Helper()
	registry := NewDetailsRegistry()
	if err := RegisterDetails(registry, "test-surface", func(details testDetails) error {
		if details.Observed == "" {
			return reportError("observed", nil)
		}
		return nil
	}); err != nil {
		t.Fatalf("register test details: %v", err)
	}
	return registry
}

func validTestReport(t *testing.T, registry *DetailsRegistry) SurfaceReportV1 {
	t.Helper()
	details, err := registry.MarshalDetails("test-surface", testDetails{Observed: "matched"})
	if err != nil {
		t.Fatalf("marshal details: %v", err)
	}
	return NewReport(
		ReleaseIdentity{
			ReleaseCommit: strings.Repeat("a", 40), SourceTree: strings.Repeat("b", 40),
			ContractDigest: "sha256:" + strings.Repeat("c", 64), DeploymentProfile: "monolith",
			Dirty: false, EvidenceClass: "repository-local",
		},
		SurfaceIdentity{
			Surface: "test-surface", CanonicalSource: "canonical.json", Consumer: "test-consumer",
			Version: "v1", SourceDigest: "sha256:" + strings.Repeat("d", 64), ConsumerDigest: "sha256:" + strings.Repeat("d", 64),
		},
		Drift{Missing: []string{}, Extra: []string{}, Incompatible: []string{}},
		Evidence{Class: "repository-local", Environment: "test", Mode: "fixture", CheckedAt: "2026-07-16T00:00:00Z", ToolVersions: map[string]string{"go": "1.25"}, Details: details},
		Outcome{Result: ResultPass, ErrorCodes: []string{}, SkippedChecks: []string{}},
	)
}
