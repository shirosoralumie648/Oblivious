package knowledge

import (
	"context"
	"fmt"
	"log"
	"strings"
)

type Processor struct {
	embedder Embedder
	qdrant   KnowledgeVectorStore
}

func NewProcessor(embedder Embedder, qdrant KnowledgeVectorStore) *Processor {
	return &Processor{embedder: embedder, qdrant: qdrant}
}

type Document struct {
	ID              string
	Title           string
	Content         []byte
	Format          string
	ChunkStrategy   string
	ChunkSize       int
	ChunkOverlap    int
	GenerationModel string
}

type Chunk struct {
	Content    string
	Section    string
	Breadcrumb string
	Page       int
}

func (p *Processor) Process(ctx context.Context, doc *Document) error {
	parsed, err := parseDocument(doc.Content, doc.Format)
	if err != nil {
		return fmt.Errorf("deepdoc parse failed: %w", err)
	}

	log.Printf("deepdoc parsed format=%s, sections=%d, tables=%d", parsed.Format, countSections(parsed.Root), countTables(parsed.Root))
	documentTitle := processorDocumentTitle(doc, parsed)

	var chunks []Chunk
	switch doc.ChunkStrategy {
	case "fixed_size":
		chunks = p.chunkFixedSize(parsed, doc.ChunkSize, doc.ChunkOverlap)
	case "semantic":
		if !isEmptyStructure(parsed) {
			chunks = p.chunkSemantic(parsed, doc.ChunkSize)
		} else {
			chunks = p.chunkFixedSize(parsed, doc.ChunkSize, doc.ChunkOverlap)
		}
	default:
		chunks = p.chunkFixedSize(parsed, doc.ChunkSize, doc.ChunkOverlap)
	}

	for i, chunk := range chunks {
		embedding, err := p.embedder.Embed(ctx, chunk.Content)
		if err != nil {
			return fmt.Errorf("embedding failed: %w", err)
		}

		err = p.qdrant.UpsertKnowledgeDocumentChunks(ctx, "default", "kb_default", doc.ID, []KnowledgeDocumentChunk{{
			ChunkIndex:    i,
			Content:       chunk.Content,
			DocumentTitle: documentTitle,
			Embedding:     embedding,
			Metadata: KnowledgeChunkMetadata{
				Extra: map[string]any{
					"section":    chunk.Section,
					"breadcrumb": chunk.Breadcrumb,
					"page":       chunk.Page,
				},
			},
		}})
		if err != nil {
			return fmt.Errorf("qdrant upsert failed: %w", err)
		}
	}

	return nil
}

func processorDocumentTitle(doc *Document, parsed *DocumentStructure) string {
	if doc != nil {
		if title := strings.TrimSpace(doc.Title); title != "" {
			return title
		}
	}
	if parsed != nil && parsed.Root != nil {
		for _, child := range parsed.Root.Children {
			if child != nil {
				if title := strings.TrimSpace(child.Title); title != "" {
					return title
				}
			}
		}
	}
	if doc != nil {
		if title := strings.TrimSpace(doc.ID); title != "" {
			return title
		}
	}
	return "Untitled document"
}

func (p *Processor) chunkFixedSize(parsed *DocumentStructure, chunkSize, overlap int) []Chunk {
	engineChunks := chunkDocumentStructure(parsed, chunkSize)
	chunks := make([]Chunk, len(engineChunks))
	for i, ec := range engineChunks {
		chunks[i] = Chunk{
			Content:    ec.Content,
			Breadcrumb: extractBreadcrumb(ec.Content),
		}
	}
	return chunks
}

func (p *Processor) chunkSemantic(parsed *DocumentStructure, chunkSize int) []Chunk {
	engineChunks := chunkDocumentStructure(parsed, chunkSize)
	chunks := make([]Chunk, len(engineChunks))
	for i, ec := range engineChunks {
		chunks[i] = Chunk{
			Content:    ec.Content,
			Breadcrumb: extractBreadcrumb(ec.Content),
		}
	}
	return chunks
}

func extractBreadcrumb(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 {
		first := strings.TrimSpace(lines[0])
		if !strings.HasSuffix(first, ".") && len(first) < 80 {
			return first
		}
	}
	return ""
}
