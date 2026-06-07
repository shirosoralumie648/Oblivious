package knowledge

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
)

func TestScoreKnowledgeCandidatePrefersTitleMatchesOverChunkAndBody(t *testing.T) {
	terms := buildKnowledgeQueryTerms("deployment")

	titleScore := scoreKnowledgeCandidate("Deployment Runbook", "general notes", sql.NullString{}, terms)
	chunkScore := scoreKnowledgeCandidate("Runbook", "general notes", sql.NullString{String: "deployment checklist", Valid: true}, terms)
	bodyScore := scoreKnowledgeCandidate("Runbook", "deployment lives in the body", sql.NullString{}, terms)

	if !(titleScore > chunkScore && chunkScore > bodyScore) {
		t.Fatalf("expected title > chunk > body, got title=%d chunk=%d body=%d", titleScore, chunkScore, bodyScore)
	}
}

func TestBuildKnowledgeSnippetCentersTheMatchedTerm(t *testing.T) {
	content := "alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu nu xi omicron pi rho sigma tau upsilon phi chi psi omega boundary review context planning notes continue here before the retrieval phrase deployment controls are documented after this section with rollback notes and extended follow-up details for operators to study during incident response handoffs"
	snippet := buildKnowledgeSnippet(content, "deployment controls")

	if snippet == "" {
		t.Fatal("expected non-empty snippet")
	}
	if snippet == content {
		t.Fatalf("expected centered snippet, got full content %q", snippet)
	}
	if !strings.Contains(strings.ToLower(snippet), "deployment controls") {
		t.Fatalf("expected snippet to contain query, got %q", snippet)
	}
	if !strings.HasPrefix(snippet, "...") {
		t.Fatalf("expected leading ellipsis for centered snippet, got %q", snippet)
	}
}

func TestChooseKnowledgeSnippetSourcePrefersChunkWhenChunkHasMoreTermHits(t *testing.T) {
	terms := buildKnowledgeQueryTerms("deployment rollback")

	source := chooseKnowledgeSnippetSource(
		"General architecture notes without the query terms together.",
		sql.NullString{String: "Deployment rollback steps are documented in this chunk.", Valid: true},
		terms,
	)

	if source != "Deployment rollback steps are documented in this chunk." {
		t.Fatalf("expected chunk source, got %q", source)
	}
}

func TestSQLStoreRetrieveKnowledgeWithOptionsUsesCrossTenantSafeVectorSearch(t *testing.T) {
	driverName := "knowledge_retrieve_options_test"
	queryer := &knowledgeRetrievalQueryer{
		rows: [][]driver.Value{
			{
				"doc_1",
				"Deployment Runbook",
				"v2",
				"kdc_1",
				int64(3),
				"v2",
				"Deployment rollback restore runbook content.",
				[]byte(`{"pageNumber":7,"sourceUrl":"https://docs.example/runbook.md"}`),
				float64(0.92),
				float64(0.92),
				"embedding_rag",
			},
		},
	}
	registerKnowledgeRetrievalDriver(driverName, queryer)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	results, err := NewSQLStore(db).RetrieveKnowledgeWithOptions(
		context.Background(),
		"org_1",
		"kb_1",
		"deployment rollback",
		[]float32{0.1, 0.2, 0.3},
		KnowledgeRetrievalOptions{Mode: KnowledgeRetrievalModeVector, Limit: 3, MinScore: 0.5},
	)
	if err != nil {
		t.Fatalf("retrieve knowledge with options: %v", err)
	}

	queryer.mu.Lock()
	query := queryer.query
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()

	for _, want := range []string{
		"organization_id = $1",
		"kb.organization_id = $1",
		"d.organization_id = $1",
		"c.organization_id = $1",
		"c.embedding <=> $3::vector",
		"Similarity",
		"RetrievalMethod",
		"KnowledgeCitation",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected retrieval query to include %q, got %s", want, query)
		}
	}
	if got := knowledgeRetrievalArgString(args, 1); got != "org_1" {
		t.Fatalf("organization arg = %q, want org_1", got)
	}
	if got := knowledgeRetrievalArgString(args, 2); got != "kb_1" {
		t.Fatalf("knowledge base arg = %q, want kb_1", got)
	}
	if got := knowledgeRetrievalArgString(args, 3); got != "[0.100000,0.200000,0.300000]" {
		t.Fatalf("embedding arg = %q, want formatted vector", got)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	result := results[0]
	if result.DocumentID != "doc_1" || result.ChunkID != "kdc_1" || result.ChunkIndex != 3 {
		t.Fatalf("unexpected retrieval result identity: %+v", result)
	}
	if result.RetrievalMethod != KnowledgeRetrievalMethodEmbeddingRAG {
		t.Fatalf("expected embedding_rag retrieval method, got %q", result.RetrievalMethod)
	}
	if result.Similarity != 0.92 || result.Score != 0.92 {
		t.Fatalf("expected similarity and score 0.92, got similarity=%v score=%v", result.Similarity, result.Score)
	}
	if result.Source.DocumentTitle != "Deployment Runbook" || result.Source.PageNumber != 7 || result.Source.SourceURL != "https://docs.example/runbook.md" {
		t.Fatalf("unexpected citation source: %+v", result.Source)
	}
}

