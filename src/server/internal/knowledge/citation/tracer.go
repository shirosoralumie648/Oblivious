package citation

import (
	"math"
	"strings"

	"oblivious/server/internal/knowledge"
)

// Tracer enriches retrieval results with full citation provenance:
// document name, page number, chunk index, highlighted positions, and a
// confidence score. It is stateless and safe for concurrent use.
type Tracer struct{}

// NewTracer returns a new citation Tracer.
func NewTracer() *Tracer { return &Tracer{} }

// EnrichedCitation bundles the citation trace with a computed confidence score.
type EnrichedCitation struct {
	knowledge.CitationTrace
	Confidence float64 `json:"confidence"`
}

// Trace builds a citation for a single retrieval result. The query is used
// to compute highlight positions and the confidence score.
func (t *Tracer) Trace(
	documentID, documentTitle, documentVersion string,
	chunkID string,
	chunkIndex int,
	pageNumber int,
	sourceURL string,
	originalText string,
	query string,
	retrievalScore float64,
) EnrichedCitation {
	query = strings.TrimSpace(query)
	matchedSnippet := buildMatchedSnippet(originalText, query)
	highlights := buildHighlights(originalText, query)
	confidence := computeConfidence(retrievalScore, query, originalText)

	return EnrichedCitation{
		CitationTrace: knowledge.CitationTrace{
			DocumentID:         documentID,
			DocumentTitle:      documentTitle,
			DocumentVersion:    documentVersion,
			ChunkID:            chunkID,
			ChunkIndex:         chunkIndex,
			PageNumber:         pageNumber,
			SourceURL:          sourceURL,
			OriginalText:       originalText,
			MatchedSnippet:     matchedSnippet,
			HighlightPositions: highlights,
			ConfidenceScore:    confidence,
		},
		Confidence: confidence,
	}
}

// TraceFromResult builds an EnrichedCitation from a HybridRetrievalResult.
func (t *Tracer) TraceFromResult(result knowledge.HybridRetrievalResult, query string) EnrichedCitation {
	return t.Trace(
		result.DocumentID,
		result.DocumentTitle,
		result.DocumentVersion,
		result.ChunkID,
		result.ChunkIndex,
		result.Citation.PageNumber,
		result.Citation.SourceURL,
		result.Citation.OriginalText,
		query,
		result.Score,
	)
}

// TraceBatch enriches a slice of retrieval results.
func (t *Tracer) TraceBatch(results []knowledge.HybridRetrievalResult, query string) []EnrichedCitation {
	out := make([]EnrichedCitation, len(results))
	for i, r := range results {
		out[i] = t.TraceFromResult(r, query)
	}
	return out
}

// ---------------------------------------------------------------------------
// Highlight position computation
// ---------------------------------------------------------------------------

// buildHighlights finds all occurrences of query terms in the original text
// and returns their character positions.
func buildHighlights(content, query string) []knowledge.HighlightPosition {
	if query == "" || content == "" {
		return nil
	}
	lowerContent := strings.ToLower(content)
	lowerQuery := strings.ToLower(query)

	var positions []knowledge.HighlightPosition

	// Try exact query match first.
	idx := 0
	for {
		pos := strings.Index(lowerContent[idx:], lowerQuery)
		if pos < 0 {
			break
		}
		absPos := idx + pos
		// Convert byte offsets to rune offsets for consistency.
		positions = append(positions, knowledge.HighlightPosition{
			Start: byteToRuneOffset(content, absPos),
			End:   byteToRuneOffset(content, absPos+len(query)),
		})
		idx = absPos + len(query)
	}

	// Also search for individual query terms.
	terms := strings.Fields(lowerQuery)
	for _, term := range terms {
		if len(term) < 2 {
			continue
		}
		termIdx := 0
		for {
			pos := strings.Index(lowerContent[termIdx:], term)
			if pos < 0 {
				break
			}
			absPos := termIdx + pos
			// Avoid duplicates from exact query match overlap.
			alreadyCovered := false
			for _, p := range positions {
				startRune := byteToRuneOffset(content, absPos)
				if startRune >= p.Start && startRune < p.End {
					alreadyCovered = true
					break
				}
			}
			if !alreadyCovered {
				positions = append(positions, knowledge.HighlightPosition{
					Start: byteToRuneOffset(content, absPos),
					End:   byteToRuneOffset(content, absPos+len(term)),
				})
			}
			termIdx = absPos + len(term)
		}
	}

	return positions
}

// byteToRuneOffset converts a byte offset to a rune offset.
func byteToRuneOffset(s string, byteOffset int) int {
	if byteOffset <= 0 {
		return 0
	}
	if byteOffset >= len(s) {
		return len([]rune(s))
	}
	return len([]rune(s[:byteOffset]))
}

// ---------------------------------------------------------------------------
// Matched snippet extraction
// ---------------------------------------------------------------------------

const maxSnippetLength = 220

// buildMatchedSnippet extracts a window around the query match in the content.
func buildMatchedSnippet(content, query string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	query = strings.TrimSpace(query)
	if query == "" {
		runes := []rune(content)
		if len(runes) <= maxSnippetLength {
			return content
		}
		return string(runes[:maxSnippetLength]) + "..."
	}

	lowerContent := strings.ToLower(content)
	lowerQuery := strings.ToLower(query)
	matchIdx := strings.Index(lowerContent, lowerQuery)
	if matchIdx < 0 {
		runes := []rune(content)
		if len(runes) <= maxSnippetLength {
			return content
		}
		return string(runes[:maxSnippetLength]) + "..."
	}

	runes := []rune(content)
	matchRunes := []rune(content[:matchIdx])
	queryRunes := []rune(query)
	start := len(matchRunes) - maxSnippetLength/3
	if start < 0 {
		start = 0
	}
	end := start + maxSnippetLength
	if end < len(matchRunes)+len(queryRunes) {
		end = len(matchRunes) + len(queryRunes)
	}
	if end > len(runes) {
		end = len(runes)
	}
	if end-start > maxSnippetLength && end == len(runes) {
		start = int(math.Max(0, float64(end-maxSnippetLength)))
	}
	snippet := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(runes) {
		snippet += "..."
	}
	return snippet
}

// ---------------------------------------------------------------------------
// Confidence scoring
// ---------------------------------------------------------------------------

// computeConfidence derives a 0-1 confidence score from the retrieval score,
// query-content term overlap, and text length.
func computeConfidence(retrievalScore float64, query, content string) float64 {
	if content == "" {
		return 0
	}

	// Base score from retrieval (normalised to 0-1 range).
	base := retrievalScore
	if base < 0 {
		base = 0
	}
	if base > 1 {
		base = 1
	}

	// Term coverage: what fraction of query terms appear in the content.
	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(terms) == 0 {
		return base
	}
	lowerContent := strings.ToLower(content)
	hits := 0
	for _, term := range terms {
		if len(term) >= 2 && strings.Contains(lowerContent, term) {
			hits++
		}
	}
	coverage := float64(hits) / float64(len(terms))

	// Length bonus: shorter chunks are more focused (higher confidence).
	runeCount := len([]rune(content))
	lengthFactor := 1.0
	if runeCount > 1000 {
		lengthFactor = 0.9
	}
	if runeCount > 2000 {
		lengthFactor = 0.8
	}

	confidence := (base*0.5 + coverage*0.4 + 0.1) * lengthFactor
	if confidence > 1.0 {
		confidence = 1.0
	}
	return math.Round(confidence*1000) / 1000
}
