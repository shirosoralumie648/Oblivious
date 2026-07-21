package surfacereport

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/migrations"
	"oblivious/server/internal/releasecontract"
)

const (
	MigrationStaticSurfaceID = "migration-static"
	MigrationLedgerSurfaceID = "migration-ledger"
	MigrationReplaySurfaceID = "migration-replay"

	MigrationReplayUnavailableCode = "migration_replay_unavailable"

	migrationDatabaseKind      = "postgresql-pgvector"
	migrationStaticSource      = "src/server/migrations"
	migrationStaticConsumer    = "monolith-migration-static-inventory"
	migrationLedgerSource      = "schema_migrations(version,checksum)"
	migrationLedgerConsumer    = "monolith-runtime-ledger"
	migrationReplaySource      = "src/server/migrations+schema_migrations(version,checksum)"
	migrationReplayConsumer    = "monolith-migration-replay"
	migrationSurfaceVersion    = "v1"
	migrationStaticEnvironment = "repository"
	migrationLedgerEnvironment = "repository-local-database"
	migrationStaticMode        = "static"
	migrationLedgerMode        = "ledger"
	migrationReplayMode        = "replay"
	migrationReplayExternal    = "external-isolated"
	migrationReplayDocker      = "docker-ephemeral"
	migrationCleanupSucceeded  = "succeeded"
	migrationCleanupFailed     = "failed"
	migrationUnavailableCount  = -1

	// SHA-256 of the canonical JSON empty identity sequence (`[]`). Non-empty
	// snapshots continue to use migrations.IdentityDigest directly.
	migrationEmptyIdentityDigest = "sha256:4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945"
)

type MigrationStaticDetails struct {
	DatabaseKind                 string         `json:"databaseKind"`
	IdentityCount                int            `json:"identityCount"`
	FileCount                    int            `json:"fileCount"`
	IdentityDigest               string         `json:"identityDigest"`
	StaticMetadataDigest         string         `json:"staticMetadataDigest"`
	NonMonolithDispositionCounts map[string]int `json:"nonMonolithDispositionCounts"`
}

type MigrationLedgerDetails struct {
	DatabaseKind   string `json:"databaseKind"`
	RowCount       int    `json:"rowCount"`
	IdentityDigest string `json:"identityDigest"`
	MatchesStatic  bool   `json:"matchesStatic"`
}

type MigrationApplyCounts struct {
	Applied int `json:"applied"`
	Skipped int `json:"skipped"`
}

type MigrationReplayDetails struct {
	DatabaseKind      string               `json:"databaseKind"`
	ReplayMode        string               `json:"replayMode"`
	InitialLedgerRows int                  `json:"initialLedgerRows"`
	FirstApply        MigrationApplyCounts `json:"firstApply"`
	SecondApply       MigrationApplyCounts `json:"secondApply"`
	FinalLedgerRows   int                  `json:"finalLedgerRows"`
	StaticDigest      string               `json:"staticDigest"`
	LedgerDigest      string               `json:"ledgerDigest"`
	CleanupResult     string               `json:"cleanupResult"`
}

type MigrationLedgerSnapshot struct {
	Identities     []migrations.MigrationIdentity `json:"identities"`
	IdentityDigest string                         `json:"identityDigest"`
}

func RegisterMigrationDetails(registry *DetailsRegistry) error {
	if err := RegisterDetails(registry, MigrationStaticSurfaceID, validateMigrationStaticDetails); err != nil {
		return err
	}
	if err := RegisterDetails(registry, MigrationLedgerSurfaceID, validateMigrationLedgerDetails); err != nil {
		return err
	}
	return RegisterDetails(registry, MigrationReplaySurfaceID, validateMigrationReplayDetails)
}

