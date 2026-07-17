package releasecontract

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func TestContractSchemaRejectsUnknownAndAuthoredIdentityFields(t *testing.T) {
	schemaBytes := readTestFile(t, "config/release/contract.schema.json")
	schema := compileTestSchema(t, schemaBytes)
	if err := schema.Validate(validSchemaDocument()); err != nil {
		t.Fatalf("valid authored contract baseline: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{
			name: "unknown root field",
			mutate: func(document map[string]any) {
				document["availability"] = "enabled"
			},
		},
		{
			name: "unknown nested field",
			mutate: func(document map[string]any) {
				profile := document["profiles"].([]any)[0].(map[string]any)
				profile["observedAt"] = "2026-07-16T00:00:00Z"
			},
		},
		{
			name: "source commit",
			mutate: func(document map[string]any) {
				document["releaseCommit"] = "0123456789012345678901234567890123456789"
			},
		},
		{
			name: "source tree",
			mutate: func(document map[string]any) {
				document["sourceTree"] = "0123456789012345678901234567890123456789"
			},
		},
		{
			name: "contract digest",
			mutate: func(document map[string]any) {
				document["contractDigest"] = "sha256:0123456789abcdef"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document := validSchemaDocument()
			tt.mutate(document)
			if err := schema.Validate(document); err == nil {
				t.Fatalf("schema accepted forbidden mutation %q", tt.name)
			}
		})
	}
}

func TestAuthoredContractV1ModelsRequiredSections(t *testing.T) {
	contract := AuthoredContractV1{
		SchemaVersion:         SchemaVersionV1,
		Capabilities:          []Capability{{ID: "identity.session", Commitment: CommitmentCommitted}},
		ReasonCodes:           []ReasonCode{{ID: "profile_parity_unproven", AppliesTo: []string{"profile"}, Description: "Profile parity has not been proven."}},
		DefaultProfile:        "monolith",
		Profiles:              []DeploymentProfile{modelTestProfile()},
		CatalogBindings:       []CatalogBinding{{ID: "model.gpt-4o-mini", SubjectKind: CatalogSubjectModel, SubjectID: "gpt-4o-mini", RuntimeClass: CatalogRuntimeServerModel, CapabilityID: "identity.session"}},
		SurfaceReferences:     []SurfaceReference{{ID: "http", CanonicalSource: "docs/api/openapi.yaml", Consumer: "runtime-route-registry", CapabilityIDs: []string{"identity.session"}}},
		ReadinessRequirements: []ReadinessRequirement{{ID: "database", CapabilityIDs: []string{"identity.session"}, DependencyIDs: []string{"postgres"}}},
	}

	encoded, err := json.Marshal(contract)
	if err != nil {
		t.Fatalf("marshal model: %v", err)
	}
	for _, key := range []string{"schemaVersion", "capabilities", "reasonCodes", "defaultProfile", "profiles", "catalogBindings", "surfaceReferences", "readinessRequirements"} {
		if !bytes.Contains(encoded, []byte(`"`+key+`"`)) {
			t.Errorf("required model section %q missing from JSON", key)
		}
	}
	assertNoAuthorityMaps(t, reflect.TypeOf(contract))
	for _, key := range []string{"refreshIntervalSeconds", "maxAgeSeconds", "allowedFutureSkewSeconds"} {
		if !bytes.Contains(encoded, []byte(`"`+key+`"`)) {
			t.Errorf("required profile field %q missing from JSON", key)
		}
	}
	if err := contract.Validate(testRepoRoot(t)); err != nil {
		t.Fatalf("validate typed model: %v", err)
	}
}

