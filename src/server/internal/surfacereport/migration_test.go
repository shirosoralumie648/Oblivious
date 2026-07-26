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
	"oblivious/server/internal/migrations"
	"oblivious/server/internal/releasecontract"
)

func TestMigrationStaticAndLedgerSurfaceContracts(t *testing.T) {
	repoRoot, profile, identity := migrationReportAuthority(t)
	identityProvider := migrationIdentityProvider{identity: identity}
	profileResolver := migrationProfileResolver{profile: profile}
	inventory, err := migrations.BuildStaticInventory(context.Background(), repoRoot, "config/release/migration-disposition.v1.json")
	if err != nil {
		t.Fatalf("build migration inventory: %v", err)
	}
	passing := Outcome{Result: ResultPass, ErrorCodes: []string{}, SkippedChecks: []string{}}

	t.Run("constructs distinct trusted static and ledger reports", func(t *testing.T) {
		staticReport, err := NewMigrationStaticReport(
			context.Background(), identityProvider, profileResolver, repoRoot,
			"config/release/contract.v1.json", "config/release/contract.schema.json", "monolith",
			inventory, passing,
		)
		if err != nil {
			t.Fatalf("construct static report: %v", err)
		}
		ledgerReport, err := NewMigrationLedgerReport(
			context.Background(), identityProvider, profileResolver, repoRoot,
			"config/release/contract.v1.json", "config/release/contract.schema.json", "monolith",
			inventory, append([]migrations.MigrationIdentity(nil), inventory.Identities...), passing,
		)
		if err != nil {
			t.Fatalf("construct ledger report: %v", err)
		}
		if staticReport.SurfaceIdentity.Surface != MigrationStaticSurfaceID || ledgerReport.SurfaceIdentity.Surface != MigrationLedgerSurfaceID {
			t.Fatalf("migration surfaces folded: static=%q ledger=%q", staticReport.SurfaceIdentity.Surface, ledgerReport.SurfaceIdentity.Surface)
		}
		if staticReport.ReleaseIdentity.ReleaseCommit != identity.ReleaseCommit || ledgerReport.ReleaseIdentity.ReleaseCommit != identity.ReleaseCommit ||
			staticReport.ReleaseIdentity.DeploymentProfile != profile.ID || ledgerReport.ReleaseIdentity.DeploymentProfile != profile.ID {
			t.Fatal("migration report did not use resolver-derived identity/profile")
		}
		if err := Validate(staticReport, NewDetailsRegistry()); err != nil {
			t.Fatalf("validate static report: %v", err)
		}
		if err := Validate(ledgerReport, NewDetailsRegistry()); err != nil {
			t.Fatalf("validate ledger report: %v", err)
		}
		var staticDetails MigrationStaticDetails
		if err := json.Unmarshal(staticReport.Evidence.Details, &staticDetails); err != nil {
			t.Fatalf("decode static details: %v", err)
		}
		var ledgerDetails MigrationLedgerDetails
		if err := json.Unmarshal(ledgerReport.Evidence.Details, &ledgerDetails); err != nil {
			t.Fatalf("decode ledger details: %v", err)
		}
		if staticDetails.IdentityCount != inventory.MonolithFileCount || staticDetails.FileCount != inventory.MonolithFileCount ||
			staticDetails.IdentityDigest != inventory.IdentityDigest || staticDetails.StaticMetadataDigest != inventory.StaticMetadataDigest {
			t.Fatalf("static details = %#v", staticDetails)
		}
		if ledgerDetails.RowCount != len(inventory.Identities) || ledgerDetails.IdentityDigest != inventory.IdentityDigest || !ledgerDetails.MatchesStatic {
			t.Fatalf("ledger details = %#v", ledgerDetails)
		}
		encoded, err := Marshal(ledgerReport, NewDetailsRegistry())
		if err != nil {
			t.Fatalf("marshal ledger report: %v", err)
		}
		for _, prohibited := range []string{"filename", "disposition", "databaseUrl", "connection", "SELECT ", "target-environment", "same-commit-release"} {
			if strings.Contains(string(encoded), prohibited) {
				t.Fatalf("ledger report contains prohibited field or claim %q", prohibited)
			}
		}
	})

	t.Run("registry keeps static and ledger details closed and non substitutable", func(t *testing.T) {
		registry := NewDetailsRegistry()
		staticDetails := MigrationStaticDetails{
			DatabaseKind: migrationDatabaseKind, IdentityCount: inventory.MonolithFileCount,
			FileCount: inventory.MonolithFileCount, IdentityDigest: inventory.IdentityDigest,
			StaticMetadataDigest:         inventory.StaticMetadataDigest,
			NonMonolithDispositionCounts: cloneStringIntMap(inventory.NonMonolithDispositionCounts),
		}
		staticRaw, err := registry.MarshalDetails(MigrationStaticSurfaceID, staticDetails)
		if err != nil {
			t.Fatalf("marshal static details: %v", err)
		}
		if err := registry.ValidateDetails(MigrationLedgerSurfaceID, staticRaw); !IsCode(err, ErrorSurfaceSchemaInvalid) {
			t.Fatalf("ledger accepted static details: %v", err)
		}
		ledgerDetails := MigrationLedgerDetails{
			DatabaseKind: migrationDatabaseKind, RowCount: len(inventory.Identities),
			IdentityDigest: inventory.IdentityDigest, MatchesStatic: true,
		}
		ledgerRaw, err := registry.MarshalDetails(MigrationLedgerSurfaceID, ledgerDetails)
		if err != nil {
			t.Fatalf("marshal ledger details: %v", err)
		}
		if err := registry.ValidateDetails(MigrationStaticSurfaceID, ledgerRaw); !IsCode(err, ErrorSurfaceSchemaInvalid) {
			t.Fatalf("static accepted ledger details: %v", err)
		}
		if _, err := registry.MarshalDetails(MigrationLedgerSurfaceID, map[string]any{"rowCount": len(inventory.Identities)}); !IsCode(err, ErrorSurfaceSchemaInvalid) {
			t.Fatalf("ledger accepted arbitrary details: %v", err)
		}
		identityBearing := append([]byte(nil), ledgerRaw[:len(ledgerRaw)-1]...)
		identityBearing = append(identityBearing, []byte(`,"releaseIdentity":{"releaseCommit":"`+strings.Repeat("f", 40)+`"}}`)...)
		if err := registry.ValidateDetails(MigrationLedgerSurfaceID, identityBearing); !IsCode(err, ErrorSurfaceSchemaInvalid) {
			t.Fatalf("ledger accepted caller identity: %v", err)
		}
	})

	t.Run("missing extra checksum order duplicate and zero ledger rows fail", func(t *testing.T) {
		valid := append([]migrations.MigrationIdentity(nil), inventory.Identities...)
		cases := []struct {
			name string
			edit func([]migrations.MigrationIdentity) []migrations.MigrationIdentity
		}{
			{name: "missing", edit: func(value []migrations.MigrationIdentity) []migrations.MigrationIdentity { return value[:len(value)-1] }},
			{name: "extra", edit: func(value []migrations.MigrationIdentity) []migrations.MigrationIdentity {
				return append(value, migrations.MigrationIdentity{Version: "9999_extra.sql", Checksum: strings.Repeat("e", 64)})
			}},
			{name: "checksum", edit: func(value []migrations.MigrationIdentity) []migrations.MigrationIdentity {
				value[0].Checksum = strings.Repeat("f", 64)
				return value
			}},
			{name: "order", edit: func(value []migrations.MigrationIdentity) []migrations.MigrationIdentity {
				value[0], value[1] = value[1], value[0]
				return value
			}},
			{name: "duplicate", edit: func(value []migrations.MigrationIdentity) []migrations.MigrationIdentity {
				return append([]migrations.MigrationIdentity{value[0]}, value...)
			}},
			{name: "zero", edit: func([]migrations.MigrationIdentity) []migrations.MigrationIdentity {
				return []migrations.MigrationIdentity{}
			}},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				ledger := testCase.edit(append([]migrations.MigrationIdentity(nil), valid...))
				if _, err := NewMigrationLedgerReport(
					context.Background(), identityProvider, profileResolver, repoRoot,
					"config/release/contract.v1.json", "config/release/contract.schema.json", "monolith",
					inventory, ledger, passing,
				); err == nil {
					t.Fatalf("ledger mutation %q was accepted", testCase.name)
				}
			})
		}
	})

	t.Run("skip profile identity and decoded report splices fail closed", func(t *testing.T) {
		withSkip := passing
		withSkip.SkippedChecks = []string{"database"}
		if _, err := NewMigrationStaticReport(
			context.Background(), identityProvider, profileResolver, repoRoot,
			"config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", inventory, withSkip,
		); err == nil {
			t.Fatal("static report accepted a skip")
		}
		forgedIdentity := identity
		forgedIdentity.EvidenceClass = "target-environment"
		if _, err := NewMigrationStaticReport(
			context.Background(), migrationIdentityProvider{identity: forgedIdentity}, profileResolver, repoRoot,
			"config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", inventory, passing,
		); err == nil {
			t.Fatal("static report accepted target evidence claim")
		}
		for _, resolver := range []migrationProfileResolver{
			{profile: releasecontract.DeploymentProfile{ID: "monolith", Commitment: releasecontract.CommitmentConditional}},
			{profile: releasecontract.DeploymentProfile{ID: "microservices", Commitment: releasecontract.CommitmentCommitted}},
		} {
			if _, err := NewMigrationStaticReport(
				context.Background(), identityProvider, resolver, repoRoot,
				"config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", inventory, passing,
			); err == nil {
				t.Fatalf("static report accepted profile %#v", resolver.profile)
			}
		}

		staticReport, err := NewMigrationStaticReport(
			context.Background(), identityProvider, profileResolver, repoRoot,
			"config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", inventory, passing,
		)
		if err != nil {
			t.Fatalf("construct static splice fixture: %v", err)
		}
		mutations := []struct {
			name string
			edit func(*SurfaceReportV1)
		}{
			{name: "fold surface", edit: func(value *SurfaceReportV1) { value.SurfaceIdentity.Surface = MigrationLedgerSurfaceID }},
			{name: "canonical source", edit: func(value *SurfaceReportV1) { value.SurfaceIdentity.CanonicalSource = migrationLedgerSource }},
			{name: "consumer", edit: func(value *SurfaceReportV1) { value.SurfaceIdentity.Consumer = migrationLedgerConsumer }},
			{name: "version", edit: func(value *SurfaceReportV1) { value.SurfaceIdentity.Version = "v2" }},
			{name: "source digest", edit: func(value *SurfaceReportV1) { value.SurfaceIdentity.SourceDigest = inventory.IdentityDigest }},
			{name: "consumer digest", edit: func(value *SurfaceReportV1) { value.SurfaceIdentity.ConsumerDigest = inventory.StaticMetadataDigest }},
			{name: "environment", edit: func(value *SurfaceReportV1) { value.Evidence.Environment = migrationLedgerEnvironment }},
			{name: "mode", edit: func(value *SurfaceReportV1) { value.Evidence.Mode = migrationLedgerMode }},
			{name: "tool versions", edit: func(value *SurfaceReportV1) { value.Evidence.ToolVersions = map[string]string{"psql": "unknown"} }},
		}
		for _, mutation := range mutations {
			t.Run(mutation.name, func(t *testing.T) {
				candidate := staticReport
				candidate.Evidence.ToolVersions = map[string]string{}
				mutation.edit(&candidate)
				if err := Validate(candidate, NewDetailsRegistry()); err == nil {
					t.Fatalf("decoded report splice %q was accepted", mutation.name)
				}
			})
		}
	})

	t.Run("tampered static inventory cannot create a report", func(t *testing.T) {
		mutations := []struct {
			name string
			edit func(*migrations.StaticInventory)
		}{
			{name: "identity digest", edit: func(value *migrations.StaticInventory) { value.IdentityDigest = "sha256:" + strings.Repeat("0", 64) }},
			{name: "metadata digest", edit: func(value *migrations.StaticInventory) {
				value.StaticMetadataDigest = "sha256:" + strings.Repeat("1", 64)
			}},
			{name: "zero count", edit: func(value *migrations.StaticInventory) { value.MonolithFileCount = 0 }},
			{name: "non monolith count", edit: func(value *migrations.StaticInventory) { value.NonMonolithDispositionCounts = map[string]int{} }},
			{name: "entry identity", edit: func(value *migrations.StaticInventory) { value.Entries[0].Identity.Checksum = strings.Repeat("2", 64) }},
		}
		for _, mutation := range mutations {
			t.Run(mutation.name, func(t *testing.T) {
				candidate := cloneStaticInventory(inventory)
				mutation.edit(&candidate)
				if _, err := NewMigrationStaticReport(
					context.Background(), identityProvider, profileResolver, repoRoot,
					"config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", candidate, passing,
				); err == nil {
					t.Fatalf("static inventory mutation %q was accepted", mutation.name)
				}
			})
		}
	})
}

