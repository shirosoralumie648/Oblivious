package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
	appws "oblivious/server/internal/ws"
)

var defaultWebSocketOriginPolicy = appws.NewOriginPolicy(nil)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin:     defaultWebSocketOriginPolicy.Allow,
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// RealtimeHandler Realtime WebSocket 处理
type RealtimeHandler struct {
	pool                       *types.ChannelPoolInterface
	adapter                    *channel.OpenAIAdapter
	originPolicy               appws.OriginPolicy
	commercialLifecycleEnabled bool
	mu                         sync.Map // connectionID -> session
}

func NewRealtimeHandler(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter) *RealtimeHandler {
	return NewRealtimeHandlerWithOriginPolicy(p, a, defaultWebSocketOriginPolicy)
}

func NewRealtimeHandlerWithOriginPolicy(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter, originPolicy appws.OriginPolicy) *RealtimeHandler {
	return &RealtimeHandler{pool: p, adapter: a, originPolicy: originPolicy}
}

func (h *RealtimeHandler) WithCommercialLifecycleEnabled(enabled bool) *RealtimeHandler {
	h.commercialLifecycleEnabled = enabled
	return h
}

func (h *RealtimeHandler) Handle(c *gin.Context) error {
	return h.HandleStream(c)
}

// HandleStream WebSocket 连接入口
func (h *RealtimeHandler) HandleStream(c *gin.Context) error {
	if !h.lifecycleEnabled() {
		h.writeDisabled(c)
		return nil
	}
	if !h.originPolicy.Allow(c.Request) {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "forbidden_origin", "message": "websocket origin is not allowed"}})
		return nil
	}

	// 1. 解析 model
	model := strings.TrimSpace(c.Query("model"))
	if model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "realtime_model_required", "message": "realtime model is required"}})
		return nil
	}

	// 2. 获取 connectionID（用于幂等）
	connectionID := c.GetHeader("OpenAI-Realtime-Connection-ID")
	if connectionID == "" {
		connectionID = c.Query("connection_id")
	}
	if connectionID == "" {
		connectionID = c.GetHeader(types.HeaderRequestID)
	}
	if connectionID == "" {
		connectionID = fmt.Sprintf("realtime_%d", time.Now().UnixNano())
	}

	if router := GetRouter(); router != nil {
		ctx := types.WithTrustedStreaming(c.Request.Context(), true)
		resp, err := router.RouteWithBilling(
			ctx,
			types.APITypeRealtime,
			model,
			"",
			connectionID,
			&types.Usage{},
			func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
				if ch == nil || ch.Channel == nil {
					return nil, types.ErrNoAvailableChannel
				}
				adapter, err := channel.AdapterForChannel(ch.Channel)
				if err != nil {
					return nil, err
				}
				usage, err := h.proxyRealtime(c, adapter, model)
				if err != nil {
					return nil, err
				}
				return &types.ProviderResponse{StatusCode: http.StatusOK, Done: true, Usage: usage}, nil
			},
		)
		if err != nil {
			if isRealtimePostUpgradeError(err) {
				return nil
			}
			writeRelayHandlerError(c, resp, err)
		}
		return nil
	}

	if _, err := h.proxyRealtime(c, h.adapter, model); err != nil {
		if !c.Writer.Written() {
			c.JSON(http.StatusBadGateway, gin.H{"error": "upstream connection failed"})
		}
		return nil
	}

	return nil
}

func (h *RealtimeHandler) lifecycleEnabled() bool {
	return h != nil && h.commercialLifecycleEnabled
}

func (h *RealtimeHandler) writeDisabled(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": gin.H{
		"code":    "unsupported_api",
		"message": "realtime API is disabled until the commercial streaming billing and settlement lifecycle is enabled",
	}})
}

