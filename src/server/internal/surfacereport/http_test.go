package surfacereport

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"oblivious/server/internal/buildinfo"
	runtimehttp "oblivious/server/internal/http"
	"oblivious/server/internal/releasecontract"
)

func TestHTTPRuntimeSurfaceContract(t *testing.T) {
	repoRoot, profile, identity := httpRuntimeReportAuthority(t)
	identities := httpRuntimeIdentityProvider{identity: identity}
	profiles := httpRuntimeProfileResolver{profile: profile}
	observation := validHTTPRuntimeObservation()

	t.Run("constructs one typed repository local report without output ownership", func(t *testing.T) {
		if got := reflect.TypeOf(NewHTTPRuntimeReport).NumIn(); got != 8 {
			t.Fatalf("NewHTTPRuntimeReport input count = %d, want 8 without identity, writer, output, or outcome inputs", got)
		}
		report, err := NewHTTPRuntimeReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", observation)
		if err != nil {
			t.Fatalf("construct http runtime report: %v", err)
		}
		if report.SurfaceIdentity.Surface != HTTPRuntimeSurfaceID || report.SurfaceIdentity.CanonicalSource != httpRuntimeCanonicalSource || report.SurfaceIdentity.Consumer != httpRuntimeConsumer || report.SurfaceIdentity.Version != httpRuntimeVersion {
			t.Fatalf("unexpected http runtime surface identity: %#v", report.SurfaceIdentity)
		}
		if report.SurfaceIdentity.SourceDigest != observation.CoreDigest || report.SurfaceIdentity.ConsumerDigest != observation.RuntimeDigest {
			t.Fatalf("unexpected http runtime digests: %#v", report.SurfaceIdentity)
		}
		if report.ReleaseIdentity.ReleaseCommit != identity.ReleaseCommit || report.ReleaseIdentity.DeploymentProfile != profile.ID || report.Evidence.Class != buildinfo.EvidenceRepositoryLocal || report.Outcome.Result != ResultPass {
			t.Fatalf("unexpected trusted report envelope: %#v", report)
		}
		if err := Validate(report, NewDetailsRegistry()); err != nil {
			t.Fatalf("validate http runtime report: %v", err)
		}
	})

	t.Run("registry accepts only the exact http runtime details", func(t *testing.T) {
		registry := NewDetailsRegistry()
		details := httpRuntimeDetailsFromObservation(observation)
		raw, err := registry.MarshalDetails(HTTPRuntimeSurfaceID, details)
		if err != nil {
			t.Fatalf("marshal http runtime details: %v", err)
		}
		if err := registry.ValidateDetails(HTTPRuntimeSurfaceID, raw); err != nil {
			t.Fatalf("validate http runtime details: %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("decode http runtime details: %v", err)
		}
		if len(decoded) != 7 {
			t.Fatalf("http runtime details field count = %d, want 7: %#v", len(decoded), decoded)
		}
		unknown := append([]byte(nil), raw[:len(raw)-1]...)
		unknown = append(unknown, []byte(`,"releaseCommit":"forged"}`)...)
		if err := registry.ValidateDetails(HTTPRuntimeSurfaceID, unknown); !IsCode(err, ErrorSurfaceSchemaInvalid) {
			t.Fatalf("unknown http runtime field error = %v", err)
		}
		if _, err := registry.MarshalDetails(HTTPRuntimeSurfaceID, map[string]any{"operationCount": 3}); !IsCode(err, ErrorSurfaceSchemaInvalid) {
			t.Fatalf("arbitrary http runtime details error = %v", err)
		}
		if err := registry.ValidateDetails("http-runtime-shadow", raw); !IsCode(err, ErrorSurfaceSchemaInvalid) {
			t.Fatalf("shadow surface registration error = %v", err)
		}
	})

	t.Run("rejects zero counts count drift digest drift and invalid parity", func(t *testing.T) {
		mutations := []struct {
			name string
			edit func(*runtimehttp.HTTPRuntimeObservation)
		}{
			{"zero operations", func(value *runtimehttp.HTTPRuntimeObservation) { value.OperationCount = 0 }},
			{"zero mounts", func(value *runtimehttp.HTTPRuntimeObservation) { value.MountedCount = 0 }},
			{"zero descriptors", func(value *runtimehttp.HTTPRuntimeObservation) { value.DescriptorCount = 0 }},
			{"zero media probes", func(value *runtimehttp.HTTPRuntimeObservation) { value.MediaProbeCount = 0 }},
			{"mount count drift", func(value *runtimehttp.HTTPRuntimeObservation) { value.MountedCount-- }},
			{"descriptor count drift", func(value *runtimehttp.HTTPRuntimeObservation) { value.DescriptorCount-- }},
			{"invalid core digest", func(value *runtimehttp.HTTPRuntimeObservation) { value.CoreDigest = "sha256:bad" }},
			{"pass digest drift", func(value *runtimehttp.HTTPRuntimeObservation) {
				value.RuntimeDigest = "sha256:" + strings.Repeat("3", 64)
			}},
			{"unknown parity", func(value *runtimehttp.HTTPRuntimeObservation) { value.ParityResult = "unknown" }},
		}
		for _, mutation := range mutations {
			t.Run(mutation.name, func(t *testing.T) {
				candidate := observation
				mutation.edit(&candidate)
				if _, err := NewHTTPRuntimeReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", candidate); err == nil {
					t.Fatalf("http runtime mutation %q was accepted", mutation.name)
				}
			})
		}
	})

	t.Run("mismatches produce a stable failure report and never a pass", func(t *testing.T) {
		failed := observation
		failed.ParityResult = "fail"
		failed.RuntimeDigest = "sha256:" + strings.Repeat("3", 64)
		failed.MismatchIDs = []string{"getAppReadinessCapabilities"}
		report, err := NewHTTPRuntimeReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", failed)
		if err != nil {
			t.Fatalf("construct explicit http runtime failure report: %v", err)
		}
		if report.Outcome.Result != ResultFail || !reflect.DeepEqual(report.Outcome.ErrorCodes, []string{httpRuntimeParityErrorCode}) || !reflect.DeepEqual(report.Drift.Incompatible, failed.MismatchIDs) {
			t.Fatalf("unexpected http runtime failure mapping: drift=%#v outcome=%#v", report.Drift, report.Outcome)
		}
		if err := Validate(report, NewDetailsRegistry()); err != nil {
			t.Fatalf("validate http runtime failure report: %v", err)
		}

		spliced := observation
		spliced.MismatchIDs = failed.MismatchIDs
		if _, err := NewHTTPRuntimeReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", spliced); err == nil {
			t.Fatal("passing http runtime observation accepted nonempty drift")
		}
	})

	t.Run("rejects identity profile skip and surface claim substitution", func(t *testing.T) {
		forgedIdentity := identity
		forgedIdentity.EvidenceClass = "target-environment"
		if _, err := NewHTTPRuntimeReport(context.Background(), httpRuntimeIdentityProvider{identity: forgedIdentity}, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", observation); err == nil {
			t.Fatal("http runtime report accepted E3 claim inflation")
		}
		forgedIdentity.EvidenceClass = "same-commit-release"
		if _, err := NewHTTPRuntimeReport(context.Background(), httpRuntimeIdentityProvider{identity: forgedIdentity}, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", observation); err == nil {
			t.Fatal("http runtime report accepted E4 claim inflation")
		}
		wrongProfile := profile
		wrongProfile.ID = "microservices"
		if _, err := NewHTTPRuntimeReport(context.Background(), identities, httpRuntimeProfileResolver{profile: wrongProfile}, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", observation); err == nil {
			t.Fatal("http runtime report accepted profile substitution")
		}

		report, err := NewHTTPRuntimeReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", observation)
		if err != nil {
			t.Fatalf("construct splice fixture: %v", err)
		}
		report.Outcome.SkippedChecks = []string{"runtime-dispatch"}
		if err := Validate(report, NewDetailsRegistry()); err == nil {
			t.Fatal("http runtime report accepted a skipped check")
		}
		report, _ = NewHTTPRuntimeReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", observation)
		report.SurfaceIdentity.Consumer = "manifest-only"
		if err := Validate(report, NewDetailsRegistry()); err == nil {
			t.Fatal("http runtime report accepted consumer substitution")
		}
	})
}

