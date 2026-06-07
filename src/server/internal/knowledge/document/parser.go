package document

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"context"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"oblivious/server/internal/knowledge"
)

// ErrUnsupportedDocumentFormat is returned when the uploaded file has an
// unrecognised extension or content type.
var ErrUnsupportedDocumentFormat = errors.New("unsupported document format")

// ErrEmptyDocument is returned when parsing yields no textual content.
var ErrEmptyDocument = errors.New("empty document")

// ErrDocumentTooLarge signals that the uploaded file exceeds the byte limit.
var ErrDocumentTooLarge = errors.New("document too large")

const defaultMaxBytes = 10 * 1024 * 1024

// Parser provides a unified interface for extracting text from various
// document formats. It is safe for concurrent use.
type Parser struct{}

// NewParser returns a new Parser.
func NewParser() *Parser { return &Parser{} }

// Parse reads the uploaded document and returns structured content including
// per-page data when the format supports it.
func (p *Parser) Parse(ctx context.Context, reader io.Reader, filename, contentType string, maxBytes int64) (knowledge.ParsedDocumentWithPages, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return knowledge.ParsedDocumentWithPages{}, err
	}
	if reader == nil {
		return knowledge.ParsedDocumentWithPages{}, ErrEmptyDocument
	}

	format, normalizedContentType, err := detectFormat(filename, contentType)
	if err != nil {
		return knowledge.ParsedDocumentWithPages{}, err
	}

	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	data, err := readLimited(reader, maxBytes)
	if err != nil {
		return knowledge.ParsedDocumentWithPages{}, err
	}

	parsed := knowledge.ParsedDocumentWithPages{
		Format:      format,
		Title:       sanitizeTitle(filename),
		ContentType: normalizedContentType,
		SizeBytes:   int64(len(data)),
	}

	switch format {
	case "docx":
		parsed.Content, err = parseDOCX(data)
	case "pdf":
		parsed.Content, parsed.Pages = parsePDF(data)
	case knowledge.KnowledgeDocumentFormatHTML:
		parsed.Content = parseHTML(data)
	case knowledge.KnowledgeDocumentFormatCSV:
		parsed.Content = parseCSV(data)
	case knowledge.KnowledgeDocumentFormatXLSX:
		parsed.Content, err = parseXLSX(data)
	case knowledge.KnowledgeDocumentFormatPPTX:
		parsed.Content, err = parsePPTX(data)
	default:
		parsed.Content = normalizeText(data)
	}
	if err != nil {
		return knowledge.ParsedDocumentWithPages{}, err
	}
	if parsed.Content == "" {
		return knowledge.ParsedDocumentWithPages{}, ErrEmptyDocument
	}
	return parsed, nil
}

// detectFormat identifies the document format from filename and content type.
func detectFormat(filename, contentType string) (string, string, error) {
	mediaType := strings.TrimSpace(strings.ToLower(contentType))
	if mediaType != "" {
		if parsed, _, err := mime.ParseMediaType(mediaType); err == nil {
			mediaType = parsed
		}
	}

	switch mediaType {
	case "text/plain":
		return "text", mediaType, nil
	case "text/markdown", "text/x-markdown":
		return "markdown", mediaType, nil
	case "application/pdf":
		return "pdf", mediaType, nil
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return "docx", mediaType, nil
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return knowledge.KnowledgeDocumentFormatXLSX, mediaType, nil
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return knowledge.KnowledgeDocumentFormatPPTX, mediaType, nil
	case "text/html", "application/xhtml+xml":
		return knowledge.KnowledgeDocumentFormatHTML, mediaType, nil
	case "text/csv", "application/csv":
		return knowledge.KnowledgeDocumentFormatCSV, mediaType, nil
	case "application/msword":
		return "", mediaType, fmt.Errorf("%w: %s", ErrUnsupportedDocumentFormat, mediaType)
	}

	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	switch ext {
	case ".txt", ".text":
		return "text", mediaType, nil
	case ".md", ".markdown":
		return "markdown", mediaType, nil
	case ".pdf":
		return "pdf", mediaType, nil
	case ".docx":
		return "docx", mediaType, nil
	case ".xlsx", ".xls":
		return knowledge.KnowledgeDocumentFormatXLSX, mediaType, nil
	case ".pptx":
		return knowledge.KnowledgeDocumentFormatPPTX, mediaType, nil
	case ".html", ".htm":
		return knowledge.KnowledgeDocumentFormatHTML, mediaType, nil
	case ".csv":
		return knowledge.KnowledgeDocumentFormatCSV, mediaType, nil
	case ".doc":
		return "", mediaType, fmt.Errorf("%w: %s", ErrUnsupportedDocumentFormat, ext)
	}

	if mediaType == "" && ext == "" {
		return "text", mediaType, nil
	}
	return "", mediaType, fmt.Errorf("%w: %s", ErrUnsupportedDocumentFormat, strings.TrimSpace(filename))
}

