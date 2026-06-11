package agent

import "strings"

// ModelRouter selects the model for an agent run iteration based on routing
// rules. When no rule matches, it returns the fallback (agent's static model).
type ModelRouter struct {
	Rules    []ModelRoutingRule
	Fallback string
}

// IterationContext holds task signals the router uses to match rules.
type IterationContext struct {
	InputText       string
	Iteration       int  // 1-based
	HasToolResult   bool // prior tool result exists
	InputCharLength int  // computed from InputText if not set
}

// SelectModel returns the first matching rule's target model, or the fallback.
func (r *ModelRouter) SelectModel(ctx IterationContext) string {
	if ctx.InputCharLength == 0 && ctx.InputText != "" {
		ctx.InputCharLength = len(ctx.InputText)
	}
	for _, rule := range r.Rules {
		if matchesRule(rule, ctx) {
			return rule.TargetModel
		}
	}
	return r.Fallback
}

func matchesRule(rule ModelRoutingRule, ctx IterationContext) bool {
	if rule.MinInputChars > 0 && ctx.InputCharLength < rule.MinInputChars {
		return false
	}
	if rule.MaxInputChars > 0 && ctx.InputCharLength > rule.MaxInputChars {
		return false
	}
	if rule.MinIteration > 0 && ctx.Iteration < rule.MinIteration {
		return false
	}
	if rule.RequiresToolResult && !ctx.HasToolResult {
		return false
	}
	if len(rule.Keywords) > 0 {
		lower := strings.ToLower(ctx.InputText)
		matched := false
		for _, kw := range rule.Keywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
