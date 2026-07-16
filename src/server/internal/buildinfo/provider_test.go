package buildinfo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"oblivious/server/internal/releasecontract"
)

const (
	fixtureContractPath = "config/release/contract.v1.json"
	fixtureSchemaPath   = "config/release/contract.schema.json"
)

func TestGitProviderDerivesCleanIdentity(t *testing.T) {
	t.Setenv("GITHUB_SHA", "")
	repoRoot := newIdentityGitFixture(t)
	want := expectedFixtureIdentity(t, repoRoot)
	provider := NewGitProvider()

	got, err := provider.Resolve(context.Background(), repoRoot, fixtureContractPath, fixtureSchemaPath)
	if err != nil {
		t.Fatalf("resolve clean identity: %v", err)
	}
	if got != want {
		t.Fatalf("identity = %#v, want %#v", got, want)
	}
	runFixtureGit(t, repoRoot, "checkout", "--detach", "-q", "HEAD")
	detached, err := provider.Resolve(context.Background(), repoRoot, fixtureContractPath, fixtureSchemaPath)
	if err != nil {
		t.Fatalf("resolve detached identity: %v", err)
	}
	if detached != want {
		t.Fatalf("detached identity = %#v, want %#v", detached, want)
	}
}

func TestGitProviderRejectsDirtyAndSubstitution(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, string)
		code   ErrorCode
	}{
		{
			name: "modified tracked",
			mutate: func(t *testing.T, root string) {
				appendFixtureFile(t, filepath.Join(root, fixtureContractPath), "\n")
			},
			code: ErrorSourceWorktreeDirty,
		},
		{
			name: "staged change",
			mutate: func(t *testing.T, root string) {
				appendFixtureFile(t, filepath.Join(root, "README.md"), "staged\n")
				runFixtureGit(t, root, "add", "README.md")
			},
			code: ErrorSourceWorktreeDirty,
		},
		{
			name: "untracked file",
			mutate: func(t *testing.T, root string) {
				if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("untracked"), 0o600); err != nil {
					t.Fatalf("write untracked file: %v", err)
				}
			},
			code: ErrorSourceWorktreeDirty,
		},
		{
			name: "GITHUB SHA mismatch",
			mutate: func(t *testing.T, _ string) {
				t.Setenv("GITHUB_SHA", strings.Repeat("f", 40))
			},
			code: ErrorBuildIdentityMismatch,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("GITHUB_SHA", "")
			repoRoot := newIdentityGitFixture(t)
			test.mutate(t, repoRoot)
			_, err := NewGitProvider().Resolve(context.Background(), repoRoot, fixtureContractPath, fixtureSchemaPath)
			if !IsCode(err, test.code) {
				t.Fatalf("error = %v, want code %s", err, test.code)
			}
		})
	}

	t.Run("identity-like environment is ignored", func(t *testing.T) {
		t.Setenv("GITHUB_SHA", "")
		t.Setenv("RELEASE_COMMIT", strings.Repeat("f", 40))
		t.Setenv("SOURCE_TREE", strings.Repeat("f", 40))
		t.Setenv("CONTRACT_DIGEST", "sha256:"+strings.Repeat("f", 64))
		repoRoot := newIdentityGitFixture(t)
		got, err := NewGitProvider().Resolve(context.Background(), repoRoot, fixtureContractPath, fixtureSchemaPath)
		if err != nil {
			t.Fatalf("resolve with ignored substitution variables: %v", err)
		}
		if got.ReleaseCommit == strings.Repeat("f", 40) || got.SourceTree == strings.Repeat("f", 40) {
			t.Fatalf("environment substituted identity: %#v", got)
		}
	})
}

func TestEmbeddedProviderRecomputesContractDigest(t *testing.T) {
	repoRoot := newIdentityGitFixture(t)
	identity := expectedFixtureIdentity(t, repoRoot)
	setLinkedIdentityForTest(t, identity)
	provider := NewEmbeddedProvider()

	got, err := provider.Resolve(context.Background(), repoRoot, fixtureContractPath, fixtureSchemaPath)
	if err != nil {
		t.Fatalf("resolve embedded identity: %v", err)
	}
	if got != identity {
		t.Fatalf("embedded identity = %#v, want %#v", got, identity)
	}

	contractPath := filepath.Join(repoRoot, fixtureContractPath)
	content, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("read fixture contract: %v", err)
	}
	mutated := strings.Replace(string(content), `"maxAgeSeconds": 120`, `"maxAgeSeconds": 121`, 1)
	if mutated == string(content) {
		t.Fatal("fixture contract mutation did not apply")
	}
	if err := os.WriteFile(contractPath, []byte(mutated), 0o644); err != nil {
		t.Fatalf("mutate fixture contract: %v", err)
	}
	if _, err := provider.Resolve(context.Background(), repoRoot, fixtureContractPath, fixtureSchemaPath); !IsCode(err, ErrorContractDigestMismatch) {
		t.Fatalf("mutation error = %v, want contract digest mismatch", err)
	}
}

