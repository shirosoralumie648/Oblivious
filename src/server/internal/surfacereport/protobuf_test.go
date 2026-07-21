package surfacereport

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/releasecontract"
)

func TestProtobufSurfaceContract(t *testing.T) {
	repoRoot, profile, identity := protobufReportAuthority(t)
	identities := protobufIdentityProvider{identity: identity}
	profiles := protobufProfileResolver{profile: profile}
	details := validProtobufDetails()
	passing := Outcome{Result: ResultPass, ErrorCodes: []string{}, SkippedChecks: []string{}}

	t.Run("constructs one trusted typed protobuf report", func(t *testing.T) {
		report, err := NewProtobufReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", details, passing)
		if err != nil {
			t.Fatalf("construct protobuf report: %v", err)
		}
		if report.SurfaceIdentity.Surface != ProtobufSurfaceID || report.SurfaceIdentity.CanonicalSource != protobufCanonicalSource || report.SurfaceIdentity.Consumer != protobufConsumer || report.SurfaceIdentity.Version != protobufVersion {
			t.Fatalf("protobuf surface identity = %#v", report.SurfaceIdentity)
		}
		if report.ReleaseIdentity.ReleaseCommit != identity.ReleaseCommit || report.ReleaseIdentity.DeploymentProfile != profile.ID || report.Evidence.Class != buildinfo.EvidenceRepositoryLocal {
			t.Fatalf("protobuf release identity/evidence = %#v / %#v", report.ReleaseIdentity, report.Evidence)
		}
		if report.SurfaceIdentity.SourceDigest != details.ManifestDigest || report.Outcome.Result != ResultPass || len(report.Outcome.SkippedChecks) != 0 {
			t.Fatalf("protobuf report result = %#v / %#v", report.SurfaceIdentity, report.Outcome)
		}
		if err := Validate(report, NewDetailsRegistry()); err != nil {
			t.Fatalf("validate protobuf report: %v", err)
		}
		encoded, err := Marshal(report, NewDetailsRegistry())
		if err != nil {
			t.Fatalf("marshal protobuf report: %v", err)
		}
		for _, prohibited := range []string{`"evidenceClass":"target-environment"`, `"evidenceClass":"same-commit-release"`} {
			if strings.Contains(string(encoded), prohibited) {
				t.Fatalf("protobuf report contains prohibited claim %q", prohibited)
			}
		}
	})

	t.Run("registry is closed and rejects identity input", func(t *testing.T) {
		registry := NewDetailsRegistry()
		raw, err := registry.MarshalDetails(ProtobufSurfaceID, details)
		if err != nil {
			t.Fatalf("marshal protobuf details: %v", err)
		}
		if err := registry.ValidateDetails(ProtobufSurfaceID, raw); err != nil {
			t.Fatalf("validate protobuf details: %v", err)
		}
		unknown := append([]byte(nil), raw[:len(raw)-1]...)
		unknown = append(unknown, []byte(`,"releaseIdentity":{"releaseCommit":"`+strings.Repeat("f", 40)+`"}}`)...)
		if err := registry.ValidateDetails(ProtobufSurfaceID, unknown); !IsCode(err, ErrorSurfaceSchemaInvalid) {
			t.Fatalf("identity-bearing details error = %v", err)
		}
		if _, err := registry.MarshalDetails(ProtobufSurfaceID, map[string]any{"manifestDigest": details.ManifestDigest}); !IsCode(err, ErrorSurfaceSchemaInvalid) {
			t.Fatalf("arbitrary protobuf map error = %v", err)
		}
	})

	t.Run("rejects tool disposition regeneration skip and claim mutations", func(t *testing.T) {
		mutations := []struct {
			name string
			edit func(*ProtobufDetails)
		}{
			{"schema", func(value *ProtobufDetails) { value.SchemaVersion = "protobuf-observation/v2" }},
			{"manifest digest", func(value *ProtobufDetails) { value.ManifestDigest = "sha256:" + strings.Repeat("0", 64) }},
			{"protoc", func(value *ProtobufDetails) { value.ToolVersions.Protoc = "25.2" }},
			{"go plugin", func(value *ProtobufDetails) { value.ToolVersions.ProtocGenGo = "1.36.10" }},
			{"grpc plugin", func(value *ProtobufDetails) { value.ToolVersions.ProtocGenGoGRPC = "1.6.1" }},
			{"generated header", func(value *ProtobufDetails) { value.GeneratedHeaderVersion = "v4.25.2" }},
			{"zero source count", func(value *ProtobufDetails) { value.SourceCount = 0 }},
			{"output drift", func(value *ProtobufDetails) { value.OutputCount++ }},
			{"managed drift", func(value *ProtobufDetails) { value.ManagedCount-- }},
			{"source-only drift", func(value *ProtobufDetails) { value.SourceOnlyCount++ }},
			{"failed regeneration", func(value *ProtobufDetails) { value.Regeneration.Result = "fail" }},
			{"generated count", func(value *ProtobufDetails) { value.Regeneration.GeneratedCount = 20 }},
			{"regeneration digest", func(value *ProtobufDetails) { value.Regeneration.Digest = "sha256:" + strings.Repeat("1", 64) }},
			{"validator error", func(value *ProtobufDetails) { value.ErrorCodes = []string{"protobuf_generated_byte_drift"} }},
			{"validator skip", func(value *ProtobufDetails) { value.SkippedChecks = []string{"regeneration"} }},
		}
		for _, mutation := range mutations {
			t.Run(mutation.name, func(t *testing.T) {
				candidate := details
				candidate.ErrorCodes = append([]string(nil), details.ErrorCodes...)
				candidate.SkippedChecks = append([]string(nil), details.SkippedChecks...)
				mutation.edit(&candidate)
				if _, err := NewProtobufReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", candidate, passing); err == nil {
					t.Fatalf("protobuf mutation %q was accepted", mutation.name)
				}
			})
		}

		withSkip := passing
		withSkip.SkippedChecks = []string{"target-runtime"}
		if _, err := NewProtobufReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", details, withSkip); err == nil {
			t.Fatal("protobuf report accepted a committed skip")
		}
		for _, evidenceClass := range []string{"target-environment", "same-commit-release"} {
			forged := identity
			forged.EvidenceClass = evidenceClass
			if _, err := NewProtobufReport(context.Background(), protobufIdentityProvider{identity: forged}, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", details, passing); err == nil {
				t.Fatalf("protobuf report accepted claim class %q", evidenceClass)
			}
		}
	})

	t.Run("rejects profile and decoded report splices", func(t *testing.T) {
		for _, resolver := range []protobufProfileResolver{
			{profile: releasecontract.DeploymentProfile{ID: "monolith", Commitment: releasecontract.CommitmentConditional}},
			{profile: releasecontract.DeploymentProfile{ID: "microservices", Commitment: releasecontract.CommitmentCommitted}},
		} {
			if _, err := NewProtobufReport(context.Background(), identities, resolver, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", details, passing); err == nil {
				t.Fatalf("protobuf report accepted profile %#v", resolver.profile)
			}
		}
		report, err := NewProtobufReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", details, passing)
		if err != nil {
			t.Fatalf("construct splice fixture: %v", err)
		}
		mutations := []struct {
			name string
			edit func(*SurfaceReportV1)
		}{
			{"canonical source", func(value *SurfaceReportV1) { value.SurfaceIdentity.CanonicalSource = "api/proto" }},
			{"consumer", func(value *SurfaceReportV1) { value.SurfaceIdentity.Consumer = "generated-files" }},
			{"version", func(value *SurfaceReportV1) { value.SurfaceIdentity.Version = "v2" }},
			{"source digest", func(value *SurfaceReportV1) { value.SurfaceIdentity.SourceDigest = "sha256:" + strings.Repeat("1", 64) }},
			{"consumer digest", func(value *SurfaceReportV1) {
				value.SurfaceIdentity.ConsumerDigest = "sha256:" + strings.Repeat("2", 64)
			}},
			{"environment", func(value *SurfaceReportV1) { value.Evidence.Environment = "target" }},
			{"mode", func(value *SurfaceReportV1) { value.Evidence.Mode = "manifest-only" }},
			{"tool evidence", func(value *SurfaceReportV1) { value.Evidence.ToolVersions = map[string]string{"protoc": "25.2"} }},
		}
		for _, mutation := range mutations {
			t.Run(mutation.name, func(t *testing.T) {
				candidate := report
				candidate.Evidence.ToolVersions = protobufEvidenceToolVersions(details.ToolVersions)
				mutation.edit(&candidate)
				if err := Validate(candidate, NewDetailsRegistry()); err == nil {
					t.Fatalf("decoded protobuf splice %q was accepted", mutation.name)
				}
			})
		}
	})
}

