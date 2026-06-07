package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"oblivious/server/internal/channel/adapter"
	"oblivious/server/internal/channel/message"
)

// ChannelService provides CRUD operations and connectivity testing for channels.
type ChannelService struct {
	store       ChannelStore
	transformer *message.Transformer
	adapters    map[string]adapter.ChannelAdapter
}

// ChannelStore defines the persistence operations required by ChannelService.
type ChannelStore interface {
	Create(ctx context.Context, input CreateChannelInput) (*ChannelConfig, error)
	Get(ctx context.Context, organizationID, id string) (*ChannelConfig, error)
	GetByID(ctx context.Context, id string) (*ChannelConfig, error)
	List(ctx context.Context, input ListChannelsInput) ([]*ChannelConfig, error)
	Update(ctx context.Context, organizationID, id string, input UpdateChannelInput) (*ChannelConfig, error)
	Delete(ctx context.Context, organizationID, id string) error
}

// NewChannelService creates a ChannelService with the given store and
// registers all built-in platform adapters.
func NewChannelService(store ChannelStore) *ChannelService {
	adapters := map[string]adapter.ChannelAdapter{
		"feishu":   adapter.NewFeiShuAdapter(),
		"wechat":   adapter.NewWeChatAdapter(),
		"discord":  adapter.NewDiscordAdapter(),
		"slack":    adapter.NewSlackAdapter(),
		"web_sdk":  adapter.NewWebSDKAdapter(),
	}
	return &ChannelService{
		store:       store,
		transformer: message.NewTransformer(),
		adapters:    adapters,
	}
}

// NewChannelServiceWithAdapters creates a ChannelService with a custom adapter
// map, allowing callers to override or extend the default set.
func NewChannelServiceWithAdapters(store ChannelStore, adapters map[string]adapter.ChannelAdapter) *ChannelService {
	svc := NewChannelService(store)
	for k, v := range adapters {
		if v != nil {
			svc.adapters[k] = v
		}
	}
	return svc
}

// CreateChannel validates the input and creates a new channel configuration.
func (s *ChannelService) CreateChannel(ctx context.Context, input CreateChannelInput) (*ChannelConfig, error) {
	if input.OrganizationID == "" {
		return nil, fmt.Errorf("organization_id is required")
	}
	if input.Name == "" {
		return nil, fmt.Errorf("channel name is required")
	}
	if input.Type == "" {
		return nil, fmt.Errorf("channel type is required")
	}
	if err := s.validateChannelType(string(input.Type)); err != nil {
		return nil, err
	}

	return s.store.Create(ctx, input)
}

// GetChannel retrieves a single channel by organization and ID.
func (s *ChannelService) GetChannel(ctx context.Context, organizationID, id string) (*ChannelConfig, error) {
	if organizationID == "" {
		return nil, fmt.Errorf("organization_id is required")
	}
	if id == "" {
		return nil, fmt.Errorf("channel id is required")
	}

	config, err := s.store.Get(ctx, organizationID, id)
	if err != nil {
		return nil, fmt.Errorf("get channel: %w", err)
	}
	if config == nil {
		return nil, fmt.Errorf("channel %q not found", id)
	}
	return config, nil
}

// ListChannels returns channels matching the given filter.
func (s *ChannelService) ListChannels(ctx context.Context, input ListChannelsInput) ([]*ChannelConfig, error) {
	if input.OrganizationID == "" {
		return nil, fmt.Errorf("organization_id is required")
	}
	if input.Limit <= 0 {
		input.Limit = 20
	}
	if input.Limit > 100 {
		input.Limit = 100
	}

	configs, err := s.store.List(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	return configs, nil
}

// UpdateChannel applies partial updates to an existing channel.
func (s *ChannelService) UpdateChannel(ctx context.Context, organizationID, id string, input UpdateChannelInput) (*ChannelConfig, error) {
	if organizationID == "" {
		return nil, fmt.Errorf("organization_id is required")
	}
	if id == "" {
		return nil, fmt.Errorf("channel id is required")
	}
	if input.Type != nil {
		if err := s.validateChannelType(string(*input.Type)); err != nil {
			return nil, err
		}
	}

	config, err := s.store.Update(ctx, organizationID, id, input)
	if err != nil {
		return nil, fmt.Errorf("update channel: %w", err)
	}
	if config == nil {
		return nil, fmt.Errorf("channel %q not found", id)
	}
	return config, nil
}

// DeleteChannel removes a channel by organization and ID.
func (s *ChannelService) DeleteChannel(ctx context.Context, organizationID, id string) error {
	if organizationID == "" {
		return fmt.Errorf("organization_id is required")
	}
	if id == "" {
		return fmt.Errorf("channel id is required")
	}

	if err := s.store.Delete(ctx, organizationID, id); err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}
	return nil
}

