package http

import (
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"oblivious/server/internal/admin"
)

func TestAdminHandlerListsRelayProviderCatalog(t *testing.T) {
	handler := newAdminHandler(admin.NewService(nil))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/channel-providers", nil)

	handler.listChannelProviders(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected provider catalog 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data struct {
			Providers []struct {
				ID             string `json:"id"`
				DisplayName    string `json:"displayName"`
				Kind           string `json:"kind"`
				Status         string `json:"status"`
				DefaultBaseURL string `json:"defaultBaseURL"`
			} `json:"providers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode provider catalog: %v", err)
	}

	providers := map[string]struct {
		DisplayName    string
		Kind           string
		Status         string
		DefaultBaseURL string
	}{}
	for _, provider := range response.Data.Providers {
		providers[provider.ID] = struct {
			DisplayName    string
			Kind           string
			Status         string
			DefaultBaseURL string
		}{
			DisplayName:    provider.DisplayName,
			Kind:           provider.Kind,
			Status:         provider.Status,
			DefaultBaseURL: provider.DefaultBaseURL,
		}
	}

	for _, id := range []string{"openai", "claude", "gemini", "deepseek", "openrouter", "ollama", "vertex", "bedrock"} {
		provider, ok := providers[id]
		if !ok {
			t.Fatalf("provider %q missing from admin catalog: %+v", id, providers)
		}
		if provider.DisplayName == "" || provider.Kind == "" || provider.Status != "supported" {
			t.Fatalf("provider %q has incomplete catalog payload: %+v", id, provider)
		}
	}
	if providers["openai"].DefaultBaseURL != "https://api.openai.com" {
		t.Fatalf("openai default base url = %q", providers["openai"].DefaultBaseURL)
	}
	if providers["openrouter"].DefaultBaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("openrouter default base url = %q", providers["openrouter"].DefaultBaseURL)
	}
	if len(response.Data.Providers) < 100 {
		t.Fatalf("provider catalog has %d entries, want at least 100", len(response.Data.Providers))
	}
	for id, wantStatus := range map[string]string{
		"groq":         "supported",
		"together":     "supported",
		"siliconflow":  "supported",
		"azure-openai": "planned",
		"perplexity":   "planned",
	} {
		provider, ok := providers[id]
		if !ok {
			t.Fatalf("provider %q missing from admin catalog", id)
		}
		if provider.Status != wantStatus {
			t.Fatalf("provider %q status = %q, want %q", id, provider.Status, wantStatus)
		}
	}
}
