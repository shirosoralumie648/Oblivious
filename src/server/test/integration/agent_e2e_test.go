//go:build integration

package integration_test

import (
	"testing"
)

// TestAgentWithModelRouter verifies model routing with iteration context.
// NOTE: Cannot import agent package due to conflicting package declarations (main vs agent).
// Run unit tests in internal/agent/model_router_integration_test.go instead.
func TestAgentWithModelRouter(t *testing.T) {
	t.Skip("Run internal/agent/model_router_integration_test.go - package conflict prevents import")
}

// TestAgentWithSkillSelector verifies skill selection and prompt injection.
// NOTE: Cannot import agent package due to conflicting package declarations (main vs agent).
// Run unit tests in internal/agent/runner_skill_test.go instead.
func TestAgentWithSkillSelector(t *testing.T) {
	t.Skip("Run internal/agent/runner_skill_test.go - package conflict prevents import")
}

// TestCallAgentTool verifies recursive agent delegation with call_agent tool.
// Requires fixing package conflicts in internal/agent first.
func TestCallAgentTool(t *testing.T) {
	t.Skip("Requires fixing internal/agent package conflicts (main vs agent)")
}

// TestWebsearchFallback verifies websearch provider fallback chain.
// Requires fixing package conflicts in internal/agent first.
func TestWebsearchFallback(t *testing.T) {
	t.Skip("Requires fixing internal/agent package conflicts (main vs agent)")
}
