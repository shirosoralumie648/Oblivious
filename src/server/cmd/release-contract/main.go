package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"reflect"
	"strings"
	"time"

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
	case "inspect-readiness":
		return runInspectReadiness(ctx, args[1:], stdout, stderr, deps)
	case "report-readiness":
		return runReadinessReport(ctx, args[1:], stdout, stderr, deps)
	case "verify-readiness-snapshot":
		return runVerifyReadinessSnapshot(ctx, args[1:], stdout, stderr, deps)
	case "report-deployment":
		return runDeploymentReport(ctx, args[1:], stdout, stderr, deps)
	case "report-protobuf":
		return runProtobufReport(ctx, args[1:], stdout, stderr, deps)
	case "verify-report":
		return runVerifyReport(args[1:], stdout, stderr)
	default:
		writeCLIError(stderr, "invalid_arguments", "subcommand")
		return 2
	}
}

func readinessSnapshotPathOptions(name string, args []string, snapshotRequired bool) (commonOptions, string, string, bool) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var options commonOptions
	bindCommonOptions(flags, &options)
	profileID := flags.String("profile", "", "explicit committed deployment profile")
	snapshotPath := flags.String("snapshot", "", "runtime-produced readiness snapshot")
	flags.String("output", "", "atomic report output path")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || !validCommonOptions(options) || strings.TrimSpace(*profileID) == "" || (snapshotRequired && strings.TrimSpace(*snapshotPath) == "") {
		return commonOptions{}, "", "", false
	}
	return options, *profileID, *snapshotPath, true
}

func loadReadinessSnapshot(ctx context.Context, options commonOptions, profileID, snapshotPath string, deps dependencies) (releasecontract.AuthoredContractV1, releasecontract.DeploymentProfile, buildinfo.BuildIdentityV1, releasecontract.ReadinessSnapshotV1, error) {
	if deps.gitProvider == nil || deps.profileResolver == nil {
		return releasecontract.AuthoredContractV1{}, releasecontract.DeploymentProfile{}, buildinfo.BuildIdentityV1{}, releasecontract.ReadinessSnapshotV1{}, errors.New("trusted readiness resolvers are required")
	}
	identityProvider := &cachedIdentityProvider{delegate: deps.gitProvider}
	identity, err := identityProvider.Resolve(ctx, options.repoRoot, options.contractPath, options.schemaPath)
	if err != nil {
		return releasecontract.AuthoredContractV1{}, releasecontract.DeploymentProfile{}, buildinfo.BuildIdentityV1{}, releasecontract.ReadinessSnapshotV1{}, err
	}
	profile, err := deps.profileResolver.ResolveCommittedProfile(ctx, options.repoRoot, options.contractPath, options.schemaPath, profileID)
	if err != nil {
		return releasecontract.AuthoredContractV1{}, releasecontract.DeploymentProfile{}, buildinfo.BuildIdentityV1{}, releasecontract.ReadinessSnapshotV1{}, err
	}
	contract, err := releasecontract.Load(ctx, options.repoRoot, options.contractPath, options.schemaPath)
	if err != nil {
		return releasecontract.AuthoredContractV1{}, releasecontract.DeploymentProfile{}, buildinfo.BuildIdentityV1{}, releasecontract.ReadinessSnapshotV1{}, err
	}
	var snapshot releasecontract.ReadinessSnapshotV1
	if err := decodeStrictBoundedFile(snapshotPath, &snapshot); err != nil {
		return contract, profile, identity, releasecontract.ReadinessSnapshotV1{}, &surfacereport.ReportError{Code: surfacereport.ErrorSurfaceSchemaInvalid, Field: "snapshot", Err: err}
	}
	return contract, profile, identity, snapshot, nil
}

func evaluateReadinessSnapshot(contract releasecontract.AuthoredContractV1, profile releasecontract.DeploymentProfile, identity buildinfo.BuildIdentityV1, snapshot releasecontract.ReadinessSnapshotV1) (releasecontract.Evaluation, error) {
	if snapshot.SchemaVersion != releasecontract.ReadinessSnapshotSchemaV1 || snapshot.Identity != identity || snapshot.Profile != profile.ID {
		return releasecontract.Evaluation{}, &releasecontract.ReadinessError{Code: releasecontract.CodeBuildIdentityMismatch, Field: "snapshot"}
	}
	evaluation, err := releasecontract.NewEvaluator().Evaluate(contract, identity, profile, snapshot.Generation, snapshot.Observations, time.Now().UTC())
	if err != nil {
		return releasecontract.Evaluation{}, err
	}
	if evaluation.ValidUntil.UnixNano() != snapshot.ValidUntil.UTC().UnixNano() || !reflect.DeepEqual(evaluation.Capabilities, snapshot.Capabilities) {
		return releasecontract.Evaluation{}, &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessUnavailable, Field: "snapshot.capabilities"}
	}
	return evaluation, nil
}

