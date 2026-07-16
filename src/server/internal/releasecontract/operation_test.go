package releasecontract

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDispatchCommittedOperationPassesLiteralArgv(t *testing.T) {
	repoRoot := t.TempDir()
	outputPath := filepath.Join(repoRoot, "argv.txt")
	sentinelPath := filepath.Join(repoRoot, "shell-evaluated")
	executable := writeOperationExecutable(t, repoRoot, "literal.sh", "#!/usr/bin/env bash\nprintf '%s' \"$1\" > \"$2\"\n")
	literal := "$(touch " + sentinelPath + "); echo unsafe"
	profile := committedOperationProfile("scripts/literal.sh", []string{literal, outputPath})
	dispatcher := NewDispatcher("unused-contract", "unused-schema", staticProfileResolver{profile: profile}, nil)

	if err := dispatcher.Dispatch(context.Background(), repoRoot, "monolith", OperationDeploy); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read literal argv output: %v", err)
	}
	if string(got) != literal {
		t.Fatalf("literal argv = %q, want %q", got, literal)
	}
	if _, err := os.Stat(sentinelPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("shell metacharacters were evaluated; sentinel stat error = %v", err)
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil || resolved == "" {
		t.Fatalf("resolve executable: %v", err)
	}
}

func TestDispatchRejectsBeforeRunner(t *testing.T) {
	repoRoot := t.TempDir()
	writeOperationExecutable(t, repoRoot, "valid.sh", "#!/usr/bin/env bash\nexit 0\n")
	sentinel := filepath.Join(repoRoot, "runner-called")

	tests := []struct {
		name      string
		profileID string
		kind      OperationKind
		resolver  ProfileResolver
		wantCode  ErrorCode
	}{
		{
			name: "missing explicit profile", profileID: "", kind: OperationDeploy,
			resolver: staticProfileResolver{profile: committedOperationProfile("scripts/valid.sh", []string{"monolith", "deploy"})},
			wantCode: ErrorProfileRequired,
		},
		{
			name: "unknown profile", profileID: "unknown", kind: OperationDeploy,
			resolver: staticProfileResolver{err: contractError(ErrorProfileUnknown, "profileId", "unknown", nil)},
			wantCode: ErrorProfileUnknown,
		},
		{
			name: "excluded profile", profileID: "dual", kind: OperationDeploy,
			resolver: staticProfileResolver{err: contractError(ErrorProfileExcluded, "profileId", "dual", nil)},
			wantCode: ErrorProfileExcluded,
		},
		{
			name: "resolver returned a different profile", profileID: "monolith", kind: OperationDeploy,
			resolver: staticProfileResolver{profile: DeploymentProfile{ID: "other", Commitment: CommitmentCommitted}},
			wantCode: ErrorOperationProfileMismatch,
		},
		{
			name: "operation profile mismatch", profileID: "monolith", kind: OperationDeploy,
			resolver: staticProfileResolver{profile: committedOperationProfileFor("monolith", "other", "scripts/valid.sh", []string{"other", "deploy"})},
			wantCode: ErrorOperationProfileMismatch,
		},
		{
			name: "unknown operation", profileID: "monolith", kind: OperationKind("unknown"),
			resolver: staticProfileResolver{profile: committedOperationProfile("scripts/valid.sh", []string{"monolith", "deploy"})},
			wantCode: ErrorOperationUnknown,
		},
		{
			name: "unsafe path", profileID: "monolith", kind: OperationDeploy,
			resolver: staticProfileResolver{profile: committedOperationProfile("../valid.sh", []string{"monolith", "deploy"})},
			wantCode: ErrorOperationPathInvalid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &recordingOperationRunner{sentinel: sentinel}
			dispatcher := NewDispatcher("unused-contract", "unused-schema", test.resolver, runner)
			err := dispatcher.Dispatch(context.Background(), repoRoot, test.profileID, test.kind)
			assertOperationErrorCode(t, err, test.wantCode)
			if runner.calls != 0 {
				t.Fatalf("runner calls = %d, want 0", runner.calls)
			}
			if _, err := os.Stat(sentinel); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("rejected dispatch created sentinel: %v", err)
			}
		})
	}
}

