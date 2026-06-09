package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"

	agentpkg "oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	agentmemory "oblivious/server/internal/memory"
)

// Retriever searches for relevant memories and injects them into the
// system prompt at the start of each conversation turn.
type Retriever struct {
	searcher MemorySearcher
	store    MemoryStore
	embedder MemoryEmbedder
}

// MemorySearcher abstracts the knowledge-base memory search.
type MemorySearcher interface {
	Search(ctx context.Context, session auth.Session, req *agentmemory.SearchRequest) ([]*agentmemory.SearchResult, error)
}

// MemoryStore abstracts agent-managed memory storage.
type MemoryStore interface {
	ListMemories(ctx context.Context, organizationID, userID string, req agentpkg.ListMemoriesRequest) ([]*agentpkg.Memory, error)
	SearchMemories(ctx context.Context, organizationID, userID string, req agentpkg.SearchMemoriesRequest) ([]*agentpkg.MemorySearchResult, error)
}

// MemoryEmbedder embeds text for vector search.
type MemoryEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// NewRetriever creates a Retriever.
func NewRetriever(searcher MemorySearcher, store MemoryStore, embedder MemoryEmbedder) *Retriever {
	return &Retriever{
		searcher: searcher,
		store:    store,
		embedder: embedder,
	}
}

// RetrieveResult contains the memories found and the modified messages.
type RetrieveResult struct {
	Messages    []chat.Message `json:"-"`
	MemoryCount int            `json:"memoryCount"`
	Searched    bool           `json:"searched"`
	UsedVector  bool           `json:"usedVector"`
}

// RetrieveAndInject searches for relevant memories and injects the top-5
// into the system prompt of the conversation messages.
//
// This should be called at the start of each conversation turn, before
// sending messages to the LLM.
func (r *Retriever) RetrieveAndInject(
	ctx context.Context,
	session auth.Session,
	agentInstance *agentpkg.Agent,
	messages []chat.Message,
	userContent string,
) RetrieveResult {
	result := RetrieveResult{
		Messages: messages,
	}

	if !agentInstance.Config.EnableMemory {
		return result
	}

	// Collect memories from both sources.
	var memories []memoryItem

	// 1. Knowledge-base memory search.
	if r.searcher != nil {
		kbResults, err := r.searcher.Search(ctx, session, &agentmemory.SearchRequest{
			Query:    userContent,
			TopK:     5,
			MinScore: 0.5,
		})
		if err == nil {
			for _, res := range kbResults {
				memories = append(memories, memoryItem{
					content: res.ChunkContent,
					score:   res.Score,
					source:  "knowledge_base",
				})
			}
		}
	}

	// 2. Agent-managed memory (vector search preferred, fallback to text search).
	if r.store != nil && agentInstance.ID != "" {
		vectorResults := r.vectorSearch(ctx, session, agentInstance, userContent)
		if len(vectorResults) > 0 {
			result.UsedVector = true
			for _, vr := range vectorResults {
				memories = append(memories, memoryItem{
					content: vr.Content,
					score:   scoreWithImportance(vr.Score, vr.Importance),
					source:  "agent_memory_vector",
				})
			}
		} else {
			// Fallback to text search.
			for _, memoryType := range agentManagedRetrievalTypes() {
				textResults, err := r.store.ListMemories(ctx, session.OrganizationID, session.User.ID, agentpkg.ListMemoriesRequest{
					AgentID: agentInstance.ID,
					Type:    memoryType,
					Query:   userContent,
					Limit:   5,
				})
				if err == nil {
					for _, mem := range textResults {
						if mem == nil {
							continue
						}
						memories = append(memories, memoryItem{
							content: mem.Content,
							score:   scoreWithImportance(1.0, mem.Importance),
							source:  "agent_memory_text",
						})
					}
				}
			}
		}
	}

	if len(memories) == 0 {
		return result
	}

	result.Searched = true

	// Deduplicate, rank, and take top 5.
	memories = deduplicateMemories(memories)
	sort.SliceStable(memories, func(i, j int) bool {
		if memories[i].score == memories[j].score {
			return memories[i].source < memories[j].source
		}
		return memories[i].score > memories[j].score
	})
	if len(memories) > 5 {
		memories = memories[:5]
	}
	result.MemoryCount = len(memories)

	// Inject into messages.
	result.Messages = injectMemories(memories, messages)
	return result
}

func (r *Retriever) vectorSearch(ctx context.Context, session auth.Session, agentInstance *agentpkg.Agent, query string) []agentMemoryItem {
	if r.embedder == nil || r.store == nil || strings.TrimSpace(query) == "" {
		return nil
	}
	embedding, err := r.embedder.Embed(ctx, query)
	if err != nil || len(embedding) == 0 {
		return nil
	}
	items := make([]agentMemoryItem, 0, 5)
	for _, memoryType := range agentManagedRetrievalTypes() {
		matches, err := r.store.SearchMemories(ctx, session.OrganizationID, session.User.ID, agentpkg.SearchMemoriesRequest{
			AgentID:   agentInstance.ID,
			Type:      memoryType,
			Embedding: embedding,
			Limit:     5,
			MinScore:  0.5,
		})
		if err != nil {
			continue
		}
		for _, m := range matches {
			if m == nil {
				continue
			}
			items = append(items, agentMemoryItem{
				Content:    m.Memory.Content,
				Score:      m.Score,
				Importance: m.Memory.Importance,
			})
		}
	}
	return items
}

func agentManagedRetrievalTypes() []string {
	return []string{agentpkg.MemoryTypeUserManaged, agentpkg.MemoryTypeLongTerm}
}

func scoreWithImportance(score float64, importance int) float64 {
	if score <= 0 {
		score = 0.5
	}
	if importance <= 0 {
		importance = 3
	}
	if importance > 5 {
		importance = 5
	}
	return score + float64(importance)*0.01
}

type memoryItem struct {
	content string
	score   float64
	source  string
}

type agentMemoryItem struct {
	Content    string
	Score      float64
	Importance int
}

// deduplicateMemories removes near-duplicate entries based on content.
func deduplicateMemories(items []memoryItem) []memoryItem {
	seen := make(map[string]memoryItem)
	for _, item := range items {
		key := normalizeMemoryItemContent(item.content)
		if key == "" {
			continue
		}
		existing, ok := seen[key]
		if !ok || item.score > existing.score {
			seen[key] = item
		}
	}
	result := make([]memoryItem, 0, len(seen))
	for _, item := range seen {
		result = append(result, item)
	}
	return result
}

func normalizeMemoryItemContent(content string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(content))), " ")
}

// injectMemories inserts a system message with memory context after the
// first system message in the list, or at the beginning if none exists.
func injectMemories(memories []memoryItem, messages []chat.Message) []chat.Message {
	var builder strings.Builder
	builder.WriteString("Relevant information from memory:\n\n")
	for i, mem := range memories {
		builder.WriteString(fmt.Sprintf("[%d] %s", i+1, mem.content))
		if i < len(memories)-1 {
			builder.WriteString("\n\n")
		}
	}
	memoryContent := builder.String()

	result := make([]chat.Message, 0, len(messages)+1)
	inserted := false
	for _, m := range messages {
		result = append(result, m)
		if !inserted && m.Role == "system" {
			result = append(result, chat.Message{
				Role:    "system",
				Content: memoryContent,
			})
			inserted = true
		}
	}

	if !inserted {
		result = append([]chat.Message{{
			Role:    "system",
			Content: memoryContent,
		}}, result...)
	}

	return result
}
