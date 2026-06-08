package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestSemanticCachePolicyClassifiesScopeAndTTL(t *testing.T) {
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	cache := NewSemanticCache(NewInMemorySemanticCacheStore(), SemanticCacheOptions{
		Now: func() time.Time { return now },
	})

	tests := []struct {
		name string
		req  SemanticCacheRequest
		want SemanticCacheScope
		ttl  time.Duration
	}{
		{
			name: "public knowledge query is globally shared",
			req:  SemanticCacheRequest{OrganizationID: "org_1", Model: "gpt-4o", Query: "What is AI?"},
			want: SemanticCacheScopeGlobal,
			ttl:  24 * time.Hour,
		},
		{
			name: "email makes query organization scoped",
			req:  SemanticCacheRequest{OrganizationID: "org_1", Model: "gpt-4o", Query: "Summarize shiro@example.com account"},
			want: SemanticCacheScopeOrganization,
			ttl:  time.Hour,
		},
		{
			name: "personal pronoun makes query organization scoped",
			req:  SemanticCacheRequest{OrganizationID: "org_1", Model: "gpt-4o", Query: "我的账户余额是多少"},
			want: SemanticCacheScopeOrganization,
			ttl:  time.Hour,
		},
		{
			name: "known organization name makes query organization scoped",
			req:  SemanticCacheRequest{OrganizationID: "org_1", OrganizationName: "Acme Corp", Model: "gpt-4o", Query: "Compare Acme Corp renewal options"},
			want: SemanticCacheScopeOrganization,
			ttl:  time.Hour,
		},
		{
			name: "explicit user scoped query is organization scoped",
			req:  SemanticCacheRequest{OrganizationID: "org_1", Model: "gpt-4o", Query: "What is AI?", UserScoped: true},
			want: SemanticCacheScopeOrganization,
			ttl:  time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := cache.PolicyFor(tt.req)
			if policy.Scope != tt.want {
				t.Fatalf("expected scope %q, got %q", tt.want, policy.Scope)
			}
			if policy.TTL != tt.ttl {
				t.Fatalf("expected TTL %s, got %s", tt.ttl, policy.TTL)
			}
		})
	}
}

func TestSemanticCacheQueryHashIncludesModelAndIgnoresChannelID(t *testing.T) {
	cache := NewSemanticCache(NewInMemorySemanticCacheStore(), SemanticCacheOptions{})

	channelA := cache.PolicyFor(SemanticCacheRequest{
		OrganizationID: "org_1",
		Model:          "gpt-4o",
		Query:          "What is AI?",
		ChannelID:      "channel_a",
	}).Key.QueryHash
	channelB := cache.PolicyFor(SemanticCacheRequest{
		OrganizationID: "org_1",
		Model:          "gpt-4o",
		Query:          "What is AI?",
		ChannelID:      "channel_b",
	}).Key.QueryHash
	otherModel := cache.PolicyFor(SemanticCacheRequest{
		OrganizationID: "org_1",
		Model:          "gpt-4o-mini",
		Query:          "What is AI?",
		ChannelID:      "channel_a",
	}).Key.QueryHash
	otherQuery := cache.PolicyFor(SemanticCacheRequest{
		OrganizationID: "org_1",
		Model:          "gpt-4o",
		Query:          "What is AGI?",
		ChannelID:      "channel_a",
	}).Key.QueryHash

	if channelA != channelB {
		t.Fatalf("query hash must not include channel id: %s != %s", channelA, channelB)
	}
	if channelA == otherModel {
		t.Fatal("query hash must include model")
	}
	if channelA == otherQuery {
		t.Fatal("query hash must include query")
	}
}

func TestSemanticCacheLookupOrderUsesGlobalThenOrgForPublicQueries(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	store := newSpySemanticCacheStore()
	cache := NewSemanticCache(store, SemanticCacheOptions{
		Now: func() time.Time { return now },
	})

	req := SemanticCacheRequest{OrganizationID: "org_1", Model: "gpt-4o", Query: "What is AI?"}
	globalKey := cache.PolicyFor(req).Key
	orgKey := globalKey
	orgKey.Scope = SemanticCacheScopeOrganization
	orgKey.OrganizationID = "org_1"

	store.entries[orgKey] = SemanticCacheEntry{
		Key:       orgKey,
		Response:  json.RawMessage(`{"source":"org"}`),
		CreatedAt: now.Add(-time.Minute),
		ExpiresAt: now.Add(time.Hour),
	}

	hit, err := cache.Lookup(ctx, req)
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	if hit == nil {
		t.Fatal("expected organization fallback hit")
	}
	if hit.Key != orgKey {
		t.Fatalf("expected org key hit, got %+v", hit.Key)
	}
	if string(hit.Response) != `{"source":"org"}` {
		t.Fatalf("unexpected response payload: %s", hit.Response)
	}

	wantLookups := []SemanticCacheKey{globalKey, orgKey}
	if !reflect.DeepEqual(store.lookups, wantLookups) {
		t.Fatalf("expected lookup order %+v, got %+v", wantLookups, store.lookups)
	}
	if !reflect.DeepEqual(store.hitIncrements, []SemanticCacheKey{orgKey}) {
		t.Fatalf("expected only org hit to be counted, got %+v", store.hitIncrements)
	}
}

