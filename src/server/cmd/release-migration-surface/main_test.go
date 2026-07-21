package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/migrations"
	"oblivious/server/internal/releasecontract"
	"oblivious/server/internal/surfacereport"
)

func TestReleaseMigrationStaticAndLedgerCommandsContract(t *testing.T) {
	t.Run("static writes one trusted report without database access", func(t *testing.T) {
		deps := migrationCommandTestDependencies(t)
		output := filepath.Join(t.TempDir(), "nested", "migration-static.json")
		writer := &recordingMigrationReportWriter{delegate: surfacereport.NewAtomicWriter()}
		deps.reportWriter = writer
		deps.lookupEnv = nil
		deps.openDatabase = nil
		deps.pingDatabase = nil

		stdout, stderr, exitCode := runMigrationCommand(t, "static", output, "", deps)
		if exitCode != 0 {
			t.Fatalf("static exit=%d stderr=%s", exitCode, stderr)
		}
		if writer.calls != 1 || writer.report.SurfaceIdentity.Surface != surfacereport.MigrationStaticSurfaceID {
			t.Fatalf("static writer calls/report = %d/%#v", writer.calls, writer.report.SurfaceIdentity)
		}
		assertMigrationReportFile(t, output, surfacereport.MigrationStaticSurfaceID)
		if strings.Contains(stdout, output) || strings.Contains(stdout, "/test/repo") {
			t.Fatalf("static stdout exposed external path: %s", stdout)
		}
		assertNoMigrationSecret(t, stdout, stderr, writer.report)
	})

	t.Run("ledger reads named environment and writes one exact report", func(t *testing.T) {
		deps := migrationCommandTestDependencies(t)
		output := filepath.Join(t.TempDir(), "migration-ledger.json")
		writer := &recordingMigrationReportWriter{delegate: surfacereport.NewAtomicWriter()}
		deps.reportWriter = writer
		secret := "postgres://release:super-secret@127.0.0.1:5432/oblivious"
		deps.lookupEnv = func(name string) (string, bool) {
			if name != "MIGRATION_TEST_DATABASE_URL" {
				t.Fatalf("lookup env name = %q", name)
			}
			return secret, true
		}
		database, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		if err != nil {
			t.Fatalf("sqlmock.New: %v", err)
		}
		defer database.Close()
		mock.ExpectPing()
		mock.ExpectQuery(regexp.QuoteMeta("SELECT version, checksum FROM schema_migrations ORDER BY version")).
			WillReturnRows(ledgerRows(testMigrationInventory(t).Identities))
		deps.openDatabase = func(driverName, dataSourceName string) (*sql.DB, error) {
			if driverName != "postgres" || dataSourceName != secret {
				t.Fatalf("open database = %q %q", driverName, dataSourceName)
			}
			return database, nil
		}

		stdout, stderr, exitCode := runMigrationCommand(t, "ledger", output, "MIGRATION_TEST_DATABASE_URL", deps)
		if exitCode != 0 {
			t.Fatalf("ledger exit=%d stderr=%s", exitCode, stderr)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("database expectations: %v", err)
		}
		if writer.calls != 1 || writer.report.SurfaceIdentity.Surface != surfacereport.MigrationLedgerSurfaceID {
			t.Fatalf("ledger writer calls/report = %d/%#v", writer.calls, writer.report.SurfaceIdentity)
		}
		content := assertMigrationReportFile(t, output, surfacereport.MigrationLedgerSurfaceID)
		for _, prohibited := range []string{"filename", "disposition", "postgres://", "super-secret"} {
			if bytes.Contains(bytes.ToLower(content), []byte(prohibited)) {
				t.Fatalf("ledger report contains %q: %s", prohibited, content)
			}
		}
		assertNoMigrationSecret(t, stdout, stderr, writer.report)
	})

	t.Run("required selectors and authority overrides fail closed", func(t *testing.T) {
		deps := migrationCommandTestDependencies(t)
		common := migrationCommandArgs("static", filepath.Join(t.TempDir(), "report.json"), "")
		for _, test := range []struct {
			name string
			args []string
		}{
			{name: "missing subcommand", args: nil},
			{name: "missing repo", args: removeMigrationFlag(common, "--repo")},
			{name: "missing contract", args: removeMigrationFlag(common, "--contract")},
			{name: "missing schema", args: removeMigrationFlag(common, "--schema")},
			{name: "missing profile", args: removeMigrationFlag(common, "--profile")},
			{name: "missing output", args: removeMigrationFlag(common, "--output")},
			{name: "literal database url", args: append(migrationCommandArgs("ledger", filepath.Join(t.TempDir(), "report.json"), "MIGRATION_TEST_DATABASE_URL"), "--database-url", "postgres://secret")},
			{name: "release identity override", args: append(common, "--release-commit", strings.Repeat("f", 40))},
			{name: "skip override", args: append(common, "--skip", "database")},
			{name: "evidence override", args: append(common, "--evidence-class", "target")},
		} {
			t.Run(test.name, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				if exitCode := runWithDependencies(context.Background(), test.args, &stdout, &stderr, deps); exitCode == 0 {
					t.Fatalf("invalid invocation passed: %v", test.args)
				}
				if strings.Contains(stdout.String()+stderr.String(), "postgres://secret") {
					t.Fatalf("literal DSN was emitted: %s%s", stdout.String(), stderr.String())
				}
			})
		}
	})

	t.Run("ledger mismatch classes fail before report output", func(t *testing.T) {
		inventory := testMigrationInventory(t)
		checksum := inventory.Identities[0].Checksum
		cases := []struct {
			name string
			rows []migrations.MigrationIdentity
		}{
			{name: "missing", rows: inventory.Identities[:1]},
			{name: "extra", rows: append(append([]migrations.MigrationIdentity{}, inventory.Identities...), migrations.MigrationIdentity{Version: "0003_extra.sql", Checksum: checksum})},
			{name: "checksum", rows: []migrations.MigrationIdentity{{Version: inventory.Identities[0].Version, Checksum: strings.Repeat("f", 64)}, inventory.Identities[1]}},
			{name: "order", rows: []migrations.MigrationIdentity{inventory.Identities[1], inventory.Identities[0]}},
			{name: "duplicate", rows: []migrations.MigrationIdentity{inventory.Identities[0], inventory.Identities[0]}},
			{name: "zero rows", rows: []migrations.MigrationIdentity{}},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				deps := migrationCommandTestDependencies(t)
				database, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
				if err != nil {
					t.Fatalf("sqlmock.New: %v", err)
				}
				defer database.Close()
				mock.ExpectPing()
				mock.ExpectQuery(regexp.QuoteMeta("SELECT version, checksum FROM schema_migrations ORDER BY version")).
					WillReturnRows(ledgerRows(test.rows))
				deps.openDatabase = func(string, string) (*sql.DB, error) { return database, nil }
				output := filepath.Join(t.TempDir(), "ledger.json")
				stdout, stderr, exitCode := runMigrationCommand(t, "ledger", output, "MIGRATION_TEST_DATABASE_URL", deps)
				if exitCode == 0 {
					t.Fatalf("ledger %s passed: stdout=%s", test.name, stdout)
				}
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Fatalf("database expectations: %v", err)
				}
				if _, err := filepath.Abs(output); err != nil {
					t.Fatal(err)
				}
				if assertFileExists(output) {
					t.Fatalf("ledger %s created output", test.name)
				}
				assertNoMigrationSecret(t, stdout, stderr, surfacereport.SurfaceReportV1{})
			})
		}
	})

	t.Run("database unavailable and output failures are sanitized", func(t *testing.T) {
		secret := "postgres://release:do-not-print@127.0.0.1:5432/oblivious"
		for _, test := range []struct {
			name   string
			mutate func(*dependencies)
		}{
			{name: "environment absent", mutate: func(deps *dependencies) { deps.lookupEnv = func(string) (string, bool) { return "", false } }},
			{name: "open fails", mutate: func(deps *dependencies) {
				deps.openDatabase = func(string, string) (*sql.DB, error) { return nil, errors.New(secret) }
			}},
			{name: "ping fails", mutate: func(deps *dependencies) {
				database, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
				if err != nil {
					t.Fatalf("sqlmock.New: %v", err)
				}
				mock.ExpectPing().WillReturnError(errors.New(secret))
				deps.openDatabase = func(string, string) (*sql.DB, error) { return database, nil }
			}},
			{name: "output fails", mutate: func(deps *dependencies) {
				database, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
				if err != nil {
					t.Fatalf("sqlmock.New: %v", err)
				}
				mock.ExpectPing()
				mock.ExpectQuery(regexp.QuoteMeta("SELECT version, checksum FROM schema_migrations ORDER BY version")).WillReturnRows(ledgerRows(testMigrationInventory(t).Identities))
				deps.openDatabase = func(string, string) (*sql.DB, error) { return database, nil }
				deps.reportWriter = &recordingMigrationReportWriter{err: &surfacereport.ReportError{Code: surfacereport.ErrorReportOutputUnwritable, Field: "destination"}}
			}},
		} {
			t.Run(test.name, func(t *testing.T) {
				deps := migrationCommandTestDependencies(t)
				deps.lookupEnv = func(string) (string, bool) { return secret, true }
				test.mutate(&deps)
				stdout, stderr, exitCode := runMigrationCommand(t, "ledger", filepath.Join(t.TempDir(), "ledger.json"), "MIGRATION_TEST_DATABASE_URL", deps)
				if exitCode == 0 {
					t.Fatalf("%s passed", test.name)
				}
				assertNoMigrationSecret(t, stdout, stderr, surfacereport.SurfaceReportV1{})
			})
		}
	})
}