func NewMigrationStaticReport(
	ctx context.Context,
	identities buildinfo.IdentityProvider,
	profiles releasecontract.ProfileResolver,
	repoRoot, contractPath, schemaPath, profileID string,
	inventory migrations.StaticInventory,
	outcome Outcome,
) (SurfaceReportV1, error) {
	if err := validatePassingMigrationOutcome(outcome); err != nil {
		return SurfaceReportV1{}, err
	}
	if err := validateStaticInventory(inventory); err != nil {
		return SurfaceReportV1{}, reportError("evidence.details", err)
	}
	details := MigrationStaticDetails{
		DatabaseKind: migrationDatabaseKind, IdentityCount: len(inventory.Identities),
		FileCount: inventory.MonolithFileCount, IdentityDigest: inventory.IdentityDigest,
		StaticMetadataDigest:         inventory.StaticMetadataDigest,
		NonMonolithDispositionCounts: cloneStringIntMap(inventory.NonMonolithDispositionCounts),
	}
	return newMigrationReport(
		ctx, identities, profiles, repoRoot, contractPath, schemaPath, profileID,
		MigrationStaticSurfaceID, migrationStaticSource, migrationStaticConsumer,
		inventory.StaticMetadataDigest, inventory.IdentityDigest,
		migrationStaticEnvironment, migrationStaticMode, details, outcome,
	)
}

func NewMigrationLedgerReport(
	ctx context.Context,
	identities buildinfo.IdentityProvider,
	profiles releasecontract.ProfileResolver,
	repoRoot, contractPath, schemaPath, profileID string,
	static migrations.StaticInventory,
	ledger []migrations.MigrationIdentity,
	outcome Outcome,
) (SurfaceReportV1, error) {
	if err := validatePassingMigrationOutcome(outcome); err != nil {
		return SurfaceReportV1{}, err
	}
	if err := validateStaticInventory(static); err != nil {
		return SurfaceReportV1{}, reportError("evidence.details.static", err)
	}
	ledgerDigest, err := migrations.IdentityDigest(ledger)
	if err != nil {
		return SurfaceReportV1{}, reportError("evidence.details.ledger", err)
	}
	if !reflect.DeepEqual(static.Identities, ledger) || static.IdentityDigest != ledgerDigest {
		return SurfaceReportV1{}, reportError("drift.incompatible", fmt.Errorf("migration identity mismatch"))
	}
	details := MigrationLedgerDetails{
		DatabaseKind: migrationDatabaseKind, RowCount: len(ledger),
		IdentityDigest: ledgerDigest, MatchesStatic: true,
	}
	return newMigrationReport(
		ctx, identities, profiles, repoRoot, contractPath, schemaPath, profileID,
		MigrationLedgerSurfaceID, migrationLedgerSource, migrationLedgerConsumer,
		ledgerDigest, static.IdentityDigest,
		migrationLedgerEnvironment, migrationLedgerMode, details, outcome,
	)
}

// DeriveReplayObservation derives apply/skip counts only from exact ledger
// identity transitions. Human migration output is deliberately not an input.
func DeriveReplayObservation(
	static []migrations.MigrationIdentity,
	before, afterFirst, afterSecond MigrationLedgerSnapshot,
) (MigrationReplayDetails, error) {
	staticDigest, err := migrations.IdentityDigest(static)
	if err != nil {
		return MigrationReplayDetails{}, reportError("evidence.details.static", err)
	}
	for _, candidate := range []struct {
		name     string
		snapshot MigrationLedgerSnapshot
	}{{"before", before}, {"afterFirst", afterFirst}, {"afterSecond", afterSecond}} {
		name, snapshot := candidate.name, candidate.snapshot
		if err := validateMigrationLedgerSnapshot(snapshot); err != nil {
			return MigrationReplayDetails{}, reportError("evidence.details."+name, err)
		}
	}
	if len(before.Identities) != 0 {
		return MigrationReplayDetails{}, reportError("evidence.details.initialLedgerRows", nil)
	}
	if !reflect.DeepEqual(afterFirst.Identities, static) || afterFirst.IdentityDigest != staticDigest {
		return MigrationReplayDetails{}, reportError("evidence.details.firstApply", nil)
	}
	if !reflect.DeepEqual(afterSecond.Identities, static) || afterSecond.IdentityDigest != staticDigest {
		return MigrationReplayDetails{}, reportError("evidence.details.secondApply", nil)
	}

	firstApply, err := deriveMigrationApplyCounts(static, before.Identities, afterFirst.Identities)
	if err != nil {
		return MigrationReplayDetails{}, reportError("evidence.details.firstApply", err)
	}
	secondApply, err := deriveMigrationApplyCounts(static, afterFirst.Identities, afterSecond.Identities)
	if err != nil {
		return MigrationReplayDetails{}, reportError("evidence.details.secondApply", err)
	}
	return MigrationReplayDetails{
		DatabaseKind: migrationDatabaseKind, InitialLedgerRows: len(before.Identities),
		FirstApply: firstApply, SecondApply: secondApply,
		FinalLedgerRows: len(afterSecond.Identities), StaticDigest: staticDigest,
		LedgerDigest: afterSecond.IdentityDigest,
	}, nil
}