func TestCheckedInContractProfilePolicyAndReferenceClosure(t *testing.T) {
	contractBytes := readTestFile(t, "config/release/contract.v1.json")
	schema := compileTestSchema(t, readTestFile(t, "config/release/contract.schema.json"))
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(contractBytes))
	if err != nil {
		t.Fatalf("decode checked-in contract for schema: %v", err)
	}
	if err := schema.Validate(document); err != nil {
		t.Fatalf("checked-in contract schema validation: %v", err)
	}

	contract := decodeTestContract(t, contractBytes)
	if err := contract.Validate(testRepoRoot(t)); err != nil {
		t.Fatalf("checked-in contract semantic validation: %v", err)
	}
	if contract.DefaultProfile != "monolith" {
		t.Fatalf("default profile = %q, want monolith", contract.DefaultProfile)
	}
	for _, profile := range contract.Profiles {
		assertDeploymentProfileTiming(t, profile)
		if profile.ID == "monolith" {
			if profile.Commitment != CommitmentCommitted {
				t.Fatalf("monolith commitment = %q", profile.Commitment)
			}
			continue
		}
		if profile.Commitment != CommitmentExcluded || profile.ReasonCode != "profile_parity_unproven" {
			t.Fatalf("candidate profile %s was promoted: commitment=%s reason=%s", profile.ID, profile.Commitment, profile.ReasonCode)
		}
	}

	wantBindings := map[string]Commitment{
		"tool.calculator":     CommitmentCommitted,
		"tool.datetime":       CommitmentCommitted,
		"tool.http_request":   CommitmentConditional,
		"tool.json_formatter": CommitmentCommitted,
		"tool.text_transform": CommitmentCommitted,
		"tool.web_search":     CommitmentConditional,
		"runtime.custom":      CommitmentConditional,
		"runtime.mcp":         CommitmentCommitted,
		"runtime.sandbox":     CommitmentExcluded,
	}
	capabilityCommitments := map[string]Commitment{}
	for _, capability := range contract.Capabilities {
		capabilityCommitments[capability.ID] = capability.Commitment
	}
	for _, binding := range contract.CatalogBindings {
		want, ok := wantBindings[binding.ID]
		if !ok {
			continue
		}
		if got := capabilityCommitments[binding.CapabilityID]; got != want {
			t.Errorf("binding %s commitment = %s, want %s", binding.ID, got, want)
		}
		delete(wantBindings, binding.ID)
	}
	if len(wantBindings) != 0 {
		t.Fatalf("missing expected catalog bindings: %v", wantBindings)
	}
}

func TestReleaseProfileOperationScriptRejectsExcludedProfiles(t *testing.T) {
	script := filepath.Join(testRepoRoot(t), "scripts/release-profile-operation.sh")
	content := readTestFile(t, "scripts/release-profile-operation.sh")
	for _, forbidden := range []string{"eval ", "bash -c", "sh -c", "$*"} {
		if bytes.Contains(content, []byte(forbidden)) {
			t.Fatalf("operation script contains command-string pattern %q", forbidden)
		}
	}
	if !bytes.Contains(content, []byte("set -euo pipefail")) {
		t.Fatal("operation script must use set -euo pipefail")
	}

	for _, profileID := range []string{"dual", "microservices", "split"} {
		t.Run(profileID, func(t *testing.T) {
			command := exec.Command("bash", script, profileID, "deploy")
			output, err := command.CombinedOutput()
			if err == nil {
				t.Fatalf("excluded profile %s unexpectedly succeeded", profileID)
			}
			if !strings.Contains(string(output), "profile_excluded") {
				t.Fatalf("excluded profile output = %q", output)
			}
		})
	}

	command := exec.Command("bash", script, "monolith", "rollback")
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "operation_unproven") {
		t.Fatalf("rollback must fail closed without effects: err=%v output=%q", err, output)
	}
}

