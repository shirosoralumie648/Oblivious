package config

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"oblivious/server/internal/releasecontract"
	pkgconfig "oblivious/server/pkg/config"
)

type ContractLoader = pkgconfig.ContractLoader
type FileContractLoader = pkgconfig.FileContractLoader
type EntrypointPreflightOptions = pkgconfig.EntrypointPreflightOptions
type ResolvedEntrypointInputs = pkgconfig.ResolvedEntrypointInputs
type EntrypointContinuation = pkgconfig.EntrypointContinuation

func RunEntrypoint(ctx context.Context, id releasecontract.EntrypointID, options EntrypointPreflightOptions, normalStartup EntrypointContinuation) error {
	return pkgconfig.RunEntrypoint(ctx, id, options, normalStartup)
}

type Config struct {
	Port                int
	Env                 string
	CORSAllowedOrigins  []string
	DatabaseURL         string
	SessionCookieName   string
	SessionCookieSecure bool
	SessionSecret       string
	SecretEncryptionKey string
	LLMBaseURL          string
	LLMAPIKey           string
	LLMTimeoutMS        int
	ModelDefaultName    string

	// AgentRelayBaseURL points the standalone Agent runtime at the Relay/OpenAI-compatible API.
	// It defaults in the runtime wiring when unset so monolith tests keep their historical env.
	AgentRelayBaseURL string

	// ChatRelayBaseURL points standalone Chat runtimes at the Relay/OpenAI-compatible API.
	// It defaults in the runtime wiring when unset so monolith deployments keep using local Relay.
	ChatRelayBaseURL string

	// Agent web search configuration. Provider stays disabled unless a supported
	// provider, endpoint, and API key are all configured.
	AgentWebSearchProvider    string
	AgentWebSearchFallback    []string
	AgentWebSearchEndpoint    string
	AgentWebSearchAPIKey      string
	AgentWebSearchResultLimit int
	// AgentWebSearchGoogleCSEID is the Programmable Search Engine ID required
	// by the google_cse web search provider.
	AgentWebSearchGoogleCSEID string

	// Qdrant vector database configuration. Required in production so
	// Knowledge/RAG routes cannot silently fall back to non-durable retrieval.
	QdrantURL        string
	QdrantAPIKey     string
	QdrantVectorSize int

	// RAG reranker configuration. Disabled unless RAG_RERANKER_BASE_URL is set.
	RAGRerankerBaseURL string
	RAGRerankerAPIKey  string
	RAGRerankerModel   string
	RAGRerankerTopK    int

	// Durable RAG index worker configuration. The worker replays failed vector
	// indexing jobs when Qdrant-backed Knowledge indexing is enabled.
	RAGIndexWorkerEnabled    bool
	RAGIndexWorkerIntervalMS int
	RAGIndexWorkerClaimLimit int
	// Durable RAG ingestion worker configuration. The worker consumes
	// persisted upload/content ingestion jobs and creates Knowledge documents.
	RAGIngestionWorkerEnabled    bool
	RAGIngestionWorkerIntervalMS int
	RAGIngestionWorkerClaimLimit int

	// Relay configuration
	RelayEnabled                            bool
	RelayDefaultModel                       string
	RelayRateLimitBackend                   string
	RelayRateLimitRedisKeyPrefix            string
	RelaySemanticCacheBackend               string
	RelayPricingMaintenanceEnabled          bool
	RelayPricingMaintenanceIntervalMS       int
	RelayPricingMaintenanceProvider         string
	RelayPricingMaintenanceSource           string
	RelayPricingMaintenanceSourceURL        string
	RelayPricingMaintenanceModels           []string
	RelayPricingMaintenanceMaxBytes         int64
	RelayPricingReconciliationLimit         int
	RelayBatchPollingWorkerEnabled          bool
	RelayBatchPollingWorkerIntervalMS       int
	RelayBatchPollingWorkerClaimLimit       int
	RelayBatchCommercialLifecycleEnabled    bool
	RelayRealtimeCommercialLifecycleEnabled bool
	RedisAddr                               string
	RedisPassword                           string
	RedisDB                                 int

	// Default channel configuration (for development)
	OpenAIAPIKey  string
	OpenAIBaseURL string

	// Stripe billing configuration
	StripeSecretKey     string
	StripeSuccessURL    string
	StripeCancelURL     string
	StripeWebhookSecret string

	// Domestic payment checkout configuration. A provider stays unavailable
	// unless its hosted checkout base URL is configured.
	AlipayCheckoutBaseURL    string
	AlipayWebhookSecret      string
	WeChatPayCheckoutBaseURL string
	WeChatPayWebhookSecret   string

	// Marketplace payout dispatch configuration. Local payout dispatch is
	// only acceptable outside production.
	MarketplacePayoutProvider      string
	MarketplacePayoutWebhookURL    string
	MarketplacePayoutWebhookSecret string

	// Scheduled task worker configuration
	ScheduleWorkerEnabled    bool
	ScheduleWorkerIntervalMS int
	ScheduleWorkerClaimLimit int

	// Workflow system-level guardrails. Zero keeps the guard disabled.
	WorkflowSystemMaxConcurrent          int
	WorkflowGlobalMaxExecutionsPerMinute int
	WorkflowRelayBaseURL                 string

	// Workflow code-interpreter sandbox configuration. The docker-backed
	// sandbox stays disabled unless WORKFLOW_SANDBOX_ENABLED=true.
	WorkflowSandboxEnabled          bool
	WorkflowSandboxAllowedLanguages string
	WorkflowSandboxMemoryMB         int
	WorkflowSandboxCPUs             int
	WorkflowSandboxDefaultTimeoutMS int
	WorkflowSandboxMaxTimeoutMS     int

	// Channel message log archive configuration. Disabled unless a file root or
	// S3-compatible object storage target is configured.
	ChannelMessageLogArchiveEnabled     bool
	ChannelMessageLogArchiveBackend     string
	ChannelMessageLogArchiveRoot        string
	ChannelMessageLogArchiveS3Endpoint  string
	ChannelMessageLogArchiveS3Region    string
	ChannelMessageLogArchiveS3Bucket    string
	ChannelMessageLogArchiveS3AccessKey string
	ChannelMessageLogArchiveS3SecretKey string
	ChannelMessageLogArchiveIntervalMS  int
	ChannelMessageLogRetentionHours     int
	ChannelMessageLogArchiveLimit       int

	// Observability request log configuration. ClickHouse is disabled unless
	// explicitly selected because local/dev environments usually do not run it.
	ObservabilityRequestLogBackend         string
	ClickHouseDSN                          string
	ClickHouseDriver                       string
	ObservabilityHTTPAlertsEnabled         bool
	AlertWebhookURL                        string
	AlertWebhookSecret                     string
	ObservabilityHTTPLatencySLOThresholdMS int

	// HTTP recovery audit records planned remediation actions for alert review.
	// It does not execute infrastructure mutations.
	ObservabilityHTTPRecoveryEnabled    bool
	ObservabilityHTTPRecoveryCooldownMS int

	// Database mode: "monolith" (default) | "dual_write" | "microservices"
	DBMode string

	// Microservices database URLs (only used in dual_write/microservices mode)
	DBURLRelay         string
	DBURLChat          string
	DBURLWorkflow      string
	DBURLRAG           string
	DBURLAgent         string
	DBURLBilling       string
	DBURLMarketplace   string
	DBURLAdmin         string
	DBURLChannel       string
	DBURLTask          string
	DBURLObservability string
}

