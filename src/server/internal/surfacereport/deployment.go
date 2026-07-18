package surfacereport

import (
	"context"
	"strings"
	"time"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/releasecontract"
)

const DeploymentSurfaceID = "deployment"

type DeploymentDetails struct {
	Profile           string `json:"profile"`
	CanonicalWorkload string `json:"canonicalWorkload"`
	StartupEndpoint   string `json:"startupEndpoint"`
	LivenessEndpoint  string `json:"livenessEndpoint"`
	ReadinessEndpoint string `json:"readinessEndpoint"`
	AuditStorage      string `json:"auditStorage"`
	MigrationState    string `json:"migrationState"`
	HarnessResult     string `json:"harnessResult"`
}

func RegisterDeploymentDetails(registry *DetailsRegistry) error {
	return RegisterDetails(registry, DeploymentSurfaceID, validateDeploymentDetails)
}

func NewDeploymentReport(
	ctx context.Context,
	identities buildinfo.IdentityProvider,
	profiles releasecontract.ProfileResolver,
	repoRoot, contractPath, schemaPath, profileID string,
	details DeploymentDetails,
	outcome Outcome,
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
	if err := validateDeploymentDetails(details); err != nil {
		return SurfaceReportV1{}, reportError("evidence.details", err)
	}
	if len(outcome.SkippedChecks) != 0 || outcome.Result != ResultPass || len(outcome.ErrorCodes) != 0 {
		return SurfaceReportV1{}, reportError("outcome", nil)
	}
	registry := NewDetailsRegistry()
	rawDetails, err := registry.MarshalDetails(DeploymentSurfaceID, details)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	consumerDigest, err := detailsDigest(rawDetails)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	report := NewReport(
		ReleaseIdentity{ReleaseCommit: identity.ReleaseCommit, SourceTree: identity.SourceTree, ContractDigest: identity.ContractDigest, DeploymentProfile: profile.ID, Dirty: identity.Dirty, EvidenceClass: identity.EvidenceClass},
		SurfaceIdentity{Surface: DeploymentSurfaceID, CanonicalSource: "deploy/kubernetes/app-deployment.yaml", Consumer: "readiness-deployment-harness", Version: "v1", SourceDigest: identity.ContractDigest, ConsumerDigest: consumerDigest},
		Drift{Missing: []string{}, Extra: []string{}, Incompatible: []string{}},
		Evidence{Class: identity.EvidenceClass, Environment: "repository", Mode: "deployment-harness", CheckedAt: time.Now().UTC().Format(time.RFC3339Nano), ToolVersions: map[string]string{}, Details: rawDetails},
		outcome,
	)
	if err := Validate(report, registry); err != nil {
		return SurfaceReportV1{}, err
	}
	return report, nil
}

func validateDeploymentDetails(details DeploymentDetails) error {
	if details.Profile != "monolith" || details.CanonicalWorkload != "deploy/kubernetes/app-deployment.yaml" || details.StartupEndpoint != "/livez" || details.LivenessEndpoint != "/livez" || details.ReadinessEndpoint != "/readyz" {
		return &ReportError{Code: ErrorSurfaceSchemaInvalid, Field: "deployment.details.contract"}
	}
	if strings.TrimSpace(details.AuditStorage) == "" || strings.ContainsAny(details.AuditStorage, "/\\") || strings.ContainsAny(details.AuditStorage, "\r\n") {
		return &ReportError{Code: ErrorSurfaceSchemaInvalid, Field: "deployment.details.auditStorage"}
	}
	if details.MigrationState != "applied_and_validated" || details.HarnessResult != "passed" {
		return &ReportError{Code: ErrorSurfaceSchemaInvalid, Field: "deployment.details.result"}
	}
	return nil
}
