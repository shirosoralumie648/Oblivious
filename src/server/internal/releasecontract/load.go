package releasecontract

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	maxContractBytes = 4 << 20
	maxSchemaBytes   = 1 << 20
)

type ProfileResolver interface {
	ResolveCommittedProfile(ctx context.Context, repoRoot, contractPath, schemaPath, profileID string) (DeploymentProfile, error)
}

type FileProfileResolver struct{}

func NewFileProfileResolver() *FileProfileResolver {
	return &FileProfileResolver{}
}

func Load(ctx context.Context, repoRoot, contractPath, schemaPath string) (AuthoredContractV1, error) {
	canonicalRoot, err := canonicalRepoRoot(repoRoot)
	if err != nil {
		return AuthoredContractV1{}, err
	}
	if err := checkContext(ctx); err != nil {
		return AuthoredContractV1{}, err
	}
	resolvedContract, err := resolveRepoFile(canonicalRoot, contractPath)
	if err != nil {
		return AuthoredContractV1{}, err
	}
	resolvedSchema, err := resolveRepoFile(canonicalRoot, schemaPath)
	if err != nil {
		return AuthoredContractV1{}, err
	}
	contractBytes, err := readBoundedFile(resolvedContract, maxContractBytes)
	if err != nil {
		return AuthoredContractV1{}, contractError(ErrorContractDecodeInvalid, "contract", filepath.Base(resolvedContract), nil)
	}
	schemaBytes, err := readBoundedFile(resolvedSchema, maxSchemaBytes)
	if err != nil {
		return AuthoredContractV1{}, contractError(ErrorContractSchemaInvalid, "schema", filepath.Base(resolvedSchema), nil)
	}
	return loadBytes(ctx, canonicalRoot, contractBytes, schemaBytes)
}

func LoadBytes(ctx context.Context, repoRoot string, contractBytes, schemaBytes []byte) (AuthoredContractV1, error) {
	canonicalRoot, err := canonicalRepoRoot(repoRoot)
	if err != nil {
		return AuthoredContractV1{}, err
	}
	return loadBytes(ctx, canonicalRoot, contractBytes, schemaBytes)
}

func loadBytes(ctx context.Context, repoRoot string, contractBytes, schemaBytes []byte) (AuthoredContractV1, error) {
	if err := checkContext(ctx); err != nil {
		return AuthoredContractV1{}, err
	}
	if len(contractBytes) == 0 || len(contractBytes) > maxContractBytes || !utf8.Valid(contractBytes) {
		return AuthoredContractV1{}, contractError(ErrorContractDecodeInvalid, "contract", "invalid_bytes", nil)
	}
	if len(schemaBytes) == 0 || len(schemaBytes) > maxSchemaBytes || !utf8.Valid(schemaBytes) {
		return AuthoredContractV1{}, contractError(ErrorContractSchemaInvalid, "schema", "invalid_bytes", nil)
	}
	if err := rejectDuplicateObjectKeys(schemaBytes); err != nil {
		return AuthoredContractV1{}, contractError(ErrorContractSchemaInvalid, "schema", "duplicate_key", nil)
	}
	if err := rejectDuplicateObjectKeys(contractBytes); err != nil {
		return AuthoredContractV1{}, err
	}

	schemaDocument, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaBytes))
	if err != nil {
		return AuthoredContractV1{}, contractError(ErrorContractSchemaInvalid, "schema", "decode", nil)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("contract.schema.json", schemaDocument); err != nil {
		return AuthoredContractV1{}, contractError(ErrorContractSchemaInvalid, "schema", "resource", nil)
	}
	compiledSchema, err := compiler.Compile("contract.schema.json")
	if err != nil {
		return AuthoredContractV1{}, contractError(ErrorContractSchemaInvalid, "schema", "compile", nil)
	}
	contractDocument, err := jsonschema.UnmarshalJSON(bytes.NewReader(contractBytes))
	if err != nil {
		return AuthoredContractV1{}, contractError(ErrorContractDecodeInvalid, "contract", "decode", nil)
	}
	if err := compiledSchema.Validate(contractDocument); err != nil {
		return AuthoredContractV1{}, contractError(ErrorContractSchemaInvalid, validationInstancePath(err), "rejected", nil)
	}

	decoder := json.NewDecoder(bytes.NewReader(contractBytes))
	decoder.DisallowUnknownFields()
	var contract AuthoredContractV1
	if err := decoder.Decode(&contract); err != nil {
		return AuthoredContractV1{}, contractError(ErrorContractDecodeInvalid, "contract", "typed_decode", nil)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return AuthoredContractV1{}, contractError(ErrorContractDecodeInvalid, "contract", "trailing_json", nil)
	}
	if err := checkContext(ctx); err != nil {
		return AuthoredContractV1{}, err
	}
	if err := contract.Validate(repoRoot); err != nil {
		return AuthoredContractV1{}, err
	}
	return contract, nil
}

