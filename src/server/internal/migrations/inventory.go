package migrations

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	MigrationDispositionSchemaV1 = "migration-disposition/v1"

	DispositionMonolithManaged       = "monolith-managed"
	DispositionMicroserviceExcluded  = "microservice-non-monolith"
	DispositionClickHouseExcluded    = "clickhouse-non-monolith"
	defaultMigrationDispositionLimit = 1 << 20
)

type InventoryErrorCode string

const (
	ErrorMigrationDispositionInvalid InventoryErrorCode = "migration_disposition_invalid"
	ErrorMigrationDispositionEmpty   InventoryErrorCode = "migration_disposition_inventory_empty"
	ErrorMigrationDispositionDrift   InventoryErrorCode = "migration_disposition_mismatch"
	ErrorMigrationPathInvalid        InventoryErrorCode = "migration_path_invalid"
	ErrorMigrationFilenameInvalid    InventoryErrorCode = "migration_filename_invalid"
	ErrorMigrationIdentityInvalid    InventoryErrorCode = "migration_identity_invalid"
	ErrorMigrationIdentityDuplicate  InventoryErrorCode = "migration_identity_duplicate"
	ErrorMigrationIdentityOrder      InventoryErrorCode = "migration_identity_order_invalid"
	ErrorMigrationLedgerUnavailable  InventoryErrorCode = "migration_ledger_unavailable"
)

type InventoryError struct {
	Code  InventoryErrorCode
	Field string
	Err   error
}

func (e *InventoryError) Error() string {
	if e.Field == "" {
		return string(e.Code)
	}
	return string(e.Code) + ": field=" + e.Field
}

func (e *InventoryError) Unwrap() error { return e.Err }

func IsInventoryCode(err error, code InventoryErrorCode) bool {
	var inventoryErr *InventoryError
	return errors.As(err, &inventoryErr) && inventoryErr.Code == code
}

type MigrationIdentity struct {
	Version  string `json:"version"`
	Checksum string `json:"checksum"`
}

type StaticMigrationEntry struct {
	Identity    MigrationIdentity `json:"identity"`
	Filename    string            `json:"filename"`
	Disposition string            `json:"disposition"`
}

type StaticInventory struct {
	Identities                   []MigrationIdentity    `json:"identities"`
	Entries                      []StaticMigrationEntry `json:"entries"`
	IdentityDigest               string                 `json:"identityDigest"`
	StaticMetadataDigest         string                 `json:"staticMetadataDigest"`
	MonolithFileCount            int                    `json:"monolithFileCount"`
	NonMonolithDispositionCounts map[string]int         `json:"nonMonolithDispositionCounts"`
}

type migrationDispositionManifest struct {
	SchemaVersion string                      `json:"schemaVersion"`
	Entries       []migrationDispositionEntry `json:"entries"`
}

type migrationDispositionEntry struct {
	Surface     string `json:"surface"`
	Pattern     string `json:"pattern"`
	Disposition string `json:"disposition"`
}

type ledgerQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// BuildStaticInventory validates the tracked SQL disposition and derives the
// monolith identity sequence from the exact bytes consumed by Apply.
func BuildStaticInventory(ctx context.Context, repoRoot, dispositionPath string) (StaticInventory, error) {
	if ctx == nil || ctx.Err() != nil {
		return StaticInventory{}, inventoryError(ErrorMigrationDispositionInvalid, "context", ctxError(ctx))
	}
	root, err := canonicalInventoryRoot(repoRoot)
	if err != nil {
		return StaticInventory{}, err
	}
	manifest, err := loadMigrationDisposition(root, dispositionPath)
	if err != nil {
		return StaticInventory{}, err
	}
	tracked, err := trackedMigrationSQL(ctx, root)
	if err != nil {
		return StaticInventory{}, err
	}

	matchedByEntry := make([]int, len(manifest.Entries))
	entries := make([]StaticMigrationEntry, 0, len(tracked))
	identities := make([]MigrationIdentity, 0, len(tracked))
	nonMonolithCounts := map[string]int{}
	for _, filename := range tracked {
		matchedIndex := -1
		for index, disposition := range manifest.Entries {
			matched, matchErr := path.Match(disposition.Pattern, filename)
			if matchErr != nil {
				return StaticInventory{}, inventoryError(ErrorMigrationDispositionInvalid, "entries.pattern", matchErr)
			}
			if !matched {
				continue
			}
			if matchedIndex >= 0 {
				return StaticInventory{}, inventoryError(ErrorMigrationDispositionDrift, filename, nil)
			}
			matchedIndex = index
		}
		if matchedIndex < 0 {
			return StaticInventory{}, inventoryError(ErrorMigrationDispositionDrift, filename, nil)
		}
		matchedByEntry[matchedIndex]++
		disposition := manifest.Entries[matchedIndex]
		content, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(filename)))
		if readErr != nil {
			return StaticInventory{}, inventoryError(ErrorMigrationPathInvalid, filename, readErr)
		}
		identity := MigrationIdentity{Version: path.Base(filename), Checksum: Checksum(content)}
		entry := StaticMigrationEntry{Identity: identity, Filename: filename, Disposition: disposition.Disposition}
		entries = append(entries, entry)
		if disposition.Disposition == DispositionMonolithManaged {
			if _, parseErr := numericMigrationVersion(identity.Version); parseErr != nil {
				return StaticInventory{}, parseErr
			}
			identities = append(identities, identity)
		} else {
			nonMonolithCounts[disposition.Disposition]++
		}
	}
	for index, count := range matchedByEntry {
		if count == 0 {
			return StaticInventory{}, inventoryError(ErrorMigrationDispositionEmpty, manifest.Entries[index].Pattern, nil)
		}
	}
	if len(identities) == 0 || len(entries) == 0 || len(nonMonolithCounts) == 0 {
		return StaticInventory{}, inventoryError(ErrorMigrationDispositionEmpty, "entries", nil)
	}

	sort.Slice(identities, func(left, right int) bool {
		return compareMigrationIdentity(identities[left], identities[right]) < 0
	})
	sort.Slice(entries, func(left, right int) bool { return entries[left].Filename < entries[right].Filename })
	identityDigest, err := IdentityDigest(identities)
	if err != nil {
		return StaticInventory{}, err
	}
	metadataDigest, err := StaticMetadataDigest(entries)
	if err != nil {
		return StaticInventory{}, err
	}
	return StaticInventory{
		Identities: identities, Entries: entries, IdentityDigest: identityDigest,
		StaticMetadataDigest: metadataDigest, MonolithFileCount: len(identities),
		NonMonolithDispositionCounts: nonMonolithCounts,
	}, nil
}