func validProtobufDetails() ProtobufDetails {
	return ProtobufDetails{
		SchemaVersion:  protobufObservationSchemaV1,
		ManifestDigest: protobufManifestDigest,
		ToolVersions: ProtobufToolVersions{
			Protoc: protobufProtocVersion, ProtocGenGo: protobufGenGoVersion,
			ProtocGenGoGRPC: protobufGenGoGRPCVersion,
		},
		GeneratedHeaderVersion: protobufGeneratedHeaderVersion,
		SourceCount:            10, OutputCount: 21, ManagedCount: 9, SourceOnlyCount: 1,
		Regeneration: ProtobufRegeneration{Result: "pass", GeneratedCount: 21, Digest: protobufRegenerationDigest},
		ErrorCodes:   []string{}, SkippedChecks: []string{},
	}
}

type protobufIdentityProvider struct {
	identity buildinfo.BuildIdentityV1
	err      error
}

func (p protobufIdentityProvider) Resolve(context.Context, string, string, string) (buildinfo.BuildIdentityV1, error) {
	return p.identity, p.err
}

type protobufProfileResolver struct {
	profile releasecontract.DeploymentProfile
	err     error
}

func (r protobufProfileResolver) ResolveCommittedProfile(context.Context, string, string, string, string) (releasecontract.DeploymentProfile, error) {
	return r.profile, r.err
}

func protobufReportAuthority(t *testing.T) (string, releasecontract.DeploymentProfile, buildinfo.BuildIdentityV1) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve protobuf report source")
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
