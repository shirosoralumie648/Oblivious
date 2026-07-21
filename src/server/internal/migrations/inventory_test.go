package migrations

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

func TestMigrationInventoryContract(t *testing.T) {
	t.Run("checked in disposition is total and keeps non monolith SQL separate", func(t *testing.T) {
		repoRoot := migrationInventoryRepoRoot(t)
		inventory, err := BuildStaticInventory(context.Background(), repoRoot, "config/release/migration-disposition.v1.json")
		if err != nil {
			t.Fatalf("build checked-in inventory: %v", err)
		}
		if inventory.MonolithFileCount == 0 || len(inventory.Identities) != inventory.MonolithFileCount {
			t.Fatalf("monolith inventory is empty or inconsistent: %#v", inventory)
		}
		if inventory.NonMonolithDispositionCounts[DispositionMicroserviceExcluded] == 0 || inventory.NonMonolithDispositionCounts[DispositionClickHouseExcluded] == 0 {
			t.Fatalf("non-monolith dispositions are not visible: %#v", inventory.NonMonolithDispositionCounts)
		}
		if len(inventory.Entries) != inventory.MonolithFileCount+inventory.NonMonolithDispositionCounts[DispositionMicroserviceExcluded]+inventory.NonMonolithDispositionCounts[DispositionClickHouseExcluded] {
			t.Fatalf("tracked SQL set does not equal disposition set: files=%d counts=%#v", len(inventory.Entries), inventory.NonMonolithDispositionCounts)
		}
		for _, entry := range inventory.Entries {
			content, readErr := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(entry.Filename)))
			if readErr != nil {
				t.Fatalf("read %s: %v", entry.Filename, readErr)
			}
			if entry.Identity.Checksum != Checksum(content) {
				t.Fatalf("%s did not consume live Checksum", entry.Filename)
			}
		}
	})

	t.Run("exact bytes drive only canonical monolith identity", func(t *testing.T) {
		repoRoot := newMigrationInventoryFixture(t)
		before, err := BuildStaticInventory(context.Background(), repoRoot, "config/release/migration-disposition.v1.json")
		if err != nil {
			t.Fatalf("build initial inventory: %v", err)
		}
		migrationPath := filepath.Join(repoRoot, "src/server/migrations/0001_alpha.sql")
		if err := os.WriteFile(migrationPath, []byte("SELECT 2;\n"), 0o600); err != nil {
			t.Fatalf("mutate migration: %v", err)
		}
		after, err := BuildStaticInventory(context.Background(), repoRoot, "config/release/migration-disposition.v1.json")
		if err != nil {
			t.Fatalf("build mutated inventory: %v", err)
		}
		if before.IdentityDigest == after.IdentityDigest {
			t.Fatal("historical SQL mutation did not change monolith identity digest")
		}
		if before.StaticMetadataDigest != after.StaticMetadataDigest {
			t.Fatal("SQL bytes changed filename/disposition metadata digest")
		}
		if len(before.Identities) != 2 || len(before.Entries) != 4 {
			t.Fatalf("non-monolith rows contaminated monolith identity: identities=%d entries=%d", len(before.Identities), len(before.Entries))
		}
	})

	t.Run("identity and static metadata digests have disjoint fields", func(t *testing.T) {
		repoRoot := newMigrationInventoryFixture(t)
		inventory, err := BuildStaticInventory(context.Background(), repoRoot, "config/release/migration-disposition.v1.json")
		if err != nil {
			t.Fatalf("build inventory: %v", err)
		}
		changedIdentities := append([]MigrationIdentity(nil), inventory.Identities...)
		changedIdentities[0].Checksum = strings.Repeat("f", 64)
		changedIdentityDigest, err := IdentityDigest(changedIdentities)
		if err != nil {
			t.Fatalf("digest checksum mutation: %v", err)
		}
		if changedIdentityDigest == inventory.IdentityDigest {
			t.Fatal("checksum mutation did not change identity digest")
		}
		changedEntries := append([]StaticMigrationEntry(nil), inventory.Entries...)
		changedEntries[0].Identity.Checksum = strings.Repeat("e", 64)
		metadataAfterIdentityMutation, err := StaticMetadataDigest(changedEntries)
		if err != nil {
			t.Fatalf("digest metadata after identity mutation: %v", err)
		}
		if metadataAfterIdentityMutation != inventory.StaticMetadataDigest {
			t.Fatal("identity entered static filename/disposition metadata digest")
		}
		changedEntries[0].Disposition = "different-static-disposition"
		metadataAfterDispositionMutation, err := StaticMetadataDigest(changedEntries)
		if err != nil {
			t.Fatalf("digest disposition mutation: %v", err)
		}
		if metadataAfterDispositionMutation == inventory.StaticMetadataDigest {
			t.Fatal("disposition mutation did not change static metadata digest")
		}
	})

	t.Run("ledger uses the exact canonical identity sequence", func(t *testing.T) {
		identities := []MigrationIdentity{
			{Version: "0001_alpha.sql", Checksum: strings.Repeat("a", 64)},
			{Version: "0002_beta.sql", Checksum: strings.Repeat("b", 64)},
		}
		database := openInventoryLedgerDB(t, identities)
		got, err := ReadLedger(context.Background(), database)
		if err != nil {
			t.Fatalf("read ledger: %v", err)
		}
		if !reflect.DeepEqual(got, identities) {
			t.Fatalf("ledger identities = %#v, want %#v", got, identities)
		}
		staticDigest, err := IdentityDigest(identities)
		if err != nil {
			t.Fatalf("static digest: %v", err)
		}
		ledgerDigest, err := IdentityDigest(got)
		if err != nil {
			t.Fatalf("ledger digest: %v", err)
		}
		if ledgerDigest != staticDigest {
			t.Fatalf("ledger digest %s != static digest %s", ledgerDigest, staticDigest)
		}
	})

	t.Run("invalid duplicate unordered empty and non monolith identities fail closed", func(t *testing.T) {
		valid := []MigrationIdentity{
			{Version: "0001_alpha.sql", Checksum: strings.Repeat("a", 64)},
			{Version: "0002_beta.sql", Checksum: strings.Repeat("b", 64)},
		}
		cases := []struct {
			name string
			rows []MigrationIdentity
			code InventoryErrorCode
		}{
			{name: "empty", rows: []MigrationIdentity{}, code: ErrorMigrationIdentityInvalid},
			{name: "duplicate", rows: []MigrationIdentity{valid[0], valid[0]}, code: ErrorMigrationIdentityDuplicate},
			{name: "unordered", rows: []MigrationIdentity{valid[1], valid[0]}, code: ErrorMigrationIdentityOrder},
			{name: "checksum shape", rows: []MigrationIdentity{{Version: "0001_alpha.sql", Checksum: "sha256:" + strings.Repeat("a", 64)}}, code: ErrorMigrationIdentityInvalid},
			{name: "non monolith contamination", rows: []MigrationIdentity{{Version: "admin.sql", Checksum: strings.Repeat("a", 64)}}, code: ErrorMigrationFilenameInvalid},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				if _, err := IdentityDigest(testCase.rows); !IsInventoryCode(err, testCase.code) {
					t.Fatalf("IdentityDigest error = %v, want %s", err, testCase.code)
				}
			})
		}

		for _, testCase := range []struct {
			name string
			rows []MigrationIdentity
			code InventoryErrorCode
		}{
			{name: "ledger zero", rows: nil, code: ErrorMigrationLedgerUnavailable},
			{name: "ledger duplicate", rows: []MigrationIdentity{valid[0], valid[0]}, code: ErrorMigrationIdentityDuplicate},
			{name: "ledger order", rows: []MigrationIdentity{valid[1], valid[0]}, code: ErrorMigrationIdentityOrder},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				database := openInventoryLedgerDB(t, testCase.rows)
				if _, err := ReadLedger(context.Background(), database); !IsInventoryCode(err, testCase.code) {
					t.Fatalf("ReadLedger error = %v, want %s", err, testCase.code)
				}
			})
		}
	})

	t.Run("disposition mutations fail with stable codes", func(t *testing.T) {
		baseManifest := validMigrationDispositionManifest()
		cases := []struct {
			name   string
			mutate func(*migrationDispositionManifest)
			code   InventoryErrorCode
		}{
			{name: "missing surface", mutate: func(value *migrationDispositionManifest) { value.Entries = value.Entries[:2] }, code: ErrorMigrationDispositionInvalid},
			{name: "duplicate surface", mutate: func(value *migrationDispositionManifest) { value.Entries[1] = value.Entries[0] }, code: ErrorMigrationDispositionInvalid},
			{name: "unsafe path", mutate: func(value *migrationDispositionManifest) { value.Entries[0].Pattern = "../*.sql" }, code: ErrorMigrationDispositionInvalid},
			{name: "empty matched surface", mutate: func(value *migrationDispositionManifest) {
				value.Entries[2].Pattern = "src/server/migrations/clickhouse/9999_*.sql"
			}, code: ErrorMigrationDispositionDrift},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				repoRoot := newMigrationInventoryFixture(t)
				manifest := baseManifest
				manifest.Entries = append([]migrationDispositionEntry(nil), baseManifest.Entries...)
				testCase.mutate(&manifest)
				writeMigrationDisposition(t, repoRoot, manifest)
				if _, err := BuildStaticInventory(context.Background(), repoRoot, "config/release/migration-disposition.v1.json"); !IsInventoryCode(err, testCase.code) {
					t.Fatalf("BuildStaticInventory error = %v, want %s", err, testCase.code)
				}
			})
		}

		t.Run("unmanaged tracked SQL", func(t *testing.T) {
			repoRoot := newMigrationInventoryFixture(t)
			writeInventoryFile(t, repoRoot, "src/server/migrations/unmanaged/0003_gamma.sql", "SELECT 3;\n")
			runInventoryGit(t, repoRoot, "add", "src/server/migrations/unmanaged/0003_gamma.sql")
			if _, err := BuildStaticInventory(context.Background(), repoRoot, "config/release/migration-disposition.v1.json"); !IsInventoryCode(err, ErrorMigrationDispositionDrift) {
				t.Fatalf("unmanaged SQL error = %v", err)
			}
		})
	})
}

func migrationInventoryRepoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate inventory test")
	}
	root, err := filepath.Abs(filepath.Join(filepath.Dir(filename), "../../../.."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

func newMigrationInventoryFixture(t *testing.T) string {
	t.Helper()
	repoRoot := t.TempDir()
	writeInventoryFile(t, repoRoot, "src/server/migrations/0002_beta.sql", "SELECT 2;\n")
	writeInventoryFile(t, repoRoot, "src/server/migrations/0001_alpha.sql", "SELECT 1;\n")
	writeInventoryFile(t, repoRoot, "src/server/migrations/microservices/admin.sql", "CREATE TABLE admin_fixture(id bigint);\n")
	writeInventoryFile(t, repoRoot, "src/server/migrations/clickhouse/0001_events.sql", "CREATE TABLE events_fixture(id UInt64) ENGINE=MergeTree ORDER BY id;\n")
	writeMigrationDisposition(t, repoRoot, validMigrationDispositionManifest())
	runInventoryGit(t, repoRoot, "init", "-q")
	runInventoryGit(t, repoRoot, "add", "config/release/migration-disposition.v1.json", "src/server/migrations")
	return repoRoot
}

func validMigrationDispositionManifest() migrationDispositionManifest {
	return migrationDispositionManifest{
		SchemaVersion: MigrationDispositionSchemaV1,
		Entries: []migrationDispositionEntry{
			{Surface: "monolith", Pattern: "src/server/migrations/*.sql", Disposition: DispositionMonolithManaged},
			{Surface: "microservices", Pattern: "src/server/migrations/microservices/*.sql", Disposition: DispositionMicroserviceExcluded},
			{Surface: "clickhouse", Pattern: "src/server/migrations/clickhouse/*.sql", Disposition: DispositionClickHouseExcluded},
		},
	}
}

func writeMigrationDisposition(t *testing.T, repoRoot string, manifest migrationDispositionManifest) {
	t.Helper()
	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal disposition: %v", err)
	}
	writeInventoryFile(t, repoRoot, "config/release/migration-disposition.v1.json", string(content)+"\n")
}

func writeInventoryFile(t *testing.T, repoRoot, relative, content string) {
	t.Helper()
	filename := filepath.Join(repoRoot, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatalf("create %s parent: %v", relative, err)
	}
	if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", relative, err)
	}
}

