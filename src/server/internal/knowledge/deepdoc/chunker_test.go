package deepdoc

import (
	"fmt"
	"strings"
	"testing"

	"oblivious/server/internal/knowledge"
)

// knowledgeChunkAlias keeps test helpers readable.
type knowledgeChunkAlias = knowledge.EngineDocumentChunk

func TestChunkAddsBreadcrumbPrefixes(t *testing.T) {
	md := strings.Join([]string{
		"# Manual",
		"## Install",
		"Run the installer.",
		"## Configure",
		"Edit the config file.",
	}, "\n")
	chunks := NewChunker(0).Chunk(ParseMarkdown(md))
	if len(chunks) != 2 {
		t.Fatalf("chunks = %d, want 2: %+v", len(chunks), chunks)
	}
	if !strings.HasPrefix(chunks[0].Content, "Manual > Install") {
		t.Fatalf("chunk 0 missing breadcrumb: %q", chunks[0].Content)
	}
	if !strings.Contains(chunks[0].Content, "Run the installer.") {
		t.Fatalf("chunk 0 missing body: %q", chunks[0].Content)
	}
	if !strings.HasPrefix(chunks[1].Content, "Manual > Configure") {
		t.Fatalf("chunk 1 missing breadcrumb: %q", chunks[1].Content)
	}
	for i, c := range chunks {
		if c.Index != i {
			t.Fatalf("chunk %d has index %d", i, c.Index)
		}
	}
	if chunks[1].StartRune <= chunks[0].StartRune {
		t.Fatalf("rune offsets not increasing: %+v", chunks)
	}
}

func TestChunkNeverSplitsTableRowsAndRepeatsHeader(t *testing.T) {
	table := &Table{HasHeader: true}
	table.Rows = append(table.Rows, []string{"id", "payload"})
	for i := 0; i < 40; i++ {
		table.Rows = append(table.Rows, []string{
			fmt.Sprintf("row-%02d", i),
			strings.Repeat("x", 60),
		})
	}
	doc := NewDocumentStructure(FormatCSV)
	doc.Root.Blocks = append(doc.Root.Blocks, Block{Type: BlockTable, Table: table})

	chunks := NewChunker(100).Chunk(doc) // 400-rune budget forces splits
	if len(chunks) < 2 {
		t.Fatalf("expected the table to split, got %d chunk(s)", len(chunks))
	}
	seen := map[string]bool{}
	for _, chunk := range chunks {
		lines := strings.Split(chunk.Content, "\n")
		if !strings.Contains(lines[0], "| id | payload |") {
			t.Fatalf("chunk missing repeated header: %q", chunk.Content)
		}
		for _, line := range lines {
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "| ") || !strings.HasSuffix(line, " |") {
				t.Fatalf("table row split or malformed: %q", line)
			}
			if strings.HasPrefix(line, "| row-") {
				id := strings.TrimSpace(strings.Split(line, "|")[1])
				if seen[id] {
					t.Fatalf("row %s duplicated across chunks", id)
				}
				seen[id] = true
			}
		}
	}
	if len(seen) != 40 {
		t.Fatalf("rows lost in chunking: got %d of 40", len(seen))
	}
}

func TestChunkRespectsTokenBudget(t *testing.T) {
	var paragraphs []string
	for i := 0; i < 30; i++ {
		paragraphs = append(paragraphs, fmt.Sprintf("Paragraph %d with several words of content to occupy space.", i))
	}
	md := "# Doc\n\n" + strings.Join(paragraphs, "\n\n")
	maxTokens := 80
	chunks := NewChunker(maxTokens).Chunk(ParseMarkdown(md))
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	limit := maxTokens * runesPerToken * 2 // generous upper bound: budget + one overflow piece
	for _, chunk := range chunks {
		if got := len([]rune(chunk.Content)); got > limit {
			t.Fatalf("chunk exceeds budget: %d runes > %d", got, limit)
		}
	}
}

func TestChunkRendersTablesAsMarkdownAndEscapesPipes(t *testing.T) {
	doc := NewDocumentStructure(FormatCSV)
	doc.Root.Blocks = append(doc.Root.Blocks, Block{Type: BlockTable, Table: &Table{
		HasHeader: true,
		Caption:   "Litmus",
		Rows:      [][]string{{"key", "value"}, {"a|b", "10"}},
	}})
	chunks := NewChunker(0).Chunk(doc)
	if len(chunks) != 1 {
		t.Fatalf("chunks = %d, want 1", len(chunks))
	}
	content := chunks[0].Content
	for _, want := range []string{"Litmus", "| key | value |", "| --- | --- |", "| a\\|b | 10 |"} {
		if !strings.Contains(content, want) {
			t.Fatalf("chunk missing %q:\n%s", want, content)
		}
	}
}

func TestChunkSplitsCodeBlocksAtLineBoundaries(t *testing.T) {
	var lines []string
	for i := 0; i < 60; i++ {
		lines = append(lines, fmt.Sprintf("line_%02d := compute(%d)", i, i))
	}
	doc := NewDocumentStructure(FormatMarkdown)
	doc.Root.Blocks = append(doc.Root.Blocks, Block{Type: BlockCode, Language: "go", Text: strings.Join(lines, "\n")})

	chunks := NewChunker(60).Chunk(doc)
	if len(chunks) < 2 {
		t.Fatalf("expected code to split, got %d chunk(s)", len(chunks))
	}
	count := 0
	for _, chunk := range chunks {
		if !strings.Contains(chunk.Content, "```go") || !strings.Contains(chunk.Content, "```") {
			t.Fatalf("chunk not fenced: %q", chunk.Content)
		}
		for _, line := range strings.Split(chunk.Content, "\n") {
			if strings.HasPrefix(line, "line_") {
				if !strings.Contains(line, ":= compute(") {
					t.Fatalf("code line split mid-line: %q", line)
				}
				count++
			}
		}
	}
	if count != 60 {
		t.Fatalf("code lines lost: %d of 60", count)
	}
}

func TestChunkEmptyAndNilInputs(t *testing.T) {
	if got := NewChunker(0).Chunk(nil); got != nil {
		t.Fatalf("nil doc => %+v", got)
	}
	if got := NewChunker(0).Chunk(NewDocumentStructure(FormatText)); got != nil {
		t.Fatalf("empty doc => %+v", got)
	}
}

func TestChunkEndToEndFromMarkdownDocument(t *testing.T) {
	md := strings.Join([]string{
		"# Product Spec",
		"",
		"Overview paragraph.",
		"",
		"## Pricing",
		"",
		"| Plan | Price |",
		"| --- | --- |",
		"| Free | 0 |",
		"| Pro | 99 |",
		"",
		"## FAQ",
		"",
		"- How? Easily.",
		"- When? Now.",
	}, "\n")
	doc, err := Parse("spec.md", []byte(md))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	chunks := NewChunker(0).Chunk(doc)
	if len(chunks) == 0 {
		t.Fatal("no chunks produced")
	}
	joined := strings.Join(collectContents(chunks), "\n---\n")
	for _, want := range []string{"Product Spec > Pricing", "| Pro | 99 |", "Product Spec > FAQ", "- How? Easily."} {
		if !strings.Contains(joined, want) {
			t.Fatalf("end-to-end output missing %q:\n%s", want, joined)
		}
	}
}

func collectContents(chunks []knowledgeChunkAlias) []string {
	out := make([]string, len(chunks))
	for i, c := range chunks {
		out[i] = c.Content
	}
	return out
}
