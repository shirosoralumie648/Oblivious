package releasecontract

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const SchemaVersionV1 = "contract/v1"

type Commitment string

const (
	CommitmentCommitted   Commitment = "committed"
	CommitmentConditional Commitment = "conditional"
	CommitmentExcluded    Commitment = "excluded"
)

type TopologyKind string

const (
	TopologyMonolith      TopologyKind = "monolith"
	TopologyMicroservices TopologyKind = "microservices"
	TopologyDual          TopologyKind = "dual"
	TopologySplit         TopologyKind = "split"
)

type CatalogSubjectKind string

const (
	CatalogSubjectModel   CatalogSubjectKind = "model"
	CatalogSubjectTool    CatalogSubjectKind = "tool"
	CatalogSubjectRuntime CatalogSubjectKind = "runtime"
)

type CatalogRuntimeClass string

const (
	CatalogRuntimeServerModel CatalogRuntimeClass = "server_model"
	CatalogRuntimeBuiltin     CatalogRuntimeClass = "builtin"
	CatalogRuntimeNetwork     CatalogRuntimeClass = "network"
	CatalogRuntimeCustom      CatalogRuntimeClass = "custom"
	CatalogRuntimeMCP         CatalogRuntimeClass = "mcp"
	CatalogRuntimeSandbox     CatalogRuntimeClass = "sandbox"
)

type OperationKind string

const (
	OperationMigrate  OperationKind = "migrate"
	OperationDeploy   OperationKind = "deploy"
	OperationRollback OperationKind = "rollback"
)

type AuthoredContractV1 struct {
	SchemaVersion         string                 `json:"schemaVersion"`
	Capabilities          []Capability           `json:"capabilities"`
	ReasonCodes           []ReasonCode           `json:"reasonCodes"`
	DefaultProfile        string                 `json:"defaultProfile"`
	Profiles              []DeploymentProfile    `json:"profiles"`
	CatalogBindings       []CatalogBinding       `json:"catalogBindings"`
	SurfaceReferences     []SurfaceReference     `json:"surfaceReferences"`
	ReadinessRequirements []ReadinessRequirement `json:"readinessRequirements"`
}

type Capability struct {
	ID         string     `json:"id"`
	Commitment Commitment `json:"commitment"`
	ReasonCode string     `json:"reasonCode,omitempty"`
}

type ReasonCode struct {
	ID          string   `json:"id"`
	AppliesTo   []string `json:"appliesTo"`
	Description string   `json:"description"`
}

type DeploymentProfile struct {
	ID                      string               `json:"id"`
	Commitment              Commitment           `json:"commitment"`
	ReasonCode              string               `json:"reasonCode,omitempty"`
	Topology                Topology             `json:"topology"`
	Entrypoints             []string             `json:"entrypoints"`
	Dependencies            []DependencyRef      `json:"dependencies"`
	StateStores             []StateStoreRef      `json:"stateStores"`
	CapabilityOverrides     []CapabilityOverride `json:"capabilityOverrides"`
	Operations              ProfileOperations    `json:"operations"`
	CatalogBindingIDs       []string             `json:"catalogBindingIds"`
	SurfaceReferenceIDs     []string             `json:"surfaceReferenceIds"`
	ReadinessRequirementIDs []string             `json:"readinessRequirementIds"`
}

type Topology struct {
	Kind       TopologyKind `json:"kind"`
	Components []string     `json:"components"`
}

type DependencyRef struct {
	ID       string `json:"id"`
	Required bool   `json:"required"`
}

type StateStoreRef struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

type CapabilityOverride struct {
	CapabilityID string     `json:"capabilityId"`
	Commitment   Commitment `json:"commitment"`
	ReasonCode   string     `json:"reasonCode,omitempty"`
}

type ProfileOperations struct {
	Migrate  OperationRef `json:"migrate"`
	Deploy   OperationRef `json:"deploy"`
	Rollback OperationRef `json:"rollback"`
}

type OperationRef struct {
	ProfileID string   `json:"profileId"`
	Path      string   `json:"path"`
	Argv      []string `json:"argv"`
}

type CatalogBinding struct {
	ID           string              `json:"id"`
	SubjectKind  CatalogSubjectKind  `json:"subjectKind"`
	SubjectID    string              `json:"subjectId"`
	RuntimeClass CatalogRuntimeClass `json:"runtimeClass"`
	CapabilityID string              `json:"capabilityId"`
}

type SurfaceReference struct {
	ID              string   `json:"id"`
	CanonicalSource string   `json:"canonicalSource"`
	Consumer        string   `json:"consumer"`
	CapabilityIDs   []string `json:"capabilityIds"`
}

type ReadinessRequirement struct {
	ID            string   `json:"id"`
	CapabilityIDs []string `json:"capabilityIds"`
	DependencyIDs []string `json:"dependencyIds"`
}

