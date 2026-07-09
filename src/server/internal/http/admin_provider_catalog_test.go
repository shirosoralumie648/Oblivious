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
				Configurable   bool   `json:"configurable"`
				Installable    bool   `json:"installable"`
				RuntimeReady   bool   `json:"runtimeReady"`
			} `json:"providers"`
			SupportedProviders []struct {
				ID           string `json:"id"`
				Status       string `json:"status"`
				Configurable bool   `json:"configurable"`
				Installable  bool   `json:"installable"`
				RuntimeReady bool   `json:"runtimeReady"`
			} `json:"supportedProviders"`
			PlannedProviders []struct {
				ID           string `json:"id"`
				Status       string `json:"status"`
				Configurable bool   `json:"configurable"`
				Installable  bool   `json:"installable"`
				RuntimeReady bool   `json:"runtimeReady"`
			} `json:"plannedProviders"`
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
		Configurable   bool
		Installable    bool
		RuntimeReady   bool
	}{}
	for _, provider := range response.Data.Providers {
		providers[provider.ID] = struct {
			DisplayName    string
			Kind           string
			Status         string
			DefaultBaseURL string
			Configurable   bool
			Installable    bool
			RuntimeReady   bool
		}{
			DisplayName:    provider.DisplayName,
			Kind:           provider.Kind,
			Status:         provider.Status,
			DefaultBaseURL: provider.DefaultBaseURL,
			Configurable:   provider.Configurable,
			Installable:    provider.Installable,
			RuntimeReady:   provider.RuntimeReady,
		}
	}

	for _, id := range []string{"openai", "claude", "gemini", "deepseek", "openrouter", "ollama", "vertex", "bedrock"} {
		provider, ok := providers[id]
		if !ok {
			t.Fatalf("provider %q missing from admin catalog: %+v", id, providers)
		}
		if provider.DisplayName == "" || provider.Kind == "" || provider.Status != "supported" ||
			!provider.Configurable || !provider.Installable || !provider.RuntimeReady {
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
		wantConfigurable := wantStatus == "supported"
		if provider.Configurable != wantConfigurable || provider.Installable != wantConfigurable || provider.RuntimeReady != wantConfigurable {
			t.Fatalf("provider %q release readiness flags = configurable:%v installable:%v runtimeReady:%v, want %v",
				id,
				provider.Configurable,
				provider.Installable,
				provider.RuntimeReady,
				wantConfigurable,
			)
		}
	}
	supportedProviders := map[string]struct{}{}
	for _, provider := range response.Data.SupportedProviders {
		if provider.Status != "supported" || !provider.Configurable || !provider.Installable || !provider.RuntimeReady {
			t.Fatalf("supported provider group contains non-installable provider: %+v", provider)
		}
		supportedProviders[provider.ID] = struct{}{}
	}
	plannedProviders := map[string]struct{}{}
	for _, provider := range response.Data.PlannedProviders {
		if provider.Status != "planned" || provider.Configurable || provider.Installable || provider.RuntimeReady {
			t.Fatalf("planned provider group contains installable provider: %+v", provider)
		}
		plannedProviders[provider.ID] = struct{}{}
	}
	for _, id := range []string{"openai", "deepseek", "claude", "gemini"} {
		if _, ok := supportedProviders[id]; !ok {
			t.Fatalf("supported provider group missing %q", id)
		}
		if _, ok := plannedProviders[id]; ok {
			t.Fatalf("planned provider group must not include supported provider %q", id)
		}
	}
	for _, id := range []string{"azure-openai", "perplexity", "cohere"} {
		if _, ok := plannedProviders[id]; !ok {
			t.Fatalf("planned provider group missing %q", id)
		}
		if _, ok := supportedProviders[id]; ok {
			t.Fatalf("supported provider group must not include planned provider %q", id)
		}
	}
}
