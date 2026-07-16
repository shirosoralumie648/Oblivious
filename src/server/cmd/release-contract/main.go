package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"strings"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/releasecontract"
	"oblivious/server/internal/surfacereport"
)

const maxReportInputBytes = 4 << 20

type dependencies struct {
	gitProvider      buildinfo.IdentityProvider
	embeddedProvider buildinfo.IdentityProvider
	profileResolver  releasecontract.ProfileResolver
	runner           releasecontract.Runner
	reportWriter     surfacereport.ReportWriter
}

type commonOptions struct {
	repoRoot     string
	contractPath string
	schemaPath   string
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return runWithDependencies(ctx, args, stdout, stderr, dependencies{
		gitProvider:      buildinfo.NewGitProvider(),
		embeddedProvider: buildinfo.NewEmbeddedProvider(),
		profileResolver:  releasecontract.NewFileProfileResolver(),
		reportWriter:     surfacereport.NewAtomicWriter(),
	})
}

func runWithDependencies(ctx context.Context, args []string, stdout, stderr io.Writer, deps dependencies) int {
	if len(args) == 0 {
		writeCLIError(stderr, "invalid_arguments", "subcommand")
		return 2
	}
	switch args[0] {
	case "validate":
		options, ok := parseCommonOptions("validate", args[1:], stderr)
		if !ok {
			return 2
		}
		contract, err := releasecontract.Load(ctx, options.repoRoot, options.contractPath, options.schemaPath)
		if err != nil {
			return writeDomainError(stderr, err)
		}
		return writeSuccess(stdout, stderr, struct {
			SchemaVersion string `json:"schemaVersion"`
			Result        string `json:"result"`
			EvidenceClass string `json:"evidenceClass"`
		}{SchemaVersion: contract.SchemaVersion, Result: "pass", EvidenceClass: buildinfo.EvidenceRepositoryLocal})
	case "digest":
		options, ok := parseCommonOptions("digest", args[1:], stderr)
		if !ok {
			return 2
		}
		contract, err := releasecontract.Load(ctx, options.repoRoot, options.contractPath, options.schemaPath)
		if err != nil {
			return writeDomainError(stderr, err)
		}
		digest, err := releasecontract.Digest(contract)
		if err != nil {
			return writeDomainError(stderr, err)
		}
		return writeSuccess(stdout, stderr, struct {
			SchemaVersion  string `json:"schemaVersion"`
			ContractDigest string `json:"contractDigest"`
			EvidenceClass  string `json:"evidenceClass"`
		}{SchemaVersion: releasecontract.CanonicalFormatV1, ContractDigest: digest, EvidenceClass: buildinfo.EvidenceRepositoryLocal})
	case "identity":
		return runIdentity(ctx, "identity", args[1:], stdout, stderr, deps.gitProvider)
	case "inspect":
		return runIdentity(ctx, "inspect", args[1:], stdout, stderr, deps.embeddedProvider)
	case "operation":
		return runOperation(ctx, args[1:], stdout, stderr, deps)
	case "report-build-identity":
		return runBuildIdentityReport(ctx, args[1:], stdout, stderr, deps)
	case "verify-report":
		return runVerifyReport(args[1:], stdout, stderr)
	default:
		writeCLIError(stderr, "invalid_arguments", "subcommand")
		return 2
	}
}

type buildInspectionInput struct {
	Binaries         []binaryObservation         `json:"binaries"`
	OCI              ociObservation              `json:"oci"`
	PackagedContract packagedContractObservation `json:"packagedContract"`
	ResidualRisks    []string                    `json:"residualRisks"`
}

type binaryObservation struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Digest  string `json:"digest"`
	Matches bool   `json:"matches"`
}

type ociObservation struct {
	Image   string `json:"image"`
	Digest  string `json:"digest"`
	Matches bool   `json:"matches"`
}

type packagedContractObservation struct {
	Path    string `json:"path"`
	Digest  string `json:"digest"`
	Matches bool   `json:"matches"`
}