type ErrorCode string

const (
	ErrorContractSchemaInvalid   ErrorCode = "contract_schema_invalid"
	ErrorContractDecodeInvalid   ErrorCode = "contract_decode_invalid"
	ErrorContractSemanticInvalid ErrorCode = "contract_semantic_invalid"
	ErrorRepoRootInvalid         ErrorCode = "repo_root_invalid"
	ErrorContractPathInvalid     ErrorCode = "contract_path_invalid"
	ErrorProfileRequired         ErrorCode = "profile_required"
	ErrorProfileUnknown          ErrorCode = "profile_unknown"
	ErrorProfileNotCommitted     ErrorCode = "profile_not_committed"
	ErrorProfileExcluded         ErrorCode = "profile_excluded"
)

type ContractError struct {
	Code  ErrorCode
	Field string
	Value string
	Err   error
}

func (e *ContractError) Error() string {
	parts := []string{string(e.Code)}
	if e.Field != "" {
		parts = append(parts, "field="+e.Field)
	}
	if e.Value != "" {
		parts = append(parts, "value="+sanitizeErrorValue(e.Value))
	}
	if e.Err != nil {
		parts = append(parts, e.Err.Error())
	}
	return strings.Join(parts, ": ")
}

func (e *ContractError) Unwrap() error { return e.Err }

func contractError(code ErrorCode, field, value string, err error) error {
	return &ContractError{Code: code, Field: field, Value: value, Err: err}
}

