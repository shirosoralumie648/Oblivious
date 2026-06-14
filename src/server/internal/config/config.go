package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                int
	Env                 string
	CORSAllowedOrigins  []string
	DatabaseURL         string
	SessionCookieName   string
	SessionCookieSecure bool
	SessionSecret       string
	LLMBaseURL          string
	LLMAPIKey           string
	LLMTimeoutMS        int
	ModelDefaultName    string

	// AgentRelayBaseURL points the standalone Agent runtime at the Relay/OpenAI-compatible API.
	// It defaults in the runtime wiring when unset so monolith tests keep their historical env.
	AgentRelayBaseURL string

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

	// Qdrant vector database configuration. Disabled unless QDRANT_URL is set.
	QdrantURL        string
	QdrantAPIKey     string
	QdrantVectorSize int

	// RAG reranker configuration. Disabled unless RAG_RERANKER_BASE_URL is set.
	RAGRerankerBaseURL string
	RAGRerankerAPIKey  string
	RAGRerankerModel   string
	RAGRerankerTopK    int

	// Relay configuration
	RelayEnabled                 bool
	RelayDefaultModel            string
	RelayRateLimitBackend        string
	RelayRateLimitRedisKeyPrefix string
	RelaySemanticCacheBackend    string
	RedisAddr                    string
	RedisPassword                string
	RedisDB                      int

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
	ObservabilityRequestLogBackend string
	ClickHouseDSN                  string
	ClickHouseDriver               string
	ObservabilityHTTPAlertsEnabled bool
	AlertWebhookURL                string
	AlertWebhookSecret             string

	// HTTP recovery currently records restart actions for 5xx alert audits. It
	// does not execute infrastructure mutations.
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

	sessionCookieName := strings.TrimSpace(os.Getenv("SESSION_COOKIE_NAME"))
	if sessionCookieName == "" {
		sessionCookieName = "oblivious_session"
	}

	sessionCookieSecure := strings.EqualFold(strings.TrimSpace(os.Getenv("SESSION_COOKIE_SECURE")), "true")
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

	// Relay configuration
	relayEnabled := true
	if relayEnabledRaw := strings.TrimSpace(os.Getenv("RELAY_ENABLED")); relayEnabledRaw != "" {
		relayEnabled = strings.EqualFold(relayEnabledRaw, "true")
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
	clickHouseDriver := strings.TrimSpace(os.Getenv("CLICKHOUSE_DRIVER"))
	if clickHouseDriver == "" {
		clickHouseDriver = "clickhouse"
	}
	observabilityHTTPAlertsEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("OBSERVABILITY_HTTP_ALERTS_ENABLED")), "true")
	alertWebhookURL := strings.TrimSpace(os.Getenv("ALERT_WEBHOOK_URL"))
	alertWebhookSecret := strings.TrimSpace(os.Getenv("ALERT_WEBHOOK_SECRET"))
	observabilityHTTPRecoveryEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("OBSERVABILITY_HTTP_RECOVERY_ENABLED")), "true")
	observabilityHTTPRecoveryCooldownMS := 300000
	observabilityHTTPRecoveryCooldownRaw := strings.TrimSpace(os.Getenv("OBSERVABILITY_HTTP_RECOVERY_COOLDOWN_MS"))
	if observabilityHTTPRecoveryCooldownRaw != "" {
		parsedCooldown, parseErr := strconv.Atoi(observabilityHTTPRecoveryCooldownRaw)
		if parseErr != nil || parsedCooldown < 1 {
			return Config{}, fmt.Errorf("invalid OBSERVABILITY_HTTP_RECOVERY_COOLDOWN_MS: %q", observabilityHTTPRecoveryCooldownRaw)
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
		Port:                         port,
		Env:                          env,
		CORSAllowedOrigins:           origins,
		DatabaseURL:                  databaseURL,
		SessionCookieName:            sessionCookieName,
		SessionCookieSecure:          sessionCookieSecure,
		SessionSecret:                sessionSecret,
		LLMBaseURL:                   llmBaseURL,
		LLMAPIKey:                    llmAPIKey,
		LLMTimeoutMS:                 llmTimeoutMS,
		ModelDefaultName:             modelDefaultName,
		AgentRelayBaseURL:            agentRelayBaseURL,
		AgentWebSearchProvider:       agentWebSearchProvider,
		AgentWebSearchFallback:       agentWebSearchFallback,
		AgentWebSearchEndpoint:       agentWebSearchEndpoint,
		AgentWebSearchAPIKey:         agentWebSearchAPIKey,
		AgentWebSearchResultLimit:    agentWebSearchResultLimit,
		AgentWebSearchGoogleCSEID:    agentWebSearchGoogleCSEID,
		QdrantURL:                    qdrantURL,
		QdrantAPIKey:                 qdrantAPIKey,
		QdrantVectorSize:             qdrantVectorSize,
		RAGRerankerBaseURL:           ragRerankerBaseURL,
		RAGRerankerAPIKey:            ragRerankerAPIKey,
		RAGRerankerModel:             ragRerankerModel,
		RAGRerankerTopK:              ragRerankerTopK,
		RelayEnabled:                 relayEnabled,
		RelayDefaultModel:            relayDefaultModel,
		RelayRateLimitBackend:        relayRateLimitBackend,
		RelayRateLimitRedisKeyPrefix: relayRateLimitRedisKeyPrefix,
		RelaySemanticCacheBackend:    relaySemanticCacheBackend,
		RedisAddr:                    redisAddr,
		RedisPassword:                redisPassword,
		RedisDB:                      redisDB,
		OpenAIAPIKey:                 openaiAPIKey,
		OpenAIBaseURL:                openaiBaseURL,
		StripeSecretKey:              stripeSecretKey,
		StripeSuccessURL:             stripeSuccessURL,
		StripeCancelURL:              stripeCancelURL,
		StripeWebhookSecret:          stripeWebhookSecret,
		AlipayCheckoutBaseURL:        alipayCheckoutBaseURL,
		AlipayWebhookSecret:          alipayWebhookSecret,
		WeChatPayCheckoutBaseURL:     weChatPayCheckoutBaseURL,
		WeChatPayWebhookSecret:       weChatPayWebhookSecret,

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

		ObservabilityRequestLogBackend: observabilityRequestLogBackend,
		ClickHouseDSN:                  clickHouseDSN,
		ClickHouseDriver:               clickHouseDriver,
		ObservabilityHTTPAlertsEnabled: observabilityHTTPAlertsEnabled,
		AlertWebhookURL:                alertWebhookURL,
		AlertWebhookSecret:             alertWebhookSecret,

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
