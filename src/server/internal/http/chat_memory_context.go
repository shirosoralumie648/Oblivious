package http

import (
	"context"
	"strings"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	relaytypes "oblivious/server/internal/relay/types"
)

type agentMemoryListService interface {
	ListMemories(ctx context.Context, session auth.Session, req agent.ListMemoriesRequest) ([]*agent.Memory, error)
}

type agentMemoryVectorStore interface {
	SearchMemories(ctx context.Context, organizationID, userID string, req agent.SearchMemoriesRequest) ([]*agent.MemorySearchResult, error)
}

type chatMemoryEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type chatMemoryContextProvider struct {
	embedder chatMemoryEmbedder
	list     agentMemoryListService
	vector   agentMemoryVectorStore
}

func (p chatMemoryContextProvider) RelevantMemories(ctx context.Context, session auth.Session, query string, limit int) ([]chat.MemoryContext, error) {
	query = strings.TrimSpace(query)
	if limit <= 0 {
		limit = 5
	}
	if limit > 5 {
		limit = 5
	}

	if p.vector != nil && p.embedder != nil && query != "" {
		embeddingCtx := ctx
		if session.User.ID != "" {
			embeddingCtx = relaytypes.WithTrustedUserID(embeddingCtx, session.User.ID)
		}
		if session.OrganizationID != "" {
			embeddingCtx = relaytypes.WithTrustedOrganizationID(embeddingCtx, session.OrganizationID)
		}
		embedding, err := p.embedder.Embed(embeddingCtx, query)
		if err == nil && len(embedding) > 0 {
			results, searchErr := p.vector.SearchMemories(ctx, session.OrganizationID, session.User.ID, agent.SearchMemoriesRequest{
				Type:      agent.MemoryTypeLongTerm,
				Embedding: embedding,
				Limit:     limit,
				MinScore:  0.5,
			})
			if searchErr != nil {
				return nil, searchErr
			}
			memories := memorySearchResultsToChatContexts(results, limit)
			if len(memories) > 0 {
				return memories, nil
			}
		}
	}

	if p.list == nil {
		return []chat.MemoryContext{}, nil
	}
	memories, err := p.list.ListMemories(ctx, session, agent.ListMemoriesRequest{
		Type:  agent.MemoryTypeLongTerm,
		Query: query,
		Limit: limit,
	})
	if err != nil {
		return nil, err
	}
	return agentMemoriesToChatContexts(memories, limit), nil
}

func memorySearchResultsToChatContexts(results []*agent.MemorySearchResult, limit int) []chat.MemoryContext {
	contexts := make([]chat.MemoryContext, 0, min(limit, len(results)))
	for _, result := range results {
		if result == nil {
			continue
		}
		content := strings.TrimSpace(result.Memory.Content)
		if content == "" {
			continue
		}
		contexts = append(contexts, chat.MemoryContext{Content: content})
		if len(contexts) >= limit {
			break
		}
	}
	return contexts
}

func agentMemoriesToChatContexts(memories []*agent.Memory, limit int) []chat.MemoryContext {
	contexts := make([]chat.MemoryContext, 0, min(limit, len(memories)))
	for _, memory := range memories {
		if memory == nil {
			continue
		}
		content := strings.TrimSpace(memory.Content)
		if content == "" {
			continue
		}
		contexts = append(contexts, chat.MemoryContext{Content: content})
		if len(contexts) >= limit {
			break
		}
	}
	return contexts
}
