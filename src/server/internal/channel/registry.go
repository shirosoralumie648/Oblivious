package channel

import (
	"encoding/json"
	"fmt"
)

type ChannelAdapter interface {
	Type() ChannelType
	TransformInbound(raw json.RawMessage) (InternalMessage, error)
	TransformOutbound(message InternalMessage) (json.RawMessage, error)
}

type AdapterRegistry struct {
	adapters map[ChannelType]ChannelAdapter
}

func NewAdapterRegistry(adapters map[ChannelType]ChannelAdapter) *AdapterRegistry {
	registry := &AdapterRegistry{adapters: map[ChannelType]ChannelAdapter{}}
	if adapters == nil {
		adapters = map[ChannelType]ChannelAdapter{
			ChannelTypeAPI:      NewAPIAdapter(),
			ChannelTypeWebhook:  NewWebhookAdapter(),
			ChannelTypeFeishu:   NewFeishuAdapter(),
			ChannelTypeWeChat:   NewWeChatAdapter(),
			ChannelTypeDiscord:  NewDiscordAdapter(),
			ChannelTypeSlack:    NewSlackAdapter(),
			ChannelTypeTelegram: NewTelegramAdapter(),
			ChannelTypeWebEmbed: NewWebEmbedAdapter(),
		}
	}
	for channelType, adapter := range adapters {
		if adapter != nil {
			registry.adapters[channelType] = adapter
		}
	}
	return registry
}

func (r *AdapterRegistry) Adapter(channelType ChannelType) (ChannelAdapter, error) {
	if r == nil {
		return nil, fmt.Errorf("channel adapter registry is nil")
	}
	adapter, ok := r.adapters[channelType]
	if !ok {
		return nil, fmt.Errorf("channel adapter %q not registered", channelType)
	}
	return adapter, nil
}