func TestMigrationReplaySurfaceContract(t *testing.T) {
	repoRoot, profile, identity := migrationReportAuthority(t)
	identityProvider := migrationIdentityProvider{identity: identity}
	profileResolver := migrationProfileResolver{profile: profile}
	inventory, err := migrations.BuildStaticInventory(context.Background(), repoRoot, "config/release/migration-disposition.v1.json")
	if err != nil {
		t.Fatalf("build migration inventory: %v", err)
	}
	before := MigrationLedgerSnapshot{Identities: []migrations.MigrationIdentity{}, IdentityDigest: migrationEmptyIdentityDigest}
	after := MigrationLedgerSnapshot{Identities: append([]migrations.MigrationIdentity(nil), inventory.Identities...), IdentityDigest: inventory.IdentityDigest}
	details, err := DeriveReplayObservation(inventory.Identities, before, after, after)
	if err != nil {
		t.Fatalf("derive replay observation: %v", err)
	}
	details.ReplayMode = migrationReplayDocker
	details.ResourceOwnership = migrationResourceOwnershipOwnedDisposable
	details.CleanupResult = migrationCleanupSucceeded
	passing := Outcome{Result: ResultPass, ErrorCodes: []string{}, SkippedChecks: []string{}}

	t.Run("derives exact apply counts and constructs a distinct trusted report", func(t *testing.T) {
		if details.InitialLedgerRows != 0 || details.FirstApply != (MigrationApplyCounts{Applied: len(inventory.Identities), Skipped: 0}) ||
			details.SecondApply != (MigrationApplyCounts{Applied: 0, Skipped: len(inventory.Identities)}) ||
			details.FinalLedgerRows != len(inventory.Identities) || details.StaticDigest != inventory.IdentityDigest || details.LedgerDigest != inventory.IdentityDigest {
			t.Fatalf("replay details = %#v", details)
		}
		report, err := NewMigrationReplayReport(
			context.Background(), identityProvider, profileResolver, repoRoot,
			"config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", details, passing,
		)
		if err != nil {
			t.Fatalf("construct replay report: %v", err)
		}
		if report.SurfaceIdentity.Surface != MigrationReplaySurfaceID || report.SurfaceIdentity.Surface == MigrationStaticSurfaceID || report.SurfaceIdentity.Surface == MigrationLedgerSurfaceID ||
			report.Evidence.Environment != "local-docker" || report.Evidence.Mode != migrationReplayMode || report.ReleaseIdentity.ReleaseCommit != identity.ReleaseCommit {
			t.Fatalf("unexpected replay envelope: %#v", report)
		}
		if err := Validate(report, NewDetailsRegistry()); err != nil {
			t.Fatalf("validate replay report: %v", err)
		}
		encoded, err := Marshal(report, NewDetailsRegistry())
		if err != nil {
			t.Fatalf("marshal replay report: %v", err)
		}
		for _, prohibited := range []string{"postgres://", "databaseUrl", "connection", "SELECT ", "migrations applied", "target-environment", "same-commit-release"} {
			if strings.Contains(string(encoded), prohibited) {
				t.Fatalf("replay report contains prohibited input or claim %q", prohibited)
			}
		}
		var decoded map[string]any
		if err := json.Unmarshal(report.Evidence.Details, &decoded); err != nil {
			t.Fatalf("decode replay details: %v", err)
		}
		if len(decoded) != 10 {
			t.Fatalf("replay details field count = %d, want 10: %#v", len(decoded), decoded)
		}
	})

	t.Run("closed registry rejects static ledger arbitrary and identity-bearing details", func(t *testing.T) {
		registry := NewDetailsRegistry()
		raw, err := registry.MarshalDetails(MigrationReplaySurfaceID, details)
		if err != nil {
			t.Fatalf("marshal replay details: %v", err)
		}
		for surface, candidate := range map[string]json.RawMessage{
			MigrationStaticSurfaceID: raw,
			MigrationLedgerSurfaceID: raw,
			MigrationReplaySurfaceID: append(append([]byte(nil), raw[:len(raw)-1]...), []byte(`,"releaseIdentity":{"releaseCommit":"forged"}}`)...),
		} {
			if err := registry.ValidateDetails(surface, candidate); !IsCode(err, ErrorSurfaceSchemaInvalid) {
				t.Fatalf("surface %q accepted substituted details: %v", surface, err)
			}
		}
		if _, err := registry.MarshalDetails(MigrationReplaySurfaceID, map[string]any{"initialLedgerRows": 0}); !IsCode(err, ErrorSurfaceSchemaInvalid) {
			t.Fatalf("replay accepted arbitrary details: %v", err)
		}
	})

	t.Run("reused partial non-noop digest and identity splices fail derivation", func(t *testing.T) {
		mutations := []struct {
			name string
			edit func(*MigrationLedgerSnapshot, *MigrationLedgerSnapshot, *MigrationLedgerSnapshot)
		}{
			{"reused database", func(value, _, _ *MigrationLedgerSnapshot) { *value = after }},
			{"partial first apply", func(_, value, _ *MigrationLedgerSnapshot) {
				value.Identities = value.Identities[:len(value.Identities)-1]
				value.IdentityDigest, _ = migrations.IdentityDigest(value.Identities)
			}},
			{"non noop second apply", func(_, _, value *MigrationLedgerSnapshot) {
				value.Identities = value.Identities[:len(value.Identities)-1]
				value.IdentityDigest, _ = migrations.IdentityDigest(value.Identities)
			}},
			{"snapshot digest", func(_, value, _ *MigrationLedgerSnapshot) { value.IdentityDigest = "sha256:" + strings.Repeat("0", 64) }},
			{"identity checksum splice", func(_, value, _ *MigrationLedgerSnapshot) { value.Identities[0].Checksum = strings.Repeat("f", 64) }},
			{"identity order", func(_, value, _ *MigrationLedgerSnapshot) {
				value.Identities[0], value.Identities[1] = value.Identities[1], value.Identities[0]
			}},
		}
		for _, mutation := range mutations {
			t.Run(mutation.name, func(t *testing.T) {
				candidateBefore := cloneMigrationSnapshot(before)
				candidateFirst := cloneMigrationSnapshot(after)
				candidateSecond := cloneMigrationSnapshot(after)
				mutation.edit(&candidateBefore, &candidateFirst, &candidateSecond)
				if _, err := DeriveReplayObservation(inventory.Identities, candidateBefore, candidateFirst, candidateSecond); err == nil {
					t.Fatalf("replay mutation %q was accepted", mutation.name)
				}
			})
		}
	})

	t.Run("count mode cleanup digest outcome identity profile and decoded splices fail", func(t *testing.T) {
		mutations := []struct {
			name string
			edit func(*MigrationReplayDetails)
		}{
			{"initial rows", func(value *MigrationReplayDetails) { value.InitialLedgerRows = 1 }},
			{"first applied", func(value *MigrationReplayDetails) { value.FirstApply.Applied-- }},
			{"first skipped", func(value *MigrationReplayDetails) { value.FirstApply.Skipped = 1 }},
			{"second applied", func(value *MigrationReplayDetails) { value.SecondApply.Applied = 1 }},
			{"second skipped", func(value *MigrationReplayDetails) { value.SecondApply.Skipped-- }},
			{"final rows", func(value *MigrationReplayDetails) { value.FinalLedgerRows-- }},
			{"static digest", func(value *MigrationReplayDetails) { value.StaticDigest = "sha256:" + strings.Repeat("0", 64) }},
			{"mode", func(value *MigrationReplayDetails) { value.ReplayMode = "shared" }},
			{"missing ownership", func(value *MigrationReplayDetails) { value.ResourceOwnership = "" }},
			{"unknown ownership", func(value *MigrationReplayDetails) { value.ResourceOwnership = "forged" }},
			{"caller owned pass", func(value *MigrationReplayDetails) { value.ResourceOwnership = migrationResourceOwnershipCallerOwned }},
			{"cleanup", func(value *MigrationReplayDetails) { value.CleanupResult = migrationCleanupFailed }},
		}
		for _, mutation := range mutations {
			t.Run(mutation.name, func(t *testing.T) {
				candidate := details
				mutation.edit(&candidate)
				if _, err := NewMigrationReplayReport(context.Background(), identityProvider, profileResolver, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", candidate, passing); err == nil {
					t.Fatalf("replay detail mutation %q was accepted", mutation.name)
				}
			})
		}

		withSkip := passing
		withSkip.SkippedChecks = []string{"database"}
		if _, err := NewMigrationReplayReport(context.Background(), identityProvider, profileResolver, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", details, withSkip); err == nil {
			t.Fatal("replay accepted skipped checks")
		}
		forged := identity
		forged.EvidenceClass = "target-environment"
		if _, err := NewMigrationReplayReport(context.Background(), migrationIdentityProvider{identity: forged}, profileResolver, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", details, passing); err == nil {
			t.Fatal("replay accepted E3 identity")
		}
		wrongProfile := profile
		wrongProfile.ID = "microservices"
		if _, err := NewMigrationReplayReport(context.Background(), identityProvider, migrationProfileResolver{profile: wrongProfile}, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", details, passing); err == nil {
			t.Fatal("replay accepted profile substitution")
		}

		report, err := NewMigrationReplayReport(context.Background(), identityProvider, profileResolver, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", details, passing)
		if err != nil {
			t.Fatalf("construct splice fixture: %v", err)
		}
		for _, mutation := range []struct {
			name string
			edit func(*SurfaceReportV1)
		}{
			{"surface", func(value *SurfaceReportV1) { value.SurfaceIdentity.Surface = MigrationLedgerSurfaceID }},
			{"consumer", func(value *SurfaceReportV1) { value.SurfaceIdentity.Consumer = migrationLedgerConsumer }},
			{"mode", func(value *SurfaceReportV1) { value.Evidence.Mode = migrationLedgerMode }},
		} {
			candidate := report
			mutation.edit(&candidate)
			if err := Validate(candidate, NewDetailsRegistry()); err == nil {
				t.Fatalf("replay decoded envelope splice %q was accepted", mutation.name)
			}
		}
	})

	t.Run("unavailable observation writes only an exact explicit failure report shape", func(t *testing.T) {
		unknown := MigrationApplyCounts{Applied: -1, Skipped: -1}
		failureDetails := MigrationReplayDetails{
			DatabaseKind: migrationDatabaseKind, ReplayMode: migrationReplayExternal, InitialLedgerRows: -1,
			FirstApply: unknown, SecondApply: unknown, FinalLedgerRows: -1,
			StaticDigest: inventory.IdentityDigest, LedgerDigest: inventory.IdentityDigest,
			ResourceOwnership: migrationResourceOwnershipCallerOwned, CleanupResult: migrationCleanupFailed,
		}
		failure := Outcome{Result: ResultFail, ErrorCodes: []string{MigrationReplayUnavailableCode}, SkippedChecks: []string{}}
		report, err := NewMigrationReplayReport(context.Background(), identityProvider, profileResolver, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", failureDetails, failure)
		if err != nil {
			t.Fatalf("construct unavailable report: %v", err)
		}
		if report.Outcome.Result != ResultFail || !reflect.DeepEqual(report.Outcome.ErrorCodes, []string{MigrationReplayUnavailableCode}) || report.Evidence.Environment != "external-isolated-database" {
			t.Fatalf("unexpected unavailable report: %#v", report)
		}
		if err := Validate(report, NewDetailsRegistry()); err != nil {
			t.Fatalf("validate unavailable report: %v", err)
		}
		if _, err := NewMigrationReplayReport(context.Background(), identityProvider, profileResolver, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", failureDetails, passing); err == nil {
			t.Fatal("unavailable details were accepted as pass")
		}
		for _, ownership := range []string{migrationResourceOwnershipCallerOwned, migrationResourceOwnershipOwnedDisposable} {
			candidate := failureDetails
			candidate.ResourceOwnership = ownership
			if _, err := NewMigrationReplayReport(context.Background(), identityProvider, profileResolver, repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json", "monolith", candidate, failure); err != nil {
				t.Fatalf("unavailable ownership %q was rejected: %v", ownership, err)
			}
		}
	})
}

func cloneMigrationSnapshot(source MigrationLedgerSnapshot) MigrationLedgerSnapshot {
	return MigrationLedgerSnapshot{Identities: append([]migrations.MigrationIdentity(nil), source.Identities...), IdentityDigest: source.IdentityDigest}
}

type migrationIdentityProvider struct {
	identity buildinfo.BuildIdentityV1
	err      error
}

func (p migrationIdentityProvider) Resolve(context.Context, string, string, string) (buildinfo.BuildIdentityV1, error) {
	return p.identity, p.err
}

type migrationProfileResolver struct {
	profile releasecontract.DeploymentProfile
	err     error
}

func (r migrationProfileResolver) ResolveCommittedProfile(context.Context, string, string, string, string) (releasecontract.DeploymentProfile, error) {
	return r.profile, r.err
}

func migrationReportAuthority(t *testing.T) (string, releasecontract.DeploymentProfile, buildinfo.BuildIdentityV1) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration report source")
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
		ReleaseCommit: strings.Repeat("c", 40), SourceTree: strings.Repeat("d", 40),
		ContractDigest: digest, Dirty: false, EvidenceClass: buildinfo.EvidenceRepositoryLocal,
	}
}

func cloneStaticInventory(source migrations.StaticInventory) migrations.StaticInventory {
	cloned := source
	cloned.Identities = append([]migrations.MigrationIdentity(nil), source.Identities...)
	cloned.Entries = append([]migrations.StaticMigrationEntry(nil), source.Entries...)
	cloned.NonMonolithDispositionCounts = cloneStringIntMap(source.NonMonolithDispositionCounts)
	return cloned
}
