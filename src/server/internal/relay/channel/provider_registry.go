package channel

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"oblivious/server/internal/relay/types"
)

type ProviderKind string

const (
	ProviderKindOpenAICompatible ProviderKind = "openai_compatible"
	ProviderKindNative           ProviderKind = "native"
)

type ProviderStatus string

const (
	ProviderStatusSupported ProviderStatus = "supported"
	ProviderStatusPlanned   ProviderStatus = "planned"
)

type ProviderSpec struct {
	ID             string
	DisplayName    string
	Kind           ProviderKind
	Status         ProviderStatus
	DefaultBaseURL string
}

var (
	ErrProviderUnknown     = errors.New("provider unknown")
	ErrProviderUnsupported = errors.New("provider unsupported")
)

var providerCatalog = buildProviderCatalog()

func buildProviderCatalog() map[string]ProviderSpec {
	catalog := make(map[string]ProviderSpec, len(supportedProviderSpecs)+len(plannedProviderNames))
	for _, spec := range supportedProviderSpecs {
		catalog[spec.ID] = spec
	}
	for id, displayName := range plannedProviderNames {
		if _, exists := catalog[id]; exists {
			continue
		}
		catalog[id] = ProviderSpec{
			ID:          id,
			DisplayName: displayName,
			Kind:        ProviderKindOpenAICompatible,
			Status:      ProviderStatusPlanned,
		}
	}
	return catalog
}

