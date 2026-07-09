package chat

type KnowledgeContext struct {
	ChunkID            string                       `json:"chunkId,omitempty"`
	ChunkIndex         int                          `json:"chunkIndex,omitempty"`
	Content            string                       `json:"content"`
	DocumentID         string                       `json:"documentId,omitempty"`
	DocumentTitle      string                       `json:"documentTitle,omitempty"`
	DocumentVersion    string                       `json:"documentVersion,omitempty"`
	HighlightPositions []KnowledgeHighlightPosition `json:"highlightPositions,omitempty"`
	KnowledgeBaseID    string                       `json:"knowledgeBaseId,omitempty"`
	KnowledgeBaseName  string                       `json:"knowledgeBaseName,omitempty"`
	OriginalText       string                       `json:"originalText,omitempty"`
	PageNumber         int                          `json:"pageNumber,omitempty"`
	RetrievalMethod    string                       `json:"retrievalMethod,omitempty"`
	Score              float64                      `json:"score,omitempty"`
	SourceURL          string                       `json:"sourceUrl,omitempty"`
}

type MemoryContext struct {
	Content string `json:"content"`
}

type SemanticWorkflowTriggerRequest struct {
	ConversationID string `json:"conversationId"`
	Message        string `json:"message"`
	MessageID      string `json:"messageId,omitempty"`
	OrganizationID string `json:"organizationId"`
	UserID         string `json:"userId"`
	WorkspaceID    string `json:"workspaceId"`
}
