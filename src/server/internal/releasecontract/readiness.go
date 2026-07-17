package releasecontract

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	ReadinessSnapshotSchemaV1 = "readiness-snapshot/v1"
	buildIdentitySchemaV1     = "build-identity/v1"
	repositoryLocalEvidence   = "repository-local"
	monolithRefreshSeconds    = int64(30)
	monolithMaxAgeSeconds     = int64(120)
	monolithFutureSkewSeconds = int64(30)
)

// BuildIdentityV1 is defined with the release contract to keep readiness and
// build identity on one dependency boundary. buildinfo re-exports this type.
type BuildIdentityV1 struct {
	SchemaVersion  string `json:"schemaVersion"`
	ReleaseCommit  string `json:"releaseCommit"`
	SourceTree     string `json:"sourceTree"`
	ContractDigest string `json:"contractDigest"`
	Dirty          bool   `json:"dirty"`
	EvidenceClass  string `json:"evidenceClass"`
}

type Availability string

const (
	AvailabilityEnabled  Availability = "enabled"
	AvailabilityDisabled Availability = "disabled"
	AvailabilityBlocked  Availability = "blocked"
)

type Boundary string

const (
	BoundaryHTTP         Boundary = "http_dispatch"
	BoundaryGRPC         Boundary = "grpc_dispatch"
	BoundaryWorkerClaim  Boundary = "worker_claim"
	BoundaryWorkerEffect Boundary = "worker_effect"
	BoundaryOutbound     Boundary = "outbound_dispatch"
	BoundaryFinancial    Boundary = "financial_dispatch"
	BoundaryOperation    Boundary = "operation_dispatch"
)

type ReadinessCode string

const (
	CodeBuildIdentityMissing   ReadinessCode = "build_identity_missing"
	CodeBuildIdentityMismatch  ReadinessCode = "build_identity_mismatch"
	CodeProfileRequired        ReadinessCode = "profile_required"
	CodeProfileExcluded        ReadinessCode = "profile_excluded"
	CodeReadinessUnavailable   ReadinessCode = "readiness_unavailable"
	CodeReadinessStale         ReadinessCode = "readiness_stale"
	CodeCapabilityUnknown      ReadinessCode = "capability_unknown"
	CodeCapabilityDisabled     ReadinessCode = "capability_disabled"
	CodeCapabilityBlocked      ReadinessCode = "capability_blocked"
	CodeReportOutputUnwritable ReadinessCode = "report_output_unwritable"
)

type ReadinessError struct {
	Code  ReadinessCode
	Field string
	Err   error
}

func (e *ReadinessError) Error() string {
	if e == nil {
		return string(CodeReadinessUnavailable)
	}
	if e.Field == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: field=%s", e.Code, e.Field)
}

func (e *ReadinessError) Unwrap() error { return e.Err }

func IsReadinessCode(err error, code ReadinessCode) bool {
	var readinessErr *ReadinessError
	return errors.As(err, &readinessErr) && readinessErr.Code == code
}

func readinessError(code ReadinessCode, field string, err error) error {
	return &ReadinessError{Code: code, Field: field, Err: err}
}

type Observation struct {
	ProbeID       string            `json:"probeId"`
	DependencyID  string            `json:"dependencyId"`
	CapabilityIDs []string          `json:"capabilityIds"`
	Availability  Availability      `json:"availability"`
	ReasonCode    string            `json:"reasonCode,omitempty"`
	ObservedAt    time.Time         `json:"observedAt"`
	Detail        map[string]string `json:"detail,omitempty"`
}

type CapabilityEvaluation struct {
	CapabilityID string       `json:"capabilityId"`
	Commitment   Commitment   `json:"commitment"`
	Availability Availability `json:"availability"`
	ReasonCode   string       `json:"reasonCode,omitempty"`
	Dependencies []string     `json:"dependencies"`
}

type Evaluation struct {
	Identity     BuildIdentityV1                 `json:"identity"`
	Profile      string                          `json:"profile"`
	Generation   uint64                          `json:"generation"`
	CheckedAt    time.Time                       `json:"checkedAt"`
	ValidUntil   time.Time                       `json:"validUntil"`
	Capabilities map[string]CapabilityEvaluation `json:"capabilities"`
	ErrorCode    ReadinessCode                   `json:"errorCode,omitempty"`
	observations []Observation
}

type ReadinessSnapshotV1 struct {
	SchemaVersion string                          `json:"schemaVersion"`
	Identity      BuildIdentityV1                 `json:"identity"`
	Profile       string                          `json:"profile"`
	Generation    uint64                          `json:"generation"`
	CheckedAt     time.Time                       `json:"checkedAt"`
	ValidUntil    time.Time                       `json:"validUntil"`
	Observations  []Observation                   `json:"observations"`
	Capabilities  map[string]CapabilityEvaluation `json:"capabilities"`
}

