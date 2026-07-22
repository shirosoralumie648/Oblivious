package surfacereport

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/releasecontract"
)

const (
	FrontendTransportSurfaceID = "frontend-transport"
	FrontendExposureSurfaceID  = "frontend-exposure"

	frontendTransportObservationSchema = "frontend-transport-observation/v1"
	frontendExposureObservationSchema  = "frontend-exposure-observation/v1"
	frontendCanonicalSource            = "src/web/src"
	frontendTransportConsumer          = "frontend-transport-verifier"
	frontendExposureConsumer           = "product-exposure-verifier"
	frontendSurfaceVersion             = "v1"
	frontendSurfaceEvidenceMode        = "compiler-sidecar"
)

// FrontendTransportDetails is the closed, repository-local projection of one
// compiler sidecar into the transport/core parity surface.
type FrontendTransportDetails struct {
	SchemaVersion   string   `json:"schemaVersion"`
	SidecarDigest   string   `json:"sidecarDigest"`
	SourceDigest    string   `json:"sourceDigest"`
	ConfigDigest    string   `json:"configDigest"`
	OperationCount  int      `json:"operationCount"`
	CoreCount       int      `json:"coreCount"`
	CompatibleCount int      `json:"compatibleCount"`
	TaxonomyDigest  string   `json:"taxonomyDigest"`
	UnresolvedCount int      `json:"unresolvedCount"`
	ErrorCodes      []string `json:"errorCodes"`
	SkippedChecks   []string `json:"skippedChecks"`
}

// FrontendExposureDetails is the separate product/navigation projection of
// the same sidecar. It deliberately does not reuse transport counts.
type FrontendExposureDetails struct {
	SchemaVersion          string   `json:"schemaVersion"`
	SidecarDigest          string   `json:"sidecarDigest"`
	SourceDigest           string   `json:"sourceDigest"`
	ConfigDigest           string   `json:"configDigest"`
	ExposureCount          int      `json:"exposureCount"`
	CatalogCount           int      `json:"catalogCount"`
	NavigationCount        int      `json:"navigationCount"`
	GeneratedConsumerCount int      `json:"generatedConsumerCount"`
	ProjectionDigest       string   `json:"projectionDigest"`
	UnresolvedCount        int      `json:"unresolvedCount"`
	ErrorCodes             []string `json:"errorCodes"`
	SkippedChecks          []string `json:"skippedChecks"`
}

// Observation aliases make the producer boundary explicit while keeping the
// details schema closed and directly marshalable by DetailsRegistry.
type FrontendTransportObservation = FrontendTransportDetails
type FrontendExposureObservation = FrontendExposureDetails

func RegisterFrontendDetails(registry *DetailsRegistry) error {
	if err := RegisterDetails(registry, FrontendTransportSurfaceID, validateFrontendTransportDetails); err != nil {
		return err
	}
	return RegisterDetails(registry, FrontendExposureSurfaceID, validateFrontendExposureDetails)
}

func NewFrontendTransportReport(
	ctx context.Context,
	identities buildinfo.IdentityProvider,
	profiles releasecontract.ProfileResolver,
	repoRoot, contractPath, schemaPath, profileID string,
	details FrontendTransportDetails,
	outcome Outcome,
) (SurfaceReportV1, error) {
	identity, profile, err := resolveFrontendAuthority(ctx, identities, profiles, repoRoot, contractPath, schemaPath, profileID)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	if err := validateFrontendTransportDetails(details); err != nil {
		return SurfaceReportV1{}, reportError("evidence.details", err)
	}
	if err := validateFrontendPassOutcome(outcome); err != nil {
		return SurfaceReportV1{}, err
	}
	registry := NewDetailsRegistry()
	rawDetails, err := registry.MarshalDetails(FrontendTransportSurfaceID, details)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	digest, err := detailsDigest(rawDetails)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	report := NewReport(
		frontendReleaseIdentity(identity, profile.ID),
		SurfaceIdentity{Surface: FrontendTransportSurfaceID, CanonicalSource: frontendCanonicalSource, Consumer: frontendTransportConsumer, Version: frontendSurfaceVersion, SourceDigest: details.SourceDigest, ConsumerDigest: digest},
		Drift{Missing: []string{}, Extra: []string{}, Incompatible: []string{}},
		Evidence{Class: identity.EvidenceClass, Environment: "repository", Mode: frontendSurfaceEvidenceMode, CheckedAt: time.Now().UTC().Format(time.RFC3339Nano), ToolVersions: map[string]string{}, Details: rawDetails},
		outcome,
	)
	if err := Validate(report, registry); err != nil {
		return SurfaceReportV1{}, err
	}
	return report, nil
}

