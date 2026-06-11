package knowledge

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"
)

type mockEmbedder struct{}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = []float32{0.1, 0.2, 0.3}
	}
	return result, nil
}

type mockQdrant struct {
	chunks []KnowledgeDocumentChunk
}

func (m *mockQdrant) UpsertKnowledgeDocumentChunks(ctx context.Context, orgID, kbID, docID string, chunks []KnowledgeDocumentChunk) error {
	m.chunks = append(m.chunks, chunks...)
	return nil
}

func (m *mockQdrant) EnsureKnowledgeBaseCollection(ctx context.Context, orgID, kbID string, vectorSize int) error {
	return nil
}

func (m *mockQdrant) DeleteKnowledgeBaseCollection(ctx context.Context, orgID, kbID string) error {
	return nil
}

func (m *mockQdrant) SearchKnowledgeChunks(ctx context.Context, orgID, kbID, query string, queryEmb []float32, opts KnowledgeRetrievalOptions) ([]KnowledgeRetrievalResult, error) {
	return nil, nil
}

func TestProcessor_ProcessDOCX(t *testing.T) {
	docx := createTestDOCX(t)

	embedder := &mockEmbedder{}
	qdrant := &mockQdrant{}
	processor := NewProcessor(embedder, qdrant)

	doc := &Document{
		ID:            "test-doc-1",
		Content:       docx,
		Format:        "docx",
		ChunkStrategy: "semantic",
		ChunkSize:     500,
		ChunkOverlap:  50,
	}

	err := processor.Process(context.Background(), doc)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if len(qdrant.chunks) == 0 {
		t.Fatal("Expected chunks to be created")
	}

	foundStructure := false
	for _, chunk := range qdrant.chunks {
		if chunk.Content != "" {
			foundStructure = true
			t.Logf("Chunk %d: %d chars", chunk.ChunkIndex, len(chunk.Content))
			if chunk.Metadata.Extra != nil {
				t.Logf("  Metadata: %+v", chunk.Metadata.Extra)
			}
		}
	}

	if !foundStructure {
		t.Error("Expected chunks to contain structure information")
	}

	t.Logf("Successfully processed DOCX: %d chunks", len(qdrant.chunks))
}

func createTestDOCX(t *testing.T) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	documentXML := `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:pPr><w:pStyle w:val="Heading1"/></w:pPr>
      <w:r><w:t>Introduction</w:t></w:r>
    </w:p>
    <w:p>
      <w:r><w:t>This is the introduction paragraph with some content.</w:t></w:r>
    </w:p>
    <w:p>
      <w:pPr><w:pStyle w:val="Heading2"/></w:pPr>
      <w:r><w:t>Technical Details</w:t></w:r>
    </w:p>
    <w:p>
      <w:r><w:t>This section contains technical information about the system.</w:t></w:r>
    </w:p>
    <w:tbl>
      <w:tr>
        <w:tc><w:p><w:r><w:t>Feature</w:t></w:r></w:p></w:tc>
        <w:tc><w:p><w:r><w:t>Status</w:t></w:r></w:p></w:tc>
      </w:tr>
      <w:tr>
        <w:tc><w:p><w:r><w:t>API</w:t></w:r></w:p></w:tc>
        <w:tc><w:p><w:r><w:t>Active</w:t></w:r></w:p></w:tc>
      </w:tr>
      <w:tr>
        <w:tc><w:p><w:r><w:t>Database</w:t></w:r></w:p></w:tc>
        <w:tc><w:p><w:r><w:t>Ready</w:t></w:r></w:p></w:tc>
      </w:tr>
    </w:tbl>
    <w:p>
      <w:pPr><w:numPr/></w:pPr>
      <w:r><w:t>First item in list</w:t></w:r>
    </w:p>
    <w:p>
      <w:pPr><w:numPr/></w:pPr>
      <w:r><w:t>Second item in list</w:t></w:r>
    </w:p>
  </w:body>
</w:document>`

	f, err := w.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(documentXML)); err != nil {
		t.Fatal(err)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func TestProcessor_ChunkStrategies(t *testing.T) {
	embedder := &mockEmbedder{}
	qdrant := &mockQdrant{}
	processor := NewProcessor(embedder, qdrant)

	content := strings.Repeat("test content ", 100)

	tests := []struct {
		name     string
		strategy string
	}{
		{"FixedSize", "fixed_size"},
		{"Semantic", "semantic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qdrant.chunks = nil
			doc := &Document{
				ID:            "test-doc",
				Content:       []byte(content),
				Format:        "text",
				ChunkStrategy: tt.strategy,
				ChunkSize:     200,
				ChunkOverlap:  20,
			}

			err := processor.Process(context.Background(), doc)
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}

			if len(qdrant.chunks) == 0 {
				t.Errorf("Strategy %s produced no chunks", tt.strategy)
			}

			t.Logf("Strategy %s: %d chunks", tt.strategy, len(qdrant.chunks))
		})
	}
}

func TestProcessor_StructureMetadata(t *testing.T) {
	docx := createComplexDOCX(t)

	embedder := &mockEmbedder{}
	qdrant := &mockQdrant{}
	processor := NewProcessor(embedder, qdrant)

	doc := &Document{
		ID:            "test-doc-structure",
		Content:       docx,
		Format:        "docx",
		ChunkStrategy: "semantic",
		ChunkSize:     300,
	}

	err := processor.Process(context.Background(), doc)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if len(qdrant.chunks) == 0 {
		t.Fatal("Expected chunks to be created")
	}

	hasStructureInfo := false
	for _, chunk := range qdrant.chunks {
		if chunk.Metadata.Extra != nil {
			if breadcrumb, ok := chunk.Metadata.Extra["breadcrumb"].(string); ok && breadcrumb != "" {
				hasStructureInfo = true
				t.Logf("Chunk %d has breadcrumb: %s", chunk.ChunkIndex, breadcrumb)
			}
		}
	}

	if !hasStructureInfo {
		t.Error("Expected at least one chunk to have structure metadata (breadcrumb)")
	}
}

func createComplexDOCX(t *testing.T) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	documentXML := `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:pPr><w:pStyle w:val="Heading1"/></w:pPr>
      <w:r><w:t>Chapter 1: Getting Started</w:t></w:r>
    </w:p>
    <w:p>
      <w:r><w:t>This chapter introduces the fundamental concepts.</w:t></w:r>
    </w:p>
    <w:p>
      <w:pPr><w:pStyle w:val="Heading2"/></w:pPr>
      <w:r><w:t>Section 1.1: Prerequisites</w:t></w:r>
    </w:p>
    <w:p>
      <w:r><w:t>Before you begin, ensure you have the following installed on your system.</w:t></w:r>
    </w:p>
    <w:p>
      <w:pPr><w:pStyle w:val="Heading1"/></w:pPr>
      <w:r><w:t>Chapter 2: Advanced Topics</w:t></w:r>
    </w:p>
    <w:p>
      <w:r><w:t>This chapter covers more advanced material for experienced users.</w:t></w:r>
    </w:p>
  </w:body>
</w:document>`

	f, err := w.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(documentXML)); err != nil {
		t.Fatal(err)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}
