package surfacereport

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"oblivious/server/internal/buildinfo"
	runtimehttp "oblivious/server/internal/http"
	"oblivious/server/internal/releasecontract"
)

const (
	HTTPRuntimeSurfaceID       = "http-runtime"
	httpRuntimeCanonicalSource = "docs/api/openapi.yaml"
	httpRuntimeConsumer        = "runtime-route-registry"
	httpRuntimeVersion         = "v1"
	httpRuntimeEvidenceMode    = "runtime-registry-parity"
	httpRuntimeParityErrorCode = "http_runtime_parity_failed"
)

type HTTPRuntimeDetails struct {
	OperationCount  int    `json:"operationCount"`
	MountedCount    int    `json:"mountedCount"`
	DescriptorCount int    `json:"descriptorCount"`
	CoreDigest      string `json:"coreDigest"`
	RuntimeDigest   string `json:"runtimeDigest"`
	MediaProbeCount int    `json:"mediaProbeCount"`
	ParityResult    string `json:"parityResult"`
}

func RegisterHTTPRuntimeDetails(registry *DetailsRegistry) error {
	return RegisterDetails(registry, HTTPRuntimeSurfaceID, validateHTTPRuntimeDetails)
}

func NewHTTPRuntimeReport(
	ctx context.Context,
	identities buildinfo.IdentityProvider,
	profiles releasecontract.ProfileResolver,
	repoRoot, contractPath, schemaPath, profileID string,
	observation runtimehttp.HTTPRuntimeObservation,
) (SurfaceReportV1, error) {
	if ctx == nil || identities == nil || profiles == nil || strings.TrimSpace(profileID) == "" {
		return SurfaceReportV1{}, reportError("releaseIdentity", nil)
	}
	identity, err := identities.Resolve(ctx, repoRoot, contractPath, schemaPath)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	if err := buildinfo.ValidateIdentity(identity); err != nil {
		return SurfaceReportV1{}, err
	}
	profile, err := profiles.ResolveCommittedProfile(ctx, repoRoot, contractPath, schemaPath, profileID)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	if profile.ID != profileID || profile.Commitment != releasecontract.CommitmentCommitted {
		return SurfaceReportV1{}, reportError("releaseIdentity.deploymentProfile", nil)
	}
	if observation.MismatchIDs == nil || !sortedUniqueNonEmpty(observation.MismatchIDs) {
		return SurfaceReportV1{}, reportError("drift.incompatible", nil)
	}
	details := httpRuntimeDetailsFromObservation(observation)
	if err := validateHTTPRuntimeDetails(details); err != nil {
		return SurfaceReportV1{}, reportError("evidence.details", err)
	}

	incompatible := make([]string, len(observation.MismatchIDs))
	copy(incompatible, observation.MismatchIDs)
	drift := Drift{Missing: []string{}, Extra: []string{}, Incompatible: incompatible}
	outcome := Outcome{Result: ResultFail, ErrorCodes: []string{httpRuntimeParityErrorCode}, SkippedChecks: []string{}}
	if details.ParityResult == "pass" {
		if len(observation.MismatchIDs) != 0 {
			return SurfaceReportV1{}, reportError("drift.incompatible", nil)
		}
		outcome = Outcome{Result: ResultPass, ErrorCodes: []string{}, SkippedChecks: []string{}}
	}

	registry := NewDetailsRegistry()
	rawDetails, err := registry.MarshalDetails(HTTPRuntimeSurfaceID, details)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	report := NewReport(
		ReleaseIdentity{
			ReleaseCommit: identity.ReleaseCommit, SourceTree: identity.SourceTree,
			ContractDigest: identity.ContractDigest, DeploymentProfile: profile.ID,
			Dirty: identity.Dirty, EvidenceClass: identity.EvidenceClass,
		},
		SurfaceIdentity{
			Surface: HTTPRuntimeSurfaceID, CanonicalSource: httpRuntimeCanonicalSource,
			Consumer: httpRuntimeConsumer, Version: httpRuntimeVersion,
			SourceDigest: details.CoreDigest, ConsumerDigest: details.RuntimeDigest,
		},
		drift,
		Evidence{
			Class: identity.EvidenceClass, Environment: "repository", Mode: httpRuntimeEvidenceMode,
			CheckedAt: time.Now().UTC().Format(time.RFC3339Nano), ToolVersions: map[string]string{}, Details: rawDetails,
		},
		outcome,
	)
	if err := Validate(report, registry); err != nil {
		return SurfaceReportV1{}, err
	}
	return report, nil
}

func httpRuntimeDetailsFromObservation(observation runtimehttp.HTTPRuntimeObservation) HTTPRuntimeDetails {
	return HTTPRuntimeDetails{
		OperationCount: observation.OperationCount, MountedCount: observation.MountedCount,
		DescriptorCount: observation.DescriptorCount, CoreDigest: observation.CoreDigest,
		RuntimeDigest: observation.RuntimeDigest, MediaProbeCount: observation.MediaProbeCount,
		ParityResult: observation.ParityResult,
	}
}

func validateHTTPRuntimeDetails(details HTTPRuntimeDetails) error {
	if details.OperationCount <= 0 || details.MountedCount <= 0 || details.DescriptorCount <= 0 || details.MediaProbeCount <= 0 {
		return fmt.Errorf("http runtime counts must be nonzero")
	}
	if details.OperationCount != details.MountedCount || details.OperationCount != details.DescriptorCount {
		return fmt.Errorf("http runtime counts differ")
	}
	if !validDigest(details.CoreDigest) || !validDigest(details.RuntimeDigest) {
		return fmt.Errorf("http runtime digest invalid")
	}
	switch details.ParityResult {
	case "pass":
		if details.CoreDigest != details.RuntimeDigest {
			return fmt.Errorf("passing http runtime digests differ")
		}
	case "fail":
	default:
		return fmt.Errorf("http runtime parity result invalid")
	}
	return nil
}

func validateHTTPRuntimeReport(report SurfaceReportV1) error {
	var details HTTPRuntimeDetails
	if err := json.Unmarshal(report.Evidence.Details, &details); err != nil {
		return reportError("evidence.details", err)
	}
	if report.SurfaceIdentity.CanonicalSource != httpRuntimeCanonicalSource || report.SurfaceIdentity.Consumer != httpRuntimeConsumer || report.SurfaceIdentity.Version != httpRuntimeVersion {
		return reportError("surfaceIdentity", nil)
	}
	if report.SurfaceIdentity.SourceDigest != details.CoreDigest || report.SurfaceIdentity.ConsumerDigest != details.RuntimeDigest {
		return reportError("surfaceIdentity.digest", nil)
	}
	if report.Evidence.Environment != "repository" || report.Evidence.Mode != httpRuntimeEvidenceMode || !reflect.DeepEqual(report.Evidence.ToolVersions, map[string]string{}) {
		return reportError("evidence", nil)
	}
	if len(report.Drift.Missing) != 0 || len(report.Drift.Extra) != 0 || len(report.Outcome.SkippedChecks) != 0 {
		return reportError("outcome", nil)
	}
	switch details.ParityResult {
	case "pass":
		if report.Outcome.Result != ResultPass || len(report.Drift.Incompatible) != 0 || len(report.Outcome.ErrorCodes) != 0 {
			return reportError("outcome", nil)
		}
	case "fail":
		if report.Outcome.Result != ResultFail || !reflect.DeepEqual(report.Outcome.ErrorCodes, []string{httpRuntimeParityErrorCode}) {
			return reportError("outcome", nil)
		}
	default:
		return reportError("outcome", nil)
	}
	return nil
}
