package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/releasecontract"
)

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

type commandIdentityProvider struct {
	identity BuildIdentityV1Alias
	calls    int
	repoRoot string
}

type BuildIdentityV1Alias = buildinfo.BuildIdentityV1

func (p *commandIdentityProvider) Resolve(_ context.Context, repoRoot, _, _ string) (buildinfo.BuildIdentityV1, error) {
	p.calls++
	p.repoRoot = repoRoot
	return p.identity, nil
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
