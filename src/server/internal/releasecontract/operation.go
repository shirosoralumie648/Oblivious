package releasecontract

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	ErrorOperationUnknown           ErrorCode = "operation_unknown"
	ErrorOperationProfileMismatch   ErrorCode = "operation_profile_mismatch"
	ErrorOperationPathInvalid       ErrorCode = "operation_path_invalid"
	ErrorOperationRunnerFailed      ErrorCode = "operation_runner_failed"
	ErrorOperationDispatcherInvalid ErrorCode = "operation_dispatcher_invalid"
)

type Runner interface {
	Run(context.Context, string, []string, []string) error
}

type Dispatcher struct {
	contractPath string
	schemaPath   string
	resolver     ProfileResolver
	runner       Runner
}

func NewDispatcher(contractPath, schemaPath string, resolver ProfileResolver, runner Runner) *Dispatcher {
	if resolver == nil {
		resolver = NewFileProfileResolver()
	}
	if runner == nil {
		runner = commandRunner{}
	}
	return &Dispatcher{
		contractPath: contractPath,
		schemaPath:   schemaPath,
		resolver:     resolver,
		runner:       runner,
	}
}

func (d *Dispatcher) Dispatch(ctx context.Context, repoRoot, profileID string, kind OperationKind) error {
	if d == nil || d.resolver == nil || d.runner == nil {
		return contractError(ErrorOperationDispatcherInvalid, "dispatcher", "uninitialized", nil)
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(profileID) == "" {
		return contractError(ErrorProfileRequired, "profileId", "missing", nil)
	}

	profile, err := d.resolver.ResolveCommittedProfile(ctx, repoRoot, d.contractPath, d.schemaPath, profileID)
	if err != nil {
		return err
	}
	if profile.ID != profileID {
		return contractError(ErrorOperationProfileMismatch, "profileId", profile.ID, nil)
	}
	if profile.Commitment != CommitmentCommitted {
		if profile.Commitment == CommitmentExcluded {
			return contractError(ErrorProfileExcluded, "profileId", profileID, nil)
		}
		return contractError(ErrorProfileNotCommitted, "profileId", profileID, nil)
	}

	operation, err := operationForKind(profile, kind)
	if err != nil {
		return err
	}
	if operation.ProfileID != profile.ID {
		return contractError(ErrorOperationProfileMismatch, "operation.profileId", operation.ProfileID, nil)
	}
	executable, err := ResolveOperationPath(repoRoot, operation.Path)
	if err != nil {
		return err
	}
	if err := d.runner.Run(ctx, executable, append([]string(nil), operation.Argv...), minimalOperationEnvironment()); err != nil {
		return contractError(ErrorOperationRunnerFailed, "operation", string(kind), err)
	}
	return nil
}

func ResolveOperationPath(repoRoot, operationPath string) (string, error) {
	root, err := canonicalRepoRoot(repoRoot)
	if err != nil {
		return "", err
	}
	if operationPath == "" || filepath.IsAbs(operationPath) || strings.ContainsRune(operationPath, '\x00') || filepath.ToSlash(filepath.Clean(operationPath)) != operationPath || !strings.HasPrefix(operationPath, "scripts/") {
		return "", contractError(ErrorOperationPathInvalid, "operation.path", operationPath, nil)
	}

	candidate := filepath.Join(root, filepath.FromSlash(operationPath))
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", contractError(ErrorOperationPathInvalid, "operation.path", operationPath, nil)
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", contractError(ErrorOperationPathInvalid, "operation.path", operationPath, nil)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", contractError(ErrorOperationPathInvalid, "operation.path", operationPath, nil)
	}
	return resolved, nil
}

func operationForKind(profile DeploymentProfile, kind OperationKind) (OperationRef, error) {
	switch kind {
	case OperationMigrate:
		return profile.Operations.Migrate, nil
	case OperationDeploy:
		return profile.Operations.Deploy, nil
	case OperationRollback:
		return profile.Operations.Rollback, nil
	default:
		return OperationRef{}, contractError(ErrorOperationUnknown, "operation", string(kind), nil)
	}
}

func minimalOperationEnvironment() []string {
	pathValue := os.Getenv("PATH")
	if pathValue == "" {
		pathValue = "/usr/bin:/bin"
	}
	return []string{"PATH=" + pathValue}
}

type commandRunner struct{}

func (commandRunner) Run(ctx context.Context, executable string, argv, environment []string) error {
	command := exec.CommandContext(ctx, executable, argv...)
	command.Env = append([]string(nil), environment...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}
