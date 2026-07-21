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
	ProtobufSurfaceID              = "protobuf"
	protobufObservationSchemaV1    = "protobuf-observation/v1"
	protobufCanonicalSource        = "config/release/protobuf-toolchain.v1.json"
	protobufConsumer               = "tracked-protobuf-generated-consumers"
	protobufVersion                = "v1"
	protobufManifestDigest         = "sha256:3b225e40c4a7659d07c2638f128ac2087483d2ebab737f4533415793dcad54eb"
	protobufRegenerationDigest     = "sha256:c74af7cc805309b00807b9ecfbbbce31ec2737c4fb726130a70ecbbc168da96a"
	protobufProtocVersion          = "25.1"
	protobufGeneratedHeaderVersion = "v4.25.1"
	protobufGenGoVersion           = "1.36.11"
	protobufGenGoGRPCVersion       = "1.6.2"
)

type ProtobufToolVersions struct {
	Protoc          string `json:"protoc"`
	ProtocGenGo     string `json:"protoc-gen-go"`
	ProtocGenGoGRPC string `json:"protoc-gen-go-grpc"`
}

type ProtobufRegeneration struct {
	Result         string `json:"result"`
	GeneratedCount int    `json:"generatedCount"`
	Digest         string `json:"digest"`
}

type ProtobufDetails struct {
	SchemaVersion          string               `json:"schemaVersion"`
	ManifestDigest         string               `json:"manifestDigest"`
	ToolVersions           ProtobufToolVersions `json:"toolVersions"`
	GeneratedHeaderVersion string               `json:"generatedHeaderVersion"`
	SourceCount            int                  `json:"sourceCount"`
	OutputCount            int                  `json:"outputCount"`
	ManagedCount           int                  `json:"managedCount"`
	SourceOnlyCount        int                  `json:"sourceOnlyCount"`
	Regeneration           ProtobufRegeneration `json:"regeneration"`
	ErrorCodes             []string             `json:"errorCodes"`
	SkippedChecks          []string             `json:"skippedChecks"`
}

func RegisterProtobufDetails(registry *DetailsRegistry) error {
	return RegisterDetails(registry, ProtobufSurfaceID, validateProtobufDetails)
}

func NewProtobufReport(
	ctx context.Context,
	identities buildinfo.IdentityProvider,
	profiles releasecontract.ProfileResolver,
	repoRoot, contractPath, schemaPath, profileID string,
	details ProtobufDetails,
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
	if err := validateProtobufDetails(details); err != nil {
		return SurfaceReportV1{}, reportError("evidence.details", err)
	}
	if outcome.Result != ResultPass || len(outcome.ErrorCodes) != 0 || len(outcome.SkippedChecks) != 0 {
		return SurfaceReportV1{}, reportError("outcome", nil)
	}

	registry := NewDetailsRegistry()
	rawDetails, err := registry.MarshalDetails(ProtobufSurfaceID, details)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	consumerDigest, err := detailsDigest(rawDetails)
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
			Surface: ProtobufSurfaceID, CanonicalSource: protobufCanonicalSource,
			Consumer: protobufConsumer, Version: protobufVersion,
			SourceDigest: details.ManifestDigest, ConsumerDigest: consumerDigest,
		},
		Drift{Missing: []string{}, Extra: []string{}, Incompatible: []string{}},
		Evidence{
			Class: identity.EvidenceClass, Environment: "repository", Mode: "fresh-regeneration",
			CheckedAt:    time.Now().UTC().Format(time.RFC3339Nano),
			ToolVersions: protobufEvidenceToolVersions(details.ToolVersions), Details: rawDetails,
		},
		outcome,
	)
	if err := Validate(report, registry); err != nil {
		return SurfaceReportV1{}, err
	}
	return report, nil
}

func validateProtobufDetails(details ProtobufDetails) error {
	expectedTools := ProtobufToolVersions{
		Protoc: protobufProtocVersion, ProtocGenGo: protobufGenGoVersion,
		ProtocGenGoGRPC: protobufGenGoGRPCVersion,
	}
	if details.SchemaVersion != protobufObservationSchemaV1 {
		return fmt.Errorf("unexpected observation schema")
	}
	if details.ManifestDigest != protobufManifestDigest || details.GeneratedHeaderVersion != protobufGeneratedHeaderVersion {
		return fmt.Errorf("protobuf manifest contract mismatch")
	}
	if details.ToolVersions != expectedTools {
		return fmt.Errorf("protobuf tool version mismatch")
	}
	if details.SourceCount != 10 || details.OutputCount != 21 || details.ManagedCount != 9 || details.SourceOnlyCount != 1 || details.SourceCount != details.ManagedCount+details.SourceOnlyCount {
		return fmt.Errorf("protobuf disposition inventory mismatch")
	}
	if details.Regeneration.Result != "pass" || details.Regeneration.GeneratedCount != details.OutputCount || details.Regeneration.Digest != protobufRegenerationDigest {
		return fmt.Errorf("protobuf regeneration mismatch")
	}
	if details.ErrorCodes == nil || details.SkippedChecks == nil || len(details.ErrorCodes) != 0 || len(details.SkippedChecks) != 0 {
		return fmt.Errorf("protobuf observation is not a no-skip pass")
	}
	return nil
}

func validateProtobufReport(report SurfaceReportV1) error {
	var details ProtobufDetails
	if err := json.Unmarshal(report.Evidence.Details, &details); err != nil {
		return reportError("evidence.details", err)
	}
	digest, err := detailsDigest(report.Evidence.Details)
	if err != nil {
		return err
	}
	if report.SurfaceIdentity.CanonicalSource != protobufCanonicalSource || report.SurfaceIdentity.Consumer != protobufConsumer || report.SurfaceIdentity.Version != protobufVersion {
		return reportError("surfaceIdentity", nil)
	}
	if report.SurfaceIdentity.SourceDigest != details.ManifestDigest || report.SurfaceIdentity.ConsumerDigest != digest {
		return reportError("surfaceIdentity.digest", nil)
	}
	if report.Evidence.Environment != "repository" || report.Evidence.Mode != "fresh-regeneration" || !reflect.DeepEqual(report.Evidence.ToolVersions, protobufEvidenceToolVersions(details.ToolVersions)) {
		return reportError("evidence", nil)
	}
	return nil
}

func protobufEvidenceToolVersions(versions ProtobufToolVersions) map[string]string {
	return map[string]string{
		"protoc":             versions.Protoc,
		"protoc-gen-go":      versions.ProtocGenGo,
		"protoc-gen-go-grpc": versions.ProtocGenGoGRPC,
	}
}
