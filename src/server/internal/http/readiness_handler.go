package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	stdhttp "net/http"
	"sort"
	"strings"
	"time"

	"oblivious/server/internal/releasecontract"
)

// AppReleaseIdentity is the non-sensitive identity subset exposed to an
// authenticated application client.
type AppReleaseIdentity struct {
	SourceTree        string `json:"sourceTree"`
	ContractDigest    string `json:"contractDigest"`
	DeploymentProfile string `json:"deploymentProfile"`
}

type AppCapabilityAvailability struct {
	CapabilityID string                       `json:"capabilityId"`
	Disposition  releasecontract.Commitment   `json:"disposition"`
	Availability releasecontract.Availability `json:"availability"`
	Enabled      bool                         `json:"enabled"`
}

type AppCapabilityProjectionResponse struct {
	ReleaseIdentity  AppReleaseIdentity          `json:"releaseIdentity"`
	Generation       uint64                      `json:"generation"`
	ProjectionDigest string                      `json:"projectionDigest"`
	Capabilities     []AppCapabilityAvailability `json:"capabilities"`
}

type AdminCapabilityAvailability struct {
	CapabilityID string                       `json:"capabilityId"`
	Disposition  releasecontract.Commitment   `json:"disposition"`
	Availability releasecontract.Availability `json:"availability"`
	Enabled      bool                         `json:"enabled"`
	ReasonCode   string                       `json:"reasonCode,omitempty"`
	Dependencies []string                     `json:"dependencies"`
	Remediation  string                       `json:"remediation,omitempty"`
	EvidenceRefs []string                     `json:"evidenceRefs"`
}

// AdminReadinessInventory intentionally contains operational detail that is
// never included in the app projection.
type AdminReadinessInventory struct {
	ReleaseIdentity releasecontract.BuildIdentityV1 `json:"releaseIdentity"`
	Profile         string                          `json:"profile"`
	Generation      uint64                          `json:"generation"`
	CheckedAt       string                          `json:"checkedAt"`
	ValidUntil      string                          `json:"validUntil"`
	Capabilities    []AdminCapabilityAvailability   `json:"capabilities"`
}

// ReadinessHandlers serves control-plane views from the in-memory manager.
// It deliberately has no database, audit-file, or probe dependency.
type ReadinessHandlers struct {
	manager     releasecontract.ReadinessManager
	authorities releasecontract.RuntimeAuthorities
}

type ReadinessHandlerOptions struct {
	Readiness   releasecontract.ReadinessManager
	Authorities releasecontract.RuntimeAuthorities
}

// NewReadinessHandlers accepts either ReadinessHandlerOptions or the two
// positional values (ReadinessManager, RuntimeAuthorities). The permissive
// shape keeps the constructor usable by small composition tests while the
// production router uses the typed options form.
func NewReadinessHandlers(args ...any) *ReadinessHandlers {
	var options ReadinessHandlerOptions
	if len(args) == 1 {
		if value, ok := args[0].(ReadinessHandlerOptions); ok {
			options = value
		}
	} else if len(args) == 2 {
		options.Readiness, _ = args[0].(releasecontract.ReadinessManager)
		options.Authorities, _ = args[1].(releasecontract.RuntimeAuthorities)
	}
	return &ReadinessHandlers{manager: options.Readiness, authorities: options.Authorities}
}

func (h *ReadinessHandlers) Livez(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodGet {
		writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, stdhttp.StatusOK, map[string]string{"status": "ok"})
}

func (h *ReadinessHandlers) Readyz(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodGet {
		writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if h == nil || h.manager == nil {
		writeReadinessHTTPError(w, &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessUnavailable, Field: "manager"})
		return
	}
	evaluation := h.manager.Evaluate()
	if evaluation.ErrorCode != "" {
		writeReadinessHTTPError(w, &releasecontract.ReadinessError{Code: evaluation.ErrorCode, Field: "evaluation"})
		return
	}
	for _, capability := range evaluation.Capabilities {
		if capability.Commitment == releasecontract.CommitmentCommitted && capability.Availability != releasecontract.AvailabilityEnabled {
			writeReadinessHTTPError(w, readinessCodeForAvailability(capability.Availability))
			return
		}
	}
	writeJSON(w, stdhttp.StatusOK, map[string]any{
		"status": "ready", "generation": evaluation.Generation,
		"checkedAt": evaluation.CheckedAt.UTC().Format(time.RFC3339Nano),
	})
}

