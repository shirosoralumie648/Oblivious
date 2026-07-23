package deepdoc

import (
	"strings"

	"oblivious/server/internal/knowledge"
)

const (
	// DefaultMaxTokens approximates the chunk budget used elsewhere in the
	// knowledge pipeline (engine default 500 runes ~ 500 token-ish budget).
	DefaultMaxTokens = 500
	// runesPerToken is the rune→token approximation (runes/4 ≈ tokens).
	runesPerToken = 4
)

// Chunker converts a DocumentStructure into structure-aware chunks aligned
// with the knowledge chunking engine's output type.
type Chunker struct {
	// MaxTokens caps each chunk's approximate token count (runes/4).
	MaxTokens int
}

// NewChunker returns a Chunker with the given token budget (<=0 uses default).
func NewChunker(maxTokens int) *Chunker {
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	return &Chunker{MaxTokens: maxTokens}
}

// Chunk walks the section tree, emitting chunks that carry a heading
// breadcrumb header, never split table rows, and respect the token budget.
func (c *Chunker) Chunk(doc *DocumentStructure) []knowledge.EngineDocumentChunk {
	if doc == nil || doc.Root == nil {
		return nil
	}
	state := &chunkState{maxRunes: c.MaxTokens * runesPerToken}
	c.walkSection(state, doc.Root, nil)
	state.flush()
	return state.chunks
}

type chunkState struct {
	maxRunes   int
	chunks     []knowledge.EngineDocumentChunk
	runeOffset int

	breadcrumb string
	body       []string
	bodyRunes  int
}

func (s *chunkState) headerRunes() int {
	if s.breadcrumb == "" {
		return 0
	}
	return len([]rune(s.breadcrumb)) + 2
}

func (s *chunkState) flush() {
	if len(s.body) == 0 {
		return
	}
	var parts []string
	if s.breadcrumb != "" {
		parts = append(parts, s.breadcrumb)
	}
	parts = append(parts, s.body...)
	content := strings.TrimSpace(strings.Join(parts, "\n\n"))
	s.body = nil
	s.bodyRunes = 0
	if content == "" {
		return
	}
	runeLen := len([]rune(content))
	s.chunks = append(s.chunks, knowledge.EngineDocumentChunk{
		Index:     len(s.chunks),
		Content:   content,
		StartRune: s.runeOffset,
		EndRune:   s.runeOffset + runeLen,
	})
	s.runeOffset += runeLen + 2
}

// setBreadcrumb flushes pending content when the heading context changes.
func (s *chunkState) setBreadcrumb(breadcrumb string) {
	if breadcrumb == s.breadcrumb {
		return
	}
	s.flush()
	s.breadcrumb = breadcrumb
}

// add appends a rendered piece, flushing first when it would overflow.
func (s *chunkState) add(piece string) {
	runes := len([]rune(piece))
	if s.bodyRunes > 0 && s.headerRunes()+s.bodyRunes+runes+2 > s.maxRunes {
		s.flush()
	}
	s.body = append(s.body, piece)
	s.bodyRunes += runes + 2
}

func (c *Chunker) walkSection(state *chunkState, section *Section, trail []string) {
	if section.Title != "" {
		trail = append(append([]string(nil), trail...), section.Title)
	}
	state.setBreadcrumb(strings.Join(trail, " > "))

	for _, block := range section.Blocks {
		switch block.Type {
		case BlockTable:
			c.addTable(state, block.Table)
		case BlockList:
			c.addList(state, block.Items)
		case BlockCode:
			c.addCode(state, block)
		default:
			c.addParagraph(state, block.Text)
		}
	}
	for _, child := range section.Children {
		c.walkSection(state, child, trail)
		// Returning to the parent context restores its breadcrumb for any
		// hypothetical trailing content; harmless when none follows.
		state.setBreadcrumb(strings.Join(trail, " > "))
	}
}

func (c *Chunker) addParagraph(state *chunkState, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	budget := state.maxRunes - state.headerRunes()
	if budget < 1 {
		budget = state.maxRunes
	}
	for _, piece := range splitRunesBySpace(text, budget) {
		state.add(piece)
	}
}

