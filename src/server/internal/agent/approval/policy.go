package approval

import (
	"strings"
	"sync"
)

// RiskLevel classifies a tool's danger level.
type RiskLevel string

const (
	RiskSafe      RiskLevel = "safe"
	RiskMedium    RiskLevel = "medium"
	RiskDangerous RiskLevel = "dangerous"
)

// Mode determines how the policy decides approval.
type Mode string

const (
	ModeTiered  Mode = "tiered"  // safe=auto, medium=first-approval, dangerous=always-approve
	ModeAll     Mode = "all"     // every tool call requires approval
	ModeNone    Mode = "none"    // no tool call requires approval
	ModeCustom  Mode = "custom"  // per-tool overrides from ToolOverrides
)

// Decision is the output of an approval evaluation.
type Decision struct {
	RequiresApproval bool      `json:"requiresApproval"`
	RiskLevel        RiskLevel `json:"riskLevel"`
	Reason           string    `json:"reason,omitempty"`
}

// ToolOverride lets callers override approval for specific tools.
type ToolOverride struct {
	// RiskLevel overrides the inferred risk level for this tool.
	RiskLevel *RiskLevel `json:"riskLevel,omitempty"`
	// RequiresApproval overrides the approval requirement.
	RequiresApproval *bool `json:"requiresApproval,omitempty"`
}

// ApprovalRecord tracks whether a medium-risk tool has been approved in a
// given conversation. This is used by tiered mode to auto-execute medium
// tools after the first approval.
type ApprovalRecord struct {
	OrganizationID string
	ConversationID string
	ToolName       string
	Approved       bool
}

// ApprovalStore abstracts persistence for approval records. The caller (the
// agent service layer) provides an implementation backed by durable storage.
type ApprovalStore interface {
	// HasPriorApproval returns true if the given tool has been approved at
	// least once in this conversation.
	HasPriorApproval(ctx interface{}, organizationID, conversationID, toolName string) bool
}

// Policy evaluates whether a tool call requires human approval.
type Policy struct {
	mu             sync.RWMutex
	mode           Mode
	toolOverrides  map[string]ToolOverride
	store          ApprovalStore
}

// NewPolicy creates a Policy with the given mode and optional store.
func NewPolicy(mode Mode, store ApprovalStore) *Policy {
	return &Policy{
		mode:          normalizeMode(mode),
		toolOverrides: make(map[string]ToolOverride),
		store:         store,
	}
}

// SetMode changes the approval mode.
func (p *Policy) SetMode(mode Mode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mode = normalizeMode(mode)
}

// GetMode returns the current approval mode.
func (p *Policy) GetMode() Mode {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mode
}

// SetOverride sets a per-tool override.
func (p *Policy) SetOverride(toolName string, override ToolOverride) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.toolOverrides[toolName] = override
}

// RemoveOverride removes a per-tool override.
func (p *Policy) RemoveOverride(toolName string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.toolOverrides, toolName)
}

// GetOverride returns the override for a tool, if any.
func (p *Policy) GetOverride(toolName string) (ToolOverride, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	override, ok := p.toolOverrides[toolName]
	return override, ok
}

// ClearOverrides removes all per-tool overrides.
func (p *Policy) ClearOverrides() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.toolOverrides = make(map[string]ToolOverride)
}

// Evaluate decides whether a tool call requires approval.
//
// Parameters:
//   - toolName: the name of the tool being called
//   - toolRiskLevel: the risk level declared by the tool (empty = inferred)
//   - organizationID, conversationID: scope for prior-approval checks
//   - ctx: passed to ApprovalStore if present
func (p *Policy) Evaluate(toolName, toolRiskLevel, organizationID, conversationID string, ctx interface{}) Decision {
	p.mu.RLock()
	mode := p.mode
	override, hasOverride := p.toolOverrides[toolName]
	p.mu.RUnlock()

	// Determine effective risk level.
	risk := NormalizeRiskLevel(toolRiskLevel)
	if risk == "" {
		risk = InferRiskLevel(toolName)
	}
	if hasOverride && override.RiskLevel != nil {
		risk = *override.RiskLevel
	}

	// Determine approval requirement.
	var requiresApproval bool
	var reason string

	switch mode {
	case ModeAll:
		requiresApproval = true
		reason = "mode=all: all tools require approval"
	case ModeNone:
		requiresApproval = false
		reason = "mode=none: no tools require approval"
	case ModeCustom:
		if hasOverride && override.RequiresApproval != nil {
			requiresApproval = *override.RequiresApproval
			reason = "mode=custom: per-tool override"
		} else {
			// Fall back to tiered logic for tools without overrides.
			requiresApproval, reason = p.tieredDecision(risk, toolName, organizationID, conversationID, ctx)
		}
	default: // ModeTiered
		requiresApproval, reason = p.tieredDecision(risk, toolName, organizationID, conversationID, ctx)
	}

	return Decision{
		RequiresApproval: requiresApproval,
		RiskLevel:        risk,
		Reason:           reason,
	}
}

func (p *Policy) tieredDecision(risk RiskLevel, toolName, organizationID, conversationID string, ctx interface{}) (bool, string) {
	switch risk {
	case RiskSafe:
		return false, "safe tool: auto-execute"
	case RiskMedium:
		if p.store != nil && strings.TrimSpace(conversationID) != "" {
			if p.store.HasPriorApproval(ctx, organizationID, conversationID, toolName) {
				return false, "medium tool: prior approval exists in conversation"
			}
		}
		return true, "medium tool: first-time approval required"
	case RiskDangerous:
		return true, "dangerous tool: always requires approval"
	default:
		return true, "unknown risk level: requires approval"
	}
}

// NormalizeRiskLevel converts a raw string to a RiskLevel constant.
func NormalizeRiskLevel(value string) RiskLevel {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "safe":
		return RiskSafe
	case "medium":
		return RiskMedium
	case "dangerous":
		return RiskDangerous
	default:
		return ""
	}
}

// InferRiskLevel guesses the risk level from a tool name.
// This mirrors the inference logic in the agent runner.
func InferRiskLevel(name string) RiskLevel {
	normalized := strings.ToLower(strings.TrimSpace(name))
	switch {
	case containsAny(normalized, "delete", "drop", "pay", "transfer", "execute_code"):
		return RiskDangerous
	case containsAny(normalized, "write", "create", "update", "post"):
		return RiskMedium
	default:
		return RiskSafe
	}
}

func containsAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func normalizeMode(value Mode) Mode {
	switch value {
	case ModeAll, ModeNone, ModeCustom:
		return value
	default:
		return ModeTiered
	}
}