type Evaluator interface {
	Evaluate(AuthoredContractV1, BuildIdentityV1, DeploymentProfile, uint64, []Observation, time.Time) (Evaluation, error)
}

type contractEvaluator struct{}

func NewEvaluator() Evaluator { return contractEvaluator{} }

func (contractEvaluator) Evaluate(contract AuthoredContractV1, identity BuildIdentityV1, profile DeploymentProfile, generation uint64, observations []Observation, now time.Time) (Evaluation, error) {
	if err := validateReadinessIdentity(contract, identity); err != nil {
		return Evaluation{}, err
	}
	if err := validateReadinessProfile(contract, profile); err != nil {
		return Evaluation{}, err
	}
	if generation == 0 {
		return Evaluation{}, readinessError(CodeReadinessUnavailable, "generation", nil)
	}
	if now.IsZero() {
		return Evaluation{}, readinessError(CodeReadinessUnavailable, "checkedAt", nil)
	}
	now = normalizeTime(now)

	reasons := make(map[string]struct{}, len(contract.ReasonCodes))
	for _, reason := range contract.ReasonCodes {
		reasons[reason.ID] = struct{}{}
	}
	capabilities, dependencies, err := applicableCapabilityPolicy(contract, profile)
	if err != nil {
		return Evaluation{}, err
	}
	indexed, validUntil, err := validateObservations(profile, observations, dependencies, reasons, now)
	if err != nil {
		return Evaluation{}, err
	}

	result := make(map[string]CapabilityEvaluation, len(capabilities))
	for capabilityID, policy := range capabilities {
		evaluation := CapabilityEvaluation{
			CapabilityID: capabilityID,
			Commitment:   policy.commitment,
			Availability: AvailabilityEnabled,
			Dependencies: append([]string(nil), policy.dependencies...),
		}
		if policy.commitment == CommitmentExcluded {
			evaluation.Availability = AvailabilityDisabled
			evaluation.ReasonCode = policy.reasonCode
		} else {
			for _, dependencyID := range policy.dependencies {
				observation := indexed[dependencyID]
				switch observation.Availability {
				case AvailabilityBlocked:
					evaluation.Availability = AvailabilityBlocked
					evaluation.ReasonCode = observation.ReasonCode
				case AvailabilityDisabled:
					if evaluation.Availability != AvailabilityBlocked {
						evaluation.Availability = AvailabilityDisabled
						evaluation.ReasonCode = observation.ReasonCode
					}
				}
			}
		}
		result[capabilityID] = evaluation
	}

	return Evaluation{
		Identity:     identity,
		Profile:      profile.ID,
		Generation:   generation,
		CheckedAt:    now,
		ValidUntil:   validUntil,
		Capabilities: cloneCapabilityEvaluations(result),
		observations: cloneObservations(observations),
	}, nil
}

func (e Evaluation) Snapshot() ReadinessSnapshotV1 {
	return ReadinessSnapshotV1{
		SchemaVersion: ReadinessSnapshotSchemaV1,
		Identity:      e.Identity,
		Profile:       e.Profile,
		Generation:    e.Generation,
		CheckedAt:     normalizeTime(e.CheckedAt),
		ValidUntil:    normalizeTime(e.ValidUntil),
		Observations:  cloneObservations(e.observations),
		Capabilities:  cloneCapabilityEvaluations(e.Capabilities),
	}
}

type capabilityPolicy struct {
	commitment   Commitment
	reasonCode   string
	dependencies []string
}

func applicableCapabilityPolicy(contract AuthoredContractV1, profile DeploymentProfile) (map[string]capabilityPolicy, map[string][]string, error) {
	capabilities := make(map[string]capabilityPolicy, len(contract.Capabilities))
	for _, capability := range contract.Capabilities {
		capabilities[capability.ID] = capabilityPolicy{commitment: capability.Commitment, reasonCode: capability.ReasonCode}
	}
	for _, override := range profile.CapabilityOverrides {
		policy, ok := capabilities[override.CapabilityID]
		if !ok {
			return nil, nil, readinessError(CodeCapabilityUnknown, "profile.capabilityOverrides", nil)
		}
		policy.commitment = override.Commitment
		policy.reasonCode = override.ReasonCode
		capabilities[override.CapabilityID] = policy
	}

	requirements := make(map[string]ReadinessRequirement, len(contract.ReadinessRequirements))
	for _, requirement := range contract.ReadinessRequirements {
		requirements[requirement.ID] = requirement
	}
	dependencyCapabilities := map[string][]string{}
	for _, requirementID := range profile.ReadinessRequirementIDs {
		requirement, ok := requirements[requirementID]
		if !ok {
			return nil, nil, readinessError(CodeReadinessUnavailable, "profile.readinessRequirementIds", nil)
		}
		for _, capabilityID := range requirement.CapabilityIDs {
			policy, ok := capabilities[capabilityID]
			if !ok {
				return nil, nil, readinessError(CodeCapabilityUnknown, "readinessRequirements.capabilityIds", nil)
			}
			if policy.commitment == CommitmentExcluded {
				continue
			}
			policy.dependencies = unionSorted(policy.dependencies, requirement.DependencyIDs)
			capabilities[capabilityID] = policy
			for _, dependencyID := range requirement.DependencyIDs {
				dependencyCapabilities[dependencyID] = unionSorted(dependencyCapabilities[dependencyID], []string{capabilityID})
			}
		}
	}
	return capabilities, dependencyCapabilities, nil
}