func (h *RealtimeHandler) proxyRealtime(c *gin.Context, adapter types.ProviderAdapter, model string) (*types.Usage, error) {
	upstreamURL, err := adapter.BuildURL(model, types.APITypeRealtime)
	if err != nil {
		return nil, err
	}
	upstreamURL = realtimeWebSocketURL(upstreamURL)
	headers, err := adapter.BuildHeaders(c.Request.Context(), model, types.APITypeRealtime)
	if err != nil {
		return nil, err
	}
	upstreamConn, _, err := websocket.DefaultDialer.Dial(upstreamURL, headers)
	if err != nil {
		return nil, err
	}
	defer upstreamConn.Close()

	// 3. 获取客户端 WebSocket 连接
	upgrader := wsUpgrader
	upgrader.CheckOrigin = h.originPolicy.Allow
	clientConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return nil, err
	}
	defer clientConn.Close()

	// 4. 双向代理
	var wg sync.WaitGroup
	wg.Add(2)
	usage := &types.Usage{}
	var usageMu sync.Mutex

	// client -> upstream
	go func() {
		defer wg.Done()
		for {
			_, msg, err := clientConn.ReadMessage()
			if err != nil {
				upstreamConn.Close()
				break
			}
			upstreamConn.WriteMessage(websocket.TextMessage, msg)
		}
	}()

	// upstream -> client
	go func() {
		defer wg.Done()
		for {
			_, msg, err := upstreamConn.ReadMessage()
			if err != nil {
				clientConn.Close()
				break
			}
			if parsed := parseRealtimeUsageEvent(msg); parsed != nil {
				usageMu.Lock()
				mergeRealtimeUsage(usage, parsed)
				usageMu.Unlock()
			}
			clientConn.WriteMessage(websocket.TextMessage, msg)
		}
	}()

	wg.Wait()

	usageMu.Lock()
	finalUsage := *usage
	usageMu.Unlock()
	if realtimeUsageIsZero(&finalUsage) {
		return nil, realtimePostUpgradeError{err: fmt.Errorf("realtime_usage_missing: upstream realtime stream closed without usage")}
	}
	return &finalUsage, nil
}

type realtimePostUpgradeError struct {
	err error
}

func (e realtimePostUpgradeError) Error() string {
	if e.err == nil {
		return "realtime post-upgrade error"
	}
	return e.err.Error()
}

func (e realtimePostUpgradeError) Unwrap() error {
	return e.err
}

func (e realtimePostUpgradeError) RelayErrorCode() string {
	return "realtime_usage_missing"
}

func isRealtimePostUpgradeError(err error) bool {
	_, ok := err.(realtimePostUpgradeError)
	return ok
}

func parseRealtimeUsageEvent(msg []byte) *types.Usage {
	var payload struct {
		Type     string `json:"type"`
		Response struct {
			Usage realtimeUsagePayload `json:"usage"`
		} `json:"response"`
		Usage realtimeUsagePayload `json:"usage"`
	}
	if err := json.Unmarshal(msg, &payload); err != nil {
		return nil
	}
	usage := payload.Response.Usage.toRelayUsage()
	if usage == nil {
		usage = payload.Usage.toRelayUsage()
	}
	return usage
}

type realtimeUsagePayload struct {
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (p realtimeUsagePayload) toRelayUsage() *types.Usage {
	promptTokens := p.PromptTokens
	if promptTokens == 0 {
		promptTokens = p.InputTokens
	}
	completionTokens := p.CompletionTokens
	if completionTokens == 0 {
		completionTokens = p.OutputTokens
	}
	totalTokens := p.TotalTokens
	if totalTokens == 0 {
		totalTokens = promptTokens + completionTokens
	}
	if promptTokens == 0 && completionTokens == 0 && totalTokens == 0 {
		return nil
	}
	return &types.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
	}
}

func mergeRealtimeUsage(dst, src *types.Usage) {
	if dst == nil || src == nil {
		return
	}
	if src.PromptTokens > 0 {
		dst.PromptTokens = src.PromptTokens
	}
	if src.CompletionTokens > 0 {
		dst.CompletionTokens = src.CompletionTokens
	}
	if src.TotalTokens > 0 {
		dst.TotalTokens = src.TotalTokens
	}
}

func realtimeUsageIsZero(usage *types.Usage) bool {
	if usage == nil {
		return true
	}
	return usage.PromptTokens == 0 &&
		usage.CompletionTokens == 0 &&
		usage.TotalTokens == 0 &&
		usage.ImageCount == 0 &&
		usage.VideoCount == 0 &&
		usage.AudioSeconds == 0 &&
		usage.StorageBytes == 0 &&
		usage.TrainingTokens == 0
}

func realtimeWebSocketURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	}
	return parsed.String()
}