func validHTTPRuntimeObservation() runtimehttp.HTTPRuntimeObservation {
	digest := "sha256:" + strings.Repeat("1", 64)
	return runtimehttp.HTTPRuntimeObservation{
		OperationCount: 3, MountedCount: 3, DescriptorCount: 3, MediaProbeCount: 2,
		CoreDigest: digest, RuntimeDigest: digest, ParityResult: "pass", MismatchIDs: []string{},
	}
}

type httpRuntimeIdentityProvider struct {
	identity buildinfo.BuildIdentityV1
	err      error
}

func (p httpRuntimeIdentityProvider) Resolve(context.Context, string, string, string) (buildinfo.BuildIdentityV1, error) {
	return p.identity, p.err
}

type httpRuntimeProfileResolver struct {
	profile releasecontract.DeploymentProfile
	err     error
}

func (r httpRuntimeProfileResolver) ResolveCommittedProfile(context.Context, string, string, string, string) (releasecontract.DeploymentProfile, error) {
	return r.profile, r.err
}

func httpRuntimeReportAuthority(t *testing.T) (string, releasecontract.DeploymentProfile, buildinfo.BuildIdentityV1) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve http runtime report source")
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
		ReleaseCommit: strings.Repeat("a", 40), SourceTree: strings.Repeat("b", 40), ContractDigest: digest,
		Dirty: false, EvidenceClass: buildinfo.EvidenceRepositoryLocal,
	}
}