// readLimited reads up to maxBytes+1 from reader so it can detect oversize.
func readLimited(reader io.Reader, maxBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrDocumentTooLarge
	}
	return data, nil
}

// sanitizeTitle extracts a displayable title from a filename.
func sanitizeTitle(filename string) string {
	title := strings.TrimSpace(filepath.Base(filename))
	if title == "." || title == "/" {
		return ""
	}
	return title
}

// normalizeText cleans raw bytes into a trimmed, newline-normalised string.
func normalizeText(data []byte) string {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.TrimSpace(text)
}

// ---------------------------------------------------------------------------
// DOCX parser
// ---------------------------------------------------------------------------

func parseDOCX(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("%w: docx archive", ErrUnsupportedDocumentFormat)
	}
	var documentXML io.ReadCloser
	for _, file := range reader.File {
		if file.Name == "word/document.xml" {
			documentXML, err = file.Open()
			if err != nil {
				return "", err
			}
			defer documentXML.Close()
			break
		}
	}
	if documentXML == nil {
		return "", fmt.Errorf("%w: missing word/document.xml", ErrUnsupportedDocumentFormat)
	}

	decoder := xml.NewDecoder(documentXML)
	var buf strings.Builder
	var paragraphs []string
	inText := false

	flush := func() {
		t := strings.TrimSpace(buf.String())
		if t != "" {
			paragraphs = append(paragraphs, t)
		}
		buf.Reset()
	}

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		switch v := token.(type) {
		case xml.StartElement:
			if v.Name.Local == "t" {
				inText = true
			}
		case xml.EndElement:
			switch v.Name.Local {
			case "t":
				inText = false
			case "p":
				flush()
			}
		case xml.CharData:
			if inText {
				buf.Write([]byte(v))
			}
		}
	}
	flush()
	return normalizeText([]byte(strings.Join(paragraphs, "\n"))), nil
}

// ---------------------------------------------------------------------------
// PDF parser with page tracking
// ---------------------------------------------------------------------------

func parsePDF(data []byte) (string, []knowledge.DocumentPage) {
	var blocks []string
	var pages []knowledge.DocumentPage
	offset := 0
	pageNum := 1

	for offset < len(data) {
		streamKW := bytes.Index(data[offset:], []byte("stream"))
		if streamKW < 0 {
			break
		}
		streamKW += offset
		streamStart := streamKW + len("stream")
		if streamStart < len(data) && data[streamStart] == '\r' {
			streamStart++
			if streamStart < len(data) && data[streamStart] == '\n' {
				streamStart++
			}
		} else if streamStart < len(data) && data[streamStart] == '\n' {
			streamStart++
		}
		streamEnd := bytes.Index(data[streamStart:], []byte("endstream"))
		if streamEnd < 0 {
			break
		}
		streamEnd += streamStart
		if stream, ok := decodePDFStream(data, streamKW, data[streamStart:streamEnd]); ok {
			if text := extractPDFStreamText(stream); text != "" {
				blocks = append(blocks, text)
				pages = append(pages, knowledge.DocumentPage{
					PageNumber: pageNum,
					Content:    text,
				})
				pageNum++
			}
		}
		offset = streamEnd + len("endstream")
	}
	joined := normalizePDFText(strings.Join(blocks, "\n"))
	if len(pages) == 0 {
		return joined, nil
	}
	return joined, pages
}

// ---------------------------------------------------------------------------
// HTML parser
// ---------------------------------------------------------------------------

func parseHTML(data []byte) string {
	text := normalizeText(data)
	for _, tag := range []string{"script", "style"} {
		for {
			start := strings.Index(strings.ToLower(text), "<"+tag)
			if start < 0 {
				break
			}
			end := strings.Index(strings.ToLower(text[start:]), "</"+tag+">")
			if end < 0 {
				break
			}
			end += start + len("</"+tag+">")
			text = text[:start] + text[end:]
		}
	}
	replacer := strings.NewReplacer(
		"<br>", "\n", "<br/>", "\n", "<br />", "\n",
		"</p>", "\n", "</div>", "\n", "</li>", "\n",
		"</tr>", "\n", "</h1>", "\n", "</h2>", "\n",
		"</h3>", "\n", "</h4>", "\n", "</h5>", "\n", "</h6>", "\n",
	)
	text = replacer.Replace(text)
	var out strings.Builder
	inTag := false
	for _, r := range text {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			out.WriteRune(r)
		}
	}
	result := strings.TrimSpace(out.String())
	lines := strings.Split(result, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}

