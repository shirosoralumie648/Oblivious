package channel

import (
	"testing"

	"oblivious/server/internal/relay/types"
)

func TestProviderCatalogIncludesGatewayProviders(t *testing.T) {
	for _, provider := range []string{"openai", "claude", "gemini", "deepseek", "openrouter", "ollama", "vertex", "bedrock"} {
		spec, ok := LookupProvider(provider)
		if !ok {
			t.Fatalf("provider %q missing from catalog", provider)
		}
		if spec.ID == "" || spec.DisplayName == "" || spec.Kind == "" {
			t.Fatalf("provider %q has incomplete catalog spec: %+v", provider, spec)
		}
	}
}

func TestProviderCatalogCoversHundredPlusCommercialProviders(t *testing.T) {
	providers := SupportedProviders()
	if len(providers) < 100 {
		t.Fatalf("provider catalog has %d providers, want at least 100", len(providers))
	}

	for _, provider := range []string{
		"openai",
		"anthropic",
		"gemini",
		"deepseek",
		"openrouter",
		"groq",
		"together",
		"fireworks",
		"mistral",
		"siliconflow",
		"moonshot",
		"zhipu",
		"qwen",
		"minimax",
	} {
		spec, ok := LookupProvider(provider)
		if !ok {
			t.Fatalf("provider %q missing from catalog", provider)
		}
		if spec.Status != ProviderStatusSupported {
			t.Fatalf("provider %q status = %q, want supported", provider, spec.Status)
		}
		if spec.Kind == ProviderKindOpenAICompatible && spec.DefaultBaseURL == "" {
			t.Fatalf("provider %q is OpenAI-compatible but has empty default base URL", provider)
		}
	}

	for _, provider := range []string{"azure-openai", "perplexity", "cohere", "ai21", "replicate"} {
		spec, ok := LookupProvider(provider)
		if !ok {
			t.Fatalf("provider %q missing from catalog", provider)
		}
		if spec.Status != ProviderStatusPlanned {
			t.Fatalf("provider %q status = %q, want planned until a native adapter is wired", provider, spec.Status)
		}
	}
}

func TestAdapterForChannelSupportsExpandedOpenAICompatibleCatalog(t *testing.T) {
	tests := map[string]string{
		"groq":        "https://api.groq.com/openai/v1/chat/completions",
		"together":    "https://api.together.xyz/v1/chat/completions",
		"fireworks":   "https://api.fireworks.ai/inference/v1/chat/completions",
		"mistral":     "https://api.mistral.ai/v1/chat/completions",
		"siliconflow": "https://api.siliconflow.cn/v1/chat/completions",
		"moonshot":    "https://api.moonshot.cn/v1/chat/completions",
		"zhipu":       "https://open.bigmodel.cn/api/paas/v4/chat/completions",
		"qwen":        "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
		"minimax":     "https://api.minimax.io/v1/chat/completions",
	}
	for provider, wantURL := range tests {
		t.Run(provider, func(t *testing.T) {
			adapter, err := AdapterForChannel(&types.Channel{
				ID:       "ch_" + provider,
				Provider: provider,
				APIKey:   "sk-" + provider,
			})
			if err != nil {
				t.Fatalf("AdapterForChannel returned error: %v", err)
			}
			if adapter.Provider() != provider {
				t.Fatalf("adapter provider = %q, want %q", adapter.Provider(), provider)
			}
			url, err := adapter.BuildURL("gpt-4o-mini", types.APITypeChat)
			if err != nil {
				t.Fatalf("BuildURL returned error: %v", err)
			}
			if url != wantURL {
				t.Fatalf("url = %q, want %q", url, wantURL)
			}
		})
	}
}

func TestAdapterForChannelRejectsPlannedCatalogProviderWithExplicitBaseURL(t *testing.T) {
	_, err := AdapterForChannel(&types.Channel{
		ID:       "ch_perplexity",
		Provider: "perplexity",
		BaseURL:  "https://api.perplexity.ai",
		APIKey:   "pplx-key",
	})
	if err == nil {
		t.Fatal("planned catalog providers must fail closed even with explicit base URL")
	}
}