func NewMigrationReplayReport(
	ctx context.Context,
	identities buildinfo.IdentityProvider,
	profiles releasecontract.ProfileResolver,
	repoRoot, contractPath, schemaPath, profileID string,
	details MigrationReplayDetails,
	outcome Outcome,
) (SurfaceReportV1, error) {
	if err := validateMigrationReplayDetails(details); err != nil {
		return SurfaceReportV1{}, reportError("evidence.details", err)
	}
	if err := validateMigrationReplayOutcome(details, outcome); err != nil {
		return SurfaceReportV1{}, err
	}
	environment := "external-isolated-database"
	if details.ReplayMode == migrationReplayDocker {
		environment = "local-docker"
	}
	return newMigrationReport(
		ctx, identities, profiles, repoRoot, contractPath, schemaPath, profileID,
		MigrationReplaySurfaceID, migrationReplaySource, migrationReplayConsumer,
		details.StaticDigest, details.LedgerDigest,
		environment, migrationReplayMode, details, outcome,
	)
}

func newMigrationReport(
	ctx context.Context,
	identities buildinfo.IdentityProvider,
	profiles releasecontract.ProfileResolver,
	repoRoot, contractPath, schemaPath, profileID,
	surfaceID, canonicalSource, consumer, sourceDigest, consumerDigest,
	environment, mode string,
	details any,
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
	registry := NewDetailsRegistry()
	rawDetails, err := registry.MarshalDetails(surfaceID, details)
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
			Surface: surfaceID, CanonicalSource: canonicalSource, Consumer: consumer,
			Version: migrationSurfaceVersion, SourceDigest: sourceDigest, ConsumerDigest: consumerDigest,
		},
		Drift{Missing: []string{}, Extra: []string{}, Incompatible: []string{}},
		Evidence{
			Class: identity.EvidenceClass, Environment: environment, Mode: mode,
			CheckedAt:    time.Now().UTC().Format(time.RFC3339Nano),
			ToolVersions: map[string]string{}, Details: rawDetails,
		},
		outcome,
	)
	if err := Validate(report, registry); err != nil {
		return SurfaceReportV1{}, err
	}
	return report, nil
}

func validateMigrationStaticDetails(details MigrationStaticDetails) error {
	if details.DatabaseKind != migrationDatabaseKind || details.IdentityCount <= 0 || details.FileCount <= 0 || details.IdentityCount != details.FileCount {
		return fmt.Errorf("migration static counts are invalid")
	}
	if !validDigest(details.IdentityDigest) || !validDigest(details.StaticMetadataDigest) {
		return fmt.Errorf("migration static digest is invalid")
	}
	if !validNonMonolithCounts(details.NonMonolithDispositionCounts) {
		return fmt.Errorf("migration non-monolith disposition is incomplete")
	}
	return nil
}

func validateMigrationLedgerDetails(details MigrationLedgerDetails) error {
	if details.DatabaseKind != migrationDatabaseKind || details.RowCount <= 0 || !validDigest(details.IdentityDigest) || !details.MatchesStatic {
		return fmt.Errorf("migration ledger comparison is invalid")
	}
	return nil
}

func validateMigrationReplayDetails(details MigrationReplayDetails) error {
	if details.DatabaseKind != migrationDatabaseKind || !validMigrationReplayMode(details.ReplayMode) ||
		!validDigest(details.StaticDigest) || !validDigest(details.LedgerDigest) ||
		(details.CleanupResult != migrationCleanupSucceeded && details.CleanupResult != migrationCleanupFailed) {
		return fmt.Errorf("migration replay identity is invalid")
	}
	if migrationReplayPassingDetails(details) || migrationReplayUnavailableDetails(details) {
		return nil
	}
	return fmt.Errorf("migration replay counts are invalid")
}

func validatePassingMigrationOutcome(outcome Outcome) error {
	if outcome.Result != ResultPass || len(outcome.ErrorCodes) != 0 || len(outcome.SkippedChecks) != 0 {
		return reportError("outcome", nil)
	}
	return nil
}