func (c AuthoredContractV1) Validate(repoRoot string) error {
	if err := validateRepoRoot(repoRoot); err != nil {
		return err
	}
	if c.SchemaVersion != SchemaVersionV1 {
		return contractError(ErrorContractSemanticInvalid, "schemaVersion", c.SchemaVersion, nil)
	}
	reasonCodes, err := indexUnique("reasonCodes", c.ReasonCodes, func(item ReasonCode) string { return item.ID })
	if err != nil {
		return err
	}
	capabilities, err := indexUnique("capabilities", c.Capabilities, func(item Capability) string { return item.ID })
	if err != nil {
		return err
	}
	profiles, err := indexUnique("profiles", c.Profiles, func(item DeploymentProfile) string { return item.ID })
	if err != nil {
		return err
	}
	catalogBindings, err := indexUnique("catalogBindings", c.CatalogBindings, func(item CatalogBinding) string { return item.ID })
	if err != nil {
		return err
	}
	surfaceReferences, err := indexUnique("surfaceReferences", c.SurfaceReferences, func(item SurfaceReference) string { return item.ID })
	if err != nil {
		return err
	}
	readinessRequirements, err := indexUnique("readinessRequirements", c.ReadinessRequirements, func(item ReadinessRequirement) string { return item.ID })
	if err != nil {
		return err
	}

	for i, reason := range c.ReasonCodes {
		if err := requireSortedUnique(fmt.Sprintf("reasonCodes[%d].appliesTo", i), reason.AppliesTo); err != nil {
			return err
		}
	}
	for i, capability := range c.Capabilities {
		if !validCommitment(capability.Commitment) {
			return contractError(ErrorContractSemanticInvalid, fmt.Sprintf("capabilities[%d].commitment", i), string(capability.Commitment), nil)
		}
		if err := validateReasonReference(reasonCodes, capability.ReasonCode, "capability", capability.Commitment != CommitmentCommitted, fmt.Sprintf("capabilities[%d].reasonCode", i)); err != nil {
			return err
		}
	}

	if c.DefaultProfile != "monolith" {
		return contractError(ErrorContractSemanticInvalid, "defaultProfile", c.DefaultProfile, nil)
	}
	defaultProfile, ok := profiles[c.DefaultProfile]
	if !ok || defaultProfile.Commitment != CommitmentCommitted {
		return contractError(ErrorContractSemanticInvalid, "defaultProfile", c.DefaultProfile, nil)
	}
	committedProfiles := 0
	for i, profile := range c.Profiles {
		if !validCommitment(profile.Commitment) {
			return contractError(ErrorContractSemanticInvalid, fmt.Sprintf("profiles[%d].commitment", i), string(profile.Commitment), nil)
		}
		if !validTopology(profile.Topology.Kind) {
			return contractError(ErrorContractSemanticInvalid, fmt.Sprintf("profiles[%d].topology.kind", i), string(profile.Topology.Kind), nil)
		}
		if profile.ID == "monolith" {
			if profile.Commitment != CommitmentCommitted || profile.Topology.Kind != TopologyMonolith {
				return contractError(ErrorContractSemanticInvalid, fmt.Sprintf("profiles[%d]", i), profile.ID, nil)
			}
		} else if profile.Commitment != CommitmentExcluded || profile.ReasonCode != "profile_parity_unproven" {
			return contractError(ErrorContractSemanticInvalid, fmt.Sprintf("profiles[%d].commitment", i), profile.ID, nil)
		}
		if profile.Commitment == CommitmentCommitted {
			committedProfiles++
		}
		if err := validateReasonReference(reasonCodes, profile.ReasonCode, "profile", profile.Commitment != CommitmentCommitted, fmt.Sprintf("profiles[%d].reasonCode", i)); err != nil {
			return err
		}
		if err := requireSortedUnique(fmt.Sprintf("profiles[%d].topology.components", i), profile.Topology.Components); err != nil {
			return err
		}
		if err := requireSortedUnique(fmt.Sprintf("profiles[%d].entrypoints", i), profile.Entrypoints); err != nil {
			return err
		}
		dependencyIDs, err := indexUnique(fmt.Sprintf("profiles[%d].dependencies", i), profile.Dependencies, func(item DependencyRef) string { return item.ID })
		if err != nil {
			return err
		}
		if _, err := indexUnique(fmt.Sprintf("profiles[%d].stateStores", i), profile.StateStores, func(item StateStoreRef) string { return item.ID }); err != nil {
			return err
		}
		for j, override := range profile.CapabilityOverrides {
			if _, ok := capabilities[override.CapabilityID]; !ok {
				return contractError(ErrorContractSemanticInvalid, fmt.Sprintf("profiles[%d].capabilityOverrides[%d].capabilityId", i, j), override.CapabilityID, nil)
			}
			if !validCommitment(override.Commitment) {
				return contractError(ErrorContractSemanticInvalid, fmt.Sprintf("profiles[%d].capabilityOverrides[%d].commitment", i, j), string(override.Commitment), nil)
			}
			if err := validateReasonReference(reasonCodes, override.ReasonCode, "capability", override.Commitment != CommitmentCommitted, fmt.Sprintf("profiles[%d].capabilityOverrides[%d].reasonCode", i, j)); err != nil {
				return err
			}
		}
		operations := []struct {
			kind OperationKind
			ref  OperationRef
		}{{OperationMigrate, profile.Operations.Migrate}, {OperationDeploy, profile.Operations.Deploy}, {OperationRollback, profile.Operations.Rollback}}
		for _, operation := range operations {
			if err := validateOperationRef(repoRoot, profile.ID, operation.kind, operation.ref); err != nil {
				return err
			}
		}
		if err := requireReferences(fmt.Sprintf("profiles[%d].catalogBindingIds", i), profile.CatalogBindingIDs, catalogBindings); err != nil {
			return err
		}
		if err := requireReferences(fmt.Sprintf("profiles[%d].surfaceReferenceIds", i), profile.SurfaceReferenceIDs, surfaceReferences); err != nil {
			return err
		}
		if err := requireReferences(fmt.Sprintf("profiles[%d].readinessRequirementIds", i), profile.ReadinessRequirementIDs, readinessRequirements); err != nil {
			return err
		}
		for _, requirementID := range profile.ReadinessRequirementIDs {
			for _, dependencyID := range readinessRequirements[requirementID].DependencyIDs {
				if _, ok := dependencyIDs[dependencyID]; !ok {
					return contractError(ErrorContractSemanticInvalid, "readinessRequirements.dependencyIds", dependencyID, nil)
				}
			}
		}
	}
	if committedProfiles != 1 {
		return contractError(ErrorContractSemanticInvalid, "profiles", fmt.Sprintf("committed=%d", committedProfiles), nil)
	}

	for i, binding := range c.CatalogBindings {
		if !validCatalogSubjectKind(binding.SubjectKind) || !validCatalogRuntimeClass(binding.RuntimeClass) {
			return contractError(ErrorContractSemanticInvalid, fmt.Sprintf("catalogBindings[%d]", i), binding.ID, nil)
		}
		if _, ok := capabilities[binding.CapabilityID]; !ok {
			return contractError(ErrorContractSemanticInvalid, fmt.Sprintf("catalogBindings[%d].capabilityId", i), binding.CapabilityID, nil)
		}
	}
	for i, surface := range c.SurfaceReferences {
		if err := requireReferences(fmt.Sprintf("surfaceReferences[%d].capabilityIds", i), surface.CapabilityIDs, capabilities); err != nil {
			return err
		}
	}
	for i, requirement := range c.ReadinessRequirements {
		if err := requireReferences(fmt.Sprintf("readinessRequirements[%d].capabilityIds", i), requirement.CapabilityIDs, capabilities); err != nil {
			return err
		}
		if err := requireSortedUnique(fmt.Sprintf("readinessRequirements[%d].dependencyIds", i), requirement.DependencyIDs); err != nil {
			return err
		}
	}
	return nil
}