func validateObservations(profile DeploymentProfile, observations []Observation, expected map[string][]string, reasons map[string]struct{}, now time.Time) (map[string]Observation, time.Time, error) {
	indexed := make(map[string]Observation, len(observations))
	var oldest time.Time
	for i, observation := range observations {
		observation = cloneObservation(observation)
		observation.ObservedAt = normalizeTime(observation.ObservedAt)
		capabilityIDs, ok := expected[observation.DependencyID]
		if !ok || strings.TrimSpace(observation.ProbeID) == "" || observation.ObservedAt.IsZero() {
			return nil, time.Time{}, readinessError(CodeReadinessUnavailable, fmt.Sprintf("observations[%d]", i), nil)
		}
		if _, duplicate := indexed[observation.DependencyID]; duplicate {
			return nil, time.Time{}, readinessError(CodeReadinessUnavailable, fmt.Sprintf("observations[%d].dependencyId", i), nil)
		}
		if !equalStrings(observation.CapabilityIDs, capabilityIDs) {
			return nil, time.Time{}, readinessError(CodeReadinessUnavailable, fmt.Sprintf("observations[%d].capabilityIds", i), nil)
		}
		if err := validateAvailabilityAndReason(observation.Availability, observation.ReasonCode, reasons); err != nil {
			return nil, time.Time{}, readinessError(CodeReadinessUnavailable, fmt.Sprintf("observations[%d]", i), err)
		}
		if observation.ObservedAt.UnixNano() > now.Add(time.Duration(profile.AllowedFutureSkewSeconds)*time.Second).UnixNano() {
			return nil, time.Time{}, readinessError(CodeReadinessUnavailable, fmt.Sprintf("observations[%d].observedAt", i), nil)
		}
		indexed[observation.DependencyID] = observation
		if oldest.IsZero() || observation.ObservedAt.UnixNano() < oldest.UnixNano() {
			oldest = observation.ObservedAt
		}
	}
	if len(indexed) != len(expected) {
		return nil, time.Time{}, readinessError(CodeReadinessUnavailable, "observations", nil)
	}
	if len(expected) == 0 {
		oldest = now
	}
	validUntil := oldest.Add(time.Duration(profile.MaxAgeSeconds) * time.Second)
	if now.UnixNano() > validUntil.UnixNano() {
		return nil, time.Time{}, readinessError(CodeReadinessStale, "observations", nil)
	}
	return indexed, validUntil, nil
}

func validateAvailabilityAndReason(availability Availability, reason string, reasons map[string]struct{}) error {
	switch availability {
	case AvailabilityEnabled:
		if reason != "" {
			return fmt.Errorf("enabled observation has reason")
		}
	case AvailabilityDisabled, AvailabilityBlocked:
		if reason == "" {
			return fmt.Errorf("denied observation has no reason")
		}
		if _, ok := reasons[reason]; !ok {
			return fmt.Errorf("unknown reason")
		}
	default:
		return fmt.Errorf("unknown availability")
	}
	return nil
}

func validateReadinessProfile(contract AuthoredContractV1, profile DeploymentProfile) error {
	if strings.TrimSpace(profile.ID) == "" {
		return readinessError(CodeProfileRequired, "profile", nil)
	}
	if profile.Commitment != CommitmentCommitted || profile.ID != "monolith" || profile.Topology.Kind != TopologyMonolith {
		return readinessError(CodeProfileExcluded, "profile", nil)
	}
	var authored *DeploymentProfile
	for i := range contract.Profiles {
		if contract.Profiles[i].ID == profile.ID {
			authored = &contract.Profiles[i]
			break
		}
	}
	if authored == nil || !reflect.DeepEqual(*authored, profile) {
		return readinessError(CodeBuildIdentityMismatch, "profile", nil)
	}
	if profile.RefreshIntervalSeconds != monolithRefreshSeconds || profile.MaxAgeSeconds != monolithMaxAgeSeconds || profile.AllowedFutureSkewSeconds != monolithFutureSkewSeconds {
		return readinessError(CodeBuildIdentityMismatch, "profile.readinessTiming", nil)
	}
	return nil
}

