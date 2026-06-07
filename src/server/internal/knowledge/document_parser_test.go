package knowledge

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
)

func TestParseUploadedDocumentAcceptsPlainTextAndMarkdown(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		contentType string
		input       string
		wantTitle   string
		wantFormat  string
		wantContent string
	}{
		{
			name:        "plain text by extension",
			filename:    " Runbook.TXT ",
			contentType: "",
			input:       "\ufeff  deploy rollback steps\r\n\r\nverify health  ",
			wantTitle:   "Runbook.TXT",
			wantFormat:  KnowledgeDocumentFormatText,
			wantContent: "deploy rollback steps\n\nverify health",
		},
		{
			name:        "markdown by content type",
			filename:    "guide",
			contentType: "text/markdown; charset=utf-8",
			input:       "# Guide\n\n- Step one",
			wantTitle:   "guide",
			wantFormat:  KnowledgeDocumentFormatMarkdown,
			wantContent: "# Guide\n\n- Step one",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document, err := ParseUploadedDocument(context.Background(), UploadedDocumentInput{
				Filename:    tt.filename,
				ContentType: tt.contentType,
				Reader:      strings.NewReader(tt.input),
				MaxBytes:    1024,
			})
			if err != nil {
				t.Fatalf("parse uploaded document: %v", err)
			}

			if document.Title != tt.wantTitle {
				t.Fatalf("expected title %q, got %q", tt.wantTitle, document.Title)
			}
			if document.Format != tt.wantFormat {
				t.Fatalf("expected format %q, got %q", tt.wantFormat, document.Format)
			}
			if document.Content != tt.wantContent {
				t.Fatalf("expected content %q, got %q", tt.wantContent, document.Content)
			}
			if document.SizeBytes <= 0 {
				t.Fatalf("expected positive size, got %d", document.SizeBytes)
			}
		})
	}
}

func TestParseUploadedDocumentExtractsSimplePDFText(t *testing.T) {
	document, err := ParseUploadedDocument(context.Background(), UploadedDocumentInput{
		Filename:    " Handbook.PDF ",
		ContentType: "application/pdf",
		Reader: strings.NewReader(`%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>
endobj
4 0 obj
<< /Length 92 >>
stream
BT
72 720 Td
(Deploy rollback steps) Tj
0 -20 Td
[(Verify ) 20 (health checks)] TJ
ET
endstream
endobj
%%EOF`),
		MaxBytes: 2048,
	})
	if err != nil {
		t.Fatalf("parse pdf document: %v", err)
	}

	if document.Title != "Handbook.PDF" {
		t.Fatalf("expected title from filename, got %q", document.Title)
	}
	if document.Format != KnowledgeDocumentFormatPDF {
		t.Fatalf("expected format %q, got %q", KnowledgeDocumentFormatPDF, document.Format)
	}
	if document.Content != "Deploy rollback steps\nVerify health checks" {
		t.Fatalf("expected extracted pdf text, got %q", document.Content)
	}
	if document.SizeBytes <= 0 {
		t.Fatalf("expected positive size, got %d", document.SizeBytes)
	}
}

func TestParseUploadedDocumentExtractsFlatePDFText(t *testing.T) {
	compressedStream := compressKnowledgePDFStream(t, []byte(`BT
72 720 Td
(Compressed onboarding runbook) Tj
0 -20 Td
[(Verify ) 15 (retrieval)] TJ
ET`))

	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n")
	pdf.WriteString("4 0 obj\n")
	pdf.WriteString("<< /Length ")
	pdf.WriteString(strconv.Itoa(len(compressedStream)))
	pdf.WriteString(" /Filter /FlateDecode >>\n")
	pdf.WriteString("stream\n")
	pdf.Write(compressedStream)
	pdf.WriteString("\nendstream\n")
	pdf.WriteString("endobj\n")
	pdf.WriteString("%%EOF")

	document, err := ParseUploadedDocument(context.Background(), UploadedDocumentInput{
		Filename:    "compressed.pdf",
		ContentType: "application/pdf",
		Reader:      bytes.NewReader(pdf.Bytes()),
		MaxBytes:    2048,
	})
	if err != nil {
		t.Fatalf("parse flate pdf document: %v", err)
	}

	if document.Format != KnowledgeDocumentFormatPDF {
		t.Fatalf("expected format %q, got %q", KnowledgeDocumentFormatPDF, document.Format)
	}
	if document.Content != "Compressed onboarding runbook\nVerify retrieval" {
		t.Fatalf("expected extracted flate pdf text, got %q", document.Content)
	}
}

