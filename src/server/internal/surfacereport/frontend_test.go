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
	"oblivious/server/internal/releasecontract"
)

func TestFrontendSurfaceReportContracts(t *testing.T) {
	repoRoot, profile, identity := frontendReportAuthority(t)
	identities := frontendIdentityProvider{identity: identity}
	profiles := frontendProfileResolver{profile: profile}
	transport := validFrontendTransportDetails()
	exposure := validFrontendExposureDetails()
	passing := Outcome{Result: ResultPass, ErrorCodes: []string{}, SkippedChecks: []string{}}

	t.Run("constructs two distinct reports from trusted identity", func(t *testing.T) {
		if got := reflect.TypeOf(NewFrontendTransportReport).NumIn(); got != 9 {
			t.Fatalf("transport constructor input count = %d, want 9", got)
		}
		if got := reflect.TypeOf(NewFrontendExposureReport).NumIn(); got != 9 {
			t.Fatalf("exposure constructor input count = %d, want 9", got)
		}
		transportReport, err := NewFrontendTransportReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", transport, passing)
		if err != nil {
			t.Fatalf("construct transport report: %v", err)
		}
		exposureReport, err := NewFrontendExposureReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", exposure, passing)
		if err != nil {
			t.Fatalf("construct exposure report: %v", err)
		}
		if transportReport.SurfaceIdentity.Surface != FrontendTransportSurfaceID || exposureReport.SurfaceIdentity.Surface != FrontendExposureSurfaceID {
			t.Fatalf("surface ids folded: %#v %#v", transportReport.SurfaceIdentity, exposureReport.SurfaceIdentity)
		}
		if transportReport.SurfaceIdentity.SourceDigest != exposureReport.SurfaceIdentity.SourceDigest || transportReport.Evidence.Details == nil || exposureReport.Evidence.Details == nil {
			t.Fatalf("shared sidecar/source digest missing: %#v %#v", transportReport.SurfaceIdentity, exposureReport.SurfaceIdentity)
		}
		if err := ValidateFrontendSurfacePair(transport, exposure); err != nil {
			t.Fatalf("valid frontend digest pair rejected: %v", err)
		}
		if err := Validate(transportReport, NewDetailsRegistry()); err != nil {
			t.Fatalf("validate transport report: %v", err)
		}
		if err := Validate(exposureReport, NewDetailsRegistry()); err != nil {
			t.Fatalf("validate exposure report: %v", err)
		}
	})

	t.Run("registers closed details schemas", func(t *testing.T) {
		registry := NewDetailsRegistry()
		transportRaw, err := registry.MarshalDetails(FrontendTransportSurfaceID, transport)
		if err != nil {
			t.Fatalf("marshal transport details: %v", err)
		}
		if err := registry.ValidateDetails(FrontendTransportSurfaceID, transportRaw); err != nil {
			t.Fatalf("validate transport details: %v", err)
		}
		exposureRaw, err := registry.MarshalDetails(FrontendExposureSurfaceID, exposure)
		if err != nil {
			t.Fatalf("marshal exposure details: %v", err)
		}
		if err := registry.ValidateDetails(FrontendExposureSurfaceID, exposureRaw); err != nil {
			t.Fatalf("validate exposure details: %v", err)
		}
		for _, raw := range []struct {
			name string
			id   string
			data json.RawMessage
		}{
			{"transport unknown field", FrontendTransportSurfaceID, appendJSONField(transportRaw, `"releaseCommit":"forged"`)},
			{"exposure unknown field", FrontendExposureSurfaceID, appendJSONField(exposureRaw, `"releaseCommit":"forged"`)},
		} {
			t.Run(raw.name, func(t *testing.T) {
				if err := registry.ValidateDetails(raw.id, raw.data); !IsCode(err, ErrorSurfaceSchemaInvalid) {
					t.Fatalf("unknown details accepted: %v", err)
				}
			})
		}
	})

	t.Run("rejects zero, digest, unresolved, skip and claim mutations", func(t *testing.T) {
		transportMutations := []struct {
			name string
			edit func(*FrontendTransportDetails)
		}{
			{"zero operations", func(v *FrontendTransportDetails) { v.OperationCount = 0 }},
			{"core count drift", func(v *FrontendTransportDetails) { v.CoreCount-- }},
			{"compatible count drift", func(v *FrontendTransportDetails) { v.CompatibleCount-- }},
			{"unresolved", func(v *FrontendTransportDetails) { v.UnresolvedCount = 1 }},
			{"taxonomy digest", func(v *FrontendTransportDetails) { v.TaxonomyDigest = "sha256:bad" }},
		}
		for _, mutation := range transportMutations {
			t.Run("transport "+mutation.name, func(t *testing.T) {
				candidate := transport
				mutation.edit(&candidate)
				if _, err := NewFrontendTransportReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", candidate, passing); err == nil {
					t.Fatalf("transport mutation accepted: %s", mutation.name)
				}
			})
		}
		exposureMutations := []struct {
			name string
			edit func(*FrontendExposureDetails)
		}{
			{"zero exposures", func(v *FrontendExposureDetails) { v.ExposureCount = 0 }},
			{"zero catalogs", func(v *FrontendExposureDetails) { v.CatalogCount = 0 }},
			{"zero navigation", func(v *FrontendExposureDetails) { v.NavigationCount = 0 }},
			{"zero generated consumers", func(v *FrontendExposureDetails) { v.GeneratedConsumerCount = 0 }},
			{"unresolved", func(v *FrontendExposureDetails) { v.UnresolvedCount = 1 }},
			{"projection digest", func(v *FrontendExposureDetails) { v.ProjectionDigest = "sha256:bad" }},
		}
		for _, mutation := range exposureMutations {
			t.Run("exposure "+mutation.name, func(t *testing.T) {
				candidate := exposure
				mutation.edit(&candidate)
				if _, err := NewFrontendExposureReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", candidate, passing); err == nil {
					t.Fatalf("exposure mutation accepted: %s", mutation.name)
				}
			})
		}
		withSkip := passing
		withSkip.SkippedChecks = []string{"frontend"}
		if _, err := NewFrontendTransportReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", transport, withSkip); err == nil {
			t.Fatal("frontend transport accepted committed skip")
		}
		for _, evidenceClass := range []string{"target-environment", "same-commit-release"} {
			forged := identity
			forged.EvidenceClass = evidenceClass
			if _, err := NewFrontendTransportReport(context.Background(), frontendIdentityProvider{identity: forged}, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", transport, passing); err == nil {
				t.Fatalf("frontend transport accepted evidence class %q", evidenceClass)
			}
		}
	})

	t.Run("validated reports reject surface and digest substitution", func(t *testing.T) {
		report, err := NewFrontendTransportReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", transport, passing)
		if err != nil {
			t.Fatalf("construct transport mutation base: %v", err)
		}
		mutations := []struct {
			name string
			edit func(*SurfaceReportV1)
		}{
			{"canonical source", func(value *SurfaceReportV1) { value.SurfaceIdentity.CanonicalSource = "src/web" }},
			{"consumer", func(value *SurfaceReportV1) { value.SurfaceIdentity.Consumer = "frontend-verifier" }},
			{"version", func(value *SurfaceReportV1) { value.SurfaceIdentity.Version = "v2" }},
			{"source digest", func(value *SurfaceReportV1) { value.SurfaceIdentity.SourceDigest = "sha256:" + strings.Repeat("f", 64) }},
			{"consumer digest", func(value *SurfaceReportV1) {
				value.SurfaceIdentity.ConsumerDigest = "sha256:" + strings.Repeat("f", 64)
			}},
		}
		for _, mutation := range mutations {
			candidate := report
			mutation.edit(&candidate)
			if err := Validate(candidate, NewDetailsRegistry()); err == nil {
				t.Fatalf("frontend report mutation accepted: %s", mutation.name)
			}
		}
	})

	t.Run("pair rejects digest splices without folding details", func(t *testing.T) {
		mutations := []struct {
			name string
			edit func(*FrontendExposureDetails)
		}{
			{"sidecar", func(value *FrontendExposureDetails) { value.SidecarDigest = "sha256:" + strings.Repeat("c", 64) }},
			{"source", func(value *FrontendExposureDetails) { value.SourceDigest = "sha256:" + strings.Repeat("d", 64) }},
			{"config", func(value *FrontendExposureDetails) { value.ConfigDigest = "sha256:" + strings.Repeat("e", 64) }},
		}
		for _, mutation := range mutations {
			candidate := exposure
			mutation.edit(&candidate)
			if err := ValidateFrontendSurfacePair(transport, candidate); err == nil {
				t.Fatalf("mismatched frontend %s digest pair accepted", mutation.name)
			}
		}
	})
}

