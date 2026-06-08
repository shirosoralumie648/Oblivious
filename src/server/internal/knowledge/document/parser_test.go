package document

import (
	"context"
	"strings"
	"testing"

	"oblivious/server/internal/knowledge"
)

func TestParserParsesAdditionalKnowledgeDocumentFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		filename    string
		contentType string
		body        string
		wantFormat  string
		wantContent string
	}{
		{
			name:        "csv",
			filename:    "matrix.csv",
			contentType: "text/csv",
			body:        "title,owner\nDeploy,Ops",
			wantFormat:  knowledge.KnowledgeDocumentFormatCSV,
			wantContent: "title | owner\nDeploy | Ops",
		},
		{
			name:        "html",
			filename:    "runbook.html",
			contentType: "text/html",
			body:        "<html><head><style>.hidden{display:none}</style><script>alert('x')</script></head><body><h1>Deploy Plan</h1><p>Rollback safely</p></body></html>",
			wantFormat:  knowledge.KnowledgeDocumentFormatHTML,
			wantContent: "Deploy Plan\nRollback safely",
		},
		{
			name:        "json",
			filename:    "fixture.json",
			contentType: "application/json",
			body:        `{"title":"Deploy","owner":"Ops"}`,
			wantFormat:  knowledge.KnowledgeDocumentFormatJSON,
			wantContent: `{"title":"Deploy","owner":"Ops"}`,
		},
		{
			name:        "xml",
			filename:    "fixture.xml",
			contentType: "application/xml",
			body:        "<doc><title>Deploy</title><owner>Ops</owner></doc>",
			wantFormat:  knowledge.KnowledgeDocumentFormatXML,
			wantContent: "<doc><title>Deploy</title><owner>Ops</owner></doc>",
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			document, err := parser.Parse(context.Background(), strings.NewReader(tt.body), tt.filename, tt.contentType, 1024)
			if err != nil {
				t.Fatalf("parse document: %v", err)
			}
			if document.Format != tt.wantFormat {
				t.Fatalf("expected format %q, got %q", tt.wantFormat, document.Format)
			}
			if document.Content != tt.wantContent {
				t.Fatalf("expected content %q, got %q", tt.wantContent, document.Content)
			}
		})
	}
}
