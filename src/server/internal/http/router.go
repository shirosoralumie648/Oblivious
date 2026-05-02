package http

import (
	"database/sql"
	stdhttp "net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"oblivious/server/internal/admin"
	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/config"
	"oblivious/server/internal/console"
	"oblivious/server/internal/knowledge"
	"oblivious/server/internal/marketplace"
	"oblivious/server/internal/task"
	"oblivious/server/internal/usage"
	"oblivious/server/internal/userprefs"
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
	replyGenerator := chat.NewHTTPReplyGenerator(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.ModelDefaultName, time.Duration(cfg.LLMTimeoutMS)*time.Millisecond)
	chatHandler := newChatHandler(chat.NewService(chat.NewSQLStore(database), replyGenerator, cfg.ModelDefaultName, usage.NewSQLRecorder(database)))
	consoleHandler := newConsoleHandler(console.NewService(console.NewSQLStore(database)), preferencesService)
	knowledgeHandler := newKnowledgeHandler(knowledge.NewService(knowledge.NewSQLStore(database)))
	preferencesHandler := newPreferencesHandler(preferencesService)
	taskHandler := newTaskHandler(task.NewService(task.NewSQLStore(database)))

	adminService := admin.NewService(admin.NewSQLStore(database))
	adminHandler := newAdminHandler(adminService)
	marketplaceStore := marketplace.NewSQLStore(database)
	marketplaceHandler := newMarketplaceHandler(
		marketplace.NewService(marketplaceStore, adminService),
		marketplace.NewSearchService(database),
	)

	mux.HandleFunc("/api/v1/auth/login", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		authHandler.login(w, r)
	})
	mux.HandleFunc("/api/v1/auth/register", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		authHandler.register(w, r)
	})
	mux.Handle("/api/v1/auth/me", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		authHandler.me(w, r)
	})))
	mux.Handle("/api/v1/auth/logout", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		authHandler.logout(w, r)
	})))
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
	mux.Handle("/api/v1/app/models", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		chatHandler.listModels(w, r)
	})))
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
			switch r.Method {
			case stdhttp.MethodPost:
				chatHandler.convertConversationToTask(w, r, conversationID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
		default:
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
		}
	})))
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
			switch r.Method {
			case stdhttp.MethodPost:
				knowledgeHandler.retrieveKnowledge(w, r, knowledgeBaseID)
			default:
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
			switch r.Method {
			case stdhttp.MethodGet:
				taskHandler.getTask(w, r, taskID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "start" {
			switch r.Method {
			case stdhttp.MethodPost:
				taskHandler.startTask(w, r, taskID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "approve" {
			switch r.Method {
			case stdhttp.MethodPost:
				taskHandler.approveTask(w, r, taskID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "pause" {
			switch r.Method {
			case stdhttp.MethodPost:
				taskHandler.pauseTask(w, r, taskID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "resume" {
			switch r.Method {
			case stdhttp.MethodPost:
				taskHandler.resumeTask(w, r, taskID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "cancel" {
			switch r.Method {
			case stdhttp.MethodPost:
				taskHandler.cancelTask(w, r, taskID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "budget" {
			switch r.Method {
			case stdhttp.MethodPost:
				taskHandler.updateTaskBudget(w, r, taskID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))
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

	return applyMiddleware(mux, withRecover, withRequestID, withLogging, withCORS(cfg.CORSAllowedOrigins))
}