func (r *FileProfileResolver) ResolveCommittedProfile(ctx context.Context, repoRoot, contractPath, schemaPath, profileID string) (DeploymentProfile, error) {
	if strings.TrimSpace(profileID) == "" {
		return DeploymentProfile{}, contractError(ErrorProfileRequired, "profileId", "missing", nil)
	}
	contract, err := Load(ctx, repoRoot, contractPath, schemaPath)
	if err != nil {
		return DeploymentProfile{}, err
	}
	for _, profile := range contract.Profiles {
		if profile.ID != profileID {
			continue
		}
		switch profile.Commitment {
		case CommitmentCommitted:
			return profile, nil
		case CommitmentExcluded:
			return DeploymentProfile{}, contractError(ErrorProfileExcluded, "profileId", profileID, nil)
		default:
			return DeploymentProfile{}, contractError(ErrorProfileNotCommitted, "profileId", profileID, nil)
		}
	}
	return DeploymentProfile{}, contractError(ErrorProfileUnknown, "profileId", profileID, nil)
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return contractError(ErrorContractDecodeInvalid, "context", "nil", nil)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("load release contract: %w", err)
	}
	return nil
}

func canonicalRepoRoot(repoRoot string) (string, error) {
	if err := validateRepoRoot(repoRoot); err != nil {
		return "", err
	}
	realRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return "", contractError(ErrorRepoRootInvalid, "repoRoot", repoRoot, nil)
	}
	return filepath.Clean(realRoot), nil
}

func resolveRepoFile(repoRoot, requested string) (string, error) {
	if requested == "" || strings.ContainsRune(requested, '\x00') {
		return "", contractError(ErrorContractPathInvalid, "path", requested, nil)
	}
	candidate := requested
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(repoRoot, filepath.FromSlash(requested))
	}
	realPath, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", contractError(ErrorContractPathInvalid, "path", requested, nil)
	}
	relative, err := filepath.Rel(repoRoot, realPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", contractError(ErrorContractPathInvalid, "path", requested, nil)
	}
	info, err := os.Stat(realPath)
	if err != nil || info.IsDir() {
		return "", contractError(ErrorContractPathInvalid, "path", requested, nil)
	}
	return realPath, nil
}

func readBoundedFile(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > limit {
		return nil, fmt.Errorf("file exceeds limit")
	}
	return content, nil
}

func rejectDuplicateObjectKeys(content []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	if err := scanJSONValue(decoder, nil); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return contractError(ErrorContractDecodeInvalid, "contract", "trailing_json", nil)
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder, path []string) error {
	token, err := decoder.Token()
	if err != nil {
		return contractError(ErrorContractDecodeInvalid, jsonPointer(path), "invalid_json", nil)
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return contractError(ErrorContractDecodeInvalid, jsonPointer(path), "invalid_object", nil)
			}
			key, ok := keyToken.(string)
			if !ok {
				return contractError(ErrorContractDecodeInvalid, jsonPointer(path), "invalid_key", nil)
			}
			if _, exists := seen[key]; exists {
				return contractError(ErrorContractDecodeInvalid, jsonPointer(append(path, key)), "duplicate_key", nil)
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder, append(path, key)); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return contractError(ErrorContractDecodeInvalid, jsonPointer(path), "invalid_object", nil)
		}
	case '[':
		index := 0
		for decoder.More() {
			if err := scanJSONValue(decoder, append(path, fmt.Sprintf("%d", index))); err != nil {
				return err
			}
			index++
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return contractError(ErrorContractDecodeInvalid, jsonPointer(path), "invalid_array", nil)
		}
	default:
		return contractError(ErrorContractDecodeInvalid, jsonPointer(path), "invalid_json", nil)
	}
	return nil
}

func validationInstancePath(err error) string {
	var validationErr *jsonschema.ValidationError
	if !errors.As(err, &validationErr) {
		return "contract"
	}
	best := validationErr.InstanceLocation
	var visit func(*jsonschema.ValidationError)
	visit = func(current *jsonschema.ValidationError) {
		if len(current.InstanceLocation) > len(best) {
			best = current.InstanceLocation
		}
		for _, cause := range current.Causes {
			visit(cause)
		}
	}
	visit(validationErr)
	return jsonPointer(best)
}

func jsonPointer(parts []string) string {
	if len(parts) == 0 {
		return "/"
	}
	escaped := make([]string, len(parts))
	for i, part := range parts {
		escaped[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~", "~0"), "/", "~1")
	}
	return "/" + strings.Join(escaped, "/")
}
