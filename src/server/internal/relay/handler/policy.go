package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/types"
)

type CommercialClass string

const (
	CommercialSupportedBilled CommercialClass = "commercial_supported_billed"
	InternalAdminOnly         CommercialClass = "internal_admin_only"
	DisabledInProduction      CommercialClass = "disabled_in_production"
)

type RoutePolicy struct {
	Method            string
	Path              string
	APIType           types.APIType
	Strategy          types.HandlerStrategy
	Class             CommercialClass
	ProductionEnabled bool
	DisabledReason    string
	FutureOwner       string
}

func AllRoutePolicies() []RoutePolicy {
	policies := make([]RoutePolicy, len(routePolicies))
	copy(policies, routePolicies)
	return policies
}

func PolicyForRoute(method, path string) (RoutePolicy, bool) {
	for _, policy := range routePolicies {
		if policy.Method == method && policy.Path == path {
			return policy, true
		}
	}
	return RoutePolicy{}, false
}

func RejectIfProductionDisabled(c *gin.Context, route Route, production bool) bool {
	if !production {
		return false
	}

	policy, ok := PolicyForRoute(route.Method, route.Path)
	if !ok {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": gin.H{
				"code":    "endpoint_policy_missing",
				"message": "relay endpoint is missing commercial policy: " + route.Method + " " + route.Path,
			},
		})
		return true
	}

	if policy.ProductionEnabled && policy.Class != DisabledInProduction {
		return false
	}

	message := "relay endpoint is disabled in production: " + route.Method + " " + route.Path
	if policy.DisabledReason != "" {
		message += " (" + policy.DisabledReason + ")"
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": gin.H{
			"code":    "endpoint_disabled_in_production",
			"message": message,
		},
	})
	return true
}