func TestSemanticCacheLookupSensitiveQueriesNeverCheckGlobal(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	store := newSpySemanticCacheStore()
	cache := NewSemanticCache(store, SemanticCacheOptions{
		Now: func() time.Time { return now },
	})

	req := SemanticCacheRequest{OrganizationID: "org_1", Model: "gpt-4o", Query: "我的账户余额是多少"}
	orgKey := cache.PolicyFor(req).Key
	globalKey := orgKey
	globalKey.Scope = SemanticCacheScopeGlobal
	globalKey.OrganizationID = ""

	store.entries[globalKey] = SemanticCacheEntry{
		Key:       globalKey,
		Response:  json.RawMessage(`{"source":"global"}`),
		CreatedAt: now.Add(-time.Minute),
		ExpiresAt: now.Add(24 * time.Hour),
	}

	hit, err := cache.Lookup(ctx, req)
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	if hit != nil {
		t.Fatalf("sensitive query must not hit global cache, got %+v", hit.Key)
	}
	if !reflect.DeepEqual(store.lookups, []SemanticCacheKey{orgKey}) {
		t.Fatalf("expected only org lookup, got %+v", store.lookups)
	}
	if len(store.hitIncrements) != 0 {
		t.Fatalf("expected no hit increments, got %+v", store.hitIncrements)
	}
}

func TestSemanticCacheExpiredEntriesAreIgnoredAndNotHitCounted(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	store := newSpySemanticCacheStore()
	cache := NewSemanticCache(store, SemanticCacheOptions{
		Now: func() time.Time { return now },
	})

	req := SemanticCacheRequest{OrganizationID: "org_1", Model: "gpt-4o", Query: "What is AI?"}
	globalKey := cache.PolicyFor(req).Key
	orgKey := globalKey
	orgKey.Scope = SemanticCacheScopeOrganization
	orgKey.OrganizationID = "org_1"

	store.entries[globalKey] = SemanticCacheEntry{
		Key:       globalKey,
		Response:  json.RawMessage(`{"source":"expired-global"}`),
		CreatedAt: now.Add(-25 * time.Hour),
		ExpiresAt: now.Add(-time.Minute),
	}
	store.entries[orgKey] = SemanticCacheEntry{
		Key:       orgKey,
		Response:  json.RawMessage(`{"source":"org"}`),
		CreatedAt: now.Add(-time.Minute),
		ExpiresAt: now.Add(time.Hour),
	}

	hit, err := cache.Lookup(ctx, req)
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	if hit == nil || hit.Key != orgKey {
		t.Fatalf("expected fallback to unexpired org entry, got %+v", hit)
	}
	if !reflect.DeepEqual(store.hitIncrements, []SemanticCacheKey{orgKey}) {
		t.Fatalf("expected only unexpired org hit to be counted, got %+v", store.hitIncrements)
	}
}

func TestSemanticCacheStoreWritesClassifiedScopeWithPolicyTTL(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	store := NewInMemorySemanticCacheStore()
	cache := NewSemanticCache(store, SemanticCacheOptions{
		Now: func() time.Time { return now },
	})

	entry, err := cache.Store(ctx, SemanticCacheRequest{
		OrganizationID: "org_1",
		Model:          "gpt-4o",
		Query:          "What is AI?",
		ChannelID:      "channel_a",
	}, json.RawMessage(`{"answer":"AI"}`))
	if err != nil {
		t.Fatalf("store failed: %v", err)
	}
	if entry.Key.Scope != SemanticCacheScopeGlobal {
		t.Fatalf("expected global scope, got %q", entry.Key.Scope)
	}
	if entry.ExpiresAt.Sub(now) != 24*time.Hour {
		t.Fatalf("expected global TTL of 24h, got %s", entry.ExpiresAt.Sub(now))
	}

	hit, err := cache.Lookup(ctx, SemanticCacheRequest{
		OrganizationID: "org_1",
		Model:          "gpt-4o",
		Query:          "What is AI?",
		ChannelID:      "channel_b",
	})
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	if hit == nil {
		t.Fatal("expected cache hit")
	}
	if hit.Key != entry.Key {
		t.Fatalf("expected channel-independent cache key %+v, got %+v", entry.Key, hit.Key)
	}
	if string(hit.Response) != `{"answer":"AI"}` {
		t.Fatalf("unexpected cached response: %s", hit.Response)
	}
}