func validateRepoRoot(repoRoot string) error {
	if repoRoot == "" || !filepath.IsAbs(repoRoot) {
		return contractError(ErrorRepoRootInvalid, "repoRoot", repoRoot, nil)
	}
	info, err := os.Stat(repoRoot)
	if err != nil || !info.IsDir() {
		return contractError(ErrorRepoRootInvalid, "repoRoot", repoRoot, nil)
	}
	return nil
}

func validateReasonReference(reasons map[string]ReasonCode, reasonID, appliesTo string, required bool, field string) error {
	if reasonID == "" {
		if required {
			return contractError(ErrorContractSemanticInvalid, field, "missing", nil)
		}
		return nil
	}
	if !required {
		return contractError(ErrorContractSemanticInvalid, field, reasonID, nil)
	}
	reason, ok := reasons[reasonID]
	if !ok || !contains(reason.AppliesTo, appliesTo) {
		return contractError(ErrorContractSemanticInvalid, field, reasonID, nil)
	}
	return nil
}

func validateOperationRef(repoRoot, profileID string, kind OperationKind, ref OperationRef) error {
	field := fmt.Sprintf("profiles.%s.operations.%s", profileID, kind)
	if ref.ProfileID != profileID {
		return contractError(ErrorContractSemanticInvalid, field+".profileId", ref.ProfileID, nil)
	}
	if len(ref.Argv) != 2 || ref.Argv[0] != profileID || ref.Argv[1] != string(kind) {
		return contractError(ErrorContractSemanticInvalid, field+".argv", strings.Join(ref.Argv, ","), nil)
	}
	if ref.Path == "" || filepath.IsAbs(ref.Path) || strings.ContainsRune(ref.Path, '\x00') || filepath.ToSlash(filepath.Clean(ref.Path)) != ref.Path || !strings.HasPrefix(ref.Path, "scripts/") {
		return contractError(ErrorContractPathInvalid, field+".path", ref.Path, nil)
	}
	resolved := filepath.Join(repoRoot, filepath.FromSlash(ref.Path))
	realPath, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		return contractError(ErrorContractPathInvalid, field+".path", ref.Path, nil)
	}
	relative, err := filepath.Rel(repoRoot, realPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return contractError(ErrorContractPathInvalid, field+".path", ref.Path, nil)
	}
	info, err := os.Stat(realPath)
	if err != nil || info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return contractError(ErrorContractPathInvalid, field+".path", ref.Path, nil)
	}
	return nil
}

func indexUnique[T any](field string, values []T, id func(T) string) (map[string]T, error) {
	result := make(map[string]T, len(values))
	previous := ""
	for i, value := range values {
		current := id(value)
		if _, exists := result[current]; exists {
			return nil, contractError(ErrorContractSemanticInvalid, field, current, nil)
		}
		if i > 0 && current <= previous {
			return nil, contractError(ErrorContractSemanticInvalid, field, current, nil)
		}
		result[current] = value
		previous = current
	}
	return result, nil
}

func requireReferences[T any](field string, ids []string, authority map[string]T) error {
	if err := requireSortedUnique(field, ids); err != nil {
		return err
	}
	for _, id := range ids {
		if _, ok := authority[id]; !ok {
			return contractError(ErrorContractSemanticInvalid, field, id, nil)
		}
	}
	return nil
}

func requireSortedUnique(field string, values []string) error {
	if len(values) == 0 {
		return contractError(ErrorContractSemanticInvalid, field, "empty", nil)
	}
	if !sort.StringsAreSorted(values) {
		return contractError(ErrorContractSemanticInvalid, field, "unsorted", nil)
	}
	for i := 1; i < len(values); i++ {
		if values[i] == values[i-1] {
			return contractError(ErrorContractSemanticInvalid, field, values[i], nil)
		}
	}
	return nil
}

func validCatalogSubjectKind(value CatalogSubjectKind) bool {
	switch value {
	case CatalogSubjectModel, CatalogSubjectTool, CatalogSubjectRuntime:
		return true
	default:
		return false
	}
}

func validCatalogRuntimeClass(value CatalogRuntimeClass) bool {
	switch value {
	case CatalogRuntimeServerModel, CatalogRuntimeBuiltin, CatalogRuntimeNetwork, CatalogRuntimeCustom, CatalogRuntimeMCP, CatalogRuntimeSandbox:
		return true
	default:
		return false
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func sanitizeErrorValue(value string) string {
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return '?'
		}
		return r
	}, value)
	if len(value) > 128 {
		return value[:128] + "..."
	}
	return value
}

func validCommitment(value Commitment) bool {
	switch value {
	case CommitmentCommitted, CommitmentConditional, CommitmentExcluded:
		return true
	default:
		return false
	}
}

func validTopology(value TopologyKind) bool {
	switch value {
	case TopologyMonolith, TopologyMicroservices, TopologyDual, TopologySplit:
		return true
	default:
		return false
	}
}
