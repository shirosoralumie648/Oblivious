package surfacereport

import (
	"context"
	"encoding/json"
	"path/filepath"
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
