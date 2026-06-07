package retrieval

import (
	"context"

	"oblivious/server/internal/knowledge"
)

type KnowledgeResultReranker struct {
	reranker *Reranker
}

func NewKnowledgeResultReranker(cfg knowledge.RerankerConfig) *KnowledgeResultReranker {
	return &KnowledgeResultReranker{reranker: NewReranker(cfg)}
}

func (r *KnowledgeResultReranker) Rerank(ctx context.Context, query string, results []knowledge.KnowledgeRetrievalResult, limit int) ([]knowledge.KnowledgeRetrievalResult, error) {
	if r == nil || r.reranker == nil {
		return results, nil
	}
	hybridResults := make([]knowledge.HybridRetrievalResult, len(results))
	for index, result := range results {
		hybridResults[index] = knowledge.HybridRetrievalResult{
			ChunkID:         result.ChunkID,
			ChunkIndex:      result.ChunkIndex,
			DocumentID:      result.DocumentID,
			DocumentTitle:   result.DocumentTitle,
			DocumentVersion: result.DocumentVersion,
			RetrievalMethod: result.RetrievalMethod,
			Score:           result.Score,
			Snippet:         result.Snippet,
			Citation:        result.Source,
		}
	}
	reranked, err := r.reranker.Rerank(ctx, query, hybridResults, limit)
	if err != nil {
		return nil, err
	}
	knowledgeResults := make([]knowledge.KnowledgeRetrievalResult, len(reranked))
	for index, result := range reranked {
		knowledgeResults[index] = knowledge.KnowledgeRetrievalResult{
			ChunkID:         result.ChunkID,
			ChunkIndex:      result.ChunkIndex,
			DocumentID:      result.DocumentID,
			DocumentTitle:   result.DocumentTitle,
			DocumentVersion: result.DocumentVersion,
			RetrievalMethod: result.RetrievalMethod,
			Score:           result.Score,
			Snippet:         result.Snippet,
			Source:          result.Citation,
		}
	}
	return knowledgeResults, nil
}
