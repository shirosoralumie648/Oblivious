package deepdoc

import (
	"html"
	"strings"
)

// ParseHTML builds a DocumentStructure from HTML using a tolerant
// stdlib-only tokenizer (golang.org/x/net/html is only an indirect module
// dependency, so we avoid promoting it). script/style/head contents are
// dropped; headings, paragraphs, tables, lists, and <pre> blocks are kept.
func ParseHTML(content string) *DocumentStructure {
	doc := NewDocumentStructure(FormatHTML)
	builder := newSectionBuilder(doc.Root)

	type tableState struct {
		table *Table
		row   []string
		cell  strings.Builder
		inRow bool
	}

	var (
		paragraph  strings.Builder
		listItems  []string
		inItem     bool
		item       strings.Builder
		pre        strings.Builder
		inPre      bool
		heading    strings.Builder
		headingLvl int
		inHeading  bool
		skipDepth  int // inside script/style/head
		inTitle    bool
		title      strings.Builder
		tables     []*tableState
	)

	flushParagraph := func() {
		text := normalizeSpace(paragraph.String())
		paragraph.Reset()
		if text != "" {
			builder.addBlock(Block{Type: BlockParagraph, Text: text})
		}
	}
	flushList := func() {
		if inItem {
			if text := normalizeSpace(item.String()); text != "" {
				listItems = append(listItems, text)
			}
			item.Reset()
			inItem = false
		}
		if len(listItems) > 0 {
			builder.addBlock(Block{Type: BlockList, Items: listItems})
			listItems = nil
		}
	}

	appendText := func(text string) {
		unescaped := html.UnescapeString(text)
		// <title> lives inside <head>, which is otherwise skipped.
		if inTitle {
			title.WriteString(unescaped)
			return
		}
		if skipDepth > 0 {
			return
		}
		switch {
		case inHeading:
			heading.WriteString(unescaped)
		case inPre:
			pre.WriteString(unescaped)
		case len(tables) > 0 && tables[len(tables)-1].inRow:
			tables[len(tables)-1].cell.WriteString(unescaped)
		case inItem:
			item.WriteString(unescaped)
		default:
			paragraph.WriteString(unescaped)
		}
	}

	i := 0
	for i < len(content) {
		lt := strings.IndexByte(content[i:], '<')
		if lt < 0 {
			appendText(content[i:])
			break
		}
		if lt > 0 {
			appendText(content[i : i+lt])
		}
		i += lt
		gt := findTagEnd(content, i)
		if gt < 0 {
			// Malformed trailing tag: treat the rest as text.
			appendText(content[i:])
			break
		}
		rawTag := content[i+1 : gt]
		i = gt + 1

		if strings.HasPrefix(rawTag, "!--") {
			// Comment: skip to -->
			if end := strings.Index(content[i:], "-->"); end >= 0 {
				i += end + 3
			} else {
				i = len(content)
			}
			continue
		}

		name, closing := parseTagName(rawTag)
		if name == "" {
			continue
		}

		switch name {
		case "script", "style", "head":
			if closing {
				if skipDepth > 0 {
					skipDepth--
				}
			} else if !strings.HasSuffix(rawTag, "/") {
				skipDepth++
			}
		case "title":
			inTitle = !closing
			if closing && doc.Title == "" {
				doc.Title = normalizeSpace(title.String())
			}
		case "h1", "h2", "h3", "h4", "h5", "h6":
			if closing {
				if inHeading {
					text := normalizeSpace(heading.String())
					heading.Reset()
					inHeading = false
					if text != "" {
						builder.startSection(text, headingLvl)
						if doc.Title == "" && headingLvl == 1 {
							doc.Title = text
						}
					}
				}
			} else {
				flushParagraph()
				flushList()
				inHeading = true
				headingLvl = int(name[1] - '0')
			}
		case "table":
			if closing {
				if len(tables) > 0 {
					st := tables[len(tables)-1]
					tables = tables[:len(tables)-1]
					if len(st.table.Rows) > 0 {
						builder.addBlock(Block{Type: BlockTable, Table: st.table})
					}
				}
			} else {
				flushParagraph()
				flushList()
				tables = append(tables, &tableState{table: &Table{}})
			}
		case "tr":
			if len(tables) > 0 {
				st := tables[len(tables)-1]
				if closing {
					if st.inRow {
						if cell := normalizeSpace(st.cell.String()); cell != "" || len(st.row) > 0 {
							// final cell flushed by td/th close normally; guard anyway
							if cell != "" {
								st.row = append(st.row, cell)
								st.cell.Reset()
							}
						}
						if len(st.row) > 0 {
							st.table.Rows = append(st.table.Rows, st.row)
						}
						st.row = nil
						st.inRow = false
					}
				} else {
					st.inRow = true
					st.row = nil
				}
			}
		case "td", "th":
			if len(tables) > 0 {
				st := tables[len(tables)-1]
				if closing {
					st.row = append(st.row, normalizeSpace(st.cell.String()))
					st.cell.Reset()
				} else if !closing && name == "th" {
					st.table.HasHeader = true
				}
			}
		case "ul", "ol":
			if closing {
				flushList()
			} else {
				flushParagraph()
			}
		case "li":
			if closing {
				if inItem {
					if text := normalizeSpace(item.String()); text != "" {
						listItems = append(listItems, text)
					}
					item.Reset()
					inItem = false
				}
			} else {
				if inItem {
					if text := normalizeSpace(item.String()); text != "" {
						listItems = append(listItems, text)
					}
					item.Reset()
				}
				inItem = true
			}
		case "pre":
			if closing {
				if inPre {
					builder.addBlock(Block{Type: BlockCode, Text: strings.Trim(pre.String(), "\n")})
					pre.Reset()
					inPre = false
				}
			} else {
				flushParagraph()
				flushList()
				inPre = true
			}
		case "p", "div", "section", "article":
			if closing || strings.HasSuffix(rawTag, "/") {
				flushParagraph()
			} else {
				flushParagraph()
			}
		case "br":
			appendText(" ")
		}
	}
	flushParagraph()
	flushList()
	// Close any unterminated table (malformed HTML).
	for len(tables) > 0 {
		st := tables[len(tables)-1]
		tables = tables[:len(tables)-1]
		if len(st.table.Rows) > 0 {
			builder.addBlock(Block{Type: BlockTable, Table: st.table})
		}
	}
	return doc
}

// findTagEnd locates the '>' closing the tag starting at content[start]=='<',
// honouring quoted attribute values.
func findTagEnd(content string, start int) int {
	quote := byte(0)
	for j := start + 1; j < len(content); j++ {
		c := content[j]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '>':
			return j
		}
	}
	return -1
}

func parseTagName(rawTag string) (name string, closing bool) {
	tag := strings.TrimSpace(rawTag)
	if tag == "" {
		return "", false
	}
	if tag[0] == '/' {
		closing = true
		tag = tag[1:]
	}
	end := 0
	for end < len(tag) {
		c := tag[end]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '/' {
			break
		}
		end++
	}
	return strings.ToLower(tag[:end]), closing
}

func normalizeSpace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
