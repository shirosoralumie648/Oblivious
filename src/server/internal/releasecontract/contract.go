package releasecontract

import (
	"fmt"
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
		parts = append(parts, "value="+e.Value)
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
	if c.SchemaVersion != SchemaVersionV1 {
		return contractError(ErrorContractSemanticInvalid, "schemaVersion", c.SchemaVersion, nil)
	}
	for i, capability := range c.Capabilities {
		if !validCommitment(capability.Commitment) {
			return contractError(ErrorContractSemanticInvalid, fmt.Sprintf("capabilities[%d].commitment", i), string(capability.Commitment), nil)
		}
	}
	for i, profile := range c.Profiles {
		if !validCommitment(profile.Commitment) {
			return contractError(ErrorContractSemanticInvalid, fmt.Sprintf("profiles[%d].commitment", i), string(profile.Commitment), nil)
		}
		if !validTopology(profile.Topology.Kind) {
			return contractError(ErrorContractSemanticInvalid, fmt.Sprintf("profiles[%d].topology.kind", i), string(profile.Topology.Kind), nil)
		}
	}
	return nil
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
