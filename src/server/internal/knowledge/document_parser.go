package knowledge

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

const (
	KnowledgeDocumentFormatDOCX     = "docx"
	KnowledgeDocumentFormatMarkdown = "markdown"
	KnowledgeDocumentFormatPDF      = "pdf"
	KnowledgeDocumentFormatText     = "text"

	defaultKnowledgeDocumentUploadMaxBytes = 10 * 1024 * 1024
)

var (
	ErrEmptyKnowledgeDocument             = errors.New("empty knowledge document")
	ErrKnowledgeDocumentTooLarge          = errors.New("knowledge document too large")
	ErrUnsupportedKnowledgeDocumentFormat = errors.New("unsupported knowledge document format")
)

type UploadedDocumentInput struct {
	Reader      io.Reader
	Filename    string
	ContentType string
	MaxBytes    int64
}

type ParsedUploadedDocument struct {
	Content     string
	Format      string
	Title       string
	ContentType string
	SizeBytes   int64
}

func ParseUploadedDocument(ctx context.Context, input UploadedDocumentInput) (ParsedUploadedDocument, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return ParsedUploadedDocument{}, err
	}
	if input.Reader == nil {
		return ParsedUploadedDocument{}, ErrEmptyKnowledgeDocument
	}

	format, normalizedContentType, err := detectUploadedKnowledgeDocumentFormat(input.Filename, input.ContentType)
	if err != nil {
		return ParsedUploadedDocument{}, err
	}

	maxBytes := input.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultKnowledgeDocumentUploadMaxBytes
	}
	data, err := readUploadedKnowledgeDocument(input.Reader, maxBytes)
	if err != nil {
		return ParsedUploadedDocument{}, err
	}
	content := ""
	switch format {
	case KnowledgeDocumentFormatDOCX:
		content, err = extractUploadedKnowledgeDOCXText(data)
		if err != nil {
			return ParsedUploadedDocument{}, err
		}
	case KnowledgeDocumentFormatPDF:
		content = extractUploadedKnowledgePDFText(data)
	default:
		content = normalizeUploadedKnowledgeDocumentText(data)
	}
	if content == "" {
		return ParsedUploadedDocument{}, ErrEmptyKnowledgeDocument
	}

	return ParsedUploadedDocument{
		Content:     content,
		Format:      format,
		Title:       normalizeUploadedKnowledgeDocumentTitle(input.Filename),
		ContentType: normalizedContentType,
		SizeBytes:   int64(len(data)),
	}, nil
}

func readUploadedKnowledgeDocument(reader io.Reader, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(reader, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrKnowledgeDocumentTooLarge
	}
	return data, nil
}

func detectUploadedKnowledgeDocumentFormat(filename, contentType string) (string, string, error) {
	mediaType := strings.TrimSpace(strings.ToLower(contentType))
	if mediaType != "" {
		parsed, _, err := mime.ParseMediaType(mediaType)
		if err == nil {
			mediaType = parsed
		}
	}

	switch mediaType {
	case "text/plain":
		return KnowledgeDocumentFormatText, mediaType, nil
	case "text/markdown", "text/x-markdown":
		return KnowledgeDocumentFormatMarkdown, mediaType, nil
	case "application/pdf":
		return KnowledgeDocumentFormatPDF, mediaType, nil
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return KnowledgeDocumentFormatDOCX, mediaType, nil
	case "application/msword":
		return "", mediaType, fmt.Errorf("%w: %s", ErrUnsupportedKnowledgeDocumentFormat, mediaType)
	}

	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	switch extension {
	case ".txt", ".text":
		return KnowledgeDocumentFormatText, mediaType, nil
	case ".md", ".markdown":
		return KnowledgeDocumentFormatMarkdown, mediaType, nil
	case ".pdf":
		return KnowledgeDocumentFormatPDF, mediaType, nil
	case ".docx":
		return KnowledgeDocumentFormatDOCX, mediaType, nil
	case ".doc":
		return "", mediaType, fmt.Errorf("%w: %s", ErrUnsupportedKnowledgeDocumentFormat, extension)
	}

	if mediaType == "" && extension == "" {
		return KnowledgeDocumentFormatText, mediaType, nil
	}
	return "", mediaType, fmt.Errorf("%w: %s", ErrUnsupportedKnowledgeDocumentFormat, strings.TrimSpace(filename))
}

func normalizeUploadedKnowledgeDocumentText(data []byte) string {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.TrimSpace(text)
}

func normalizeUploadedKnowledgeDocumentTitle(filename string) string {
	title := strings.TrimSpace(filepath.Base(filename))
	if title == "." || title == string(filepath.Separator) {
		return ""
	}
	return title
}