func TestSQLStoreRetrieveKnowledgeWithOptionsWithoutEmbeddingUsesKeywordOnlyQuery(t *testing.T) {
	driverName := "knowledge_retrieve_keyword_fallback_test"
	queryer := &knowledgeRetrievalQueryer{}
	registerKnowledgeRetrievalDriver(driverName, queryer)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	if _, err := NewSQLStore(db).RetrieveKnowledgeWithOptions(
		context.Background(),
		"org_1",
		"kb_1",
		"deployment rollback",
		nil,
		KnowledgeRetrievalOptions{Mode: KnowledgeRetrievalModeHybrid, Limit: 3},
	); err != nil {
		t.Fatalf("retrieve knowledge with keyword fallback: %v", err)
	}

	queryer.mu.Lock()
	query := queryer.query
	queryer.mu.Unlock()

	if strings.Contains(query, "<=>") {
		t.Fatalf("expected keyword fallback query not to use pgvector distance, got %s", query)
	}
	if !strings.Contains(query, "websearch_to_tsquery") || !strings.Contains(query, "organization_id = $1") {
		t.Fatalf("expected keyword fallback query to use tenant-scoped full text search, got %s", query)
	}
}

func TestSQLStoreCreateKnowledgeDocumentWithOptionsPersistsCrossTenantChunksAndEmbeddings(t *testing.T) {
	driverName := "knowledge_write_options_test"
	queryer := &knowledgeRetrievalQueryer{}
	registerKnowledgeRetrievalDriver(driverName, queryer)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	document, err := NewSQLStore(db).CreateKnowledgeDocumentWithOptions(
		context.Background(),
		"org_1",
		"kb_1",
		"Deployment Runbook",
		"Deployment rollback content",
		[]KnowledgeDocumentChunk{
			{
				ChunkIndex:      0,
				Content:         "Deployment rollback content",
				DocumentVersion: "v2",
				Embedding:       []float32{0.1, 0.2, 0.3},
				Metadata: KnowledgeChunkMetadata{
					DocumentVersion: "v2",
					PageNumber:      7,
					SourceURL:       "https://docs.example/runbook.md",
				},
			},
		},
		KnowledgeDocumentOptions{DocumentVersion: "v2", SourceURL: "https://docs.example/runbook.md", PageNumber: 7, UpdateStrategy: KnowledgeUpdateStrategyVersioned},
	)
	if err != nil {
		t.Fatalf("create knowledge document with options: %v", err)
	}
	if document.ID == "" || document.DocumentVersion != "v2" || document.UpdateStrategy != KnowledgeUpdateStrategyVersioned {
		t.Fatalf("expected returned document to include versioned metadata, got %+v", document)
	}

	queryer.mu.Lock()
	execQueries := strings.Join(queryer.execQueries, "\n")
	execArgs := append([][]driver.NamedValue(nil), queryer.execArgs...)
	queryer.mu.Unlock()

	for _, want := range []string{
		"knowledge_documents",
		"knowledge_document_chunks",
		"organization_id",
		"embedding",
		"embedding_model",
		"metadata",
		"document_version",
	} {
		if !strings.Contains(execQueries, want) {
			t.Fatalf("expected write queries to include %q, got %s", want, execQueries)
		}
	}
	if len(execArgs) < 2 {
		t.Fatalf("expected document and chunk insert execs, got %#v", execArgs)
	}
	if got := knowledgeRetrievalArgString(execArgs[0], 2); got != "org_1" {
		t.Fatalf("document organization arg = %q, want org_1", got)
	}
	chunkArgs := knowledgeRetrievalExecArgsForQuery(t, queryer, "INSERT INTO knowledge_document_chunks")
	if got := knowledgeRetrievalArgString(chunkArgs, 3); got != "org_1" {
		t.Fatalf("chunk organization arg = %q, want org_1", got)
	}
	if got := knowledgeRetrievalArgString(chunkArgs, 6); got != "[0.100000,0.200000,0.300000]" {
		t.Fatalf("chunk embedding arg = %q, want formatted vector", got)
	}
	if got := knowledgeRetrievalArgString(chunkArgs, 10); !strings.Contains(got, `"sourceUrl":"https://docs.example/runbook.md"`) {
		t.Fatalf("chunk metadata arg = %q, want source URL metadata", got)
	}
}