func validateMigrationReplayOutcome(details MigrationReplayDetails, outcome Outcome) error {
	if len(outcome.SkippedChecks) != 0 {
		return reportError("outcome.skippedChecks", nil)
	}
	if migrationReplayPassingDetails(details) {
		return validatePassingMigrationOutcome(outcome)
	}
	if outcome.Result != ResultFail || !reflect.DeepEqual(outcome.ErrorCodes, []string{MigrationReplayUnavailableCode}) {
		return reportError("outcome", nil)
	}
	return nil
}

func migrationReplayPassingDetails(details MigrationReplayDetails) bool {
	return details.InitialLedgerRows == 0 && details.FirstApply.Applied > 0 && details.FirstApply.Skipped == 0 &&
		details.SecondApply.Applied == 0 && details.SecondApply.Skipped == details.FirstApply.Applied &&
		details.FinalLedgerRows == details.FirstApply.Applied && details.StaticDigest == details.LedgerDigest &&
		details.CleanupResult == migrationCleanupSucceeded
}

func migrationReplayUnavailableDetails(details MigrationReplayDetails) bool {
	return details.InitialLedgerRows == migrationUnavailableCount &&
		details.FirstApply == (MigrationApplyCounts{Applied: migrationUnavailableCount, Skipped: migrationUnavailableCount}) &&
		details.SecondApply == (MigrationApplyCounts{Applied: migrationUnavailableCount, Skipped: migrationUnavailableCount}) &&
		details.FinalLedgerRows == migrationUnavailableCount && details.StaticDigest == details.LedgerDigest
}

func validMigrationReplayMode(mode string) bool {
	return mode == migrationReplayExternal || mode == migrationReplayDocker
}

func validateMigrationLedgerSnapshot(snapshot MigrationLedgerSnapshot) error {
	if snapshot.Identities == nil {
		return fmt.Errorf("migration ledger identities are missing")
	}
	expectedDigest := migrationEmptyIdentityDigest
	if len(snapshot.Identities) != 0 {
		var err error
		expectedDigest, err = migrations.IdentityDigest(snapshot.Identities)
		if err != nil {
			return err
		}
	}
	if snapshot.IdentityDigest != expectedDigest {
		return fmt.Errorf("migration ledger snapshot digest mismatch")
	}
	return nil
}

func deriveMigrationApplyCounts(static, before, after []migrations.MigrationIdentity) (MigrationApplyCounts, error) {
	if !reflect.DeepEqual(after, static) {
		return MigrationApplyCounts{}, fmt.Errorf("migration ledger is incomplete")
	}
	beforeSet := make(map[migrations.MigrationIdentity]struct{}, len(before))
	for _, identity := range before {
		beforeSet[identity] = struct{}{}
	}
	counts := MigrationApplyCounts{}
	for _, identity := range static {
		if _, exists := beforeSet[identity]; exists {
			counts.Skipped++
		} else {
			counts.Applied++
		}
	}
	if counts.Applied+counts.Skipped != len(static) {
		return MigrationApplyCounts{}, fmt.Errorf("migration apply counts are incomplete")
	}
	return counts, nil
}

func validateStaticInventory(inventory migrations.StaticInventory) error {
	if inventory.MonolithFileCount <= 0 || inventory.MonolithFileCount != len(inventory.Identities) || len(inventory.Entries) <= inventory.MonolithFileCount {
		return fmt.Errorf("migration inventory counts are invalid")
	}
	identityDigest, err := migrations.IdentityDigest(inventory.Identities)
	if err != nil || identityDigest != inventory.IdentityDigest {
		return fmt.Errorf("migration inventory identity digest mismatch")
	}
	metadataDigest, err := migrations.StaticMetadataDigest(inventory.Entries)
	if err != nil || metadataDigest != inventory.StaticMetadataDigest {
		return fmt.Errorf("migration inventory metadata digest mismatch")
	}
	if !validNonMonolithCounts(inventory.NonMonolithDispositionCounts) {
		return fmt.Errorf("migration inventory non-monolith disposition is incomplete")
	}

	identityEntries := make(map[string]migrations.MigrationIdentity, len(inventory.Identities))
	computedNonMonolith := map[string]int{}
	monolithEntries := 0
	previousFilename := ""
	for _, entry := range inventory.Entries {
		if previousFilename != "" && previousFilename >= entry.Filename {
			return fmt.Errorf("migration inventory entries are not canonical")
		}
		previousFilename = entry.Filename
		switch entry.Disposition {
		case migrations.DispositionMonolithManaged:
			monolithEntries++
			identityEntries[entry.Identity.Version] = entry.Identity
		case migrations.DispositionMicroserviceExcluded, migrations.DispositionClickHouseExcluded:
			computedNonMonolith[entry.Disposition]++
		default:
			return fmt.Errorf("migration inventory disposition is invalid")
		}
	}
	if !reflect.DeepEqual(computedNonMonolith, inventory.NonMonolithDispositionCounts) {
		return fmt.Errorf("migration non-monolith counts mismatch")
	}
	for _, identity := range inventory.Identities {
		if got, ok := identityEntries[identity.Version]; !ok || got != identity {
			return fmt.Errorf("migration monolith entry mismatch")
		}
	}
	if monolithEntries != len(inventory.Identities) || len(identityEntries) != len(inventory.Identities) {
		return fmt.Errorf("migration monolith entry count mismatch")
	}
	return nil
}

