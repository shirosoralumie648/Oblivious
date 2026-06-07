package knowledge

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
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

func TestSQLStoreListAndGetKnowledgeBasesReturnRAGConfig(t *testing.T) {
	driverName := "knowledge_base_config_read_test"
	now := time.Date(2026, time.June, 7, 9, 0, 0, 0, time.UTC)
	queryer := &knowledgeRetrievalQueryer{
		rowsByQuery: map[string][][]driver.Value{
			"FROM knowledge_bases": {
				{
					"kb_1",
					"Ops KB",
					int64(2),
					"vector_only",
					int64(8),
					float64(0.42),
					float64(0.8),
					float64(0.2),
					"bge-reranker-large",
					int64(12),
					"semantic",
					int64(900),
					int64(120),
					"text-embedding-3-large",
					"incremental",
					now,
				},
			},
		},
	}
	registerKnowledgeRetrievalDriver(driverName, queryer)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	listed, err := NewSQLStore(db).ListKnowledgeBases(context.Background(), "org_1")
	if err != nil {
		t.Fatalf("list knowledge bases: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected one knowledge base, got %+v", listed)
	}
	if listed[0].RetrievalMode != KnowledgeRetrievalModeVector || listed[0].RetrievalLimit != 8 || listed[0].ChunkStrategy != KnowledgeChunkStrategySemantic {
		t.Fatalf("expected listed base to include RAG config, got %+v", listed[0])
	}
	if listed[0].EmbeddingModel != "text-embedding-3-large" || listed[0].UpdateStrategy != KnowledgeUpdateStrategyIncremental {
		t.Fatalf("expected listed base to include embedding/update config, got %+v", listed[0])
	}

	detail, err := NewSQLStore(db).GetKnowledgeBase(context.Background(), "org_1", "kb_1")
	if err != nil {
		t.Fatalf("get knowledge base: %v", err)
	}
	if detail.RetrievalMode != KnowledgeRetrievalModeVector || detail.RerankerModel != "bge-reranker-large" || detail.RerankTopK != 12 {
		t.Fatalf("expected detail base to include RAG config, got %+v", detail)
	}
}

func TestSQLStoreCreateRetrievalTestCasePersistsExpectedResult(t *testing.T) {
	driverName := "knowledge_retrieval_test_case_create_test"
	queryer := &knowledgeRetrievalQueryer{
		rowsByQuery: map[string][][]driver.Value{
			"INSERT INTO knowledge_retrieval_test_cases": {
				{
					"krtc_1",
					"kb_1",
					"deployment rollback",
					"doc_1",
					"Deployment Runbook",
					"v2",
					"kdc_1",
					int64(3),
					"rollback snippet",
					[]byte(`{"documentId":"doc_1","documentTitle":"Deployment Runbook","documentVersion":"v2","chunkId":"kdc_1","chunkIndex":3,"snippet":"rollback snippet","score":0.91}`),
				},
			},
		},
	}
	registerKnowledgeRetrievalDriver(driverName, queryer)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	testCase, err := NewSQLStore(db).CreateRetrievalTestCase(context.Background(), "org_1", "kb_1", CreateKnowledgeRetrievalTestCaseRequest{
		Query: "deployment rollback",
		ExpectedResult: KnowledgeRetrievalResult{
			ChunkID:         "kdc_1",
			ChunkIndex:      3,
			DocumentID:      "doc_1",
			DocumentTitle:   "Deployment Runbook",
			DocumentVersion: "v2",
			Score:           0.91,
			Snippet:         "rollback snippet",
		},
	})
	if err != nil {
		t.Fatalf("create retrieval test case: %v", err)
	}
	if testCase.ID != "krtc_1" || testCase.KnowledgeBaseID != "kb_1" || testCase.ExpectedChunkID != "kdc_1" {
		t.Fatalf("unexpected retrieval test case: %+v", testCase)
	}
	if testCase.ExpectedResult.DocumentID != "doc_1" || testCase.ExpectedResult.ChunkIndex != 3 {
		t.Fatalf("expected result was not decoded, got %+v", testCase.ExpectedResult)
	}

	queryer.mu.Lock()
	query := queryer.query
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()
	for _, want := range []string{
		"INSERT INTO knowledge_retrieval_test_cases",
		"SELECT",
		"FROM knowledge_bases",
		"kb.organization_id = $1",
		"kb.id = $2",
		"expected_result",
		"RETURNING",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected create retrieval test case query to include %q, got %s", want, query)
		}
	}
	if got := knowledgeRetrievalArgString(args, 1); got != "org_1" {
		t.Fatalf("organization arg = %q, want org_1", got)
	}
	if got := knowledgeRetrievalArgString(args, 2); got != "kb_1" {
		t.Fatalf("knowledge base arg = %q, want kb_1", got)
	}
	if got := knowledgeRetrievalArgString(args, 4); got != "deployment rollback" {
		t.Fatalf("query arg = %q, want deployment rollback", got)
	}
	if got := knowledgeRetrievalArgString(args, 11); !strings.Contains(got, `"chunkId":"kdc_1"`) {
		t.Fatalf("expected result arg = %q, want chunk id JSON", got)
	}
}