func Load() (Config, error) {
	portRaw := strings.TrimSpace(os.Getenv("SERVER_PORT"))
	if portRaw == "" {
		portRaw = "8080"
	}

	port, err := strconv.Atoi(portRaw)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("invalid SERVER_PORT: %q", portRaw)
	}

	env := strings.TrimSpace(os.Getenv("APP_ENV"))
	if env == "" {
		env = "development"
	}

	originsRaw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	origins := []string{}
	if originsRaw != "" {
		for _, part := range strings.Split(originsRaw, ",") {
			value := strings.TrimSpace(part)
			if value != "" {
				origins = append(origins, value)
			}
		}
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	sessionSecret := strings.TrimSpace(os.Getenv("SESSION_SECRET"))
	if sessionSecret == "" {
		return Config{}, fmt.Errorf("SESSION_SECRET is required")
	}
	secretEncryptionKey := strings.TrimSpace(os.Getenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY"))

	sessionCookieName := strings.TrimSpace(os.Getenv("SESSION_COOKIE_NAME"))
	if sessionCookieName == "" {
		sessionCookieName = "oblivious_session"
	}

	sessionCookieSecure := strings.EqualFold(strings.TrimSpace(os.Getenv("SESSION_COOKIE_SECURE")), "true")
	if err := validateProductionSessionConfig(env, sessionSecret, sessionCookieSecure); err != nil {
		return Config{}, err
	}
	if err := validateProductionSecretEncryptionConfig(env, sessionSecret, secretEncryptionKey); err != nil {
		return Config{}, err
	}
	llmBaseURL := strings.TrimSpace(os.Getenv("LLM_BASE_URL"))
	llmAPIKey := strings.TrimSpace(os.Getenv("LLM_API_KEY"))
	llmTimeoutMS := 30000
	llmTimeoutRaw := strings.TrimSpace(os.Getenv("LLM_TIMEOUT_MS"))
	if llmTimeoutRaw != "" {
		parsedTimeout, parseErr := strconv.Atoi(llmTimeoutRaw)
		if parseErr != nil || parsedTimeout < 1 {
			return Config{}, fmt.Errorf("invalid LLM_TIMEOUT_MS: %q", llmTimeoutRaw)
		}
		llmTimeoutMS = parsedTimeout
	}
	modelDefaultName := strings.TrimSpace(os.Getenv("MODEL_DEFAULT_NAME"))
	if modelDefaultName == "" {
		modelDefaultName = "demo-reply"
	}
	chatRelayBaseURL := strings.TrimSpace(os.Getenv("CHAT_RELAY_BASE_URL"))
	agentRelayBaseURL := strings.TrimSpace(os.Getenv("AGENT_RELAY_BASE_URL"))
	agentWebSearchProvider := strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_WEB_SEARCH_PROVIDER")))
	agentWebSearchFallbackRaw := strings.TrimSpace(os.Getenv("AGENT_WEB_SEARCH_FALLBACK"))
	var agentWebSearchFallback []string
	if agentWebSearchFallbackRaw != "" {
		for _, part := range strings.Split(agentWebSearchFallbackRaw, ",") {
			if value := strings.TrimSpace(part); value != "" {
				agentWebSearchFallback = append(agentWebSearchFallback, value)
			}
		}
	}
	agentWebSearchEndpoint := strings.TrimSpace(os.Getenv("AGENT_WEB_SEARCH_ENDPOINT"))
	agentWebSearchAPIKey := strings.TrimSpace(os.Getenv("AGENT_WEB_SEARCH_API_KEY"))
	agentWebSearchGoogleCSEID := strings.TrimSpace(os.Getenv("AGENT_WEB_SEARCH_GOOGLE_CSE_ID"))
	agentWebSearchResultLimit := 5
	agentWebSearchResultLimitRaw := strings.TrimSpace(os.Getenv("AGENT_WEB_SEARCH_RESULT_LIMIT"))
	if agentWebSearchResultLimitRaw != "" {
		parsedLimit, parseErr := strconv.Atoi(agentWebSearchResultLimitRaw)
		if parseErr != nil || parsedLimit < 1 {
			return Config{}, fmt.Errorf("invalid AGENT_WEB_SEARCH_RESULT_LIMIT: %q", agentWebSearchResultLimitRaw)
		}
		agentWebSearchResultLimit = parsedLimit
	}
	qdrantURL := strings.TrimSpace(os.Getenv("QDRANT_URL"))
	qdrantAPIKey := strings.TrimSpace(os.Getenv("QDRANT_API_KEY"))
	qdrantVectorSize := 1536
	qdrantVectorSizeRaw := strings.TrimSpace(os.Getenv("QDRANT_VECTOR_SIZE"))
	if qdrantVectorSizeRaw != "" {
		parsedSize, parseErr := strconv.Atoi(qdrantVectorSizeRaw)
		if parseErr != nil || parsedSize < 1 {
			return Config{}, fmt.Errorf("invalid QDRANT_VECTOR_SIZE: %q", qdrantVectorSizeRaw)
		}
		qdrantVectorSize = parsedSize
	}
	if strings.EqualFold(env, "production") && qdrantURL == "" {
		return Config{}, fmt.Errorf("QDRANT_URL is required when APP_ENV=production for Knowledge/RAG routes")
	}
	ragRerankerBaseURL := strings.TrimSpace(os.Getenv("RAG_RERANKER_BASE_URL"))
	ragRerankerAPIKey := strings.TrimSpace(os.Getenv("RAG_RERANKER_API_KEY"))
	ragRerankerModel := strings.TrimSpace(os.Getenv("RAG_RERANKER_MODEL"))
	if ragRerankerModel == "" {
		ragRerankerModel = "bge-reranker-large"
	}
	ragRerankerTopK := 5
	ragRerankerTopKRaw := strings.TrimSpace(os.Getenv("RAG_RERANKER_TOP_K"))
	if ragRerankerTopKRaw != "" {
		parsedTopK, parseErr := strconv.Atoi(ragRerankerTopKRaw)
		if parseErr != nil || parsedTopK < 1 {
			return Config{}, fmt.Errorf("invalid RAG_RERANKER_TOP_K: %q", ragRerankerTopKRaw)
		}
		ragRerankerTopK = parsedTopK
	}
	ragIndexWorkerEnabled := !strings.EqualFold(env, "test") && qdrantURL != ""
	if raw := strings.TrimSpace(os.Getenv("RAG_INDEX_WORKER_ENABLED")); raw != "" {
		ragIndexWorkerEnabled = strings.EqualFold(raw, "true")
	}
	ragIndexWorkerIntervalMS := 60000
	ragIndexWorkerIntervalRaw := strings.TrimSpace(os.Getenv("RAG_INDEX_WORKER_INTERVAL_MS"))
	if ragIndexWorkerIntervalRaw != "" {
		parsedInterval, parseErr := strconv.Atoi(ragIndexWorkerIntervalRaw)
		if parseErr != nil || parsedInterval < 1 {
			return Config{}, fmt.Errorf("invalid RAG_INDEX_WORKER_INTERVAL_MS: %q", ragIndexWorkerIntervalRaw)
		}
		ragIndexWorkerIntervalMS = parsedInterval
	}
	ragIndexWorkerClaimLimit := 10
	ragIndexWorkerClaimLimitRaw := strings.TrimSpace(os.Getenv("RAG_INDEX_WORKER_CLAIM_LIMIT"))
	if ragIndexWorkerClaimLimitRaw != "" {
		parsedLimit, parseErr := strconv.Atoi(ragIndexWorkerClaimLimitRaw)
		if parseErr != nil || parsedLimit < 1 {
			return Config{}, fmt.Errorf("invalid RAG_INDEX_WORKER_CLAIM_LIMIT: %q", ragIndexWorkerClaimLimitRaw)
		}
		ragIndexWorkerClaimLimit = parsedLimit
	}
	if strings.EqualFold(env, "production") && qdrantURL != "" && !ragIndexWorkerEnabled {
		return Config{}, fmt.Errorf("RAG_INDEX_WORKER_ENABLED=false is not allowed when APP_ENV=production and QDRANT_URL is set")
	}
	ragIngestionWorkerEnabled := !strings.EqualFold(env, "test")
	if raw := strings.TrimSpace(os.Getenv("RAG_INGESTION_WORKER_ENABLED")); raw != "" {
		ragIngestionWorkerEnabled = strings.EqualFold(raw, "true")
	}
	ragIngestionWorkerIntervalMS := 60000
	ragIngestionWorkerIntervalRaw := strings.TrimSpace(os.Getenv("RAG_INGESTION_WORKER_INTERVAL_MS"))
	if ragIngestionWorkerIntervalRaw != "" {
		parsedInterval, parseErr := strconv.Atoi(ragIngestionWorkerIntervalRaw)
		if parseErr != nil || parsedInterval < 1 {
			return Config{}, fmt.Errorf("invalid RAG_INGESTION_WORKER_INTERVAL_MS: %q", ragIngestionWorkerIntervalRaw)
		}
		ragIngestionWorkerIntervalMS = parsedInterval
	}
	ragIngestionWorkerClaimLimit := 10
	ragIngestionWorkerClaimLimitRaw := strings.TrimSpace(os.Getenv("RAG_INGESTION_WORKER_CLAIM_LIMIT"))
	if ragIngestionWorkerClaimLimitRaw != "" {
		parsedLimit, parseErr := strconv.Atoi(ragIngestionWorkerClaimLimitRaw)
		if parseErr != nil || parsedLimit < 1 {
			return Config{}, fmt.Errorf("invalid RAG_INGESTION_WORKER_CLAIM_LIMIT: %q", ragIngestionWorkerClaimLimitRaw)
		}
		ragIngestionWorkerClaimLimit = parsedLimit
	}
	if strings.EqualFold(env, "production") && !ragIngestionWorkerEnabled {
		return Config{}, fmt.Errorf("RAG_INGESTION_WORKER_ENABLED=false is not allowed when APP_ENV=production")
	}

	// Relay configuration
	relayEnabled := true
	if relayEnabledRaw := strings.TrimSpace(os.Getenv("RELAY_ENABLED")); relayEnabledRaw != "" {
		relayEnabled = strings.EqualFold(relayEnabledRaw, "true")
	}
	if strings.EqualFold(env, "production") && !relayEnabled {
		return Config{}, fmt.Errorf("RELAY_ENABLED=false is not allowed when APP_ENV=production")
	}

	relayDefaultModel := strings.TrimSpace(os.Getenv("RELAY_DEFAULT_MODEL"))
	if relayDefaultModel == "" {
		relayDefaultModel = "gpt-4o-mini"
	}
	relaySemanticCacheBackend := strings.ToLower(strings.TrimSpace(os.Getenv("RELAY_SEMANTIC_CACHE_BACKEND")))
	switch relaySemanticCacheBackend {
	case "":
		relaySemanticCacheBackend = "memory"
	case "none", "memory", "sql":
	default:
		return Config{}, fmt.Errorf("invalid RELAY_SEMANTIC_CACHE_BACKEND: %q", relaySemanticCacheBackend)
	}
	relayRateLimitBackend := strings.ToLower(strings.TrimSpace(os.Getenv("RELAY_RATE_LIMIT_BACKEND")))
	switch relayRateLimitBackend {
	case "":
		relayRateLimitBackend = "memory"
	case "none", "memory", "redis":
	default:
		return Config{}, fmt.Errorf("invalid RELAY_RATE_LIMIT_BACKEND: %q", relayRateLimitBackend)
	}
	redisAddr := ""
	redisPassword := ""
	redisDB := 0
	if redisURLRaw := strings.TrimSpace(os.Getenv("REDIS_URL")); relayRateLimitBackend == "redis" && redisURLRaw != "" {
		parsedRedisURL, parseErr := parseRedisURL(redisURLRaw)
		if parseErr != nil {
			return Config{}, parseErr
		}
		redisAddr = parsedRedisURL.addr
		redisPassword = parsedRedisURL.password
		redisDB = parsedRedisURL.db
	}
	if redisAddrRaw, ok := os.LookupEnv("REDIS_ADDR"); ok {
		redisAddr = strings.TrimSpace(redisAddrRaw)
	}
	if redisPasswordRaw, ok := os.LookupEnv("REDIS_PASSWORD"); ok {
		redisPassword = strings.TrimSpace(redisPasswordRaw)
	}
	if redisDBRaw := strings.TrimSpace(os.Getenv("REDIS_DB")); redisDBRaw != "" {
		parsedDB, parseErr := parseRedisDB(redisDBRaw)
		if parseErr != nil {
			return Config{}, parseErr
		}
		redisDB = parsedDB
	}
	relayRateLimitRedisKeyPrefix := strings.TrimSpace(os.Getenv("RELAY_RATE_LIMIT_REDIS_KEY_PREFIX"))

	relayPricingMaintenanceEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("RELAY_PRICING_MAINTENANCE_ENABLED")), "true")
	relayPricingMaintenanceIntervalMS := 3600000
	if raw := strings.TrimSpace(os.Getenv("RELAY_PRICING_MAINTENANCE_INTERVAL_MS")); raw != "" {
		parsedInterval, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsedInterval < 1 {
			return Config{}, fmt.Errorf("invalid RELAY_PRICING_MAINTENANCE_INTERVAL_MS: %q", raw)
		}
		relayPricingMaintenanceIntervalMS = parsedInterval
	}
	relayPricingMaintenanceProvider := strings.ToLower(strings.TrimSpace(os.Getenv("RELAY_PRICING_MAINTENANCE_PROVIDER")))
	relayPricingMaintenanceSource := strings.TrimSpace(os.Getenv("RELAY_PRICING_MAINTENANCE_SOURCE"))
	if relayPricingMaintenanceSource == "" {
		relayPricingMaintenanceSource = "litellm"
	}
	relayPricingMaintenanceSourceURL := strings.TrimSpace(os.Getenv("RELAY_PRICING_MAINTENANCE_SOURCE_URL"))
	relayPricingMaintenanceModels := splitCommaEnv(os.Getenv("RELAY_PRICING_MAINTENANCE_REQUIRED_MODELS"))
	relayPricingMaintenanceMaxBytes := int64(0)
	if raw := strings.TrimSpace(os.Getenv("RELAY_PRICING_MAINTENANCE_MAX_BYTES")); raw != "" {
		parsedMaxBytes, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || parsedMaxBytes < 1 {
			return Config{}, fmt.Errorf("invalid RELAY_PRICING_MAINTENANCE_MAX_BYTES: %q", raw)
		}
		relayPricingMaintenanceMaxBytes = parsedMaxBytes
	}
	relayPricingReconciliationLimit := 100
	if raw := strings.TrimSpace(os.Getenv("RELAY_PRICING_RECONCILIATION_LIMIT")); raw != "" {
		parsedLimit, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsedLimit < 1 {
			return Config{}, fmt.Errorf("invalid RELAY_PRICING_RECONCILIATION_LIMIT: %q", raw)
		}
		relayPricingReconciliationLimit = parsedLimit
	}
	if relayPricingMaintenanceEnabled {
		if relayPricingMaintenanceProvider == "" {
			return Config{}, fmt.Errorf("RELAY_PRICING_MAINTENANCE_PROVIDER is required when RELAY_PRICING_MAINTENANCE_ENABLED=true")
		}
		if relayPricingMaintenanceSourceURL == "" {
			return Config{}, fmt.Errorf("RELAY_PRICING_MAINTENANCE_SOURCE_URL is required when RELAY_PRICING_MAINTENANCE_ENABLED=true")
		}
		parsedSourceURL, parseErr := url.Parse(relayPricingMaintenanceSourceURL)
		if parseErr != nil || parsedSourceURL.Scheme != "https" || strings.TrimSpace(parsedSourceURL.Host) == "" {
			return Config{}, fmt.Errorf("RELAY_PRICING_MAINTENANCE_SOURCE_URL must be an https URL with a host")
		}
	}
	relayBatchPollingWorkerEnabled := false
	if raw := strings.TrimSpace(os.Getenv("RELAY_BATCH_POLLING_WORKER_ENABLED")); raw != "" {
		relayBatchPollingWorkerEnabled = strings.EqualFold(raw, "true")
	}
	relayBatchPollingWorkerIntervalMS := 60000
	if raw := strings.TrimSpace(os.Getenv("RELAY_BATCH_POLLING_WORKER_INTERVAL_MS")); raw != "" {
		parsedInterval, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsedInterval < 1 {
			return Config{}, fmt.Errorf("invalid RELAY_BATCH_POLLING_WORKER_INTERVAL_MS: %q", raw)
		}
		relayBatchPollingWorkerIntervalMS = parsedInterval
	}
	relayBatchPollingWorkerClaimLimit := 10
	if raw := strings.TrimSpace(os.Getenv("RELAY_BATCH_POLLING_WORKER_CLAIM_LIMIT")); raw != "" {
		parsedLimit, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsedLimit < 1 {
			return Config{}, fmt.Errorf("invalid RELAY_BATCH_POLLING_WORKER_CLAIM_LIMIT: %q", raw)
		}
		relayBatchPollingWorkerClaimLimit = parsedLimit
	}
	relayBatchCommercialLifecycleEnabled := false
	if raw := strings.TrimSpace(os.Getenv("RELAY_BATCH_COMMERCIAL_LIFECYCLE_ENABLED")); raw != "" {
		relayBatchCommercialLifecycleEnabled = strings.EqualFold(raw, "true")
	}
	if strings.EqualFold(env, "production") && relayBatchCommercialLifecycleEnabled && !relayBatchPollingWorkerEnabled {
		return Config{}, fmt.Errorf("RELAY_BATCH_POLLING_WORKER_ENABLED=true is required when RELAY_BATCH_COMMERCIAL_LIFECYCLE_ENABLED=true in production")
	}
	relayRealtimeCommercialLifecycleEnabled := false
	if raw := strings.TrimSpace(os.Getenv("RELAY_REALTIME_COMMERCIAL_LIFECYCLE_ENABLED")); raw != "" {
		relayRealtimeCommercialLifecycleEnabled = strings.EqualFold(raw, "true")
	}

	// Default channel configuration (for development)
	openaiAPIKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	openaiBaseURL := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL"))
	if openaiBaseURL == "" {
		openaiBaseURL = "https://api.openai.com"
	}

	stripeSecretKey := strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY"))
	stripeSuccessURL := strings.TrimSpace(os.Getenv("STRIPE_SUCCESS_URL"))
	stripeCancelURL := strings.TrimSpace(os.Getenv("STRIPE_CANCEL_URL"))
	stripeWebhookSecret := strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET"))
	alipayCheckoutBaseURL := strings.TrimSpace(os.Getenv("ALIPAY_CHECKOUT_BASE_URL"))
	alipayWebhookSecret := strings.TrimSpace(os.Getenv("ALIPAY_WEBHOOK_SECRET"))
	weChatPayCheckoutBaseURL := strings.TrimSpace(os.Getenv("WECHATPAY_CHECKOUT_BASE_URL"))
	weChatPayWebhookSecret := strings.TrimSpace(os.Getenv("WECHATPAY_WEBHOOK_SECRET"))
	marketplacePayoutProvider := strings.ToLower(strings.TrimSpace(os.Getenv("MARKETPLACE_PAYOUT_PROVIDER")))
	if marketplacePayoutProvider == "" {
		marketplacePayoutProvider = "local"
	}
	marketplacePayoutWebhookURL := strings.TrimSpace(os.Getenv("MARKETPLACE_PAYOUT_WEBHOOK_URL"))
	marketplacePayoutWebhookSecret := strings.TrimSpace(os.Getenv("MARKETPLACE_PAYOUT_WEBHOOK_SECRET"))

	scheduleWorkerEnabled := !strings.EqualFold(env, "test")
	if raw := strings.TrimSpace(os.Getenv("SCHEDULE_WORKER_ENABLED")); raw != "" {
		scheduleWorkerEnabled = strings.EqualFold(raw, "true")
	}
	scheduleWorkerIntervalMS := 60000
	scheduleWorkerIntervalRaw := strings.TrimSpace(os.Getenv("SCHEDULE_WORKER_INTERVAL_MS"))
	if scheduleWorkerIntervalRaw != "" {
		parsedInterval, parseErr := strconv.Atoi(scheduleWorkerIntervalRaw)
		if parseErr != nil || parsedInterval < 1 {
			return Config{}, fmt.Errorf("invalid SCHEDULE_WORKER_INTERVAL_MS: %q", scheduleWorkerIntervalRaw)
		}
		scheduleWorkerIntervalMS = parsedInterval
	}
	scheduleWorkerClaimLimit := 50
	scheduleWorkerClaimLimitRaw := strings.TrimSpace(os.Getenv("SCHEDULE_WORKER_CLAIM_LIMIT"))
	if scheduleWorkerClaimLimitRaw != "" {
		parsedLimit, parseErr := strconv.Atoi(scheduleWorkerClaimLimitRaw)
		if parseErr != nil || parsedLimit < 1 {
			return Config{}, fmt.Errorf("invalid SCHEDULE_WORKER_CLAIM_LIMIT: %q", scheduleWorkerClaimLimitRaw)
		}
		scheduleWorkerClaimLimit = parsedLimit
	}
	workflowSystemMaxConcurrent := 0
	workflowSystemMaxConcurrentRaw := strings.TrimSpace(os.Getenv("WORKFLOW_SYSTEM_MAX_CONCURRENT"))
	if workflowSystemMaxConcurrentRaw != "" {
		parsedLimit, parseErr := strconv.Atoi(workflowSystemMaxConcurrentRaw)
		if parseErr != nil || parsedLimit < 1 {
			return Config{}, fmt.Errorf("invalid WORKFLOW_SYSTEM_MAX_CONCURRENT: %q", workflowSystemMaxConcurrentRaw)
		}
		workflowSystemMaxConcurrent = parsedLimit
	}
	workflowGlobalMaxExecutionsPerMinute := 0
	workflowGlobalMaxExecutionsPerMinuteRaw := strings.TrimSpace(os.Getenv("WORKFLOW_GLOBAL_MAX_EXECUTIONS_PER_MINUTE"))
	if workflowGlobalMaxExecutionsPerMinuteRaw != "" {
		parsedLimit, parseErr := strconv.Atoi(workflowGlobalMaxExecutionsPerMinuteRaw)
		if parseErr != nil || parsedLimit < 1 {
			return Config{}, fmt.Errorf("invalid WORKFLOW_GLOBAL_MAX_EXECUTIONS_PER_MINUTE: %q", workflowGlobalMaxExecutionsPerMinuteRaw)
		}
		workflowGlobalMaxExecutionsPerMinute = parsedLimit
	}
	workflowRelayBaseURL := strings.TrimSpace(os.Getenv("WORKFLOW_RELAY_BASE_URL"))
	workflowSandboxEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("WORKFLOW_SANDBOX_ENABLED")), "true")
	workflowSandboxAllowedLanguages := strings.TrimSpace(os.Getenv("WORKFLOW_SANDBOX_ALLOWED_LANGUAGES"))
	workflowSandboxMemoryMB := 0
	if raw := strings.TrimSpace(os.Getenv("WORKFLOW_SANDBOX_MEMORY_MB")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 1 {
			return Config{}, fmt.Errorf("invalid WORKFLOW_SANDBOX_MEMORY_MB: %q", raw)
		}
		workflowSandboxMemoryMB = parsed
	}
	workflowSandboxCPUs := 0
	if raw := strings.TrimSpace(os.Getenv("WORKFLOW_SANDBOX_CPUS")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 1 {
			return Config{}, fmt.Errorf("invalid WORKFLOW_SANDBOX_CPUS: %q", raw)
		}
		workflowSandboxCPUs = parsed
	}
	workflowSandboxDefaultTimeoutMS := 0
	if raw := strings.TrimSpace(os.Getenv("WORKFLOW_SANDBOX_DEFAULT_TIMEOUT_MS")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 1 {
			return Config{}, fmt.Errorf("invalid WORKFLOW_SANDBOX_DEFAULT_TIMEOUT_MS: %q", raw)
		}
		workflowSandboxDefaultTimeoutMS = parsed
	}
	workflowSandboxMaxTimeoutMS := 0
	if raw := strings.TrimSpace(os.Getenv("WORKFLOW_SANDBOX_MAX_TIMEOUT_MS")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 1 {
			return Config{}, fmt.Errorf("invalid WORKFLOW_SANDBOX_MAX_TIMEOUT_MS: %q", raw)
		}
		workflowSandboxMaxTimeoutMS = parsed
	}
	channelMessageLogArchiveRoot := strings.TrimSpace(os.Getenv("CHANNEL_MESSAGE_LOG_ARCHIVE_ROOT"))
	channelMessageLogArchiveBackend := strings.ToLower(strings.TrimSpace(os.Getenv("CHANNEL_MESSAGE_LOG_ARCHIVE_BACKEND")))
	channelMessageLogArchiveS3Endpoint := strings.TrimSpace(os.Getenv("CHANNEL_MESSAGE_LOG_ARCHIVE_S3_ENDPOINT"))
	channelMessageLogArchiveS3Region := strings.TrimSpace(os.Getenv("CHANNEL_MESSAGE_LOG_ARCHIVE_S3_REGION"))
	channelMessageLogArchiveS3Bucket := strings.TrimSpace(os.Getenv("CHANNEL_MESSAGE_LOG_ARCHIVE_S3_BUCKET"))
	channelMessageLogArchiveS3AccessKey := strings.TrimSpace(os.Getenv("CHANNEL_MESSAGE_LOG_ARCHIVE_S3_ACCESS_KEY"))
	channelMessageLogArchiveS3SecretKey := strings.TrimSpace(os.Getenv("CHANNEL_MESSAGE_LOG_ARCHIVE_S3_SECRET_KEY"))
	if channelMessageLogArchiveBackend == "minio" {
		channelMessageLogArchiveBackend = "s3"
	}
	switch channelMessageLogArchiveBackend {
	case "":
		if channelMessageLogArchiveRoot != "" {
			channelMessageLogArchiveBackend = "file"
		}
	case "file", "s3":
	default:
		return Config{}, fmt.Errorf("invalid CHANNEL_MESSAGE_LOG_ARCHIVE_BACKEND: %q", channelMessageLogArchiveBackend)
	}
	channelMessageLogArchiveEnabled := channelMessageLogArchiveBackend != ""
	if channelMessageLogArchiveBackend == "file" && channelMessageLogArchiveRoot == "" {
		return Config{}, fmt.Errorf("CHANNEL_MESSAGE_LOG_ARCHIVE_ROOT is required when CHANNEL_MESSAGE_LOG_ARCHIVE_BACKEND=file")
	}
	if channelMessageLogArchiveBackend == "s3" {
		if channelMessageLogArchiveS3Endpoint == "" {
			return Config{}, fmt.Errorf("CHANNEL_MESSAGE_LOG_ARCHIVE_S3_ENDPOINT is required when CHANNEL_MESSAGE_LOG_ARCHIVE_BACKEND=s3")
		}
		if channelMessageLogArchiveS3Bucket == "" {
			return Config{}, fmt.Errorf("CHANNEL_MESSAGE_LOG_ARCHIVE_S3_BUCKET is required when CHANNEL_MESSAGE_LOG_ARCHIVE_BACKEND=s3")
		}
		if channelMessageLogArchiveS3AccessKey == "" {
			return Config{}, fmt.Errorf("CHANNEL_MESSAGE_LOG_ARCHIVE_S3_ACCESS_KEY is required when CHANNEL_MESSAGE_LOG_ARCHIVE_BACKEND=s3")
		}
		if channelMessageLogArchiveS3SecretKey == "" {
			return Config{}, fmt.Errorf("CHANNEL_MESSAGE_LOG_ARCHIVE_S3_SECRET_KEY is required when CHANNEL_MESSAGE_LOG_ARCHIVE_BACKEND=s3")
		}
		if channelMessageLogArchiveS3Region == "" {
			channelMessageLogArchiveS3Region = "us-east-1"
		}
	}
	channelMessageLogArchiveIntervalMS := 3600000
	channelMessageLogArchiveIntervalRaw := strings.TrimSpace(os.Getenv("CHANNEL_MESSAGE_LOG_ARCHIVE_INTERVAL_MS"))
	if channelMessageLogArchiveIntervalRaw != "" {
		parsedInterval, parseErr := strconv.Atoi(channelMessageLogArchiveIntervalRaw)
		if parseErr != nil || parsedInterval < 1 {
			return Config{}, fmt.Errorf("invalid CHANNEL_MESSAGE_LOG_ARCHIVE_INTERVAL_MS: %q", channelMessageLogArchiveIntervalRaw)
		}
		channelMessageLogArchiveIntervalMS = parsedInterval
	}
	channelMessageLogRetentionHours := 30 * 24
	channelMessageLogRetentionHoursRaw := strings.TrimSpace(os.Getenv("CHANNEL_MESSAGE_LOG_RETENTION_HOURS"))
	if channelMessageLogRetentionHoursRaw != "" {
		parsedRetention, parseErr := strconv.Atoi(channelMessageLogRetentionHoursRaw)
		if parseErr != nil || parsedRetention < 1 {
			return Config{}, fmt.Errorf("invalid CHANNEL_MESSAGE_LOG_RETENTION_HOURS: %q", channelMessageLogRetentionHoursRaw)
		}
		channelMessageLogRetentionHours = parsedRetention
	}
	channelMessageLogArchiveLimit := 500
	channelMessageLogArchiveLimitRaw := strings.TrimSpace(os.Getenv("CHANNEL_MESSAGE_LOG_ARCHIVE_LIMIT"))
	if channelMessageLogArchiveLimitRaw != "" {
		parsedLimit, parseErr := strconv.Atoi(channelMessageLogArchiveLimitRaw)
		if parseErr != nil || parsedLimit < 1 {
			return Config{}, fmt.Errorf("invalid CHANNEL_MESSAGE_LOG_ARCHIVE_LIMIT: %q", channelMessageLogArchiveLimitRaw)
		}
		channelMessageLogArchiveLimit = parsedLimit
	}
	observabilityRequestLogBackend := strings.ToLower(strings.TrimSpace(os.Getenv("OBSERVABILITY_REQUEST_LOG_BACKEND")))
	switch observabilityRequestLogBackend {
	case "":
		observabilityRequestLogBackend = "none"
	case "none", "clickhouse":
	default:
		return Config{}, fmt.Errorf("invalid OBSERVABILITY_REQUEST_LOG_BACKEND: %q", observabilityRequestLogBackend)
	}
	clickHouseDSN := strings.TrimSpace(os.Getenv("CLICKHOUSE_DSN"))
	if observabilityRequestLogBackend == "clickhouse" && clickHouseDSN == "" {
		return Config{}, fmt.Errorf("CLICKHOUSE_DSN is required when OBSERVABILITY_REQUEST_LOG_BACKEND=clickhouse")
	}
	if strings.EqualFold(env, "production") && relayEnabled && observabilityRequestLogBackend == "none" {
		return Config{}, fmt.Errorf("OBSERVABILITY_REQUEST_LOG_BACKEND must not be none when APP_ENV=production and RELAY_ENABLED=true")
	}
	if err := validateProductionPaymentConfig(
		env,
		stripeSecretKey,
		stripeSuccessURL,
		stripeCancelURL,
		stripeWebhookSecret,
		alipayCheckoutBaseURL,
		alipayWebhookSecret,
		weChatPayCheckoutBaseURL,
		weChatPayWebhookSecret,
	); err != nil {
		return Config{}, err
	}
	if err := validateMarketplacePayoutConfig(
		env,
		marketplacePayoutProvider,
		marketplacePayoutWebhookURL,
		marketplacePayoutWebhookSecret,
	); err != nil {
		return Config{}, err
	}
	clickHouseDriver := strings.TrimSpace(os.Getenv("CLICKHOUSE_DRIVER"))
	if clickHouseDriver == "" {
		clickHouseDriver = "clickhouse"
	}
	observabilityHTTPAlertsEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("OBSERVABILITY_HTTP_ALERTS_ENABLED")), "true")
	alertWebhookURL := strings.TrimSpace(os.Getenv("ALERT_WEBHOOK_URL"))
	alertWebhookSecret := strings.TrimSpace(os.Getenv("ALERT_WEBHOOK_SECRET"))
	observabilityHTTPRecoveryEnabledRaw := strings.TrimSpace(os.Getenv("OBSERVABILITY_HTTP_RECOVERY_AUDIT_ENABLED"))
	if observabilityHTTPRecoveryEnabledRaw == "" {
		observabilityHTTPRecoveryEnabledRaw = strings.TrimSpace(os.Getenv("OBSERVABILITY_HTTP_RECOVERY_ENABLED"))
	}
	observabilityHTTPRecoveryEnabled := strings.EqualFold(observabilityHTTPRecoveryEnabledRaw, "true")
	observabilityHTTPLatencySLOThresholdMS := 5000
	observabilityHTTPLatencySLOThresholdRaw := strings.TrimSpace(os.Getenv("OBSERVABILITY_HTTP_LATENCY_SLO_THRESHOLD_MS"))
	if observabilityHTTPLatencySLOThresholdRaw != "" {
		parsedThreshold, parseErr := strconv.Atoi(observabilityHTTPLatencySLOThresholdRaw)
		if parseErr != nil || parsedThreshold < 1 {
			return Config{}, fmt.Errorf("invalid OBSERVABILITY_HTTP_LATENCY_SLO_THRESHOLD_MS: %q", observabilityHTTPLatencySLOThresholdRaw)
		}
		observabilityHTTPLatencySLOThresholdMS = parsedThreshold
	}
	observabilityHTTPRecoveryCooldownMS := 300000
	observabilityHTTPRecoveryCooldownName := "OBSERVABILITY_HTTP_RECOVERY_AUDIT_COOLDOWN_MS"
	observabilityHTTPRecoveryCooldownRaw := strings.TrimSpace(os.Getenv(observabilityHTTPRecoveryCooldownName))
	if observabilityHTTPRecoveryCooldownRaw == "" {
		observabilityHTTPRecoveryCooldownName = "OBSERVABILITY_HTTP_RECOVERY_COOLDOWN_MS"
		observabilityHTTPRecoveryCooldownRaw = strings.TrimSpace(os.Getenv(observabilityHTTPRecoveryCooldownName))
	}
	if observabilityHTTPRecoveryCooldownRaw != "" {
		parsedCooldown, parseErr := strconv.Atoi(observabilityHTTPRecoveryCooldownRaw)
		if parseErr != nil || parsedCooldown < 1 {
			return Config{}, fmt.Errorf("invalid %s: %q", observabilityHTTPRecoveryCooldownName, observabilityHTTPRecoveryCooldownRaw)
		}
		observabilityHTTPRecoveryCooldownMS = parsedCooldown
	}

	dbMode := strings.ToLower(strings.TrimSpace(os.Getenv("OBLIVIOUS_DB_MODE")))
	if dbMode == "" {
		dbMode = "monolith"
	}
	switch dbMode {
	case "monolith", "dual_write", "microservices":
	default:
		return Config{}, fmt.Errorf("invalid OBLIVIOUS_DB_MODE: %q (must be monolith, dual_write, or microservices)", dbMode)
	}

	dbURLRelay := strings.TrimSpace(os.Getenv("DB_URL_RELAY"))
	dbURLChat := strings.TrimSpace(os.Getenv("DB_URL_CHAT"))
	dbURLWorkflow := strings.TrimSpace(os.Getenv("DB_URL_WORKFLOW"))
	dbURLRAG := strings.TrimSpace(os.Getenv("DB_URL_RAG"))
	dbURLAgent := strings.TrimSpace(os.Getenv("DB_URL_AGENT"))
	dbURLBilling := strings.TrimSpace(os.Getenv("DB_URL_BILLING"))
	dbURLMarketplace := strings.TrimSpace(os.Getenv("DB_URL_MARKETPLACE"))
	dbURLAdmin := strings.TrimSpace(os.Getenv("DB_URL_ADMIN"))
	dbURLChannel := strings.TrimSpace(os.Getenv("DB_URL_CHANNEL"))
	dbURLTask := strings.TrimSpace(os.Getenv("DB_URL_TASK"))
	dbURLObservability := strings.TrimSpace(os.Getenv("DB_URL_OBSERVABILITY"))

	return Config{
		Port:                                    port,
		Env:                                     env,
		CORSAllowedOrigins:                      origins,
		DatabaseURL:                             databaseURL,
		SessionCookieName:                       sessionCookieName,
		SessionCookieSecure:                     sessionCookieSecure,
		SessionSecret:                           sessionSecret,
		SecretEncryptionKey:                     secretEncryptionKey,
		LLMBaseURL:                              llmBaseURL,
		LLMAPIKey:                               llmAPIKey,
		LLMTimeoutMS:                            llmTimeoutMS,
		ModelDefaultName:                        modelDefaultName,
		AgentRelayBaseURL:                       agentRelayBaseURL,
		ChatRelayBaseURL:                        chatRelayBaseURL,
		AgentWebSearchProvider:                  agentWebSearchProvider,
		AgentWebSearchFallback:                  agentWebSearchFallback,
		AgentWebSearchEndpoint:                  agentWebSearchEndpoint,
		AgentWebSearchAPIKey:                    agentWebSearchAPIKey,
		AgentWebSearchResultLimit:               agentWebSearchResultLimit,
		AgentWebSearchGoogleCSEID:               agentWebSearchGoogleCSEID,
		QdrantURL:                               qdrantURL,
		QdrantAPIKey:                            qdrantAPIKey,
		QdrantVectorSize:                        qdrantVectorSize,
		RAGRerankerBaseURL:                      ragRerankerBaseURL,
		RAGRerankerAPIKey:                       ragRerankerAPIKey,
		RAGRerankerModel:                        ragRerankerModel,
		RAGRerankerTopK:                         ragRerankerTopK,
		RAGIndexWorkerEnabled:                   ragIndexWorkerEnabled,
		RAGIndexWorkerIntervalMS:                ragIndexWorkerIntervalMS,
		RAGIndexWorkerClaimLimit:                ragIndexWorkerClaimLimit,
		RAGIngestionWorkerEnabled:               ragIngestionWorkerEnabled,
		RAGIngestionWorkerIntervalMS:            ragIngestionWorkerIntervalMS,
		RAGIngestionWorkerClaimLimit:            ragIngestionWorkerClaimLimit,
		RelayEnabled:                            relayEnabled,
		RelayDefaultModel:                       relayDefaultModel,
		RelayRateLimitBackend:                   relayRateLimitBackend,
		RelayRateLimitRedisKeyPrefix:            relayRateLimitRedisKeyPrefix,
		RelaySemanticCacheBackend:               relaySemanticCacheBackend,
		RelayPricingMaintenanceEnabled:          relayPricingMaintenanceEnabled,
		RelayPricingMaintenanceIntervalMS:       relayPricingMaintenanceIntervalMS,
		RelayPricingMaintenanceProvider:         relayPricingMaintenanceProvider,
		RelayPricingMaintenanceSource:           relayPricingMaintenanceSource,
		RelayPricingMaintenanceSourceURL:        relayPricingMaintenanceSourceURL,
		RelayPricingMaintenanceModels:           relayPricingMaintenanceModels,
		RelayPricingMaintenanceMaxBytes:         relayPricingMaintenanceMaxBytes,
		RelayPricingReconciliationLimit:         relayPricingReconciliationLimit,
		RelayBatchPollingWorkerEnabled:          relayBatchPollingWorkerEnabled,
		RelayBatchPollingWorkerIntervalMS:       relayBatchPollingWorkerIntervalMS,
		RelayBatchPollingWorkerClaimLimit:       relayBatchPollingWorkerClaimLimit,
		RelayBatchCommercialLifecycleEnabled:    relayBatchCommercialLifecycleEnabled,
		RelayRealtimeCommercialLifecycleEnabled: relayRealtimeCommercialLifecycleEnabled,
		RedisAddr:                               redisAddr,
		RedisPassword:                           redisPassword,
		RedisDB:                                 redisDB,
		OpenAIAPIKey:                            openaiAPIKey,
		OpenAIBaseURL:                           openaiBaseURL,
		StripeSecretKey:                         stripeSecretKey,
		StripeSuccessURL:                        stripeSuccessURL,
		StripeCancelURL:                         stripeCancelURL,
		StripeWebhookSecret:                     stripeWebhookSecret,
		AlipayCheckoutBaseURL:                   alipayCheckoutBaseURL,
		AlipayWebhookSecret:                     alipayWebhookSecret,
		WeChatPayCheckoutBaseURL:                weChatPayCheckoutBaseURL,
		WeChatPayWebhookSecret:                  weChatPayWebhookSecret,
		MarketplacePayoutProvider:               marketplacePayoutProvider,
		MarketplacePayoutWebhookURL:             marketplacePayoutWebhookURL,
		MarketplacePayoutWebhookSecret:          marketplacePayoutWebhookSecret,

		ScheduleWorkerEnabled:    scheduleWorkerEnabled,
		ScheduleWorkerIntervalMS: scheduleWorkerIntervalMS,
		ScheduleWorkerClaimLimit: scheduleWorkerClaimLimit,

		WorkflowSystemMaxConcurrent:          workflowSystemMaxConcurrent,
		WorkflowGlobalMaxExecutionsPerMinute: workflowGlobalMaxExecutionsPerMinute,
		WorkflowRelayBaseURL:                 workflowRelayBaseURL,
		WorkflowSandboxEnabled:               workflowSandboxEnabled,
		WorkflowSandboxAllowedLanguages:      workflowSandboxAllowedLanguages,
		WorkflowSandboxMemoryMB:              workflowSandboxMemoryMB,
		WorkflowSandboxCPUs:                  workflowSandboxCPUs,
		WorkflowSandboxDefaultTimeoutMS:      workflowSandboxDefaultTimeoutMS,
		WorkflowSandboxMaxTimeoutMS:          workflowSandboxMaxTimeoutMS,

		ChannelMessageLogArchiveEnabled:     channelMessageLogArchiveEnabled,
		ChannelMessageLogArchiveBackend:     channelMessageLogArchiveBackend,
		ChannelMessageLogArchiveRoot:        channelMessageLogArchiveRoot,
		ChannelMessageLogArchiveS3Endpoint:  channelMessageLogArchiveS3Endpoint,
		ChannelMessageLogArchiveS3Region:    channelMessageLogArchiveS3Region,
		ChannelMessageLogArchiveS3Bucket:    channelMessageLogArchiveS3Bucket,
		ChannelMessageLogArchiveS3AccessKey: channelMessageLogArchiveS3AccessKey,
		ChannelMessageLogArchiveS3SecretKey: channelMessageLogArchiveS3SecretKey,
		ChannelMessageLogArchiveIntervalMS:  channelMessageLogArchiveIntervalMS,
		ChannelMessageLogRetentionHours:     channelMessageLogRetentionHours,
		ChannelMessageLogArchiveLimit:       channelMessageLogArchiveLimit,

		ObservabilityRequestLogBackend:         observabilityRequestLogBackend,
		ClickHouseDSN:                          clickHouseDSN,
		ClickHouseDriver:                       clickHouseDriver,
		ObservabilityHTTPAlertsEnabled:         observabilityHTTPAlertsEnabled,
		AlertWebhookURL:                        alertWebhookURL,
		AlertWebhookSecret:                     alertWebhookSecret,
		ObservabilityHTTPLatencySLOThresholdMS: observabilityHTTPLatencySLOThresholdMS,

		ObservabilityHTTPRecoveryEnabled:    observabilityHTTPRecoveryEnabled,
		ObservabilityHTTPRecoveryCooldownMS: observabilityHTTPRecoveryCooldownMS,

		DBMode:             dbMode,
		DBURLRelay:         dbURLRelay,
		DBURLChat:          dbURLChat,
		DBURLWorkflow:      dbURLWorkflow,
		DBURLRAG:           dbURLRAG,
		DBURLAgent:         dbURLAgent,
		DBURLBilling:       dbURLBilling,
		DBURLMarketplace:   dbURLMarketplace,
		DBURLAdmin:         dbURLAdmin,
		DBURLChannel:       dbURLChannel,
		DBURLTask:          dbURLTask,
		DBURLObservability: dbURLObservability,
	}, nil
}

