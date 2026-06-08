package marketplace

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

func TestBuildOrderByRecommendedUsesHybridRecommendationSignals(t *testing.T) {
	orderBy := buildOrderBy("recommended", "rank")

	for _, fragment := range []string{
		"install_count",
		"rating_avg",
		"rank",
		"category_match",
		"tag_match",
		"exploration_score",
		"marketplace_agent_ranking_signals",
		"curated_weight",
		"governance_weight",
		"click_count",
		"impression_count",
		"install_conversion_count",
	} {
		if !strings.Contains(orderBy, fragment) {
			t.Fatalf("expected recommended order to include %q, got: %s", fragment, orderBy)
		}
	}
}

func TestBuildOrderByInstallsAliasMatchesPopular(t *testing.T) {
	if buildOrderBy("installs", "") != buildOrderBy("popular", "") {
		t.Fatalf("expected sort=installs to match sort=popular")
	}
}

func TestAddRecommendationMetadataExplainsRecommendedResults(t *testing.T) {
	agents := []*PublishedAgent{{
		ID:           "agent_invoice",
		Name:         "Invoice Reconciliation Agent",
		CategoryName: "Finance",
		Tags:         []string{"billing", "invoice"},
		InstallCount: 42,
		RatingAvg:    4.7,
	}}

	addRecommendationMetadata(agents, MarketplaceSearchFilter{
		Query:        "invoice",
		CategorySlug: "finance",
		Tags:         []string{"billing"},
		Sort:         "recommended",
	})

	if agents[0].Recommendation == nil {
		t.Fatal("expected recommendation metadata")
	}
	if agents[0].Recommendation.Score <= 0 {
		t.Fatalf("expected positive recommendation score, got %+v", agents[0].Recommendation)
	}
	for _, want := range []string{"Matches \"invoice\"", "Finance", "billing", "4.7 rating", "42 installs"} {
		if !strings.Contains(agents[0].Recommendation.Reason, want) {
			t.Fatalf("expected reason to contain %q, got %q", want, agents[0].Recommendation.Reason)
		}
	}
}

func TestSearchAgentsRecommendedRanksContentMatchesOverGenericHotAgents(t *testing.T) {
	database := searchTestDB(t)
	seedSearchMarketplace(t, database)
	service := NewSearchService(database)

	agents, total, err := service.SearchAgents(context.Background(), MarketplaceSearchFilter{
		Query:        "invoice",
		CategorySlug: "finance",
		Tags:         []string{"billing"},
		Sort:         "recommended",
		Limit:        10,
	})
	if err != nil {
		t.Fatalf("SearchAgents returned error: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 finance billing matches, got total=%d agents=%v", total, agentIDs(agents))
	}
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}
	if agents[0].ID != "agent_invoice_relevant" {
		t.Fatalf("expected content-matched invoice agent first, got %v", agentIDs(agents))
	}
	if agents[1].ID != "agent_invoice_explorer" {
		t.Fatalf("expected exploration candidate second, got %v", agentIDs(agents))
	}
}

func TestSearchAgentsRecommendedFallbackExplorationIsDeterministicAndNonEmpty(t *testing.T) {
	database := searchTestDB(t)
	seedSearchMarketplace(t, database)
	service := NewSearchService(database)

	filter := MarketplaceSearchFilter{Sort: "recommended", Limit: 4}
	first, total, err := service.SearchAgents(context.Background(), filter)
	if err != nil {
		t.Fatalf("SearchAgents first call returned error: %v", err)
	}
	second, _, err := service.SearchAgents(context.Background(), filter)
	if err != nil {
		t.Fatalf("SearchAgents second call returned error: %v", err)
	}
	if total < 4 || len(first) != 4 || len(second) != 4 {
		t.Fatalf("expected recommended fallback to return 4 of %d agents, first=%v second=%v", total, agentIDs(first), agentIDs(second))
	}
	if strings.Join(agentIDs(first), ",") != strings.Join(agentIDs(second), ",") {
		t.Fatalf("expected deterministic exploration ordering, first=%v second=%v", agentIDs(first), agentIDs(second))
	}
	if !containsAgent(first, "agent_invoice_explorer") {
		t.Fatalf("expected low-traffic exploration candidate to appear in fallback result, got %v", agentIDs(first))
	}
}