func extractUploadedKnowledgeDOCXText(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("%w: docx archive", ErrUnsupportedKnowledgeDocumentFormat)
	}

	var documentXML io.ReadCloser
	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}
		documentXML, err = file.Open()
		if err != nil {
			return "", err
		}
		defer documentXML.Close()
		break
	}
	if documentXML == nil {
		return "", fmt.Errorf("%w: missing word/document.xml", ErrUnsupportedKnowledgeDocumentFormat)
	}

	decoder := xml.NewDecoder(documentXML)
	var paragraph strings.Builder
	var paragraphs []string
	inText := false

	flushParagraph := func() {
		text := strings.TrimSpace(paragraph.String())
		if text != "" {
			paragraphs = append(paragraphs, text)
		}
		paragraph.Reset()
	}

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}

		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "t" {
				inText = true
			}
		case xml.EndElement:
			switch value.Name.Local {
			case "t":
				inText = false
			case "p":
				flushParagraph()
			}
		case xml.CharData:
			if inText {
				paragraph.Write([]byte(value))
			}
		}
	}
	flushParagraph()

	return normalizeUploadedKnowledgeDocumentText([]byte(strings.Join(paragraphs, "\n"))), nil
}

func extractUploadedKnowledgePDFText(data []byte) string {
	var blocks []string
	offset := 0
	for offset < len(data) {
		streamKeyword := bytes.Index(data[offset:], []byte("stream"))
		if streamKeyword < 0 {
			break
		}
		streamKeyword += offset
		streamStart := streamKeyword + len("stream")
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
		if stream, ok := decodeUploadedKnowledgePDFStream(data, streamKeyword, data[streamStart:streamEnd]); ok {
			if text := extractUploadedKnowledgePDFStreamText(stream); text != "" {
				blocks = append(blocks, text)
			}
		}
		offset = streamEnd + len("endstream")
	}
	return normalizeExtractedKnowledgePDFText(strings.Join(blocks, "\n"))
}

func decodeUploadedKnowledgePDFStream(data []byte, streamKeyword int, stream []byte) ([]byte, bool) {
	header := uploadedKnowledgePDFStreamHeader(data, streamKeyword)
	if !bytes.Contains(header, []byte("/Filter")) {
		return stream, true
	}
	if !uploadedKnowledgePDFStreamHasOnlyFlateDecodeFilter(header) {
		return nil, false
	}

	reader, err := zlib.NewReader(bytes.NewReader(stream))
	if err != nil {
		return nil, false
	}
	defer reader.Close()

	decoded, err := io.ReadAll(io.LimitReader(reader, defaultKnowledgeDocumentUploadMaxBytes+1))
	if err != nil {
		return nil, false
	}
	if len(decoded) > defaultKnowledgeDocumentUploadMaxBytes {
		return nil, false
	}
	return decoded, true
}

func uploadedKnowledgePDFStreamHeader(data []byte, streamKeyword int) []byte {
	headerStart := streamKeyword - 512
	if headerStart < 0 {
		headerStart = 0
	}
	return data[headerStart:streamKeyword]
}

func uploadedKnowledgePDFStreamHasOnlyFlateDecodeFilter(header []byte) bool {
	filterIndex := bytes.Index(header, []byte("/Filter"))
	if filterIndex < 0 {
		return false
	}
	_, offset := readKnowledgePDFName(header, filterIndex)
	offset = skipKnowledgePDFWhitespaceAndComments(header, offset)
	if offset >= len(header) {
		return false
	}

	if header[offset] == '/' {
		name, _ := readKnowledgePDFName(header, offset)
		return name == "FlateDecode" || name == "Fl"
	}

	if header[offset] != '[' {
		return false
	}
	offset++
	filterCount := 0
	hasFlate := false
	for offset < len(header) {
		offset = skipKnowledgePDFWhitespaceAndComments(header, offset)
		if offset >= len(header) {
			return false
		}
		if header[offset] == ']' {
			return filterCount == 1 && hasFlate
		}
		if header[offset] != '/' {
			return false
		}
		name, next := readKnowledgePDFName(header, offset)
		filterCount++
		if name == "FlateDecode" || name == "Fl" {
			hasFlate = true
		}
		offset = next
	}
	return false
}

func readKnowledgePDFName(data []byte, start int) (string, int) {
	if start >= len(data) || data[start] != '/' {
		return "", start
	}
	i := start + 1
	for i < len(data) && !isKnowledgePDFWhitespace(data[i]) && !isKnowledgePDFDelimiter(data[i]) {
		i++
	}
	return string(data[start+1 : i]), i
}

type uploadedKnowledgePDFTextBuilder struct {
	current strings.Builder
	lines   []string
}

func (b *uploadedKnowledgePDFTextBuilder) add(text string) {
	if text == "" {
		return
	}
	b.current.WriteString(text)
}

func (b *uploadedKnowledgePDFTextBuilder) newLine() {
	line := strings.TrimSpace(b.current.String())
	if line == "" {
		b.current.Reset()
		return
	}
	b.lines = append(b.lines, line)
	b.current.Reset()
}

func (b *uploadedKnowledgePDFTextBuilder) string() string {
	b.newLine()
	return strings.Join(b.lines, "\n")
}

