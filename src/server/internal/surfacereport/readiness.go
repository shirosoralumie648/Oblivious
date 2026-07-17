package surfacereport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/releasecontract"
)

const ReadinessSurfaceID = "readiness"

type ReadinessDetails struct {
	Generation uint64 `json:"generation"`
	CheckedAt  string `json:"checkedAt"`
	ValidUntil string `json:"validUntil"`
}

type ReadinessInspection struct {
	Online  *releasecontract.Evaluation
	Offline *releasecontract.ReadinessSnapshotV1
}

func OnlineReadinessInspection(evaluation releasecontract.Evaluation) ReadinessInspection {
	return ReadinessInspection{Online: &evaluation}
}

func OfflineReadinessInspection(snapshot releasecontract.ReadinessSnapshotV1) ReadinessInspection {
	return ReadinessInspection{Offline: &snapshot}
}

func RegisterReadinessDetails(registry *DetailsRegistry) error {
	return RegisterDetails(registry, ReadinessSurfaceID, validateReadinessDetails)
}

func NewReadinessReport(
	ctx context.Context,
	identities buildinfo.IdentityProvider,
	profiles releasecontract.ProfileResolver,
	repoRoot, contractPath, schemaPath, profileID string,
	input ReadinessInspection,
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
	contract, err := releasecontract.Load(ctx, repoRoot, contractPath, schemaPath)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	snapshot, mode, err := readinessInspectionSnapshot(input)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	if err := validateReadinessSnapshotAuthority(contract, identity, profile, snapshot, outcome); err != nil {
		return SurfaceReportV1{}, err
	}
	if len(outcome.SkippedChecks) != 0 {
		return SurfaceReportV1{}, reportError("outcome.skippedChecks", nil)
	}
	if err := validateReadinessOutcomeCodes(contract, outcome); err != nil {
		return SurfaceReportV1{}, err
	}

	details := ReadinessDetails{
		Generation: snapshot.Generation,
		CheckedAt:  snapshot.CheckedAt.UTC().Format(time.RFC3339Nano),
		ValidUntil: snapshot.ValidUntil.UTC().Format(time.RFC3339Nano),
	}
	registry := NewDetailsRegistry()
	if err := RegisterReadinessDetails(registry); err != nil {
		return SurfaceReportV1{}, err
	}
	rawDetails, err := registry.MarshalDetails(ReadinessSurfaceID, details)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	consumerDigest, err := detailsDigest(rawDetails)
	if err != nil {
		return SurfaceReportV1{}, err
	}
	checkedAt := time.Now().UTC().Format(time.RFC3339Nano)
	report := NewReport(
		ReleaseIdentity{
			ReleaseCommit: identity.ReleaseCommit, SourceTree: identity.SourceTree,
			ContractDigest: identity.ContractDigest, DeploymentProfile: profile.ID,
			Dirty: identity.Dirty, EvidenceClass: identity.EvidenceClass,
		},
		SurfaceIdentity{
			Surface: ReadinessSurfaceID, CanonicalSource: "config/release/contract.v1.json",
			Consumer: "runtime-readiness-inspector", Version: "v1",
			SourceDigest: identity.ContractDigest, ConsumerDigest: consumerDigest,
		},
		Drift{Missing: []string{}, Extra: []string{}, Incompatible: []string{}},
		Evidence{
			Class: identity.EvidenceClass, Environment: "repository", Mode: mode,
			CheckedAt: checkedAt, ToolVersions: map[string]string{}, Details: rawDetails,
		},
		outcome,
	)
	if err := Validate(report, registry); err != nil {
		return SurfaceReportV1{}, err
	}
	return report, nil
}

func readinessInspectionSnapshot(input ReadinessInspection) (releasecontract.ReadinessSnapshotV1, string, error) {
	if (input.Online == nil) == (input.Offline == nil) {
		return releasecontract.ReadinessSnapshotV1{}, "", reportError("evidence.details", nil)
	}
	if input.Online != nil {
		return input.Online.Snapshot(), "online", nil
	}
	content, err := json.Marshal(input.Offline)
	if err != nil {
		return releasecontract.ReadinessSnapshotV1{}, "", reportError("evidence.details", err)
	}
	var snapshot releasecontract.ReadinessSnapshotV1
	if err := json.Unmarshal(content, &snapshot); err != nil {
		return releasecontract.ReadinessSnapshotV1{}, "", reportError("evidence.details", err)
	}
	return snapshot, "offline", nil
}