func (c *Chunker) addList(state *chunkState, items []string) {
	var lines []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			lines = append(lines, "- "+item)
		}
	}
	if len(lines) == 0 {
		return
	}
	// Lists may split between items: feed greedily.
	var group []string
	groupRunes := 0
	budget := state.maxRunes - state.headerRunes()
	flushGroup := func() {
		if len(group) > 0 {
			state.add(strings.Join(group, "\n"))
			group = nil
			groupRunes = 0
		}
	}
	for _, line := range lines {
		runes := len([]rune(line)) + 1
		if groupRunes > 0 && groupRunes+runes > budget {
			flushGroup()
		}
		group = append(group, line)
		groupRunes += runes
	}
	flushGroup()
}

func (c *Chunker) addCode(state *chunkState, block Block) {
	text := strings.Trim(block.Text, "\n")
	if strings.TrimSpace(text) == "" {
		return
	}
	fenceOpen := "```" + block.Language
	budget := state.maxRunes - state.headerRunes() - len([]rune(fenceOpen)) - 8
	if budget < 1 {
		budget = state.maxRunes
	}
	lines := strings.Split(text, "\n")
	var group []string
	groupRunes := 0
	flushGroup := func() {
		if len(group) > 0 {
			state.add(fenceOpen + "\n" + strings.Join(group, "\n") + "\n```")
			group = nil
			groupRunes = 0
		}
	}
	for _, line := range lines {
		runes := len([]rune(line)) + 1
		if groupRunes > 0 && groupRunes+runes > budget {
			flushGroup()
		}
		group = append(group, line)
		groupRunes += runes
	}
	flushGroup()
}

// addTable renders a table as markdown, splitting between rows only —
// a row is never split, and continuation chunks repeat the header row.
func (c *Chunker) addTable(state *chunkState, table *Table) {
	if table == nil || len(table.Rows) == 0 {
		return
	}
	width := 0
	for _, row := range table.Rows {
		if len(row) > width {
			width = len(row)
		}
	}
	if width == 0 {
		return
	}

	var headerLines []string
	rows := table.Rows
	if table.HasHeader && len(rows) > 1 {
		headerLines = []string{renderTableRow(rows[0], width), renderTableSeparator(width)}
		rows = rows[1:]
	}
	caption := strings.TrimSpace(table.Caption)

	budget := state.maxRunes - state.headerRunes()
	headerRunes := 0
	for _, line := range headerLines {
		headerRunes += len([]rune(line)) + 1
	}
	if caption != "" {
		headerRunes += len([]rune(caption)) + 1
	}

	var group []string
	groupRunes := 0
	flushGroup := func() {
		if len(group) == 0 {
			return
		}
		var lines []string
		if caption != "" {
			lines = append(lines, caption)
		}
		lines = append(lines, headerLines...)
		lines = append(lines, group...)
		state.add(strings.Join(lines, "\n"))
		group = nil
		groupRunes = 0
	}
	for _, row := range rows {
		line := renderTableRow(row, width)
		runes := len([]rune(line)) + 1
		// Never split a row: oversize rows ship whole in their own chunk.
		if groupRunes > 0 && headerRunes+groupRunes+runes > budget {
			flushGroup()
		}
		group = append(group, line)
		groupRunes += runes
	}
	flushGroup()
}

func renderTableRow(row []string, width int) string {
	cells := make([]string, width)
	for i := 0; i < width; i++ {
		if i < len(row) {
			cells[i] = strings.ReplaceAll(row[i], "|", "\\|")
		}
	}
	return "| " + strings.Join(cells, " | ") + " |"
}

func renderTableSeparator(width int) string {
	cells := make([]string, width)
	for i := range cells {
		cells[i] = "---"
	}
	return "| " + strings.Join(cells, " | ") + " |"
}

// splitRunesBySpace splits text into pieces of at most maxRunes runes,
// preferring space boundaries (mirrors the chunking engine's splitText).
func splitRunesBySpace(text string, maxRunes int) []string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return []string{text}
	}
	var pieces []string
	start := 0
	for start < len(runes) {
		end := start + maxRunes
		if end >= len(runes) {
			pieces = append(pieces, strings.TrimSpace(string(runes[start:])))
			break
		}
		split := end
		for split > start && runes[split-1] != ' ' {
			split--
		}
		if split == start {
			split = end
		}
		piece := strings.TrimSpace(string(runes[start:split]))
		if piece != "" {
			pieces = append(pieces, piece)
		}
		start = split
		for start < len(runes) && runes[start] == ' ' {
			start++
		}
	}
	return pieces
}
