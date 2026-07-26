package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/releasecontract"
	"oblivious/server/internal/surfacereport"
)

func TestRunReportBuildIdentityUsesTrustedResolversAndAtomicWriter(t *testing.T) {
	identity := commandTestIdentity()
	provider := &commandIdentityProvider{identity: identity}
	profiles := &reportProfileResolver{profile: commandCommittedProfile()}
	inspectionPath := writeCommandInspection(t, identity, nil)
	outputPath := filepath.Join(t.TempDir(), "nested", "build-report.json")
	args := reportCommandArgs(commandSourceRoot(t), "monolith", inspectionPath, outputPath)
	var stdout, stderr bytes.Buffer
	exitCode := runWithDependencies(context.Background(), args, &stdout, &stderr, dependencies{
		gitProvider: provider, profileResolver: profiles, reportWriter: surfacereport.NewAtomicWriter(),
	})
	if exitCode != 0 {
		t.Fatalf("report exit=%d stderr=%s", exitCode, stderr.String())
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	report, err := surfacereport.Decode(content, surfacereport.NewDetailsRegistry())
	if err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.ReleaseIdentity.DeploymentProfile != "monolith" || report.ReleaseIdentity.ReleaseCommit != identity.ReleaseCommit {
		t.Fatalf("report identity = %#v", report.ReleaseIdentity)
	}
	if provider.calls != 1 || profiles.calls != 1 || provider.paths != profiles.paths {
		t.Fatalf("resolver calls/paths = %d/%d %#v/%#v", provider.calls, profiles.calls, provider.paths, profiles.paths)
	}
}

func TestRunReportBuildIdentityRejectsUntrustedProfile(t *testing.T) {
	identity := commandTestIdentity()
	for _, test := range []struct {
		name      string
		profileID string
		profile   releasecontract.DeploymentProfile
		err       error
	}{
		{name: "omitted"},
		{name: "microservices", profileID: "microservices", profile: releasecontract.DeploymentProfile{ID: "microservices", Commitment: releasecontract.CommitmentExcluded}},
		{name: "dual", profileID: "dual", profile: releasecontract.DeploymentProfile{ID: "dual", Commitment: releasecontract.CommitmentExcluded}},
		{name: "split", profileID: "split", profile: releasecontract.DeploymentProfile{ID: "split", Commitment: releasecontract.CommitmentConditional}},
		{name: "unknown", profileID: "unknown", err: errors.New("profile_unknown")},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := &commandIdentityProvider{identity: identity}
			profiles := &reportProfileResolver{profile: test.profile, err: test.err}
			inspectionPath := writeCommandInspection(t, identity, nil)
			outputPath := filepath.Join(t.TempDir(), "report.json")
			if err := os.WriteFile(outputPath, []byte("prior-report"), 0o644); err != nil {
				t.Fatalf("write prior output: %v", err)
			}
			args := reportCommandArgs(commandSourceRoot(t), test.profileID, inspectionPath, outputPath)
			var stdout, stderr bytes.Buffer
			if exitCode := runWithDependencies(context.Background(), args, &stdout, &stderr, dependencies{
				gitProvider: provider, profileResolver: profiles, reportWriter: surfacereport.NewAtomicWriter(),
			}); exitCode == 0 {
				t.Fatal("untrusted profile produced report")
			}
			content, err := os.ReadFile(outputPath)
			if err != nil || string(content) != "prior-report" {
				t.Fatalf("prior output changed: %q error=%v", content, err)
			}
			if bytes.Contains(content, []byte(test.profileID)) && test.profileID != "" {
				t.Fatalf("raw profile entered report: %q", content)
			}
		})
	}

	t.Run("caller identity input", func(t *testing.T) {
		provider := &commandIdentityProvider{identity: identity}
		inspectionPath := writeCommandInspection(t, identity, map[string]any{"releaseIdentity": map[string]any{"releaseCommit": strings.Repeat("f", 40)}})
		args := reportCommandArgs(commandSourceRoot(t), "monolith", inspectionPath, filepath.Join(t.TempDir(), "report.json"))
		var stdout, stderr bytes.Buffer
		if exitCode := runWithDependencies(context.Background(), args, &stdout, &stderr, dependencies{
			gitProvider: provider, profileResolver: &reportProfileResolver{profile: commandCommittedProfile()}, reportWriter: surfacereport.NewAtomicWriter(),
		}); exitCode == 0 || provider.calls != 0 {
			t.Fatalf("caller identity exit/provider calls = %d/%d", exitCode, provider.calls)
		}
	})
}

func TestRunVerifyReportRejectsSchemaAndIdentitySplice(t *testing.T) {
	identity := commandTestIdentity()
	inspectionPath := writeCommandInspection(t, identity, nil)
	validPath := filepath.Join(t.TempDir(), "valid-report.json")
	var stdout, stderr bytes.Buffer
	if exitCode := runWithDependencies(context.Background(), reportCommandArgs(commandSourceRoot(t), "monolith", inspectionPath, validPath), &stdout, &stderr, dependencies{
		gitProvider: &commandIdentityProvider{identity: identity}, profileResolver: &reportProfileResolver{profile: commandCommittedProfile()}, reportWriter: surfacereport.NewAtomicWriter(),
	}); exitCode != 0 {
		t.Fatalf("create valid report exit=%d stderr=%s", exitCode, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if exitCode := runWithDependencies(context.Background(), []string{"verify-report", "--input", validPath}, &stdout, &stderr, dependencies{}); exitCode != 0 {
		t.Fatalf("verify valid report exit=%d stderr=%s", exitCode, stderr.String())
	}

	content, err := os.ReadFile(validPath)
	if err != nil {
		t.Fatalf("read valid report: %v", err)
	}
	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "flat", mutate: func(value map[string]any) { value["releaseCommit"] = identity.ReleaseCommit }},
		{name: "unregistered", mutate: func(value map[string]any) { value["surfaceIdentity"].(map[string]any)["surface"] = "unknown" }},
		{name: "identity splice", mutate: func(value map[string]any) {
			binaries := value["evidence"].(map[string]any)["details"].(map[string]any)["binaries"].([]any)
			binaries[0].(map[string]any)["identity"].(map[string]any)["releaseCommit"] = strings.Repeat("f", 40)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var value map[string]any
			if err := json.Unmarshal(content, &value); err != nil {
				t.Fatalf("decode report: %v", err)
			}
			test.mutate(value)
			mutated, err := json.Marshal(value)
			if err != nil {
				t.Fatalf("marshal mutation: %v", err)
			}
			path := filepath.Join(t.TempDir(), "mutated.json")
			if err := os.WriteFile(path, mutated, 0o644); err != nil {
				t.Fatalf("write mutation: %v", err)
			}
			stdout.Reset()
			stderr.Reset()
			if exitCode := runWithDependencies(context.Background(), []string{"verify-report", "--input", path}, &stdout, &stderr, dependencies{}); exitCode == 0 {
				t.Fatal("mutated report verified")
			}
		})
	}
}