func TestIdentityProvidersUseExplicitRepoRoot(t *testing.T) {
	t.Setenv("GITHUB_SHA", "")
	repoRoot := newIdentityGitFixture(t)
	identity := expectedFixtureIdentity(t, repoRoot)
	setLinkedIdentityForTest(t, identity)
	originalCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	otherCWD := t.TempDir()
	if err := os.Chdir(otherCWD); err != nil {
		t.Fatalf("change cwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalCWD) })

	for name, provider := range map[string]IdentityProvider{
		"git":      NewGitProvider(),
		"embedded": NewEmbeddedProvider(),
	} {
		t.Run(name, func(t *testing.T) {
			got, err := provider.Resolve(context.Background(), repoRoot, fixtureContractPath, fixtureSchemaPath)
			if err != nil {
				t.Fatalf("resolve from divergent cwd: %v", err)
			}
			if got != identity {
				t.Fatalf("identity = %#v, want %#v", got, identity)
			}
		})
	}

	wrongRoot := t.TempDir()
	if _, err := NewGitProvider().Resolve(context.Background(), wrongRoot, fixtureContractPath, fixtureSchemaPath); !IsCode(err, ErrorBuildIdentityMissing) {
		t.Fatalf("wrong-root error = %v, want build identity missing", err)
	}
}

func newIdentityGitFixture(t *testing.T) string {
	t.Helper()
	repoRoot := t.TempDir()
	for _, relative := range []string{fixtureContractPath, fixtureSchemaPath, "scripts/release-profile-operation.sh"} {
		source := filepath.Join(sourceRepositoryRoot(t), filepath.FromSlash(relative))
		destination := filepath.Join(repoRoot, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			t.Fatalf("create fixture directory: %v", err)
		}
		content, err := os.ReadFile(source)
		if err != nil {
			t.Fatalf("read fixture source %s: %v", relative, err)
		}
		mode := os.FileMode(0o644)
		if strings.HasPrefix(relative, "scripts/") {
			mode = 0o755
		}
		if err := os.WriteFile(destination, content, mode); err != nil {
			t.Fatalf("write fixture %s: %v", relative, err)
		}
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("identity fixture\n"), 0o644); err != nil {
		t.Fatalf("write fixture README: %v", err)
	}
	runFixtureGit(t, repoRoot, "init", "-q")
	runFixtureGit(t, repoRoot, "config", "user.name", "Oblivious Test")
	runFixtureGit(t, repoRoot, "config", "user.email", "oblivious-test@example.invalid")
	runFixtureGit(t, repoRoot, "add", ".")
	runFixtureGit(t, repoRoot, "commit", "-q", "-m", "fixture")
	return repoRoot
}

func expectedFixtureIdentity(t *testing.T, repoRoot string) BuildIdentityV1 {
	t.Helper()
	contract, err := releasecontract.Load(context.Background(), repoRoot, fixtureContractPath, fixtureSchemaPath)
	if err != nil {
		t.Fatalf("load fixture contract: %v", err)
	}
	digest, err := releasecontract.Digest(contract)
	if err != nil {
		t.Fatalf("digest fixture contract: %v", err)
	}
	return BuildIdentityV1{
		SchemaVersion:  BuildIdentitySchemaV1,
		ReleaseCommit:  strings.TrimSpace(runFixtureGit(t, repoRoot, "rev-parse", "HEAD^{commit}")),
		SourceTree:     strings.TrimSpace(runFixtureGit(t, repoRoot, "rev-parse", "HEAD^{tree}")),
		ContractDigest: digest,
		Dirty:          false,
		EvidenceClass:  EvidenceRepositoryLocal,
	}
}

func sourceRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve provider test source path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../.."))
}

func runFixtureGit(t *testing.T, repoRoot string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func appendFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open fixture file: %v", err)
	}
	defer file.Close()
	if _, err := file.WriteString(content); err != nil {
		t.Fatalf("append fixture file: %v", err)
	}
}

func setLinkedIdentityForTest(t *testing.T, identity BuildIdentityV1) {
	t.Helper()
	previous := LinkerIdentity{
		ReleaseCommit: linkedReleaseCommit, SourceTree: linkedSourceTree,
		ContractDigest: linkedContractDigest, Dirty: linkedDirty, EvidenceClass: linkedEvidenceClass,
	}
	linkedReleaseCommit = identity.ReleaseCommit
	linkedSourceTree = identity.SourceTree
	linkedContractDigest = identity.ContractDigest
	linkedDirty = "false"
	linkedEvidenceClass = identity.EvidenceClass
	t.Cleanup(func() {
		linkedReleaseCommit = previous.ReleaseCommit
		linkedSourceTree = previous.SourceTree
		linkedContractDigest = previous.ContractDigest
		linkedDirty = previous.Dirty
		linkedEvidenceClass = previous.EvidenceClass
	})
}