func validateProductionSessionConfig(env, sessionSecret string, sessionCookieSecure bool) error {
	if !strings.EqualFold(env, "production") {
		return nil
	}
	normalizedSecret := strings.ToLower(strings.TrimSpace(sessionSecret))
	switch normalizedSecret {
	case "change-me", "changeme", "test-secret", "secret", "dev-secret", "development":
		return fmt.Errorf("SESSION_SECRET must not use a default value when APP_ENV=production")
	}
	if len(sessionSecret) < 32 {
		return fmt.Errorf("SESSION_SECRET must be at least 32 characters when APP_ENV=production")
	}
	if !sessionCookieSecure {
		return fmt.Errorf("SESSION_COOKIE_SECURE=false is not allowed when APP_ENV=production")
	}
	return nil
}

func validateProductionSecretEncryptionConfig(env, sessionSecret, secretEncryptionKey string) error {
	if !strings.EqualFold(env, "production") {
		return nil
	}
	trimmedKey := strings.TrimSpace(secretEncryptionKey)
	if trimmedKey == "" {
		return fmt.Errorf("OBLIVIOUS_SECRET_ENCRYPTION_KEY is required when APP_ENV=production")
	}
	normalizedKey := strings.ToLower(trimmedKey)
	switch normalizedKey {
	case "change-me", "changeme", "test-secret", "secret", "dev-secret", "development":
		return fmt.Errorf("OBLIVIOUS_SECRET_ENCRYPTION_KEY must not use a default value when APP_ENV=production")
	}
	if len(trimmedKey) < 32 {
		return fmt.Errorf("OBLIVIOUS_SECRET_ENCRYPTION_KEY must be at least 32 characters when APP_ENV=production")
	}
	if trimmedKey == strings.TrimSpace(sessionSecret) {
		return fmt.Errorf("OBLIVIOUS_SECRET_ENCRYPTION_KEY must be distinct from SESSION_SECRET when APP_ENV=production")
	}
	return nil
}

