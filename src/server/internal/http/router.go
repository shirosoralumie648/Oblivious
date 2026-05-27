package http

import (
	"database/sql"
	"fmt"
	stdhttp "net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"oblivious/server/internal/admin"
	"oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/config"
	"oblivious/server/internal/console"
	"oblivious/server/internal/knowledge"
	"oblivious/server/internal/marketplace"
	"oblivious/server/internal/mcp"
	"oblivious/server/internal/memory"
	"oblivious/server/internal/notification"
	"oblivious/server/internal/quota"
	"oblivious/server/internal/task"
	"oblivious/server/internal/tenant"
	"oblivious/server/internal/usage"
	"oblivious/server/internal/userprefs"
	"oblivious/server/internal/ws"
)

func NewRouter(cfg config.Config, database *sql.DB) stdhttp.Handler {
	mux := stdhttp.NewServeMux()
	mux.HandleFunc("/healthz", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		writeJSON(w, stdhttp.StatusOK, map[string]string{"status": "ok"})
	})
	mux.Handle("/metrics", promhttp.Handler())

	authService := auth.NewService(auth.NewSQLStore(database))
	authMiddleware := newAuthMiddleware(cfg, authService)
	preferencesService := userprefs.NewService(userprefs.NewSQLStore(database))
	authHandler := newAuthHandler(authService, authMiddleware, preferencesService)

	// Chat service with optional Relay gateway
	var chatService *chat.Service
	if cfg.RelayEnabled {
		relayGateway := chat.NewRelayGateway(
			chat.WithRelayURL("http://localhost:"+fmt.Sprintf("%d", cfg.Port)+"/v1"),
			chat.WithDefaultModel(cfg.RelayDefaultModel),
		)
		var gateway chat.ChatGateway = relayGateway
		if cfg.Env != "production" {
			localGenerator := chat.NewHTTPReplyGenerator(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.ModelDefaultName, time.Duration(cfg.LLMTimeoutMS)*time.Millisecond)
			gateway = chat.NewCompositeGateway(relayGateway, localGenerator)
		}
		chatService = chat.NewServiceWithGateway(chat.NewSQLStore(database), gateway, cfg.ModelDefaultName, usage.NewSQLRecorder(database))
	} else {
		replyGenerator := chat.NewHTTPReplyGenerator(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.ModelDefaultName, time.Duration(cfg.LLMTimeoutMS)*time.Millisecond)
		chatService = chat.NewService(chat.NewSQLStore(database), replyGenerator, cfg.ModelDefaultName, usage.NewSQLRecorder(database))
	}
	chatHandler := newChatHandler(chatService)
	consoleHandler := newConsoleHandler(console.NewService(console.NewSQLStore(database)), preferencesService)
	knowledgeHandler := newKnowledgeHandler(knowledge.NewService(knowledge.NewSQLStore(database)))
	preferencesHandler := newPreferencesHandler(preferencesService)
	taskHandler := newTaskHandler(task.NewService(task.NewSQLStore(database)))

	// Agent service with shared gateway
	var agentService *agent.Service
	if chatService.HasStreamSupport() {
		agentService = agent.NewService(agent.NewSQLStore(database), chatService.ChatGateway())
	} else {
		relayGateway := chat.NewRelayGateway(
			chat.WithRelayURL("http://localhost:"+fmt.Sprintf("%d", cfg.Port)+"/v1"),
			chat.WithDefaultModel(cfg.RelayDefaultModel),
		)
		agentService = agent.NewService(agent.NewSQLStore(database), relayGateway)
	}
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

	// MCP client
	mcpClient := mcp.NewClient(mcp.NewSQLStore(database))
	mcpHandler := newMCPHandler(mcpClient)

	// Inject MCP client into agent service
	agentService.SetMCPClient(mcpClient)

	// Quota service
	quotaService := quota.NewService(quota.NewSQLStore(database))
	quotaHandler := newQuotaHandler(quotaService)

	// Admin service
	adminService := admin.NewService(admin.NewSQLStore(database))
	adminHandler := newAdminHandler(adminService)
	tenantHandler := newTenantHandler(tenant.NewService(tenant.NewSQLStore(database)), authService, authMiddleware)
	sensitiveActionRateLimit := auth.RateLimitPolicy{Limit: 5, Window: time.Minute, BlockDuration: 15 * time.Minute}

	// Marketplace service
	marketplaceStore := marketplace.NewSQLStore(database)
	marketplaceHandler := newMarketplaceHandler(
		marketplace.NewService(marketplaceStore, adminService),
		marketplace.NewSearchService(database),
	)

	// Notification service
	notificationHandler := newNotificationHandler(notification.NewService(notification.NewSQLStore(database)))

	registerAuthRoutes(mux, authMiddleware, authHandler)

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

	// Models route
	mux.Handle("/api/v1/app/models", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		chatHandler.listModels(w, r)
	})))

	// Chat routes
	mux.Handle("/api/v1/app/conversations", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			chatHandler.listConversations(w, r)
		case stdhttp.MethodPost:
			chatHandler.createConversation(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/app/conversations/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/v1/app/conversations/")
		parts := strings.Split(trimmedPath, "/")
		if len(parts) != 2 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		conversationID := parts[0]
		switch parts[1] {
		case "messages":
			switch r.Method {
			case stdhttp.MethodGet:
				chatHandler.listMessages(w, r, conversationID)
			case stdhttp.MethodPost:
				chatHandler.sendMessage(w, r, conversationID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
		case "config":
			switch r.Method {
			case stdhttp.MethodGet:
				chatHandler.getConversationConfig(w, r, conversationID)
			case stdhttp.MethodPut:
				chatHandler.updateConversationConfig(w, r, conversationID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
		case "convert-to-task":
			if r.Method == stdhttp.MethodPost {
				chatHandler.convertConversationToTask(w, r, conversationID)
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
		default:
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
		}
	})))

	// Knowledge routes
	mux.Handle("/api/v1/app/knowledge-bases", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			knowledgeHandler.listKnowledgeBases(w, r)
		case stdhttp.MethodPost:
			knowledgeHandler.createKnowledgeBase(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/app/knowledge-bases/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/v1/app/knowledge-bases/")
		parts := strings.Split(trimmedPath, "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		knowledgeBaseID := parts[0]
		if len(parts) == 1 {
			switch r.Method {
			case stdhttp.MethodGet:
				knowledgeHandler.getKnowledgeBase(w, r, knowledgeBaseID)
			case stdhttp.MethodPut:
				knowledgeHandler.updateKnowledgeBase(w, r, knowledgeBaseID)
			case stdhttp.MethodDelete:
				knowledgeHandler.deleteKnowledgeBase(w, r, knowledgeBaseID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "documents" {
			switch r.Method {
			case stdhttp.MethodGet:
				knowledgeHandler.listKnowledgeDocuments(w, r, knowledgeBaseID)
			case stdhttp.MethodPost:
				knowledgeHandler.createKnowledgeDocument(w, r, knowledgeBaseID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "retrieve" {
			if r.Method == stdhttp.MethodPost {
				knowledgeHandler.retrieveKnowledge(w, r, knowledgeBaseID)
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 3 && parts[1] == "documents" && parts[2] != "" {
			documentID := parts[2]
			switch r.Method {
			case stdhttp.MethodPut:
				knowledgeHandler.updateKnowledgeDocument(w, r, knowledgeBaseID, documentID)
			case stdhttp.MethodDelete:
				knowledgeHandler.deleteKnowledgeDocument(w, r, knowledgeBaseID, documentID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
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

	// Console routes
	mux.Handle("/api/v1/console/usage", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		consoleHandler.getUsage(w, r)
	})))
	mux.Handle("/api/v1/console/access", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		consoleHandler.getAccess(w, r)
	})))
	mux.Handle("/api/v1/console/models", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		consoleHandler.getModels(w, r)
	})))
	mux.Handle("/api/v1/console/billing", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		consoleHandler.getBilling(w, r)
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
		}
		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))

	serveWithSession := func(w stdhttp.ResponseWriter, r *stdhttp.Request, handler stdhttp.HandlerFunc) {
		authMiddleware.requireSession(handler).ServeHTTP(w, r)
	}

	// Marketplace routes
	mux.HandleFunc("/api/v1/marketplace/featured", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		marketplaceHandler.getFeaturedAgents(w, r)
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
		marketplaceHandler.searchAgents(w, r)
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