func TestSemanticCacheStorePersistsQueryEmbedding(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	store := NewInMemorySemanticCacheStore()
	cache := NewSemanticCache(store, SemanticCacheOptions{
		Now: func() time.Time { return now },
	})

	req := SemanticCacheRequest{
		OrganizationID: "org_1",
		Model:          "gpt-4o",
		Query:          "What is semantic caching?",
		QueryEmbedding: []float32{0.1, 0.2, 0.3},
	}
	entry, err := cache.Store(ctx, req, json.RawMessage(`{"answer":"reuse similar answers"}`))
	if err != nil {
		t.Fatalf("store failed: %v", err)
	}
	if !reflect.DeepEqual(entry.QueryEmbedding, req.QueryEmbedding) {
		t.Fatalf("stored entry lost query embedding: got %+v want %+v", entry.QueryEmbedding, req.QueryEmbedding)
	}

	req.QueryEmbedding[0] = 9.9
	stored, err := store.Get(ctx, entry.Key)
	if err != nil {
		t.Fatalf("get stored entry: %v", err)
	}
	if stored == nil || !reflect.DeepEqual(stored.QueryEmbedding, []float32{0.1, 0.2, 0.3}) {
		t.Fatalf("cache store must defensively copy embeddings, got %+v", stored)
	}
}

func TestSemanticCacheLookupFallsBackToSimilarOrgScopedQueryWithinSameOrganization(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	store := NewInMemorySemanticCacheStore()
	cache := NewSemanticCache(store, SemanticCacheOptions{
		Now: func() time.Time { return now },
	})

	storedReq := SemanticCacheRequest{
		OrganizationID: "org_1",
		Model:          "gpt-4o",
		Query:          "My account: explain billing invoice retention policy.",
		ChannelID:      "channel_a",
		UserScoped:     true,
	}
	entry, err := cache.Store(ctx, storedReq, json.RawMessage(`{"answer":"retention"}`))
	if err != nil {
		t.Fatalf("store failed: %v", err)
	}
	if entry.Key.Scope != SemanticCacheScopeOrganization {
		t.Fatalf("expected stored entry to remain organization scoped, got %q", entry.Key.Scope)
	}

	similarReq := SemanticCacheRequest{
		OrganizationID: "org_1",
		Model:          "gpt-4o",
		Query:          "billing invoice retention policy explain MY account",
		ChannelID:      "channel_b",
		UserScoped:     true,
	}
	if cache.PolicyFor(similarReq).Key.QueryHash == entry.Key.QueryHash {
		t.Fatal("test requires a non-exact query hash")
	}

	hit, err := cache.Lookup(ctx, similarReq)
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	if hit == nil {
		t.Fatal("expected similar organization-scoped cache hit")
	}
	if hit.Key != entry.Key {
		t.Fatalf("expected similar hit to return stored entry key %+v, got %+v", entry.Key, hit.Key)
	}
	if string(hit.Response) != `{"answer":"retention"}` {
		t.Fatalf("unexpected cached response: %s", hit.Response)
	}

	crossOrgReq := similarReq
	crossOrgReq.OrganizationID = "org_2"
	crossOrgHit, err := cache.Lookup(ctx, crossOrgReq)
	if err != nil {
		t.Fatalf("cross-org lookup failed: %v", err)
	}
	if crossOrgHit != nil {
		t.Fatalf("user-scoped similar query must not cross organization boundary, got %+v", crossOrgHit.Key)
	}
}

func TestSemanticCacheTextSimilarityRequires085Threshold(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	store := NewInMemorySemanticCacheStore()
	cache := NewSemanticCache(store, SemanticCacheOptions{
		Now: func() time.Time { return now },
	})

	storedReq := SemanticCacheRequest{
		OrganizationID: "org_1",
		Model:          "gpt-4o",
		Query:          "alpha beta gamma delta epsilon zeta eta theta iota",
		UserScoped:     true,
	}
	if _, err := cache.Store(ctx, storedReq, json.RawMessage(`{"answer":"too broad"}`)); err != nil {
		t.Fatalf("store failed: %v", err)
	}

	hit, err := cache.Lookup(ctx, SemanticCacheRequest{
		OrganizationID: "org_1",
		Model:          "gpt-4o",
		Query:          "alpha beta gamma delta epsilon zeta eta theta kappa",
		UserScoped:     true,
	})
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	if hit != nil {
		t.Fatalf("0.80 text similarity must not meet the 0.85 semantic cache threshold, got hit %+v", hit.Key)
	}
}

