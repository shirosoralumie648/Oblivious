package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/types"
)

var globalRouter types.RouterInterface

func SetRouter(r types.RouterInterface) {
	globalRouter = r
}

func GetRouter() types.RouterInterface {
	return globalRouter
}

// Route 定义
type Route struct {
	Method    string
	Path      string
	APIType   types.APIType
	Strategy  types.HandlerStrategy
	Retryable bool
}

// RegisterRoutes 注册全部 35 个 OpenAI 路由
func RegisterRoutes(e *gin.Engine, handlers map[types.APIType]types.Handler) {
	routes := getOpenAIRoutes()

	for _, r := range routes {
		h, ok := handlers[r.APIType]
		if !ok {
			continue
		}

		switch r.Strategy {
		case types.StrategyPassthrough:
			e.Handle(r.Method, r.Path, func(c *gin.Context) {
				h.Handle(c)
			})
		case types.StrategyFileProxy:
			e.Handle(r.Method, r.Path, func(c *gin.Context) {
				h.Handle(c)
			})
		default: // StrategyNative
			if r.APIType == types.APITypeRealtime {
				e.Handle(r.Method, r.Path, func(c *gin.Context) {
					h.HandleStream(c)
				})
			} else {
				e.Handle(r.Method, r.Path, func(c *gin.Context) {
					h.Handle(c)
				})
			}
		}
	}

	// 健康检查
	e.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}

// getOpenAIRoutes 返回所有 OpenAI 兼容路由定义
func getOpenAIRoutes() []Route {
	return []Route{
		// Chat / Responses
		{Method: "POST", Path: "/v1/chat/completions", APIType: types.APITypeChat, Strategy: types.StrategyNative, Retryable: true},
		{Method: "POST", Path: "/v1/responses", APIType: types.APITypeResponses, Strategy: types.StrategyNative, Retryable: true},

		// Realtime (WebSocket)
		{Method: "GET", Path: "/v1/realtime", APIType: types.APITypeRealtime, Strategy: types.StrategyNative, Retryable: false},

		// Embeddings
		{Method: "POST", Path: "/v1/embeddings", APIType: types.APITypeEmbeddings, Strategy: types.StrategyNative, Retryable: true},

		// Images
		{Method: "POST", Path: "/v1/images/generations", APIType: types.APITypeImageGen, Strategy: types.StrategyNative, Retryable: false},
		{Method: "POST", Path: "/v1/images/edits", APIType: types.APITypeImageEdit, Strategy: types.StrategyNative, Retryable: false},
		{Method: "POST", Path: "/v1/images/variations", APIType: types.APITypeImageVar, Strategy: types.StrategyNative, Retryable: false},

		// Videos
		{Method: "POST", Path: "/v1/videos", APIType: types.APITypeVideos, Strategy: types.StrategyNative, Retryable: false},

		// Audio
		{Method: "POST", Path: "/v1/audio/speech", APIType: types.APITypeAudioSpeech, Strategy: types.StrategyNative, Retryable: false},
		{Method: "POST", Path: "/v1/audio/transcriptions", APIType: types.APITypeAudioSTT, Strategy: types.StrategyNative, Retryable: false},
		{Method: "POST", Path: "/v1/audio/translations", APIType: types.APITypeAudioTranslate, Strategy: types.StrategyNative, Retryable: false},

		// Moderations / Legacy
		{Method: "POST", Path: "/v1/moderations", APIType: types.APITypeModeration, Strategy: types.StrategyNative, Retryable: false},
		{Method: "POST", Path: "/v1/completions", APIType: types.APITypeCompletions, Strategy: types.StrategyNative, Retryable: true},

		// Batch
		{Method: "POST", Path: "/v1/batch", APIType: types.APITypeBatch, Strategy: types.StrategyNative, Retryable: false},
		{Method: "GET", Path: "/v1/batches", APIType: types.APITypeBatch, Strategy: types.StrategyPassthrough, Retryable: false},
		{Method: "GET", Path: "/v1/batches/:id", APIType: types.APITypeBatch, Strategy: types.StrategyPassthrough, Retryable: false},

		// Files
		{Method: "POST", Path: "/v1/files", APIType: types.APITypeFiles, Strategy: types.StrategyFileProxy, Retryable: false},
		{Method: "GET", Path: "/v1/files", APIType: types.APITypeFiles, Strategy: types.StrategyPassthrough, Retryable: false},
		{Method: "GET", Path: "/v1/files/:id", APIType: types.APITypeFiles, Strategy: types.StrategyPassthrough, Retryable: false},
		{Method: "DELETE", Path: "/v1/files/:id", APIType: types.APITypeFiles, Strategy: types.StrategyPassthrough, Retryable: false},
		{Method: "GET", Path: "/v1/files/:id/content", APIType: types.APITypeFiles, Strategy: types.StrategyPassthrough, Retryable: false},

		// Fine-tuning
		{Method: "POST", Path: "/v1/fine_tuning/jobs", APIType: types.APITypeFineTuning, Strategy: types.StrategyPassthrough, Retryable: false},
		{Method: "GET", Path: "/v1/fine_tuning/jobs", APIType: types.APITypeFineTuning, Strategy: types.StrategyPassthrough, Retryable: false},
		{Method: "GET", Path: "/v1/fine_tuning/jobs/:id", APIType: types.APITypeFineTuning, Strategy: types.StrategyPassthrough, Retryable: false},
		{Method: "POST", Path: "/v1/fine_tuning/jobs/:id/cancel", APIType: types.APITypeFineTuning, Strategy: types.StrategyPassthrough, Retryable: false},
		{Method: "GET", Path: "/v1/fine_tuning/jobs/:id/events", APIType: types.APITypeFineTuning, Strategy: types.StrategyPassthrough, Retryable: false},

		// Assistants / Threads / Runs
		{Method: "POST", Path: "/v1/assistants", APIType: types.APITypeAssistants, Strategy: types.StrategyPassthrough, Retryable: false},
		{Method: "GET", Path: "/v1/assistants", APIType: types.APITypeAssistants, Strategy: types.StrategyPassthrough, Retryable: false},
		{Method: "GET", Path: "/v1/assistants/:id", APIType: types.APITypeAssistants, Strategy: types.StrategyPassthrough, Retryable: false},
		{Method: "POST", Path: "/v1/threads", APIType: types.APITypeThreads, Strategy: types.StrategyPassthrough, Retryable: false},
		{Method: "GET", Path: "/v1/threads/:id", APIType: types.APITypeThreads, Strategy: types.StrategyPassthrough, Retryable: false},
		{Method: "POST", Path: "/v1/threads/:id/runs", APIType: types.APITypeRuns, Strategy: types.StrategyPassthrough, Retryable: false},
		{Method: "GET", Path: "/v1/threads/:id/runs/:rid", APIType: types.APITypeRuns, Strategy: types.StrategyPassthrough, Retryable: false},
		{Method: "POST", Path: "/v1/threads/:id/runs/:rid/submit", APIType: types.APITypeRuns, Strategy: types.StrategyPassthrough, Retryable: false},
	}
}
