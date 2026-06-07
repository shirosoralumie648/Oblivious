package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// InternalMessage is the unified message format shared across all channel adapters.
type InternalMessage struct {
	ID             string         `json:"id"`
	ConversationID string         `json:"conversation_id"`
	Role           string         `json:"role"`
	Content        []ContentPart  `json:"content"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	Timestamp      time.Time      `json:"timestamp"`
}

// ContentPart represents a single piece of message content.
type ContentPart struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	URL      string         `json:"url,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ChannelAdapter defines the interface every platform adapter must implement.
type ChannelAdapter interface {
	// Type returns the channel type identifier (e.g. "feishu", "slack").
	Type() string

	// TransformInbound converts a platform-specific raw payload into the
	// unified InternalMessage format.
	TransformInbound(raw json.RawMessage) (InternalMessage, error)

	// TransformOutbound converts an InternalMessage into the platform-specific
	// payload format for delivery.
	TransformOutbound(message InternalMessage) (json.RawMessage, error)

	// TestConnection verifies that the adapter can reach the upstream platform
	// with the supplied configuration.
	TestConnection(ctx context.Context, config map[string]any) error
}

// ValidateMessage checks that an InternalMessage has the minimum required fields.
func ValidateMessage(message InternalMessage) error {
	if message.ConversationID == "" {
		return fmt.Errorf("conversation_id is required")
	}
	if message.Role == "" {
		return fmt.Errorf("role is required")
	}
	if len(message.Content) == 0 {
		return fmt.Errorf("content is required")
	}
	return nil
}

// FirstTextPart extracts the first text content part from a message.
func FirstTextPart(content []ContentPart) string {
	for _, part := range content {
		if part.Type == "text" && part.Text != "" {
			return part.Text
		}
	}
	return ""
}

// FirstCardPart extracts the first card content part from a message.
func FirstCardPart(content []ContentPart) (ContentPart, bool) {
	for _, part := range content {
		if part.Type == "card" {
			return part, true
		}
	}
	return ContentPart{}, false
}

// FirstCardMetadata extracts the first platform card payload from message content.
func FirstCardMetadata(content []ContentPart) map[string]any {
	part, ok := FirstCardPart(content)
	if !ok || len(part.Metadata) == 0 {
		return nil
	}
	return part.Metadata
}

// FirstContentPart extracts the first content part matching the requested type.
func FirstContentPart(content []ContentPart, contentType string) (ContentPart, bool) {
	for _, part := range content {
		if part.Type == contentType {
			return part, true
		}
	}
	return ContentPart{}, false
}

// StringMetadata extracts a string value from a metadata map.
func StringMetadata(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key].(string)
	if !ok {
		return ""
	}
	return value
}

// FirstNonEmpty returns the first non-empty string from the provided values.
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