func TestResolveOperationPathRejectsEscapes(t *testing.T) {
	repoRoot := t.TempDir()
	valid := writeOperationExecutable(t, repoRoot, "valid.sh", "#!/usr/bin/env bash\nexit 0\n")
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "outside.sh")
	if err := os.WriteFile(outside, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write outside executable: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(repoRoot, "scripts", "escape.sh")); err != nil {
		t.Fatalf("create escape symlink: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "config"), 0o755); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	outsideAllowlist := filepath.Join(repoRoot, "config", "tool.sh")
	if err := os.WriteFile(outsideAllowlist, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write outside-allowlist executable: %v", err)
	}

	resolved, err := ResolveOperationPath(repoRoot, "scripts/valid.sh")
	if err != nil {
		t.Fatalf("resolve valid path: %v", err)
	}
	if resolved != valid {
		t.Fatalf("resolved path = %q, want %q", resolved, valid)
	}

	tests := []struct {
		name string
		path string
	}{
		{name: "absolute", path: outside},
		{name: "traversal", path: "scripts/../config/tool.sh"},
		{name: "nul", path: "scripts/valid.sh\x00suffix"},
		{name: "outside allowlist", path: "config/tool.sh"},
		{name: "symlink escape", path: "scripts/escape.sh"},
		{name: "missing", path: "scripts/missing.sh"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ResolveOperationPath(repoRoot, test.path)
			assertOperationErrorCode(t, err, ErrorOperationPathInvalid)
		})
	}
}

type staticProfileResolver struct {
	profile DeploymentProfile
	err     error
}

func (r staticProfileResolver) ResolveCommittedProfile(context.Context, string, string, string, string) (DeploymentProfile, error) {
	return r.profile, r.err
}

type recordingOperationRunner struct {
	calls       int
	executable  string
	argv        []string
	environment []string
	sentinel    string
}

func (r *recordingOperationRunner) Run(_ context.Context, executable string, argv, environment []string) error {
	r.calls++
	r.executable = executable
	r.argv = append([]string(nil), argv...)
	r.environment = append([]string(nil), environment...)
	if r.sentinel != "" {
		return os.WriteFile(r.sentinel, []byte("called"), 0o600)
	}
	return nil
}

func committedOperationProfile(path string, argv []string) DeploymentProfile {
	return committedOperationProfileFor("monolith", "monolith", path, argv)
}

func committedOperationProfileFor(profileID, operationProfileID, path string, argv []string) DeploymentProfile {
	return DeploymentProfile{
		ID:         profileID,
		Commitment: CommitmentCommitted,
		Operations: ProfileOperations{
			Migrate:  OperationRef{ProfileID: operationProfileID, Path: path, Argv: append([]string(nil), argv...)},
			Deploy:   OperationRef{ProfileID: operationProfileID, Path: path, Argv: append([]string(nil), argv...)},
			Rollback: OperationRef{ProfileID: operationProfileID, Path: path, Argv: append([]string(nil), argv...)},
		},
	}
}

func writeOperationExecutable(t *testing.T, repoRoot, name, content string) string {
	t.Helper()
	directory := filepath.Join(repoRoot, "scripts")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create scripts directory: %v", err)
	}
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write operation executable: %v", err)
	}
	return path
}

func assertOperationErrorCode(t *testing.T, err error, want ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want code %s", want)
	}
	var contractErr *ContractError
	if !errors.As(err, &contractErr) {
		t.Fatalf("error %T = %v, want ContractError", err, err)
	}
	if contractErr.Code != want {
		t.Fatalf("error code = %s, want %s (error: %v)", contractErr.Code, want, err)
	}
}

func TestOperationRunnerReceivesMinimalEnvironment(t *testing.T) {
	repoRoot := t.TempDir()
	writeOperationExecutable(t, repoRoot, "valid.sh", "#!/usr/bin/env bash\nexit 0\n")
	runner := &recordingOperationRunner{}
	dispatcher := NewDispatcher("unused-contract", "unused-schema", staticProfileResolver{profile: committedOperationProfile("scripts/valid.sh", []string{"literal;value", "$(not-evaluated)"})}, runner)

	if err := dispatcher.Dispatch(context.Background(), repoRoot, "monolith", OperationDeploy); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !reflect.DeepEqual(runner.argv, []string{"literal;value", "$(not-evaluated)"}) {
		t.Fatalf("argv = %#v", runner.argv)
	}
	if len(runner.environment) != 1 || !strings.HasPrefix(runner.environment[0], "PATH=") {
		t.Fatalf("environment = %#v, want PATH only", runner.environment)
	}
}
