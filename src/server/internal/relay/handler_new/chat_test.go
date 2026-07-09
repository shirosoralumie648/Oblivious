package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

func TestChatStreamProxiesFirstProviderChunkBeforeUpstreamCompletes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamReceived := make(chan struct{})
	releaseUpstream := make(chan struct{})
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() {
			close(releaseUpstream)
		})
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var upstreamBody struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Errorf("decode upstream body: %v", err)
			return
		}
		if upstreamBody.Model != "gpt-4o-mini" || !upstreamBody.Stream {
			t.Errorf("upstream stream request not preserved: %+v", upstreamBody)
			return
		}
		close(upstreamReceived)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		select {
		case <-releaseUpstream:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() {
		release()
		upstream.Close()
	})

	router := &chatStreamBillingRouter{
		channel: &types.RouteChannel{
			Channel: &types.Channel{
				ID:       "ch_chat_stream",
				Provider: "openai",
				BaseURL:  upstream.URL,
				APIKey:   "sk-chat-stream",
				Enabled:  true,
			},
			ChannelID: "ch_chat_stream",
			Enabled:   true,
			Healthy:   true,
		},
	}
	previous := GetRouter()
	SetRouter(router)
	t.Cleanup(func() {
		SetRouter(previous)
	})

	handler := NewChatHandler(nil, channel.NewOpenAIAdapter("https://direct.example.invalid", "sk-direct"))
	engine := gin.New()
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		_ = handler.Handle(c)
	})
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)

	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini","stream":true,"messages":[{"role":"user","content":"hello"}]}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	responseCh := make(chan *http.Response, 1)
	errCh := make(chan error, 1)
	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			errCh <- err
			return
		}
		responseCh <- resp
	}()

	select {
	case <-upstreamReceived:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("upstream did not receive streaming request")
	}

	var resp *http.Response
	select {
	case err := <-errCh:
		t.Fatalf("client request failed before first chunk: %v", err)
	case resp = <-responseCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected first streaming response before upstream completed; handler buffered the upstream stream")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if contentType := resp.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", contentType)
	}

	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil {
		t.Fatalf("read first streaming line: %v", err)
	}
	if !strings.Contains(line, `"hel"`) {
		t.Fatalf("expected first provider SSE chunk, got %q", line)
	}
	release()
	if router.routeWithBillingCalls != 1 {
		t.Fatalf("RouteWithBilling calls = %d, want 1", router.routeWithBillingCalls)
	}
}

func TestChatStreamDoesNotWriteCapturedStreamTwice(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"once\"}}]}\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}))
	t.Cleanup(upstream.Close)

	router := &chatStreamBillingRouter{
		channel: &types.RouteChannel{
			Channel: &types.Channel{
				ID:       "ch_chat_stream_once",
				Provider: "openai",
				BaseURL:  upstream.URL,
				APIKey:   "sk-chat-stream",
				Enabled:  true,
			},
			ChannelID: "ch_chat_stream_once",
			Enabled:   true,
			Healthy:   true,
		},
	}
	previous := GetRouter()
	SetRouter(router)
	t.Cleanup(func() {
		SetRouter(previous)
	})

	handler := NewChatHandler(nil, channel.NewOpenAIAdapter("https://direct.example.invalid", "sk-direct"))
	engine := gin.New()
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		_ = handler.Handle(c)
	})
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)

	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini","stream":true,"messages":[{"role":"user","content":"hello"}]}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("client request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if got := strings.Count(string(body), `"once"`); got != 1 {
		t.Fatalf("expected exactly one streamed chunk, got %d occurrences in body %q", got, string(body))
	}
}

type chatStreamBillingRouter struct {
	routeWithBillingCalls int
	channel               *types.RouteChannel
}

func (r *chatStreamBillingRouter) Route(_ context.Context, _ string, fn func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	return fn(r.channel)
}

func (r *chatStreamBillingRouter) RouteWithBilling(_ context.Context, _ types.APIType, _, _, _ string, _ *types.Usage, fn func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	r.routeWithBillingCalls++
	return fn(r.channel)
}

func (r *chatStreamBillingRouter) RecordChannelSuccess(_ string) {}

func (r *chatStreamBillingRouter) RecordChannelFailure(_ string) {}
