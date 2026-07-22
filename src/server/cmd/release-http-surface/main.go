package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	stdhttp "net/http"
	"os"
	"path/filepath"
	"strings"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/config"
	runtimehttp "oblivious/server/internal/http"
	"oblivious/server/internal/releasecontract"
	"oblivious/server/internal/surfacereport"
)

const (
	expectedHTTPRuntimeOperationCount = 197
	httpRuntimeManifestSchemaV2       = "route-surface-manifest/v2"
	maxHTTPRuntimeManifestBytes       = 4 << 20
)

type dependencies struct {
	identityProvider buildinfo.IdentityProvider
	profileResolver  releasecontract.ProfileResolver
	reportWriter     surfacereport.ReportWriter
	snapshotBuilder  func() ([]runtimehttp.RouteSurfaceDescriptor, error)
	manifestLoader   func(string) (httpSurfaceManifest, error)
}

type commandOptions struct {
	repoRoot, contractPath, schemaPath  string
	profileID, manifestPath, outputPath string
}

type httpSurfaceManifest struct {
	SchemaVersion    string                                    `json:"schemaVersion"`
	GeneratedFrom    json.RawMessage                           `json:"generatedFrom"`
	ProjectionDigest string                                    `json:"projectionDigest"`
	Scope            runtimehttp.PublicOperationScopeV1        `json:"scope"`
	Operations       []runtimehttp.OperationContractMetadataV1 `json:"operations"`
	RouteSamples     json.RawMessage                           `json:"routeSamples"`
}

type commandError struct {
	code  string
	field string
	err   error
}

func (e *commandError) Error() string {
	if e.field == "" {
		return e.code
	}
	return e.code + ": field=" + e.field
}

func (e *commandError) Unwrap() error { return e.err }

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return runWithDependencies(ctx, args, stdout, stderr, dependencies{
		identityProvider: buildinfo.NewGitProvider(),
		profileResolver:  releasecontract.NewFileProfileResolver(),
		reportWriter:     surfacereport.NewAtomicWriter(),
		snapshotBuilder:  buildHTTPRuntimeSnapshot,
		manifestLoader:   loadHTTPRuntimeManifest,
	})
}

func runWithDependencies(ctx context.Context, args []string, stdout, stderr io.Writer, deps dependencies) int {
	options, ok := parseOptions(args)
	if !ok {
		writeCLIError(stderr, "invalid_arguments", "arguments")
		return 2
	}
	if ctx == nil || deps.identityProvider == nil || deps.profileResolver == nil || deps.reportWriter == nil || deps.snapshotBuilder == nil || deps.manifestLoader == nil {
		writeCLIError(stderr, "invalid_arguments", "dependencies")
		return 2
	}

	snapshot, err := deps.snapshotBuilder()
	if err != nil {
		return writeDomainError(stderr, &commandError{code: "http_runtime_unavailable", field: "runtime", err: err})
	}
	if err := validateHTTPRuntimeSnapshot(snapshot); err != nil {
		return writeDomainError(stderr, err)
	}

	resolvedManifest, err := resolveManifestPath(options.repoRoot, options.manifestPath)
	if err != nil {
		return writeDomainError(stderr, err)
	}
	manifest, err := deps.manifestLoader(resolvedManifest)
	if err != nil {
		return writeDomainError(stderr, err)
	}
	expected, err := manifestOperationsForSnapshot(manifest, snapshot)
	if err != nil {
		return writeDomainError(stderr, err)
	}
	observation, err := runtimehttp.CompareRouteSurfaceSnapshot(manifest.Scope, expected, snapshot)
	if err != nil {
		return writeDomainError(stderr, err)
	}
	if observation.OperationCount != expectedHTTPRuntimeOperationCount || observation.MountedCount != expectedHTTPRuntimeOperationCount || observation.DescriptorCount != expectedHTTPRuntimeOperationCount || observation.MediaProbeCount <= 0 || observation.ParityResult != "pass" {
		return writeDomainError(stderr, &commandError{code: "http_runtime_inventory_invalid", field: "observation"})
	}

	report, err := surfacereport.NewHTTPRuntimeReport(
		ctx,
		deps.identityProvider,
		deps.profileResolver,
		options.repoRoot,
		options.contractPath,
		options.schemaPath,
		options.profileID,
		observation,
	)
	if err != nil {
		return writeDomainError(stderr, err)
	}
	if err := deps.reportWriter.Write(ctx, options.outputPath, report); err != nil {
		return writeDomainError(stderr, err)
	}
	return writeSuccess(stdout, stderr, struct {
		SchemaVersion string `json:"schemaVersion"`
		Surface       string `json:"surface"`
		Profile       string `json:"profile"`
		Result        string `json:"result"`
		EvidenceClass string `json:"evidenceClass"`
		Operations    int    `json:"operations"`
		MediaProbes   int    `json:"mediaProbes"`
	}{
		SchemaVersion: report.SchemaVersion,
		Surface:       report.SurfaceIdentity.Surface,
		Profile:       report.ReleaseIdentity.DeploymentProfile,
		Result:        string(report.Outcome.Result),
		EvidenceClass: report.ReleaseIdentity.EvidenceClass,
		Operations:    observation.OperationCount,
		MediaProbes:   observation.MediaProbeCount,
	})
}