func migrationCommandTestDependencies(t *testing.T) dependencies {
	t.Helper()
	return dependencies{
		identityProvider: &migrationCommandIdentityProvider{identity: migrationCommandIdentity()},
		profileResolver:  &migrationCommandProfileResolver{profile: releasecontract.DeploymentProfile{ID: "monolith", Commitment: releasecontract.CommitmentCommitted}},
		reportWriter:     surfacereport.NewAtomicWriter(),
		buildInventory: func(context.Context, string, string) (migrations.StaticInventory, error) {
			return testMigrationInventory(t), nil
		},
		lookupEnv: func(name string) (string, bool) {
			if name != "MIGRATION_TEST_DATABASE_URL" {
				t.Fatalf("lookup env name = %q", name)
			}
			return "postgres://release:do-not-print@127.0.0.1:5432/oblivious", true
		},
		openDatabase: func(string, string) (*sql.DB, error) { return nil, errors.New("database not configured") },
		pingDatabase: func(ctx context.Context, database *sql.DB) error { return database.PingContext(ctx) },
	}
}

func runMigrationCommand(t *testing.T, subcommand, output, envName string, deps dependencies) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	exitCode := runWithDependencies(context.Background(), migrationCommandArgs(subcommand, output, envName), &stdout, &stderr, deps)
	return stdout.String(), stderr.String(), exitCode
}

