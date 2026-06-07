package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/types"
	"oblivious/server/internal/relay/channel"
)

// parseMessages 从原始 JSON map 解析 Messages
func parseMessages(raw map[string]any) []channel.Message {
	messagesRaw, ok := raw["messages"].([]any)
	if !ok {
		return nil
	}
	messages := make([]channel.Message, 0, len(messagesRaw))
	for _, m := range messagesRaw {
		mm, ok := m.(map[string]any)
		if !ok {
			continue
		}
		messages = append(messages, channel.Message{
			Role:    getString(mm, "role"),
			Content: getString(mm, "content"),
		})
	}
	return messages
}

// parseInt 将 float64 (JSON unmarshal) 转换为 int
func parseInt(v any) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}

// getString 安全获取 map[string]any 中的 string
func getString(m map[string]any, key string) string {
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

// getBool 安全获取 map[string]any 中的 bool
func getBool(m map[string]any, key string) bool {
	if b, ok := m[key].(bool); ok {
		return b
	}
	return false
}

// buildUpstreamRequest 构建转发到上游的 HTTP 请求
func buildUpstreamRequest(req *channel.ProviderRequest) (*http.Request, error) {
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
	return upstreamReq, nil
}

// marshalRequest 将 ProviderRequest 序列化为 JSON
func marshalRequest(req *channel.ProviderRequest) ([]byte, error) {
	m := map[string]any{
		"model":  req.Model,
		"stream": req.Stream,
	}
	if len(req.Messages) > 0 {
		messages := make([]map[string]any, len(req.Messages))
		for i, msg := range req.Messages {
			messages[i] = map[string]any{"role": msg.Role, "content": msg.Content}
		}
		m["messages"] = messages
	}
	if req.MaxTokens > 0 {
		m["max_tokens"] = req.MaxTokens
	}
	if req.Input != "" {
		m["input"] = req.Input
	}
	return json.Marshal(m)
}

// passthroughHelper 通用的透传函数
func passthroughHelper(c *gin.Context, adapter *channel.OpenAIAdapter, method, path string, body []byte, apiType types.APIType) {
	upstreamURL, _ := adapter.BuildURL("gpt-4o", apiType)
	upstreamURL = upstreamURL + path
	req, err := http.NewRequest(method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
		return
	}
	headers, _ := adapter.BuildHeaders(c.Request.Context(), "gpt-4o", apiType)
	req.Header = headers
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
		return
	}
	defer resp.Body.Close()

	bodyOut, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, "application/json", bodyOut)
}
