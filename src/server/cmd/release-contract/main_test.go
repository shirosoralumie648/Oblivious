package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