// ---------------------------------------------------------------------------
// CSV parser
// ---------------------------------------------------------------------------

func parseCSV(data []byte) string {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	var lines []string
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return normalizeText(data)
		}
		lines = append(lines, strings.Join(record, " | "))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// ---------------------------------------------------------------------------
// XLSX parser
// ---------------------------------------------------------------------------

func parseXLSX(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("%w: xlsx archive", ErrUnsupportedDocumentFormat)
	}

	sharedStrings, _ := readXLSXSharedStrings(reader)

	sheetContent, err := readZipEntry(reader, "xl/worksheets/sheet1.xml")
	if err != nil {
		if len(sharedStrings) > 0 {
			return normalizeText([]byte(strings.Join(sharedStrings, "\n"))), nil
		}
		return "", fmt.Errorf("%w: missing sheet1.xml", ErrUnsupportedDocumentFormat)
	}

	var rows []string
	decoder := xml.NewDecoder(bytes.NewReader(sheetContent))
	inValue := false
	var cellValues []string
	var currentRow []string

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		switch v := token.(type) {
		case xml.StartElement:
			if v.Name.Local == "v" || v.Name.Local == "t" {
				inValue = true
			}
		case xml.EndElement:
			switch v.Name.Local {
			case "v", "t":
				inValue = false
			case "c":
				val := strings.Join(cellValues, "")
				currentRow = append(currentRow, val)
				cellValues = nil
			case "row":
				if len(currentRow) > 0 {
					rows = append(rows, strings.Join(currentRow, " | "))
				}
				currentRow = nil
			}
		case xml.CharData:
			if inValue {
				cellValues = append(cellValues, string(v))
			}
		}
	}

	if len(rows) == 0 && len(sharedStrings) > 0 {
		return normalizeText([]byte(strings.Join(sharedStrings, "\n"))), nil
	}
	return normalizeText([]byte(strings.Join(rows, "\n"))), nil
}

func readXLSXSharedStrings(reader *zip.Reader) ([]string, error) {
	content, err := readZipEntry(reader, "xl/sharedStrings.xml")
	if err != nil {
		return nil, nil
	}
	var result []string
	decoder := xml.NewDecoder(bytes.NewReader(content))
	inT := false
	var buf strings.Builder
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch v := token.(type) {
		case xml.StartElement:
			if v.Name.Local == "t" {
				inT = true
			}
		case xml.EndElement:
			if v.Name.Local == "t" {
				inT = false
				result = append(result, buf.String())
				buf.Reset()
			}
		case xml.CharData:
			if inT {
				buf.Write([]byte(v))
			}
		}
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// PPTX parser
// ---------------------------------------------------------------------------

func parsePPTX(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("%w: pptx archive", ErrUnsupportedDocumentFormat)
	}

	var allText []string
	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, "ppt/slides/slide") || !strings.HasSuffix(file.Name, ".xml") {
			continue
		}
		f, err := file.Open()
		if err != nil {
			continue
		}
		content, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			continue
		}
		text := extractPPTXSlideText(content)
		if text != "" {
			allText = append(allText, text)
		}
	}
	if len(allText) == 0 {
		return "", ErrEmptyDocument
	}
	return normalizeText([]byte(strings.Join(allText, "\n\n"))), nil
}

func extractPPTXSlideText(data []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	inT := false
	var paragraphs []string
	var buf strings.Builder

	flush := func() {
		t := strings.TrimSpace(buf.String())
		if t != "" {
			paragraphs = append(paragraphs, t)
		}
		buf.Reset()
	}

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			break
		}
		switch v := token.(type) {
		case xml.StartElement:
			if v.Name.Local == "t" {
				inT = true
			}
		case xml.EndElement:
			switch v.Name.Local {
			case "t":
				inT = false
			case "p":
				flush()
			}
		case xml.CharData:
			if inT {
				buf.Write([]byte(v))
			}
		}
	}
	flush()
	return strings.Join(paragraphs, "\n")
}

// ---------------------------------------------------------------------------
// Shared zip helpers
// ---------------------------------------------------------------------------

func readZipEntry(reader *zip.Reader, name string) ([]byte, error) {
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		f, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return io.ReadAll(f)
	}
	return nil, fmt.Errorf("entry %s not found", name)
}

