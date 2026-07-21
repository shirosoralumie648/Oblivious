package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"regexp"
	"strings"

	_ "github.com/lib/pq"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/migrations"
	"oblivious/server/internal/releasecontract"
	"oblivious/server/internal/surfacereport"
)

const migrationDispositionPath = "config/release/migration-disposition.v1.json"

var databaseEnvironmentNamePattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

type dependencies struct {
	identityProvider buildinfo.IdentityProvider
	profileResolver  releasecontract.ProfileResolver
	reportWriter     surfacereport.ReportWriter
	buildInventory   func(context.Context, string, string) (migrations.StaticInventory, error)
	lookupEnv        func(string) (string, bool)
	openDatabase     func(string, string) (*sql.DB, error)
	pingDatabase     func(context.Context, *sql.DB) error
}

type commandOptions struct {
	repoRoot     string
	contractPath string
	schemaPath   string
	profileID    string
	outputPath   string
	databaseEnv  string
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return runWithDependencies(ctx, args, stdout, stderr, dependencies{
		identityProvider: buildinfo.NewGitProvider(),
		profileResolver:  releasecontract.NewFileProfileResolver(),
		reportWriter:     surfacereport.NewAtomicWriter(),
		buildInventory:   migrations.BuildStaticInventory,
		lookupEnv:        os.LookupEnv,
		openDatabase:     sql.Open,
		pingDatabase: func(ctx context.Context, database *sql.DB) error {
			return database.PingContext(ctx)
		},
	})
}

func runWithDependencies(ctx context.Context, args []string, stdout, stderr io.Writer, deps dependencies) int {
	if len(args) == 0 || ctx == nil {
		writeCLIError(stderr, "invalid_arguments", "subcommand")
		return 2
	}
	switch args[0] {
	case "static":
		if !validStaticDependencies(deps) {
			writeCLIError(stderr, "invalid_arguments", "dependencies")
			return 2
		}
		options, ok := parseOptions("static", args[1:], false, stderr)
		if !ok {
			return 2
		}
		return runStatic(ctx, options, stdout, stderr, deps)
	case "ledger":
		if !validLedgerDependencies(deps) {
			writeCLIError(stderr, "invalid_arguments", "dependencies")
			return 2
		}
		options, ok := parseOptions("ledger", args[1:], true, stderr)
		if !ok {
			return 2
		}
		return runLedger(ctx, options, stdout, stderr, deps)
	default:
		writeCLIError(stderr, "invalid_arguments", "subcommand")
		return 2
	}
}

func validStaticDependencies(deps dependencies) bool {
	return deps.identityProvider != nil && deps.profileResolver != nil && deps.reportWriter != nil && deps.buildInventory != nil
}

func validLedgerDependencies(deps dependencies) bool {
	return validStaticDependencies(deps) && deps.lookupEnv != nil && deps.openDatabase != nil && deps.pingDatabase != nil
}

func parseOptions(name string, args []string, requireDatabaseEnv bool, stderr io.Writer) (commandOptions, bool) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var options commandOptions
	flags.StringVar(&options.repoRoot, "repo", "", "explicit repository root")
	flags.StringVar(&options.contractPath, "contract", "", "authored release contract path")
	flags.StringVar(&options.schemaPath, "schema", "", "release contract schema path")
	flags.StringVar(&options.profileID, "profile", "", "explicit committed deployment profile")
	flags.StringVar(&options.outputPath, "output", "", "atomic report output path")
	if requireDatabaseEnv {
		flags.StringVar(&options.databaseEnv, "database-url-env", "", "name of the environment variable containing the PostgreSQL URL")
	}
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 ||
		strings.TrimSpace(options.repoRoot) == "" || strings.TrimSpace(options.contractPath) == "" ||
		strings.TrimSpace(options.schemaPath) == "" || strings.TrimSpace(options.profileID) == "" ||
		strings.TrimSpace(options.outputPath) == "" ||
		(requireDatabaseEnv && !databaseEnvironmentNamePattern.MatchString(options.databaseEnv)) {
		writeCLIError(stderr, "invalid_arguments", name)
		return commandOptions{}, false
	}
	return options, true
}

func runStatic(ctx context.Context, options commandOptions, stdout, stderr io.Writer, deps dependencies) int {
	inventory, err := deps.buildInventory(ctx, options.repoRoot, migrationDispositionPath)
	if err != nil {
		return writeDomainError(stderr, err)
	}
	report, err := surfacereport.NewMigrationStaticReport(
		ctx, deps.identityProvider, deps.profileResolver,
		options.repoRoot, options.contractPath, options.schemaPath, options.profileID,
		inventory, passingOutcome(),
	)
	if err != nil {
		return writeDomainError(stderr, err)
	}
	return writeReport(ctx, stdout, stderr, deps.reportWriter, options.outputPath, report)
}

