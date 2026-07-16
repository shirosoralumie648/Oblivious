package buildinfo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"oblivious/server/internal/releasecontract"
)

const (
	gitCommandTimeout = 10 * time.Second
	gitOutputLimit    = 64 << 10
)

type IdentityProvider interface {
	Resolve(ctx context.Context, repoRoot, contractPath, schemaPath string) (BuildIdentityV1, error)
}

type GitProvider struct{}

func NewGitProvider() *GitProvider { return &GitProvider{} }

func (p *GitProvider) Resolve(ctx context.Context, repoRoot, contractPath, schemaPath string) (BuildIdentityV1, error) {
	if !filepath.IsAbs(repoRoot) {
		return BuildIdentityV1{}, identityError(ErrorBuildIdentityMismatch, "repoRoot", nil)
	}
	status, err := runGit(ctx, repoRoot, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return BuildIdentityV1{}, identityError(ErrorBuildIdentityMissing, "gitStatus", err)
	}
	if strings.TrimSpace(status) != "" {
		return BuildIdentityV1{}, identityError(ErrorSourceWorktreeDirty, "worktree", nil)
	}
	releaseCommit, err := runGit(ctx, repoRoot, "rev-parse", "HEAD^{commit}")
	if err != nil {
		return BuildIdentityV1{}, identityError(ErrorBuildIdentityMissing, "releaseCommit", err)
	}
	sourceTree, err := runGit(ctx, repoRoot, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return BuildIdentityV1{}, identityError(ErrorBuildIdentityMissing, "sourceTree", err)
	}
	contract, err := releasecontract.Load(ctx, repoRoot, contractPath, schemaPath)
	if err != nil {
		return BuildIdentityV1{}, identityError(ErrorContractDigestMismatch, "contractDigest", err)
	}
	contractDigest, err := releasecontract.Digest(contract)
	if err != nil {
		return BuildIdentityV1{}, identityError(ErrorContractDigestMismatch, "contractDigest", err)
	}
	identity := BuildIdentityV1{
		SchemaVersion:  BuildIdentitySchemaV1,
		ReleaseCommit:  strings.TrimSpace(releaseCommit),
		SourceTree:     strings.TrimSpace(sourceTree),
		ContractDigest: contractDigest,
		Dirty:          false,
		EvidenceClass:  EvidenceRepositoryLocal,
	}
	if err := ValidateIdentity(identity); err != nil {
		return BuildIdentityV1{}, err
	}
	if expected := strings.TrimSpace(os.Getenv("GITHUB_SHA")); expected != "" && expected != identity.ReleaseCommit {
		return BuildIdentityV1{}, identityError(ErrorBuildIdentityMismatch, "GITHUB_SHA", nil)
	}
	return identity, nil
}

type EmbeddedProvider struct{}

func NewEmbeddedProvider() *EmbeddedProvider { return &EmbeddedProvider{} }

func (p *EmbeddedProvider) Resolve(ctx context.Context, repoRoot, contractPath, schemaPath string) (BuildIdentityV1, error) {
	identity, err := ParseLinkedIdentity()
	if err != nil {
		return BuildIdentityV1{}, err
	}
	contract, err := releasecontract.Load(ctx, repoRoot, contractPath, schemaPath)
	if err != nil {
		return BuildIdentityV1{}, identityError(ErrorContractDigestMismatch, "contractDigest", err)
	}
	digest, err := releasecontract.Digest(contract)
	if err != nil {
		return BuildIdentityV1{}, identityError(ErrorContractDigestMismatch, "contractDigest", err)
	}
	if digest != identity.ContractDigest {
		return BuildIdentityV1{}, identityError(ErrorContractDigestMismatch, "contractDigest", nil)
	}
	return identity, nil
}

func runGit(ctx context.Context, repoRoot string, args ...string) (string, error) {
	if ctx == nil {
		return "", errors.New("nil context")
	}
	commandContext, cancel := context.WithTimeout(ctx, gitCommandTimeout)
	defer cancel()
	command := exec.CommandContext(commandContext, "git", append([]string{"-C", repoRoot}, args...)...)
	var output limitedBuffer
	output.limit = gitOutputLimit
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		return "", err
	}
	if commandContext.Err() != nil {
		return "", commandContext.Err()
	}
	return output.String(), nil
}

type limitedBuffer struct {
	buffer bytes.Buffer
	limit  int64
}

func (b *limitedBuffer) Write(content []byte) (int, error) {
	if int64(b.buffer.Len()) >= b.limit {
		return len(content), nil
	}
	remaining := b.limit - int64(b.buffer.Len())
	_, err := io.CopyN(&b.buffer, bytes.NewReader(content), minInt64(remaining, int64(len(content))))
	return len(content), err
}

func (b *limitedBuffer) String() string { return b.buffer.String() }

func minInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}
