package knowledge

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
)

type BlockType string

const (
	BlockParagraph BlockType = "paragraph"
	BlockTable     BlockType = "table"
	BlockList      BlockType = "list"
)

type Block struct {
	Type  BlockType
	Text  string
	Items []string
	Table *Table
}

type Table struct {
	Rows      [][]string
	HasHeader bool
}

type Section struct {
	Title    string
	Level    int
	Blocks   []Block
	Children []*Section
}

type DocumentStructure struct {
	Format string
	Root   *Section
}

func parseDocument(data []byte, format string) (*DocumentStructure, error) {
	switch format {
	case "docx":
		return parseDOCX(data)
	case "text":
		return parseText(string(data))
	default:
		return parseText(string(data))
	}
}

func parseDOCX(data []byte) (*DocumentStructure, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	var docXML []byte
	for _, file := range reader.File {
		if file.Name == "word/document.xml" {
			rc, _ := file.Open()
			docXML, _ = io.ReadAll(rc)
			rc.Close()
			break
		}
	}

	doc := &DocumentStructure{Format: "docx", Root: &Section{Level: 0}}
	builder := &sectionBuilder{root: doc.Root, stack: []*Section{doc.Root}}

	decoder := xml.NewDecoder(bytes.NewReader(docXML))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local == "p" {
			text, style := parseParagraph(decoder, start)
			text = normalizeSpace(text)
			if text == "" {
				continue
			}
			if level := headingLevel(style); level > 0 {
				builder.startSection(text, level)
			} else {
				builder.addBlock(Block{Type: BlockParagraph, Text: text})
			}
		}
	}
	return doc, nil
}

func parseText(content string) (*DocumentStructure, error) {
	doc := &DocumentStructure{Format: "text", Root: &Section{Level: 0}}
	doc.Root.Blocks = []Block{{Type: BlockParagraph, Text: content}}
	return doc, nil
}

func parseParagraph(decoder *xml.Decoder, start xml.StartElement) (string, string) {
	var sb strings.Builder
	var style string
	depth := 1
	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch el := token.(type) {
		case xml.StartElement:
			depth++
			if el.Name.Local == "pStyle" {
				for _, attr := range el.Attr {
					if attr.Name.Local == "val" {
						style = attr.Value
					}
				}
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			sb.Write(el)
		}
	}
	return sb.String(), style
}

func headingLevel(style string) int {
	lower := strings.ToLower(style)
	if !strings.HasPrefix(lower, "heading") {
		return 0
	}
	last := style[len(style)-1]
	if last >= '1' && last <= '6' {
		return int(last - '0')
	}
	return 1
}

func normalizeSpace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

type sectionBuilder struct {
	root  *Section
	stack []*Section
}

func (b *sectionBuilder) current() *Section {
	return b.stack[len(b.stack)-1]
}

func (b *sectionBuilder) startSection(title string, level int) {
	for len(b.stack) > 1 && b.current().Level >= level {
		b.stack = b.stack[:len(b.stack)-1]
	}
	section := &Section{Title: title, Level: level}
	parent := b.current()
	parent.Children = append(parent.Children, section)
	b.stack = append(b.stack, section)
}

func (b *sectionBuilder) addBlock(block Block) {
	b.current().Blocks = append(b.current().Blocks, block)
}

func countSections(s *Section) int {
	if s == nil {
		return 0
	}
	count := 0
	if s.Title != "" {
		count = 1
	}
	for _, child := range s.Children {
		count += countSections(child)
	}
	return count
}

func countTables(s *Section) int {
	if s == nil {
		return 0
	}
	count := 0
	for _, block := range s.Blocks {
		if block.Type == BlockTable {
			count++
		}
	}
	for _, child := range s.Children {
		count += countTables(child)
	}
	return count
}

func isEmptyStructure(doc *DocumentStructure) bool {
	if doc == nil || doc.Root == nil {
		return true
	}
	return countSections(doc.Root) == 0 && len(doc.Root.Blocks) == 0
}

func chunkDocumentStructure(doc *DocumentStructure, maxTokens int) []EngineDocumentChunk {
	if doc == nil || doc.Root == nil {
		return nil
	}

	maxRunes := maxTokens * 4
	state := &chunkState{maxRunes: maxRunes}
	walkSection(state, doc.Root, nil)
	state.flush()
	return state.chunks
}

type chunkState struct {
	maxRunes   int
	chunks     []EngineDocumentChunk
	runeOffset int
	breadcrumb string
	body       []string
	bodyRunes  int
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
	s.chunks = append(s.chunks, EngineDocumentChunk{
		Index:     len(s.chunks),
		Content:   content,
		StartRune: s.runeOffset,
		EndRune:   s.runeOffset + runeLen,
	})
	s.runeOffset += runeLen + 2
}

func (s *chunkState) add(piece string) {
	runes := len([]rune(piece))
	if s.bodyRunes > 0 && s.bodyRunes+runes > s.maxRunes {
		s.flush()
	}
	s.body = append(s.body, piece)
	s.bodyRunes += runes + 2
}

func walkSection(state *chunkState, section *Section, trail []string) {
	if section.Title != "" {
		trail = append(append([]string(nil), trail...), section.Title)
		state.breadcrumb = strings.Join(trail, " > ")
	}

	for _, block := range section.Blocks {
		if block.Type == BlockParagraph && block.Text != "" {
			state.add(block.Text)
		}
	}

	for _, child := range section.Children {
		walkSection(state, child, trail)
	}
}