var routePolicies = []RoutePolicy{
	supported("POST", "/v1/chat/completions", types.APITypeChat, types.StrategyNative),
	supported("POST", "/v1/responses", types.APITypeResponses, types.StrategyNative),
	disabled("GET", "/v1/realtime", types.APITypeRealtime, types.StrategyNative, "realtime settlement and client-abort billing are not defined", "Phase 15"),
	supported("POST", "/v1/embeddings", types.APITypeEmbeddings, types.StrategyNative),
	supported("POST", "/v1/images/generations", types.APITypeImageGen, types.StrategyNative),
	supported("POST", "/v1/images/edits", types.APITypeImageEdit, types.StrategyNative),
	supported("POST", "/v1/images/variations", types.APITypeImageVar, types.StrategyNative),
	disabled("POST", "/v1/videos", types.APITypeVideos, types.StrategyNative, "video billing and provider behavior are not verified", "Phase 15"),
	supported("POST", "/v1/audio/speech", types.APITypeAudioSpeech, types.StrategyNative),
	supported("POST", "/v1/audio/transcriptions", types.APITypeAudioSTT, types.StrategyNative),
	supported("POST", "/v1/audio/translations", types.APITypeAudioTranslate, types.StrategyNative),
	supported("POST", "/v1/moderations", types.APITypeModeration, types.StrategyNative),
	supported("POST", "/v1/completions", types.APITypeCompletions, types.StrategyNative),
	disabled("POST", "/v1/batch", types.APITypeBatch, types.StrategyNative, "async batch settlement and audit are not defined", "Phase 15"),
	disabled("GET", "/v1/batches", types.APITypeBatch, types.StrategyPassthrough, "batch passthrough lacks commercial audit and settlement", "Phase 15"),
	disabled("GET", "/v1/batches/:id", types.APITypeBatch, types.StrategyPassthrough, "batch passthrough lacks commercial audit and settlement", "Phase 15"),
	disabled("POST", "/v1/files", types.APITypeFiles, types.StrategyFileProxy, "file mapping persistence and storage billing are not implemented", "Phase 15"),
	disabled("GET", "/v1/files", types.APITypeFiles, types.StrategyPassthrough, "file passthrough lacks tenant file ownership and audit", "Phase 15"),
	disabled("GET", "/v1/files/:id", types.APITypeFiles, types.StrategyPassthrough, "file passthrough lacks tenant file ownership and audit", "Phase 15"),
	disabled("DELETE", "/v1/files/:id", types.APITypeFiles, types.StrategyPassthrough, "file passthrough lacks tenant file ownership and audit", "Phase 15"),
	disabled("GET", "/v1/files/:id/content", types.APITypeFiles, types.StrategyPassthrough, "file passthrough lacks tenant file ownership and audit", "Phase 15"),
	disabled("POST", "/v1/fine_tuning/jobs", types.APITypeFineTuning, types.StrategyPassthrough, "fine-tuning job lifecycle and training-token billing are not implemented", "Phase 15"),
	disabled("GET", "/v1/fine_tuning/jobs", types.APITypeFineTuning, types.StrategyPassthrough, "fine-tuning job lifecycle and audit are not implemented", "Phase 15"),
	disabled("GET", "/v1/fine_tuning/jobs/:id", types.APITypeFineTuning, types.StrategyPassthrough, "fine-tuning job lifecycle and audit are not implemented", "Phase 15"),
	disabled("POST", "/v1/fine_tuning/jobs/:id/cancel", types.APITypeFineTuning, types.StrategyPassthrough, "fine-tuning cancel billing and audit are not implemented", "Phase 15"),
	disabled("GET", "/v1/fine_tuning/jobs/:id/events", types.APITypeFineTuning, types.StrategyPassthrough, "fine-tuning event streaming audit is not implemented", "Phase 15"),
	disabled("POST", "/v1/assistants", types.APITypeAssistants, types.StrategyPassthrough, "assistants lifecycle billing and governance are not implemented", "Phase 15"),
	disabled("GET", "/v1/assistants", types.APITypeAssistants, types.StrategyPassthrough, "assistants lifecycle billing and governance are not implemented", "Phase 15"),
	disabled("GET", "/v1/assistants/:id", types.APITypeAssistants, types.StrategyPassthrough, "assistants lifecycle billing and governance are not implemented", "Phase 15"),
	disabled("POST", "/v1/threads", types.APITypeThreads, types.StrategyPassthrough, "threads lifecycle billing and audit are not implemented", "Phase 15"),
	disabled("GET", "/v1/threads/:id", types.APITypeThreads, types.StrategyPassthrough, "threads lifecycle billing and audit are not implemented", "Phase 15"),
	disabled("POST", "/v1/threads/:id/runs", types.APITypeRuns, types.StrategyPassthrough, "runs lifecycle billing and tool-call audit are not implemented", "Phase 15"),
	disabled("GET", "/v1/threads/:id/runs/:rid", types.APITypeRuns, types.StrategyPassthrough, "runs lifecycle billing and tool-call audit are not implemented", "Phase 15"),
	disabled("POST", "/v1/threads/:id/runs/:rid/submit", types.APITypeRuns, types.StrategyPassthrough, "run submit-tool-output billing and audit are not implemented", "Phase 15"),
}

func supported(method, path string, apiType types.APIType, strategy types.HandlerStrategy) RoutePolicy {
	return RoutePolicy{
		Method:            method,
		Path:              path,
		APIType:           apiType,
		Strategy:          strategy,
		Class:             CommercialSupportedBilled,
		ProductionEnabled: true,
	}
}

func disabled(method, path string, apiType types.APIType, strategy types.HandlerStrategy, reason, futureOwner string) RoutePolicy {
	return RoutePolicy{
		Method:            method,
		Path:              path,
		APIType:           apiType,
		Strategy:          strategy,
		Class:             DisabledInProduction,
		ProductionEnabled: false,
		DisabledReason:    reason,
		FutureOwner:       futureOwner,
	}
}