func validateProductionPaymentConfig(env, stripeSecretKey, stripeSuccessURL, stripeCancelURL, stripeWebhookSecret, alipayCheckoutBaseURL, alipayWebhookSecret, weChatPayCheckoutBaseURL, weChatPayWebhookSecret string) error {
	if !strings.EqualFold(env, "production") {
		return nil
	}

	stripeConfigured := allNonEmpty(stripeSecretKey, stripeSuccessURL, stripeCancelURL, stripeWebhookSecret)
	if anyNonEmpty(stripeSecretKey, stripeSuccessURL, stripeCancelURL, stripeWebhookSecret) && !stripeConfigured {
		return fmt.Errorf("STRIPE_SECRET_KEY, STRIPE_SUCCESS_URL, STRIPE_CANCEL_URL, and STRIPE_WEBHOOK_SECRET are required together when APP_ENV=production")
	}

	alipayConfigured := allNonEmpty(alipayCheckoutBaseURL, alipayWebhookSecret)
	if anyNonEmpty(alipayCheckoutBaseURL, alipayWebhookSecret) && !alipayConfigured {
		return fmt.Errorf("ALIPAY_CHECKOUT_BASE_URL and ALIPAY_WEBHOOK_SECRET are required together when APP_ENV=production")
	}
	if alipayConfigured {
		if err := validateHTTPSURL("ALIPAY_CHECKOUT_BASE_URL", alipayCheckoutBaseURL); err != nil {
			return err
		}
	}

	weChatPayConfigured := allNonEmpty(weChatPayCheckoutBaseURL, weChatPayWebhookSecret)
	if anyNonEmpty(weChatPayCheckoutBaseURL, weChatPayWebhookSecret) && !weChatPayConfigured {
		return fmt.Errorf("WECHATPAY_CHECKOUT_BASE_URL and WECHATPAY_WEBHOOK_SECRET are required together when APP_ENV=production")
	}
	if weChatPayConfigured {
		if err := validateHTTPSURL("WECHATPAY_CHECKOUT_BASE_URL", weChatPayCheckoutBaseURL); err != nil {
			return err
		}
	}

	if !stripeConfigured && !alipayConfigured && !weChatPayConfigured {
		return fmt.Errorf("at least one payment provider must be fully configured when APP_ENV=production")
	}
	return nil
}

