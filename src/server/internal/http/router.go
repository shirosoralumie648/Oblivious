package http

import (
	"context"
	"database/sql"
	"fmt"
	stdhttp "net/http"
	"strings"
	"time"

	"oblivious/server/internal/admin"
	"oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
	publishingchannel "oblivious/server/internal/channel"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/config"
	"oblivious/server/internal/console"
	"oblivious/server/internal/knowledge"
	"oblivious/server/internal/knowledge/retrieval"
	"oblivious/server/internal/marketplace"
	"oblivious/server/internal/mcp"
	"oblivious/server/internal/memory"
	"oblivious/server/internal/notification"
	"oblivious/server/internal/observability"
	"oblivious/server/internal/payment"
	"oblivious/server/internal/quota"
	"oblivious/server/internal/relay"
	"oblivious/server/internal/schedule"
	stripebilling "oblivious/server/internal/stripe"
	"oblivious/server/internal/task"
	"oblivious/server/internal/tenant"
	"oblivious/server/internal/usage"
	"oblivious/server/internal/userprefs"
	"oblivious/server/internal/workflow"
	"oblivious/server/internal/ws"
)

func NewRouter(cfg config.Config, database *sql.DB) stdhttp.Handler {
	return NewRouterWithOptions(cfg, database, RouterOptions{})
}

type RouterOptions struct {
	CheckoutCreator             stripebilling.CheckoutCreator
	CheckoutCreators            map[string]stripebilling.CheckoutCreator
	PaymentProviderRegistry     *payment.Registry
	RelayPricingStore           *relay.PricingStore
	ChannelRuntimeStatsProvider admin.ChannelRuntimeStatsProvider
	RelayConfigApplier          admin.RelayConfigApplier
	MarketplacePayoutProvider   marketplace.MarketplacePayoutProvider
	AdminService                *admin.Service
	WorkflowService             *workflow.Service
	ScheduleService             *schedule.Service
	AlertStateStore             observability.AlertStateStore
	AlertRoutingRuleStore       observability.AlertRoutingRuleStore
	AlertProviderConfigStore    observability.AlertProviderConfigStore
	AuthStore                   auth.Store
	AdminQuotaSettingsService   adminQuotaSettingsService
}

type stripeMarketplaceSettlementAdapter struct {
	service *marketplace.SettlementService
}

func (a stripeMarketplaceSettlementAdapter) ApplyPaidInstallCheckoutCompleted(ctx context.Context, input stripebilling.MarketplaceCheckoutCompleted) error {
	_, err := a.service.ApplyPaidInstallCheckoutCompleted(ctx, marketplace.PaidInstallCheckoutCompleted{
		EventID:                   input.EventID,
		OrderID:                   input.OrderID,
		PaymentIntentID:           input.PaymentIntentID,
		ProviderCheckoutSessionID: input.ProviderCheckoutSessionID,
		ProviderPaymentIntentID:   input.ProviderPaymentIntentID,
	})
	return err
}

func (a stripeMarketplaceSettlementAdapter) ApplyMarketplaceRefund(ctx context.Context, input stripebilling.MarketplaceRefund) error {
	return a.service.ApplyMarketplaceRefund(ctx, marketplace.MarketplaceRefund{
		EventID:                 input.EventID,
		ProviderRefundID:        input.ProviderRefundID,
		PaymentIntentID:         input.PaymentIntentID,
		ProviderPaymentIntentID: input.ProviderPaymentIntentID,
		Amount:                  input.Amount,
		Currency:                input.Currency,
		Reason:                  input.Reason,
	})
}

