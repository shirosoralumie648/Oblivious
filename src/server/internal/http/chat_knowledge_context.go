package http

import (
	"context"
	"sort"
	"strings"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/knowledge"
)

type chatKnowledgeRetriever interface {
	Get(ctx context.Context, session auth.Session, knowledgeBaseID string) (knowledge.KnowledgeBase, error)
	RetrieveWithOptions(ctx context.Context, session auth.Session, knowledgeBaseID, query string, options knowledge.KnowledgeRetrievalOptions) ([]knowledge.KnowledgeRetrievalResult, error)
}

type chatKnowledgeContextProvider struct {
	retriever chatKnowledgeRetriever
}

func (p chatKnowledgeContextProvider) RelevantKnowledge(ctx context.Context, session auth.Session, knowledgeBaseIDs []string, query string, limit int) ([]chat.KnowledgeContext, error) {
	query = strings.Join(strings.Fields(strings.TrimSpace(query)), " ")
	if query == "" || p.retriever == nil {
		return []chat.KnowledgeContext{}, nil
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 5 {
		limit = 5
	}

	normalizedIDs := normalizeChatKnowledgeBaseIDs(knowledgeBaseIDs)
	if len(normalizedIDs) == 0 {
		return []chat.KnowledgeContext{}, nil
	}

	contexts := make([]chat.KnowledgeContext, 0, limit)
	for _, knowledgeBaseID := range normalizedIDs {
		base, err := p.retriever.Get(ctx, session, knowledgeBaseID)
		if err != nil {
			return nil, err
		}
		results, err := p.retriever.RetrieveWithOptions(ctx, session, knowledgeBaseID, query, knowledge.KnowledgeRetrievalOptions{
			Mode:  knowledge.KnowledgeRetrievalModeHybrid,
			Limit: limit,
		})
		if err != nil {
			return nil, err
		}
		contexts = append(contexts, knowledgeResultsToChatContexts(base, results, limit)...)
	}

	sort.SliceStable(contexts, func(i, j int) bool {
		return contexts[i].Score > contexts[j].Score
	})
	if len(contexts) > limit {
		contexts = contexts[:limit]
	}
	return contexts, nil
}

func normalizeChatKnowledgeBaseIDs(ids []string) []string {
	normalized := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func knowledgeResultsToChatContexts(base knowledge.KnowledgeBase, results []knowledge.KnowledgeRetrievalResult, remaining int) []chat.KnowledgeContext {
	if remaining <= 0 {
		return []chat.KnowledgeContext{}
	}

	contexts := make([]chat.KnowledgeContext, 0, min(remaining, len(results)))
	for _, result := range results {
		content := strings.TrimSpace(result.Source.MatchedSnippet)
		if content == "" {
			content = strings.TrimSpace(result.Snippet)
		}
		if content == "" {
			content = strings.TrimSpace(result.Source.OriginalText)
		}
		if content == "" {
			continue
		}

		documentTitle := strings.TrimSpace(result.Source.DocumentTitle)
		if documentTitle == "" {
			documentTitle = strings.TrimSpace(result.DocumentTitle)
		}
		documentID := strings.TrimSpace(result.Source.DocumentID)
		if documentID == "" {
			documentID = strings.TrimSpace(result.DocumentID)
		}
		documentVersion := strings.TrimSpace(result.Source.DocumentVersion)
		if documentVersion == "" {
			documentVersion = strings.TrimSpace(result.DocumentVersion)
		}
		chunkID := strings.TrimSpace(result.Source.ChunkID)
		if chunkID == "" {
			chunkID = strings.TrimSpace(result.ChunkID)
		}
		chunkIndex := result.Source.ChunkIndex
		if chunkIndex == 0 {
			chunkIndex = result.ChunkIndex
		}
		contexts = append(contexts, chat.KnowledgeContext{
			ChunkID:            chunkID,
			ChunkIndex:         chunkIndex,
			Content:            content,
			DocumentID:         documentID,
			DocumentTitle:      documentTitle,
			DocumentVersion:    documentVersion,
			HighlightPositions: chatKnowledgeHighlightPositions(result.Source.HighlightPositions),
			KnowledgeBaseID:    strings.TrimSpace(base.ID),
			KnowledgeBaseName:  strings.TrimSpace(base.Name),
			OriginalText:       strings.TrimSpace(result.Source.OriginalText),
			PageNumber:         result.Source.PageNumber,
			RetrievalMethod:    strings.TrimSpace(result.RetrievalMethod),
			Score:              result.Similarity,
			SourceURL:          strings.TrimSpace(result.Source.SourceURL),
		})
		if len(contexts) >= remaining {
			break
		}
	}
	return contexts
}

func chatKnowledgeHighlightPositions(positions []knowledge.KnowledgeHighlightPosition) []chat.KnowledgeHighlightPosition {
	if len(positions) == 0 {
		return nil
	}
	converted := make([]chat.KnowledgeHighlightPosition, 0, len(positions))
	for _, position := range positions {
		converted = append(converted, chat.KnowledgeHighlightPosition{
			Start: position.Start,
			End:   position.End,
		})
	}
	return converted
}
