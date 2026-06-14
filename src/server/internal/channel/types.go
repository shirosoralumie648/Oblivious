package channel

import (
	"encoding/json"
	"strings"
	"time"
)

// Role identifies the sender of a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// ContentType identifies the kind of content in a message part.
type ContentType string

const (
	ContentTypeText  ContentType = "text"
	ContentTypeImage ContentType = "image"
	ContentTypeFile  ContentType = "file"
	ContentTypeCard  ContentType = "card"
)

// ContentPart is a single piece of message content.
type ContentPart struct {
	Type     ContentType    `json:"type"`
	Text     string         `json:"text,omitempty"`
	URL      string         `json:"url,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// InternalMessage is the unified message format used across all channel types.
type InternalMessage struct {
	ID             string         `json:"id"`
	ConversationID string         `json:"conversation_id"`
	Role           Role           `json:"role"`
	Content        []ContentPart  `json:"content"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	Timestamp      time.Time      `json:"timestamp"`
}

// ChannelType identifies a messaging platform integration.
type ChannelType string

const (
	ChannelTypeAPI      ChannelType = "api"
	ChannelTypeWebhook  ChannelType = "webhook"
	ChannelTypeFeishu   ChannelType = "feishu"
	ChannelTypeWeChat   ChannelType = "wechat"
	ChannelTypeDiscord  ChannelType = "discord"
	ChannelTypeSlack    ChannelType = "slack"
	ChannelTypeTelegram ChannelType = "telegram"
	ChannelTypeWebEmbed ChannelType = "web_embed"
)

// ChannelStatus represents the operational state of a channel.
type ChannelStatus string

const (
	ChannelStatusActive   ChannelStatus = "active"
	ChannelStatusDegraded ChannelStatus = "degraded"
	ChannelStatusDisabled ChannelStatus = "disabled"
)

// ChannelConfig holds the configuration and metadata for a channel.
type ChannelConfig struct {
	ID             string         `json:"id"`
	OrganizationID string         `json:"organization_id"`
	Type           ChannelType    `json:"type"`
	Name           string         `json:"name"`
	Config         map[string]any `json:"config"`
	Status         ChannelStatus  `json:"status"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

func IsChannelSecretConfigKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
	return strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "apikey") ||
		strings.Contains(normalized, "password")
}

// Direction indicates whether a message is inbound or outbound.
type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
)

// MessageStatus tracks the delivery state of a channel message.
type MessageStatus string

const (
	MessageStatusRecorded         MessageStatus = "recorded"
	MessageStatusRetryPending     MessageStatus = "retry_pending"
	MessageStatusSending          MessageStatus = "sending"
	MessageStatusPermanentFailure MessageStatus = "permanent_failure"
)

// ChannelMessageLog records a single inbound or outbound message for audit
// and retry purposes.
type ChannelMessageLog struct {
	ID                 string          `json:"id"`
	ChannelID          string          `json:"channel_id"`
	ConversationID     string          `json:"conversation_id,omitempty"`
	Direction          Direction       `json:"direction"`
	RawMessage         json.RawMessage `json:"raw_message"`
	TransformedMessage InternalMessage `json:"transformed_message,omitempty"`
	TransformSuccess   bool            `json:"transform_success"`
	TransformError     string          `json:"transform_error,omitempty"`
	Status             MessageStatus   `json:"status"`
	RetryCount         int             `json:"retry_count"`
	FailureReason      string          `json:"failure_reason,omitempty"`
	NextRetryAt        *time.Time      `json:"next_retry_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
}

// CreateChannelInput contains the fields needed to create a new channel.
type CreateChannelInput struct {
	OrganizationID string         `json:"organization_id"`
	Type           ChannelType    `json:"type"`
	Name           string         `json:"name"`
	Config         map[string]any `json:"config"`
}

// UpdateChannelInput contains the optional fields for updating a channel.
type UpdateChannelInput struct {
	Name   string         `json:"name,omitempty"`
	Type   *ChannelType   `json:"type,omitempty"`
	Config map[string]any `json:"config,omitempty"`
	Status *ChannelStatus `json:"status,omitempty"`
}

// ListChannelsInput contains filter and pagination parameters.
type ListChannelsInput struct {
	OrganizationID string `json:"organization_id"`
	Type           string `json:"type,omitempty"`
	Status         string `json:"status,omitempty"`
	Limit          int    `json:"limit,omitempty"`
	Offset         int    `json:"offset,omitempty"`
}

// TestConnectionResult holds the outcome of a channel connectivity test.
type TestConnectionResult struct {
	ChannelID string `json:"channel_id"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

func validRole(role Role) bool {
	switch role {
	case RoleUser, RoleAssistant, RoleSystem:
		return true
	default:
		return false
	}
}

func validContentType(contentType ContentType) bool {
	switch contentType {
	case ContentTypeText, ContentTypeImage, ContentTypeFile, ContentTypeCard:
		return true
	default:
		return false
	}
}