func TestSemanticCacheLookupUsesEmbeddingSimilarityWhenQueryTextDiffers(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	store := NewInMemorySemanticCacheStore()
	cache := NewSemanticCache(store, SemanticCacheOptions{
		Now: func() time.Time { return now },
	})

	storedReq := SemanticCacheRequest{
		OrganizationID: "org_1",
		Model:          "gpt-4o",
		Query:          "pricing forecast renewal terms",
		QueryEmbedding: []float32{1, 0, 0},
	}
	entry, err := cache.Store(ctx, storedReq, json.RawMessage(`{"answer":"pricing"}`))
	if err != nil {
		t.Fatalf("store failed: %v", err)
	}

	decoyReq := SemanticCacheRequest{
		OrganizationID: "org_1",
		Model:          "gpt-4o",
		Query:          "security incident response",
		QueryEmbedding: []float32{0, 1, 0},
	}
	if _, err := cache.Store(ctx, decoyReq, json.RawMessage(`{"answer":"security"}`)); err != nil {
		t.Fatalf("store decoy: %v", err)
	}

	hit, err := cache.Lookup(ctx, SemanticCacheRequest{
		OrganizationID: "org_1",
		Model:          "gpt-4o",
		Query:          "completely different visible words",
		QueryEmbedding: []float32{0.99, 0.01, 0},
	})
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	if hit == nil {
		t.Fatal("expected embedding-similar cache hit")
	}
	if hit.Key != entry.Key {
		t.Fatalf("expected embedding similarity to pick stored entry %+v, got %+v", entry.Key, hit.Key)
	}
	if string(hit.Response) != `{"answer":"pricing"}` {
		t.Fatalf("unexpected cached response: %s", hit.Response)
	}
}

func TestSemanticCacheEmbeddingSimilarityRequires085Threshold(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	store := NewInMemorySemanticCacheStore()
	cache := NewSemanticCache(store, SemanticCacheOptions{
		Now: func() time.Time { return now },
	})

	storedReq := SemanticCacheRequest{
		OrganizationID: "org_1",
		Model:          "gpt-4o",
		Query:          "pricing forecast renewal terms",
		QueryEmbedding: []float32{1, 0},
	}
	if _, err := cache.Store(ctx, storedReq, json.RawMessage(`{"answer":"too broad"}`)); err != nil {
		t.Fatalf("store failed: %v", err)
	}

	hit, err := cache.Lookup(ctx, SemanticCacheRequest{
		OrganizationID: "org_1",
		Model:          "gpt-4o",
		Query:          "unrelated visible words",
		QueryEmbedding: []float32{0.84, 0.5425864},
	})
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	if hit != nil {
		t.Fatalf("0.80 embedding similarity must not meet the 0.85 semantic cache threshold, got hit %+v", hit.Key)
	}
}

func TestSemanticCacheMigrationCreatesRequiredExtensionsAndIndexes(t *testing.T) {
	migration, err := os.ReadFile("../../../migrations/0040_relay_semantic_cache.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(migration)
	for _, want := range []string{
		"CREATE EXTENSION IF NOT EXISTS pgcrypto",
		"CREATE EXTENSION IF NOT EXISTS vector",
		"CREATE TABLE IF NOT EXISTS relay_semantic_cache",
		"query_embedding vector(1536)",
		"response JSONB NOT NULL",
		"hit_count INTEGER NOT NULL DEFAULT 0",
		"USING hnsw",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration missing %q:\n%s", want, sql)
		}
	}
}

func TestSQLSemanticCacheStoreUsesPgvectorForSimilarityLookup(t *testing.T) {
	source, err := os.ReadFile("semantic_cache_sql_store.go")
	if err != nil {
		t.Fatalf("read sql store source: %v", err)
	}
	sqlStore := string(source)
	for _, want := range []string{
		"query_embedding <=>",
		"1 - (query_embedding <=>",
		"ORDER BY query_embedding <=>",
	} {
		if !strings.Contains(sqlStore, want) {
			t.Fatalf("SQL semantic cache store must use pgvector similarity; missing %q", want)
		}
	}
	if strings.Contains(sqlStore, "semanticCacheQuerySimilarity(query, entry.Query)") {
		t.Fatal("SQL semantic cache store must not scan rows and fall back to text/Jaccard similarity")
	}
}

