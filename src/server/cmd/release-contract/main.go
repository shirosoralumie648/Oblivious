package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/releasecontract"
)

type dependencies struct {
	gitProvider      buildinfo.IdentityProvider
	embeddedProvider buildinfo.IdentityProvider
	profileResolver  releasecontract.ProfileResolver
	runner           releasecontract.Runner
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
	default:
		writeCLIError(stderr, "invalid_arguments", "subcommand")
		return 2
	}
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
