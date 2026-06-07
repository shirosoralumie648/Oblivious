package handler

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"oblivious/server/internal/relay/types"
	"oblivious/server/internal/relay/channel"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// RealtimeHandler Realtime WebSocket 处理
type RealtimeHandler struct {
	pool    *types.ChannelPoolInterface
	adapter *channel.OpenAIAdapter
	mu      sync.Map // connectionID -> session
}

func NewRealtimeHandler(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter) *RealtimeHandler {
	return &RealtimeHandler{pool: p, adapter: a}
}

// HandleStream WebSocket 连接入口
func (h *RealtimeHandler) HandleStream(c *gin.Context) error {
	// 1. 解析 model
	model := c.Query("model")
	if model == "" {
		model = "gpt-4o-realtime-preview"
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
	upstreamURL = upstreamURL + "/v1/realtime"
	upstreamReq, _ := http.NewRequest("GET", upstreamURL, nil)
	headers, _ := h.adapter.BuildHeaders(c.Request.Context(), model, types.APITypeRealtime)
	upstreamReq.Header = headers
	upstreamReq.Header.Set("Upgrade", "websocket")
	upstreamReq.Header.Set("Connection", "upgrade")

	upstreamConn, _, err := websocket.DefaultDialer.Dial(upstreamURL, upstreamReq.Header)
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