var supportedProviderSpecs = []ProviderSpec{
	openAICompatibleProvider("openai", "OpenAI", "https://api.openai.com"),
	openAICompatibleProvider("deepseek", "DeepSeek", "https://api.deepseek.com"),
	openAICompatibleProvider("openrouter", "OpenRouter", "https://openrouter.ai/api/v1"),
	openAICompatibleProvider("ollama", "Ollama", "http://localhost:11434/v1"),
	openAICompatibleProvider("groq", "Groq", "https://api.groq.com/openai/v1"),
	openAICompatibleProvider("together", "Together AI", "https://api.together.xyz/v1"),
	openAICompatibleProvider("fireworks", "Fireworks AI", "https://api.fireworks.ai/inference/v1"),
	openAICompatibleProvider("mistral", "Mistral AI", "https://api.mistral.ai/v1"),
	openAICompatibleProvider("siliconflow", "SiliconFlow", "https://api.siliconflow.cn/v1"),
	openAICompatibleProvider("moonshot", "Moonshot AI", "https://api.moonshot.cn/v1"),
	openAICompatibleProvider("zhipu", "Zhipu AI", "https://open.bigmodel.cn/api/paas/v4"),
	openAICompatibleProvider("qwen", "Alibaba Cloud DashScope", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
	openAICompatibleProvider("minimax", "MiniMax", "https://api.minimax.io/v1"),
	{
		ID:             "claude",
		DisplayName:    "Claude",
		Kind:           ProviderKindNative,
		Status:         ProviderStatusSupported,
		DefaultBaseURL: "https://api.anthropic.com",
	},
	{
		ID:             "gemini",
		DisplayName:    "Gemini",
		Kind:           ProviderKindNative,
		Status:         ProviderStatusSupported,
		DefaultBaseURL: "https://generativelanguage.googleapis.com",
	},
	{
		ID:             "vertex",
		DisplayName:    "Vertex AI",
		Kind:           ProviderKindNative,
		Status:         ProviderStatusSupported,
		DefaultBaseURL: "",
	},
	{
		ID:             "bedrock",
		DisplayName:    "Amazon Bedrock",
		Kind:           ProviderKindNative,
		Status:         ProviderStatusSupported,
		DefaultBaseURL: "",
	},
}

func openAICompatibleProvider(id, displayName, defaultBaseURL string) ProviderSpec {
	return ProviderSpec{
		ID:             id,
		DisplayName:    displayName,
		Kind:           ProviderKindOpenAICompatible,
		Status:         ProviderStatusSupported,
		DefaultBaseURL: defaultBaseURL,
	}
}

var plannedProviderNames = map[string]string{
	"a2a":                       "A2A",
	"abliteration":              "Abliteration",
	"aiml":                      "AI/ML API",
	"ai21":                      "AI21",
	"ai21-chat":                 "AI21 Chat",
	"amazon-nova":               "Amazon Nova",
	"anthropic-text":            "Anthropic Text",
	"apertis":                   "Apertis",
	"aihubmix":                  "AIHubMix",
	"assemblyai":                "AssemblyAI",
	"auto-router":               "Auto Router",
	"sagemaker":                 "Amazon SageMaker",
	"azure-openai":              "Azure OpenAI",
	"azure-ai":                  "Azure AI",
	"azure-ai-agents":           "Azure AI Foundry Agents",
	"azure-text":                "Azure Text",
	"baseten":                   "Baseten",
	"bytez":                     "Bytez",
	"cerebras":                  "Cerebras",
	"charity-engine":            "Charity Engine",
	"chutes":                    "Chutes",
	"clarifai":                  "Clarifai",
	"cloudflare":                "Cloudflare AI Workers",
	"codestral":                 "Codestral",
	"cohere":                    "Cohere",
	"cohere-chat":               "Cohere Chat",
	"cometapi":                  "CometAPI",
	"compactifai":               "CompactifAI",
	"crusoe":                    "Crusoe",
	"custom":                    "Custom Provider",
	"custom-openai":             "Custom OpenAI",
	"databricks":                "Databricks",
	"datarobot":                 "DataRobot",
	"deepgram":                  "Deepgram",
	"deepinfra":                 "DeepInfra",
	"elevenlabs":                "ElevenLabs",
	"empower":                   "Empower",
	"fal-ai":                    "Fal AI",
	"featherless-ai":            "Featherless AI",
	"friendliai":                "FriendliAI",
	"galadriel":                 "Galadriel",
	"github-copilot":            "GitHub Copilot",
	"chatgpt":                   "ChatGPT Subscription",
	"github":                    "GitHub Models",
	"gmi":                       "GMI Cloud",
	"gradient-ai":               "Gradient AI",
	"heroku":                    "Heroku",
	"hosted-vllm":               "Hosted vLLM",
	"huggingface":               "Hugging Face",
	"hyperbolic":                "Hyperbolic",
	"watsonx":                   "IBM watsonx.ai",
	"lambda-ai":                 "Lambda AI",
	"lemonade":                  "Lemonade",
	"litellm-proxy":             "LiteLLM Proxy",
	"llamafile":                 "Llamafile",
	"lm-studio":                 "LM Studio",
	"maritalk":                  "Maritalk",
	"meta-llama":                "Meta Llama API",
	"docker-model-runner":       "Docker Model Runner",
	"morph":                     "Morph",
	"nanogpt":                   "NanoGPT",
	"nebius":                    "Nebius AI Studio",
	"nlp-cloud":                 "NLP Cloud",
	"novita":                    "Novita AI",
	"nscale":                    "Nscale",
	"nvidia-nim":                "NVIDIA NIM",
	"oci":                       "Oracle Cloud Infrastructure",
	"ollama-chat":               "Ollama Chat",
	"oobabooga":                 "Oobabooga",
	"openai-like":               "OpenAI-like",
	"ovhcloud":                  "OVHcloud AI Endpoints",
	"perplexity":                "Perplexity AI",
	"petals":                    "Petals",
	"poe":                       "Poe",
	"publicai":                  "PublicAI",
	"predibase":                 "Predibase",
	"replicate":                 "Replicate",
	"sagemaker-chat":            "SageMaker Chat",
	"sambanova":                 "SambaNova",
	"sap":                       "SAP Generative AI Hub",
	"scaleway":                  "Scaleway",
	"snowflake":                 "Snowflake",
	"synthetic":                 "Synthetic",
	"text-completion-codestral": "Text Completion Codestral",
	"text-completion-openai":    "Text Completion OpenAI",
	"triton":                    "Triton",
	"v0":                        "V0",
	"vercel-ai-gateway":         "Vercel AI Gateway",
	"vllm":                      "vLLM",
	"volcengine":                "Volcengine",
	"wandb":                     "Weights & Biases Inference",
	"watsonx-text":              "watsonx Text",
	"xai":                       "xAI",
	"ragflow":                   "RAGFlow",
	"cursor":                    "Cursor BYOK",
	"langgraph":                 "LangGraph",
	"vertex-ai-agent-engine":    "Vertex AI Agent Engine",
	"venice":                    "Venice.ai",
	"gigachat":                  "GigaChat",
	"helicone":                  "Helicone",
	"llamagate":                 "LlamaGate",
	"xiaomi-mimo":               "Xiaomi MiMo",
	"manus":                     "Manus",
	"sarvam":                    "Sarvam",
	"bedrock-mantle":            "Bedrock Mantle",
	"infinity":                  "Infinity",
	"jina-ai":                   "Jina AI",
	"voyage":                    "Voyage AI",
	"xinference":                "Xinference",
	"stability":                 "Stability AI",
	"recraft":                   "Recraft",
	"runwayml":                  "RunwayML",
	"black-forest-labs":         "Black Forest Labs",
}

var providerAliases = map[string]string{
	"alibaba":          "qwen",
	"alibaba-cloud":    "qwen",
	"amazon-bedrock":   "bedrock",
	"anthropic":        "claude",
	"aws-bedrock":      "bedrock",
	"bigmodel":         "zhipu",
	"bigmodel-cn":      "zhipu",
	"dashscope":        "qwen",
	"fireworks-ai":     "fireworks",
	"google":           "gemini",
	"google-ai":        "gemini",
	"google-ai-studio": "gemini",
	"google-gemini":    "gemini",
	"google-vertex":    "vertex",
	"google-vertex-ai": "vertex",
	"groq-ai":          "groq",
	"minimax-ai":       "minimax",
	"mistral-ai":       "mistral",
	"moonshot-ai":      "moonshot",
	"open-router":      "openrouter",
	"silicon-flow":     "siliconflow",
	"together-ai":      "together",
	"vertex-ai":        "vertex",
	"z-ai":             "zhipu",
	"zai":              "zhipu",
}

func NormalizeProvider(provider string) string {
	normalized := strings.ToLower(strings.TrimSpace(provider))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	if alias, ok := providerAliases[normalized]; ok {
		return alias
	}
	return normalized
}

func LookupProvider(provider string) (ProviderSpec, bool) {
	spec, ok := providerCatalog[NormalizeProvider(provider)]
	return spec, ok
}

func SupportedProviders() []ProviderSpec {
	providers := make([]ProviderSpec, 0, len(providerCatalog))
	for _, spec := range providerCatalog {
		providers = append(providers, spec)
	}
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].ID < providers[j].ID
	})
	return providers
}