func runInspectReadiness(ctx context.Context, args []string, stdout, stderr io.Writer, deps dependencies) int {
	options, profileID, snapshotPath, ok := readinessSnapshotPathOptions("inspect-readiness", args, true)
	if !ok {
		writeCLIError(stderr, "invalid_arguments", "inspect-readiness")
		return 2
	}
	contract, profile, identity, snapshot, err := loadReadinessSnapshot(ctx, options, profileID, snapshotPath, deps)
	if err != nil {
		return writeDomainError(stderr, err)
	}
	_, evalErr := evaluateReadinessSnapshot(contract, profile, identity, snapshot)
	result := "pass"
	errorCode := ""
	if evalErr != nil {
		result = "fail"
		var readinessErr *releasecontract.ReadinessError
		if errors.As(evalErr, &readinessErr) {
			errorCode = string(readinessErr.Code)
		} else {
			errorCode = string(releasecontract.CodeReadinessUnavailable)
		}
	}
	return writeSuccess(stdout, stderr, struct {
		SchemaVersion string `json:"schemaVersion"`
		Profile       string `json:"profile"`
		Generation    uint64 `json:"generation"`
		Result        string `json:"result"`
		ErrorCode     string `json:"errorCode,omitempty"`
	}{releasecontract.ReadinessSnapshotSchemaV1, profile.ID, snapshot.Generation, result, errorCode})
}

func runReadinessReport(ctx context.Context, args []string, stdout, stderr io.Writer, deps dependencies) int {
	options, profileID, snapshotPath, ok := readinessSnapshotPathOptions("report-readiness", args, true)
	if !ok {
		writeCLIError(stderr, "invalid_arguments", "report-readiness")
		return 2
	}
	if deps.reportWriter == nil {
		writeCLIError(stderr, "invalid_arguments", "reportWriter")
		return 2
	}
	outputPath := readinessOutputPath(args)
	if outputPath == "" {
		writeCLIError(stderr, "invalid_arguments", "output")
		return 2
	}
	reportDeps := deps
	reportDeps.gitProvider = &cachedIdentityProvider{delegate: deps.gitProvider}
	reportDeps.profileResolver = &cachedProfileResolver{delegate: deps.profileResolver}
	contract, profile, identity, snapshot, err := loadReadinessSnapshot(ctx, options, profileID, snapshotPath, reportDeps)
	if err != nil {
		return writeReportProducerError(ctx, stderr, err, deps.reportWriter, outputPath)
	}
	_, evalErr := evaluateReadinessSnapshot(contract, profile, identity, snapshot)
	outcome := surfacereport.Outcome{Result: surfacereport.ResultPass, ErrorCodes: []string{}, SkippedChecks: []string{}}
	if evalErr != nil {
		outcome.Result = surfacereport.ResultFail
		var readinessErr *releasecontract.ReadinessError
		if errors.As(evalErr, &readinessErr) {
			outcome.ErrorCodes = []string{string(readinessErr.Code)}
		} else {
			outcome.ErrorCodes = []string{string(releasecontract.CodeReadinessUnavailable)}
		}
	}
	report, err := surfacereport.NewReadinessReport(ctx, reportDeps.gitProvider, reportDeps.profileResolver, options.repoRoot, options.contractPath, options.schemaPath, profileID, surfacereport.OfflineReadinessInspection(snapshot), outcome)
	if err != nil {
		return writeDomainError(stderr, err)
	}
	if err := deps.reportWriter.Write(ctx, outputPath, report); err != nil {
		return writeDomainError(stderr, err)
	}
	return writeSuccess(stdout, stderr, map[string]any{"schemaVersion": report.SchemaVersion, "surface": report.SurfaceIdentity.Surface, "profile": report.ReleaseIdentity.DeploymentProfile, "result": report.Outcome.Result, "output": outputPath})
}

func runVerifyReadinessSnapshot(ctx context.Context, args []string, stdout, stderr io.Writer, deps dependencies) int {
	options, profileID, snapshotPath, ok := readinessSnapshotPathOptions("verify-readiness-snapshot", args, true)
	if !ok {
		writeCLIError(stderr, "invalid_arguments", "verify-readiness-snapshot")
		return 2
	}
	contract, profile, identity, snapshot, err := loadReadinessSnapshot(ctx, options, profileID, snapshotPath, deps)
	if err != nil {
		return writeDomainError(stderr, err)
	}
	if _, err := evaluateReadinessSnapshot(contract, profile, identity, snapshot); err != nil {
		return writeDomainError(stderr, err)
	}
	return writeSuccess(stdout, stderr, map[string]any{"schemaVersion": releasecontract.ReadinessSnapshotSchemaV1, "profile": profile.ID, "generation": snapshot.Generation, "result": "pass", "evidenceClass": buildinfo.EvidenceRepositoryLocal})
}