// ---------------------------------------------------------------------------
// PDF helpers
// ---------------------------------------------------------------------------

func decodePDFStream(data []byte, streamKeyword int, stream []byte) ([]byte, bool) {
	header := pdfStreamHeader(data, streamKeyword)
	if !bytes.Contains(header, []byte("/Filter")) {
		return stream, true
	}
	if !pdfHasOnlyFlateDecode(header) {
		return nil, false
	}
	r, err := zlib.NewReader(bytes.NewReader(stream))
	if err != nil {
		return nil, false
	}
	defer r.Close()
	decoded, err := io.ReadAll(io.LimitReader(r, defaultMaxBytes+1))
	if err != nil || len(decoded) > defaultMaxBytes {
		return nil, false
	}
	return decoded, true
}

func pdfStreamHeader(data []byte, streamKeyword int) []byte {
	start := streamKeyword - 512
	if start < 0 {
		start = 0
	}
	return data[start:streamKeyword]
}

func pdfHasOnlyFlateDecode(header []byte) bool {
	idx := bytes.Index(header, []byte("/Filter"))
	if idx < 0 {
		return false
	}
	_, off := pdfReadName(header, idx)
	off = pdfSkipWS(header, off)
	if off >= len(header) {
		return false
	}
	if header[off] == '/' {
		name, _ := pdfReadName(header, off)
		return name == "FlateDecode" || name == "Fl"
	}
	if header[off] != '[' {
		return false
	}
	off++
	count := 0
	hasFlate := false
	for off < len(header) {
		off = pdfSkipWS(header, off)
		if off >= len(header) {
			return false
		}
		if header[off] == ']' {
			return count == 1 && hasFlate
		}
		if header[off] != '/' {
			return false
		}
		name, next := pdfReadName(header, off)
		count++
		if name == "FlateDecode" || name == "Fl" {
			hasFlate = true
		}
		off = next
	}
	return false
}

func pdfReadName(data []byte, start int) (string, int) {
	if start >= len(data) || data[start] != '/' {
		return "", start
	}
	i := start + 1
	for i < len(data) && !pdfIsWS(data[i]) && !pdfIsDelim(data[i]) {
		i++
	}
	return string(data[start+1 : i]), i
}

func extractPDFStreamText(stream []byte) string {
	var lines []string
	var current strings.Builder
	var textOps []string

	add := func(s string) { current.WriteString(s) }
	newLine := func() {
		line := strings.TrimSpace(current.String())
		if line != "" {
			lines = append(lines, line)
		}
		current.Reset()
	}

	for i := 0; i < len(stream); {
		i = pdfSkipWS(stream, i)
		if i >= len(stream) {
			break
		}
		switch stream[i] {
		case '(':
			text, next, ok := pdfParseLiteralString(stream, i)
			if ok {
				textOps = append(textOps, text)
				i = next
				continue
			}
		case '[':
			text, next, ok := pdfParseTextArray(stream, i)
			if ok {
				textOps = append(textOps, text)
				i = next
				continue
			}
		case '<':
			text, next, ok := pdfParseHexString(stream, i)
			if ok {
				textOps = append(textOps, text)
				i = next
				continue
			}
		}
		token, next := pdfReadToken(stream, i)
		switch token {
		case "Tj", "TJ":
			add(lastTextOp(textOps))
			textOps = nil
		case "'", "\"":
			newLine()
			add(lastTextOp(textOps))
			textOps = nil
		case "Td", "TD", "T*":
			newLine()
			textOps = nil
		case "BT", "ET":
			textOps = nil
		}
		i = next
	}
	newLine()
	return strings.Join(lines, "\n")
}

func lastTextOp(ops []string) string {
	for i := len(ops) - 1; i >= 0; i-- {
		if ops[i] != "" {
			return ops[i]
		}
	}
	return ""
}

func pdfParseTextArray(data []byte, start int) (string, int, bool) {
	if start >= len(data) || data[start] != '[' {
		return "", start, false
	}
	var text strings.Builder
	depth := 1
	for i := start + 1; i < len(data); {
		i = pdfSkipWS(data, i)
		if i >= len(data) {
			break
		}
		switch data[i] {
		case '(':
			v, next, ok := pdfParseLiteralString(data, i)
			if !ok {
				return "", start, false
			}
			text.WriteString(v)
			i = next
		case '<':
			v, next, ok := pdfParseHexString(data, i)
			if ok {
				text.WriteString(v)
				i = next
			} else {
				_, i = pdfReadToken(data, i)
			}
		case '[':
			depth++
			i++
		case ']':
			depth--
			i++
			if depth == 0 {
				return text.String(), i, true
			}
		default:
			_, i = pdfReadToken(data, i)
		}
	}
	return "", start, false
}