func TestSQLSemanticCacheStorePersistsEntriesAndHitCounts(t *testing.T) {
	database := testSemanticCacheDatabase(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)

	cache := NewSemanticCache(NewSQLSemanticCacheStore(database), SemanticCacheOptions{
		Now: func() time.Time { return now },
	})
	req := SemanticCacheRequest{
		OrganizationID: "org_1",
		Model:          "gpt-4o",
		Query:          "What is semantic caching?",
	}

	stored, err := cache.Store(ctx, req, json.RawMessage(`{"answer":"reuse similar answers"}`))
	if err != nil {
		t.Fatalf("store entry: %v", err)
	}

	reopened := NewSemanticCache(NewSQLSemanticCacheStore(database), SemanticCacheOptions{
		Now: func() time.Time { return now.Add(time.Minute) },
	})
	hit, err := reopened.Lookup(ctx, req)
	if err != nil {
		t.Fatalf("lookup persisted entry: %v", err)
	}
	if hit == nil {
		t.Fatal("expected persisted cache hit")
	}
	if hit.Key != stored.Key {
		t.Fatalf("expected persisted key %+v, got %+v", stored.Key, hit.Key)
	}
	assertJSONRawMessageEqual(t, "persisted response", hit.Response, json.RawMessage(`{"answer":"reuse similar answers"}`))
	if hit.HitCount != 1 {
		t.Fatalf("expected lookup to return incremented hit count 1, got %d", hit.HitCount)
	}

	again, err := reopened.Lookup(ctx, req)
	if err != nil {
		t.Fatalf("second lookup persisted entry: %v", err)
	}
	if again == nil || again.HitCount != 2 {
		t.Fatalf("expected second lookup hit count 2, got %+v", again)
	}
}

func testSemanticCacheDatabase(t *testing.T) *sql.DB {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for semantic cache SQL store integration tests")
	}

	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	// Pin to a single connection so the advisory lock is held for the
	// lifetime of the test and cannot be bypassed by the connection pool.
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	if err := database.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})

	if _, err := database.Exec(`SELECT pg_advisory_lock(104240)`); err != nil {
		t.Fatalf("lock semantic cache test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(`SELECT pg_advisory_unlock(104240)`); err != nil {
			t.Fatalf("unlock semantic cache test database: %v", err)
		}
	})

	if _, err := database.Exec(`DROP TABLE IF EXISTS relay_semantic_cache CASCADE`); err != nil {
		t.Fatalf("drop semantic cache table: %v", err)
	}
	migration, err := os.ReadFile("../../../migrations/0040_relay_semantic_cache.sql")
	if err != nil {
		t.Fatalf("read semantic cache migration: %v", err)
	}
	if _, err := database.Exec(string(migration)); err != nil {
		t.Fatalf("apply semantic cache migration: %v", err)
	}
	if _, err := database.Exec(`TRUNCATE relay_semantic_cache`); err != nil {
		t.Fatalf("truncate semantic cache table: %v", err)
	}

	return database
}

func assertJSONRawMessageEqual(t *testing.T, label string, got, want json.RawMessage) {
	t.Helper()

	var gotValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("invalid %s JSON %s: %v", label, got, err)
	}
	var wantValue any
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("invalid expected %s JSON %s: %v", label, want, err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("unexpected %s: got %s want %s", label, got, want)
	}
}

type spySemanticCacheStore struct {
	entries       map[SemanticCacheKey]SemanticCacheEntry
	lookups       []SemanticCacheKey
	hitIncrements []SemanticCacheKey
}

func newSpySemanticCacheStore() *spySemanticCacheStore {
	return &spySemanticCacheStore{entries: make(map[SemanticCacheKey]SemanticCacheEntry)}
}

func (s *spySemanticCacheStore) Get(ctx context.Context, key SemanticCacheKey) (*SemanticCacheEntry, error) {
	s.lookups = append(s.lookups, key)
	entry, ok := s.entries[key]
	if !ok {
		return nil, nil
	}
	return &entry, nil
}

func (s *spySemanticCacheStore) Put(ctx context.Context, entry SemanticCacheEntry) error {
	s.entries[entry.Key] = entry
	return nil
}

func (s *spySemanticCacheStore) IncrementHit(ctx context.Context, key SemanticCacheKey) error {
	s.hitIncrements = append(s.hitIncrements, key)
	entry := s.entries[key]
	entry.HitCount++
	s.entries[key] = entry
	return nil
}