func runBuildIdentityReport(ctx context.Context, args []string, stdout, stderr io.Writer, deps dependencies) int {
	flags := flag.NewFlagSet("report-build-identity", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var options commonOptions
	bindCommonOptions(flags, &options)
	profileID := flags.String("profile", "", "explicit committed deployment profile")
	inspectionPath := flags.String("inspection", "", "typed build inspection observations")
	outputPath := flags.String("output", "", "atomic report output path")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || !validCommonOptions(options) || strings.TrimSpace(*profileID) == "" || *inspectionPath == "" || *outputPath == "" || deps.gitProvider == nil || deps.profileResolver == nil || deps.reportWriter == nil {
		writeCLIError(stderr, "invalid_arguments", "report-build-identity")
		return 2
	}
	var input buildInspectionInput
	if err := decodeStrictBoundedFile(*inspectionPath, &input); err != nil {
		writeCLIError(stderr, string(surfacereport.ErrorSurfaceSchemaInvalid), "inspection")
		return 1
	}
	cached := &cachedIdentityProvider{delegate: deps.gitProvider}
	identity, err := cached.Resolve(ctx, options.repoRoot, options.contractPath, options.schemaPath)
	if err != nil {
		return writeReportProducerError(ctx, stderr, err, deps.reportWriter, *outputPath)
	}
	details := enrichBuildDetails(input, identity)
	report, err := surfacereport.NewBuildIdentityReport(
		ctx, cached, deps.profileResolver,
		options.repoRoot, options.contractPath, options.schemaPath, *profileID,
		details, surfacereport.Outcome{Result: surfacereport.ResultPass, ErrorCodes: []string{}, SkippedChecks: []string{}},
	)
	if err != nil {
		return writeReportProducerError(ctx, stderr, err, deps.reportWriter, *outputPath)
	}
	if err := deps.reportWriter.Write(ctx, *outputPath, report); err != nil {
		return writeDomainError(stderr, err)
	}
	return writeSuccess(stdout, stderr, struct {
		SchemaVersion     string `json:"schemaVersion"`
		Surface           string `json:"surface"`
		DeploymentProfile string `json:"deploymentProfile"`
		Result            string `json:"result"`
		EvidenceClass     string `json:"evidenceClass"`
		Output            string `json:"output"`
	}{
		SchemaVersion: report.SchemaVersion, Surface: report.SurfaceIdentity.Surface,
		DeploymentProfile: report.ReleaseIdentity.DeploymentProfile, Result: string(report.Outcome.Result),
		EvidenceClass: report.Evidence.Class, Output: *outputPath,
	})
}

func runVerifyReport(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("verify-report", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	inputPath := flags.String("input", "", "surface report path")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *inputPath == "" {
		writeCLIError(stderr, "invalid_arguments", "verify-report")
		return 2
	}
	content, err := readBoundedFile(*inputPath)
	if err != nil {
		writeCLIError(stderr, string(surfacereport.ErrorSurfaceSchemaInvalid), "input")
		return 1
	}
	report, err := surfacereport.Decode(content, surfacereport.NewDetailsRegistry())
	if err != nil {
		return writeDomainError(stderr, err)
	}
	return writeSuccess(stdout, stderr, struct {
		SchemaVersion string `json:"schemaVersion"`
		Surface       string `json:"surface"`
		Result        string `json:"result"`
		EvidenceClass string `json:"evidenceClass"`
	}{SchemaVersion: report.SchemaVersion, Surface: report.SurfaceIdentity.Surface, Result: string(report.Outcome.Result), EvidenceClass: report.Evidence.Class})
}

type cachedIdentityProvider struct {
	delegate buildinfo.IdentityProvider
	identity buildinfo.BuildIdentityV1
	err      error
	resolved bool
}

func (p *cachedIdentityProvider) Resolve(ctx context.Context, repoRoot, contractPath, schemaPath string) (buildinfo.BuildIdentityV1, error) {
	if !p.resolved {
		p.identity, p.err = p.delegate.Resolve(ctx, repoRoot, contractPath, schemaPath)
		p.resolved = true
	}
	return p.identity, p.err
}

func enrichBuildDetails(input buildInspectionInput, identity buildinfo.BuildIdentityV1) surfacereport.BuildIdentityDetails {
	binaries := make([]surfacereport.BinaryInspection, 0, len(input.Binaries))
	for _, binary := range input.Binaries {
		binaries = append(binaries, surfacereport.BinaryInspection{
			Name: binary.Name, Path: binary.Path, Digest: binary.Digest, Identity: identity, Matches: binary.Matches,
		})
	}
	return surfacereport.BuildIdentityDetails{
		Binaries:         binaries,
		OCI:              surfacereport.OCIInspection{Image: input.OCI.Image, Digest: input.OCI.Digest, Identity: identity, Matches: input.OCI.Matches},
		PackagedContract: surfacereport.PackagedContractInspection{Path: input.PackagedContract.Path, Digest: input.PackagedContract.Digest, Identity: identity, Matches: input.PackagedContract.Matches},
		ResidualRisks:    input.ResidualRisks,
	}
}

func decodeStrictBoundedFile(path string, destination any) error {
	content, err := readBoundedFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON")
	}
	return nil
}

func readBoundedFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxReportInputBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxReportInputBytes {
		return nil, errors.New("report input exceeds size limit")
	}
	return content, nil
}

