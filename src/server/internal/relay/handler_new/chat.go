package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: defaultWebSocketOriginPolicy.Allow,
}

// ChatHandler Chat Completions 处理
type ChatHandler struct {
	pool    *types.ChannelPoolInterface
	adapter *channel.OpenAIAdapter
}

func NewChatHandler(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter) *ChatHandler {
	return &ChatHandler{pool: p, adapter: a}
}

// Handle 同步请求
func (h *ChatHandler) Handle(c *gin.Context) error {
	var rawReq map[string]any
	if err := json.NewDecoder(c.Request.Body).Decode(&rawReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "invalid JSON body"}})
		return nil
	}

	model, _ := rawReq["model"].(string)
	if model == "" {
		model = "gpt-4o"
	}
	stream, _ := rawReq["stream"].(bool)

	// 构建 ProviderRequest
	req := &channel.ProviderRequest{
		APIType:   types.APITypeChat,
		Model:     model,
		Stream:    stream,
		Messages:  parseMessages(rawReq),
		MaxTokens: parseInt(rawReq["max_tokens"]),
	}

	// 估算用量
	usage := h.adapter.EstimateUsage(req)
	if req.Stream {
		c.Request = c.Request.WithContext(types.WithTrustedStreaming(c.Request.Context(), true))
	}

	// 通过 executeRequest 路由
	resp, err := h.executeRequest(c, req, usage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "relay_error", "message": err.Error()}})
		return nil
	}

	if stream {
		h.handleStream(c, req, resp)
		return nil
	}

	c.Data(http.StatusOK, "application/json", resp.Content)
	return nil
}

// handleStream SSE 流式处理
func (h *ChatHandler) handleStream(c *gin.Context, req *channel.ProviderRequest, resp *types.ProviderResponse) {
	if c.Writer.Written() {
		return
	}

	writeChatSSEHeaders(c)
	if len(resp.Content) > 0 {
		c.Writer.Write(resp.Content)
		c.Writer.(http.Flusher).Flush()
	}
}

// HandleStream 实现 Handler 接口（用于 WebSocket，但 Chat 用 Handle）
func (h *ChatHandler) HandleStream(c *gin.Context) error {
	return h.Handle(c)
}

func (h *ChatHandler) executeRequest(c *gin.Context, req *channel.ProviderRequest, usage *types.Usage) (*types.ProviderResponse, error) {
	router := GetRouter()
	if router == nil {
		return nil, types.ErrNoAvailableChannel
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("chat_%d", time.Now().UnixNano())
	}

	return router.RouteWithBilling(
		c.Request.Context(),
		req.APIType,
		req.Model,
		"", // channel selected by router
		idempotencyKey,
		usage,
		func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
			if ch == nil || ch.Channel == nil {
				return nil, types.ErrNoAvailableChannel
			}
			adapter, err := channel.AdapterForChannel(ch.Channel)
			if err != nil {
				return nil, err
			}
			upstreamURL, err := adapter.BuildURL(req.Model, req.APIType)
			if err != nil {
				return nil, err
			}
			headers, err := adapter.BuildHeaders(c.Request.Context(), req.Model, req.APIType)
			if err != nil {
				return nil, err
			}

			providerReq := &channel.ProviderRequest{
				APIType:   req.APIType,
				Model:     req.Model,
				URL:       upstreamURL,
				Stream:    req.Stream,
				Messages:  req.Messages,
				MaxTokens: req.MaxTokens,
				Headers:   headers,
			}
			if req.Stream {
				providerReq.StreamChunkCallback = func(chunk []byte) error {
					if !c.Writer.Written() {
						writeChatSSEHeaders(c)
						c.Status(http.StatusOK)
					}
					if _, err := c.Writer.Write(chunk); err != nil {
						return err
					}
					c.Writer.Flush()
					return nil
				}
			}

			return h.doUpstreamRequest(providerReq)
		},
	)
}

func (h *ChatHandler) doUpstreamRequest(req *channel.ProviderRequest) (*types.ProviderResponse, error) {
	body, err := marshalRequest(req)
	if err != nil {
		return nil, err
	}
	upstreamReq, err := http.NewRequest("POST", req.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	upstreamReq.Header = req.Headers.Clone()
	upstreamReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var bodyOut []byte
	if req.Stream && req.StreamChunkCallback != nil && resp.StatusCode < http.StatusBadRequest {
		bodyOut, err = copyChatProviderStream(resp.Body, req.StreamChunkCallback)
		if err != nil {
			return nil, err
		}
	} else {
		bodyOut, _ = io.ReadAll(resp.Body)
	}
	return &types.ProviderResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header.Clone(),
		Content:    bodyOut,
	}, nil
}

func writeChatSSEHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
}

func copyChatProviderStream(body io.Reader, onChunk func([]byte) error) ([]byte, error) {
	var captured bytes.Buffer
	buffer := make([]byte, 32*1024)
	for {
		n, readErr := body.Read(buffer)
		if n > 0 {
			chunk := append([]byte(nil), buffer[:n]...)
			if _, err := captured.Write(chunk); err != nil {
				return nil, err
			}
			if err := onChunk(chunk); err != nil {
				return nil, err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	return captured.Bytes(), nil
}