func TestRunVerifyAllSurfaceReportTypesContract(t *testing.T) {
	root := t.TempDir()
	if output := os.Getenv("OBLIVIOUS_SURFACE_FIXTURE_DIR"); output != "" {
		root = output
	}
	reportsDir := filepath.Join(root, "reports")
	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		t.Fatalf("create fixture report directory: %v", err)
	}

	identity := commandTestIdentity()
	identityContent, err := json.Marshal(identity)
	if err != nil {
		t.Fatalf("marshal fixture identity: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "identity.json"), identityContent, 0o644); err != nil {
		t.Fatalf("write fixture identity: %v", err)
	}

	reports := commandFixtureSurfaceReports(t, identity)
	if len(reports) != 10 {
		t.Fatalf("fixture report count = %d, want 10", len(reports))
	}
	seen := map[string]struct{}{}
	for _, report := range reports {
		surface := report.SurfaceIdentity.Surface
		if _, duplicate := seen[surface]; duplicate {
			t.Fatalf("duplicate fixture surface %q", surface)
		}
		seen[surface] = struct{}{}
		content, err := surfacereport.Marshal(report, surfacereport.NewDetailsRegistry())
		if err != nil {
			t.Fatalf("marshal %s fixture: %v", surface, err)
		}
		path := filepath.Join(reportsDir, surface+".json")
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatalf("write %s fixture: %v", surface, err)
		}
		var stdout, stderr bytes.Buffer
		if exitCode := runWithDependencies(context.Background(), []string{"verify-report", "--input", path}, &stdout, &stderr, dependencies{}); exitCode != 0 {
			t.Fatalf("verify %s fixture exit=%d stderr=%s", surface, exitCode, stderr.String())
		}
		var status struct {
			SchemaVersion string `json:"schemaVersion"`
			Surface       string `json:"surface"`
			Result        string `json:"result"`
			EvidenceClass string `json:"evidenceClass"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &status); err != nil || status.SchemaVersion != surfacereport.SurfaceReportSchemaV1 || status.Surface != surface || status.Result != "pass" || status.EvidenceClass != buildinfo.EvidenceRepositoryLocal {
			t.Fatalf("unexpected %s verify status: %#v err=%v", surface, status, err)
		}
	}
}

func commandFixtureSurfaceReports(t *testing.T, identity buildinfo.BuildIdentityV1) []surfacereport.SurfaceReportV1 {
	t.Helper()
	digest := func(value string) string { return "sha256:" + strings.Repeat(value, 64) }
	buildDetails := surfacereport.BuildIdentityDetails{
		Binaries: []surfacereport.BinaryInspection{
			{Name: "grpc-smoke", Path: "bin/oblivious-grpc-smoke", Digest: digest("1"), Identity: identity, Matches: true},
			{Name: "migrate", Path: "bin/oblivious-migrate", Digest: digest("2"), Identity: identity, Matches: true},
			{Name: "server", Path: "bin/oblivious-server", Digest: digest("3"), Identity: identity, Matches: true},
		},
		OCI:              surfacereport.OCIInspection{Image: "oblivious:fixture", Digest: digest("4"), Identity: identity, Matches: true},
		PackagedContract: surfacereport.PackagedContractInspection{Path: "config/release/contract.v1.json", Digest: identity.ContractDigest, Identity: identity, Matches: true},
		ResidualRisks:    []string{"external target not inspected"},
	}
	protobufDetails := surfacereport.ProtobufDetails{
		SchemaVersion: "protobuf-observation/v1", ManifestDigest: "sha256:3b225e40c4a7659d07c2638f128ac2087483d2ebab737f4533415793dcad54eb",
		ToolVersions:           surfacereport.ProtobufToolVersions{Protoc: "25.1", ProtocGenGo: "1.36.11", ProtocGenGoGRPC: "1.6.2"},
		GeneratedHeaderVersion: "v4.25.1", SourceCount: 10, OutputCount: 21, ManagedCount: 9, SourceOnlyCount: 1,
		Regeneration: surfacereport.ProtobufRegeneration{Result: "pass", GeneratedCount: 21, Digest: "sha256:c74af7cc805309b00807b9ecfbbbce31ec2737c4fb726130a70ecbbc168da96a"},
		ErrorCodes:   []string{}, SkippedChecks: []string{},
	}
	transportDetails := surfacereport.FrontendTransportDetails{
		SchemaVersion: "frontend-transport-observation/v1", SidecarDigest: digest("5"), SourceDigest: digest("6"), ConfigDigest: digest("7"),
		OperationCount: 264, CoreCount: 264, CompatibleCount: 264, TaxonomyDigest: digest("8"), UnresolvedCount: 0,
		ErrorCodes: []string{}, SkippedChecks: []string{},
	}
	exposureDetails := surfacereport.FrontendExposureDetails{
		SchemaVersion: "frontend-exposure-observation/v1", SidecarDigest: digest("5"), SourceDigest: digest("6"), ConfigDigest: digest("7"),
		ExposureCount: 80, CatalogCount: 14, NavigationCount: 67, GeneratedConsumerCount: 374, ProjectionDigest: digest("9"), UnresolvedCount: 0,
		ErrorCodes: []string{}, SkippedChecks: []string{},
	}
	staticDetails := surfacereport.MigrationStaticDetails{
		DatabaseKind: "postgresql-pgvector", IdentityCount: 2, FileCount: 2, IdentityDigest: digest("a"), StaticMetadataDigest: digest("b"),
		NonMonolithDispositionCounts: map[string]int{"clickhouse-non-monolith": 1, "microservice-non-monolith": 1},
	}
	ledgerDetails := surfacereport.MigrationLedgerDetails{DatabaseKind: "postgresql-pgvector", RowCount: 2, IdentityDigest: digest("a"), MatchesStatic: true}
	replayDetails := surfacereport.MigrationReplayDetails{
		DatabaseKind: "postgresql-pgvector", ReplayMode: "docker-ephemeral", ResourceOwnership: "owned-disposable", InitialLedgerRows: 0,
		FirstApply: surfacereport.MigrationApplyCounts{Applied: 2, Skipped: 0}, SecondApply: surfacereport.MigrationApplyCounts{Applied: 0, Skipped: 2},
		FinalLedgerRows: 2, StaticDigest: digest("a"), LedgerDigest: digest("a"), CleanupResult: "succeeded",
	}

	return []surfacereport.SurfaceReportV1{
		commandFixtureReport(t, identity, surfacereport.BuildIdentitySurfaceID, "config/release/contract.v1.json", "binary-oci-packaged-contract-inspector", identity.ContractDigest, commandFixtureDetailsDigest(t, surfacereport.BuildIdentitySurfaceID, buildDetails), "repository", "inspection", map[string]string{}, buildDetails),
		commandFixtureReport(t, identity, surfacereport.ReadinessSurfaceID, "config/release/contract.v1.json", "runtime-readiness-inspector", identity.ContractDigest, commandFixtureDetailsDigest(t, surfacereport.ReadinessSurfaceID, surfacereport.ReadinessDetails{Generation: 1, CheckedAt: "2026-07-22T10:50:00Z", ValidUntil: "2026-07-22T10:52:00Z"}), "repository", "offline", map[string]string{}, surfacereport.ReadinessDetails{Generation: 1, CheckedAt: "2026-07-22T10:50:00Z", ValidUntil: "2026-07-22T10:52:00Z"}),
		commandFixtureReport(t, identity, surfacereport.DeploymentSurfaceID, "deploy/kubernetes/app-deployment.yaml", "readiness-deployment-harness", identity.ContractDigest, commandFixtureDetailsDigest(t, surfacereport.DeploymentSurfaceID, surfacereport.DeploymentDetails{Profile: "monolith", CanonicalWorkload: "deploy/kubernetes/app-deployment.yaml", StartupEndpoint: "/livez", LivenessEndpoint: "/livez", ReadinessEndpoint: "/readyz", AuditStorage: "emptyDir", MigrationState: "applied_and_validated", HarnessResult: "passed"}), "repository", "deployment-harness", map[string]string{}, surfacereport.DeploymentDetails{Profile: "monolith", CanonicalWorkload: "deploy/kubernetes/app-deployment.yaml", StartupEndpoint: "/livez", LivenessEndpoint: "/livez", ReadinessEndpoint: "/readyz", AuditStorage: "emptyDir", MigrationState: "applied_and_validated", HarnessResult: "passed"}),
		commandFixtureReport(t, identity, surfacereport.HTTPRuntimeSurfaceID, "docs/api/openapi.yaml", "runtime-route-registry", digest("c"), digest("c"), "repository", "runtime-registry-parity", map[string]string{}, surfacereport.HTTPRuntimeDetails{OperationCount: 264, MountedCount: 264, DescriptorCount: 264, CoreDigest: digest("c"), RuntimeDigest: digest("c"), MediaProbeCount: 1, ParityResult: "pass"}),
		commandFixtureReport(t, identity, surfacereport.FrontendTransportSurfaceID, "src/web/src", "frontend-transport-verifier", transportDetails.SourceDigest, commandFixtureDetailsDigest(t, surfacereport.FrontendTransportSurfaceID, transportDetails), "repository", "compiler-sidecar", map[string]string{}, transportDetails),
		commandFixtureReport(t, identity, surfacereport.FrontendExposureSurfaceID, "src/web/src", "product-exposure-verifier", exposureDetails.SourceDigest, commandFixtureDetailsDigest(t, surfacereport.FrontendExposureSurfaceID, exposureDetails), "repository", "compiler-sidecar", map[string]string{}, exposureDetails),
		commandFixtureReport(t, identity, surfacereport.ProtobufSurfaceID, "config/release/protobuf-toolchain.v1.json", "tracked-protobuf-generated-consumers", protobufDetails.ManifestDigest, commandFixtureDetailsDigest(t, surfacereport.ProtobufSurfaceID, protobufDetails), "repository", "fresh-regeneration", map[string]string{"protoc": "25.1", "protoc-gen-go": "1.36.11", "protoc-gen-go-grpc": "1.6.2"}, protobufDetails),
		commandFixtureReport(t, identity, surfacereport.MigrationStaticSurfaceID, "src/server/migrations", "monolith-migration-static-inventory", staticDetails.StaticMetadataDigest, staticDetails.IdentityDigest, "repository", "static", map[string]string{}, staticDetails),
		commandFixtureReport(t, identity, surfacereport.MigrationLedgerSurfaceID, "schema_migrations(version,checksum)", "monolith-runtime-ledger", ledgerDetails.IdentityDigest, ledgerDetails.IdentityDigest, "repository-local-database", "ledger", map[string]string{}, ledgerDetails),
		commandFixtureReport(t, identity, surfacereport.MigrationReplaySurfaceID, "src/server/migrations+schema_migrations(version,checksum)", "monolith-migration-replay", replayDetails.StaticDigest, replayDetails.LedgerDigest, "local-docker", "replay", map[string]string{}, replayDetails),
	}
}

func commandFixtureReport(t *testing.T, identity buildinfo.BuildIdentityV1, surface, canonicalSource, consumer, sourceDigest, consumerDigest, environment, mode string, toolVersions map[string]string, details any) surfacereport.SurfaceReportV1 {
	t.Helper()
	registry := surfacereport.NewDetailsRegistry()
	raw, err := registry.MarshalDetails(surface, details)
	if err != nil {
		t.Fatalf("marshal %s fixture details: %v", surface, err)
	}
	return surfacereport.NewReport(
		surfacereport.ReleaseIdentity{ReleaseCommit: identity.ReleaseCommit, SourceTree: identity.SourceTree, ContractDigest: identity.ContractDigest, DeploymentProfile: "monolith", Dirty: false, EvidenceClass: buildinfo.EvidenceRepositoryLocal},
		surfacereport.SurfaceIdentity{Surface: surface, CanonicalSource: canonicalSource, Consumer: consumer, Version: "v1", SourceDigest: sourceDigest, ConsumerDigest: consumerDigest},
		surfacereport.Drift{Missing: []string{}, Extra: []string{}, Incompatible: []string{}},
		surfacereport.Evidence{Class: buildinfo.EvidenceRepositoryLocal, Environment: environment, Mode: mode, CheckedAt: "2026-07-22T10:51:00Z", ToolVersions: toolVersions, Details: raw},
		surfacereport.Outcome{Result: surfacereport.ResultPass, ErrorCodes: []string{}, SkippedChecks: []string{}},
	)
}

func commandFixtureDetailsDigest(t *testing.T, surface string, details any) string {
	t.Helper()
	raw, err := surfacereport.NewDetailsRegistry().MarshalDetails(surface, details)
	if err != nil {
		t.Fatalf("marshal %s fixture details for digest: %v", surface, err)
	}
	var normalized any
	if err := json.Unmarshal(raw, &normalized); err != nil {
		t.Fatalf("normalize %s fixture details: %v", surface, err)
	}
	canonical, err := json.Marshal(normalized)
	if err != nil {
		t.Fatalf("canonicalize %s fixture details: %v", surface, err)
	}
	sum := sha256.Sum256(canonical)
	return fmt.Sprintf("sha256:%x", sum)
}

func TestRunReportPreservesProducerFailure(t *testing.T) {
	producerErr := &buildinfo.IdentityError{Code: buildinfo.ErrorSourceWorktreeDirty, Field: "worktree"}
	provider := &commandIdentityProvider{err: producerErr}
	writer := &commandReportWriter{err: &surfacereport.ReportError{Code: surfacereport.ErrorReportOutputUnwritable, Field: "parent"}}
	args := reportCommandArgs(commandSourceRoot(t), "monolith", writeCommandInspection(t, commandTestIdentity(), nil), filepath.Join(t.TempDir(), "report.json"))
	var stdout, stderr bytes.Buffer
	exitCode := runWithDependencies(context.Background(), args, &stdout, &stderr, dependencies{
		gitProvider: provider, profileResolver: &reportProfileResolver{profile: commandCommittedProfile()}, reportWriter: writer,
	})
	if exitCode == 0 || writer.calls != 1 {
		t.Fatalf("producer/writer failure exit/calls = %d/%d", exitCode, writer.calls)
	}
	if !bytes.Contains(stderr.Bytes(), []byte(string(buildinfo.ErrorSourceWorktreeDirty))) || bytes.Contains(stderr.Bytes(), []byte(string(surfacereport.ErrorReportOutputUnwritable))) {
		t.Fatalf("primary error not preserved: %s", stderr.String())
	}
}

func TestRunValidateDigestAndIdentity(t *testing.T) {
	repoRoot := commandSourceRoot(t)
	identity := commandTestIdentity()
	provider := &commandIdentityProvider{identity: identity}
	deps := dependencies{gitProvider: provider, embeddedProvider: provider, profileResolver: releasecontract.NewFileProfileResolver()}
	common := []string{"--repo", repoRoot, "--contract", "config/release/contract.v1.json", "--schema", "config/release/contract.schema.json"}

	for _, subcommand := range []string{"validate", "digest", "identity"} {
		t.Run(subcommand, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exitCode := runWithDependencies(context.Background(), append([]string{subcommand}, common...), &stdout, &stderr, deps)
			if exitCode != 0 {
				t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
			}
			var output map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
				t.Fatalf("decode output: %v", err)
			}
			if output["evidenceClass"] != buildinfo.EvidenceRepositoryLocal {
				t.Fatalf("output = %#v", output)
			}
		})
	}
	if provider.calls != 1 {
		t.Fatalf("identity provider calls = %d, want 1", provider.calls)
	}
}

func TestRunOperationRequiresExplicitProfile(t *testing.T) {
	repoRoot := commandSourceRoot(t)
	common := []string{"--repo", repoRoot, "--contract", "config/release/contract.v1.json", "--schema", "config/release/contract.schema.json", "--kind", "deploy"}
	runner := &commandRunner{}
	deps := dependencies{profileResolver: releasecontract.NewFileProfileResolver(), runner: runner}
	var stdout, stderr bytes.Buffer
	if exitCode := runWithDependencies(context.Background(), append([]string{"operation"}, common...), &stdout, &stderr, deps); exitCode == 0 {
		t.Fatal("operation without profile passed")
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls after missing profile = %d", runner.calls)
	}

	stdout.Reset()
	stderr.Reset()
	excluded := append(append([]string{"operation"}, common...), "--profile", "dual")
	if exitCode := runWithDependencies(context.Background(), excluded, &stdout, &stderr, deps); exitCode == 0 {
		t.Fatal("excluded profile operation passed")
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls after excluded profile = %d", runner.calls)
	}

	stdout.Reset()
	stderr.Reset()
	profile := commandCommittedProfile()
	deps.profileResolver = commandProfileResolver{profile: profile}
	success := append(append([]string{"operation"}, common...), "--profile", "monolith")
	if exitCode := runWithDependencies(context.Background(), success, &stdout, &stderr, deps); exitCode != 0 {
		t.Fatalf("recorded operation exit=%d stderr=%s", exitCode, stderr.String())
	}
	if runner.calls != 1 || !strings.HasSuffix(runner.executable, "scripts/release-profile-operation.sh") {
		t.Fatalf("runner = %#v", runner)
	}
}

func TestRunRejectsIdentityOverrideFlags(t *testing.T) {
	provider := &commandIdentityProvider{identity: commandTestIdentity()}
	repoRoot := commandSourceRoot(t)
	common := []string{"--repo", repoRoot, "--contract", "config/release/contract.v1.json", "--schema", "config/release/contract.schema.json"}
	for _, override := range []string{"--release-commit", "--source-tree", "--contract-digest", "--identity-json"} {
		t.Run(override, func(t *testing.T) {
			args := append(append([]string{"identity"}, common...), override, "caller-value")
			var stdout, stderr bytes.Buffer
			if exitCode := runWithDependencies(context.Background(), args, &stdout, &stderr, dependencies{gitProvider: provider}); exitCode == 0 {
				t.Fatalf("override %s passed", override)
			}
			if !bytes.Contains(stderr.Bytes(), []byte(`"code":"invalid_arguments"`)) {
				t.Fatalf("stderr = %s", stderr.String())
			}
		})
	}
	if provider.calls != 0 {
		t.Fatalf("provider called %d times for rejected flags", provider.calls)
	}
}

func TestRunInspectHasNoExternalSideEffects(t *testing.T) {
	provider := &commandIdentityProvider{identity: commandTestIdentity()}
	runner := &commandRunner{}
	args := []string{"inspect", "--repo", "/packaged/root", "--contract", "contract.json", "--schema", "schema.json"}
	var stdout, stderr bytes.Buffer
	exitCode := runWithDependencies(context.Background(), args, &stdout, &stderr, dependencies{embeddedProvider: provider, runner: runner})
	if exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	if provider.calls != 1 || provider.repoRoot != "/packaged/root" || runner.calls != 0 {
		t.Fatalf("provider=%#v runner=%#v", provider, runner)
	}
}

func TestRunReadinessAndDeploymentReportCommandsContract(t *testing.T) {
	identity := commandTestIdentity()
	repoRoot := commandSourceRoot(t)
	observation := map[string]any{
		"profile": "monolith", "canonicalWorkload": "deploy/kubernetes/app-deployment.yaml",
		"startupEndpoint": "/livez", "livenessEndpoint": "/livez", "readinessEndpoint": "/readyz",
		"auditStorage": "emptyDir", "migrationState": "applied_and_validated", "harnessResult": "passed",
	}
	content, err := json.Marshal(observation)
	if err != nil {
		t.Fatalf("marshal observation: %v", err)
	}
	observationPath := filepath.Join(t.TempDir(), "deployment.json")
	if err := os.WriteFile(observationPath, content, 0o644); err != nil {
		t.Fatalf("write observation: %v", err)
	}
	outputPath := filepath.Join(t.TempDir(), "deployment-report.json")
	args := []string{"report-deployment", "--repo", repoRoot, "--contract", "config/release/contract.v1.json", "--schema", "config/release/contract.schema.json", "--profile", "monolith", "--observation", observationPath, "--output", outputPath}
	var stdout, stderr bytes.Buffer
	if exitCode := runWithDependencies(context.Background(), args, &stdout, &stderr, dependencies{gitProvider: &commandIdentityProvider{identity: identity}, profileResolver: &reportProfileResolver{profile: commandCommittedProfile()}, reportWriter: surfacereport.NewAtomicWriter()}); exitCode != 0 {
		t.Fatalf("deployment report exit=%d stderr=%s", exitCode, stderr.String())
	}
	reportContent, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read deployment report: %v", err)
	}
	report, err := surfacereport.Decode(reportContent, surfacereport.NewDetailsRegistry())
	if err != nil || report.SurfaceIdentity.Surface != surfacereport.DeploymentSurfaceID {
		t.Fatalf("deployment report = %#v err=%v", report, err)
	}
	for _, override := range []string{"--release-commit", "--source-tree", "--contract-digest", "--generation", "--checked-at", "--skipped-checks"} {
		stdout.Reset()
		stderr.Reset()
		invalid := append([]string{"report-deployment"}, args[1:]...)
		invalid = append(invalid, override, "caller-value")
		if exitCode := runWithDependencies(context.Background(), invalid, &stdout, &stderr, dependencies{gitProvider: &commandIdentityProvider{identity: identity}, profileResolver: &reportProfileResolver{profile: commandCommittedProfile()}, reportWriter: surfacereport.NewAtomicWriter()}); exitCode == 0 {
			t.Fatalf("override %s passed", override)
		}
	}
}

func TestRunProtobufSurfaceReportContract(t *testing.T) {
	repoRoot := commandSourceRoot(t)
	identity := commandTestIdentity()
	observationPath := writeProtobufObservation(t, nil)
	outputPath := filepath.Join(t.TempDir(), "nested", "protobuf-report.json")
	provider := &commandIdentityProvider{identity: identity}
	profiles := &reportProfileResolver{profile: commandCommittedProfile()}
	writer := &countingAtomicReportWriter{delegate: surfacereport.NewAtomicWriter()}
	args := protobufReportCommandArgs(repoRoot, observationPath, outputPath)
	var stdout, stderr bytes.Buffer
	if exitCode := runWithDependencies(context.Background(), args, &stdout, &stderr, dependencies{
		gitProvider: provider, profileResolver: profiles, reportWriter: writer,
	}); exitCode != 0 {
		t.Fatalf("protobuf report exit=%d stderr=%s", exitCode, stderr.String())
	}
	if provider.calls != 1 || profiles.calls != 1 || writer.calls != 1 || provider.paths != profiles.paths {
		t.Fatalf("protobuf resolver/writer calls = %d/%d/%d paths=%#v/%#v", provider.calls, profiles.calls, writer.calls, provider.paths, profiles.paths)
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read protobuf report: %v", err)
	}
	report, err := surfacereport.Decode(content, surfacereport.NewDetailsRegistry())
	if err != nil {
		t.Fatalf("decode protobuf report: %v", err)
	}
	if report.SurfaceIdentity.Surface != surfacereport.ProtobufSurfaceID || report.ReleaseIdentity.ReleaseCommit != identity.ReleaseCommit || report.ReleaseIdentity.DeploymentProfile != "monolith" {
		t.Fatalf("protobuf report identity = %#v / %#v", report.SurfaceIdentity, report.ReleaseIdentity)
	}

	t.Run("rejects caller authority flags without replacing prior output", func(t *testing.T) {
		for _, override := range []string{"--release-commit", "--source-tree", "--contract-digest", "--evidence-class", "--skipped-checks"} {
			t.Run(override, func(t *testing.T) {
				priorPath := filepath.Join(t.TempDir(), "protobuf-report.json")
				if err := os.WriteFile(priorPath, []byte("prior-report"), 0o644); err != nil {
					t.Fatalf("write prior report: %v", err)
				}
				invalid := protobufReportCommandArgs(repoRoot, observationPath, priorPath)
				invalid = append(invalid, override, "caller-value")
				localProvider := &commandIdentityProvider{identity: identity}
				stdout.Reset()
				stderr.Reset()
				if exitCode := runWithDependencies(context.Background(), invalid, &stdout, &stderr, dependencies{
					gitProvider: localProvider, profileResolver: &reportProfileResolver{profile: commandCommittedProfile()}, reportWriter: surfacereport.NewAtomicWriter(),
				}); exitCode == 0 {
					t.Fatalf("protobuf override %s passed", override)
				}
				prior, err := os.ReadFile(priorPath)
				if err != nil || string(prior) != "prior-report" || localProvider.calls != 0 {
					t.Fatalf("override changed authority/output: calls=%d content=%q err=%v", localProvider.calls, prior, err)
				}
			})
		}
	})

	t.Run("rejects unknown and identity-bearing observations atomically", func(t *testing.T) {
		for _, extra := range []map[string]any{
			{"unknown": true},
			{"releaseIdentity": map[string]any{"releaseCommit": strings.Repeat("f", 40)}},
			{"evidenceClass": "target-environment"},
		} {
			priorPath := filepath.Join(t.TempDir(), "protobuf-report.json")
			if err := os.WriteFile(priorPath, []byte("prior-report"), 0o644); err != nil {
				t.Fatalf("write prior report: %v", err)
			}
			localProvider := &commandIdentityProvider{identity: identity}
			stdout.Reset()
			stderr.Reset()
			if exitCode := runWithDependencies(context.Background(), protobufReportCommandArgs(repoRoot, writeProtobufObservation(t, extra), priorPath), &stdout, &stderr, dependencies{
				gitProvider: localProvider, profileResolver: &reportProfileResolver{profile: commandCommittedProfile()}, reportWriter: surfacereport.NewAtomicWriter(),
			}); exitCode == 0 {
				t.Fatalf("identity-bearing protobuf observation passed: %#v", extra)
			}
			prior, err := os.ReadFile(priorPath)
			if err != nil || string(prior) != "prior-report" || localProvider.calls != 0 {
				t.Fatalf("invalid observation changed authority/output: calls=%d content=%q err=%v", localProvider.calls, prior, err)
			}
		}
	})

	t.Run("rejects failed validator result and preserves output", func(t *testing.T) {
		failedPath := writeProtobufObservation(t, map[string]any{
			"regeneration": map[string]any{"result": "fail", "generatedCount": 21, "digest": "sha256:c74af7cc805309b00807b9ecfbbbce31ec2737c4fb726130a70ecbbc168da96a"},
		})
		priorPath := filepath.Join(t.TempDir(), "protobuf-report.json")
		if err := os.WriteFile(priorPath, []byte("prior-report"), 0o644); err != nil {
			t.Fatalf("write prior report: %v", err)
		}
		failedWriter := &countingAtomicReportWriter{delegate: surfacereport.NewAtomicWriter()}
		stdout.Reset()
		stderr.Reset()
		if exitCode := runWithDependencies(context.Background(), protobufReportCommandArgs(repoRoot, failedPath, priorPath), &stdout, &stderr, dependencies{
			gitProvider: &commandIdentityProvider{identity: identity}, profileResolver: &reportProfileResolver{profile: commandCommittedProfile()}, reportWriter: failedWriter,
		}); exitCode == 0 {
			t.Fatal("failed protobuf regeneration produced a report")
		}
		prior, err := os.ReadFile(priorPath)
		if err != nil || string(prior) != "prior-report" || failedWriter.calls != 1 {
			t.Fatalf("producer failure changed output/calls: content=%q calls=%d err=%v", prior, failedWriter.calls, err)
		}
	})
}

func TestRunFrontendSurfaceReportsContract(t *testing.T) {
	repoRoot := commandSourceRoot(t)
	identity := commandTestIdentity()
	sidecarDigest := "sha256:" + strings.Repeat("1", 64)
	sourceDigest := "sha256:" + strings.Repeat("2", 64)
	configDigest := "sha256:" + strings.Repeat("3", 64)
	transport := surfacereport.FrontendTransportDetails{
		SchemaVersion: "frontend-transport-observation/v1",
		SidecarDigest: sidecarDigest, SourceDigest: sourceDigest, ConfigDigest: configDigest,
		OperationCount: 12, CoreCount: 12, CompatibleCount: 12,
		TaxonomyDigest: "sha256:" + strings.Repeat("4", 64), UnresolvedCount: 0,
		ErrorCodes: []string{}, SkippedChecks: []string{},
	}
	exposure := surfacereport.FrontendExposureDetails{
		SchemaVersion: "frontend-exposure-observation/v1",
		SidecarDigest: sidecarDigest, SourceDigest: sourceDigest, ConfigDigest: configDigest,
		ExposureCount: 80, CatalogCount: 14, NavigationCount: 67, GeneratedConsumerCount: 264,
		ProjectionDigest: "sha256:" + strings.Repeat("5", 64), UnresolvedCount: 0,
		ErrorCodes: []string{}, SkippedChecks: []string{},
	}
	tempRoot := t.TempDir()
	transportObservation := writeFrontendObservation(t, tempRoot, "transport-observation.json", transport)
	exposureObservation := writeFrontendObservation(t, tempRoot, "exposure-observation.json", exposure)
	commands := []struct {
		name, observation, output, surface string
	}{
		{"report-frontend-transport", transportObservation, filepath.Join(tempRoot, "reports", "frontend-transport.json"), surfacereport.FrontendTransportSurfaceID},
		{"report-frontend-exposure", exposureObservation, filepath.Join(tempRoot, "reports", "frontend-exposure.json"), surfacereport.FrontendExposureSurfaceID},
	}

	for _, command := range commands {
		t.Run(command.name, func(t *testing.T) {
			args := frontendReportCommandArgs(command.name, repoRoot, command.observation, sidecarDigest, command.output)
			var stdout, stderr bytes.Buffer
			exitCode := runWithDependencies(context.Background(), args, &stdout, &stderr, dependencies{
				gitProvider:     &commandIdentityProvider{identity: identity},
				profileResolver: &reportProfileResolver{profile: commandCommittedProfile()},
				reportWriter:    surfacereport.NewAtomicWriter(),
			})
			if exitCode != 0 {
				t.Fatalf("%s exit=%d stderr=%s", command.name, exitCode, stderr.String())
			}
			content, err := os.ReadFile(command.output)
			if err != nil {
				t.Fatalf("read %s report: %v", command.name, err)
			}
			report, err := surfacereport.Decode(content, surfacereport.NewDetailsRegistry())
			if err != nil {
				t.Fatalf("decode %s report: %v", command.name, err)
			}
			if report.SurfaceIdentity.Surface != command.surface || report.ReleaseIdentity.ReleaseCommit != identity.ReleaseCommit || report.ReleaseIdentity.SourceTree != identity.SourceTree || report.ReleaseIdentity.ContractDigest != identity.ContractDigest || report.ReleaseIdentity.DeploymentProfile != "monolith" || report.ReleaseIdentity.EvidenceClass != buildinfo.EvidenceRepositoryLocal {
				t.Fatalf("unexpected %s authority: %#v %#v", command.name, report.SurfaceIdentity, report.ReleaseIdentity)
			}
			var details struct {
				SidecarDigest string `json:"sidecarDigest"`
			}
			if err := json.Unmarshal(report.Evidence.Details, &details); err != nil || details.SidecarDigest != sidecarDigest {
				t.Fatalf("unexpected %s sidecar digest: %#v err=%v", command.name, details, err)
			}
		})
	}

	for _, command := range commands {
		t.Run(command.name+" rejects sidecar splice", func(t *testing.T) {
			output := filepath.Join(tempRoot, command.name+"-splice.json")
			args := frontendReportCommandArgs(command.name, repoRoot, command.observation, "sha256:"+strings.Repeat("f", 64), output)
			var stdout, stderr bytes.Buffer
			if exitCode := runWithDependencies(context.Background(), args, &stdout, &stderr, dependencies{
				gitProvider:     &commandIdentityProvider{identity: identity},
				profileResolver: &reportProfileResolver{profile: commandCommittedProfile()},
				reportWriter:    surfacereport.NewAtomicWriter(),
			}); exitCode == 0 {
				t.Fatalf("%s accepted sidecar digest splice", command.name)
			}
			if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("%s wrote spliced report: %v", command.name, err)
			}
		})
	}

	for _, forbidden := range []string{"--release-commit", "--source-tree", "--contract-digest", "--evidence-class", "--skipped-checks"} {
		t.Run("rejects caller claim "+forbidden, func(t *testing.T) {
			output := filepath.Join(tempRoot, "forged-"+strings.TrimPrefix(forbidden, "--")+".json")
			args := append(frontendReportCommandArgs("report-frontend-transport", repoRoot, transportObservation, sidecarDigest, output), forbidden, "forged")
			var stdout, stderr bytes.Buffer
			provider := &commandIdentityProvider{identity: identity}
			if exitCode := runWithDependencies(context.Background(), args, &stdout, &stderr, dependencies{
				gitProvider:     provider,
				profileResolver: &reportProfileResolver{profile: commandCommittedProfile()},
				reportWriter:    surfacereport.NewAtomicWriter(),
			}); exitCode == 0 {
				t.Fatalf("frontend report accepted caller claim flag %s", forbidden)
			}
			if provider.calls != 0 {
				t.Fatalf("frontend report resolved identity for rejected flag %s", forbidden)
			}
		})
	}
}

func TestReadinessOutputPathScansCommonFlags(t *testing.T) {
	args := []string{"--repo", "/repo", "--contract", "contract.json", "--schema", "schema.json", "--profile", "monolith", "--snapshot", "snapshot.json", "--output", "/tmp/readiness.json"}
	if got := readinessOutputPath(args); got != "/tmp/readiness.json" {
		t.Fatalf("readiness output path = %q", got)
	}
	if got := readinessOutputPath([]string{"--output"}); got != "" {
		t.Fatalf("incomplete output flag = %q", got)
	}
}

type commandIdentityProvider struct {
	identity BuildIdentityV1Alias
	err      error
	calls    int
	repoRoot string
	paths    commandResolverPaths
}

type BuildIdentityV1Alias = buildinfo.BuildIdentityV1

func (p *commandIdentityProvider) Resolve(_ context.Context, repoRoot, contract, schema string) (buildinfo.BuildIdentityV1, error) {
	p.calls++
	p.repoRoot = repoRoot
	p.paths = commandResolverPaths{repo: repoRoot, contract: contract, schema: schema}
	return p.identity, p.err
}

type commandResolverPaths struct {
	repo, contract, schema string
}

type reportProfileResolver struct {
	profile releasecontract.DeploymentProfile
	err     error
	calls   int
	paths   commandResolverPaths
}

func (r *reportProfileResolver) ResolveCommittedProfile(_ context.Context, repo, contract, schema, _ string) (releasecontract.DeploymentProfile, error) {
	r.calls++
	r.paths = commandResolverPaths{repo: repo, contract: contract, schema: schema}
	return r.profile, r.err
}

type commandReportWriter struct {
	err   error
	calls int
}

type countingAtomicReportWriter struct {
	delegate surfacereport.ReportWriter
	calls    int
}

func (w *countingAtomicReportWriter) Write(ctx context.Context, destination string, report surfacereport.SurfaceReportV1) error {
	w.calls++
	return w.delegate.Write(ctx, destination, report)
}

func (w *commandReportWriter) Write(context.Context, string, surfacereport.SurfaceReportV1) error {
	w.calls++
	return w.err
}

type commandRunner struct {
	calls      int
	executable string
}

func (r *commandRunner) Run(_ context.Context, executable string, _, _ []string) error {
	r.calls++
	r.executable = executable
	return nil
}

type commandProfileResolver struct {
	profile releasecontract.DeploymentProfile
}

func (r commandProfileResolver) ResolveCommittedProfile(context.Context, string, string, string, string) (releasecontract.DeploymentProfile, error) {
	return r.profile, nil
}

func commandCommittedProfile() releasecontract.DeploymentProfile {
	ref := releasecontract.OperationRef{ProfileID: "monolith", Path: "scripts/release-profile-operation.sh", Argv: []string{"monolith", "deploy"}}
	return releasecontract.DeploymentProfile{
		ID: "monolith", Commitment: releasecontract.CommitmentCommitted,
		Operations: releasecontract.ProfileOperations{Migrate: ref, Deploy: ref, Rollback: ref},
	}
}

func commandTestIdentity() buildinfo.BuildIdentityV1 {
	return buildinfo.BuildIdentityV1{
		SchemaVersion: buildinfo.BuildIdentitySchemaV1,
		ReleaseCommit: strings.Repeat("a", 40), SourceTree: strings.Repeat("b", 40),
		ContractDigest: "sha256:" + strings.Repeat("c", 64), Dirty: false,
		EvidenceClass: buildinfo.EvidenceRepositoryLocal,
	}
}

func commandSourceRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve command test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../.."))
}

func reportCommandArgs(repoRoot, profile, inspection, output string) []string {
	args := []string{
		"report-build-identity", "--repo", repoRoot,
		"--contract", "config/release/contract.v1.json",
		"--schema", "config/release/contract.schema.json",
		"--inspection", inspection, "--output", output,
	}
	if profile != "" {
		args = append(args, "--profile", profile)
	}
	return args
}

func writeCommandInspection(t *testing.T, identity buildinfo.BuildIdentityV1, extra map[string]any) string {
	t.Helper()
	digest := "sha256:" + strings.Repeat("d", 64)
	value := map[string]any{
		"binaries": []any{
			map[string]any{"name": "grpc-smoke", "path": "/usr/local/bin/oblivious-grpc-smoke", "digest": digest, "matches": true},
			map[string]any{"name": "migrate", "path": "/usr/local/bin/oblivious-migrate", "digest": digest, "matches": true},
			map[string]any{"name": "server", "path": "/usr/local/bin/oblivious-server", "digest": digest, "matches": true},
		},
		"oci":              map[string]any{"image": "oblivious:test", "digest": digest, "matches": true},
		"packagedContract": map[string]any{"path": "/app/config/release/contract.v1.json", "digest": identity.ContractDigest, "matches": true},
		"residualRisks":    []string{"external target not inspected"},
	}
	for key, item := range extra {
		value[key] = item
	}
	content, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal inspection: %v", err)
	}
	path := filepath.Join(t.TempDir(), "inspection.json")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write inspection: %v", err)
	}
	return path
}

func protobufReportCommandArgs(repoRoot, observation, output string) []string {
	return []string{
		"report-protobuf", "--repo", repoRoot,
		"--contract", "config/release/contract.v1.json",
		"--schema", "config/release/contract.schema.json",
		"--profile", "monolith", "--observation", observation, "--output", output,
	}
}

func frontendReportCommandArgs(command, repoRoot, observation, sidecarDigest, output string) []string {
	return []string{
		command, "--repo", repoRoot,
		"--contract", "config/release/contract.v1.json",
		"--schema", "config/release/contract.schema.json",
		"--profile", "monolith", "--observation", observation,
		"--sidecar-digest", sidecarDigest, "--output", output,
	}
}

func writeFrontendObservation(t *testing.T, root, name string, value any) string {
	t.Helper()
	content, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal frontend observation: %v", err)
	}
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write frontend observation: %v", err)
	}
	return path
}

func writeProtobufObservation(t *testing.T, extra map[string]any) string {
	t.Helper()
	value := map[string]any{
		"schemaVersion":  "protobuf-observation/v1",
		"manifestDigest": "sha256:3b225e40c4a7659d07c2638f128ac2087483d2ebab737f4533415793dcad54eb",
		"toolVersions": map[string]any{
			"protoc": "25.1", "protoc-gen-go": "1.36.11", "protoc-gen-go-grpc": "1.6.2",
		},
		"generatedHeaderVersion": "v4.25.1",
		"sourceCount":            10, "outputCount": 21, "managedCount": 9, "sourceOnlyCount": 1,
		"regeneration": map[string]any{
			"result": "pass", "generatedCount": 21,
			"digest": "sha256:c74af7cc805309b00807b9ecfbbbce31ec2737c4fb726130a70ecbbc168da96a",
		},
		"errorCodes": []string{}, "skippedChecks": []string{},
	}
	for key, item := range extra {
		value[key] = item
	}
	content, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal protobuf observation: %v", err)
	}
	path := filepath.Join(t.TempDir(), "protobuf-observation.json")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write protobuf observation: %v", err)
	}
	return path
}
