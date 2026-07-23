package deepdoc

import (
	"strings"
)

// ParseMarkdown builds a DocumentStructure from markdown text, honouring
// ATX headings, fenced code blocks, pipe tables, and list items.
func ParseMarkdown(content string) *DocumentStructure {
	doc := NewDocumentStructure(FormatMarkdown)
	builder := newSectionBuilder(doc.Root)

	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	var paragraph []string
	var list []string

	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		text := strings.TrimSpace(strings.Join(paragraph, " "))
		paragraph = nil
		if text != "" {
			builder.addBlock(Block{Type: BlockParagraph, Text: text})
		}
	}
	flushList := func() {
		if len(list) == 0 {
			return
		}
		items := list
		list = nil
		builder.addBlock(Block{Type: BlockList, Items: items})
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Fenced code block.
		if strings.HasPrefix(trimmed, "```") {
			flushParagraph()
			flushList()
			language := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			var code []string
			i++
			for ; i < len(lines); i++ {
				if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
					break
				}
				code = append(code, lines[i])
			}
			builder.addBlock(Block{Type: BlockCode, Text: strings.Join(code, "\n"), Language: language})
			continue
		}

		// ATX heading.
		if level, title, ok := parseATXHeading(trimmed); ok {
			flushParagraph()
			flushList()
			builder.startSection(title, level)
			if doc.Title == "" && level == 1 {
				doc.Title = title
			}
			continue
		}

		// Pipe table: current line and the next form a header + separator.
		if isTableRow(trimmed) && i+1 < len(lines) && isTableSeparator(strings.TrimSpace(lines[i+1])) {
			flushParagraph()
			flushList()
			table := &Table{HasHeader: true}
			table.Rows = append(table.Rows, splitTableRow(trimmed))
			i += 2
			for ; i < len(lines); i++ {
				rowLine := strings.TrimSpace(lines[i])
				if !isTableRow(rowLine) {
					i--
					break
				}
				table.Rows = append(table.Rows, splitTableRow(rowLine))
			}
			builder.addBlock(Block{Type: BlockTable, Table: table})
			continue
		}

		// List item.
		if item, ok := parseListItem(trimmed); ok {
			flushParagraph()
			list = append(list, item)
			continue
		}

		// Blank line terminates paragraph and list.
		if trimmed == "" {
			flushParagraph()
			flushList()
			continue
		}

		flushList()
		paragraph = append(paragraph, trimmed)
	}
	flushParagraph()
	flushList()
	return doc
}

func parseATXHeading(line string) (level int, title string, ok bool) {
	if !strings.HasPrefix(line, "#") {
		return 0, "", false
	}
	level = 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level > 6 || level == len(line) || line[level] != ' ' {
		return 0, "", false
	}
	title = strings.TrimSpace(strings.TrimRight(line[level+1:], "#"))
	if title == "" {
		return 0, "", false
	}
	return level, title, true
}

func parseListItem(line string) (string, bool) {
	for _, prefix := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(line[len(prefix):]), true
		}
	}
	// Ordered list: "1. item"
	dot := strings.Index(line, ". ")
	if dot > 0 && dot <= 3 {
		digits := line[:dot]
		allDigits := digits != ""
		for _, r := range digits {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return strings.TrimSpace(line[dot+2:]), true
		}
	}
	return "", false
}

func isTableRow(line string) bool {
	return strings.HasPrefix(line, "|") && strings.Count(line, "|") >= 2
}

func isTableSeparator(line string) bool {
	if !isTableRow(line) {
		return false
	}
	inner := strings.Trim(line, "|")
	for _, cell := range strings.Split(inner, "|") {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			return false
		}
		for _, r := range cell {
			if r != '-' && r != ':' {
				return false
			}
		}
	}
	return true
}

func splitTableRow(line string) []string {
	inner := strings.Trim(strings.TrimSpace(line), "|")
	parts := strings.Split(inner, "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}