func validateMarketplacePayoutConfig(env, provider, webhookURL, webhookSecret string) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = "local"
	}

	switch provider {
	case "local":
		if strings.EqualFold(env, "production") {
			return fmt.Errorf("MARKETPLACE_PAYOUT_PROVIDER=local is not allowed when APP_ENV=production")
		}
		if anyNonEmpty(webhookURL, webhookSecret) {
			return fmt.Errorf("MARKETPLACE_PAYOUT_PROVIDER=webhook is required when marketplace payout webhook settings are configured")
		}
		return nil
	case "webhook":
		if !allNonEmpty(webhookURL, webhookSecret) {
			return fmt.Errorf("MARKETPLACE_PAYOUT_WEBHOOK_URL and MARKETPLACE_PAYOUT_WEBHOOK_SECRET are required together when MARKETPLACE_PAYOUT_PROVIDER=webhook")
		}
		return validateHTTPURL("MARKETPLACE_PAYOUT_WEBHOOK_URL", webhookURL)
	default:
		return fmt.Errorf("invalid MARKETPLACE_PAYOUT_PROVIDER: %q (must be local or webhook)", provider)
	}
}

func validateHTTPURL(name, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid %s: %w", name, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must use http or https", name)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("%s must include a host", name)
	}
	if parsed.User != nil {
		return fmt.Errorf("%s must not include credentials", name)
	}
	return nil
}