func parseOptions(args []string) (commandOptions, bool) {
	flags := flag.NewFlagSet("release-http-surface", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var options commandOptions
	flags.StringVar(&options.repoRoot, "repo", "", "explicit repository root")
	flags.StringVar(&options.contractPath, "contract", "", "release contract path")
	flags.StringVar(&options.schemaPath, "schema", "", "release contract schema path")
	flags.StringVar(&options.profileID, "profile", "", "committed deployment profile")
	flags.StringVar(&options.manifestPath, "manifest", "", "compare-only route surface manifest")
	flags.StringVar(&options.outputPath, "output", "", "atomic report output path")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return commandOptions{}, false
	}
	for _, value := range []string{options.repoRoot, options.contractPath, options.schemaPath, options.profileID, options.manifestPath, options.outputPath} {
		if strings.TrimSpace(value) == "" {
			return commandOptions{}, false
		}
	}
	if !filepath.IsAbs(options.repoRoot) {
		return commandOptions{}, false
	}
	return options, true
}

func buildHTTPRuntimeSnapshot() (snapshot []runtimehttp.RouteSurfaceDescriptor, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			snapshot = nil
			err = fmt.Errorf("construct runtime registrar")
		}
	}()
	var registrar *runtimehttp.RouteSurfaceRegistrar
	runtimehttp.NewRouterWithOptions(config.Config{
		CORSAllowedOrigins: []string{"http://127.0.0.1"},
		Env:                "test",
		Port:               8080,
		SessionCookieName:  "oblivious_session",
		LLMTimeoutMS:       30000,
		ModelDefaultName:   "release-http-surface",
	}, nil, runtimehttp.RouterOptions{
		RouteSurfaceRegistrarFactory: func(mux *stdhttp.ServeMux, policies runtimehttp.RouteSurfacePolicies) (*runtimehttp.RouteSurfaceRegistrar, error) {
			created, createErr := runtimehttp.NewRouteSurfaceRegistrar(mux, policies)
			registrar = created
			return created, createErr
		},
	})
	if registrar == nil {
		return nil, &commandError{code: "http_runtime_unavailable", field: "registrar"}
	}
	return registrar.Snapshot(), nil
}