func (h *ReadinessHandlers) Admin(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodGet {
		writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if h == nil || h.manager == nil {
		writeReadinessHTTPError(w, &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessUnavailable, Field: "manager"})
		return
	}
	evaluation := h.manager.Evaluate()
	if evaluation.ErrorCode != "" {
		writeReadinessHTTPError(w, &releasecontract.ReadinessError{Code: evaluation.ErrorCode, Field: "evaluation"})
		return
	}
	capabilities := make([]AdminCapabilityAvailability, 0, len(evaluation.Capabilities))
	for _, id := range sortedCapabilityIDs(evaluation.Capabilities) {
		item := evaluation.Capabilities[id]
		capabilities = append(capabilities, AdminCapabilityAvailability{
			CapabilityID: id, Disposition: item.Commitment, Availability: item.Availability,
			Enabled:    item.Availability == releasecontract.AvailabilityEnabled,
			ReasonCode: item.ReasonCode, Dependencies: append([]string(nil), item.Dependencies...),
			Remediation: remediationFor(item.ReasonCode), EvidenceRefs: []string{"readiness-snapshot/v1"},
		})
	}
	writeJSON(w, stdhttp.StatusOK, AdminReadinessInventory{
		ReleaseIdentity: evaluation.Identity, Profile: evaluation.Profile, Generation: evaluation.Generation,
		CheckedAt: evaluation.CheckedAt.UTC().Format(time.RFC3339Nano), ValidUntil: evaluation.ValidUntil.UTC().Format(time.RFC3339Nano),
		Capabilities: capabilities,
	})
}

func (h *ReadinessHandlers) App(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodGet {
		writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if h == nil || h.manager == nil || !h.authorities.Valid() {
		writeReadinessHTTPError(w, &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessUnavailable, Field: "authorities"})
		return
	}
	evaluation := h.manager.Evaluate()
	if evaluation.ErrorCode != "" {
		writeReadinessHTTPError(w, &releasecontract.ReadinessError{Code: evaluation.ErrorCode, Field: "evaluation"})
		return
	}
	items := make([]AppCapabilityAvailability, 0, len(evaluation.Capabilities))
	for _, id := range sortedCapabilityIDs(evaluation.Capabilities) {
		item := evaluation.Capabilities[id]
		if item.Commitment == releasecontract.CommitmentExcluded {
			continue
		}
		items = append(items, AppCapabilityAvailability{CapabilityID: id, Disposition: item.Commitment, Availability: item.Availability, Enabled: item.Availability == releasecontract.AvailabilityEnabled})
	}
	identity := AppReleaseIdentity{SourceTree: evaluation.Identity.SourceTree, ContractDigest: evaluation.Identity.ContractDigest, DeploymentProfile: evaluation.Profile}
	digest, err := projectionDigest(identity, evaluation.Generation, items)
	if err != nil {
		writeReadinessHTTPError(w, &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessUnavailable, Field: "projection"})
		return
	}
	writeJSON(w, stdhttp.StatusOK, AppCapabilityProjectionResponse{ReleaseIdentity: identity, Generation: evaluation.Generation, ProjectionDigest: digest, Capabilities: items})
}

func projectionDigest(identity AppReleaseIdentity, generation uint64, items []AppCapabilityAvailability) (string, error) {
	value := struct {
		Identity     AppReleaseIdentity          `json:"identity"`
		Generation   uint64                      `json:"generation"`
		Capabilities []AppCapabilityAvailability `json:"capabilities"`
	}{identity, generation, items}
	content, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func sortedCapabilityIDs(values map[string]releasecontract.CapabilityEvaluation) []string {
	ids := make([]string, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func remediationFor(reason string) string {
	if strings.TrimSpace(reason) == "" {
		return ""
	}
	return fmt.Sprintf("resolve readiness dependency for %s", reason)
}

func readinessCodeForAvailability(availability releasecontract.Availability) *releasecontract.ReadinessError {
	switch availability {
	case releasecontract.AvailabilityDisabled:
		return &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityDisabled, Field: "capability"}
	case releasecontract.AvailabilityBlocked:
		return &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityBlocked, Field: "capability"}
	default:
		return &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessUnavailable, Field: "capability"}
	}
}

func writeReadinessHTTPError(w stdhttp.ResponseWriter, err error) {
	var readinessErr *releasecontract.ReadinessError
	if !errorsAsReadiness(err, &readinessErr) {
		readinessErr = &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessUnavailable, Field: "readiness"}
	}
	status := stdhttp.StatusServiceUnavailable
	if readinessErr.Code == releasecontract.CodeCapabilityDisabled {
		status = stdhttp.StatusForbidden
	}
	writeError(w, status, string(readinessErr.Code), string(readinessErr.Code))
}

func errorsAsReadiness(err error, target **releasecontract.ReadinessError) bool {
	if err == nil {
		return false
	}
	if value, ok := err.(*releasecontract.ReadinessError); ok {
		*target = value
		return true
	}
	return false
}
