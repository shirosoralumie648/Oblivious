package releasecontract

import (
	"bytes"
	"encoding/json"
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

func validSchemaDocument() map[string]any {
	return map[string]any{
		"schemaVersion":  "contract/v1",
		"capabilities":   []any{map[string]any{"id": "identity.session", "commitment": "committed"}},
		"reasonCodes":    []any{map[string]any{"id": "profile_parity_unproven", "appliesTo": []any{"profile"}, "description": "Profile parity has not been proven."}},
		"defaultProfile": "monolith",
		"profiles": []any{map[string]any{
			"id": "monolith", "commitment": "committed",
			"topology":            map[string]any{"kind": "monolith", "components": []any{"server"}},
			"entrypoints":         []any{"server"},
			"dependencies":        []any{map[string]any{"id": "postgres", "required": true}},
			"stateStores":         []any{map[string]any{"id": "primary", "kind": "postgres"}},
			"capabilityOverrides": []any{},
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

func modelTestProfile() DeploymentProfile {
	operation := func(kind OperationKind) OperationRef {
		return OperationRef{ProfileID: "monolith", Path: "scripts/release-profile-operation.sh", Argv: []string{"monolith", string(kind)}}
	}
	return DeploymentProfile{
		ID: "monolith", Commitment: CommitmentCommitted,
		Topology:                Topology{Kind: TopologyMonolith, Components: []string{"server"}},
		Entrypoints:             []string{"server"},
		Dependencies:            []DependencyRef{{ID: "postgres", Required: true}},
		StateStores:             []StateStoreRef{{ID: "primary", Kind: "postgres"}},
		CapabilityOverrides:     []CapabilityOverride{},
		Operations:              ProfileOperations{Migrate: operation(OperationMigrate), Deploy: operation(OperationDeploy), Rollback: operation(OperationRollback)},
		CatalogBindingIDs:       []string{"model.gpt-4o-mini"},
		SurfaceReferenceIDs:     []string{"http"},
		ReadinessRequirementIDs: []string{"database"},
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