func NewFrontendExposureReport(
	ctx context.Context,
	identities buildinfo.IdentityProvider,
	profiles releasecontract.ProfileResolver,
	repoRoot, contractPath, schemaPath, profileID string,
	details FrontendExposureDetails,
	outcome Outcome,
) (SurfaceReportV1, error) {
	identity, profile, err := resolveFrontendAuthority(ctx, identities, profiles, repoRoot, contractPath, schemaPath, profileID)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	if err := validateFrontendExposureDetails(details); err != nil {
		return SurfaceReportV1{}, reportError("evidence.details", err)
	}
	if err := validateFrontendPassOutcome(outcome); err != nil {
		return SurfaceReportV1{}, err
	}
	registry := NewDetailsRegistry()
	rawDetails, err := registry.MarshalDetails(FrontendExposureSurfaceID, details)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	digest, err := detailsDigest(rawDetails)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	report := NewReport(
		frontendReleaseIdentity(identity, profile.ID),
		SurfaceIdentity{Surface: FrontendExposureSurfaceID, CanonicalSource: frontendCanonicalSource, Consumer: frontendExposureConsumer, Version: frontendSurfaceVersion, SourceDigest: details.SourceDigest, ConsumerDigest: digest},
		Drift{Missing: []string{}, Extra: []string{}, Incompatible: []string{}},
		Evidence{Class: identity.EvidenceClass, Environment: "repository", Mode: frontendSurfaceEvidenceMode, CheckedAt: time.Now().UTC().Format(time.RFC3339Nano), ToolVersions: map[string]string{}, Details: rawDetails},
		outcome,
	)
	if err := Validate(report, registry); err != nil {
		return SurfaceReportV1{}, err
	}
	return report, nil
}

// ValidateFrontendSurfacePair prevents independently produced reports from
// splicing different compiler inputs. It validates only shared provenance;
// transport and exposure details remain separate schemas.
func ValidateFrontendSurfacePair(transport FrontendTransportDetails, exposure FrontendExposureDetails) error {
	if err := validateFrontendTransportDetails(transport); err != nil {
		return err
	}
	if err := validateFrontendExposureDetails(exposure); err != nil {
		return err
	}
	if transport.SidecarDigest != exposure.SidecarDigest || transport.SourceDigest != exposure.SourceDigest || transport.ConfigDigest != exposure.ConfigDigest {
		return fmt.Errorf("frontend sidecar provenance differs")
	}
	return nil
}

func resolveFrontendAuthority(ctx context.Context, identities buildinfo.IdentityProvider, profiles releasecontract.ProfileResolver, repoRoot, contractPath, schemaPath, profileID string) (buildinfo.BuildIdentityV1, releasecontract.DeploymentProfile, error) {
	if ctx == nil || identities == nil || profiles == nil || strings.TrimSpace(profileID) == "" {
		return buildinfo.BuildIdentityV1{}, releasecontract.DeploymentProfile{}, reportError("releaseIdentity", nil)
	}
	identity, err := identities.Resolve(ctx, repoRoot, contractPath, schemaPath)
	if err != nil {
		return buildinfo.BuildIdentityV1{}, releasecontract.DeploymentProfile{}, err
	}
	if err := buildinfo.ValidateIdentity(identity); err != nil {
		return buildinfo.BuildIdentityV1{}, releasecontract.DeploymentProfile{}, err
	}
	profile, err := profiles.ResolveCommittedProfile(ctx, repoRoot, contractPath, schemaPath, profileID)
	if err != nil {
		return buildinfo.BuildIdentityV1{}, releasecontract.DeploymentProfile{}, err
	}
	if profile.ID != profileID || profile.Commitment != releasecontract.CommitmentCommitted {
		return buildinfo.BuildIdentityV1{}, releasecontract.DeploymentProfile{}, reportError("releaseIdentity.deploymentProfile", nil)
	}
	return identity, profile, nil
}

func frontendReleaseIdentity(identity buildinfo.BuildIdentityV1, profileID string) ReleaseIdentity {
	return ReleaseIdentity{ReleaseCommit: identity.ReleaseCommit, SourceTree: identity.SourceTree, ContractDigest: identity.ContractDigest, DeploymentProfile: profileID, Dirty: identity.Dirty, EvidenceClass: identity.EvidenceClass}
}

func validateFrontendPassOutcome(outcome Outcome) error {
	if outcome.Result != ResultPass || outcome.ErrorCodes == nil || len(outcome.ErrorCodes) != 0 || outcome.SkippedChecks == nil || len(outcome.SkippedChecks) != 0 {
		return reportError("outcome", nil)
	}
	return nil
}

