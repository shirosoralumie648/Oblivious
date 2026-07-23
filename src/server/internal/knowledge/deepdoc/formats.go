package deepdoc

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrInvalidDOCX is returned when a .docx archive cannot be read.
var ErrInvalidDOCX = errors.New("deepdoc: invalid docx archive")

const maxDocxXMLBytes = 64 * 1024 * 1024

// ParseDOCX builds a DocumentStructure from .docx bytes (zip + WordprocessingML).
// Heading levels come from paragraph styles (Heading1..Heading6 / 标题 styles
// mapping by trailing digit); w:tbl becomes tables; numbering properties mark
// list items.
func ParseDOCX(data []byte) (*DocumentStructure, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidDOCX, err)
	}
	var docXML []byte
	for _, file := range reader.File {
		if file.Name == "word/document.xml" {
			rc, openErr := file.Open()
			if openErr != nil {
				return nil, fmt.Errorf("%w: %v", ErrInvalidDOCX, openErr)
			}
			docXML, err = io.ReadAll(io.LimitReader(rc, maxDocxXMLBytes))
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("%w: %v", ErrInvalidDOCX, err)
			}
			break
		}
	}
	if docXML == nil {
		return nil, fmt.Errorf("%w: word/document.xml missing", ErrInvalidDOCX)
	}

	doc := NewDocumentStructure(FormatDOCX)
	builder := newSectionBuilder(doc.Root)

	decoder := xml.NewDecoder(bytes.NewReader(docXML))
	var listItems []string
	flushList := func() {
		if len(listItems) > 0 {
			builder.addBlock(Block{Type: BlockList, Items: listItems})
			listItems = nil
		}
	}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidDOCX, err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "tbl":
			flushList()
			table, err := parseDocxTable(decoder, start)
			if err != nil {
				return nil, err
			}
			if len(table.Rows) > 0 {
				builder.addBlock(Block{Type: BlockTable, Table: table})
			}
		case "p":
			text, style, isList, err := parseDocxParagraph(decoder, start)
			if err != nil {
				return nil, err
			}
			text = normalizeSpace(text)
			if text == "" {
				continue
			}
			if level := headingLevelFromStyle(style); level > 0 {
				flushList()
				builder.startSection(text, level)
				if doc.Title == "" && level == 1 {
					doc.Title = text
				}
			} else if isList {
				listItems = append(listItems, text)
			} else {
				flushList()
				builder.addBlock(Block{Type: BlockParagraph, Text: text})
			}
		}
	}
	flushList()
	return doc, nil
}

// parseDocxParagraph consumes a <w:p> element returning its text, style id,
// and whether it carries numbering (list) properties.
func parseDocxParagraph(decoder *xml.Decoder, start xml.StartElement) (text, style string, isList bool, err error) {
	var sb strings.Builder
	depth := 1
	for depth > 0 {
		token, tokenErr := decoder.Token()
		if tokenErr != nil {
			return "", "", false, fmt.Errorf("%w: %v", ErrInvalidDOCX, tokenErr)
		}
		switch el := token.(type) {
		case xml.StartElement:
			depth++
			switch el.Name.Local {
			case "pStyle":
				for _, attr := range el.Attr {
					if attr.Name.Local == "val" {
						style = attr.Value
					}
				}
			case "numPr":
				isList = true
			case "tab":
				sb.WriteString(" ")
			case "br":
				sb.WriteString(" ")
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			sb.Write(el)
		}
	}
	return sb.String(), style, isList, nil
}

// parseDocxTable consumes a <w:tbl> element building a Table from its rows.
func parseDocxTable(decoder *xml.Decoder, start xml.StartElement) (*Table, error) {
	table := &Table{}
	var row []string
	var cell strings.Builder
	inCell := false
	depth := 1
	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidDOCX, err)
		}
		switch el := token.(type) {
		case xml.StartElement:
			depth++
			switch el.Name.Local {
			case "tr":
				row = nil
			case "tc":
				inCell = true
				cell.Reset()
			}
		case xml.EndElement:
			depth--
			switch el.Name.Local {
			case "tc":
				row = append(row, normalizeSpace(cell.String()))
				inCell = false
			case "tr":
				if len(row) > 0 {
					table.Rows = append(table.Rows, row)
				}
			}
		case xml.CharData:
			if inCell {
				cell.Write(el)
			}
		}
	}
	if len(table.Rows) > 1 {
		table.HasHeader = true
	}
	return table, nil
}