func TestParseUploadedDocumentAcceptsPDFByExtension(t *testing.T) {
	document, err := ParseUploadedDocument(context.Background(), UploadedDocumentInput{
		Filename:    "manual.pdf",
		ContentType: "",
		Reader: strings.NewReader(`%PDF-1.4
4 0 obj
<< /Length 33 >>
stream
BT
(Extension based PDF) Tj
ET
endstream
endobj
%%EOF`),
		MaxBytes: 1024,
	})
	if err != nil {
		t.Fatalf("parse pdf document by extension: %v", err)
	}
	if document.Format != KnowledgeDocumentFormatPDF {
		t.Fatalf("expected format %q, got %q", KnowledgeDocumentFormatPDF, document.Format)
	}
	if document.Content != "Extension based PDF" {
		t.Fatalf("expected extracted pdf text, got %q", document.Content)
	}
}

func TestParseUploadedDocumentExtractsDOCXText(t *testing.T) {
	docx := minimalKnowledgeDOCX(t, `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Deploy &amp; rollback</w:t></w:r><w:r><w:t> steps</w:t></w:r></w:p>
    <w:p><w:r><w:t>Verify health checks</w:t></w:r></w:p>
  </w:body>
</w:document>`)

	tests := []struct {
		name        string
		filename    string
		contentType string
	}{
		{
			name:     "docx by extension",
			filename: "Manual.DOCX",
		},
		{
			name:        "docx by content type",
			filename:    "manual",
			contentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document, err := ParseUploadedDocument(context.Background(), UploadedDocumentInput{
				Filename:    tt.filename,
				ContentType: tt.contentType,
				Reader:      bytes.NewReader(docx),
				MaxBytes:    4096,
			})
			if err != nil {
				t.Fatalf("parse docx document: %v", err)
			}

			if document.Format != KnowledgeDocumentFormatDOCX {
				t.Fatalf("expected format %q, got %q", KnowledgeDocumentFormatDOCX, document.Format)
			}
			if document.Content != "Deploy & rollback steps\nVerify health checks" {
				t.Fatalf("expected extracted docx text, got %q", document.Content)
			}
			if document.SizeBytes <= 0 {
				t.Fatalf("expected positive size, got %d", document.SizeBytes)
			}
		})
	}
}

func TestParseUploadedDocumentRejectsLegacyDOC(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		contentType string
	}{
		{
			name:     "doc extension",
			filename: "manual.doc",
		},
		{
			name:        "doc content type",
			filename:    "manual",
			contentType: "application/msword",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseUploadedDocument(context.Background(), UploadedDocumentInput{
				Filename:    tt.filename,
				ContentType: tt.contentType,
				Reader:      strings.NewReader("word document bytes"),
				MaxBytes:    1024,
			})
			if !errors.Is(err, ErrUnsupportedKnowledgeDocumentFormat) {
				t.Fatalf("expected unsupported format error, got %v", err)
			}
		})
	}
}

func TestParseUploadedDocumentRejectsEmptyAndOversizedUploads(t *testing.T) {
	_, err := ParseUploadedDocument(context.Background(), UploadedDocumentInput{
		Filename: "empty.txt",
		Reader:   strings.NewReader("   \n\t "),
		MaxBytes: 1024,
	})
	if !errors.Is(err, ErrEmptyKnowledgeDocument) {
		t.Fatalf("expected empty document error, got %v", err)
	}

	_, err = ParseUploadedDocument(context.Background(), UploadedDocumentInput{
		Filename: "large.txt",
		Reader:   strings.NewReader("abcdef"),
		MaxBytes: 4,
	})
	if !errors.Is(err, ErrKnowledgeDocumentTooLarge) {
		t.Fatalf("expected too large error, got %v", err)
	}
}

func minimalKnowledgeDOCX(t *testing.T, documentXML string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	file, err := writer.Create("word/document.xml")
	if err != nil {
		t.Fatalf("create document.xml: %v", err)
	}
	if _, err := io.WriteString(file, documentXML); err != nil {
		t.Fatalf("write document.xml: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close docx zip: %v", err)
	}
	return buffer.Bytes()
}

func compressKnowledgePDFStream(t *testing.T, stream []byte) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zlib.NewWriter(&buffer)
	if _, err := writer.Write(stream); err != nil {
		t.Fatalf("compress pdf stream: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close compressed pdf stream: %v", err)
	}
	return buffer.Bytes()
}
