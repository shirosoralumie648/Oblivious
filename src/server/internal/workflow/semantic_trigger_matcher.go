package workflow

import (
	"context"
	"math"
	"strings"

	relaytypes "oblivious/server/internal/relay/types"
)

type SemanticTriggerEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

type EmbeddingSemanticTriggerMatcher struct {
	embedder SemanticTriggerEmbedder
}

func NewEmbeddingSemanticTriggerMatcher(embedder SemanticTriggerEmbedder) *EmbeddingSemanticTriggerMatcher {
	return &EmbeddingSemanticTriggerMatcher{embedder: embedder}
}

func (m *EmbeddingSemanticTriggerMatcher) MatchSemanticTrigger(ctx context.Context, req SemanticTriggerMatchRequest) (SemanticTriggerMatchDecision, error) {
	if m == nil || m.embedder == nil || strings.TrimSpace(req.Message) == "" || req.Threshold <= 0 {
		return SemanticTriggerMatchDecision{}, nil
	}
	keywords := normalizedSemanticTriggerKeywords(req.Keywords)
	if len(keywords) == 0 {
		return SemanticTriggerMatchDecision{}, nil
	}

	ctx = withSemanticTriggerRelayIdentity(ctx, req)
	queryEmbedding, err := m.embedder.Embed(ctx, strings.TrimSpace(req.Message))
	if err != nil {
		return SemanticTriggerMatchDecision{}, err
	}
	keywordEmbeddings, err := m.embedder.EmbedBatch(ctx, keywords)
	if err != nil {
		return SemanticTriggerMatchDecision{}, err
	}

	bestKeyword := ""
	bestScore := 0.0
	for index, keyword := range keywords {
		if index >= len(keywordEmbeddings) {
			break
		}
		score := semanticTriggerEmbeddingSimilarity(queryEmbedding, keywordEmbeddings[index])
		if score > bestScore {
			bestKeyword = keyword
			bestScore = score
		}
	}
	if bestScore < req.Threshold || bestKeyword == "" {
		return SemanticTriggerMatchDecision{Score: bestScore, MatchMethod: "embedding"}, nil
	}
	return SemanticTriggerMatchDecision{
		Matched:     true,
		Keyword:     bestKeyword,
		Score:       bestScore,
		MatchMethod: "embedding",
	}, nil
}

func normalizedSemanticTriggerKeywords(keywords []string) []string {
	normalized := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		if trimmed := strings.TrimSpace(keyword); trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return normalized
}

func withSemanticTriggerRelayIdentity(ctx context.Context, req SemanticTriggerMatchRequest) context.Context {
	if strings.TrimSpace(req.UserID) != "" {
		ctx = relaytypes.WithTrustedUserID(ctx, strings.TrimSpace(req.UserID))
	}
	if strings.TrimSpace(req.OrganizationID) != "" {
		ctx = relaytypes.WithTrustedOrganizationID(ctx, strings.TrimSpace(req.OrganizationID))
	}
	return ctx
}

func semanticTriggerEmbeddingSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, aNorm, bNorm float64
	for index := range a {
		av := float64(a[index])
		bv := float64(b[index])
		dot += av * bv
		aNorm += av * av
		bNorm += bv * bv
	}
	if aNorm == 0 || bNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(aNorm) * math.Sqrt(bNorm))
}