func headingLevelFromStyle(style string) int {
	if style == "" {
		return 0
	}
	lower := strings.ToLower(style)
	if !strings.HasPrefix(lower, "heading") && !strings.HasPrefix(style, "标题") && !strings.HasPrefix(lower, "berschrift") && !strings.HasPrefix(lower, "titre") {
		return 0
	}
	// Trailing digit names the level: Heading1, 标题2, ...
	last := style[len(style)-1]
	if last >= '1' && last <= '6' {
		return int(last - '0')
	}
	if strings.HasPrefix(lower, "heading") || strings.HasPrefix(style, "标题") {
		return 1
	}
	return 0
}

// ParseCSV builds a single-table DocumentStructure from CSV or TSV content.
func ParseCSV(content string, delimiter rune) *DocumentStructure {
	doc := NewDocumentStructure(FormatCSV)
	reader := csv.NewReader(strings.NewReader(content))
	reader.Comma = delimiter
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	table := &Table{}
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}
		trimmed := make([]string, len(record))
		empty := true
		for i, field := range record {
			trimmed[i] = strings.TrimSpace(field)
			if trimmed[i] != "" {
				empty = false
			}
		}
		if !empty {
			table.Rows = append(table.Rows, trimmed)
		}
	}
	if len(table.Rows) > 0 {
		table.HasHeader = detectCSVHeader(table.Rows)
		doc.Root.Blocks = append(doc.Root.Blocks, Block{Type: BlockTable, Table: table})
	}
	return doc
}

// detectCSVHeader guesses a header row: first row has no numeric-looking
// cells while a later row does.
func detectCSVHeader(rows [][]string) bool {
	if len(rows) < 2 {
		return false
	}
	for _, cell := range rows[0] {
		if cell == "" || looksNumeric(cell) {
			return false
		}
	}
	for _, cell := range rows[1] {
		if looksNumeric(cell) {
			return true
		}
	}
	return false
}

func looksNumeric(text string) bool {
	if text == "" {
		return false
	}
	dots := 0
	for i, r := range text {
		switch {
		case r >= '0' && r <= '9':
		case r == '.':
			dots++
			if dots > 1 {
				return false
			}
		case (r == '-' || r == '+') && i == 0:
		case r == ',':
		default:
			return false
		}
	}
	return true
}

// ParsePlainText builds a DocumentStructure from plain text using blank-line
// paragraph splitting plus an indentation/short-line section heuristic: a
// short unindented line followed by indented or longer content is treated as
// a section heading.
func ParsePlainText(content string) *DocumentStructure {
	doc := NewDocumentStructure(FormatText)
	builder := newSectionBuilder(doc.Root)

	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	blocks := strings.Split(normalized, "\n\n")
	for _, raw := range blocks {
		lines := strings.Split(raw, "\n")
		var paragraph []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if isPlainHeadingCandidate(line, trimmed) {
				if len(paragraph) > 0 {
					builder.addBlock(Block{Type: BlockParagraph, Text: strings.Join(paragraph, " ")})
					paragraph = nil
				}
				builder.startSection(trimmed, 1)
				continue
			}
			paragraph = append(paragraph, trimmed)
		}
		if len(paragraph) > 0 {
			builder.addBlock(Block{Type: BlockParagraph, Text: strings.Join(paragraph, " ")})
		}
	}
	return doc
}

// isPlainHeadingCandidate marks short, unindented lines that end without
// sentence punctuation as headings (e.g. "INTRODUCTION", "1. Overview").
func isPlainHeadingCandidate(rawLine, trimmed string) bool {
	if len(trimmed) > 80 || strings.HasPrefix(rawLine, " ") || strings.HasPrefix(rawLine, "\t") {
		return false
	}
	if strings.HasSuffix(trimmed, ".") || strings.HasSuffix(trimmed, ",") || strings.HasSuffix(trimmed, ";") || strings.HasSuffix(trimmed, "。") || strings.HasSuffix(trimmed, "，") {
		return false
	}
	words := strings.Fields(trimmed)
	if len(words) == 0 || len(words) > 8 {
		return false
	}
	upper := strings.ToUpper(trimmed)
	if upper == trimmed && len(words) <= 8 {
		return true
	}
	// Numbered headings: "1. Overview", "2.3 Detail"
	first := words[0]
	digits := strings.Trim(first, ".0123456789")
	return digits == "" && strings.ContainsAny(first, "0123456789") && len(words) >= 2
}
