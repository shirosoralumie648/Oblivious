package handler

import (
	"context"
	"strings"

	relaycache "oblivious/server/internal/relay/cache"
)

type SemanticCacheEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

func attachSemanticCacheEmbedding(ctx context.Context, req relaycache.SemanticCacheRequest, embedder SemanticCacheEmbedder) relaycache.SemanticCacheRequest {
	if embedder == nil || strings.TrimSpace(req.Query) == "" {
		return req
	}
	embedding, err := embedder.Embed(ctx, req.Query)
	if err != nil || len(embedding) == 0 {
		return req
	}
	req.QueryEmbedding = append([]float32(nil), embedding...)
	return req
}
