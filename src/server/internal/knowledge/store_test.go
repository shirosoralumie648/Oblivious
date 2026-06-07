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
	if chunks[0].Metadata.PageNumber != 7 || chunks[0].Metadata.SourceURL != "https://docs.example/runbook.md" {
		t.Fatalf("expected metadata to be decoded, got %+v", chunks[0].Metadata)
	}

	queryer.mu.Lock()
	query := queryer.query
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()
	for _, want := range []string{"JOIN knowledge_documents", "JOIN knowledge_bases", "c.organization_id = $1", "kb.organization_id = $1", "kb.id = $2", "d.id = $3", "ORDER BY c.chunk_index ASC"} {
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
	if chunk.ChunkID != "kdc_1" || chunk.Content != "Updated chunk content." || chunk.DocumentVersion != "v2" {
		t.Fatalf("unexpected updated chunk: %+v", chunk)
	}

	queryer.mu.Lock()
	query := queryer.query
	args := append([]driver.NamedValue(nil), queryer.args...)
	queryer.mu.Unlock()
	for _, want := range []string{"UPDATE knowledge_document_chunks", "FROM knowledge_documents", "JOIN knowledge_bases", "c.organization_id = $1", "kb.organization_id = $1", "c.id = $4", "metadata", "RETURNING"} {
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
				{"kdc_left", int64(1), "Alpha beta", "v2", []byte(`{"documentVersion":"v2","pageNumber":3}`)},
				{"kdc_right", int64(2), "gamma delta", "v2", []byte(`{"documentVersion":"v2","pageNumber":3}`)},
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
				{"kdc_current", int64(1), "Alpha beta\n\ngamma delta", "v2", []byte(`{"documentVersion":"v2","pageNumber":3}`)},
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
	tx.queryer.execQueries = append(tx.queryer.execQueries, query)
	tx.queryer.execArgs = append(tx.queryer.execArgs, append([]driver.NamedValue(nil), args...))
	tx.queryer.mu.Unlock()
	return driver.RowsAffected(1), nil
}

func (tx knowledgeRetrievalTx) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	tx.queryer.mu.Lock()
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