func TestAdapterForChannelRejectsCatalogProviderWithoutCallableBaseURL(t *testing.T) {
	_, err := AdapterForChannel(&types.Channel{
		ID:       "ch_perplexity",
		Provider: "perplexity",
		APIKey:   "pplx-key",
	})
	if err == nil {
		t.Fatal("planned catalog provider without explicit base URL should fail closed")
	}
}

func TestAdapterForChannelSupportsOpenAICompatibleProviderFamily(t *testing.T) {
	for _, provider := range []string{"openai", "deepseek", "openrouter", "ollama", "claude"} {
		t.Run(provider, func(t *testing.T) {
			adapter, err := AdapterForChannel(&types.Channel{
				ID:       "ch_" + provider,
				Provider: provider,
				BaseURL:  "https://" + provider + ".example.com",
				APIKey:   "sk-" + provider,
			})
			if err != nil {
				t.Fatalf("AdapterForChannel returned error: %v", err)
			}
			if adapter.Provider() != provider {
				t.Fatalf("adapter provider = %q, want %q", adapter.Provider(), provider)
			}
			url, err := adapter.BuildURL("gpt-4o-mini", types.APITypeChat)
			if err != nil {
				t.Fatalf("BuildURL returned error: %v", err)
			}
			wantURL := "https://" + provider + ".example.com/v1/chat/completions"
			if provider == "claude" {
				wantURL = "https://" + provider + ".example.com/v1/messages"
			}
			if url != wantURL {
				t.Fatalf("url = %q", url)
			}
		})
	}
}

func TestAdapterForChannelSupportsGeminiProvider(t *testing.T) {
	adapter, err := AdapterForChannel(&types.Channel{
		ID:       "ch_gemini",
		Provider: "gemini",
		BaseURL:  "https://generativelanguage.googleapis.com",
		APIKey:   "sk-gemini",
	})
	if err != nil {
		t.Fatalf("AdapterForChannel returned error: %v", err)
	}
	if adapter.Provider() != "gemini" {
		t.Fatalf("adapter provider = %q, want gemini", adapter.Provider())
	}
	url, err := adapter.BuildURL("gemini-1.5-flash", types.APITypeChat)
	if err != nil {
		t.Fatalf("BuildURL returned error: %v", err)
	}
	if url != "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent" {
		t.Fatalf("url = %q", url)
	}
}

func TestAdapterForChannelSupportsBedrockProvider(t *testing.T) {
	adapter, err := AdapterForChannel(&types.Channel{
		ID:       "ch_bedrock",
		Provider: "bedrock",
		BaseURL:  "https://bedrock-runtime.us-east-1.amazonaws.com",
		APIKey:   "bedrock-key|us-east-1",
	})
	if err != nil {
		t.Fatalf("AdapterForChannel returned error: %v", err)
	}
	if adapter.Provider() != "bedrock" {
		t.Fatalf("adapter provider = %q, want bedrock", adapter.Provider())
	}
	url, err := adapter.BuildURL("claude-3-5-sonnet-20241022", types.APITypeChat)
	if err != nil {
		t.Fatalf("BuildURL returned error: %v", err)
	}
	if url != "https://bedrock-runtime.us-east-1.amazonaws.com/model/anthropic.claude-3-5-sonnet-20241022-v2:0/converse" {
		t.Fatalf("url = %q", url)
	}
}

func TestAdapterForChannelSupportsVertexProvider(t *testing.T) {
	adapter, err := AdapterForChannel(&types.Channel{
		ID:       "ch_vertex",
		Provider: "vertex",
		BaseURL:  "https://us-central1-aiplatform.googleapis.com",
		APIKey:   "vertex-key|demo-project|us-central1",
	})
	if err != nil {
		t.Fatalf("AdapterForChannel returned error: %v", err)
	}
	if adapter.Provider() != "vertex" {
		t.Fatalf("adapter provider = %q, want vertex", adapter.Provider())
	}
	url, err := adapter.BuildURL("gemini-1.5-pro", types.APITypeChat)
	if err != nil {
		t.Fatalf("BuildURL returned error: %v", err)
	}
	if url != "https://us-central1-aiplatform.googleapis.com/v1/projects/demo-project/locations/us-central1/publishers/google/models/gemini-1.5-pro:generateContent?key=vertex-key" {
		t.Fatalf("url = %q", url)
	}
}