func validateReadinessSnapshotAuthority(contract releasecontract.AuthoredContractV1, identity buildinfo.BuildIdentityV1, profile releasecontract.DeploymentProfile, snapshot releasecontract.ReadinessSnapshotV1, outcome Outcome) error {
	if snapshot.SchemaVersion != releasecontract.ReadinessSnapshotSchemaV1 || snapshot.Identity != identity || snapshot.Profile != profile.ID || snapshot.Generation == 0 || snapshot.CheckedAt.IsZero() || snapshot.ValidUntil.IsZero() {
		return reportError("evidence.details.identity", nil)
	}
	now := time.Now().UTC()
	evaluated, evaluateErr := releasecontract.NewEvaluator().Evaluate(contract, identity, profile, snapshot.Generation, snapshot.Observations, now)
	if evaluateErr != nil {
		var readinessErr *releasecontract.ReadinessError
		if !errors.As(evaluateErr, &readinessErr) || outcome.Result != ResultFail || !containsString(outcome.ErrorCodes, string(readinessErr.Code)) {
			return reportError("evidence.details.evaluation", evaluateErr)
		}
	} else {
		if evaluated.ValidUntil.UnixNano() != snapshot.ValidUntil.UTC().UnixNano() || !reflect.DeepEqual(evaluated.Capabilities, snapshot.Capabilities) {
			return reportError("evidence.details.evaluation", nil)
		}
		if outcome.Result == ResultPass {
			for _, capability := range evaluated.Capabilities {
				if capability.Commitment == releasecontract.CommitmentCommitted && capability.Availability != releasecontract.AvailabilityEnabled {
					return reportError("outcome.result", nil)
				}
			}
		}
	}
	allowedFuture := now.Add(time.Duration(profile.AllowedFutureSkewSeconds) * time.Second)
	if snapshot.CheckedAt.UTC().UnixNano() > allowedFuture.UnixNano() || snapshot.CheckedAt.UTC().UnixNano() > snapshot.ValidUntil.UTC().UnixNano() {
		return reportError("evidence.details.checkedAt", nil)
	}
	return nil
}

func validateReadinessOutcomeCodes(contract releasecontract.AuthoredContractV1, outcome Outcome) error {
	reasons := make(map[string]struct{}, len(contract.ReasonCodes))
	for _, reason := range contract.ReasonCodes {
		reasons[reason.ID] = struct{}{}
	}
	for _, code := range outcome.ErrorCodes {
		if _, ok := reasons[code]; !ok {
			return reportError("outcome.errorCodes", nil)
		}
	}
	return nil
}

func validateReadinessDetails(details ReadinessDetails) error {
	if details.Generation == 0 {
		return fmt.Errorf("generation must be greater than zero")
	}
	checkedAt, err := time.Parse(time.RFC3339Nano, details.CheckedAt)
	if err != nil || checkedAt.UTC().Format(time.RFC3339Nano) != details.CheckedAt || !strings.HasSuffix(details.CheckedAt, "Z") {
		return fmt.Errorf("checkedAt must be canonical UTC")
	}
	validUntil, err := time.Parse(time.RFC3339Nano, details.ValidUntil)
	if err != nil || validUntil.UTC().Format(time.RFC3339Nano) != details.ValidUntil || !strings.HasSuffix(details.ValidUntil, "Z") {
		return fmt.Errorf("validUntil must be canonical UTC")
	}
	if validUntil.UnixNano() < checkedAt.UnixNano() {
		return fmt.Errorf("validUntil precedes checkedAt")
	}
	return nil
}

func containsString(values []string, target string) bool {
	index := sort.SearchStrings(values, target)
	return index < len(values) && values[index] == target
}