func validFrontendTransportDetails() FrontendTransportDetails {
	return FrontendTransportDetails{
		SchemaVersion: "frontend-transport-observation/v1",
		SidecarDigest: "sha256:" + strings.Repeat("1", 64), SourceDigest: "sha256:" + strings.Repeat("2", 64), ConfigDigest: "sha256:" + strings.Repeat("3", 64),
		OperationCount: 264, CoreCount: 264, CompatibleCount: 264, TaxonomyDigest: "sha256:" + strings.Repeat("4", 64), UnresolvedCount: 0,
		ErrorCodes: []string{}, SkippedChecks: []string{},
	}
}

func validFrontendExposureDetails() FrontendExposureDetails {
	return FrontendExposureDetails{
		SchemaVersion: "frontend-exposure-observation/v1",
		SidecarDigest: "sha256:" + strings.Repeat("1", 64), SourceDigest: "sha256:" + strings.Repeat("2", 64), ConfigDigest: "sha256:" + strings.Repeat("3", 64),
		ExposureCount: 91, CatalogCount: 1, NavigationCount: 1, GeneratedConsumerCount: 374, ProjectionDigest: "sha256:" + strings.Repeat("5", 64), UnresolvedCount: 0,
		ErrorCodes: []string{}, SkippedChecks: []string{},
	}
}

func appendJSONField(raw json.RawMessage, field string) json.RawMessage {
	return append(append([]byte(nil), raw[:len(raw)-1]...), []byte(","+field+"}")...)
}

func frontendReportAuthority(t *testing.T) (string, releasecontract.DeploymentProfile, buildinfo.BuildIdentityV1) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve frontend report source")
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
		SchemaVersion: buildinfo.BuildIdentitySchemaV1, ReleaseCommit: strings.Repeat("a", 40), SourceTree: strings.Repeat("b", 40), ContractDigest: digest,
		Dirty: false, EvidenceClass: buildinfo.EvidenceRepositoryLocal,
	}
}

type frontendIdentityProvider struct {
	identity buildinfo.BuildIdentityV1
	err      error
}

func (p frontendIdentityProvider) Resolve(context.Context, string, string, string) (buildinfo.BuildIdentityV1, error) {
	return p.identity, p.err
}

type frontendProfileResolver struct {
	profile releasecontract.DeploymentProfile
	err     error
}

func (r frontendProfileResolver) ResolveCommittedProfile(context.Context, string, string, string, string) (releasecontract.DeploymentProfile, error) {
	return r.profile, r.err
}
