package http

import (
	stdhttp "net/http"
	"strings"
)

func publishingChannelRouteSurfaceOperations() []OperationContractMetadataV1 {
	return routeSurfaceOperationsFromSpecs([]routeSurfaceOperationSpec{
		{"GET", "/api/v1/channels", "listPublishingChannels", "cookie", false, "channel.delivery", "", "none", "", "200", "application/json", "inline", "sha256:689d23e85ae5dbde8df9febec392b4ac4499558ae41972a6db3f120252c9098c"},
		{"POST", "/api/v1/channels", "createPublishingChannel", "cookie+csrf", true, "channel.delivery", "application/json", "ref", "#/components/schemas/ChannelConfigRequest", "201", "application/json", "inline", "sha256:0755f9650a696d18bf63a7235104d4a517f4f6cc04e98597722ee74c12d3d341"},
		{"POST", "/api/v1/channels/webhook/{channelId}", "receivePublishingChannelWebhook", "public", false, "channel.delivery", "application/json", "inline", "sha256:82ef96cebaf5fbe16269fd18b0240d78f5b9b90a4155a17eb797115b09148ecf", "200", "application/json", "inline", "sha256:6f3070343af85f5d187c1620f507f055dc5815911d547b77c7aba3865360fa9e"},
		{"DELETE", "/api/v1/channels/{channelId}", "deletePublishingChannel", "cookie+csrf", true, "channel.delivery", "", "none", "", "200", "application/json", "inline", "sha256:0755f9650a696d18bf63a7235104d4a517f4f6cc04e98597722ee74c12d3d341"},
		{"GET", "/api/v1/channels/{channelId}", "getPublishingChannel", "cookie", false, "channel.delivery", "", "none", "", "200", "application/json", "inline", "sha256:0755f9650a696d18bf63a7235104d4a517f4f6cc04e98597722ee74c12d3d341"},
		{"PUT", "/api/v1/channels/{channelId}", "updatePublishingChannel", "cookie+csrf", true, "channel.delivery", "application/json", "ref", "#/components/schemas/ChannelConfigRequest", "200", "application/json", "inline", "sha256:0755f9650a696d18bf63a7235104d4a517f4f6cc04e98597722ee74c12d3d341"},
		{"GET", "/api/v1/channels/{channelId}/failed-messages", "listPublishingChannelFailedMessages", "cookie", false, "channel.delivery", "", "none", "", "200", "application/json", "inline", "sha256:4c001e4d7a141e6cbf78780ed51fd1e8892e3b5618e6575d34cf909cd5e7dab6"},
		{"GET", "/api/v1/channels/{channelId}/messages", "listPublishingChannelMessages", "cookie", false, "channel.delivery", "", "none", "", "200", "application/json", "inline", "sha256:4c001e4d7a141e6cbf78780ed51fd1e8892e3b5618e6575d34cf909cd5e7dab6"},
		{"POST", "/api/v1/channels/{channelId}/retry-failed-messages", "retryPublishingChannelFailedMessages", "cookie+csrf", true, "channel.delivery", "application/json", "ref", "#/components/schemas/RetryFailedChannelMessagesRequest", "200", "application/json", "inline", "sha256:4fc81d0615c8b0a52dcfd7ee46577c3bcbff0d551a832f95835b8c8b5f961c0f"},
		{"POST", "/api/v1/channels/{channelId}/send", "sendPublishingChannelMessage", "cookie+csrf", true, "channel.delivery", "application/json", "ref", "#/components/schemas/SendChannelMessageRequest", "200", "application/json", "inline", "sha256:6f3070343af85f5d187c1620f507f055dc5815911d547b77c7aba3865360fa9e"},
		{"PATCH", "/api/v1/channels/{channelId}/status", "updatePublishingChannelStatus", "cookie+csrf", true, "channel.delivery", "application/json", "ref", "#/components/schemas/ChannelStatusRequest", "200", "application/json", "inline", "sha256:0755f9650a696d18bf63a7235104d4a517f4f6cc04e98597722ee74c12d3d341"},
		{"POST", "/api/v1/channels/{channelId}/test", "testPublishingChannel", "cookie+csrf", true, "channel.delivery", "", "none", "", "200", "application/json", "inline", "sha256:c92d20b8853aae7d041d128293f6a771db09aa65d54a954beedc8f8ebbb125ed"},
	})
}

func publishingChannelRouteHandler(channelHandler channelHandler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.URL.Path == "/api/v1/channels" {
			if r.Method == stdhttp.MethodGet {
				channelHandler.listChannels(w, r)
			} else {
				channelHandler.createChannel(w, r)
			}
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/channels/webhook/") {
			channelID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/channels/webhook/"), "/")
			if channelID == "" || strings.Contains(channelID, "/") {
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
				return
			}
			channelHandler.receiveWebhook(w, r, channelID)
			return
		}

		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/channels/"), "/")
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
			default:
				channelHandler.deleteChannel(w, r, channelID)
			}
			return
		}
		if len(parts) != 2 {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}
		switch parts[1] {
		case "status":
			channelHandler.updateChannelStatus(w, r, channelID)
		case "test":
			channelHandler.testChannel(w, r, channelID)
		case "messages":
			channelHandler.listChannelMessages(w, r, channelID)
		case "failed-messages":
			channelHandler.listFailedChannelMessages(w, r, channelID)
		case "retry-failed-messages":
			channelHandler.retryFailedChannelMessages(w, r, channelID)
		case "send":
			channelHandler.sendChannelMessage(w, r, channelID)
		default:
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
		}
	})
}

func registerPublishingChannelRouteSurfaces(registrar *RouteSurfaceRegistrar, channelHandler channelHandler) error {
	operations := publishingChannelRouteSurfaceOperations()
	return registerRouteSurfaceBindings(registrar, routeSurfaceBindingsForHandler(operations, RouteSurfaceAuthSession, publishingChannelRouteHandler(channelHandler)))
}

func registerPublishingChannelRoutes(mux *stdhttp.ServeMux, authMiddleware sessionMiddleware, channelHandler channelHandler) {
	if err := registerPublishingChannelRouteSurfaces(mustRouteSurfaceAdapterRegistrar(mux, authMiddleware), channelHandler); err != nil {
		panic(err)
	}
}