func TestLoadBytesRejectsNegativeFamilies(t *testing.T) {
	validContract := readTestFile(t, "config/release/contract.v1.json")
	validSchema := readTestFile(t, "config/release/contract.schema.json")
	repoRoot := testRepoRoot(t)

	tests := []struct {
		name     string
		root     func(*testing.T) string
		contract func(*testing.T) []byte
		wantCode ErrorCode
	}{
		{
			name: "invalid UTF-8",
			contract: func(*testing.T) []byte {
				return append(append([]byte{}, validContract...), 0xff)
			},
			wantCode: ErrorContractDecodeInvalid,
		},
		{
			name: "duplicate object key",
			contract: func(*testing.T) []byte {
				return bytes.Replace(validContract, []byte(`"schemaVersion": "contract/v1",`), []byte(`"schemaVersion": "contract/v1", "schemaVersion": "contract/v1",`), 1)
			},
			wantCode: ErrorContractDecodeInvalid,
		},
		{
			name: "trailing JSON value",
			contract: func(*testing.T) []byte {
				return append(append([]byte{}, validContract...), []byte("\n{}")...)
			},
			wantCode: ErrorContractDecodeInvalid,
		},
		{
			name: "unknown enum",
			contract: func(*testing.T) []byte {
				return bytes.Replace(validContract, []byte(`"commitment": "committed"`), []byte(`"commitment": "advertised"`), 1)
			},
			wantCode: ErrorContractSchemaInvalid,
		},
		{
			name: "unknown nested field",
			contract: func(t *testing.T) []byte {
				document := decodeJSONDocument(t, validContract)
				profiles := document["profiles"].([]any)
				profiles[0].(map[string]any)["availability"] = "enabled"
				return marshalJSONDocument(t, document)
			},
			wantCode: ErrorContractSchemaInvalid,
		},
		{
			name: "multiple committed profiles",
			contract: func(t *testing.T) []byte {
				return mutateCheckedInContract(t, func(contract *AuthoredContractV1) {
					contract.Profiles[0].Commitment = CommitmentCommitted
					contract.Profiles[0].ReasonCode = ""
				})
			},
			wantCode: ErrorContractSemanticInvalid,
		},
		{
			name: "broken capability reference",
			contract: func(t *testing.T) []byte {
				return mutateCheckedInContract(t, func(contract *AuthoredContractV1) {
					contract.SurfaceReferences[0].CapabilityIDs[0] = "unknown.capability"
				})
			},
			wantCode: ErrorContractSemanticInvalid,
		},
		{
			name: "duplicate capability ID",
			contract: func(t *testing.T) []byte {
				return mutateCheckedInContract(t, func(contract *AuthoredContractV1) {
					contract.Capabilities[1].ID = contract.Capabilities[0].ID
				})
			},
			wantCode: ErrorContractSemanticInvalid,
		},
		{
			name: "duplicate profile ID",
			contract: func(t *testing.T) []byte {
				return mutateCheckedInContract(t, func(contract *AuthoredContractV1) {
					contract.Profiles[1].ID = contract.Profiles[0].ID
				})
			},
			wantCode: ErrorContractSemanticInvalid,
		},
		{
			name: "unknown default profile",
			contract: func(t *testing.T) []byte {
				return mutateCheckedInContract(t, func(contract *AuthoredContractV1) {
					contract.DefaultProfile = "unknown"
				})
			},
			wantCode: ErrorContractSemanticInvalid,
		},
		{
			name: "excluded default profile",
			contract: func(t *testing.T) []byte {
				return mutateCheckedInContract(t, func(contract *AuthoredContractV1) {
					contract.DefaultProfile = "dual"
				})
			},
			wantCode: ErrorContractSemanticInvalid,
		},
		{
			name: "missing refresh interval",
			contract: func(t *testing.T) []byte {
				document := decodeJSONDocument(t, validContract)
				delete(document["profiles"].([]any)[0].(map[string]any), "refreshIntervalSeconds")
				return marshalJSONDocument(t, document)
			},
			wantCode: ErrorContractSchemaInvalid,
		},
		{
			name: "missing max age",
			contract: func(t *testing.T) []byte {
				document := decodeJSONDocument(t, validContract)
				delete(document["profiles"].([]any)[0].(map[string]any), "maxAgeSeconds")
				return marshalJSONDocument(t, document)
			},
			wantCode: ErrorContractSchemaInvalid,
		},
		{
			name: "missing allowed future skew",
			contract: func(t *testing.T) []byte {
				document := decodeJSONDocument(t, validContract)
				delete(document["profiles"].([]any)[0].(map[string]any), "allowedFutureSkewSeconds")
				return marshalJSONDocument(t, document)
			},
			wantCode: ErrorContractSchemaInvalid,
		},
		{
			name: "zero refresh interval",
			contract: func(t *testing.T) []byte {
				document := decodeJSONDocument(t, validContract)
				document["profiles"].([]any)[0].(map[string]any)["refreshIntervalSeconds"] = 0
				return marshalJSONDocument(t, document)
			},
			wantCode: ErrorContractSchemaInvalid,
		},
		{
			name: "zero max age",
			contract: func(t *testing.T) []byte {
				document := decodeJSONDocument(t, validContract)
				document["profiles"].([]any)[0].(map[string]any)["maxAgeSeconds"] = 0
				return marshalJSONDocument(t, document)
			},
			wantCode: ErrorContractSchemaInvalid,
		},
		{
			name: "negative allowed future skew",
			contract: func(t *testing.T) []byte {
				document := decodeJSONDocument(t, validContract)
				document["profiles"].([]any)[0].(map[string]any)["allowedFutureSkewSeconds"] = -1
				return marshalJSONDocument(t, document)
			},
			wantCode: ErrorContractSchemaInvalid,
		},
		{
			name:     "absolute operation path",
			contract: operationPathMutation(t, "/tmp/release-profile-operation.sh"),
			wantCode: ErrorContractPathInvalid,
		},
		{
			name:     "traversal operation path",
			contract: operationPathMutation(t, "scripts/../outside.sh"),
			wantCode: ErrorContractPathInvalid,
		},
		{
			name:     "NUL operation path",
			contract: operationPathMutation(t, "scripts/unsafe\x00.sh"),
			wantCode: ErrorContractSchemaInvalid,
		},
		{
			name:     "missing operation path",
			root:     func(t *testing.T) string { return prepareOperationRoot(t) },
			contract: operationPathMutation(t, "scripts/missing.sh"),
			wantCode: ErrorContractPathInvalid,
		},
		{
			name: "non-executable operation path",
			root: func(t *testing.T) string {
				root := prepareOperationRoot(t)
				if err := os.WriteFile(filepath.Join(root, "scripts/nonexec.sh"), []byte("#!/usr/bin/env bash\nexit 0\n"), 0o644); err != nil {
					t.Fatalf("write non-executable script: %v", err)
				}
				return root
			},
			contract: operationPathMutation(t, "scripts/nonexec.sh"),
			wantCode: ErrorContractPathInvalid,
		},
		{
			name: "symlink escaping operation path",
			root: func(t *testing.T) string {
				root := prepareOperationRoot(t)
				external := filepath.Join(t.TempDir(), "external.sh")
				if err := os.WriteFile(external, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
					t.Fatalf("write external script: %v", err)
				}
				if err := os.Symlink(external, filepath.Join(root, "scripts/escape.sh")); err != nil {
					t.Fatalf("create escaping symlink: %v", err)
				}
				return root
			},
			contract: operationPathMutation(t, "scripts/escape.sh"),
			wantCode: ErrorContractPathInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := repoRoot
			if tt.root != nil {
				root = tt.root(t)
			}
			contractBytes := tt.contract(t)
			contract, err := LoadBytes(context.Background(), root, contractBytes, validSchema)
			if err == nil {
				t.Fatalf("negative family returned usable contract: %+v", contract)
			}
			assertContractErrorCode(t, err, tt.wantCode)
		})
	}
}