func validateHTTPRuntimeSnapshot(snapshot []runtimehttp.RouteSurfaceDescriptor) error {
	if len(snapshot) != expectedHTTPRuntimeOperationCount {
		return &commandError{code: "http_runtime_inventory_invalid", field: "descriptorCount"}
	}
	seen := make(map[string]struct{}, len(snapshot))
	for _, descriptor := range snapshot {
		key := httpSurfaceKey(descriptor.Method, descriptor.Path)
		if key == " " || strings.TrimSpace(descriptor.OperationID) == "" || strings.TrimSpace(descriptor.CapabilityID) == "" {
			return &commandError{code: "http_runtime_inventory_invalid", field: "descriptor"}
		}
		if _, exists := seen[key]; exists {
			return &commandError{code: "http_runtime_inventory_invalid", field: "duplicate"}
		}
		seen[key] = struct{}{}
	}
	return nil
}

func loadHTTPRuntimeManifest(path string) (httpSurfaceManifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return httpSurfaceManifest{}, &commandError{code: "http_runtime_manifest_invalid", field: "manifest", err: err}
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxHTTPRuntimeManifestBytes+1))
	if err != nil || len(content) == 0 || len(content) > maxHTTPRuntimeManifestBytes {
		return httpSurfaceManifest{}, &commandError{code: "http_runtime_manifest_invalid", field: "manifest", err: err}
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var manifest httpSurfaceManifest
	if err := decoder.Decode(&manifest); err != nil {
		return httpSurfaceManifest{}, &commandError{code: "http_runtime_manifest_invalid", field: "manifest", err: err}
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return httpSurfaceManifest{}, &commandError{code: "http_runtime_manifest_invalid", field: "manifest"}
	}
	if manifest.SchemaVersion != httpRuntimeManifestSchemaV2 || strings.TrimSpace(manifest.ProjectionDigest) == "" || len(manifest.Operations) == 0 || len(manifest.Scope.Dispositions) == 0 {
		return httpSurfaceManifest{}, &commandError{code: "http_runtime_manifest_invalid", field: "manifest"}
	}
	return manifest, nil
}

func manifestOperationsForSnapshot(manifest httpSurfaceManifest, snapshot []runtimehttp.RouteSurfaceDescriptor) ([]runtimehttp.OperationContractMetadataV1, error) {
	wanted := make(map[string]struct{}, len(snapshot))
	for _, descriptor := range snapshot {
		wanted[httpSurfaceKey(descriptor.Method, descriptor.Path)] = struct{}{}
	}
	all := make(map[string]struct{}, len(manifest.Operations))
	expected := make([]runtimehttp.OperationContractMetadataV1, 0, len(snapshot))
	for _, operation := range manifest.Operations {
		key := httpSurfaceKey(operation.Method, operation.NormalizedPath)
		if _, exists := all[key]; exists {
			return nil, &commandError{code: "http_runtime_manifest_invalid", field: "duplicateOperation"}
		}
		all[key] = struct{}{}
		if _, ok := wanted[key]; ok {
			expected = append(expected, operation)
		}
	}
	if len(expected) != expectedHTTPRuntimeOperationCount {
		return nil, &commandError{code: "http_runtime_manifest_invalid", field: "operationClosure"}
	}
	return expected, nil
}

func resolveManifestPath(repoRoot, requested string) (string, error) {
	root, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return "", &commandError{code: "http_runtime_manifest_invalid", field: "repo"}
	}
	candidate := requested
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, filepath.FromSlash(candidate))
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", &commandError{code: "http_runtime_manifest_invalid", field: "manifest"}
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", &commandError{code: "http_runtime_manifest_invalid", field: "manifest"}
	}
	return resolved, nil
}

func httpSurfaceKey(method, path string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + " " + strings.TrimSpace(path)
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
	var routeErr *runtimehttp.RouteSurfaceContractError
	if errors.As(err, &routeErr) {
		writeCLIError(stderr, routeErr.Code, "runtime")
		return 1
	}
	var localErr *commandError
	if errors.As(err, &localErr) {
		writeCLIError(stderr, localErr.code, localErr.field)
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