func extractUploadedKnowledgePDFStreamText(stream []byte) string {
	builder := uploadedKnowledgePDFTextBuilder{}
	var textOperands []string

	for i := 0; i < len(stream); {
		i = skipKnowledgePDFWhitespaceAndComments(stream, i)
		if i >= len(stream) {
			break
		}

		switch stream[i] {
		case '(':
			text, next, ok := parseKnowledgePDFLiteralString(stream, i)
			if ok {
				textOperands = append(textOperands, text)
				i = next
				continue
			}
		case '[':
			text, next, ok := parseKnowledgePDFTextArray(stream, i)
			if ok {
				textOperands = append(textOperands, text)
				i = next
				continue
			}
		case '<':
			text, next, ok := parseKnowledgePDFHexString(stream, i)
			if ok {
				textOperands = append(textOperands, text)
				i = next
				continue
			}
		}

		token, next := readKnowledgePDFToken(stream, i)
		switch token {
		case "Tj", "TJ":
			builder.add(lastKnowledgePDFTextOperand(textOperands))
			textOperands = nil
		case "'", "\"":
			builder.newLine()
			builder.add(lastKnowledgePDFTextOperand(textOperands))
			textOperands = nil
		case "Td", "TD", "T*":
			builder.newLine()
			textOperands = nil
		case "BT", "ET":
			textOperands = nil
		}
		i = next
	}

	return builder.string()
}

func lastKnowledgePDFTextOperand(operands []string) string {
	for i := len(operands) - 1; i >= 0; i-- {
		if operands[i] != "" {
			return operands[i]
		}
	}
	return ""
}

func parseKnowledgePDFTextArray(data []byte, start int) (string, int, bool) {
	if start >= len(data) || data[start] != '[' {
		return "", start, false
	}
	var text strings.Builder
	depth := 1
	for i := start + 1; i < len(data); {
		i = skipKnowledgePDFWhitespaceAndComments(data, i)
		if i >= len(data) {
			break
		}

		switch data[i] {
		case '(':
			value, next, ok := parseKnowledgePDFLiteralString(data, i)
			if !ok {
				return "", start, false
			}
			text.WriteString(value)
			i = next
		case '<':
			value, next, ok := parseKnowledgePDFHexString(data, i)
			if ok {
				text.WriteString(value)
				i = next
			} else {
				_, i = readKnowledgePDFToken(data, i)
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
			_, i = readKnowledgePDFToken(data, i)
		}
	}
	return "", start, false
}

func parseKnowledgePDFLiteralString(data []byte, start int) (string, int, bool) {
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
				if isKnowledgePDFOctalDigit(next) {
					value := 0
					digits := 0
					i++
					for i < len(data) && digits < 3 && isKnowledgePDFOctalDigit(data[i]) {
						value = value*8 + int(data[i]-'0')
						i++
						digits++
					}
					out = append(out, byte(value))
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
				return decodeKnowledgePDFStringBytes(out), i + 1, true
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
		i++
	}
	return "", start, false
}

func parseKnowledgePDFHexString(data []byte, start int) (string, int, bool) {
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
				hi, okHi := knowledgePDFHexValue(nibbles[j])
				lo, okLo := knowledgePDFHexValue(nibbles[j+1])
				if !okHi || !okLo {
					return "", start, false
				}
				out = append(out, hi<<4|lo)
			}
			return decodeKnowledgePDFStringBytes(out), i + 1, true
		}
		if isKnowledgePDFWhitespace(data[i]) {
			continue
		}
		if _, ok := knowledgePDFHexValue(data[i]); !ok {
			return "", start, false
		}
		nibbles = append(nibbles, data[i])
	}
	return "", start, false
}

func decodeKnowledgePDFStringBytes(data []byte) string {
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

func readKnowledgePDFToken(data []byte, start int) (string, int) {
	if start >= len(data) {
		return "", start
	}
	i := start
	for i < len(data) && !isKnowledgePDFWhitespace(data[i]) && !isKnowledgePDFDelimiter(data[i]) {
		i++
	}
	if i == start {
		return string(data[start]), start + 1
	}
	return string(data[start:i]), i
}

func skipKnowledgePDFWhitespaceAndComments(data []byte, start int) int {
	for i := start; i < len(data); {
		for i < len(data) && isKnowledgePDFWhitespace(data[i]) {
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

func isKnowledgePDFWhitespace(c byte) bool {
	switch c {
	case 0x00, '\t', '\n', '\f', '\r', ' ':
		return true
	default:
		return false
	}
}

func isKnowledgePDFDelimiter(c byte) bool {
	switch c {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	default:
		return false
	}
}

func isKnowledgePDFOctalDigit(c byte) bool {
	return c >= '0' && c <= '7'
}

func knowledgePDFHexValue(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

func normalizeExtractedKnowledgePDFText(text string) string {
	text = strings.ReplaceAll(text, "\x00", "")
	text = normalizeUploadedKnowledgeDocumentText([]byte(text))
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