// ReadLedger reads the live schema_migrations identity without filenames or
// other static metadata and rejects non-canonical database ordering.
func ReadLedger(ctx context.Context, database ledgerQueryer) ([]MigrationIdentity, error) {
	if ctx == nil || ctx.Err() != nil || database == nil {
		return nil, inventoryError(ErrorMigrationLedgerUnavailable, "database", ctxError(ctx))
	}
	rows, err := database.QueryContext(ctx, `SELECT version, checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, inventoryError(ErrorMigrationLedgerUnavailable, "schema_migrations", err)
	}
	defer rows.Close()
	identities := make([]MigrationIdentity, 0)
	for rows.Next() {
		var identity MigrationIdentity
		if err := rows.Scan(&identity.Version, &identity.Checksum); err != nil {
			return nil, inventoryError(ErrorMigrationLedgerUnavailable, "schema_migrations", err)
		}
		identities = append(identities, identity)
	}
	if err := rows.Err(); err != nil {
		return nil, inventoryError(ErrorMigrationLedgerUnavailable, "schema_migrations", err)
	}
	if len(identities) == 0 {
		return nil, inventoryError(ErrorMigrationLedgerUnavailable, "schema_migrations.empty", nil)
	}
	if err := validateCanonicalIdentities(identities); err != nil {
		return nil, err
	}
	return identities, nil
}

// IdentityDigest covers only the canonical (version, checksum) sequence shared
// by static inventory and the runtime ledger.
func IdentityDigest(identities []MigrationIdentity) (string, error) {
	if err := validateCanonicalIdentities(identities); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(identities)
	if err != nil {
		return "", inventoryError(ErrorMigrationIdentityInvalid, "identities", err)
	}
	return sha256Digest(encoded), nil
}

// StaticMetadataDigest intentionally excludes checksums and versions. Filename
// and disposition changes are static facts and never participate in ledger equality.
func StaticMetadataDigest(entries []StaticMigrationEntry) (string, error) {
	if len(entries) == 0 {
		return "", inventoryError(ErrorMigrationDispositionEmpty, "metadata", nil)
	}
	type metadataEntry struct {
		Filename    string `json:"filename"`
		Disposition string `json:"disposition"`
	}
	metadata := make([]metadataEntry, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if !safeMigrationPath(entry.Filename) || strings.TrimSpace(entry.Disposition) == "" {
			return "", inventoryError(ErrorMigrationDispositionInvalid, "metadata", nil)
		}
		if _, exists := seen[entry.Filename]; exists {
			return "", inventoryError(ErrorMigrationDispositionDrift, entry.Filename, nil)
		}
		seen[entry.Filename] = struct{}{}
		metadata = append(metadata, metadataEntry{Filename: entry.Filename, Disposition: entry.Disposition})
	}
	sort.Slice(metadata, func(left, right int) bool { return metadata[left].Filename < metadata[right].Filename })
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return "", inventoryError(ErrorMigrationDispositionInvalid, "metadata", err)
	}
	return sha256Digest(encoded), nil
}

func loadMigrationDisposition(repoRoot, requested string) (migrationDispositionManifest, error) {
	resolved, err := resolveInventoryFile(repoRoot, requested)
	if err != nil {
		return migrationDispositionManifest{}, err
	}
	file, err := os.Open(resolved)
	if err != nil {
		return migrationDispositionManifest{}, inventoryError(ErrorMigrationPathInvalid, "disposition", err)
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, defaultMigrationDispositionLimit+1))
	if err != nil || len(content) == 0 || len(content) > defaultMigrationDispositionLimit {
		return migrationDispositionManifest{}, inventoryError(ErrorMigrationDispositionInvalid, "disposition", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var manifest migrationDispositionManifest
	if err := decoder.Decode(&manifest); err != nil {
		return migrationDispositionManifest{}, inventoryError(ErrorMigrationDispositionInvalid, "disposition", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return migrationDispositionManifest{}, inventoryError(ErrorMigrationDispositionInvalid, "disposition", err)
	}
	if err := validateMigrationDisposition(manifest); err != nil {
		return migrationDispositionManifest{}, err
	}
	return manifest, nil
}

func validateMigrationDisposition(manifest migrationDispositionManifest) error {
	if manifest.SchemaVersion != MigrationDispositionSchemaV1 || len(manifest.Entries) == 0 {
		return inventoryError(ErrorMigrationDispositionInvalid, "schemaVersion", nil)
	}
	expected := map[string]string{
		"monolith":      DispositionMonolithManaged,
		"microservices": DispositionMicroserviceExcluded,
		"clickhouse":    DispositionClickHouseExcluded,
	}
	seenSurfaces := make(map[string]struct{}, len(manifest.Entries))
	seenPatterns := make(map[string]struct{}, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		if expected[entry.Surface] != entry.Disposition || !safeMigrationPattern(entry.Pattern) {
			return inventoryError(ErrorMigrationDispositionInvalid, "entries", nil)
		}
		if _, exists := seenSurfaces[entry.Surface]; exists {
			return inventoryError(ErrorMigrationDispositionInvalid, "entries.surface", nil)
		}
		if _, exists := seenPatterns[entry.Pattern]; exists {
			return inventoryError(ErrorMigrationDispositionInvalid, "entries.pattern", nil)
		}
		seenSurfaces[entry.Surface] = struct{}{}
		seenPatterns[entry.Pattern] = struct{}{}
	}
	if len(seenSurfaces) != len(expected) {
		return inventoryError(ErrorMigrationDispositionInvalid, "entries.surface", nil)
	}
	return nil
}

func trackedMigrationSQL(ctx context.Context, repoRoot string) ([]string, error) {
	command := exec.CommandContext(ctx, "git", "-C", repoRoot, "ls-files", "-z", "--", "src/server/migrations")
	output, err := command.Output()
	if err != nil {
		return nil, inventoryError(ErrorMigrationDispositionDrift, "git", err)
	}
	parts := bytes.Split(output, []byte{0})
	tracked := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		filename := filepath.ToSlash(string(part))
		if path.Ext(filename) != ".sql" {
			continue
		}
		if !safeMigrationPath(filename) {
			return nil, inventoryError(ErrorMigrationPathInvalid, filename, nil)
		}
		tracked = append(tracked, filename)
	}
	sort.Strings(tracked)
	if len(tracked) == 0 {
		return nil, inventoryError(ErrorMigrationDispositionEmpty, "git", nil)
	}
	return tracked, nil
}

func validateCanonicalIdentities(identities []MigrationIdentity) error {
	if len(identities) == 0 {
		return inventoryError(ErrorMigrationIdentityInvalid, "identities.empty", nil)
	}
	seen := make(map[string]struct{}, len(identities))
	for index, identity := range identities {
		if _, err := numericMigrationVersion(identity.Version); err != nil {
			return err
		}
		if len(identity.Checksum) != sha256.Size*2 || identity.Checksum != strings.ToLower(identity.Checksum) {
			return inventoryError(ErrorMigrationIdentityInvalid, identity.Version, nil)
		}
		if _, err := hex.DecodeString(identity.Checksum); err != nil {
			return inventoryError(ErrorMigrationIdentityInvalid, identity.Version, err)
		}
		if _, exists := seen[identity.Version]; exists {
			return inventoryError(ErrorMigrationIdentityDuplicate, identity.Version, nil)
		}
		seen[identity.Version] = struct{}{}
		if index > 0 && compareMigrationIdentity(identities[index-1], identity) >= 0 {
			return inventoryError(ErrorMigrationIdentityOrder, identity.Version, nil)
		}
	}
	return nil
}

func numericMigrationVersion(version string) (int, error) {
	if !filePattern.MatchString(version) {
		return 0, inventoryError(ErrorMigrationFilenameInvalid, version, nil)
	}
	value, err := strconv.Atoi(version[:4])
	if err != nil {
		return 0, inventoryError(ErrorMigrationFilenameInvalid, version, err)
	}
	return value, nil
}

func compareMigrationIdentity(left, right MigrationIdentity) int {
	leftVersion, leftErr := numericMigrationVersion(left.Version)
	rightVersion, rightErr := numericMigrationVersion(right.Version)
	if leftErr == nil && rightErr == nil && leftVersion != rightVersion {
		if leftVersion < rightVersion {
			return -1
		}
		return 1
	}
	return strings.Compare(left.Version, right.Version)
}

func canonicalInventoryRoot(requested string) (string, error) {
	if !filepath.IsAbs(requested) {
		return "", inventoryError(ErrorMigrationPathInvalid, "repoRoot", nil)
	}
	resolved, err := filepath.EvalSymlinks(requested)
	if err != nil {
		return "", inventoryError(ErrorMigrationPathInvalid, "repoRoot", err)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", inventoryError(ErrorMigrationPathInvalid, "repoRoot", err)
	}
	return filepath.Clean(resolved), nil
}

func resolveInventoryFile(repoRoot, requested string) (string, error) {
	if requested == "" || strings.ContainsRune(requested, 0) {
		return "", inventoryError(ErrorMigrationPathInvalid, "disposition", nil)
	}
	candidate := requested
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(repoRoot, filepath.FromSlash(requested))
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", inventoryError(ErrorMigrationPathInvalid, "disposition", err)
	}
	relative, err := filepath.Rel(repoRoot, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", inventoryError(ErrorMigrationPathInvalid, "disposition", err)
	}
	info, err := os.Stat(resolved)
	if err != nil || info.IsDir() {
		return "", inventoryError(ErrorMigrationPathInvalid, "disposition", err)
	}
	return resolved, nil
}

func safeMigrationPattern(value string) bool {
	if value == "" || strings.Contains(value, "\\") || path.IsAbs(value) || path.Clean(value) != value || strings.Contains(value, "..") || path.Ext(value) != ".sql" {
		return false
	}
	_, err := path.Match(value, "src/server/migrations/example.sql")
	return err == nil
}

func safeMigrationPath(value string) bool {
	return value != "" && !strings.Contains(value, "\\") && !path.IsAbs(value) && path.Clean(value) == value && !strings.HasPrefix(value, "../") && path.Ext(value) == ".sql"
}

func sha256Digest(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func inventoryError(code InventoryErrorCode, field string, err error) error {
	return &InventoryError{Code: code, Field: field, Err: err}
}

func ctxError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