// TestConnection verifies that the channel can communicate with its upstream
// platform. For webhook-based channels it performs an HTTP probe; for adapter-
// based channels it delegates to the adapter's TestConnection method.
func (s *ChannelService) TestConnection(ctx context.Context, config *ChannelConfig) (TestConnectionResult, error) {
	if config == nil {
		return TestConnectionResult{}, fmt.Errorf("channel config is required")
	}

	result := TestConnectionResult{
		ChannelID: config.ID,
		Type:      string(config.Type),
	}

	adp, ok := s.adapters[string(config.Type)]
	if !ok {
		result.Status = "failed"
		result.Message = fmt.Sprintf("no adapter registered for channel type %q", config.Type)
		return result, nil
	}

	if err := adp.TestConnection(ctx, config.Config); err != nil {
		result.Status = "failed"
		result.Message = err.Error()
		return result, nil
	}

	result.Status = "success"
	result.Message = "connection successful"
	return result, nil
}

// ReceiveMessage transforms an inbound raw payload into the unified message
// format using the adapter registered for the given channel type.
func (s *ChannelService) ReceiveMessage(channelType string, raw []byte) (ChannelMessageLog, error) {
	adp, ok := s.adapters[channelType]
	if !ok {
		return ChannelMessageLog{}, fmt.Errorf("no adapter for channel type %q", channelType)
	}

	transformResult := s.transformer.TransformInboundSafe(channelType, adp, raw)

	log := ChannelMessageLog{
		Direction:  DirectionInbound,
		RawMessage: append(json.RawMessage(nil), raw...),
		Status:     MessageStatusRecorded,
		CreatedAt:  time.Now().UTC(),
	}

	if !transformResult.Success {
		log.TransformSuccess = false
		log.TransformError = transformResult.Error
		return log, nil
	}

	log.ConversationID = transformResult.Message.ConversationID
	log.TransformedMessage = toChannelMessage(transformResult.Message)
	log.TransformSuccess = true
	return log, nil
}

// SendMessage transforms an outbound InternalMessage into the platform-specific
// format using the adapter registered for the given channel type.
func (s *ChannelService) SendMessage(channelType string, message InternalMessage) (ChannelMessageLog, error) {
	adp, ok := s.adapters[channelType]
	if !ok {
		return ChannelMessageLog{}, fmt.Errorf("no adapter for channel type %q", channelType)
	}

	adapterMsg := toAdapterMessage(message)
	transformResult := s.transformer.TransformOutboundSafe(channelType, adp, adapterMsg)

	log := ChannelMessageLog{
		ChannelID:      "",
		ConversationID: message.ConversationID,
		Direction:      DirectionOutbound,
		Status:         MessageStatusRecorded,
		CreatedAt:      time.Now().UTC(),
	}

	if !transformResult.Success {
		log.TransformSuccess = false
		log.TransformError = transformResult.Error
		return log, nil
	}

	log.RawMessage = append(json.RawMessage(nil), transformResult.Raw...)
	log.TransformedMessage = message
	log.TransformSuccess = true
	return log, nil
}

// GetAdapter returns the adapter registered for the given channel type.
func (s *ChannelService) GetAdapter(channelType string) (adapter.ChannelAdapter, bool) {
	adp, ok := s.adapters[channelType]
	return adp, ok
}

func (s *ChannelService) validateChannelType(channelType string) error {
	channelType = strings.TrimSpace(channelType)
	if channelType == "" {
		return fmt.Errorf("channel type is required")
	}
	knownTypes := map[string]bool{
		"api": true, "webhook": true, "feishu": true, "wechat": true,
		"discord": true, "slack": true, "telegram": true, "web_embed": true,
		"web_sdk": true,
	}
	if !knownTypes[channelType] {
		return fmt.Errorf("unsupported channel type: %s", channelType)
	}
	return nil
}

func toChannelMessage(msg adapter.InternalMessage) InternalMessage {
	content := make([]ContentPart, len(msg.Content))
	for i, part := range msg.Content {
		content[i] = ContentPart{
			Type:     ContentType(part.Type),
			Text:     part.Text,
			URL:      part.URL,
			Metadata: part.Metadata,
		}
	}
	return InternalMessage{
		ID:             msg.ID,
		ConversationID: msg.ConversationID,
		Role:           Role(msg.Role),
		Content:        content,
		Metadata:       msg.Metadata,
		Timestamp:      msg.Timestamp,
	}
}

func toAdapterMessage(msg InternalMessage) adapter.InternalMessage {
	content := make([]adapter.ContentPart, len(msg.Content))
	for i, part := range msg.Content {
		content[i] = adapter.ContentPart{
			Type:     string(part.Type),
			Text:     part.Text,
			URL:      part.URL,
			Metadata: part.Metadata,
		}
	}
	return adapter.InternalMessage{
		ID:             msg.ID,
		ConversationID: msg.ConversationID,
		Role:           string(msg.Role),
		Content:        content,
		Metadata:       msg.Metadata,
		Timestamp:      msg.Timestamp,
	}
}