func AdapterForChannel(ch *types.Channel) (types.ProviderAdapter, error) {
	if ch == nil {
		return nil, fmt.Errorf("%w: missing channel", ErrProviderUnknown)
	}
	spec, ok := LookupProvider(ch.Provider)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderUnknown, ch.Provider)
	}

	explicitBaseURL := strings.TrimSpace(ch.BaseURL)
	baseURL := explicitBaseURL
	if baseURL == "" {
		baseURL = spec.DefaultBaseURL
		if spec.Status != ProviderStatusSupported {
			return nil, fmt.Errorf("%w: %s adapter is %s", ErrProviderUnsupported, spec.ID, spec.Status)
		}
	}
	switch spec.Kind {
	case ProviderKindOpenAICompatible:
		if spec.Status != ProviderStatusSupported && explicitBaseURL == "" {
			return nil, fmt.Errorf("%w: %s adapter is %s", ErrProviderUnsupported, spec.ID, spec.Status)
		}
		return NewOpenAICompatibleAdapter(spec.ID, baseURL, ch.APIKey), nil
	case ProviderKindNative:
		if spec.Status != ProviderStatusSupported {
			return nil, fmt.Errorf("%w: %s adapter is %s", ErrProviderUnsupported, spec.ID, spec.Status)
		}
		if spec.ID == "claude" {
			return NewClaudeAdapter(baseURL, ch.APIKey), nil
		}
		if spec.ID == "gemini" {
			return NewGeminiAdapter(baseURL, ch.APIKey), nil
		}
		if spec.ID == "vertex" {
			return NewVertexAdapter(baseURL, ch.APIKey), nil
		}
		if spec.ID == "bedrock" {
			return NewBedrockAdapter(baseURL, ch.APIKey), nil
		}
		return nil, fmt.Errorf("%w: %s adapter is %s", ErrProviderUnsupported, spec.ID, spec.Status)
	default:
		return nil, fmt.Errorf("%w: %s adapter kind %s", ErrProviderUnsupported, spec.ID, spec.Kind)
	}
}