func writeReportProducerError(ctx context.Context, stderr io.Writer, producerErr error, writer surfacereport.ReportWriter, outputPath string) int {
	writerErr := writer.Write(ctx, outputPath, surfacereport.SurfaceReportV1{})
	return writeDomainError(stderr, surfacereport.PreserveProducerError(producerErr, writerErr))
}

func runIdentity(ctx context.Context, name string, args []string, stdout, stderr io.Writer, provider buildinfo.IdentityProvider) int {
	options, ok := parseCommonOptions(name, args, stderr)
	if !ok || provider == nil {
		if ok {
			writeCLIError(stderr, "build_identity_missing", "provider")
		}
		return 2
	}
	identity, err := provider.Resolve(ctx, options.repoRoot, options.contractPath, options.schemaPath)
	if err != nil {
		return writeDomainError(stderr, err)
	}
	return writeSuccess(stdout, stderr, identity)
}

func runOperation(ctx context.Context, args []string, stdout, stderr io.Writer, deps dependencies) int {
	flags := flag.NewFlagSet("operation", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var options commonOptions
	bindCommonOptions(flags, &options)
	profileID := flags.String("profile", "", "explicit committed deployment profile")
	kind := flags.String("kind", "", "migrate, deploy, or rollback")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || !validCommonOptions(options) || *profileID == "" || *kind == "" {
		writeCLIError(stderr, "invalid_arguments", "operation")
		return 2
	}
	dispatcher := releasecontract.NewDispatcher(options.contractPath, options.schemaPath, deps.profileResolver, deps.runner)
	if err := dispatcher.Dispatch(ctx, options.repoRoot, *profileID, releasecontract.OperationKind(*kind)); err != nil {
		return writeDomainError(stderr, err)
	}
	return writeSuccess(stdout, stderr, struct {
		SchemaVersion string `json:"schemaVersion"`
		ProfileID     string `json:"profileId"`
		Operation     string `json:"operation"`
		Result        string `json:"result"`
		EvidenceClass string `json:"evidenceClass"`
	}{SchemaVersion: "operation-result/v1", ProfileID: *profileID, Operation: *kind, Result: "pass", EvidenceClass: buildinfo.EvidenceRepositoryLocal})
}

func parseCommonOptions(name string, args []string, stderr io.Writer) (commonOptions, bool) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var options commonOptions
	bindCommonOptions(flags, &options)
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || !validCommonOptions(options) {
		writeCLIError(stderr, "invalid_arguments", name)
		return commonOptions{}, false
	}
	return options, true
}

func bindCommonOptions(flags *flag.FlagSet, options *commonOptions) {
	flags.StringVar(&options.repoRoot, "repo", "", "explicit repository root")
	flags.StringVar(&options.contractPath, "contract", "", "release contract path")
	flags.StringVar(&options.schemaPath, "schema", "", "release contract schema path")
}

func validCommonOptions(options commonOptions) bool {
	return options.repoRoot != "" && options.contractPath != "" && options.schemaPath != ""
}

func writeSuccess(stdout, stderr io.Writer, value any) int {
	if err := json.NewEncoder(stdout).Encode(value); err != nil {
		writeCLIError(stderr, "output_unwritable", "stdout")
		return 1
	}
	return 0
}

func writeDomainError(stderr io.Writer, err error) int {
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