func runInventoryGit(t *testing.T, repoRoot string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

var inventoryDriverSequence atomic.Uint64

func openInventoryLedgerDB(t *testing.T, identities []MigrationIdentity) *sql.DB {
	t.Helper()
	name := "migration_inventory_ledger_" + strings.Repeat("x", int(inventoryDriverSequence.Add(1)))
	sql.Register(name, &inventoryLedgerDriver{identities: append([]MigrationIdentity(nil), identities...)})
	database, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("open ledger database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

type inventoryLedgerDriver struct {
	identities []MigrationIdentity
}

func (d *inventoryLedgerDriver) Open(string) (driver.Conn, error) {
	return &inventoryLedgerConn{identities: append([]MigrationIdentity(nil), d.identities...)}, nil
}

type inventoryLedgerConn struct {
	identities []MigrationIdentity
}

func (c *inventoryLedgerConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}
func (c *inventoryLedgerConn) Close() error { return nil }
func (c *inventoryLedgerConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported")
}
func (c *inventoryLedgerConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if query != `SELECT version, checksum FROM schema_migrations ORDER BY version` {
		return nil, errors.New("unexpected ledger query")
	}
	return &inventoryLedgerRows{identities: append([]MigrationIdentity(nil), c.identities...)}, nil
}

type inventoryLedgerRows struct {
	identities []MigrationIdentity
	index      int
}

func (r *inventoryLedgerRows) Columns() []string { return []string{"version", "checksum"} }
func (r *inventoryLedgerRows) Close() error      { return nil }
func (r *inventoryLedgerRows) Next(destination []driver.Value) error {
	if r.index >= len(r.identities) {
		return io.EOF
	}
	destination[0] = r.identities[r.index].Version
	destination[1] = r.identities[r.index].Checksum
	r.index++
	return nil
}