func migrationCommandArgs(subcommand, output, envName string) []string {
	args := []string{
		subcommand,
		"--repo", "/test/repo",
		"--contract", "config/release/contract.v1.json",
		"--schema", "config/release/contract.schema.json",
		"--profile", "monolith",
		"--output", output,
	}
	if subcommand == "ledger" {
		args = append(args, "--database-url-env", envName)
	}
	return args
}

func removeMigrationFlag(args []string, name string) []string {
	result := make([]string, 0, len(args)-2)
	for index := 0; index < len(args); index++ {
		if args[index] == name && index+1 < len(args) {
			index++
			continue
		}
		result = append(result, args[index])
	}
	return result
}

func testMigrationInventory(t *testing.T) migrations.StaticInventory {
	t.Helper()
	identities := []migrations.MigrationIdentity{
		{Version: "0001_alpha.sql", Checksum: strings.Repeat("a", 64)},
		{Version: "0002_beta.sql", Checksum: strings.Repeat("b", 64)},
	}
	entries := []migrations.StaticMigrationEntry{
		{Identity: identities[0], Filename: "src/server/migrations/0001_alpha.sql", Disposition: migrations.DispositionMonolithManaged},
		{Identity: identities[1], Filename: "src/server/migrations/0002_beta.sql", Disposition: migrations.DispositionMonolithManaged},
		{Identity: migrations.MigrationIdentity{Version: "0001_click.sql", Checksum: strings.Repeat("c", 64)}, Filename: "src/server/migrations/clickhouse/0001_click.sql", Disposition: migrations.DispositionClickHouseExcluded},
		{Identity: migrations.MigrationIdentity{Version: "0001_service.sql", Checksum: strings.Repeat("d", 64)}, Filename: "src/server/migrations/microservices/0001_service.sql", Disposition: migrations.DispositionMicroserviceExcluded},
	}
	identityDigest, err := migrations.IdentityDigest(identities)
	if err != nil {
		t.Fatalf("identity digest: %v", err)
	}
	metadataDigest, err := migrations.StaticMetadataDigest(entries)
	if err != nil {
		t.Fatalf("metadata digest: %v", err)
	}
	return migrations.StaticInventory{
		Identities: identities, Entries: entries, IdentityDigest: identityDigest,
		StaticMetadataDigest: metadataDigest, MonolithFileCount: len(identities),
		NonMonolithDispositionCounts: map[string]int{
			migrations.DispositionClickHouseExcluded:   1,
			migrations.DispositionMicroserviceExcluded: 1,
		},
	}
}

