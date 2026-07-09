package handler

import (
	"net/http"
	"net/url"
	"strings"
	"sync"

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
	commercialLifecycleEnabled bool
	mu                         sync.Map // connectionID -> session
}

func NewRealtimeHandler(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter) *RealtimeHandler {
	return &RealtimeHandler{pool: p, adapter: a}
}

func (h *RealtimeHandler) WithCommercialLifecycleEnabled(enabled bool) *RealtimeHandler {
	h.commercialLifecycleEnabled = enabled
	return h
}

// HandleStream WebSocket 连接入口
func (h *RealtimeHandler) HandleStream(c *gin.Context) error {
	// 1. 解析 model
	if !h.lifecycleEnabled() {
		h.writeDisabled(c)
		return nil
	}
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
	_ = connectionID

	// 3. TODO: 鉴权（后续 Auth 中间件实现）

	// 4. TODO: PreBill 预扣（Plan D 实现 BillingHook 后启用）

	// 5. Upgrade 至 WebSocket
	upstreamURL, _ := h.adapter.BuildURL(model, types.APITypeRealtime)
	upstreamURL = realtimeWebSocketURL(upstreamURL)
	headers, _ := h.adapter.BuildHeaders(c.Request.Context(), model, types.APITypeRealtime)

	upstreamConn, _, err := websocket.DefaultDialer.Dial(upstreamURL, headers)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream connection failed"})
		return nil
	}
	defer upstreamConn.Close()

	// 6. 获取客户端 WebSocket 连接
	clientConn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return err
	}
	defer clientConn.Close()

	// 7. 双向代理
	var wg sync.WaitGroup
	wg.Add(2)

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
			clientConn.WriteMessage(websocket.TextMessage, msg)
		}
	}()

	wg.Wait()

	// 8. TODO: 连接关闭后结算

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