func validateFrontendTransportDetails(details FrontendTransportDetails) error {
	if details.SchemaVersion != frontendTransportObservationSchema || details.OperationCount <= 0 || details.CoreCount <= 0 || details.CompatibleCount <= 0 || details.OperationCount != details.CoreCount || details.CoreCount != details.CompatibleCount || details.UnresolvedCount != 0 {
		return fmt.Errorf("frontend transport counts or schema invalid")
	}
	if !validFrontendDigests(details.SidecarDigest, details.SourceDigest, details.ConfigDigest, details.TaxonomyDigest) {
		return fmt.Errorf("frontend transport digest invalid")
	}
	return validateFrontendNoSkip(details.ErrorCodes, details.SkippedChecks)
}

func validateFrontendExposureDetails(details FrontendExposureDetails) error {
	if details.SchemaVersion != frontendExposureObservationSchema || details.ExposureCount <= 0 || details.CatalogCount <= 0 || details.NavigationCount <= 0 || details.GeneratedConsumerCount <= 0 || details.UnresolvedCount != 0 {
		return fmt.Errorf("frontend exposure counts or schema invalid")
	}
	if !validFrontendDigests(details.SidecarDigest, details.SourceDigest, details.ConfigDigest, details.ProjectionDigest) {
		return fmt.Errorf("frontend exposure digest invalid")
	}
	return validateFrontendNoSkip(details.ErrorCodes, details.SkippedChecks)
}

func validateFrontendNoSkip(errorCodes, skipped []string) error {
	if errorCodes == nil || skipped == nil || len(errorCodes) != 0 || len(skipped) != 0 {
		return fmt.Errorf("frontend observation is not a no-skip pass")
	}
	return nil
}

func validFrontendDigests(values ...string) bool {
	for _, value := range values {
		if !validDigest(value) {
			return false
		}
	}
	return true
}

func validateFrontendTransportReport(report SurfaceReportV1) error {
	var details FrontendTransportDetails
	if err := json.Unmarshal(report.Evidence.Details, &details); err != nil {
		return reportError("evidence.details", err)
	}
	if err := validateFrontendTransportDetails(details); err != nil {
		return reportError("evidence.details", err)
	}
	if report.SurfaceIdentity.CanonicalSource != frontendCanonicalSource || report.SurfaceIdentity.Consumer != frontendTransportConsumer || report.SurfaceIdentity.Version != frontendSurfaceVersion || report.SurfaceIdentity.SourceDigest != details.SourceDigest {
		return reportError("surfaceIdentity", nil)
	}
	if report.Evidence.Environment != "repository" || report.Evidence.Mode != frontendSurfaceEvidenceMode || !reflect.DeepEqual(report.Evidence.ToolVersions, map[string]string{}) {
		return reportError("evidence", nil)
	}
	if len(report.Drift.Missing) != 0 || len(report.Drift.Extra) != 0 || len(report.Drift.Incompatible) != 0 || report.Outcome.Result != ResultPass || len(report.Outcome.ErrorCodes) != 0 || len(report.Outcome.SkippedChecks) != 0 {
		return reportError("outcome", nil)
	}
	digest, err := detailsDigest(report.Evidence.Details)
	if err != nil || report.SurfaceIdentity.ConsumerDigest != digest {
		return reportError("surfaceIdentity.digest", err)
	}
	return nil
}

func validateFrontendExposureReport(report SurfaceReportV1) error {
	var details FrontendExposureDetails
	if err := json.Unmarshal(report.Evidence.Details, &details); err != nil {
		return reportError("evidence.details", err)
	}
	if err := validateFrontendExposureDetails(details); err != nil {
		return reportError("evidence.details", err)
	}
	if report.SurfaceIdentity.CanonicalSource != frontendCanonicalSource || report.SurfaceIdentity.Consumer != frontendExposureConsumer || report.SurfaceIdentity.Version != frontendSurfaceVersion || report.SurfaceIdentity.SourceDigest != details.SourceDigest {
		return reportError("surfaceIdentity", nil)
	}
	if report.Evidence.Environment != "repository" || report.Evidence.Mode != frontendSurfaceEvidenceMode || !reflect.DeepEqual(report.Evidence.ToolVersions, map[string]string{}) {
		return reportError("evidence", nil)
	}
	if len(report.Drift.Missing) != 0 || len(report.Drift.Extra) != 0 || len(report.Drift.Incompatible) != 0 || report.Outcome.Result != ResultPass || len(report.Outcome.ErrorCodes) != 0 || len(report.Outcome.SkippedChecks) != 0 {
		return reportError("outcome", nil)
	}
	digest, err := detailsDigest(report.Evidence.Details)
	if err != nil || report.SurfaceIdentity.ConsumerDigest != digest {
		return reportError("surfaceIdentity.digest", err)
	}
	return nil
}
