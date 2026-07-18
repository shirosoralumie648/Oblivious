package surfacereport

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/releasecontract"
)

func TestReadinessSurfaceContract(t *testing.T) {
	repoRoot, contract, profile, identity := loadReadinessReportAuthority(t)
	now := time.Now().UTC().Add(-time.Second)
	evaluation := readinessReportEvaluation(t, contract, profile, identity, now, now)
	identities := readinessIdentityProvider{identity: identity}
	profiles := readinessProfileResolver{profile: profile}
	passing := Outcome{Result: ResultPass, ErrorCodes: []string{}, SkippedChecks: []string{}}

	t.Run("online and offline construction return validated values without output ownership", func(t *testing.T) {
		if got := reflect.TypeOf(NewReadinessReport).NumIn(); got != 9 {
			t.Fatalf("NewReadinessReport input count = %d, want 9 with no writer or output path", got)
		}
		working := t.TempDir()
		for _, input := range []ReadinessInspection{
			OnlineReadinessInspection(evaluation),
			OfflineReadinessInspection(evaluation.Snapshot()),
		} {
			report, err := NewReadinessReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", input, passing)
			if err != nil {
				t.Fatalf("construct readiness report: %v", err)
			}
			if report.SurfaceIdentity.Surface != ReadinessSurfaceID || report.ReleaseIdentity.DeploymentProfile != "monolith" {
				t.Fatalf("report identities = %#v / %#v", report.SurfaceIdentity, report.ReleaseIdentity)
			}
			registry := NewDetailsRegistry()
			if err := Validate(report, registry); err != nil {
				t.Fatalf("validate readiness report: %v", err)
			}
		}
		entries, err := os.ReadDir(working)
		if err != nil {
			t.Fatalf("read constructor sentinel directory: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("report constructor wrote files: %#v", entries)
		}
	})

	t.Run("registry accepts exactly generation checkedAt and validUntil", func(t *testing.T) {
		registry := NewDetailsRegistry()
		details := ReadinessDetails{Generation: 1, CheckedAt: now.Format(time.RFC3339Nano), ValidUntil: now.Add(120 * time.Second).Format(time.RFC3339Nano)}
		raw, err := registry.MarshalDetails(ReadinessSurfaceID, details)
		if err != nil {
			t.Fatalf("marshal readiness details: %v", err)
		}
		if err := registry.ValidateDetails(ReadinessSurfaceID, raw); err != nil {
			t.Fatalf("validate readiness details: %v", err)
		}
		for _, invalid := range []json.RawMessage{
			json.RawMessage(`{"generation":1,"checkedAt":"` + now.Format(time.RFC3339Nano) + `"}`),
			json.RawMessage(`{"generation":1,"checkedAt":"` + now.Format(time.RFC3339Nano) + `","validUntil":"` + now.Add(120*time.Second).Format(time.RFC3339Nano) + `","arbitrary":true}`),
			json.RawMessage(`null`),
		} {
			if err := registry.ValidateDetails(ReadinessSurfaceID, invalid); !IsCode(err, ErrorSurfaceSchemaInvalid) {
				t.Fatalf("invalid details error = %T %v", err, err)
			}
		}
		if err := (&DetailsRegistry{schemas: make(map[string]detailsSchema)}).ValidateDetails(ReadinessSurfaceID, raw); !IsCode(err, ErrorSurfaceSchemaInvalid) {
			t.Fatalf("unregistered readiness details error = %T %v", err, err)
		}
	})

	t.Run("identity and profile splice are rejected", func(t *testing.T) {
		snapshot := evaluation.Snapshot()
		snapshot.Identity.ReleaseCommit = repeatReadinessHex("c", 40)
		_, err := NewReadinessReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", OfflineReadinessInspection(snapshot), passing)
		if err == nil {
			t.Fatal("identity-spliced snapshot produced a report")
		}
		snapshot = evaluation.Snapshot()
		snapshot.Profile = "microservices"
		_, err = NewReadinessReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", OfflineReadinessInspection(snapshot), passing)
		if err == nil {
			t.Fatal("profile-spliced snapshot produced a report")
		}
	})

	t.Run("committed skips and E3 E4 claim classes are rejected", func(t *testing.T) {
		withSkip := passing
		withSkip.SkippedChecks = []string{"target-runtime"}
		_, err := NewReadinessReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", OnlineReadinessInspection(evaluation), withSkip)
		if err == nil {
			t.Fatal("readiness report accepted a committed skip")
		}
		for _, evidenceClass := range []string{"target-environment", "same-commit-release"} {
			forged := identity
			forged.EvidenceClass = evidenceClass
			_, err := NewReadinessReport(context.Background(), readinessIdentityProvider{identity: forged}, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", OnlineReadinessInspection(evaluation), passing)
			if err == nil {
				t.Fatalf("readiness report accepted claim class %q", evidenceClass)
			}
		}
	})

	t.Run("offline stale snapshot emits only an explicit failure report", func(t *testing.T) {
		observedAt := time.Now().UTC().Add(-121 * time.Second)
		staleBase := readinessReportEvaluation(t, contract, profile, identity, observedAt, observedAt)
		stale := staleBase.Snapshot()
		failure := Outcome{Result: ResultFail, ErrorCodes: []string{"readiness_stale"}, SkippedChecks: []string{}}
		report, err := NewReadinessReport(context.Background(), identities, profiles, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", OfflineReadinessInspection(stale), failure)
		if err != nil {
			t.Fatalf("construct stale failure report: %v", err)
		}
		if report.Outcome.Result != ResultFail || !reflect.DeepEqual(report.Outcome.ErrorCodes, []string{"readiness_stale"}) {
			t.Fatalf("stale report outcome = %#v", report.Outcome)
		}
	})

	t.Run("inspection input cannot become runtime authorization", func(t *testing.T) {
		if _, ok := reflect.TypeOf(ReadinessInspection{}).FieldByName("Manager"); ok {
			t.Fatal("readiness inspection accepts a runtime manager")
		}
		if _, ok := reflect.TypeOf(ReadinessInspection{}).FieldByName("Guard"); ok {
			t.Fatal("readiness inspection accepts a runtime guard")
		}
		if _, ok := reflect.TypeOf(ReadinessInspection{}).FieldByName("Writer"); ok {
			t.Fatal("readiness inspection accepts a report writer")
		}
	})
}

type readinessIdentityProvider struct {
	identity buildinfo.BuildIdentityV1
	err      error
}

func (p readinessIdentityProvider) Resolve(context.Context, string, string, string) (buildinfo.BuildIdentityV1, error) {
	return p.identity, p.err
}

type readinessProfileResolver struct {
	profile releasecontract.DeploymentProfile
	err     error
}

func (r readinessProfileResolver) ResolveCommittedProfile(context.Context, string, string, string, string) (releasecontract.DeploymentProfile, error) {
	return r.profile, r.err
}

func loadReadinessReportAuthority(t *testing.T) (string, releasecontract.AuthoredContractV1, releasecontract.DeploymentProfile, buildinfo.BuildIdentityV1) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve readiness report test source")
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
	identity := buildinfo.BuildIdentityV1{
		SchemaVersion: buildinfo.BuildIdentitySchemaV1, ReleaseCommit: repeatReadinessHex("a", 40),
		SourceTree: repeatReadinessHex("b", 40), ContractDigest: digest,
		EvidenceClass: buildinfo.EvidenceRepositoryLocal,
	}
	return repoRoot, contract, profile, identity
}

func readinessReportEvaluation(t *testing.T, contract releasecontract.AuthoredContractV1, profile releasecontract.DeploymentProfile, identity buildinfo.BuildIdentityV1, observedAt, checkedAt time.Time) releasecontract.Evaluation {
	t.Helper()
	observations := readinessReportObservations(contract, profile, observedAt)
	evaluation, err := releasecontract.NewEvaluator().Evaluate(contract, identity, profile, 1, observations, checkedAt)
	if err != nil {
		t.Fatalf("evaluate readiness report fixture: %v", err)
	}
	return evaluation
}

func readinessReportObservations(contract releasecontract.AuthoredContractV1, profile releasecontract.DeploymentProfile, observedAt time.Time) []releasecontract.Observation {
	capabilities := make(map[string]releasecontract.Commitment, len(contract.Capabilities))
	for _, capability := range contract.Capabilities {
		capabilities[capability.ID] = capability.Commitment
	}
	for _, override := range profile.CapabilityOverrides {
		capabilities[override.CapabilityID] = override.Commitment
	}
	requirements := make(map[string]releasecontract.ReadinessRequirement, len(contract.ReadinessRequirements))
	for _, requirement := range contract.ReadinessRequirements {
		requirements[requirement.ID] = requirement
	}
	byDependency := map[string]map[string]struct{}{}
	for _, requirementID := range profile.ReadinessRequirementIDs {
		requirement := requirements[requirementID]
		for _, capabilityID := range requirement.CapabilityIDs {
			if capabilities[capabilityID] == releasecontract.CommitmentExcluded {
				continue
			}
			for _, dependencyID := range requirement.DependencyIDs {
				if byDependency[dependencyID] == nil {
					byDependency[dependencyID] = map[string]struct{}{}
				}
				byDependency[dependencyID][capabilityID] = struct{}{}
			}
		}
	}
	observations := make([]releasecontract.Observation, 0, len(byDependency))
	for dependencyID, capabilitySet := range byDependency {
		capabilityIDs := make([]string, 0, len(capabilitySet))
		for capabilityID := range capabilitySet {
			capabilityIDs = append(capabilityIDs, capabilityID)
		}
		sort.Strings(capabilityIDs)
		observations = append(observations, releasecontract.Observation{
			ProbeID: "probe." + dependencyID, DependencyID: dependencyID,
			CapabilityIDs: capabilityIDs, Availability: releasecontract.AvailabilityEnabled,
			ObservedAt: observedAt,
		})
	}
	return observations
}

func repeatReadinessHex(value string, count int) string {
	result := ""
	for len(result) < count {
		result += value
	}
	return result[:count]
}
