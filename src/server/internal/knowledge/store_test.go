package knowledge

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
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

func testKnowledgeRAGSQLStore(t *testing.T) (*SQLStore, context.Context) {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		if strings.EqualFold(os.Getenv("OBLIVIOUS_REQUIRE_TEST_DATABASE"), "true") {
			t.Fatal("TEST_DATABASE_URL is required for DB-backed knowledge RAG tests")
		}
		t.Skip("TEST_DATABASE_URL is required for DB-backed knowledge RAG tests")
	}

	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})

	if _, err := database.Exec(`SELECT pg_advisory_lock(104227)`); err != nil {
		t.Fatalf("lock knowledge RAG test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(`SELECT pg_advisory_unlock(104227)`); err != nil {
			t.Fatalf("unlock knowledge RAG test database: %v", err)
		}
	})

	statements := []string{
		`DROP TABLE IF EXISTS knowledge_document_chunks CASCADE`,
		`DROP TABLE IF EXISTS knowledge_documents CASCADE`,
		`DROP TABLE IF EXISTS knowledge_bases CASCADE`,
		`CREATE TABLE knowledge_bases (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, organization_id TEXT, name TEXT NOT NULL, document_count INTEGER NOT NULL DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE knowledge_documents (id TEXT PRIMARY KEY, knowledge_base_id TEXT NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE, organization_id TEXT, title TEXT NOT NULL, content TEXT NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE knowledge_document_chunks (id TEXT PRIMARY KEY, document_id TEXT NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE, organization_id TEXT, chunk_index INTEGER NOT NULL, content TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare knowledge RAG database: %v\nstatement: %s", err, statement)
		}
	}

	migration, err := os.ReadFile("../../migrations/0032_knowledge_rag_index.sql")
	if err != nil {
		t.Fatalf("read knowledge RAG migration: %v", err)
	}
	if _, err := database.Exec(string(migration)); err != nil {
		t.Fatalf("apply knowledge RAG migration: %v", err)
	}

	return NewSQLStore(database), context.Background()
}

func TestKnowledgeStorePersistsChunkEmbeddingsAndRetrievesBySimilarity(t *testing.T) {
	store, ctx := testKnowledgeRAGSQLStore(t)

	base, err := store.CreateKnowledgeBase(ctx, "workspace_1", "org_1", "RAG Sources")
	if err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	_, err = store.CreateKnowledgeDocument(ctx, "org_1", base.ID, "Deployment Runbook", "deployment rollback recovery", []KnowledgeDocumentChunk{
		{
			Content:        "General notes that are farther away.",
			ChunkIndex:     0,
			Embedding:      testKnowledgeEmbedding(0, 1),
			EmbeddingModel: "text-embedding-3-small",
			IndexedAt:      time.Now().UTC(),
		},
		{
			Content:        "Deployment rollback runbook source citation.",
			ChunkIndex:     1,
			Embedding:      testKnowledgeEmbedding(1, 0),
			EmbeddingModel: "text-embedding-3-small",
			IndexedAt:      time.Now().UTC(),
		},
	})
	if err != nil {
		t.Fatalf("create knowledge document: %v", err)
	}

	results, err := store.RetrieveKnowledge(ctx, "org_1", base.ID, testKnowledgeEmbedding(1, 0), 5, 0)
	if err != nil {
		t.Fatalf("retrieve knowledge: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected vector retrieval results")
	}
	if results[0].ChunkIndex != 1 {
		t.Fatalf("expected closest chunk index 1 first, got %+v", results[0])
	}
	if results[0].RetrievalMethod != knowledgeRAGRetrievalMethod {
		t.Fatalf("expected embedding RAG retrieval method, got %q", results[0].RetrievalMethod)
	}
	if results[0].Source.DocumentTitle != "Deployment Runbook" || results[0].Source.ChunkID == "" {
		t.Fatalf("expected source citation fields, got %+v", results[0].Source)
	}
}

func TestKnowledgeStoreRetrieveRAGRejectsCrossTenantChunks(t *testing.T) {
	store, ctx := testKnowledgeRAGSQLStore(t)

	baseOne, err := store.CreateKnowledgeBase(ctx, "workspace_1", "org_1", "Org One")
	if err != nil {
		t.Fatalf("create org one knowledge base: %v", err)
	}
	baseTwo, err := store.CreateKnowledgeBase(ctx, "workspace_2", "org_2", "Org Two")
	if err != nil {
		t.Fatalf("create org two knowledge base: %v", err)
	}
	if _, err := store.CreateKnowledgeDocument(ctx, "org_1", baseOne.ID, "Org One Source", "tenant one source", []KnowledgeDocumentChunk{{
		Content:        "Tenant one citation.",
		ChunkIndex:     0,
		Embedding:      testKnowledgeEmbedding(1, 0),
		EmbeddingModel: "text-embedding-3-small",
		IndexedAt:      time.Now().UTC(),
	}}); err != nil {
		t.Fatalf("create org one document: %v", err)
	}
	if _, err := store.CreateKnowledgeDocument(ctx, "org_2", baseTwo.ID, "Org Two Source", "tenant two source", []KnowledgeDocumentChunk{{
		Content:        "Tenant two citation should not leak.",
		ChunkIndex:     0,
		Embedding:      testKnowledgeEmbedding(1, 0),
		EmbeddingModel: "text-embedding-3-small",
		IndexedAt:      time.Now().UTC(),
	}}); err != nil {
		t.Fatalf("create org two document: %v", err)
	}

	results, err := store.RetrieveKnowledge(ctx, "org_1", baseOne.ID, testKnowledgeEmbedding(1, 0), 10, 0)
	if err != nil {
		t.Fatalf("retrieve knowledge: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected only org one result, got %d: %+v", len(results), results)
	}
	if results[0].DocumentTitle != "Org One Source" {
		t.Fatalf("expected org one source only, got %+v", results[0])
	}
}

func testKnowledgeEmbedding(first, second float32) []float32 {
	embedding := make([]float32, 1536)
	embedding[0] = first
	embedding[1] = second
	return embedding
}
