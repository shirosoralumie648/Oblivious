package message

import (
	"encoding/json"
	"fmt"
	"time"

	"oblivious/server/internal/channel/adapter"
)

// Transformer converts messages between platform-specific formats and the
// unified InternalMessage format used by the channel system.
type Transformer struct{}

// NewTransformer creates a new message Transformer.
func NewTransformer() *Transformer {
	return &Transformer{}
}

// TransformInbound converts a platform-specific raw payload into an
// InternalMessage using the provided adapter.
func (t *Transformer) TransformInbound(adapterType string, adp adapter.ChannelAdapter, raw json.RawMessage) (adapter.InternalMessage, error) {
	if adp == nil {
		return adapter.InternalMessage{}, fmt.Errorf("adapter is required for type %q", adapterType)
	}
	if len(raw) == 0 {
		return adapter.InternalMessage{}, fmt.Errorf("raw payload is required")
	}

	message, err := adp.TransformInbound(raw)
	if err != nil {
		return adapter.InternalMessage{}, fmt.Errorf("transform inbound for %s: %w", adapterType, err)
	}

	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now().UTC()
	}
	if message.Metadata == nil {
		message.Metadata = map[string]any{}
	}
	message.Metadata["adapter"] = adapterType

	return message, nil
}

// TransformOutbound converts an InternalMessage into a platform-specific
// payload using the provided adapter.
func (t *Transformer) TransformOutbound(adapterType string, adp adapter.ChannelAdapter, message adapter.InternalMessage) (json.RawMessage, error) {
	if adp == nil {
		return nil, fmt.Errorf("adapter is required for type %q", adapterType)
	}

	raw, err := adp.TransformOutbound(message)
	if err != nil {
		return nil, fmt.Errorf("transform outbound for %s: %w", adapterType, err)
	}

	return raw, nil
}

// TransformResult holds the outcome of a transformation attempt.
type TransformResult struct {
	Success   bool                `json:"success"`
	Message   adapter.InternalMessage `json:"message,omitempty"`
	Raw       json.RawMessage     `json:"raw,omitempty"`
	Error     string              `json:"error,omitempty"`
	Direction string              `json:"direction"`
}

// TransformInboundSafe wraps TransformInbound and never returns an error.
// Instead it captures the result in a TransformResult.
func (t *Transformer) TransformInboundSafe(adapterType string, adp adapter.ChannelAdapter, raw json.RawMessage) TransformResult {
	result := TransformResult{Direction: "inbound"}
	message, err := t.TransformInbound(adapterType, adp, raw)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Success = true
	result.Message = message
	return result
}

// TransformOutboundSafe wraps TransformOutbound and never returns an error.
// Instead it captures the result in a TransformResult.
func (t *Transformer) TransformOutboundSafe(adapterType string, adp adapter.ChannelAdapter, message adapter.InternalMessage) TransformResult {
	result := TransformResult{Direction: "outbound"}
	raw, err := t.TransformOutbound(adapterType, adp, message)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Success = true
	result.Raw = raw
	return result
}