func TestSQLStoreListRetrievalTestCasesReturnsSavedExpectedResults(t *testing.T) {
	driverName := "knowledge_retrieval_test_case_list_test"
	queryer := &knowledgeRetrievalQueryer{
		rowsByQuery: map[string][][]driver.Value{
			"FROM knowledge_retrieval_test_cases": {
				{
					"krtc_1",
					"kb_1",
					"deployment rollback",
					"doc_1",
					"Deployment Runbook",
					"v2",
					"kdc_1",
					int64(3),
					"rollback snippet",
					[]byte(`{"documentId":"doc_1","documentTitle":"Deployment Runbook","documentVersion":"v2","chunkId":"kdc_1","chunkIndex":3,"snippet":"rollback snippet","score":0.91}`),
				},
			},
		},
	}
	registerKnowledgeRetrievalDriver(driverName, queryer)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	testCases, err := NewSQLStore(db).ListRetrievalTestCases(context.Background(), "org_1", "kb_1")
	if err != nil {
		t.Fatalf("list retrieval test cases: %v", err)
	}
	if len(testCases) != 1 {
		t.Fatalf("expected one retrieval test case, got %+v", testCases)
	}
	testCase := testCases[0]
	if testCase.ID != "krtc_1" || testCase.Query != "deployment rollback" || testCase.ExpectedDocumentVersion != "v2" {
		t.Fatalf("unexpected retrieval test case: %+v", testCase)
	}
	if testCase.ExpectedResult.DocumentTitle != "Deployment Runbook" || testCase.ExpectedResult.Score != 0.91 {
		t.Fatalf("expected result was not decoded, got %+v", testCase.ExpectedResult)
	}

	queryer.mu.Lock()
	query := queryer.query
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()
	for _, want := range []string{
		"FROM knowledge_retrieval_test_cases",
		"organization_id = $1",
		"knowledge_base_id = $2",
		"ORDER BY created_at DESC",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected list retrieval test case query to include %q, got %s", want, query)
		}
	}
	if got := knowledgeRetrievalArgString(args, 1); got != "org_1" {
		t.Fatalf("organization arg = %q, want org_1", got)
	}
	if got := knowledgeRetrievalArgString(args, 2); got != "kb_1" {
		t.Fatalf("knowledge base arg = %q, want kb_1", got)
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
	mu             sync.Mutex
	query          string
	args           []driver.NamedValue
	rows           [][]driver.Value
	rowsByQuery    map[string][][]driver.Value
	columnsByQuery map[string][]string
	execQueries    []string
	execArgs       [][]driver.NamedValue
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
	columns := knowledgeRetrievalResultColumns()
	if len(c.queryer.rowsByQuery) > 0 {
		for pattern, configuredRows := range c.queryer.rowsByQuery {
			if strings.Contains(query, pattern) {
				rows = append([][]driver.Value(nil), configuredRows...)
				if configuredColumns, ok := c.queryer.columnsByQuery[pattern]; ok {
					columns = append([]string(nil), configuredColumns...)
				} else if len(configuredRows) > 0 {
					columns = generatedKnowledgeTestColumns(len(configuredRows[0]))
				}
				break
			}
		}
	}
	c.queryer.mu.Unlock()
	return &knowledgeRetrievalRows{
		columns: columns,
		rows:    rows,
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

func knowledgeRetrievalResultColumns() []string {
	return []string{
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
	}
}

func generatedKnowledgeTestColumns(count int) []string {
	columns := make([]string, count)
	for index := range columns {
		columns[index] = "col_" + strconv.Itoa(index+1)
	}
	return columns
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