func validateMigrationReport(report SurfaceReportV1) error {
	if len(report.Evidence.ToolVersions) != 0 || report.SurfaceIdentity.Version != migrationSurfaceVersion {
		return reportError("evidence", nil)
	}
	switch report.SurfaceIdentity.Surface {
	case MigrationStaticSurfaceID:
		var details MigrationStaticDetails
		if err := json.Unmarshal(report.Evidence.Details, &details); err != nil {
			return reportError("evidence.details", err)
		}
		if report.SurfaceIdentity.CanonicalSource != migrationStaticSource || report.SurfaceIdentity.Consumer != migrationStaticConsumer ||
			report.SurfaceIdentity.SourceDigest != details.StaticMetadataDigest || report.SurfaceIdentity.ConsumerDigest != details.IdentityDigest ||
			report.Evidence.Environment != migrationStaticEnvironment || report.Evidence.Mode != migrationStaticMode {
			return reportError("surfaceIdentity", nil)
		}
	case MigrationLedgerSurfaceID:
		var details MigrationLedgerDetails
		if err := json.Unmarshal(report.Evidence.Details, &details); err != nil {
			return reportError("evidence.details", err)
		}
		if report.SurfaceIdentity.CanonicalSource != migrationLedgerSource || report.SurfaceIdentity.Consumer != migrationLedgerConsumer ||
			report.SurfaceIdentity.SourceDigest != details.IdentityDigest || report.SurfaceIdentity.ConsumerDigest != details.IdentityDigest ||
			report.Evidence.Environment != migrationLedgerEnvironment || report.Evidence.Mode != migrationLedgerMode {
			return reportError("surfaceIdentity", nil)
		}
	case MigrationReplaySurfaceID:
		var details MigrationReplayDetails
		if err := json.Unmarshal(report.Evidence.Details, &details); err != nil {
			return reportError("evidence.details", err)
		}
		expectedEnvironment := "external-isolated-database"
		if details.ReplayMode == migrationReplayDocker {
			expectedEnvironment = "local-docker"
		}
		if report.SurfaceIdentity.CanonicalSource != migrationReplaySource || report.SurfaceIdentity.Consumer != migrationReplayConsumer ||
			report.SurfaceIdentity.SourceDigest != details.StaticDigest || report.SurfaceIdentity.ConsumerDigest != details.LedgerDigest ||
			report.Evidence.Environment != expectedEnvironment || report.Evidence.Mode != migrationReplayMode {
			return reportError("surfaceIdentity", nil)
		}
		if err := validateMigrationReplayOutcome(details, report.Outcome); err != nil {
			return err
		}
	default:
		return reportError("surfaceIdentity.surface", nil)
	}
	return nil
}

func validNonMonolithCounts(counts map[string]int) bool {
	if len(counts) != 2 || counts[migrations.DispositionMicroserviceExcluded] <= 0 || counts[migrations.DispositionClickHouseExcluded] <= 0 {
		return false
	}
	for key, count := range counts {
		if count <= 0 || (key != migrations.DispositionMicroserviceExcluded && key != migrations.DispositionClickHouseExcluded) {
			return false
		}
	}
	return true
}

func cloneStringIntMap(source map[string]int) map[string]int {
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	cloned := make(map[string]int, len(source))
	for _, key := range keys {
		cloned[key] = source[key]
	}
	return cloned
}