func TestLoadBytesUsesExplicitRepoRootAndIgnoresCWD(t *testing.T) {
	repoRoot := prepareOperationRoot(t)
	contractBytes := readTestFile(t, "config/release/contract.v1.json")
	schemaBytes := readTestFile(t, "config/release/contract.schema.json")
	configDir := filepath.Join(repoRoot, "config/release")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create release config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "contract.v1.json"), contractBytes, 0o644); err != nil {
		t.Fatalf("write rooted contract: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "contract.schema.json"), schemaBytes, 0o644); err != nil {
		t.Fatalf("write rooted schema: %v", err)
	}
	otherCWD := t.TempDir()
	if err := os.MkdirAll(filepath.Join(otherCWD, "scripts"), 0o755); err != nil {
		t.Fatalf("create alternate cwd scripts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(otherCWD, "scripts/release-profile-operation.sh"), []byte("not executable\n"), 0o644); err != nil {
		t.Fatalf("write alternate cwd script: %v", err)
	}
	originalCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(otherCWD); err != nil {
		t.Fatalf("change cwd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalCWD); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	})

	contract, err := LoadBytes(context.Background(), repoRoot, contractBytes, schemaBytes)
	if err != nil {
		t.Fatalf("load with explicit root from unrelated cwd: %v", err)
	}
	if contract.DefaultProfile != "monolith" {
		t.Fatalf("default profile = %q", contract.DefaultProfile)
	}
	fileContract, err := Load(context.Background(), repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json")
	if err != nil {
		t.Fatalf("file load with explicit root from unrelated cwd: %v", err)
	}
	if fileContract.DefaultProfile != "monolith" {
		t.Fatalf("file-loaded default profile = %q", fileContract.DefaultProfile)
	}
}

func TestLoadBytesRejectsInvalidRepoRoots(t *testing.T) {
	contractBytes := readTestFile(t, "config/release/contract.v1.json")
	schemaBytes := readTestFile(t, "config/release/contract.schema.json")
	filePath := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(filePath, []byte("file"), 0o644); err != nil {
		t.Fatalf("write root fixture: %v", err)
	}
	tests := []struct {
		name string
		root string
	}{
		{name: "empty", root: ""},
		{name: "relative", root: "relative/root"},
		{name: "missing", root: filepath.Join(t.TempDir(), "missing")},
		{name: "not directory", root: filePath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadBytes(context.Background(), tt.root, contractBytes, schemaBytes)
			if err == nil {
				t.Fatalf("invalid root %q accepted", tt.root)
			}
			assertContractErrorCode(t, err, ErrorRepoRootInvalid)
		})
	}
}

func TestFileProfileResolverRequiresCommittedExplicitProfile(t *testing.T) {
	resolver := NewFileProfileResolver()
	repoRoot := testRepoRoot(t)
	contractPath := "config/release/contract.v1.json"
	schemaPath := "config/release/contract.schema.json"

	tests := []struct {
		name     string
		profile  string
		wantCode ErrorCode
		wantOK   bool
	}{
		{name: "omitted", wantCode: ErrorProfileRequired},
		{name: "unknown", profile: "unknown", wantCode: ErrorProfileUnknown},
		{name: "dual excluded", profile: "dual", wantCode: ErrorProfileExcluded},
		{name: "microservices excluded", profile: "microservices", wantCode: ErrorProfileExcluded},
		{name: "split excluded", profile: "split", wantCode: ErrorProfileExcluded},
		{name: "monolith committed", profile: "monolith", wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := resolver.ResolveCommittedProfile(context.Background(), repoRoot, contractPath, schemaPath, tt.profile)
			if tt.wantOK {
				if err != nil || profile.ID != "monolith" || profile.Commitment != CommitmentCommitted {
					t.Fatalf("resolve committed monolith: profile=%+v err=%v", profile, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("profile %q unexpectedly resolved: %+v", tt.profile, profile)
			}
			assertContractErrorCode(t, err, tt.wantCode)
		})
	}
}

func TestLoadRejectsContractAndSchemaOutsideExplicitRoot(t *testing.T) {
	repoRoot := t.TempDir()
	externalRoot := t.TempDir()
	contractPath := filepath.Join(externalRoot, "contract.v1.json")
	schemaPath := filepath.Join(externalRoot, "contract.schema.json")
	if err := os.WriteFile(contractPath, readTestFile(t, "config/release/contract.v1.json"), 0o644); err != nil {
		t.Fatalf("write external contract: %v", err)
	}
	if err := os.WriteFile(schemaPath, readTestFile(t, "config/release/contract.schema.json"), 0o644); err != nil {
		t.Fatalf("write external schema: %v", err)
	}
	_, err := Load(context.Background(), repoRoot, contractPath, schemaPath)
	if err == nil {
		t.Fatal("Load accepted contract files outside explicit repo root")
	}
	assertContractErrorCode(t, err, ErrorContractPathInvalid)
}

func validSchemaDocument() map[string]any {
	return map[string]any{
		"schemaVersion":  "contract/v1",
		"capabilities":   []any{map[string]any{"id": "identity.session", "commitment": "committed"}},
		"reasonCodes":    []any{map[string]any{"id": "profile_parity_unproven", "appliesTo": []any{"profile"}, "description": "Profile parity has not been proven."}},
		"defaultProfile": "monolith",
		"profiles": []any{map[string]any{
			"id": "monolith", "commitment": "committed",
			"refreshIntervalSeconds":   30,
			"maxAgeSeconds":            120,
			"allowedFutureSkewSeconds": 30,
			"topology":                 map[string]any{"kind": "monolith", "components": []any{"server"}},
			"entrypoints":              []any{"server"},
			"dependencies":             []any{map[string]any{"id": "postgres", "required": true}},
			"stateStores":              []any{map[string]any{"id": "primary", "kind": "postgres"}},
			"capabilityOverrides":      []any{},
			"operations": map[string]any{
				"migrate":  map[string]any{"profileId": "monolith", "path": "scripts/release-profile-operation.sh", "argv": []any{"monolith", "migrate"}},
				"deploy":   map[string]any{"profileId": "monolith", "path": "scripts/release-profile-operation.sh", "argv": []any{"monolith", "deploy"}},
				"rollback": map[string]any{"profileId": "monolith", "path": "scripts/release-profile-operation.sh", "argv": []any{"monolith", "rollback"}},
			},
			"catalogBindingIds":       []any{"model.gpt-4o-mini"},
			"surfaceReferenceIds":     []any{"http"},
			"readinessRequirementIds": []any{"database"},
		}},
		"catalogBindings":       []any{map[string]any{"id": "model.gpt-4o-mini", "subjectKind": "model", "subjectId": "gpt-4o-mini", "runtimeClass": "server_model", "capabilityId": "identity.session"}},
		"surfaceReferences":     []any{map[string]any{"id": "http", "canonicalSource": "docs/api/openapi.yaml", "consumer": "runtime-route-registry", "capabilityIds": []any{"identity.session"}}},
		"readinessRequirements": []any{map[string]any{"id": "database", "capabilityIds": []any{"identity.session"}, "dependencyIds": []any{"postgres"}}},
	}
}

func assertDeploymentProfileTiming(t *testing.T, profile DeploymentProfile) {
	t.Helper()
	if profile.RefreshIntervalSeconds != 30 || profile.MaxAgeSeconds != 120 || profile.AllowedFutureSkewSeconds != 30 {
		t.Errorf(
			"profile %s timing = %d/%d/%d, want 30/120/30",
			profile.ID,
			profile.RefreshIntervalSeconds,
			profile.MaxAgeSeconds,
			profile.AllowedFutureSkewSeconds,
		)
	}
}

func decodeTestContract(t *testing.T, content []byte) AuthoredContractV1 {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var contract AuthoredContractV1
	if err := decoder.Decode(&contract); err != nil {
		t.Fatalf("decode contract: %v", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		t.Fatalf("expected contract EOF, got %v", err)
	}
	return contract
}

func mutateCheckedInContract(t *testing.T, mutate func(*AuthoredContractV1)) []byte {
	t.Helper()
	contract := decodeTestContract(t, readTestFile(t, "config/release/contract.v1.json"))
	mutate(&contract)
	content, err := json.Marshal(contract)
	if err != nil {
		t.Fatalf("marshal mutated contract: %v", err)
	}
	return content
}

func operationPathMutation(t *testing.T, path string) func(*testing.T) []byte {
	t.Helper()
	return func(t *testing.T) []byte {
		return mutateCheckedInContract(t, func(contract *AuthoredContractV1) {
			for i := range contract.Profiles {
				if contract.Profiles[i].ID == "monolith" {
					contract.Profiles[i].Operations.Migrate.Path = path
					return
				}
			}
		})
	}
}

func prepareOperationRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	scriptsDir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("create scripts dir: %v", err)
	}
	content := readTestFile(t, "scripts/release-profile-operation.sh")
	if err := os.WriteFile(filepath.Join(scriptsDir, "release-profile-operation.sh"), content, 0o755); err != nil {
		t.Fatalf("write operation script: %v", err)
	}
	return root
}

func decodeJSONDocument(t *testing.T, content []byte) map[string]any {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatalf("decode JSON document: %v", err)
	}
	return document
}

func marshalJSONDocument(t *testing.T, document map[string]any) []byte {
	t.Helper()
	content, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal JSON document: %v", err)
	}
	return content
}

func assertContractErrorCode(t *testing.T, err error, want ErrorCode) {
	t.Helper()
	var contractErr *ContractError
	if !errors.As(err, &contractErr) {
		t.Fatalf("error %T %v is not ContractError", err, err)
	}
	if contractErr.Code != want {
		t.Fatalf("error code = %q, want %q (error=%v)", contractErr.Code, want, err)
	}
}

func modelTestProfile() DeploymentProfile {
	operation := func(kind OperationKind) OperationRef {
		return OperationRef{ProfileID: "monolith", Path: "scripts/release-profile-operation.sh", Argv: []string{"monolith", string(kind)}}
	}
	return DeploymentProfile{
		ID: "monolith", Commitment: CommitmentCommitted,
		RefreshIntervalSeconds:   30,
		MaxAgeSeconds:            120,
		AllowedFutureSkewSeconds: 30,
		Topology:                 Topology{Kind: TopologyMonolith, Components: []string{"server"}},
		Entrypoints:              []string{"server"},
		Dependencies:             []DependencyRef{{ID: "postgres", Required: true}},
		StateStores:              []StateStoreRef{{ID: "primary", Kind: "postgres"}},
		CapabilityOverrides:      []CapabilityOverride{},
		Operations:               ProfileOperations{Migrate: operation(OperationMigrate), Deploy: operation(OperationDeploy), Rollback: operation(OperationRollback)},
		CatalogBindingIDs:        []string{"model.gpt-4o-mini"},
		SurfaceReferenceIDs:      []string{"http"},
		ReadinessRequirementIDs:  []string{"database"},
	}
}

func compileTestSchema(t *testing.T, schemaBytes []byte) *jsonschema.Schema {
	t.Helper()
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaBytes))
	if err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("contract.schema.json", document); err != nil {
		t.Fatalf("add schema resource: %v", err)
	}
	schema, err := compiler.Compile("contract.schema.json")
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	return schema
}

func readTestFile(t *testing.T, relative string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(testRepoRoot(t), filepath.FromSlash(relative)))
	if err != nil {
		t.Fatalf("read %s: %v", relative, err)
	}
	return content
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../.."))
}

func assertNoAuthorityMaps(t *testing.T, typ reflect.Type) {
	t.Helper()
	seen := map[reflect.Type]bool{}
	var visit func(reflect.Type)
	visit = func(current reflect.Type) {
		for current.Kind() == reflect.Pointer || current.Kind() == reflect.Slice || current.Kind() == reflect.Array {
			current = current.Elem()
		}
		if seen[current] {
			return
		}
		seen[current] = true
		switch current.Kind() {
		case reflect.Map:
			t.Errorf("authored authority contains open-ended map type %s", current)
		case reflect.Struct:
			for i := 0; i < current.NumField(); i++ {
				visit(current.Field(i).Type)
			}
		}
	}
	visit(typ)
}