func TestSearchAgentsRecommendedUsesRankingSignals(t *testing.T) {
	database := searchTestDB(t)
	seedSearchMarketplace(t, database)
	service := NewSearchService(database)

	if _, err := database.Exec(`
		INSERT INTO marketplace_agent_ranking_signals (
			agent_id, impression_count, click_count, install_conversion_count,
			curated_weight, governance_weight, updated_at
		)
		VALUES
			('agent_invoice_explorer', 100, 40, 20, 2.0, 1.0, NOW()),
			('agent_invoice_relevant', 100, 5, 2, 1.0, 1.0, NOW())
	`); err != nil {
		t.Fatalf("insert ranking signals: %v", err)
	}

	agents, _, err := service.SearchAgents(context.Background(), MarketplaceSearchFilter{
		Query:        "invoice",
		CategorySlug: "finance",
		Tags:         []string{"billing"},
		Sort:         "recommended",
		Limit:        10,
	})
	if err != nil {
		t.Fatalf("SearchAgents returned error: %v", err)
	}
	if len(agents) < 2 || agents[0].ID != "agent_invoice_explorer" {
		t.Fatalf("expected curated high-conversion explorer first, got %v", agentIDs(agents))
	}
}

func TestSearchAgentsRecommendedDemotesGovernanceWeightedAgents(t *testing.T) {
	database := searchTestDB(t)
	seedSearchMarketplace(t, database)
	service := NewSearchService(database)

	if _, err := database.Exec(`
		INSERT INTO marketplace_agent_ranking_signals (
			agent_id, impression_count, click_count, install_conversion_count,
			curated_weight, governance_weight, updated_at
		)
		VALUES
			('agent_generic_hot', 1000, 400, 200, 1.0, 0.2, NOW()),
			('agent_high_rating', 100, 30, 15, 1.0, 1.0, NOW())
	`); err != nil {
		t.Fatalf("insert ranking signals: %v", err)
	}

	agents, _, err := service.SearchAgents(context.Background(), MarketplaceSearchFilter{
		Sort:  "recommended",
		Limit: 4,
	})
	if err != nil {
		t.Fatalf("SearchAgents returned error: %v", err)
	}
	if len(agents) < 2 {
		t.Fatalf("expected at least two agents, got %v", agentIDs(agents))
	}
	if indexOfAgent(agents, "agent_generic_hot") <= indexOfAgent(agents, "agent_high_rating") {
		t.Fatalf("expected governance-demoted hot agent below high-rating agent, got %v", agentIDs(agents))
	}
}

func TestSearchAgentsExistingSortsRemainStable(t *testing.T) {
	database := searchTestDB(t)
	seedSearchMarketplace(t, database)
	service := NewSearchService(database)

	cases := []struct {
		name string
		sort string
		want string
	}{
		{name: "popular", sort: "popular", want: "agent_generic_hot"},
		{name: "installs", sort: "installs", want: "agent_generic_hot"},
		{name: "rating", sort: "rating", want: "agent_high_rating"},
		{name: "newest", sort: "newest", want: "agent_invoice_explorer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agents, _, err := service.SearchAgents(context.Background(), MarketplaceSearchFilter{Sort: tc.sort, Limit: 4})
			if err != nil {
				t.Fatalf("SearchAgents returned error: %v", err)
			}
			if len(agents) == 0 || agents[0].ID != tc.want {
				t.Fatalf("expected %s first for sort=%s, got %v", tc.want, tc.sort, agentIDs(agents))
			}
		})
	}
}

