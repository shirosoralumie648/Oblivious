package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
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
		msg := channel.Message{
			Role:       getString(mm, "role"),
			Content:    getString(mm, "content"),
			ToolCallID: getString(mm, "tool_call_id"),
		}

		// Preserve tool_calls array from assistant messages.
		if tcRaw, ok := mm["tool_calls"].([]any); ok {
			toolCalls := make([]types.ToolCall, 0, len(tcRaw))
			for _, tc := range tcRaw {
				tcm, ok := tc.(map[string]any)
				if !ok {
					continue
				}
				var fn types.ToolFunction
				if fnRaw, ok := tcm["function"].(map[string]any); ok {
					fn = types.ToolFunction{
						Name:      getString(fnRaw, "name"),
						Arguments: getString(fnRaw, "arguments"),
					}
				}
				toolCalls = append(toolCalls, types.ToolCall{
					ID:       getString(tcm, "id"),
					Type:     getString(tcm, "type"),
					Function: fn,
				})
			}
			msg.ToolCalls = toolCalls
		}

		messages = append(messages, msg)
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

func applyTrustedInternalIdentity(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := types.TrustedAPITokenIDFromContext(ctx); ok {
		return
	}
	if _, ok := types.TrustedUserIDFromContext(ctx); ok {
		return
	}
	if _, ok := types.TrustedOrganizationIDFromContext(ctx); ok {
		return
	}
	userID, organizationID, requestID, userGroup, ok := trustedIdentityFromHeaders(c, false)
	if !ok {
		return
	}
	ctx = types.WithTrustedUserID(ctx, userID)
	ctx = types.WithTrustedOrganizationID(ctx, organizationID)
	if requestID != "" {
		ctx = types.WithTrustedRequestID(ctx, requestID)
	}
	if userGroup != "" {
		ctx = types.WithTrustedUserGroup(ctx, userGroup)
	}
	if conversationID := strings.TrimSpace(c.GetHeader(types.HeaderInternalConversation)); conversationID != "" {
		ctx = types.WithTrustedConversationID(ctx, conversationID)
	}
	c.Request = c.Request.WithContext(ctx)
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
			entry := map[string]any{
				"role":    msg.Role,
				"content": msg.Content,
			}
			if len(msg.ToolCalls) > 0 {
				tcs := make([]map[string]any, len(msg.ToolCalls))
				for j, tc := range msg.ToolCalls {
					tcs[j] = map[string]any{
						"id":   tc.ID,
						"type": tc.Type,
						"function": map[string]any{
							"name":      tc.Function.Name,
							"arguments": tc.Function.Arguments,
						},
					}
				}
				entry["tool_calls"] = tcs
			}
			if msg.ToolCallID != "" {
				entry["tool_call_id"] = msg.ToolCallID
			}
			messages[i] = entry
		}
		m["messages"] = messages
	}
	if req.MaxTokens > 0 {
		m["max_tokens"] = req.MaxTokens
	}
	if req.Input != "" {
		m["input"] = req.Input
	}
	if len(req.Tools) > 0 {
		m["tools"] = req.Tools
	}
	if req.ToolChoice != nil {
		m["tool_choice"] = req.ToolChoice
	}
	return json.Marshal(m)
}

func providerResponseFromHTTP(statusCode int, body []byte) *types.ProviderResponse {
	return providerResponseFromHTTPWithHeaders(statusCode, nil, body)
}

func providerResponseFromHTTPWithHeaders(statusCode int, headers http.Header, body []byte) *types.ProviderResponse {
	return &types.ProviderResponse{
		StatusCode: statusCode,
		Headers:    headers.Clone(),
		Content:    body,
		Done:       true,
		Usage:      parseProviderUsage(body),
	}
}

func parseProviderUsage(body []byte) *types.Usage {
	var payload struct {
		Usage *types.Usage `json:"usage"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	return payload.Usage
}

func executeProviderAdapterRequest(ctx context.Context, adapter types.ProviderAdapter, req *channel.ProviderRequest) (*types.ProviderResponse, error) {
	resp, err := adapter.DoRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyOut, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return &types.ProviderResponse{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header.Clone(),
			Content:    bodyOut,
			Done:       true,
			Error:      adapter.MapError(resp.StatusCode, bodyOut),
		}, nil
	}
	providerResp, err := adapter.ConvertResponse(bodyOut, req.Stream)
	if err != nil {
		return nil, err
	}
	if providerResp == nil {
		return providerResponseFromHTTPWithHeaders(resp.StatusCode, resp.Header, bodyOut), nil
	}
	providerResp.StatusCode = resp.StatusCode
	providerResp.Headers = resp.Header.Clone()
	if len(providerResp.Content) == 0 {
		providerResp.Content = bodyOut
	}
	providerResp.Done = true
	return providerResp, nil
}

type relayStatusError interface {
	StatusCode() int
}

type relayCodeError interface {
	RelayErrorCode() string
}

func writeRelayHandlerError(c *gin.Context, resp *types.ProviderResponse, err error) {
	if resp != nil {
		statusCode := resp.StatusCode
		if statusCode < http.StatusContinue {
			statusCode = http.StatusBadGateway
		}
		if len(resp.Content) > 0 && (resp.StatusCode >= http.StatusBadRequest || resp.Error != nil || err == nil) {
			c.Data(statusCode, "application/json", resp.Content)
			return
		}
		if resp.Error != nil {
			code := resp.Error.Code
			if code == "" {
				code = "provider_error"
			}
			message := resp.Error.Message
			if message == "" && err != nil {
				message = err.Error()
			}
			c.JSON(statusCode, gin.H{"error": gin.H{"code": code, "message": message}})
			return
		}
	}

	statusCode := http.StatusInternalServerError
	if err != nil {
		if statusErr, ok := err.(relayStatusError); ok && statusErr.StatusCode() >= http.StatusBadRequest {
			statusCode = statusErr.StatusCode()
		}
	}
	message := "relay request failed"
	code := "relay_error"
	if err != nil {
		message = err.Error()
		if codedErr, ok := err.(relayCodeError); ok && codedErr.RelayErrorCode() != "" {
			code = codedErr.RelayErrorCode()
		}
	}
	c.JSON(statusCode, gin.H{"error": gin.H{"code": code, "message": message}})
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
