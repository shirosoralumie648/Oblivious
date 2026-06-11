package deepdoc

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
)

// Format names returned by DetectFormat and stamped on DocumentStructure.
const (
	FormatMarkdown = "markdown"
	FormatHTML     = "html"
	FormatDOCX     = "docx"
	FormatCSV      = "csv"
	FormatTSV      = "tsv"
	FormatText     = "text"
)

// DetectFormat identifies the document format from the filename extension
// and content sniffing. Content wins when the extension is missing or
// ambiguous.
func DetectFormat(filename string, content []byte) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md", ".markdown":
		return FormatMarkdown
	case ".html", ".htm":
		return FormatHTML
	case ".docx":
		return FormatDOCX
	case ".csv":
		return FormatCSV
	case ".tsv":
		return FormatTSV
	case ".txt", ".text", ".log":
		return sniffTextual(content, FormatText)
	}
	return sniffContent(content)
}

func sniffContent(content []byte) string {
	if len(content) >= 4 && bytes.HasPrefix(content, []byte{0x50, 0x4b, 0x03, 0x04}) {
		return FormatDOCX // zip magic; docx is the zip container we support
	}
	return sniffTextual(content, FormatText)
}

func sniffTextual(content []byte, fallback string) string {
	sample := string(content)
	if len(sample) > 8192 {
		sample = sample[:8192]
	}
	trimmed := strings.TrimSpace(sample)
	lower := strings.ToLower(trimmed)

	if strings.HasPrefix(lower, "<!doctype html") || strings.HasPrefix(lower, "<html") ||
		strings.Contains(lower, "<body") || strings.Contains(lower, "</p>") ||
		strings.Contains(lower, "<div") {
		return FormatHTML
	}

	if looksLikeMarkdown(trimmed) {
		return FormatMarkdown
	}

	if format, ok := looksLikeDelimited(trimmed); ok {
		return format
	}

	return fallback
}

func looksLikeMarkdown(sample string) bool {
	score := 0
	for _, line := range strings.Split(sample, "\n") {
		trimmed := strings.TrimSpace(line)
		if _, _, ok := parseATXHeading(trimmed); ok {
			score += 2
		}
		if strings.HasPrefix(trimmed, "```") {
			score += 2
		}
		if _, ok := parseListItem(trimmed); ok {
			score++
		}
		if isTableRow(trimmed) {
			score++
		}
	}
	return score >= 2
}

// looksLikeDelimited reports csv/tsv when several lines share a consistent
// delimiter count greater than zero.
func looksLikeDelimited(sample string) (string, bool) {
	lines := nonEmptyLines(sample, 6)
	if len(lines) < 2 {
		return "", false
	}
	for _, candidate := range []struct {
		delim  string
		format string
	}{{"\t", FormatTSV}, {",", FormatCSV}} {
		count := strings.Count(lines[0], candidate.delim)
		if count == 0 {
			continue
		}
		consistent := true
		for _, line := range lines[1:] {
			if strings.Count(line, candidate.delim) != count {
				consistent = false
				break
			}
		}
		if consistent {
			return candidate.format, true
		}
	}
	return "", false
}

func nonEmptyLines(sample string, max int) []string {
	var lines []string
	for _, line := range strings.Split(sample, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
		if len(lines) == max {
			break
		}
	}
	return lines
}

// Parse detects the format and dispatches to the matching parser.
func Parse(filename string, content []byte) (*DocumentStructure, error) {
	format := DetectFormat(filename, content)
	switch format {
	case FormatMarkdown:
		return ParseMarkdown(string(content)), nil
	case FormatHTML:
		return ParseHTML(string(content)), nil
	case FormatDOCX:
		return ParseDOCX(content)
	case FormatCSV:
		return ParseCSV(string(content), ','), nil
	case FormatTSV:
		doc := ParseCSV(string(content), '\t')
		doc.Format = FormatTSV
		return doc, nil
	case FormatText:
		return ParsePlainText(string(content)), nil
	default:
		return nil, fmt.Errorf("deepdoc: unsupported format %q", format)
	}
}
