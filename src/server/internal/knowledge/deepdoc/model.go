// Package deepdoc provides layout-aware document parsing (inspired by
// ragflow's deepdoc): instead of flattening uploads to plain text, it
// extracts heading hierarchy, tables, lists, and code blocks so chunking
// can respect document structure.
package deepdoc

// BlockType identifies the kind of content a Block carries.
type BlockType string

const (
	BlockParagraph BlockType = "paragraph"
	BlockTable     BlockType = "table"
	BlockList      BlockType = "list"
	BlockCode      BlockType = "code"
)

// Table is a structured table with optional caption. Rows include the
// header row (first) when one was detected.
type Table struct {
	Caption   string     `json:"caption,omitempty"`
	Rows      [][]string `json:"rows"`
	HasHeader bool       `json:"hasHeader,omitempty"`
}

// Block is a single content unit inside a Section.
type Block struct {
	Type     BlockType `json:"type"`
	Text     string    `json:"text,omitempty"`     // paragraph or code content
	Language string    `json:"language,omitempty"` // code fence language
	Items    []string  `json:"items,omitempty"`    // list items
	Table    *Table    `json:"table,omitempty"`
}

// Section is a node in the heading hierarchy. Level follows heading depth
// (1 == H1). The synthetic root section has Level 0 and an empty Title.
type Section struct {
	Title    string     `json:"title,omitempty"`
	Level    int        `json:"level"`
	Blocks   []Block    `json:"blocks,omitempty"`
	Children []*Section `json:"children,omitempty"`
}

// DocumentStructure is the common structural model all parsers produce.
type DocumentStructure struct {
	Title    string            `json:"title,omitempty"`
	Format   string            `json:"format"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Root     *Section          `json:"root"`
}

// NewDocumentStructure returns an empty structure with a level-0 root.
func NewDocumentStructure(format string) *DocumentStructure {
	return &DocumentStructure{
		Format: format,
		Root:   &Section{Level: 0},
	}
}

// IsEmpty reports whether the document contains no content blocks.
func (d *DocumentStructure) IsEmpty() bool {
	if d == nil || d.Root == nil {
		return true
	}
	var walk func(s *Section) bool
	walk = func(s *Section) bool {
		if len(s.Blocks) > 0 {
			return false
		}
		for _, c := range s.Children {
			if !walk(c) {
				return false
			}
		}
		return true
	}
	return walk(d.Root)
}

// sectionBuilder maintains a stack of sections so parsers can append
// headings/blocks without tracking tree positions themselves.
type sectionBuilder struct {
	root  *Section
	stack []*Section
}

func newSectionBuilder(root *Section) *sectionBuilder {
	return &sectionBuilder{root: root, stack: []*Section{root}}
}

func (b *sectionBuilder) current() *Section {
	return b.stack[len(b.stack)-1]
}

// startSection opens a heading at the given level, popping deeper or
// equal levels first so the hierarchy mirrors heading nesting.
func (b *sectionBuilder) startSection(title string, level int) {
	if level < 1 {
		level = 1
	}
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