func runLedger(ctx context.Context, options commandOptions, stdout, stderr io.Writer, deps dependencies) int {
	inventory, err := deps.buildInventory(ctx, options.repoRoot, migrationDispositionPath)
	if err != nil {
		return writeDomainError(stderr, err)
	}
	databaseURL, found := deps.lookupEnv(options.databaseEnv)
	if !found || strings.TrimSpace(databaseURL) == "" {
		return writeDomainError(stderr, ledgerUnavailable("database-url-env", nil))
	}
	database, err := deps.openDatabase("postgres", databaseURL)
	if err != nil || database == nil {
		return writeDomainError(stderr, ledgerUnavailable("database.open", err))
	}
	defer database.Close()
	if err := deps.pingDatabase(ctx, database); err != nil {
		return writeDomainError(stderr, ledgerUnavailable("database.ping", err))
	}
	ledger, err := migrations.ReadLedger(ctx, database)
	if err != nil {
		return writeDomainError(stderr, err)
	}
	report, err := surfacereport.NewMigrationLedgerReport(
		ctx, deps.identityProvider, deps.profileResolver,
		options.repoRoot, options.contractPath, options.schemaPath, options.profileID,
		inventory, ledger, passingOutcome(),
	)
	if err != nil {
		return writeDomainError(stderr, err)
	}
	return writeReport(ctx, stdout, stderr, deps.reportWriter, options.outputPath, report)
}

func writeReport(ctx context.Context, stdout, stderr io.Writer, writer surfacereport.ReportWriter, outputPath string, report surfacereport.SurfaceReportV1) int {
	if err := writer.Write(ctx, outputPath, report); err != nil {
		return writeDomainError(stderr, err)
	}
	return writeSuccess(stdout, stderr, struct {
		SchemaVersion string `json:"schemaVersion"`
		Surface       string `json:"surface"`
		Profile       string `json:"profile"`
		Result        string `json:"result"`
		EvidenceClass string `json:"evidenceClass"`
	}{
		SchemaVersion: report.SchemaVersion,
		Surface:       report.SurfaceIdentity.Surface,
		Profile:       report.ReleaseIdentity.DeploymentProfile,
		Result:        string(report.Outcome.Result),
		EvidenceClass: report.Evidence.Class,
	})
}

func passingOutcome() surfacereport.Outcome {
	return surfacereport.Outcome{Result: surfacereport.ResultPass, ErrorCodes: []string{}, SkippedChecks: []string{}}
}

func ledgerUnavailable(field string, err error) error {
	return &migrations.InventoryError{Code: migrations.ErrorMigrationLedgerUnavailable, Field: field, Err: err}
}

func writeSuccess(stdout, stderr io.Writer, value any) int {
	if err := json.NewEncoder(stdout).Encode(value); err != nil {
		writeCLIError(stderr, "output_unwritable", "stdout")
		return 1
	}
	return 0
}

func writeDomainError(stderr io.Writer, err error) int {
	var inventoryErr *migrations.InventoryError
	if errors.As(err, &inventoryErr) {
		writeCLIError(stderr, string(inventoryErr.Code), inventoryErr.Field)
		return 1
	}
	var identityErr *buildinfo.IdentityError
	if errors.As(err, &identityErr) {
		writeCLIError(stderr, string(identityErr.Code), identityErr.Field)
		return 1
	}
	var contractErr *releasecontract.ContractError
	if errors.As(err, &contractErr) {
		writeCLIError(stderr, string(contractErr.Code), contractErr.Field)
		return 1
	}
	var reportErr *surfacereport.ReportError
	if errors.As(err, &reportErr) {
		writeCLIError(stderr, string(reportErr.Code), reportErr.Field)
		return 1
	}
	writeCLIError(stderr, "internal_error", "operation")
	return 1
}

func writeCLIError(stderr io.Writer, code, field string) {
	_ = json.NewEncoder(stderr).Encode(struct {
		Error struct {
			Code  string `json:"code"`
			Field string `json:"field,omitempty"`
		} `json:"error"`
	}{Error: struct {
		Code  string `json:"code"`
		Field string `json:"field,omitempty"`
	}{Code: code, Field: field}})
}