func validateHTTPSURL(name, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid %s: %w", name, err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("%s must use https", name)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("%s must include a host", name)
	}
	if parsed.User != nil {
		return fmt.Errorf("%s must not include credentials", name)
	}
	return nil
}

func allNonEmpty(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}

func anyNonEmpty(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func splitCommaEnv(raw string) []string {
	values := []string{}
	for _, part := range strings.Split(raw, ",") {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

type redisURLConfig struct {
	addr     string
	password string
	db       int
}

func parseRedisURL(raw string) (redisURLConfig, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return redisURLConfig{}, fmt.Errorf("invalid REDIS_URL: %q", raw)
	}
	if parsed.Scheme != "redis" && parsed.Scheme != "rediss" {
		return redisURLConfig{}, fmt.Errorf("invalid REDIS_URL scheme: %q", parsed.Scheme)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return redisURLConfig{}, fmt.Errorf("invalid REDIS_URL: host is required")
	}
	cfg := redisURLConfig{addr: parsed.Host}
	if password, ok := parsed.User.Password(); ok {
		cfg.password = password
	}
	dbRaw := strings.Trim(strings.TrimSpace(parsed.Path), "/")
	if dbRaw == "" {
		return cfg, nil
	}
	parsedDB, parseErr := parseRedisDB(dbRaw)
	if parseErr != nil {
		return redisURLConfig{}, fmt.Errorf("invalid REDIS_URL database: %q", dbRaw)
	}
	cfg.db = parsedDB
	return cfg, nil
}

func parseRedisDB(raw string) (int, error) {
	parsedDB, err := strconv.Atoi(raw)
	if err != nil || parsedDB < 0 {
		return 0, fmt.Errorf("invalid REDIS_DB: %q", raw)
	}
	return parsedDB, nil
}
