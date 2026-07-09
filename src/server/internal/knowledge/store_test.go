package knowledge

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"io"
	"os"
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

func TestSQLStoreRetrieveKnowledgeWithOptionsPopulatesCitationHighlights(t *testing.T) {
	driverName := "knowledge_retrieve_citation_highlight_test"
	queryer := &knowledgeRetrievalQueryer{
		rows: [][]driver.Value{
			{
				"doc_1",
				"Deployment Manual",
				"v3",
				"kdc_1",
				int64(0),
				"v3",
				"Deployment controls require approval before production rollout.",
				[]byte(`{"pageNumber":15,"sourceUrl":"https://docs.example/manual.pdf"}`),
				float64(0.91),
				float64(0.91),
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
		"deployment controls",
		[]float32{0.1, 0.2, 0.3},
		KnowledgeRetrievalOptions{Mode: KnowledgeRetrievalModeVector, Limit: 1},
	)
	if err != nil {
		t.Fatalf("retrieve knowledge with citation highlights: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	source := results[0].Source
	if source.MatchedSnippet == "" || source.OriginalText == "" || source.PageNumber != 15 || source.SourceURL != "https://docs.example/manual.pdf" {
		t.Fatalf("expected citation source metadata to be populated, got %+v", source)
	}
	if len(source.HighlightPositions) != 1 {
		t.Fatalf("expected exact query highlight position, got %+v", source.HighlightPositions)
	}
	if got := source.HighlightPositions[0]; got.Start != 0 || got.End != 19 {
		t.Fatalf("expected highlight 0-19, got %+v", got)
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
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()

	if strings.Contains(query, "<=>") {
		t.Fatalf("expected keyword fallback query not to use pgvector distance, got %s", query)
	}
	if !strings.Contains(query, "websearch_to_tsquery") || !strings.Contains(query, "organization_id = $1") {
		t.Fatalf("expected keyword fallback query to use tenant-scoped full text search, got %s", query)
	}
	if got := knowledgeRetrievalArgString(args, 6); got != KnowledgeRetrievalMethodKeyword {
		t.Fatalf("expected keyword retrieval method arg, got %q", got)
	}
}

func TestSQLStoreFilterReadyKnowledgeRetrievalResultsKeepsOnlyLiveReadyChunks(t *testing.T) {
	driverName := "knowledge_filter_ready_retrieval_results_test"
	queryer := &knowledgeRetrievalQueryer{
		rowsByQuery: map[string][][]driver.Value{
			"WITH candidate": {
				{"doc_ready", "chunk_ready"},
			},
		},
	}
	registerKnowledgeRetrievalDriver(driverName, queryer)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	results, err := NewSQLStore(db).FilterReadyKnowledgeRetrievalResults(context.Background(), " org_1 ", " kb_1 ", []KnowledgeRetrievalResult{
		{DocumentID: "doc_deleted", ChunkID: "chunk_deleted", Snippet: "deleted stale vector"},
		{DocumentID: "doc_pending", ChunkID: "chunk_pending", Snippet: "pending stale vector"},
		{DocumentID: "doc_ready", ChunkID: "chunk_ready", Snippet: "current vector"},
	})
	if err != nil {
		t.Fatalf("filter ready retrieval results: %v", err)
	}
	if len(results) != 1 || results[0].DocumentID != "doc_ready" || results[0].ChunkID != "chunk_ready" {
		t.Fatalf("expected only live ready chunk, got %+v", results)
	}

	queryer.mu.Lock()
	query := queryer.query
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()
	for _, want := range []string{
		"WITH candidate",
		"JOIN knowledge_documents d",
		"JOIN knowledge_document_chunks kdc",
		"JOIN knowledge_bases kb",
		"COALESCE(d.index_status, '') = $3",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected ready filter query to include %q, got %s", want, query)
		}
	}
	if got := knowledgeRetrievalArgString(args, 1); got != "org_1" {
		t.Fatalf("organization arg = %q, want org_1", got)
	}
	if got := knowledgeRetrievalArgString(args, 2); got != "kb_1" {
		t.Fatalf("knowledge base arg = %q, want kb_1", got)
	}
	if got := knowledgeRetrievalArgString(args, 3); got != KnowledgeDocumentIndexStatusReady {
		t.Fatalf("index status arg = %q, want ready", got)
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

func TestSQLStoreCreateKnowledgeDocumentWithOptionsCreatesTransactionalIndexOutbox(t *testing.T) {
	driverName := "knowledge_create_transactional_index_outbox_test"
	now := time.Date(2026, time.July, 2, 11, 0, 0, 0, time.UTC)
	queryer := &knowledgeRetrievalQueryer{
		rowsByQuery: map[string][][]driver.Value{
			"INSERT INTO knowledge_index_jobs": {
				{
					"kij_tx_create",
					"org_1",
					"kb_1",
					"doc_tx_create",
					KnowledgeIndexJobOperationUpsertDocument,
					KnowledgeIndexJobStatusPending,
					"",
					int64(0),
					int64(defaultKnowledgeIndexJobMaxAttempts),
					nil,
					"",
					now,
					nil,
					now,
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

	options := KnowledgeDocumentOptions{DocumentVersion: "v3", UpdateStrategy: KnowledgeUpdateStrategyFullReplace}
	options.createIndexJob = true
	document, err := NewSQLStore(db).CreateKnowledgeDocumentWithOptions(
		context.Background(),
		"org_1",
		"kb_1",
		"Transactional Outbox Runbook",
		"Transactional outbox content",
		[]KnowledgeDocumentChunk{{ChunkIndex: 0, Content: "Transactional outbox content", DocumentVersion: "v3"}},
		options,
	)
	if err != nil {
		t.Fatalf("create knowledge document with transactional outbox: %v", err)
	}
	if document.IndexStatus != KnowledgeDocumentIndexStatusPending {
		t.Fatalf("expected pending document index status, got %+v", document)
	}

	documentQuery, documentArgs := knowledgeRetrievalExecForQuery(t, queryer, "INSERT INTO knowledge_documents")
	for _, want := range []string{
		"index_status",
		"index_error",
		"indexed_at",
		"SELECT $1, $2, kb.id",
	} {
		if !strings.Contains(documentQuery, want) {
			t.Fatalf("expected document insert to include %q, got %s", want, documentQuery)
		}
	}
	if got := knowledgeRetrievalArgString(documentArgs, 8); got != KnowledgeDocumentIndexStatusPending {
		t.Fatalf("document index status arg = %q, want pending", got)
	}

	queryer.mu.Lock()
	jobQuery := queryer.query
	jobArgs := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()
	for _, want := range []string{
		"INSERT INTO knowledge_index_jobs",
		"JOIN knowledge_bases kb",
		"WHERE d.organization_id = $2",
		"RETURNING id, organization_id, knowledge_base_id, document_id",
	} {
		if !strings.Contains(jobQuery, want) {
			t.Fatalf("expected transactional outbox job query to include %q, got %s", want, jobQuery)
		}
	}
	if got := knowledgeRetrievalArgString(jobArgs, 2); got != "org_1" {
		t.Fatalf("job organization arg = %q, want org_1", got)
	}
	if got := knowledgeRetrievalArgString(jobArgs, 3); got != "kb_1" {
		t.Fatalf("job knowledge base arg = %q, want kb_1", got)
	}
	if got := knowledgeRetrievalArgString(jobArgs, 4); got != document.ID {
		t.Fatalf("job document arg = %q, want returned document id %q", got, document.ID)
	}
	if got := knowledgeRetrievalArgString(jobArgs, 5); got != KnowledgeIndexJobOperationUpsertDocument {
		t.Fatalf("job operation arg = %q", got)
	}
	if got := knowledgeRetrievalArgString(jobArgs, 6); got != KnowledgeIndexJobStatusPending {
		t.Fatalf("job status arg = %q", got)
	}
	if got := knowledgeRetrievalArgInt(jobArgs, 7); got != defaultKnowledgeIndexJobMaxAttempts {
		t.Fatalf("job max attempts arg = %d", got)
	}
}

func TestSQLStoreUpdateKnowledgeDocumentVersionedReplacesOnlyCurrentVersionChunks(t *testing.T) {
	driverName := "knowledge_update_versioned_options_test"
	queryer := &knowledgeRetrievalQueryer{}
	registerKnowledgeRetrievalDriver(driverName, queryer)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	document, err := NewSQLStore(db).UpdateKnowledgeDocumentWithOptions(
		context.Background(),
		"org_1",
		"kb_1",
		"doc_1",
		"Deployment Runbook",
		"Deployment rollback v2 content",
		[]KnowledgeDocumentChunk{
			{
				ChunkIndex:      0,
				Content:         "Deployment rollback v2 content",
				DocumentVersion: "v2",
				Metadata:        KnowledgeChunkMetadata{DocumentVersion: "v2"},
			},
		},
		KnowledgeDocumentOptions{DocumentVersion: "v2", UpdateStrategy: KnowledgeUpdateStrategyVersioned},
	)
	if err != nil {
		t.Fatalf("update knowledge document with versioned options: %v", err)
	}
	if document.DocumentVersion != "v2" || document.UpdateStrategy != KnowledgeUpdateStrategyVersioned {
		t.Fatalf("expected returned document to include versioned metadata, got %+v", document)
	}

	deleteQuery, deleteArgs := knowledgeRetrievalExecForQuery(t, queryer, "DELETE FROM knowledge_document_chunks")
	if !strings.Contains(deleteQuery, "document_version") {
		t.Fatalf("expected versioned update to delete chunks only for the current document version, got %s", deleteQuery)
	}
	if got := knowledgeRetrievalArgString(deleteArgs, 1); got != "doc_1" {
		t.Fatalf("delete document arg = %q, want doc_1", got)
	}
	if got := knowledgeRetrievalArgString(deleteArgs, 2); got != "v2" {
		t.Fatalf("delete version arg = %q, want v2", got)
	}
}

func TestSQLStoreUpdateKnowledgeDocumentWithOptionsCreatesTransactionalIndexOutbox(t *testing.T) {
	driverName := "knowledge_update_transactional_index_outbox_test"
	now := time.Date(2026, time.July, 2, 11, 30, 0, 0, time.UTC)
	queryer := &knowledgeRetrievalQueryer{
		rowsByQuery: map[string][][]driver.Value{
			"INSERT INTO knowledge_index_jobs": {
				{
					"kij_tx_update",
					"org_1",
					"kb_1",
					"doc_1",
					KnowledgeIndexJobOperationUpsertDocument,
					KnowledgeIndexJobStatusPending,
					"",
					int64(0),
					int64(defaultKnowledgeIndexJobMaxAttempts),
					nil,
					"",
					now,
					nil,
					now,
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

	options := KnowledgeDocumentOptions{DocumentVersion: "v4", UpdateStrategy: KnowledgeUpdateStrategyFullReplace}
	options.createIndexJob = true
	document, err := NewSQLStore(db).UpdateKnowledgeDocumentWithOptions(
		context.Background(),
		"org_1",
		"kb_1",
		"doc_1",
		"Transactional Update Runbook",
		"Updated transactional outbox content",
		[]KnowledgeDocumentChunk{{ChunkIndex: 0, Content: "Updated transactional outbox content", DocumentVersion: "v4"}},
		options,
	)
	if err != nil {
		t.Fatalf("update knowledge document with transactional outbox: %v", err)
	}
	if document.IndexStatus != KnowledgeDocumentIndexStatusPending {
		t.Fatalf("expected pending document index status, got %+v", document)
	}

	updateQuery, updateArgs := knowledgeRetrievalExecForQuery(t, queryer, "UPDATE knowledge_documents")
	for _, want := range []string{
		"index_status = $8",
		"index_error = ''",
		"indexed_at = NULL",
	} {
		if !strings.Contains(updateQuery, want) {
			t.Fatalf("expected document update to include %q, got %s", want, updateQuery)
		}
	}
	if got := knowledgeRetrievalArgString(updateArgs, 8); got != KnowledgeDocumentIndexStatusPending {
		t.Fatalf("document index status arg = %q, want pending", got)
	}

	queryer.mu.Lock()
	jobQuery := queryer.query
	jobArgs := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()
	if !strings.Contains(jobQuery, "INSERT INTO knowledge_index_jobs") {
		t.Fatalf("expected transactional outbox job insert, got %s", jobQuery)
	}
	if got := knowledgeRetrievalArgString(jobArgs, 4); got != "doc_1" {
		t.Fatalf("job document arg = %q, want doc_1", got)
	}
	if got := knowledgeRetrievalArgString(jobArgs, 6); got != KnowledgeIndexJobStatusPending {
		t.Fatalf("job status arg = %q", got)
	}
}

func TestSQLStoreDeleteKnowledgeDocumentWithOptionsCreatesTransactionalDeleteIndexOutbox(t *testing.T) {
	driverName := "knowledge_delete_transactional_index_outbox_test"
	now := time.Date(2026, time.July, 2, 12, 0, 0, 0, time.UTC)
	queryer := &knowledgeRetrievalQueryer{
		rowsByQuery: map[string][][]driver.Value{
			"INSERT INTO knowledge_index_jobs": {
				{
					"kij_tx_delete",
					"org_1",
					"kb_1",
					"doc_1",
					KnowledgeIndexJobOperationDeleteDocument,
					KnowledgeIndexJobStatusPending,
					"",
					int64(0),
					int64(defaultKnowledgeIndexJobMaxAttempts),
					nil,
					"",
					now,
					nil,
					now,
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

	options := KnowledgeDocumentOptions{}
	options.createIndexJob = true
	if err := NewSQLStore(db).DeleteKnowledgeDocumentWithOptions(context.Background(), "org_1", "kb_1", "doc_1", options); err != nil {
		t.Fatalf("delete knowledge document with transactional outbox: %v", err)
	}

	queryer.mu.Lock()
	jobQuery := queryer.query
	jobArgs := append([]driver.NamedValue(nil), queryer.args...)
	calls := append([]string(nil), queryer.calls...)
	queryer.mu.Unlock()
	for _, want := range []string{
		"INSERT INTO knowledge_index_jobs",
		"JOIN knowledge_bases kb",
		"WHERE d.organization_id = $2",
		"RETURNING id, organization_id, knowledge_base_id, document_id",
	} {
		if !strings.Contains(jobQuery, want) {
			t.Fatalf("expected transactional delete outbox job query to include %q, got %s", want, jobQuery)
		}
	}
	if got := knowledgeRetrievalArgString(jobArgs, 2); got != "org_1" {
		t.Fatalf("job organization arg = %q, want org_1", got)
	}
	if got := knowledgeRetrievalArgString(jobArgs, 3); got != "kb_1" {
		t.Fatalf("job knowledge base arg = %q, want kb_1", got)
	}
	if got := knowledgeRetrievalArgString(jobArgs, 4); got != "doc_1" {
		t.Fatalf("job document arg = %q, want doc_1", got)
	}
	if got := knowledgeRetrievalArgString(jobArgs, 5); got != KnowledgeIndexJobOperationDeleteDocument {
		t.Fatalf("job operation arg = %q, want delete_document", got)
	}
	if got := knowledgeRetrievalArgString(jobArgs, 6); got != KnowledgeIndexJobStatusPending {
		t.Fatalf("job status arg = %q", got)
	}

	deleteQuery, deleteArgs := knowledgeRetrievalExecForQuery(t, queryer, "DELETE FROM knowledge_documents")
	if !strings.Contains(deleteQuery, "USING knowledge_bases kb") {
		t.Fatalf("expected tenant-scoped document delete, got %s", deleteQuery)
	}
	if got := knowledgeRetrievalArgString(deleteArgs, 1); got != "org_1" {
		t.Fatalf("delete scope arg = %q, want org_1", got)
	}
	if got := knowledgeRetrievalArgString(deleteArgs, 2); got != "kb_1" {
		t.Fatalf("delete knowledge base arg = %q, want kb_1", got)
	}
	if got := knowledgeRetrievalArgString(deleteArgs, 3); got != "doc_1" {
		t.Fatalf("delete document arg = %q, want doc_1", got)
	}
	if len(calls) < 2 || !strings.Contains(calls[0], "INSERT INTO knowledge_index_jobs") || !strings.Contains(calls[1], "DELETE FROM knowledge_documents") {
		t.Fatalf("expected delete outbox job to be inserted before deleting SQL document, calls=%v", calls)
	}
}

func TestSQLStoreUpdateKnowledgeDocumentIncrementalReplacesOnlyChangedChunkHashes(t *testing.T) {
	driverName := "knowledge_update_incremental_options_test"
	unchangedHash := knowledgeDocumentChunkContentHash("unchanged content")
	queryer := &knowledgeRetrievalQueryer{
		rowsByQuery: map[string][][]driver.Value{
			"FROM knowledge_document_chunks": {
				{"kdc_keep", int64(0), "unchanged content", []byte(`{"extra":{"contentHash":"` + unchangedHash + `"}}`)},
				{"kdc_replace", int64(1), "old content", []byte(`{"extra":{"contentHash":"old_hash"}}`)},
			},
		},
	}
	registerKnowledgeRetrievalDriver(driverName, queryer)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	document, err := NewSQLStore(db).UpdateKnowledgeDocumentWithOptions(
		context.Background(),
		"org_1",
		"kb_1",
		"doc_1",
		"Deployment Runbook",
		"unchanged content\nnew content",
		[]KnowledgeDocumentChunk{
			{ChunkIndex: 0, Content: "unchanged content", DocumentVersion: "v1"},
			{ChunkIndex: 1, Content: "new content", DocumentVersion: "v1"},
		},
		KnowledgeDocumentOptions{DocumentVersion: "v1", UpdateStrategy: KnowledgeUpdateStrategyIncremental},
	)
	if err != nil {
		t.Fatalf("update knowledge document with incremental options: %v", err)
	}
	if document.UpdateStrategy != KnowledgeUpdateStrategyIncremental {
		t.Fatalf("expected returned document to include incremental strategy, got %+v", document)
	}

	deleteQuery, deleteArgs := knowledgeRetrievalExecForQuery(t, queryer, "DELETE FROM knowledge_document_chunks")
	if strings.Contains(deleteQuery, "WHERE document_id = $1\n\t") && !strings.Contains(deleteQuery, "chunk_index") {
		t.Fatalf("expected incremental update not to delete every chunk for document, got %s", deleteQuery)
	}
	if !strings.Contains(deleteQuery, "chunk_index = ANY") {
		t.Fatalf("expected incremental delete to target changed chunk indexes, got %s", deleteQuery)
	}
	if got := knowledgeRetrievalArgString(deleteArgs, 1); got != "doc_1" {
		t.Fatalf("delete document arg = %q, want doc_1", got)
	}
	if got := knowledgeRetrievalArgString(deleteArgs, 2); got != "v1" {
		t.Fatalf("delete version arg = %q, want v1", got)
	}
	insertArgs := knowledgeRetrievalExecArgsForQuery(t, queryer, "INSERT INTO knowledge_document_chunks")
	if got := knowledgeRetrievalArgInt(insertArgs, 4); got != 1 {
		t.Fatalf("expected only changed chunk index 1 to be inserted, got chunk_index=%d", got)
	}
	if got := knowledgeRetrievalArgString(insertArgs, 10); !strings.Contains(got, `"contentHash":"`+knowledgeDocumentChunkContentHash("new content")+`"`) {
		t.Fatalf("expected inserted metadata to include new content hash, got %s", got)
	}
}

func TestSQLStoreDiffKnowledgeDocumentChunksReturnsOnlyChangedIncrementalChunks(t *testing.T) {
	driverName := "knowledge_diff_incremental_chunks_test"
	unchangedHash := knowledgeDocumentChunkContentHash("unchanged content")
	queryer := &knowledgeRetrievalQueryer{
		rowsByQuery: map[string][][]driver.Value{
			"FROM knowledge_document_chunks": {
				{"kdc_keep", int64(0), "unchanged content", []byte(`{"extra":{"contentHash":"` + unchangedHash + `"}}`)},
				{"kdc_replace", int64(1), "old content", []byte(`{"extra":{"contentHash":"old_hash"}}`)},
			},
		},
	}
	registerKnowledgeRetrievalDriver(driverName, queryer)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	changed, err := NewSQLStore(db).DiffKnowledgeDocumentChunks(
		context.Background(),
		"org_1",
		"kb_1",
		"doc_1",
		[]KnowledgeDocumentChunk{
			{ChunkIndex: 0, Content: "unchanged content", DocumentVersion: "v1"},
			{ChunkIndex: 1, Content: "new content", DocumentVersion: "v1"},
		},
		KnowledgeDocumentOptions{DocumentVersion: "v1", UpdateStrategy: KnowledgeUpdateStrategyIncremental},
	)
	if err != nil {
		t.Fatalf("diff incremental chunks: %v", err)
	}
	if len(changed) != 1 || changed[0].ChunkIndex != 1 || changed[0].Content != "new content" {
		t.Fatalf("expected only changed chunk index 1, got %+v", changed)
	}

	queryer.mu.Lock()
	query := queryer.query
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()
	for _, want := range []string{"JOIN knowledge_documents", "JOIN knowledge_bases", "c.organization_id = $2", "kb.organization_id = $2", "kb.id = $3", "c.document_version = $4"} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected diff query to include %q, got %s", want, query)
		}
	}
	if got := knowledgeRetrievalArgString(args, 1); got != "doc_1" {
		t.Fatalf("document arg = %q, want doc_1", got)
	}
	if got := knowledgeRetrievalArgString(args, 2); got != "org_1" {
		t.Fatalf("organization arg = %q, want org_1", got)
	}
	if got := knowledgeRetrievalArgString(args, 3); got != "kb_1" {
		t.Fatalf("knowledge base arg = %q, want kb_1", got)
	}
	if got := knowledgeRetrievalArgString(args, 4); got != "v1" {
		t.Fatalf("document version arg = %q, want v1", got)
	}
}

func TestSQLStoreListKnowledgeDocumentChunksReturnsTenantScopedChunkViews(t *testing.T) {
	driverName := "knowledge_list_document_chunks_test"
	queryer := &knowledgeRetrievalQueryer{
		rowsByQuery: map[string][][]driver.Value{
			"FROM knowledge_document_chunks": {
				{
					"kdc_1",
					int64(2),
					"Deployment rollback chunk.",
					"Deployment Runbook",
					"v2",
					[]byte(`{"documentVersion":"v2","pageNumber":7,"sourceUrl":"https://docs.example/runbook.md"}`),
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

	chunks, err := NewSQLStore(db).ListKnowledgeDocumentChunks(context.Background(), "org_1", "kb_1", "doc_1")
	if err != nil {
		t.Fatalf("list document chunks: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected one chunk, got %+v", chunks)
	}
	if chunks[0].ChunkID != "kdc_1" || chunks[0].ChunkIndex != 2 || chunks[0].CharCount != 26 || chunks[0].EstimatedTokenCount == 0 {
		t.Fatalf("unexpected chunk view: %+v", chunks[0])
	}
	if chunks[0].DocumentTitle != "Deployment Runbook" {
		t.Fatalf("expected document title to be preserved, got %q", chunks[0].DocumentTitle)
	}
	if chunks[0].Metadata.PageNumber != 7 || chunks[0].Metadata.SourceURL != "https://docs.example/runbook.md" {
		t.Fatalf("expected metadata to be decoded, got %+v", chunks[0].Metadata)
	}

	queryer.mu.Lock()
	query := queryer.query
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()
	for _, want := range []string{"d.title", "JOIN knowledge_documents", "JOIN knowledge_bases", "c.organization_id = $1", "kb.organization_id = $1", "kb.id = $2", "d.id = $3", "ORDER BY c.chunk_index ASC"} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected list chunks query to include %q, got %s", want, query)
		}
	}
	if got := knowledgeRetrievalArgString(args, 1); got != "org_1" {
		t.Fatalf("organization arg = %q, want org_1", got)
	}
	if got := knowledgeRetrievalArgString(args, 2); got != "kb_1" {
		t.Fatalf("knowledge base arg = %q, want kb_1", got)
	}
	if got := knowledgeRetrievalArgString(args, 3); got != "doc_1" {
		t.Fatalf("document arg = %q, want doc_1", got)
	}
}

func TestSQLStoreUpdateKnowledgeDocumentChunkUpdatesContentHashAndReturnsChunk(t *testing.T) {
	driverName := "knowledge_update_document_chunk_test"
	queryer := &knowledgeRetrievalQueryer{
		rowsByQuery: map[string][][]driver.Value{
			"UPDATE knowledge_document_chunks": {
				{
					"kdc_1",
					int64(1),
					"Updated chunk content.",
					"Deployment Runbook",
					"v2",
					[]byte(`{"documentVersion":"v2","pageNumber":8}`),
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

	chunk, err := NewSQLStore(db).UpdateKnowledgeDocumentChunk(context.Background(), "org_1", "kb_1", "doc_1", "kdc_1", "Updated chunk content.")
	if err != nil {
		t.Fatalf("update document chunk: %v", err)
	}
	if chunk.ChunkID != "kdc_1" || chunk.Content != "Updated chunk content." || chunk.DocumentTitle != "Deployment Runbook" || chunk.DocumentVersion != "v2" {
		t.Fatalf("unexpected updated chunk: %+v", chunk)
	}

	queryer.mu.Lock()
	query := queryer.query
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()
	for _, want := range []string{"UPDATE knowledge_document_chunks", "FROM knowledge_documents", "JOIN knowledge_bases", "c.organization_id = $1", "kb.organization_id = $1", "c.id = $4", "metadata", "RETURNING", "d.title"} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected update chunk query to include %q, got %s", want, query)
		}
	}
	if got := knowledgeRetrievalArgString(args, 1); got != "org_1" {
		t.Fatalf("organization arg = %q, want org_1", got)
	}
	if got := knowledgeRetrievalArgString(args, 4); got != "kdc_1" {
		t.Fatalf("chunk arg = %q, want kdc_1", got)
	}
	if got := knowledgeRetrievalArgString(args, 5); got != "Updated chunk content." {
		t.Fatalf("content arg = %q, want updated content", got)
	}
	if got := knowledgeRetrievalArgString(args, 6); !strings.Contains(got, `"contentHash":"`+knowledgeDocumentChunkContentHash("Updated chunk content.")+`"`) {
		t.Fatalf("metadata arg = %q, want updated content hash", got)
	}
}

func TestSQLStoreSplitKnowledgeDocumentChunkReindexesFollowingChunks(t *testing.T) {
	driverName := "knowledge_split_document_chunk_test"
	queryer := &knowledgeRetrievalQueryer{
		rowsByQuery: map[string][][]driver.Value{
			"AND c.id = $4": {
				{
					"kdc_original",
					int64(1),
					"Alpha beta gamma delta",
					"v2",
					[]byte(`{"documentVersion":"v2","pageNumber":3,"startRune":100,"endRune":122}`),
				},
			},
			"ORDER BY c.chunk_index ASC": {
				{"kdc_left", int64(1), "Alpha beta", "Deployment Runbook", "v2", []byte(`{"documentVersion":"v2","pageNumber":3}`)},
				{"kdc_right", int64(2), "gamma delta", "Deployment Runbook", "v2", []byte(`{"documentVersion":"v2","pageNumber":3}`)},
			},
		},
	}
	registerKnowledgeRetrievalDriver(driverName, queryer)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	chunks, err := NewSQLStore(db).SplitKnowledgeDocumentChunk(context.Background(), "org_1", "kb_1", "doc_1", "kdc_original", 10)
	if err != nil {
		t.Fatalf("split document chunk: %v", err)
	}
	if len(chunks) != 2 || chunks[0].Content != "Alpha beta" || chunks[1].Content != "gamma delta" {
		t.Fatalf("expected split chunks returned, got %+v", chunks)
	}

	queryer.mu.Lock()
	execQueries := strings.Join(queryer.execQueries, "\n")
	queryer.mu.Unlock()
	for _, want := range []string{
		"UPDATE knowledge_document_chunks",
		"chunk_index = -chunk_index - 100000",
		"chunk_index = (-chunk_index - 100000) + 1",
		"INSERT INTO knowledge_document_chunks",
		"document_id = $",
	} {
		if !strings.Contains(execQueries, want) {
			t.Fatalf("expected split execs to include %q, got %s", want, execQueries)
		}
	}

	_, leftArgs := knowledgeRetrievalExecForQuery(t, queryer, "SET content = $5")
	leftMetadata := knowledgeRetrievalArgString(leftArgs, 6)
	if !strings.Contains(leftMetadata, `"startRune":100`) || !strings.Contains(leftMetadata, `"endRune":110`) {
		t.Fatalf("left split metadata = %s, want original range 100-110", leftMetadata)
	}
	rightArgs := knowledgeRetrievalExecArgsForQuery(t, queryer, "INSERT INTO knowledge_document_chunks")
	rightMetadata := knowledgeRetrievalArgString(rightArgs, 8)
	if !strings.Contains(rightMetadata, `"startRune":111`) || !strings.Contains(rightMetadata, `"endRune":122`) {
		t.Fatalf("right split metadata = %s, want trimmed original range 111-122", rightMetadata)
	}
}

func TestSQLStoreMergeKnowledgeDocumentChunksCombinesNeighborAndReindexes(t *testing.T) {
	driverName := "knowledge_merge_document_chunks_test"
	queryer := &knowledgeRetrievalQueryer{
		rowsByQuery: map[string][][]driver.Value{
			"AND c.id = $4": {
				{"kdc_current", int64(1), "Alpha beta", "v2", []byte(`{"documentVersion":"v2","pageNumber":3,"startRune":100,"endRune":110}`)},
			},
			"AND c.chunk_index = $4": {
				{"kdc_next", int64(2), "gamma delta", "v2", []byte(`{"documentVersion":"v2","pageNumber":3,"startRune":111,"endRune":122}`)},
			},
			"ORDER BY c.chunk_index ASC": {
				{"kdc_current", int64(1), "Alpha beta\n\ngamma delta", "Deployment Runbook", "v2", []byte(`{"documentVersion":"v2","pageNumber":3}`)},
			},
		},
	}
	registerKnowledgeRetrievalDriver(driverName, queryer)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	chunks, err := NewSQLStore(db).MergeKnowledgeDocumentChunks(context.Background(), "org_1", "kb_1", "doc_1", "kdc_current", "next")
	if err != nil {
		t.Fatalf("merge document chunks: %v", err)
	}
	if len(chunks) != 1 || chunks[0].Content != "Alpha beta\n\ngamma delta" {
		t.Fatalf("expected merged chunk returned, got %+v", chunks)
	}

	queryer.mu.Lock()
	execQueries := strings.Join(queryer.execQueries, "\n")
	queryer.mu.Unlock()
	for _, want := range []string{
		"UPDATE knowledge_document_chunks",
		"DELETE FROM knowledge_document_chunks",
		"chunk_index = chunk_index - 1",
	} {
		if !strings.Contains(execQueries, want) {
			t.Fatalf("expected merge execs to include %q, got %s", want, execQueries)
		}
	}

	_, updateArgs := knowledgeRetrievalExecForQuery(t, queryer, "SET content = $5")
	mergedMetadata := knowledgeRetrievalArgString(updateArgs, 6)
	if !strings.Contains(mergedMetadata, `"startRune":100`) || !strings.Contains(mergedMetadata, `"endRune":122`) {
		t.Fatalf("merged metadata = %s, want range 100-122", mergedMetadata)
	}
}

func TestSQLStoreListKnowledgeDocumentVersionsReturnsTenantScopedHistory(t *testing.T) {
	driverName := "knowledge_document_versions_list_test"
	updatedAt := time.Date(2026, time.June, 7, 10, 30, 0, 0, time.UTC)
	queryer := &knowledgeRetrievalQueryer{
		columnsByQuery: map[string][]string{
			"FROM knowledge_document_versions": {"document_version", "title", "content", "update_strategy", "chunk_count", "updated_at"},
		},
		rowsByQuery: map[string][][]driver.Value{
			"FROM knowledge_document_versions": {
				{"v3", "Runbook", "Current version content.", "versioned", int64(2), updatedAt},
				{"v2", "Runbook", "Previous version content.", "versioned", int64(1), updatedAt.Add(-time.Hour)},
			},
		},
		rows: [][]driver.Value{
			{"v3", "Runbook", "Current version content.", "versioned", int64(2), updatedAt},
			{"v2", "Runbook", "Previous version content.", "versioned", int64(1), updatedAt.Add(-time.Hour)},
		},
	}
	registerKnowledgeRetrievalDriver(driverName, queryer)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	versions, err := NewSQLStore(db).ListKnowledgeDocumentVersions(context.Background(), "org_1", "kb_1", "doc_1")
	if err != nil {
		t.Fatalf("list knowledge document versions: %v", err)
	}

	queryer.mu.Lock()
	query := queryer.query
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()

	for _, want := range []string{
		"knowledge_document_versions",
		"knowledge_documents",
		"knowledge_bases",
		"organization_id = $1",
		"kb.id = $2",
		"d.id = $3",
		"chunk_count",
		"ORDER BY v.updated_at DESC",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected document versions query to include %q, got %s", want, query)
		}
	}
	if got := knowledgeRetrievalArgString(args, 1); got != "org_1" {
		t.Fatalf("organization arg = %q, want org_1", got)
	}
	if got := knowledgeRetrievalArgString(args, 2); got != "kb_1" {
		t.Fatalf("knowledge base arg = %q, want kb_1", got)
	}
	if got := knowledgeRetrievalArgString(args, 3); got != "doc_1" {
		t.Fatalf("document arg = %q, want doc_1", got)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	if versions[0].DocumentVersion != "v3" || versions[0].ChunkCount != 2 || versions[0].Content != "Current version content." {
		t.Fatalf("unexpected current version: %+v", versions[0])
	}
	if versions[1].DocumentVersion != "v2" || versions[1].ChunkCount != 1 || versions[1].Content != "Previous version content." {
		t.Fatalf("unexpected previous version: %+v", versions[1])
	}
}

func TestSQLStoreCreateKnowledgeIngestionJobPersistsDurablePayload(t *testing.T) {
	driverName := "knowledge_ingestion_job_create_test"
	now := time.Date(2026, time.July, 2, 13, 30, 0, 0, time.UTC)
	queryer := &knowledgeRetrievalQueryer{
		rowsByQuery: map[string][][]driver.Value{
			"INSERT INTO knowledge_ingestion_jobs": {
				{
					"kig_1",
					"org_1",
					"kb_1",
					"",
					"Async Runbook",
					"Durable upload content",
					[]byte("# Async Runbook\n\nDurable upload content"),
					"runbook.md",
					"text/markdown",
					int64(len("# Async Runbook\n\nDurable upload content")),
					"v2",
					KnowledgeUpdateStrategyVersioned,
					"https://docs.example/upload.pdf",
					int64(7),
					KnowledgeIngestionJobStatusPending,
					"",
					int64(0),
					int64(defaultKnowledgeIngestionJobMaxAttempts),
					nil,
					"",
					now,
					nil,
					now,
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

	job, err := NewSQLStore(db).CreateKnowledgeIngestionJob(context.Background(), CreateKnowledgeIngestionJobRequest{
		OrganizationID:  "org_1",
		KnowledgeBaseID: "kb_1",
		Title:           " Async Runbook ",
		Content:         " Durable upload content ",
		RawContent:      []byte("# Async Runbook\n\nDurable upload content"),
		RawFilename:     " runbook.md ",
		RawContentType:  " text/markdown ",
		Options: KnowledgeDocumentOptions{
			DocumentVersion: "v2",
			PageNumber:      7,
			SourceURL:       " https://docs.example/upload.pdf ",
			UpdateStrategy:  KnowledgeUpdateStrategyVersioned,
		},
	})
	if err != nil {
		t.Fatalf("create knowledge ingestion job: %v", err)
	}
	if job.ID != "kig_1" || job.Status != KnowledgeIngestionJobStatusPending || job.Options.DocumentVersion != "v2" || job.Options.PageNumber != 7 {
		t.Fatalf("unexpected ingestion job: %+v", job)
	}
	if string(job.RawContent) != "# Async Runbook\n\nDurable upload content" || job.RawFilename != "runbook.md" || job.RawContentType != "text/markdown" || job.RawSizeBytes != int64(len("# Async Runbook\n\nDurable upload content")) {
		t.Fatalf("unexpected raw ingestion payload: raw=%q filename=%q contentType=%q size=%d", string(job.RawContent), job.RawFilename, job.RawContentType, job.RawSizeBytes)
	}

	queryer.mu.Lock()
	query := queryer.query
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()
	for _, want := range []string{
		"INSERT INTO knowledge_ingestion_jobs",
		"raw_content",
		"raw_filename",
		"raw_content_type",
		"raw_size_bytes",
		"FROM knowledge_bases kb",
		"WHERE kb.organization_id = $2",
		"RETURNING id, organization_id, knowledge_base_id",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected ingestion job insert query to include %q, got %s", want, query)
		}
	}
	if got := knowledgeRetrievalArgString(args, 2); got != "org_1" {
		t.Fatalf("organization arg = %q, want org_1", got)
	}
	if got := knowledgeRetrievalArgString(args, 3); got != "kb_1" {
		t.Fatalf("knowledge base arg = %q, want kb_1", got)
	}
	if got := knowledgeRetrievalArgString(args, 4); got != "Async Runbook" {
		t.Fatalf("title arg = %q, want Async Runbook", got)
	}
	if got := knowledgeRetrievalArgString(args, 5); got != "Durable upload content" {
		t.Fatalf("content arg = %q, want Durable upload content", got)
	}
	if got := knowledgeRetrievalArgBytes(args, 6); string(got) != "# Async Runbook\n\nDurable upload content" {
		t.Fatalf("raw content arg = %q", string(got))
	}
	if got := knowledgeRetrievalArgString(args, 7); got != "runbook.md" {
		t.Fatalf("raw filename arg = %q", got)
	}
	if got := knowledgeRetrievalArgString(args, 8); got != "text/markdown" {
		t.Fatalf("raw content type arg = %q", got)
	}
	if got := knowledgeRetrievalArgInt(args, 9); got != len("# Async Runbook\n\nDurable upload content") {
		t.Fatalf("raw size arg = %d", got)
	}
	if got := knowledgeRetrievalArgString(args, 10); got != "v2" {
		t.Fatalf("document version arg = %q, want v2", got)
	}
	if got := knowledgeRetrievalArgString(args, 11); got != KnowledgeUpdateStrategyVersioned {
		t.Fatalf("update strategy arg = %q, want versioned", got)
	}
	if got := knowledgeRetrievalArgString(args, 12); got != "https://docs.example/upload.pdf" {
		t.Fatalf("source URL arg = %q", got)
	}
	if got := knowledgeRetrievalArgInt(args, 13); got != 7 {
		t.Fatalf("page number arg = %d, want 7", got)
	}
	if got := knowledgeRetrievalArgString(args, 14); got != KnowledgeIngestionJobStatusPending {
		t.Fatalf("status arg = %q", got)
	}
}

func TestSQLStoreClaimKnowledgeIngestionJobsRecoversExpiredLeasesWithOwnerAndMaxAttempts(t *testing.T) {
	driverName := "knowledge_ingestion_jobs_claim_test"
	now := time.Date(2026, time.July, 2, 14, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(defaultKnowledgeIngestionJobClaimLease)
	queryer := &knowledgeRetrievalQueryer{
		rowsByQuery: map[string][][]driver.Value{
			"FROM updated": {
				{
					"kig_1",
					"org_1",
					"kb_1",
					"",
					"Async Runbook",
					"Durable upload content",
					[]byte("raw durable upload content"),
					"runbook.md",
					"text/markdown",
					int64(len("raw durable upload content")),
					"v3",
					KnowledgeUpdateStrategyFullReplace,
					"https://docs.example/upload.md",
					int64(4),
					KnowledgeIngestionJobStatusProcessing,
					"previous failure",
					int64(2),
					int64(5),
					now,
					"worker_ingest_1",
					leaseUntil,
					nil,
					now.Add(-time.Hour),
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

	jobs, err := NewSQLStore(db).ClaimKnowledgeIngestionJobs(context.Background(), now, 2, "worker_ingest_1")
	if err != nil {
		t.Fatalf("claim knowledge ingestion jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one claimed ingestion job, got %+v", jobs)
	}
	job := jobs[0]
	if job.ID != "kig_1" || job.Status != KnowledgeIngestionJobStatusProcessing || job.Attempts != 2 || job.MaxAttempts != 5 {
		t.Fatalf("unexpected claimed ingestion job: %+v", job)
	}
	if string(job.RawContent) != "raw durable upload content" || job.RawFilename != "runbook.md" || job.RawContentType != "text/markdown" || job.RawSizeBytes != int64(len("raw durable upload content")) {
		t.Fatalf("expected claimed raw ingestion payload to round trip, got raw=%q filename=%q contentType=%q size=%d", string(job.RawContent), job.RawFilename, job.RawContentType, job.RawSizeBytes)
	}
	if job.LockedBy != "worker_ingest_1" || job.LockedAt == nil || !job.LockedAt.Equal(now) || !job.AvailableAt.Equal(leaseUntil) {
		t.Fatalf("expected claimed ingestion lease owner and deadline, got %+v", job)
	}
	if job.Options.DocumentVersion != "v3" || job.Options.PageNumber != 4 || job.Options.SourceURL != "https://docs.example/upload.md" {
		t.Fatalf("expected ingestion options to round trip, got %+v", job.Options)
	}

	queryer.mu.Lock()
	query := queryer.query
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()
	for _, want := range []string{
		"FROM knowledge_ingestion_jobs",
		"status IN ($2, $3)",
		"status = $4",
		"attempts <= max_attempts",
		"FOR UPDATE SKIP LOCKED",
		"locked_at = $1",
		"locked_by = $6",
		"available_at = $7",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected ingestion claim query to include %q, got %s", want, query)
		}
	}
	if got := knowledgeRetrievalArgString(args, 2); got != KnowledgeIngestionJobStatusPending {
		t.Fatalf("pending status arg = %q", got)
	}
	if got := knowledgeRetrievalArgString(args, 3); got != KnowledgeIngestionJobStatusFailed {
		t.Fatalf("failed status arg = %q", got)
	}
	if got := knowledgeRetrievalArgString(args, 4); got != KnowledgeIngestionJobStatusProcessing {
		t.Fatalf("processing status arg = %q", got)
	}
	if got := knowledgeRetrievalArgInt(args, 5); got != 2 {
		t.Fatalf("limit arg = %d, want 2", got)
	}
	if got := knowledgeRetrievalArgString(args, 6); got != "worker_ingest_1" {
		t.Fatalf("worker owner arg = %q", got)
	}
}

func TestSQLStoreClaimKnowledgeIngestionJobsRecoversExhaustedExpiredLeaseForDeadLetter(t *testing.T) {
	driverName := "knowledge_ingestion_jobs_exhausted_claim_test"
	now := time.Date(2026, time.July, 2, 15, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(defaultKnowledgeIngestionJobClaimLease)
	queryer := &knowledgeRetrievalQueryer{
		rowsByQuery: map[string][][]driver.Value{
			"FROM updated": {
				{
					"kig_exhausted",
					"org_1",
					"kb_1",
					"",
					"Broken Upload",
					"",
					[]byte("malformed raw payload"),
					"broken.pdf",
					"application/pdf",
					int64(len("malformed raw payload")),
					"v1",
					KnowledgeUpdateStrategyFullReplace,
					"",
					int64(0),
					KnowledgeIngestionJobStatusProcessing,
					"parser failed before process restart",
					int64(5),
					int64(5),
					now,
					"worker_ingest_recovery",
					leaseUntil,
					nil,
					now.Add(-time.Hour),
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

	jobs, err := NewSQLStore(db).ClaimKnowledgeIngestionJobs(context.Background(), now, 1, "worker_ingest_recovery")
	if err != nil {
		t.Fatalf("claim exhausted knowledge ingestion job: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one exhausted ingestion job to be recovered, got %+v", jobs)
	}
	job := jobs[0]
	if job.ID != "kig_exhausted" || job.Attempts != 5 || job.MaxAttempts != 5 || job.Status != KnowledgeIngestionJobStatusProcessing {
		t.Fatalf("unexpected exhausted ingestion job recovery: %+v", job)
	}

	queryer.mu.Lock()
	query := queryer.query
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()
	for _, want := range []string{
		"status = $4",
		"available_at <= $1",
		"attempts <= max_attempts",
		"FOR UPDATE SKIP LOCKED",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected exhausted ingestion claim query to include %q, got %s", want, query)
		}
	}
	if got := knowledgeRetrievalArgString(args, 4); got != KnowledgeIngestionJobStatusProcessing {
		t.Fatalf("processing status arg = %q", got)
	}
}

func TestSQLStoreListKnowledgeIngestionJobsScopesByOrganizationAndKnowledgeBase(t *testing.T) {
	driverName := "knowledge_ingestion_jobs_list_test"
	completedAt := time.Date(2026, time.July, 3, 11, 5, 0, 0, time.UTC)
	queryer := &knowledgeRetrievalQueryer{
		rowsByQuery: map[string][][]driver.Value{
			"FROM knowledge_ingestion_jobs": {
				{
					"kig_ready",
					"org_1",
					"kb_1",
					"doc_ready",
					"Async Runbook",
					"Durable upload content",
					[]byte("raw listed upload content"),
					"upload.md",
					"text/markdown",
					int64(len("raw listed upload content")),
					"v4",
					KnowledgeUpdateStrategyFullReplace,
					"https://docs.example/upload.md",
					int64(9),
					KnowledgeIngestionJobStatusSucceeded,
					"",
					int64(2),
					int64(5),
					nil,
					"",
					completedAt,
					completedAt,
					completedAt.Add(-10 * time.Minute),
					completedAt,
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

	jobs, err := NewSQLStore(db).ListKnowledgeIngestionJobs(context.Background(), " org_1 ", " kb_1 ")
	if err != nil {
		t.Fatalf("list knowledge ingestion jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one ingestion job, got %+v", jobs)
	}
	job := jobs[0]
	if job.ID != "kig_ready" || job.DocumentID != "doc_ready" || job.Status != KnowledgeIngestionJobStatusSucceeded {
		t.Fatalf("unexpected listed ingestion job: %+v", job)
	}
	if string(job.RawContent) != "raw listed upload content" || job.RawFilename != "upload.md" || job.RawContentType != "text/markdown" || job.RawSizeBytes != int64(len("raw listed upload content")) {
		t.Fatalf("expected listed raw ingestion payload to round trip, got raw=%q filename=%q contentType=%q size=%d", string(job.RawContent), job.RawFilename, job.RawContentType, job.RawSizeBytes)
	}
	if job.Options.DocumentVersion != "v4" || job.Options.PageNumber != 9 || job.Options.SourceURL != "https://docs.example/upload.md" {
		t.Fatalf("expected listed ingestion options to round trip, got %+v", job.Options)
	}

	queryer.mu.Lock()
	query := queryer.query
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()
	for _, want := range []string{
		"FROM knowledge_ingestion_jobs",
		"WHERE organization_id = $1",
		"AND knowledge_base_id = $2",
		"ORDER BY updated_at DESC, created_at DESC, id DESC",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected ingestion list query to include %q, got %s", want, query)
		}
	}
	if got := knowledgeRetrievalArgString(args, 1); got != "org_1" {
		t.Fatalf("organization arg = %q, want org_1", got)
	}
	if got := knowledgeRetrievalArgString(args, 2); got != "kb_1" {
		t.Fatalf("knowledge base arg = %q, want kb_1", got)
	}
}

func TestSQLStoreClaimKnowledgeIndexJobsRecoversExpiredLeasesWithOwnerAndMaxAttempts(t *testing.T) {
	driverName := "knowledge_index_jobs_claim_test"
	now := time.Date(2026, time.July, 2, 9, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(defaultKnowledgeIndexJobClaimLease)
	queryer := &knowledgeRetrievalQueryer{
		rowsByQuery: map[string][][]driver.Value{
			"FROM updated": {
				{
					"kij_1",
					"org_1",
					"kb_1",
					"doc_1",
					KnowledgeIndexJobOperationUpsertDocument,
					KnowledgeIndexJobStatusProcessing,
					"previous failure",
					int64(2),
					int64(5),
					now,
					"worker_rag_1",
					leaseUntil,
					nil,
					now.Add(-time.Hour),
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

	jobs, err := NewSQLStore(db).ClaimKnowledgeIndexJobs(context.Background(), now, 2, "worker_rag_1")
	if err != nil {
		t.Fatalf("claim knowledge index jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one claimed job, got %+v", jobs)
	}
	job := jobs[0]
	if job.ID != "kij_1" || job.Status != KnowledgeIndexJobStatusProcessing || job.Attempts != 2 || job.MaxAttempts != 5 {
		t.Fatalf("unexpected claimed job: %+v", job)
	}
	if job.LockedBy != "worker_rag_1" || job.LockedAt == nil || !job.LockedAt.Equal(now) || !job.AvailableAt.Equal(leaseUntil) {
		t.Fatalf("expected claimed lease owner and deadline, got %+v", job)
	}

	queryer.mu.Lock()
	query := queryer.query
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()
	for _, want := range []string{
		"status IN ($2, $3)",
		"status = $4",
		"attempts <= max_attempts",
		"FOR UPDATE SKIP LOCKED",
		"locked_at = $1",
		"locked_by = $6",
		"available_at = $7",
		"completed_at = NULL",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected claim query to include %q, got %s", want, query)
		}
	}
	if got := knowledgeRetrievalArgString(args, 2); got != KnowledgeIndexJobStatusPending {
		t.Fatalf("pending status arg = %q", got)
	}
	if got := knowledgeRetrievalArgString(args, 3); got != KnowledgeIndexJobStatusFailed {
		t.Fatalf("failed status arg = %q", got)
	}
	if got := knowledgeRetrievalArgString(args, 4); got != KnowledgeIndexJobStatusProcessing {
		t.Fatalf("processing status arg = %q", got)
	}
	if got := knowledgeRetrievalArgInt(args, 5); got != 2 {
		t.Fatalf("limit arg = %d, want 2", got)
	}
	if got := knowledgeRetrievalArgString(args, 6); got != "worker_rag_1" {
		t.Fatalf("worker owner arg = %q", got)
	}
}

func TestSQLStoreClaimKnowledgeIndexJobsRecoversExhaustedExpiredLeaseForDeadLetter(t *testing.T) {
	driverName := "knowledge_index_jobs_exhausted_claim_test"
	now := time.Date(2026, time.July, 2, 9, 30, 0, 0, time.UTC)
	leaseUntil := now.Add(defaultKnowledgeIndexJobClaimLease)
	queryer := &knowledgeRetrievalQueryer{
		rowsByQuery: map[string][][]driver.Value{
			"FROM updated": {
				{
					"kij_exhausted",
					"org_1",
					"kb_1",
					"doc_1",
					KnowledgeIndexJobOperationUpsertDocument,
					KnowledgeIndexJobStatusProcessing,
					"qdrant timeout before process restart",
					int64(5),
					int64(5),
					now,
					"worker_rag_recovery",
					leaseUntil,
					nil,
					now.Add(-time.Hour),
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

	jobs, err := NewSQLStore(db).ClaimKnowledgeIndexJobs(context.Background(), now, 1, "worker_rag_recovery")
	if err != nil {
		t.Fatalf("claim exhausted knowledge index job: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one exhausted index job to be recovered, got %+v", jobs)
	}
	job := jobs[0]
	if job.ID != "kij_exhausted" || job.Attempts != 5 || job.MaxAttempts != 5 || job.Status != KnowledgeIndexJobStatusProcessing {
		t.Fatalf("unexpected exhausted index job recovery: %+v", job)
	}

	queryer.mu.Lock()
	query := queryer.query
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()
	for _, want := range []string{
		"status = $4",
		"available_at <= $1",
		"attempts <= max_attempts",
		"FOR UPDATE SKIP LOCKED",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected exhausted index claim query to include %q, got %s", want, query)
		}
	}
	if got := knowledgeRetrievalArgString(args, 4); got != KnowledgeIndexJobStatusProcessing {
		t.Fatalf("processing status arg = %q", got)
	}
}

func TestSQLStoreMarksKnowledgeIndexJobDeadLetterWithOwnerGuard(t *testing.T) {
	driverName := "knowledge_index_jobs_dead_letter_test"
	queryer := &knowledgeRetrievalQueryer{}
	registerKnowledgeRetrievalDriver(driverName, queryer)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	completedAt := time.Date(2026, time.July, 2, 10, 0, 0, 0, time.UTC)
	err = NewSQLStore(db).MarkKnowledgeIndexJobDeadLetter(context.Background(), "org_1", "kij_1", "worker_rag_1", "dead_letter: qdrant unavailable", completedAt)
	if err != nil {
		t.Fatalf("mark knowledge index job dead-letter: %v", err)
	}

	query, args := knowledgeRetrievalExecForQuery(t, queryer, "UPDATE knowledge_index_jobs")
	for _, want := range []string{
		"status = $4",
		"error = $5",
		"locked_at = NULL",
		"locked_by = ''",
		"available_at = $6",
		"completed_at = $6",
		"AND ($3 = '' OR locked_by = $3)",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected dead-letter query to include %q, got %s", want, query)
		}
	}
	if got := knowledgeRetrievalArgString(args, 3); got != "worker_rag_1" {
		t.Fatalf("worker owner arg = %q", got)
	}
	if got := knowledgeRetrievalArgString(args, 4); got != KnowledgeIndexJobStatusDeadLetter {
		t.Fatalf("status arg = %q", got)
	}
	if got := knowledgeRetrievalArgString(args, 5); got != "dead_letter: qdrant unavailable" {
		t.Fatalf("reason arg = %q", got)
	}
}

func TestSQLStoreMarksKnowledgeIndexJobSucceededPreservesCompletedByAndReleasesLease(t *testing.T) {
	driverName := "knowledge_index_jobs_succeeded_completed_by_test"
	queryer := &knowledgeRetrievalQueryer{}
	registerKnowledgeRetrievalDriver(driverName, queryer)

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open capture db: %v", err)
	}
	defer db.Close()

	completedAt := time.Date(2026, time.July, 2, 11, 0, 0, 0, time.UTC)
	err = NewSQLStore(db).MarkKnowledgeIndexJobSucceeded(context.Background(), "org_1", "kij_1", "worker_rag_1", completedAt)
	if err != nil {
		t.Fatalf("mark knowledge index job succeeded: %v", err)
	}

	query, args := knowledgeRetrievalExecForQuery(t, queryer, "UPDATE knowledge_index_jobs")
	for _, want := range []string{
		"status = $4",
		"error = ''",
		"locked_at = NULL",
		"completed_by = CASE",
		"ELSE $3",
		"locked_by = ''",
		"available_at = $5",
		"completed_at = $5",
		"AND ($3 = '' OR locked_by = $3)",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("expected succeeded query to include %q, got %s", want, query)
		}
	}
	if got := knowledgeRetrievalArgString(args, 3); got != "worker_rag_1" {
		t.Fatalf("worker owner arg = %q", got)
	}
	if got := knowledgeRetrievalArgString(args, 4); got != KnowledgeIndexJobStatusSucceeded {
		t.Fatalf("status arg = %q", got)
	}
}

func TestKnowledgeIndexJobsMigrationsDeclareDeadLetterLeaseAndAttemptFields(t *testing.T) {
	baselineRaw, err := os.ReadFile("../../migrations/0083_knowledge_index_jobs.sql")
	if err != nil {
		t.Fatalf("read knowledge index jobs baseline migration: %v", err)
	}
	upgradeRaw, err := os.ReadFile("../../migrations/0086_knowledge_index_jobs_dead_letter.sql")
	if err != nil {
		t.Fatalf("read knowledge index jobs dead-letter migration: %v", err)
	}
	completedByRaw, err := os.ReadFile("../../migrations/0100_knowledge_index_jobs_completed_by.sql")
	if err != nil {
		t.Fatalf("read knowledge index jobs completed-by migration: %v", err)
	}
	migration := string(baselineRaw) + "\n" + string(upgradeRaw) + "\n" + string(completedByRaw)

	for _, want := range []string{
		"max_attempts INTEGER NOT NULL DEFAULT 5",
		"locked_at TIMESTAMPTZ",
		"locked_by TEXT NOT NULL DEFAULT ''",
		"completed_by TEXT NOT NULL DEFAULT ''",
		"completed_at TIMESTAMPTZ",
		"'dead_letter'",
		"attempts >= max_attempts",
		"SET completed_by = locked_by",
	} {
		if !strings.Contains(migration, want) {
			t.Fatalf("expected knowledge index job migrations to contain %q, got:\n%s", want, migration)
		}
	}
}

func TestKnowledgeIndexJobsMigrationsDeclareDeleteDocumentJobsSurviveDeletedDocuments(t *testing.T) {
	baselineRaw, err := os.ReadFile("../../migrations/0083_knowledge_index_jobs.sql")
	if err != nil {
		t.Fatalf("read knowledge index jobs baseline migration: %v", err)
	}
	deleteOperationRaw, err := os.ReadFile("../../migrations/0092_knowledge_index_jobs_delete_operation.sql")
	if err != nil {
		t.Fatalf("read knowledge index jobs delete-operation migration: %v", err)
	}
	baseline := string(baselineRaw)
	deleteOperation := string(deleteOperationRaw)

	if strings.Contains(baseline, "document_id TEXT NOT NULL REFERENCES knowledge_documents") {
		t.Fatalf("baseline index jobs migration must not keep a document FK that would delete vector cleanup jobs:\n%s", baseline)
	}
	for _, want := range []string{
		"document_id TEXT NOT NULL",
		"CHECK (operation IN ('upsert_document', 'delete_document'))",
	} {
		if !strings.Contains(baseline, want) {
			t.Fatalf("expected baseline knowledge index job migration to contain %q, got:\n%s", want, baseline)
		}
	}
	for _, want := range []string{
		"DROP CONSTRAINT IF EXISTS knowledge_index_jobs_document_id_fkey",
		"DROP CONSTRAINT IF EXISTS knowledge_index_jobs_operation_check",
		"CHECK (operation IN ('upsert_document', 'delete_document'))",
	} {
		if !strings.Contains(deleteOperation, want) {
			t.Fatalf("expected delete-operation migration to contain %q, got:\n%s", want, deleteOperation)
		}
	}
}

func TestKnowledgeIngestionJobsMigrationDeclaresDurableWorkerFields(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/0093_knowledge_ingestion_jobs.sql")
	if err != nil {
		t.Fatalf("read knowledge ingestion jobs migration: %v", err)
	}
	ownershipRaw, err := os.ReadFile("../../migrations/microservices/table-ownership.json")
	if err != nil {
		t.Fatalf("read table ownership: %v", err)
	}
	migration := string(raw)

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS knowledge_ingestion_jobs",
		"organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE",
		"knowledge_base_id TEXT NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE",
		"document_id TEXT NOT NULL DEFAULT ''",
		"raw_content BYTEA NOT NULL DEFAULT ''",
		"raw_filename TEXT NOT NULL DEFAULT ''",
		"raw_content_type TEXT NOT NULL DEFAULT ''",
		"raw_size_bytes BIGINT NOT NULL DEFAULT 0",
		"document_version TEXT NOT NULL DEFAULT 'v1'",
		"update_strategy TEXT NOT NULL DEFAULT 'full_replace'",
		"source_url TEXT NOT NULL DEFAULT ''",
		"page_number INTEGER NOT NULL DEFAULT 0",
		"max_attempts INTEGER NOT NULL DEFAULT 5",
		"locked_at TIMESTAMPTZ",
		"locked_by TEXT NOT NULL DEFAULT ''",
		"completed_at TIMESTAMPTZ",
		"CHECK (status IN ('pending', 'processing', 'succeeded', 'failed', 'dead_letter'))",
		"CHECK (raw_size_bytes >= 0)",
	} {
		if !strings.Contains(migration, want) {
			t.Fatalf("expected knowledge ingestion job migration to contain %q, got:\n%s", want, migration)
		}
	}
	if !strings.Contains(string(ownershipRaw), `"knowledge_ingestion_jobs"`) {
		t.Fatalf("expected table ownership to include knowledge_ingestion_jobs, got:\n%s", ownershipRaw)
	}
}

func TestKnowledgeDocumentVersionHistoryMigrationBackfillsExistingDocuments(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/0075_knowledge_document_version_history.sql")
	if err != nil {
		t.Fatalf("read knowledge document version history migration: %v", err)
	}
	migration := string(raw)

	for _, want := range []string{
		"INSERT INTO knowledge_document_versions",
		"'kdv_' || md5(d.organization_id || ':' || d.knowledge_base_id || ':' || d.id || ':' || COALESCE(NULLIF(d.document_version, ''), 'v1'))",
		"FROM knowledge_documents d",
		"JOIN knowledge_bases kb ON kb.id = d.knowledge_base_id",
		"COALESCE(NULLIF(d.document_version, ''), 'v1')",
		"ON CONFLICT (organization_id, knowledge_base_id, document_id, document_version)",
	} {
		if !strings.Contains(migration, want) {
			t.Fatalf("expected migration to contain %q, got:\n%s", want, migration)
		}
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
	calls          []string
	execQueries    []string
	execArgs       [][]driver.NamedValue
}

func (q *knowledgeRetrievalQueryer) matchRowsPattern(query string) (string, bool) {
	if len(q.rowsByQuery) == 0 {
		return "", false
	}
	best := ""
	for pattern := range q.rowsByQuery {
		if strings.Contains(query, pattern) && len(pattern) > len(best) {
			best = pattern
		}
	}
	return best, best != ""
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
	return knowledgeRetrievalTx{queryer: c.queryer}, nil
}

func (c knowledgeRetrievalConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.queryer.mu.Lock()
	c.queryer.calls = append(c.queryer.calls, "exec:"+query)
	c.queryer.execQueries = append(c.queryer.execQueries, query)
	c.queryer.execArgs = append(c.queryer.execArgs, append([]driver.NamedValue(nil), args...))
	c.queryer.mu.Unlock()
	return driver.RowsAffected(1), nil
}

func (c knowledgeRetrievalConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.queryer.mu.Lock()
	c.queryer.calls = append(c.queryer.calls, "query:"+query)
	c.queryer.query = query
	c.queryer.args = append([]driver.NamedValue(nil), args...)
	rows := append([][]driver.Value(nil), c.queryer.rows...)
	columns := knowledgeRetrievalResultColumns()
	if pattern, ok := c.queryer.matchRowsPattern(query); ok {
		rows = append([][]driver.Value(nil), c.queryer.rowsByQuery[pattern]...)
		if configuredColumns, ok := c.queryer.columnsByQuery[pattern]; ok {
			columns = append([]string(nil), configuredColumns...)
		} else if len(rows) > 0 {
			columns = generatedKnowledgeTestColumns(len(rows[0]))
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

type knowledgeRetrievalTx struct {
	queryer *knowledgeRetrievalQueryer
}

func (knowledgeRetrievalTx) Commit() error {
	return nil
}

func (knowledgeRetrievalTx) Rollback() error {
	return nil
}

func (tx knowledgeRetrievalTx) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	tx.queryer.mu.Lock()
	tx.queryer.calls = append(tx.queryer.calls, "exec:"+query)
	tx.queryer.execQueries = append(tx.queryer.execQueries, query)
	tx.queryer.execArgs = append(tx.queryer.execArgs, append([]driver.NamedValue(nil), args...))
	tx.queryer.mu.Unlock()
	return driver.RowsAffected(1), nil
}

func (tx knowledgeRetrievalTx) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	tx.queryer.mu.Lock()
	tx.queryer.calls = append(tx.queryer.calls, "query:"+query)
	tx.queryer.query = query
	tx.queryer.args = append([]driver.NamedValue(nil), args...)
	rows := append([][]driver.Value(nil), tx.queryer.rows...)
	columns := knowledgeRetrievalResultColumns()
	if pattern, ok := tx.queryer.matchRowsPattern(query); ok {
		rows = append([][]driver.Value(nil), tx.queryer.rowsByQuery[pattern]...)
		if configuredColumns, ok := tx.queryer.columnsByQuery[pattern]; ok {
			columns = append([]string(nil), configuredColumns...)
		} else if len(rows) > 0 {
			columns = generatedKnowledgeTestColumns(len(rows[0]))
		}
	}
	tx.queryer.mu.Unlock()
	return &knowledgeRetrievalRows{
		columns: columns,
		rows:    rows,
	}, nil
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

func knowledgeRetrievalArgBytes(args []driver.NamedValue, ordinal int) []byte {
	for _, arg := range args {
		if arg.Ordinal != ordinal {
			continue
		}
		switch value := arg.Value.(type) {
		case []byte:
			return append([]byte(nil), value...)
		case string:
			return []byte(value)
		}
	}
	return nil
}

func knowledgeRetrievalArgInt(args []driver.NamedValue, ordinal int) int {
	for _, arg := range args {
		if arg.Ordinal != ordinal {
			continue
		}
		switch value := arg.Value.(type) {
		case int:
			return value
		case int64:
			return int(value)
		}
	}
	return 0
}

func knowledgeRetrievalExecArgsForQuery(t *testing.T, queryer *knowledgeRetrievalQueryer, pattern string) []driver.NamedValue {
	t.Helper()
	_, args := knowledgeRetrievalExecForQuery(t, queryer, pattern)
	return args
}

func knowledgeRetrievalExecForQuery(t *testing.T, queryer *knowledgeRetrievalQueryer, pattern string) (string, []driver.NamedValue) {
	t.Helper()
	queryer.mu.Lock()
	defer queryer.mu.Unlock()
	for index, query := range queryer.execQueries {
		if strings.Contains(query, pattern) {
			return query, append([]driver.NamedValue(nil), queryer.execArgs[index]...)
		}
	}
	t.Fatalf("expected exec query matching %q, got %v", pattern, queryer.execQueries)
	return "", nil
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
