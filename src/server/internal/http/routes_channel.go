package http

import (
	stdhttp "net/http"
	"strings"
)

func registerPublishingChannelRoutes(mux *stdhttp.ServeMux, authMiddleware sessionMiddleware, channelHandler channelHandler) {
	mux.Handle("/api/v1/channels", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			channelHandler.listChannels(w, r)
		case stdhttp.MethodPost:
			channelHandler.createChannel(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))

	mux.Handle("/api/v1/channels/webhook/", stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		channelID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/channels/webhook/"), "/")
		if channelID == "" || strings.Contains(channelID, "/") {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}
		switch r.Method {
		case stdhttp.MethodPost:
			channelHandler.receiveWebhook(w, r, channelID)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}))

	mux.Handle("/api/v1/channels/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/v1/channels/")
		parts := strings.Split(trimmedPath, "/")
		if len(parts) == 0 || parts[0] == "" || parts[0] == "webhook" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		channelID := parts[0]
		if len(parts) == 1 {
			switch r.Method {
			case stdhttp.MethodGet:
				channelHandler.getChannel(w, r, channelID)
			case stdhttp.MethodPut:
				channelHandler.updateChannel(w, r, channelID)
			case stdhttp.MethodDelete:
				channelHandler.deleteChannel(w, r, channelID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "status" {
			switch r.Method {
			case stdhttp.MethodPatch:
				channelHandler.updateChannelStatus(w, r, channelID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "test" {
			switch r.Method {
			case stdhttp.MethodPost:
				channelHandler.testChannel(w, r, channelID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "messages" {
			switch r.Method {
			case stdhttp.MethodGet:
				channelHandler.listChannelMessages(w, r, channelID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "failed-messages" {
			switch r.Method {
			case stdhttp.MethodGet:
				channelHandler.listFailedChannelMessages(w, r, channelID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "retry-failed-messages" {
			switch r.Method {
			case stdhttp.MethodPost:
				channelHandler.retryFailedChannelMessages(w, r, channelID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "send" {
			switch r.Method {
			case stdhttp.MethodPost:
				channelHandler.sendChannelMessage(w, r, channelID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))
}