var knowledgeRetrievalDrivers sync.Map

func registerKnowledgeRetrievalDriver(name string, queryer *knowledgeRetrievalQueryer) {
	if _, loaded := knowledgeRetrievalDrivers.LoadOrStore(name, queryer); loaded {
		return
	}
	sql.Register(name, knowledgeRetrievalDriver{name: name})
}

type knowledgeRetrievalQueryer struct {
	mu          sync.Mutex
	query       string
	args        []driver.NamedValue
	rows        [][]driver.Value
	execQueries []string
	execArgs    [][]driver.NamedValue
}

type knowledgeRetrievalDriver struct {
	name string
}

func (d knowledgeRetrievalDriver) Open(_ string) (driver.Conn, error) {
	queryer, _ := knowledgeRetrievalDrivers.Load(d.name)
	return knowledgeRetrievalConn{queryer: queryer.(*knowledgeRetrievalQueryer)}, nil
}

type knowledgeRetrievalConn struct {
	queryer *knowledgeRetrievalQueryer
}

func (c knowledgeRetrievalConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, driver.ErrSkip
}

func (c knowledgeRetrievalConn) Close() error {
	return nil
}

func (c knowledgeRetrievalConn) Begin() (driver.Tx, error) {
	return knowledgeRetrievalTx{}, nil
}

func (c knowledgeRetrievalConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.queryer.mu.Lock()
	c.queryer.execQueries = append(c.queryer.execQueries, query)
	c.queryer.execArgs = append(c.queryer.execArgs, append([]driver.NamedValue(nil), args...))
	c.queryer.mu.Unlock()
	return driver.RowsAffected(1), nil
}

func (c knowledgeRetrievalConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.queryer.mu.Lock()
	c.queryer.query = query
	c.queryer.args = append([]driver.NamedValue(nil), args...)
	rows := append([][]driver.Value(nil), c.queryer.rows...)
	c.queryer.mu.Unlock()
	return &knowledgeRetrievalRows{
		columns: []string{
			"document_id",
			"document_title",
			"document_version",
			"chunk_id",
			"chunk_index",
			"chunk_version",
			"content",
			"metadata",
			"similarity",
			"score",
			"retrieval_method",
		},
		rows: rows,
	}, nil
}

func (c knowledgeRetrievalConn) CheckNamedValue(_ *driver.NamedValue) error {
	return nil
}

type knowledgeRetrievalTx struct{}

func (knowledgeRetrievalTx) Commit() error {
	return nil
}

func (knowledgeRetrievalTx) Rollback() error {
	return nil
}

type knowledgeRetrievalRows struct {
	columns []string
	index   int
	rows    [][]driver.Value
}

func (r *knowledgeRetrievalRows) Columns() []string {
	return r.columns
}

func (r *knowledgeRetrievalRows) Close() error {
	return nil
}

func (r *knowledgeRetrievalRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func knowledgeRetrievalArgString(args []driver.NamedValue, ordinal int) string {
	for _, arg := range args {
		if arg.Ordinal == ordinal {
			value, _ := arg.Value.(string)
			return value
		}
	}
	return ""
}

func knowledgeRetrievalExecArgsForQuery(t *testing.T, queryer *knowledgeRetrievalQueryer, pattern string) []driver.NamedValue {
	t.Helper()
	queryer.mu.Lock()
	defer queryer.mu.Unlock()
	for index, query := range queryer.execQueries {
		if strings.Contains(query, pattern) {
			return append([]driver.NamedValue(nil), queryer.execArgs[index]...)
		}
	}
	t.Fatalf("expected exec query matching %q, got %v", pattern, queryer.execQueries)
	return nil
}

func TestSQLStoreKnowledgeChunkMetadataJSONShape(t *testing.T) {
	metadata := KnowledgeChunkMetadata{PageNumber: 7, SourceURL: "https://docs.example/runbook.md"}
	raw, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	if !strings.Contains(string(raw), "pageNumber") || !strings.Contains(string(raw), "sourceUrl") {
		t.Fatalf("expected chunk metadata JSON shape, got %s", raw)
	}
}