func NewRouterWithOptions(cfg config.Config, database *sql.DB, options RouterOptions) stdhttp.Handler {
	mux := stdhttp.NewServeMux()
	notificationService := notification.NewService(notification.NewSQLStore(database))
	workflowService := options.WorkflowService
	if workflowService == nil {
		workflowService = newConfiguredWorkflowServiceWithStoreNotifierAndAlerts(cfg, workflow.NewSQLStore(database), notificationService, currentHTTPAlertSink())
	}
	mux.HandleFunc("/healthz", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		writeJSON(w, stdhttp.StatusOK, map[string]string{"status": "ok"})
	})
	// workflowMetricsHandler calls RefreshExecutionHealthMetrics before serving the Prometheus scrape.
	mux.Handle("/metrics", workflowMetricsHandler(workflowService))

	authStore := options.AuthStore
	if authStore == nil {
		authStore = auth.NewSQLStore(database)
	}
	authService := auth.NewService(authStore)
	authMiddleware := newAuthMiddleware(cfg, authService)
	preferencesService := userprefs.NewService(userprefs.NewSQLStore(database))
	authHandler := newAuthHandler(authService, authMiddleware, preferencesService)

	relayGateway := chat.NewRelayGateway(
		chat.WithRelayURL("http://localhost:"+fmt.Sprintf("%d", cfg.Port)+"/v1"),
		chat.WithDefaultModel(cfg.RelayDefaultModel),
	)
	var replyGenerator chat.ReplyGenerator
	var agentGateway chat.ChatGateway
	if cfg.RelayEnabled {
		var gateway chat.ChatGateway = relayGateway
		if cfg.Env != "production" {
			localGenerator := chat.NewHTTPReplyGenerator("", "", cfg.ModelDefaultName, time.Duration(cfg.LLMTimeoutMS)*time.Millisecond)
			gateway = chat.NewCompositeGateway(relayGateway, localGenerator)
		}
		replyGenerator = gateway
		agentGateway = gateway
	} else {
		localGenerator := chat.NewHTTPReplyGenerator("", "", cfg.ModelDefaultName, time.Duration(cfg.LLMTimeoutMS)*time.Millisecond)
		replyGenerator = localGenerator
		agentGateway = chat.NewLocalGateway(localGenerator)
	}
	chatService := chat.NewService(chat.NewSQLStore(database), replyGenerator, cfg.ModelDefaultName, usage.NewSQLRecorder(database))
	chatHandler := newChatHandler(chatService)
	knowledgeStore := knowledge.NewSQLStore(database)
	knowledgeService := newKnowledgeService(cfg, knowledgeStore)
	knowledgeHandler := newKnowledgeHandler(knowledgeService)
	preferencesHandler := newPreferencesHandler(preferencesService)
	taskHandler := newTaskHandler(task.NewService(task.NewSQLStore(database)))

	agentService := agent.NewService(agent.NewSQLStore(database), agentGateway)
	agentHandler := newAgentHandler(agentService)

	// Memory service with Relay embedder
	var memoryService *memory.Service
	if cfg.RelayEnabled {
		embedder := memory.NewRelayEmbedder(
			"http://localhost:"+fmt.Sprintf("%d", cfg.Port)+"/v1",
			"text-embedding-3-small",
		)
		chunker := memory.DefaultChunker()
		memoryService = memory.NewService(memory.NewSQLStore(database), embedder, chunker, "text-embedding-3-small")
		// Inject memory into agent service
		agentService.SetMemory(memoryService)
	}
	memoryHandler := newMemoryHandler(memoryService)
	agentMemoryHandler := newAgentMemoriesHandler(agentService)
	agentRunsHandler := newAgentRunsHandler(agentService)

	// MCP client
	mcpClient := mcp.NewClient(mcp.NewSQLStore(database))
	mcpHandler := newMCPHandler(mcpClient)

	// Inject MCP client into agent service
	agentService.SetMCPClient(mcpClient)

	// Quota service
	quotaService := quota.NewService(quota.NewSQLStore(database))
	quotaHandler := newQuotaHandler(quotaService)
	marketplaceStore := marketplace.NewSQLStore(database)
	marketplaceSettlementOptions := []marketplace.SettlementServiceOption{}
	if options.MarketplacePayoutProvider != nil {
		marketplaceSettlementOptions = append(marketplaceSettlementOptions, marketplace.WithMarketplacePayoutProvider(options.MarketplacePayoutProvider))
	}
	marketplaceSettlementService := marketplace.NewSettlementService(marketplaceStore, marketplaceSettlementOptions...)
	checkoutCreator := options.CheckoutCreator
	if checkoutCreator == nil {
		checkoutCreator = stripebilling.CheckoutCreatorFunc(stripebilling.CreateCheckoutSession)
	}
	paymentProviderRegistry, checkoutCreators := buildPaymentCheckoutProviders(cfg, checkoutCreator, options.PaymentProviderRegistry, options.CheckoutCreators)
	consoleHandler := newConsoleHandler(
		console.NewServiceWithAPITokens(
			console.NewSQLStore(database),
			relay.NewRelayAPITokenSQLStore(database),
			console.WithBillingPaymentProviders(consoleBillingPaymentProviders(paymentProviderRegistry, checkoutCreators)),
		),
		preferencesService,
	)
	billingHandler := newBillingHandler(checkoutCreator, stripebilling.CheckoutConfig{
		SecretKey:     cfg.StripeSecretKey,
		SuccessURL:    cfg.StripeSuccessURL,
		CancelURL:     cfg.StripeCancelURL,
		WebhookSecret: cfg.StripeWebhookSecret,
	}, stripebilling.NewSQLPaymentIntentStore(database), quotaService, paymentProviderRegistry, checkoutCreators)
	billingLifecycleService := stripebilling.NewLifecycleService(
		stripebilling.NewSQLLifecycleStore(database),
		stripebilling.WithMarketplaceSettlementApplier(stripeMarketplaceSettlementAdapter{service: marketplaceSettlementService}),
	)
	stripeWebhookHandler := stripebilling.NewWebhookHandler(stripebilling.NewSQLWebhookLedger(database), cfg.StripeWebhookSecret, billingLifecycleService)
	paymentWebhookLedger := stripebilling.NewSQLWebhookLedger(database)
	domesticPaymentLifecycle := stripeDomesticPaymentLifecycleAdapter{service: billingLifecycleService, settlementService: marketplaceSettlementService}
	alipayWebhookHandler := newDomesticPaymentWebhookHandler("alipay", cfg.AlipayWebhookSecret, paymentWebhookLedger, domesticPaymentLifecycle)
	weChatPayWebhookHandler := newDomesticPaymentWebhookHandler("wechatpay", cfg.WeChatPayWebhookSecret, paymentWebhookLedger, domesticPaymentLifecycle)

	adminOptions := []admin.ServiceOption{
		admin.WithChannelRuntimeStatsProvider(options.ChannelRuntimeStatsProvider),
		admin.WithRelayConfigApplier(options.RelayConfigApplier),
	}
	if options.RelayPricingStore != nil {
		adminOptions = append(adminOptions, admin.WithRelayPricingSettingsApplier(func(settings admin.RelayPricingSettings) {
			options.RelayPricingStore.ApplyMultipliers(settings.ModelMultipliers, settings.GroupMultipliers)
		}))
	}
	adminService := options.AdminService
	if adminService == nil {
		adminService = admin.NewService(admin.NewSQLStore(database), adminOptions...)
	}
	tenantHandler := newTenantHandler(tenant.NewService(tenant.NewSQLStore(database)), authService, authMiddleware)
	sensitiveActionRateLimit := auth.RateLimitPolicy{Limit: 5, Window: time.Minute, BlockDuration: 15 * time.Minute}

	// Marketplace service
	marketplaceGovernanceService := marketplace.NewGovernanceService(marketplaceStore)
	marketplaceService := marketplace.NewService(
		marketplaceStore,
		adminService,
		marketplace.WithAutomatedReview(marketplaceGovernanceService),
		marketplace.WithReviewSLAAlertSink(currentHTTPAlertSink()),
	)
	adminQuotaSettingsService := options.AdminQuotaSettingsService
	if adminQuotaSettingsService == nil {
		adminQuotaSettingsService = quotaService
	}
	adminHandler := newAdminHandlerWithQuotaPayoutsAndReviewSLA(adminService, adminQuotaSettingsService, marketplaceSettlementService, marketplaceService)
	marketplaceHandler := newMarketplaceHandler(
		marketplaceService,
		marketplace.NewSearchService(database),
		withMarketplaceCheckout(marketplaceSettlementService, checkoutCreator, stripebilling.CheckoutConfig{
			SecretKey:     cfg.StripeSecretKey,
			SuccessURL:    cfg.StripeSuccessURL,
			CancelURL:     cfg.StripeCancelURL,
			WebhookSecret: cfg.StripeWebhookSecret,
		}, paymentProviderRegistry, checkoutCreators),
		withMarketplaceGovernance(marketplaceGovernanceService),
	)

	// Notification service
	notificationHandler := newNotificationHandler(notificationService)
	channelStore := publishingchannel.NewSQLStore(database)
	channelHandler := newChannelHandler(channelStore, publishingchannel.NewServiceWithOptions(
		publishingchannel.NewAdapterRegistry(nil),
		publishingchannel.WithChannelHealthNotifier(publishingChannelHealthNotifier),
	))
	workflowHandler := newWorkflowHandler(newWorkflowServiceAdapter(workflowService))
	chatService.SetSemanticWorkflowTriggerer(workflowSemanticTriggerDispatcher{service: newWorkflowServiceAdapter(workflowService)})
	scheduleService := options.ScheduleService
	if scheduleService == nil {
		scheduleService = newScheduleService(schedule.NewSQLStore(database), workflowService, agentService)
	}
	scheduleHandler := newScheduleHandler(scheduleService)
	alertStateStore := options.AlertStateStore
	if alertStateStore == nil {
		alertStateStore = observability.NewSQLAlertStateStore(database)
	}
	alertRoutingRuleStore := options.AlertRoutingRuleStore
	if alertRoutingRuleStore == nil {
		alertRoutingRuleStore = observability.NewSQLAlertRoutingRuleStore(database)
	}
	alertProviderConfigStore := options.AlertProviderConfigStore
	if alertProviderConfigStore == nil {
		alertProviderConfigStore = observability.NewSQLAlertProviderConfigStore(database)
	}
	observabilityAlertHandler := newObservabilityAlertHandlerWithStores(alertStateStore, alertRoutingRuleStore, alertProviderConfigStore)

	registerAuthRoutes(mux, authMiddleware, authHandler)
	registerKnowledgeRoutes(mux, authMiddleware, knowledgeHandler)
	registerKnowledgeAliasRoutes(mux, authMiddleware, knowledgeHandler)
	registerAgentMemoryRoutes(mux, authMiddleware, agentMemoryHandler)
	registerAgentRunRoutes(mux, authMiddleware, agentRunsHandler)
	registerWorkflowRoutes(mux, authMiddleware, workflowHandler)
	registerScheduleRoutes(mux, authMiddleware, scheduleHandler)
	registerPublishingChannelRoutes(mux, authMiddleware, channelHandler)
	registerObservabilityAlertRoutes(mux, authMiddleware, observabilityAlertHandler)
	registerConsoleRoutes(mux, authMiddleware, consoleHandler)
	registerChatRoutes(mux, authMiddleware, chatHandler)
	registerConversationAliasRoutes(mux, authMiddleware, chatHandler)

	// Preferences routes
	mux.Handle("/api/v1/app/me/preferences", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			preferencesHandler.get(w, r)
		case stdhttp.MethodPut:
			preferencesHandler.update(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))

	// Task routes
	mux.Handle("/api/v1/app/tasks", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			taskHandler.listTasks(w, r)
		case stdhttp.MethodPost:
			taskHandler.createTask(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/app/tasks/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/v1/app/tasks/")
		parts := strings.Split(trimmedPath, "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		taskID := parts[0]
		if len(parts) == 1 {
			if r.Method == stdhttp.MethodGet {
				taskHandler.getTask(w, r, taskID)
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 {
			switch parts[1] {
			case "start":
				if r.Method == stdhttp.MethodPost {
					taskHandler.startTask(w, r, taskID)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "approve":
				if r.Method == stdhttp.MethodPost {
					taskHandler.approveTask(w, r, taskID)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "pause":
				if r.Method == stdhttp.MethodPost {
					taskHandler.pauseTask(w, r, taskID)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "resume":
				if r.Method == stdhttp.MethodPost {
					taskHandler.resumeTask(w, r, taskID)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "cancel":
				if r.Method == stdhttp.MethodPost {
					taskHandler.cancelTask(w, r, taskID)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "budget":
				if r.Method == stdhttp.MethodPost {
					taskHandler.updateTaskBudget(w, r, taskID)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			default:
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			}
			return
		}

		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))

	// Agent routes
	mux.Handle("/api/v1/app/agents", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			agentHandler.listAgents(w, r)
		case stdhttp.MethodPost:
			agentHandler.createAgent(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/app/agents/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/v1/app/agents/")
		parts := strings.Split(trimmedPath, "/")

		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		if parts[0] == "runs" && len(parts) == 2 && parts[1] != "" {
			if r.Method == stdhttp.MethodGet {
				agentHandler.getRun(w, r, parts[1])
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if parts[0] == "tool-runs" && len(parts) == 3 && parts[1] != "" {
			switch parts[2] {
			case "approve":
				if r.Method == stdhttp.MethodPost {
					agentHandler.approveToolRun(w, r, parts[1])
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "reject":
				if r.Method == stdhttp.MethodPost {
					agentHandler.rejectToolRun(w, r, parts[1])
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "retry":
				if r.Method == stdhttp.MethodPost {
					agentHandler.retryToolRun(w, r, parts[1])
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			default:
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			}
			return
		}

		agentID := parts[0]

		if len(parts) == 1 {
			switch r.Method {
			case stdhttp.MethodGet:
				agentHandler.getAgent(w, r, agentID)
			case stdhttp.MethodPut:
				agentHandler.updateAgent(w, r, agentID)
			case stdhttp.MethodDelete:
				agentHandler.deleteAgent(w, r, agentID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "conversations" {
			switch r.Method {
			case stdhttp.MethodGet:
				agentHandler.listConversations(w, r, agentID)
			case stdhttp.MethodPost:
				agentHandler.createConversation(w, r, agentID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "tools" {
			if r.Method == stdhttp.MethodGet {
				agentHandler.listAvailableTools(w, r, agentID)
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))

	// Agent conversation routes
	mux.Handle("/api/v1/app/agents/conversations/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/v1/app/agents/conversations/")
		parts := strings.Split(trimmedPath, "/")

		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		conversationID := parts[0]

		if len(parts) == 1 {
			switch r.Method {
			case stdhttp.MethodGet:
				agentHandler.getConversation(w, r, conversationID)
			case stdhttp.MethodDelete:
				agentHandler.deleteConversation(w, r, conversationID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "messages" {
			switch r.Method {
			case stdhttp.MethodGet:
				agentHandler.listMessages(w, r, conversationID)
			case stdhttp.MethodPost:
				agentHandler.sendMessage(w, r, conversationID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "runs" {
			if r.Method == stdhttp.MethodGet {
				agentHandler.listRuns(w, r, conversationID)
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))

	// Memory routes
	mux.Handle("/api/v1/app/memory/documents", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			memoryHandler.listDocuments(w, r)
		case stdhttp.MethodPost:
			memoryHandler.addDocument(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/app/memory/documents/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/v1/app/memory/documents/")
		parts := strings.Split(trimmedPath, "/")

		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		documentID := parts[0]

		if len(parts) == 1 {
			switch r.Method {
			case stdhttp.MethodGet:
				memoryHandler.getDocument(w, r, documentID)
			case stdhttp.MethodPut:
				memoryHandler.updateDocument(w, r, documentID)
			case stdhttp.MethodDelete:
				memoryHandler.deleteDocument(w, r, documentID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "chunks" {
			if r.Method == stdhttp.MethodGet {
				memoryHandler.listChunks(w, r, documentID)
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))
	mux.Handle("/api/v1/app/memory/search", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		memoryHandler.search(w, r)
	})))

	// MCP Server routes
	mux.Handle("/api/v1/app/mcp-servers", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			mcpHandler.listServers(w, r)
		case stdhttp.MethodPost:
			mcpHandler.addServer(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/app/mcp-servers/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/v1/app/mcp-servers/")
		parts := strings.Split(trimmedPath, "/")

		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		serverID := parts[0]

		if len(parts) == 1 {
			switch r.Method {
			case stdhttp.MethodGet:
				mcpHandler.getServer(w, r, serverID)
			case stdhttp.MethodDelete:
				mcpHandler.deleteServer(w, r, serverID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 {
			switch parts[1] {
			case "connect":
				if r.Method == stdhttp.MethodPost {
					mcpHandler.connectServer(w, r, serverID)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "disconnect":
				if r.Method == stdhttp.MethodPost {
					mcpHandler.disconnectServer(w, r, serverID)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "tools":
				if r.Method == stdhttp.MethodGet {
					mcpHandler.listServerTools(w, r, serverID)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "status":
				if r.Method == stdhttp.MethodGet {
					mcpHandler.getServerStatus(w, r, serverID)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "execute":
				if r.Method == stdhttp.MethodPost {
					mcpHandler.executeTool(w, r, serverID)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			default:
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			}
			return
		}

		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))

	// Quota routes
	mux.Handle("/api/v1/app/quota", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		quotaHandler.getQuota(w, r)
	})))
	mux.Handle("/api/v1/app/packages", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		quotaHandler.listPackages(w, r)
	})))
	mux.Handle("/api/v1/app/quota/topup", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		quotaHandler.topup(w, r)
	})))
	mux.Handle("/api/v1/billing/checkout", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		billingHandler.checkout(w, r)
	})))
	mux.HandleFunc("/api/v1/billing/stripe/webhook", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		stripeWebhookHandler.HandleWebhook(w, r)
	})
	mux.HandleFunc("/api/v1/billing/alipay/webhook", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		alipayWebhookHandler.handle(w, r)
	})
	mux.HandleFunc("/api/v1/billing/wechatpay/webhook", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		weChatPayWebhookHandler.handle(w, r)
	})

	// Notification routes
	mux.Handle("/api/v1/app/notifications", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			notificationHandler.list(w, r)
		case stdhttp.MethodPost:
			notificationHandler.create(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/app/notifications/unread-count", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		notificationHandler.getUnreadCount(w, r)
	})))
	mux.Handle("/api/v1/app/notifications/mark-all-read", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		notificationHandler.markAllRead(w, r)
	})))
	mux.Handle("/api/v1/app/notifications/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/app/notifications/")
		if id == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}
		switch r.Method {
		case stdhttp.MethodPatch:
			notificationHandler.markRead(w, r, id)
		case stdhttp.MethodDelete:
			notificationHandler.delete(w, r, id)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))

	// WebSocket route
	mux.HandleFunc("/api/v1/ws", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		session, ok := authMiddleware.currentSession(r)
		if !ok {
			writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		ws.ServeWS(ws.DefaultHub(), w, r, session.User.ID)
	})

	// Admin routes (require admin role)
	mux.Handle("/api/v1/admin/stats", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		adminHandler.getStats(w, r)
	})))
	mux.Handle("/api/v1/admin/api-tokens", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		adminHandler.listAPITokens(w, r)
	})))
	mux.Handle("/api/v1/admin/api-tokens/", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/api-tokens/"), "/"), "/")
		if len(parts) == 2 && parts[0] != "" && parts[1] == "revoke" {
			if r.Method != stdhttp.MethodPost {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			adminHandler.revokeAPIToken(w, r, parts[0])
			return
		}
		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))
	mux.Handle("/api/v1/admin/settings/relay-pricing", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			adminHandler.getRelayPricingSettings(w, r)
		case stdhttp.MethodPut:
			adminHandler.updateRelayPricingSettings(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/admin/settings/usage-limits", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			adminHandler.listUsageLimitSettings(w, r)
		case stdhttp.MethodPut:
			adminHandler.updateUsageLimitSettings(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/admin/billing/summary", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		adminHandler.getBillingSummary(w, r)
	})))
	mux.Handle("/api/v1/admin/billing/", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		surface := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/billing/"), "/")
		switch surface {
		case "sessions":
			adminHandler.listBillingSessions(w, r)
		case "payment-intents":
			adminHandler.listPaymentIntents(w, r)
		case "webhook-events":
			adminHandler.listWebhookEvents(w, r)
		case "subscriptions":
			adminHandler.listSubscriptions(w, r)
		case "topups":
			adminHandler.listTopups(w, r)
		case "invoices":
			adminHandler.listInvoices(w, r)
		case "refunds":
			adminHandler.listRefunds(w, r)
		case "settlements":
			adminHandler.listMarketplaceSettlements(w, r)
		case "payouts":
			adminHandler.listMarketplacePayouts(w, r)
		default:
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
		}
	})))
	mux.Handle("/api/v1/admin/billing/payouts", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		adminHandler.listMarketplacePayouts(w, r)
	})))
	mux.Handle("/api/v1/admin/billing/topups", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		adminHandler.listTopups(w, r)
	})))
	mux.Handle("/api/v1/admin/billing/topups/", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/billing/topups/"), "/"), "/")
		if len(parts) == 2 && parts[0] != "" && parts[1] == "refund" {
			if r.Method != stdhttp.MethodPost {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			adminHandler.recordTopupRefund(w, r, parts[0])
			return
		}
		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))
	mux.Handle("/api/v1/admin/billing/payouts/", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/billing/payouts/"), "/"), "/")
		if len(parts) == 1 && parts[0] == "create-due" {
			if r.Method != stdhttp.MethodPost {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			adminHandler.createDueMarketplacePayouts(w, r)
			return
		}
		if len(parts) == 2 && parts[0] != "" && parts[1] == "paid" {
			if r.Method != stdhttp.MethodPost {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			adminHandler.markMarketplacePayoutPaid(w, r, parts[0])
			return
		}
		if len(parts) == 2 && parts[0] != "" && parts[1] == "failed" {
			if r.Method != stdhttp.MethodPost {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			adminHandler.markMarketplacePayoutFailed(w, r, parts[0])
			return
		}
		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))
	mux.Handle("/api/v1/admin/channel-providers", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		adminHandler.listChannelProviders(w, r)
	})))
	mux.Handle("/api/v1/admin/channels", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			adminHandler.listChannels(w, r)
		case stdhttp.MethodPost:
			adminHandler.createChannel(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/admin/channels/batch", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		adminHandler.batchUpdateChannels(w, r)
	})))
	mux.Handle("/api/v1/admin/channels/stats", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		adminHandler.listChannelRuntimeStats(w, r)
	})))
	mux.Handle("/api/v1/admin/channels/", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/channels/"), "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		channelID := parts[0]
		if len(parts) == 1 {
			switch r.Method {
			case stdhttp.MethodGet:
				adminHandler.getChannel(w, r, channelID)
			case stdhttp.MethodPut:
				adminHandler.updateChannel(w, r, channelID)
			case stdhttp.MethodDelete:
				adminHandler.deleteChannel(w, r, channelID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		switch parts[1] {
		case "test":
			if r.Method == stdhttp.MethodPost {
				adminHandler.testChannel(w, r, channelID)
				return
			}
		case "health":
			if r.Method == stdhttp.MethodGet {
				adminHandler.getChannelHealth(w, r, channelID)
				return
			}
		case "sync-models":
			if r.Method == stdhttp.MethodPost {
				adminHandler.syncChannelModels(w, r, channelID)
				return
			}
		case "model-updates":
			if len(parts) == 3 && r.Method == stdhttp.MethodPost {
				switch parts[2] {
				case "detect":
					adminHandler.detectChannelModelUpdates(w, r, channelID)
					return
				case "apply":
					adminHandler.applyChannelModelUpdates(w, r, channelID)
					return
				}
			}
		case "refresh-balance":
			if r.Method == stdhttp.MethodPost {
				adminHandler.refreshChannelBalance(w, r, channelID)
				return
			}
		}
		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))
	mux.Handle("/api/v1/admin/routes", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			adminHandler.listRoutes(w, r)
		case stdhttp.MethodPost:
			adminHandler.createRoute(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/admin/routes/", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		routeID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/routes/"), "/")
		if routeID == "" || strings.Contains(routeID, "/") {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}
		switch r.Method {
		case stdhttp.MethodGet:
			adminHandler.getRoute(w, r, routeID)
		case stdhttp.MethodPut:
			adminHandler.updateRoute(w, r, routeID)
		case stdhttp.MethodDelete:
			adminHandler.deleteRoute(w, r, routeID)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/admin/plans", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			adminHandler.listPlans(w, r)
		case stdhttp.MethodPost:
			adminHandler.createPlan(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/admin/plans/", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		planID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/plans/"), "/")
		if planID == "" || strings.Contains(planID, "/") {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}
		switch r.Method {
		case stdhttp.MethodGet:
			adminHandler.getPlan(w, r, planID)
		case stdhttp.MethodPut:
			adminHandler.updatePlan(w, r, planID)
		case stdhttp.MethodDelete:
			adminHandler.deactivatePlan(w, r, planID)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/admin/users", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			adminHandler.listUsers(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/admin/users/", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users/"), "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		userID := parts[0]
		if len(parts) == 1 {
			switch r.Method {
			case stdhttp.MethodGet:
				adminHandler.getUser(w, r, userID)
			case stdhttp.MethodPut:
				adminHandler.updateUser(w, r, userID)
			case stdhttp.MethodPatch:
				adminHandler.updateUserQuota(w, r, userID)
			case stdhttp.MethodDelete:
				adminHandler.deleteUser(w, r, userID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		switch parts[1] {
		case "disable":
			if r.Method == stdhttp.MethodPost {
				adminHandler.disableUser(w, r, userID)
				return
			}
		case "enable":
			if r.Method == stdhttp.MethodPost {
				adminHandler.enableUser(w, r, userID)
				return
			}
		}
		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))
	mux.Handle("/api/v1/admin/organizations", authMiddleware.requireAdmin(authMiddleware.rateLimit("sensitive.admin", sensitiveActionRateLimit, stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			tenantHandler.listOrganizations(w, r)
		case stdhttp.MethodPost:
			tenantHandler.createOrganization(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}))))
	mux.Handle("/api/v1/admin/organizations/", authMiddleware.requireAdmin(authMiddleware.rateLimit("sensitive.admin", sensitiveActionRateLimit, stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/organizations/"), "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		organizationID := parts[0]
		if len(parts) == 1 {
			switch r.Method {
			case stdhttp.MethodGet:
				tenantHandler.getOrganization(w, r, organizationID)
			case stdhttp.MethodPut:
				tenantHandler.updateOrganization(w, r, organizationID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "archive" {
			if r.Method == stdhttp.MethodPost {
				tenantHandler.archiveOrganization(w, r, organizationID)
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "members" {
			if r.Method == stdhttp.MethodGet {
				tenantHandler.listOrganizationMembers(w, r, organizationID)
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	}))))
	mux.Handle("/api/v1/app/organizations", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			tenantHandler.listMyOrganizations(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/app/organizations/", authMiddleware.requireSession(authMiddleware.rateLimit("sensitive.organization", sensitiveActionRateLimit, stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/app/organizations/"), "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		organizationID := parts[0]
		if len(parts) == 2 && parts[1] == "select" {
			if r.Method == stdhttp.MethodPost {
				tenantHandler.selectOrganization(w, r, organizationID)
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}
		if len(parts) == 2 && parts[1] == "members" {
			if r.Method == stdhttp.MethodGet {
				tenantHandler.listOrganizationMembers(w, r, organizationID)
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}
		if len(parts) == 2 && parts[1] == "invitations" {
			if r.Method == stdhttp.MethodPost {
				tenantHandler.inviteMember(w, r, organizationID)
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}
		if len(parts) == 4 && parts[1] == "invitations" && parts[3] == "revoke" {
			if r.Method == stdhttp.MethodPost {
				tenantHandler.revokeInvitation(w, r, organizationID, parts[2])
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}
		if len(parts) == 3 && parts[1] == "members" {
			switch r.Method {
			case stdhttp.MethodPut:
				tenantHandler.updateMemberRole(w, r, organizationID, parts[2])
			case stdhttp.MethodDelete:
				tenantHandler.removeMember(w, r, organizationID, parts[2])
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}
		if len(parts) == 2 && parts[1] == "ownership-transfer" {
			if r.Method == stdhttp.MethodPost {
				tenantHandler.transferOwnership(w, r, organizationID)
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	}))))
	mux.Handle("/api/v1/app/organization-invitations/", authMiddleware.requireSession(authMiddleware.rateLimit("sensitive.organization", sensitiveActionRateLimit, stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/app/organization-invitations/"), "/"), "/")
		if len(parts) == 2 && parts[0] != "" && parts[1] == "accept" {
			if r.Method == stdhttp.MethodPost {
				tenantHandler.acceptInvitation(w, r, parts[0])
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}
		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	}))))
	mux.Handle("/api/v1/admin/audit-logs", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		adminHandler.listAuditLogs(w, r)
	})))
	mux.Handle("/api/v1/admin/reviews", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		adminHandler.listReviews(w, r)
	})))
	mux.Handle("/api/v1/admin/reviews/", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/reviews/"), "/"), "/")
		if len(parts) == 2 && parts[0] == "sla" && parts[1] == "enforce" {
			if r.Method == stdhttp.MethodPost {
				adminHandler.enforceReviewSLA(w, r)
				return
			}
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if len(parts) != 2 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		switch parts[1] {
		case "approve":
			if r.Method == stdhttp.MethodPost {
				adminHandler.approveAgent(w, r, parts[0])
				return
			}
		case "reject":
			if r.Method == stdhttp.MethodPost {
				adminHandler.rejectAgent(w, r, parts[0])
				return
			}
		case "needs-changes":
			if r.Method == stdhttp.MethodPost {
				adminHandler.needsChangesAgent(w, r, parts[0])
				return
			}
		}
		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))
	mux.Handle("/api/v1/admin/marketplace/agents/", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/marketplace/agents/"), "/"), "/")
		if len(parts) != 2 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}
		switch parts[1] {
		case "takedown":
			if r.Method == stdhttp.MethodPost {
				marketplaceHandler.takedownAgent(w, r, parts[0])
				return
			}
		case "reinstate":
			if r.Method == stdhttp.MethodPost {
				marketplaceHandler.reinstateAgent(w, r, parts[0])
				return
			}
		}
		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))
	mux.Handle("/api/v1/admin/marketplace/abuse-reports", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		marketplaceHandler.listAbuseReports(w, r)
	})))
	mux.Handle("/api/v1/admin/marketplace/abuse-reports/", authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/marketplace/abuse-reports/"), "/"), "/")
		if len(parts) != 2 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}
		switch parts[1] {
		case "resolve":
			if r.Method == stdhttp.MethodPost {
				marketplaceHandler.resolveAbuseReport(w, r, parts[0], "resolved")
				return
			}
		case "dismiss":
			if r.Method == stdhttp.MethodPost {
				marketplaceHandler.resolveAbuseReport(w, r, parts[0], "dismissed")
				return
			}
		}
		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))

	serveWithSession := func(w stdhttp.ResponseWriter, r *stdhttp.Request, handler stdhttp.HandlerFunc) {
		authMiddleware.requireSession(handler).ServeHTTP(w, r)
	}
	serveWithOptionalSession := func(w stdhttp.ResponseWriter, r *stdhttp.Request, handler stdhttp.HandlerFunc) {
		if session, ok := authMiddleware.currentSession(r); ok {
			attachSessionToObservabilityScope(r, session)
			r = r.WithContext(context.WithValue(r.Context(), sessionContextKey, session))
		}
		handler(w, r)
	}

	// Marketplace routes
	mux.HandleFunc("/api/v1/marketplace/featured", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		serveWithOptionalSession(w, r, marketplaceHandler.getFeaturedAgents)
	})
	mux.HandleFunc("/api/v1/marketplace/curated", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		marketplaceHandler.getCuratedSections(w, r)
	})
	mux.HandleFunc("/api/v1/marketplace/categories", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		marketplaceHandler.listCategories(w, r)
	})
	mux.HandleFunc("/api/v1/marketplace/search", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		serveWithOptionalSession(w, r, marketplaceHandler.searchAgents)
	})
	mux.HandleFunc("/api/v1/marketplace/templates", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			marketplaceHandler.listTemplates(w, r)
		case stdhttp.MethodPost:
			serveWithSession(w, r, marketplaceHandler.createTemplate)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})
	mux.HandleFunc("/api/v1/marketplace/templates/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/marketplace/templates/"), "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		templateID := parts[0]
		if len(parts) == 1 {
			if r.Method != stdhttp.MethodGet {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			marketplaceHandler.getTemplate(w, r, templateID)
			return
		}

		if len(parts) == 2 && parts[1] == "install" {
			if r.Method != stdhttp.MethodPost {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			serveWithSession(w, r, func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				marketplaceHandler.installTemplate(w, r, templateID)
			})
			return
		}

		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})
	mux.HandleFunc("/api/v1/marketplace/agents", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			marketplaceHandler.listAgents(w, r)
		case stdhttp.MethodPost:
			serveWithSession(w, r, marketplaceHandler.publishAgent)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})
	mux.HandleFunc("/api/v1/marketplace/my-agents", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		serveWithSession(w, r, marketplaceHandler.listMyAgents)
	})
	mux.HandleFunc("/api/v1/marketplace/installs", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		serveWithSession(w, r, marketplaceHandler.listInstalledAgents)
	})
	mux.HandleFunc("/api/v1/marketplace/installs/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		agentID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/marketplace/installs/"), "/")
		if agentID == "" || strings.Contains(agentID, "/") {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}
		if r.Method != stdhttp.MethodDelete {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		serveWithSession(w, r, func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			marketplaceHandler.uninstallAgent(w, r, agentID)
		})
	})
	mux.HandleFunc("/api/v1/marketplace/publisher/settlement-preferences", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			serveWithSession(w, r, marketplaceHandler.getPublisherSettlementPreferences)
		case stdhttp.MethodPut:
			serveWithSession(w, r, marketplaceHandler.updatePublisherSettlementPreferences)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})
	mux.HandleFunc("/api/v1/marketplace/publisher/stats", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		serveWithSession(w, r, marketplaceHandler.getPublisherStats)
	})
	mux.HandleFunc("/api/v1/marketplace/agents/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/marketplace/agents/"), "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		agentID := parts[0]
		if len(parts) == 1 {
			switch r.Method {
			case stdhttp.MethodGet:
				marketplaceHandler.getAgent(w, r, agentID)
			case stdhttp.MethodPut:
				serveWithSession(w, r, func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
					marketplaceHandler.updateAgent(w, r, agentID)
				})
			case stdhttp.MethodDelete:
				serveWithSession(w, r, func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
					marketplaceHandler.deleteAgent(w, r, agentID)
				})
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		switch parts[1] {
		case "install":
			switch r.Method {
			case stdhttp.MethodPost:
				serveWithSession(w, r, func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
					marketplaceHandler.installAgent(w, r, agentID)
				})
			case stdhttp.MethodDelete:
				serveWithSession(w, r, func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
					marketplaceHandler.uninstallAgent(w, r, agentID)
				})
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
		case "reviews":
			switch r.Method {
			case stdhttp.MethodGet:
				marketplaceHandler.listReviews(w, r, agentID)
			case stdhttp.MethodPost:
				serveWithSession(w, r, func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
					marketplaceHandler.submitReview(w, r, agentID)
				})
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
		case "appeal":
			if r.Method != stdhttp.MethodPost {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			serveWithSession(w, r, func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				marketplaceHandler.appealAgent(w, r, agentID)
			})
		case "abuse-reports":
			if r.Method != stdhttp.MethodPost {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			serveWithSession(w, r, func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				marketplaceHandler.reportAbuse(w, r, agentID)
			})
		case "versions":
			if r.Method != stdhttp.MethodGet {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			marketplaceHandler.getAgentVersions(w, r, agentID)
		case "stats":
			if r.Method != stdhttp.MethodGet {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			serveWithSession(w, r, func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				marketplaceHandler.getAgentStats(w, r, agentID)
			})
		default:
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
		}
	})

	return applyMiddleware(authMiddleware.securityGuard(mux), withRecover, withRequestID, withLogging, withCORS(cfg.CORSAllowedOrigins))
}

func newKnowledgeService(cfg config.Config, knowledgeStore any) *knowledge.Service {
	service := knowledge.NewService(knowledgeStore)
	if cfg.RelayEnabled {
		service = knowledge.NewServiceWithEmbedder(
			knowledgeStore,
			memory.NewRelayEmbedder(
				"http://localhost:"+fmt.Sprintf("%d", cfg.Port)+"/v1",
				"text-embedding-3-small",
			),
			"text-embedding-3-small",
		)
	}
	if strings.TrimSpace(cfg.QdrantURL) != "" {
		service.WithVectorStore(
			knowledge.NewQdrantVectorStore(cfg.QdrantURL, cfg.QdrantAPIKey),
			cfg.QdrantVectorSize,
		)
	}
	if strings.TrimSpace(cfg.RAGRerankerBaseURL) != "" {
		service.WithReranker(retrieval.NewKnowledgeResultReranker(knowledge.RerankerConfig{
			APIKey:  cfg.RAGRerankerAPIKey,
			BaseURL: cfg.RAGRerankerBaseURL,
			Model:   cfg.RAGRerankerModel,
			TopK:    cfg.RAGRerankerTopK,
		}))
	}
	return service
}
