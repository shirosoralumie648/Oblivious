package chat

import (
	"errors"
	"time"
)

var ErrMessageShareExpired = errors.New("message share expired")
var ErrUnsupportedChatAction = errors.New("chat action is not supported by store")

type ConversationConfig struct {
	ConversationID       string    `json:"conversationId"`
	KnowledgeBaseIDs     []string  `json:"knowledgeBaseIds"`
	ModelID              string    `json:"modelId"`
	PersonaID            string    `json:"personaId,omitempty"`
	PersonaRole          string    `json:"personaRole,omitempty"`
	PersonaStyle         string    `json:"personaStyle,omitempty"`
	PersonaTone          string    `json:"personaTone,omitempty"`
	PersonaConstraints   string    `json:"personaConstraints,omitempty"`
	SystemPromptOverride string    `json:"systemPromptOverride,omitempty"`
	Temperature          float64   `json:"temperature"`
	MaxOutputTokens      int       `json:"maxOutputTokens"`
	ToolsEnabled         bool      `json:"toolsEnabled"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

type CreateMessageOptions struct {
	Attachments        []MessageAttachment `json:"attachments,omitempty"`
	KnowledgeCitations []KnowledgeCitation `json:"knowledgeCitations,omitempty"`
}

type MessageOverrides struct {
	ModelID              *string
	SystemPromptOverride *string
	Temperature          *float64
	MaxOutputTokens      *int
	ToolsEnabled         *bool
	PersonaID            *string
}

type ModelOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Persona represents an AI personality configuration that can be applied to conversations.
type Persona struct {
	CreatedAt          time.Time `json:"createdAt"`
	ID                 string    `json:"id"`
	WorkspaceID        string    `json:"workspaceId"`
	Name               string    `json:"name"`
	Role               string    `json:"role"`
	Style              string    `json:"style"`
	Tone               string    `json:"tone"`
	Constraints        string    `json:"constraints,omitempty"`
	OpeningMessage     string    `json:"openingMessage,omitempty"`
	SuggestedQuestions []string  `json:"suggestedQuestions,omitempty"`
}

// MessageAttachment represents a file or image attached to a message.
type MessageAttachment struct {
	CreatedAt      time.Time `json:"createdAt,omitempty"`
	ID             string    `json:"id"`
	MessageID      string    `json:"messageId,omitempty"`
	FileName       string    `json:"fileName,omitempty"`
	FileType       string    `json:"fileType,omitempty"`
	FileSize       int64     `json:"fileSize,omitempty"`
	Name           string    `json:"name,omitempty"`
	ContentType    string    `json:"contentType,omitempty"`
	ProviderFileID string    `json:"providerFileId,omitempty"`
	SizeBytes      int64     `json:"sizeBytes,omitempty"`
	Type           string    `json:"type,omitempty"`
	URL            string    `json:"url,omitempty"`
	Description    string    `json:"description,omitempty"`
}

// MessageMetadata holds extra metadata for a message, including attachments.
type MessageMetadata struct {
	Attachments        []MessageAttachment `json:"attachments,omitempty"`
	KnowledgeCitations []KnowledgeCitation `json:"knowledgeCitations,omitempty"`
}

type KnowledgeHighlightPosition struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type KnowledgeCitation struct {
	ChunkID            string                       `json:"chunkId"`
	ChunkIndex         int                          `json:"chunkIndex"`
	DocumentID         string                       `json:"documentId"`
	DocumentTitle      string                       `json:"documentTitle"`
	DocumentVersion    string                       `json:"documentVersion"`
	HighlightPositions []KnowledgeHighlightPosition `json:"highlightPositions,omitempty"`
	KnowledgeBaseID    string                       `json:"knowledgeBaseId"`
	KnowledgeBaseName  string                       `json:"knowledgeBaseName"`
	OriginalText       string                       `json:"originalText"`
	PageNumber         int                          `json:"pageNumber,omitempty"`
	Score              float64                      `json:"score"`
	Snippet            string                       `json:"snippet"`
	SourceURL          string                       `json:"sourceUrl,omitempty"`
}

type MessageShare struct {
	ConversationID string     `json:"conversationId"`
	CreatedAt      time.Time  `json:"createdAt"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	ID             string     `json:"id"`
	MessageID      string     `json:"messageId"`
	OrganizationID string     `json:"organizationId"`
	URL            string     `json:"url"`
}

type MessageShareDetail struct {
	MessageShare
	Message Message `json:"message"`
}

type ConversationShareStoreOptions struct {
	StartMessageID string
	EndMessageID   string
	ExpiresAt      *time.Time
}

type ConversationShare struct {
	ConversationID string     `json:"conversationId"`
	CreatedAt      time.Time  `json:"createdAt"`
	EndMessageID   string     `json:"endMessageId,omitempty"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	StartMessageID string     `json:"startMessageId,omitempty"`
	URL            string     `json:"url"`
}

type ConversationShareDetail struct {
	ConversationShare
	Conversation Conversation `json:"conversation"`
	Messages     []Message    `json:"messages"`
}

// MessageSearchResult holds a single search hit inside message content.
type MessageSearchResult struct {
	MessageID      string    `json:"messageId"`
	ConversationID string    `json:"conversationId"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"createdAt"`
	Snippet        string    `json:"snippet"`
}