func ledgerRows(identities []migrations.MigrationIdentity) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"version", "checksum"})
	for _, identity := range identities {
		rows.AddRow(identity.Version, identity.Checksum)
	}
	return rows
}

func migrationCommandIdentity() buildinfo.BuildIdentityV1 {
	return buildinfo.BuildIdentityV1{
		SchemaVersion: buildinfo.BuildIdentitySchemaV1,
		ReleaseCommit: strings.Repeat("a", 40), SourceTree: strings.Repeat("b", 40),
		ContractDigest: "sha256:" + strings.Repeat("c", 64), Dirty: false,
		EvidenceClass: buildinfo.EvidenceRepositoryLocal,
	}
}

type migrationCommandIdentityProvider struct {
	identity buildinfo.BuildIdentityV1
	err      error
}

func (p *migrationCommandIdentityProvider) Resolve(context.Context, string, string, string) (buildinfo.BuildIdentityV1, error) {
	return p.identity, p.err
}

type migrationCommandProfileResolver struct {
	profile releasecontract.DeploymentProfile
	err     error
}

func (r *migrationCommandProfileResolver) ResolveCommittedProfile(context.Context, string, string, string, string) (releasecontract.DeploymentProfile, error) {
	return r.profile, r.err
}

type recordingMigrationReportWriter struct {
	delegate surfacereport.ReportWriter
	report   surfacereport.SurfaceReportV1
	path     string
	calls    int
	err      error
}

func (w *recordingMigrationReportWriter) Write(ctx context.Context, path string, report surfacereport.SurfaceReportV1) error {
	w.calls++
	w.path = path
	w.report = report
	if w.err != nil {
		return w.err
	}
	return w.delegate.Write(ctx, path, report)
}

func assertMigrationReportFile(t *testing.T, path, surface string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	report, err := surfacereport.Decode(content, surfacereport.NewDetailsRegistry())
	if err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.SurfaceIdentity.Surface != surface {
		t.Fatalf("surface = %q, want %q", report.SurfaceIdentity.Surface, surface)
	}
	return content
}

func assertNoMigrationSecret(t *testing.T, stdout, stderr string, report surfacereport.SurfaceReportV1) {
	t.Helper()
	reportJSON, _ := json.Marshal(report)
	combined := stdout + stderr + string(reportJSON)
	for _, secret := range []string{"postgres://", "super-secret", "do-not-print"} {
		if strings.Contains(combined, secret) {
			t.Fatalf("secret %q leaked: %s", secret, combined)
		}
	}
}

func assertFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