func pdfParseLiteralString(data []byte, start int) (string, int, bool) {
	if start >= len(data) || data[start] != '(' {
		return "", start, false
	}
	var out []byte
	depth := 1
	for i := start + 1; i < len(data); {
		c := data[i]
		if c == '\\' {
			if i+1 >= len(data) {
				return "", start, false
			}
			next := data[i+1]
			switch next {
			case 'n':
				out = append(out, '\n')
				i += 2
			case 'r':
				out = append(out, '\r')
				i += 2
			case 't':
				out = append(out, '\t')
				i += 2
			case 'b':
				out = append(out, '\b')
				i += 2
			case 'f':
				out = append(out, '\f')
				i += 2
			case '(', ')', '\\':
				out = append(out, next)
				i += 2
			case '\r':
				i += 2
				if i < len(data) && data[i] == '\n' {
					i++
				}
			case '\n':
				i += 2
			default:
				if pdfIsOctal(next) {
					val := 0
					digits := 0
					i++
					for i < len(data) && digits < 3 && pdfIsOctal(data[i]) {
						val = val*8 + int(data[i]-'0')
						i++
						digits++
					}
					out = append(out, byte(val))
				} else {
					out = append(out, next)
					i += 2
				}
			}
			continue
		}
		switch c {
		case '(':
			depth++
			out = append(out, c)
		case ')':
			depth--
			if depth == 0 {
				return pdfDecodeString(out), i + 1, true
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
		i++
	}
	return "", start, false
}

func pdfParseHexString(data []byte, start int) (string, int, bool) {
	if start >= len(data) || data[start] != '<' || (start+1 < len(data) && data[start+1] == '<') {
		return "", start, false
	}
	var nibbles []byte
	for i := start + 1; i < len(data); i++ {
		if data[i] == '>' {
			if len(nibbles)%2 == 1 {
				nibbles = append(nibbles, '0')
			}
			out := make([]byte, 0, len(nibbles)/2)
			for j := 0; j+1 < len(nibbles); j += 2 {
				hi, okH := pdfHexVal(nibbles[j])
				lo, okL := pdfHexVal(nibbles[j+1])
				if !okH || !okL {
					return "", start, false
				}
				out = append(out, hi<<4|lo)
			}
			return pdfDecodeString(out), i + 1, true
		}
		if pdfIsWS(data[i]) {
			continue
		}
		if _, ok := pdfHexVal(data[i]); !ok {
			return "", start, false
		}
		nibbles = append(nibbles, data[i])
	}
	return "", start, false
}

func pdfDecodeString(data []byte) string {
	if len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF {
		units := make([]uint16, 0, (len(data)-2)/2)
		for i := 2; i+1 < len(data); i += 2 {
			units = append(units, uint16(data[i])<<8|uint16(data[i+1]))
		}
		return string(utf16.Decode(units))
	}
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		units := make([]uint16, 0, (len(data)-2)/2)
		for i := 2; i+1 < len(data); i += 2 {
			units = append(units, uint16(data[i+1])<<8|uint16(data[i]))
		}
		return string(utf16.Decode(units))
	}
	return string(data)
}

func pdfReadToken(data []byte, start int) (string, int) {
	if start >= len(data) {
		return "", start
	}
	i := start
	for i < len(data) && !pdfIsWS(data[i]) && !pdfIsDelim(data[i]) {
		i++
	}
	if i == start {
		return string(data[start]), start + 1
	}
	return string(data[start:i]), i
}

func pdfSkipWS(data []byte, start int) int {
	for i := start; i < len(data); {
		for i < len(data) && pdfIsWS(data[i]) {
			i++
		}
		if i < len(data) && data[i] == '%' {
			for i < len(data) && data[i] != '\r' && data[i] != '\n' {
				i++
			}
			continue
		}
		return i
	}
	return len(data)
}

func pdfIsWS(c byte) bool {
	switch c {
	case 0x00, '\t', '\n', '\f', '\r', ' ':
		return true
	}
	return false
}

func pdfIsDelim(c byte) bool {
	switch c {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}

func pdfIsOctal(c byte) bool { return c >= '0' && c <= '7' }

func pdfHexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

func normalizePDFText(text string) string {
	text = strings.ReplaceAll(text, "\x00", "")
	text = normalizeText([]byte(text))
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			normalized = append(normalized, line)
		}
	}
	return strings.Join(normalized, "\n")
}
