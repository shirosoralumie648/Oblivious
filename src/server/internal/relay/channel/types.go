package channel

import (
	"oblivious/server/internal/relay/types"
)

// Re-export channel types from types package for backward compatibility
type (
	Channel         = types.Channel
	ModelRoute      = types.ModelRoute
	RouteChannel    = types.RouteChannel
	ChannelStats    = types.ChannelStats
	Message         = types.Message
	ProviderRequest = types.ProviderRequest
	Capabilities    = types.Capabilities
	ProviderAdapter = types.ProviderAdapter
)
