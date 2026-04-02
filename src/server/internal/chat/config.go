package chat

import "time"

type ConversationConfig struct {
	ConversationID        string    `json:"conversationId"`
	ModelID               string    `json:"modelId"`
	SystemPromptOverride  string    `json:"systemPromptOverride,omitempty"`
	Temperature           float64   `json:"temperature"`
	MaxOutputTokens       int       `json:"maxOutputTokens"`
	ToolsEnabled          bool      `json:"toolsEnabled"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

type ModelOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}
