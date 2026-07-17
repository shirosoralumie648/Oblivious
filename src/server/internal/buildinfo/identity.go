package buildinfo

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"oblivious/server/internal/releasecontract"
)

const (
	BuildIdentitySchemaV1   = "build-identity/v1"
	EvidenceRepositoryLocal = "repository-local"
)

type ErrorCode string

const (
	ErrorBuildIdentityMissing   ErrorCode = "build_identity_missing"
	ErrorBuildIdentityMismatch  ErrorCode = "build_identity_mismatch"
	ErrorSourceWorktreeDirty    ErrorCode = "source_worktree_dirty"
	ErrorContractDigestMismatch ErrorCode = "contract_digest_mismatch"
)

type IdentityError struct {
	Code  ErrorCode `json:"code"`
	Field string    `json:"field,omitempty"`
	Err   error     `json:"-"`
}

func (e *IdentityError) Error() string {
	if e.Field == "" {
		return string(e.Code)
	}
	return string(e.Code) + ": field=" + e.Field
}

func (e *IdentityError) Unwrap() error { return e.Err }

func IsCode(err error, code ErrorCode) bool {
	var identityErr *IdentityError
	return errors.As(err, &identityErr) && identityErr.Code == code
}

type BuildIdentityV1 = releasecontract.BuildIdentityV1

var (
	gitObjectPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
	digestPattern    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

func ValidateIdentity(identity BuildIdentityV1) error {
	if strings.TrimSpace(identity.SchemaVersion) == "" {
		return identityError(ErrorBuildIdentityMissing, "schemaVersion", nil)
	}
	if identity.SchemaVersion != BuildIdentitySchemaV1 {
		return identityError(ErrorBuildIdentityMismatch, "schemaVersion", nil)
	}
	if identity.ReleaseCommit == "" {
		return identityError(ErrorBuildIdentityMissing, "releaseCommit", nil)
	}
	if !gitObjectPattern.MatchString(identity.ReleaseCommit) {
		return identityError(ErrorBuildIdentityMismatch, "releaseCommit", nil)
	}
	if identity.SourceTree == "" {
		return identityError(ErrorBuildIdentityMissing, "sourceTree", nil)
	}
	if !gitObjectPattern.MatchString(identity.SourceTree) {
		return identityError(ErrorBuildIdentityMismatch, "sourceTree", nil)
	}
	if identity.ContractDigest == "" {
		return identityError(ErrorBuildIdentityMissing, "contractDigest", nil)
	}
	if !digestPattern.MatchString(identity.ContractDigest) {
		return identityError(ErrorContractDigestMismatch, "contractDigest", nil)
	}
	if identity.Dirty {
		return identityError(ErrorSourceWorktreeDirty, "dirty", nil)
	}
	if identity.EvidenceClass == "" {
		return identityError(ErrorBuildIdentityMissing, "evidenceClass", nil)
	}
	if identity.EvidenceClass != EvidenceRepositoryLocal {
		return identityError(ErrorBuildIdentityMismatch, "evidenceClass", nil)
	}
	return nil
}

func MarshalIdentity(identity BuildIdentityV1) ([]byte, error) {
	if err := ValidateIdentity(identity); err != nil {
		return nil, err
	}
	return json.Marshal(identity)
}

func identityError(code ErrorCode, field string, err error) error {
	return &IdentityError{Code: code, Field: field, Err: err}
}