func runDeploymentReport(ctx context.Context, args []string, stdout, stderr io.Writer, deps dependencies) int {
	flags := flag.NewFlagSet("report-deployment", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var options commonOptions
	bindCommonOptions(flags, &options)
	profileID := flags.String("profile", "", "explicit committed deployment profile")
	observationPath := flags.String("observation", "", "typed deployment observation")
	outputPath := flags.String("output", "", "atomic report output path")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || !validCommonOptions(options) || strings.TrimSpace(*profileID) == "" || strings.TrimSpace(*observationPath) == "" || strings.TrimSpace(*outputPath) == "" || deps.gitProvider == nil || deps.profileResolver == nil || deps.reportWriter == nil {
		writeCLIError(stderr, "invalid_arguments", "report-deployment")
		return 2
	}
	var details surfacereport.DeploymentDetails
	if err := decodeStrictBoundedFile(*observationPath, &details); err != nil {
		writeCLIError(stderr, string(surfacereport.ErrorSurfaceSchemaInvalid), "observation")
		return 1
	}
	report, err := surfacereport.NewDeploymentReport(ctx, deps.gitProvider, deps.profileResolver, options.repoRoot, options.contractPath, options.schemaPath, *profileID, details, surfacereport.Outcome{Result: surfacereport.ResultPass, ErrorCodes: []string{}, SkippedChecks: []string{}})
	if err != nil {
		return writeReportProducerError(ctx, stderr, err, deps.reportWriter, *outputPath)
	}
	if err := deps.reportWriter.Write(ctx, *outputPath, report); err != nil {
		return writeDomainError(stderr, err)
	}
	return writeSuccess(stdout, stderr, map[string]any{"schemaVersion": report.SchemaVersion, "surface": report.SurfaceIdentity.Surface, "profile": report.ReleaseIdentity.DeploymentProfile, "result": report.Outcome.Result, "output": *outputPath})
}

func runProtobufReport(ctx context.Context, args []string, stdout, stderr io.Writer, deps dependencies) int {
	flags := flag.NewFlagSet("report-protobuf", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var options commonOptions
	bindCommonOptions(flags, &options)
	profileID := flags.String("profile", "", "explicit committed deployment profile")
	observationPath := flags.String("observation", "", "typed protobuf observation")
	outputPath := flags.String("output", "", "atomic report output path")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || !validCommonOptions(options) || strings.TrimSpace(*profileID) == "" || strings.TrimSpace(*observationPath) == "" || strings.TrimSpace(*outputPath) == "" || deps.gitProvider == nil || deps.profileResolver == nil || deps.reportWriter == nil {
		writeCLIError(stderr, "invalid_arguments", "report-protobuf")
		return 2
	}
	var details surfacereport.ProtobufDetails
	if err := decodeStrictBoundedFile(*observationPath, &details); err != nil {
		writeCLIError(stderr, string(surfacereport.ErrorSurfaceSchemaInvalid), "observation")
		return 1
	}
	report, err := surfacereport.NewProtobufReport(
		ctx, deps.gitProvider, deps.profileResolver,
		options.repoRoot, options.contractPath, options.schemaPath, *profileID,
		details, surfacereport.Outcome{Result: surfacereport.ResultPass, ErrorCodes: []string{}, SkippedChecks: []string{}},
	)
	if err != nil {
		return writeReportProducerError(ctx, stderr, err, deps.reportWriter, *outputPath)
	}
	if err := deps.reportWriter.Write(ctx, *outputPath, report); err != nil {
		return writeDomainError(stderr, err)
	}
	return writeSuccess(stdout, stderr, map[string]any{
		"schemaVersion": report.SchemaVersion,
		"surface":       report.SurfaceIdentity.Surface,
		"profile":       report.ReleaseIdentity.DeploymentProfile,
		"result":        report.Outcome.Result,
		"output":        *outputPath,
	})
}

func readinessOutputPath(args []string) string {
	for index := 0; index < len(args); index++ {
		if args[index] != "--output" || index+1 >= len(args) {
			continue
		}
		return strings.TrimSpace(args[index+1])
	}
	return ""
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

type cachedProfileResolver struct {
	delegate releasecontract.ProfileResolver
	profile  releasecontract.DeploymentProfile
	err      error
	resolved bool
}

func (r *cachedProfileResolver) ResolveCommittedProfile(ctx context.Context, repoRoot, contractPath, schemaPath, profileID string) (releasecontract.DeploymentProfile, error) {
	if !r.resolved {
		r.profile, r.err = r.delegate.ResolveCommittedProfile(ctx, repoRoot, contractPath, schemaPath, profileID)
		r.resolved = true
	}
	return r.profile, r.err
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