func searchTestDB(t *testing.T) *sql.DB {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		if os.Getenv("OBLIVIOUS_REQUIRE_TEST_DATABASE") == "true" {
			t.Fatal("TEST_DATABASE_URL is required when OBLIVIOUS_REQUIRE_TEST_DATABASE=true")
		}
		t.Skip("TEST_DATABASE_URL is required for marketplace search integration tests")
	}
	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open search database: %v", err)
	}
	// Pin to a single connection so the advisory lock is held for the
	// lifetime of the test and cannot be bypassed by the connection pool.
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	if err := database.Ping(); err != nil {
		t.Fatalf("ping search database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})
	if _, err := database.Exec(`SELECT pg_advisory_lock(104212)`); err != nil {
		t.Fatalf("lock search database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(`SELECT pg_advisory_unlock(104212)`); err != nil {
			t.Fatalf("unlock search database: %v", err)
		}
	})

	statements := []string{
		`DROP TABLE IF EXISTS marketplace_agent_ranking_signals CASCADE`,
		`DROP TABLE IF EXISTS agent_installs CASCADE`,
		`DROP TABLE IF EXISTS published_agents CASCADE`,
		`DROP TABLE IF EXISTS categories CASCADE`,
		`DROP TABLE IF EXISTS organizations CASCADE`,
		`DROP TABLE IF EXISTS users CASCADE`,
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, role TEXT NOT NULL DEFAULT 'user', name TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE organizations (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active', metadata JSONB NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE categories (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, slug TEXT NOT NULL UNIQUE, display_order INTEGER NOT NULL DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE published_agents (id TEXT PRIMARY KEY, owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, name TEXT NOT NULL, description TEXT NOT NULL, icon_url TEXT, category_id TEXT REFERENCES categories(id), tags TEXT[] NOT NULL DEFAULT '{}', tools JSONB, example_conversations JSONB, system_prompt TEXT, visibility TEXT NOT NULL DEFAULT 'public', status TEXT NOT NULL DEFAULT 'approved', review_reason TEXT, pricing_type TEXT NOT NULL DEFAULT 'free', pricing_amount DECIMAL(10,2) DEFAULT 0, install_count INTEGER NOT NULL DEFAULT 0, rating_avg DECIMAL(3,2) DEFAULT 0, rating_count INTEGER NOT NULL DEFAULT 0, reviewed_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE agent_installs (id TEXT PRIMARY KEY, agent_id TEXT NOT NULL REFERENCES published_agents(id) ON DELETE CASCADE, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, version_id TEXT, installed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE(organization_id, agent_id, user_id))`,
		`CREATE TABLE marketplace_agent_ranking_signals (agent_id TEXT PRIMARY KEY REFERENCES published_agents(id) ON DELETE CASCADE, impression_count BIGINT NOT NULL DEFAULT 0, click_count BIGINT NOT NULL DEFAULT 0, install_conversion_count BIGINT NOT NULL DEFAULT 0, curated_weight DECIMAL(8,4) NOT NULL DEFAULT 1.0, governance_weight DECIMAL(8,4) NOT NULL DEFAULT 1.0, updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare search database: %v", err)
		}
	}
	return database
}

func seedSearchMarketplace(t *testing.T, database *sql.DB) {
	t.Helper()
	if _, err := database.Exec(`
		INSERT INTO users (id, email, password_hash, name, created_at)
		VALUES ('owner_search', 'owner-search@example.com', 'hash', 'Publisher', NOW()),
		       ('user_search', 'user-search@example.com', 'hash', 'Buyer', NOW())
	`); err != nil {
		t.Fatalf("insert search users: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO organizations (id, slug, name, status, metadata, created_at, updated_at)
		VALUES ('org_search', 'org-search', 'Search Org', 'active', '{}', NOW(), NOW())
	`); err != nil {
		t.Fatalf("insert search org: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO categories (id, name, slug, display_order)
		VALUES ('cat_finance', 'Finance', 'finance', 1),
		       ('cat_productivity', 'Productivity', 'productivity', 2)
	`); err != nil {
		t.Fatalf("insert search categories: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO published_agents (
			id, owner_id, organization_id, name, description, category_id, tags, tools,
			visibility, status, pricing_type, pricing_amount, install_count, rating_avg, rating_count, created_at, updated_at
		)
		VALUES
			('agent_generic_hot', 'owner_search', 'org_search', 'Generic Operations Copilot', 'A broad productivity assistant for daily operations.', 'cat_productivity', ARRAY['operations']::text[], '{}'::jsonb, 'public', 'approved', 'free', 0, 500, 4.8, 80, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
			('agent_high_rating', 'owner_search', 'org_search', 'Quality Desk', 'A support quality assistant.', 'cat_productivity', ARRAY['support']::text[], '{}'::jsonb, 'public', 'approved', 'free', 0, 30, 5.0, 12, '2026-02-01T00:00:00Z', '2026-02-01T00:00:00Z'),
			('agent_invoice_relevant', 'owner_search', 'org_search', 'Invoice Reconciliation Agent', 'Matches invoice payments and billing exceptions.', 'cat_finance', ARRAY['billing','invoice']::text[], '{}'::jsonb, 'public', 'approved', 'free', 0, 35, 4.4, 9, '2026-03-01T00:00:00Z', '2026-03-01T00:00:00Z'),
			('agent_invoice_explorer', 'owner_search', 'org_search', 'Invoice Exception Scout', 'Finds unusual billing mismatches for finance teams.', 'cat_finance', ARRAY['billing']::text[], '{}'::jsonb, 'public', 'approved', 'free', 0, 1, 4.0, 1, '2026-04-01T00:00:00Z', '2026-04-01T00:00:00Z')
	`); err != nil {
		t.Fatalf("insert search agents: %v", err)
	}
}

func agentIDs(agents []*PublishedAgent) []string {
	ids := make([]string, 0, len(agents))
	for _, agent := range agents {
		ids = append(ids, agent.ID)
	}
	return ids
}

func containsAgent(agents []*PublishedAgent, agentID string) bool {
	for _, agent := range agents {
		if agent.ID == agentID {
			return true
		}
	}
	return false
}

func indexOfAgent(agents []*PublishedAgent, agentID string) int {
	for idx, agent := range agents {
		if agent.ID == agentID {
			return idx
		}
	}
	return len(agents)
}