var readinessGitObject = regexp.MustCompile(`^[0-9a-f]{40}$`)
var readinessDigest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func validateReadinessIdentity(contract AuthoredContractV1, identity BuildIdentityV1) error {
	if identity.SchemaVersion == "" || identity.ReleaseCommit == "" || identity.SourceTree == "" || identity.ContractDigest == "" || identity.EvidenceClass == "" {
		return readinessError(CodeBuildIdentityMissing, "identity", nil)
	}
	if identity.SchemaVersion != buildIdentitySchemaV1 || !readinessGitObject.MatchString(identity.ReleaseCommit) || !readinessGitObject.MatchString(identity.SourceTree) || !readinessDigest.MatchString(identity.ContractDigest) || identity.Dirty || identity.EvidenceClass != repositoryLocalEvidence {
		return readinessError(CodeBuildIdentityMismatch, "identity", nil)
	}
	digest, err := Digest(contract)
	if err != nil || digest != identity.ContractDigest {
		return readinessError(CodeBuildIdentityMismatch, "identity.contractDigest", err)
	}
	return nil
}

type EntrypointID string

func RequireProfileEntrypoint(profile DeploymentProfile, entrypointID EntrypointID) error {
	if strings.TrimSpace(string(entrypointID)) == "" {
		return readinessError(CodeProfileRequired, "entrypointId", nil)
	}
	if profile.ID == "" || profile.Commitment != CommitmentCommitted || profile.ID != "monolith" || profile.Topology.Kind != TopologyMonolith {
		return readinessError(CodeProfileExcluded, "profile", nil)
	}
	for _, candidate := range profile.Entrypoints {
		if candidate == string(entrypointID) {
			return nil
		}
	}
	return readinessError(CodeProfileExcluded, "entrypointId", nil)
}

type ReadinessManager interface {
	Bootstrap(context.Context) error
	StartRefresh(context.Context)
	Require(string) error
	Evaluate() Evaluation
	ExportAudit(string) error
}

type Guard interface {
	Require(context.Context, string, Boundary) error
}

type ManagerGuard struct {
	Manager ReadinessManager
	Observe func(context.Context, string, Boundary, error)
}

func (g ManagerGuard) Require(ctx context.Context, capabilityID string, boundary Boundary) error {
	if g.Manager == nil {
		return readinessError(CodeReadinessUnavailable, "manager", nil)
	}
	err := g.Manager.Require(capabilityID)
	if g.Observe != nil {
		g.Observe(ctx, capabilityID, boundary, err)
	}
	return err
}

type EffectDescriptor struct {
	ID           string
	CapabilityID string
	Boundary     Boundary
	Owner        string
}

type EffectRegistrar interface {
	Register(EffectDescriptor) error
}

func cloneEvaluation(source Evaluation) Evaluation {
	result := source
	result.Identity = source.Identity
	result.CheckedAt = normalizeTime(source.CheckedAt)
	result.ValidUntil = normalizeTime(source.ValidUntil)
	result.Capabilities = cloneCapabilityEvaluations(source.Capabilities)
	result.observations = cloneObservations(source.observations)
	return result
}

func cloneObservations(source []Observation) []Observation {
	result := make([]Observation, len(source))
	for i := range source {
		result[i] = cloneObservation(source[i])
	}
	return result
}

func cloneCapabilityEvaluations(source map[string]CapabilityEvaluation) map[string]CapabilityEvaluation {
	result := make(map[string]CapabilityEvaluation, len(source))
	for id, capability := range source {
		capability.Dependencies = append([]string(nil), capability.Dependencies...)
		result[id] = capability
	}
	return result
}

func cloneObservation(source Observation) Observation {
	result := source
	result.CapabilityIDs = append([]string(nil), source.CapabilityIDs...)
	result.Detail = make(map[string]string, len(source.Detail))
	for key, value := range source.Detail {
		result.Detail[key] = value
	}
	return result
}

func normalizeTime(value time.Time) time.Time {
	return time.Unix(0, value.UTC().UnixNano()).UTC()
}

func unionSorted(existing, added []string) []string {
	set := make(map[string]struct{}, len(existing)+len(added))
	for _, value := range existing {
		set[value] = struct{}{}
	}
	for _, value := range added {
		set[value] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func equalStrings(left, right []string) bool {
	leftCopy := append([]string(nil), left...)
	rightCopy := append([]string(nil), right...)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	return reflect.DeepEqual(leftCopy, rightCopy)
}
