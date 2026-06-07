package marketplace

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

// SearchService provides full-text search with multi-dimensional filters (D-30).
type SearchService struct {
	db *sql.DB
}

// NewSearchService creates a new SearchService.
func NewSearchService(db *sql.DB) *SearchService {
	return &SearchService{db: db}
}

// searchClause contains a SQL WHERE fragment and its parameter values.
type searchClause struct {
	cond string
	args []interface{}
}

// SearchAgents performs full-text search with filters, sorting, and pagination (D-30).
// Returns the matching agents slice and total count.
func (s *SearchService) SearchAgents(ctx context.Context, filter MarketplaceSearchFilter) ([]*PublishedAgent, int, error) {
	// Clamp limit
	limit := filter.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	// Build WHERE clauses
	var clauses []searchClause
	paramIdx := 1

	// Base filters: only approved + public agents
	clauses = append(clauses, searchClause{cond: "a.status = 'approved'", args: nil})
	clauses = append(clauses, searchClause{cond: "a.visibility = 'public'", args: nil})

	// Full-text search (D-30)
	hasQuery := false
	if strings.TrimSpace(filter.Query) != "" {
		hasQuery = true
		paramIdx++
		clauses = append(clauses, searchClause{
			cond: fmt.Sprintf("(to_tsvector('english', a.name || ' ' || a.description) @@ plainto_tsquery('english', $%d))", paramIdx-1),
			args: []interface{}{filter.Query},
		})
	}

	// Category filter
	if filter.CategorySlug != "" {
		clauses = append(clauses, searchClause{
			cond: fmt.Sprintf("c.slug = $%d", paramIdx),
			args: []interface{}{filter.CategorySlug},
		})
		paramIdx++
	}

	// Tags filter (array overlap)
	if len(filter.Tags) > 0 {
		clauses = append(clauses, searchClause{
			cond: fmt.Sprintf("a.tags && $%d::text[]", paramIdx),
			args: []interface{}{pq.Array(filter.Tags)},
		})
		paramIdx++
	}

	// Rating range filters
	if filter.MinRating > 0 {
		clauses = append(clauses, searchClause{
			cond: fmt.Sprintf("a.rating_avg >= $%d", paramIdx),
			args: []interface{}{float64(filter.MinRating)},
		})
		paramIdx++
	}
	if filter.MaxRating > 0 {
		clauses = append(clauses, searchClause{
			cond: fmt.Sprintf("a.rating_avg <= $%d", paramIdx),
			args: []interface{}{float64(filter.MaxRating)},
		})
		paramIdx++
	}

	// Pricing type filter
	if filter.PricingType != "" {
		clauses = append(clauses, searchClause{
			cond: fmt.Sprintf("a.pricing_type = $%d", paramIdx),
			args: []interface{}{filter.PricingType},
		})
		paramIdx++
	}

	// Build ORDER BY clause
	orderBy := buildOrderBy(filter.Sort, recommendationSignals(hasQuery, filter.CategorySlug != "", len(filter.Tags) > 0))

	// Build args list
	var args []interface{}
	for _, c := range clauses {
		args = append(args, c.args...)
	}

	// Build WHERE string
	var conds []string
	for _, c := range clauses {
		conds = append(conds, c.cond)
	}
	whereClause := strings.Join(conds, " AND ")

	// Count query (no FTS rank, no ORDER BY, no LIMIT)
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM published_agents a
		LEFT JOIN categories c ON a.category_id = c.id
		WHERE %s
	`, whereClause)

	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("search agents: count: %w", err)
	}

	// Data query
	dataQuery := fmt.Sprintf(`
		SELECT %s
		FROM published_agents a
		LEFT JOIN categories c ON a.category_id = c.id
		LEFT JOIN users u ON a.owner_id = u.id
		LEFT JOIN marketplace_agent_ranking_signals mars ON mars.agent_id = a.id
		WHERE %s
		%s
		LIMIT $%d OFFSET $%d
	`, selectAgentColumns, whereClause, orderBy, paramIdx, paramIdx+1)

	dataArgs := append(args, limit, offset)

	// Execute data query
	rows, err := s.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("search agents: query: %w", err)
	}
	defer rows.Close()

	var agents []*PublishedAgent
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("search agents: scan: %w", err)
		}
		agents = append(agents, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("search agents: rows: %w", err)
	}

	addRecommendationMetadata(agents, filter)

	return agents, total, nil
}

func addRecommendationMetadata(agents []*PublishedAgent, filter MarketplaceSearchFilter) {
	if filter.Sort != "recommended" {
		return
	}
	for _, agent := range agents {
		if agent == nil {
			continue
		}
		score, reasons := recommendationMetadataForAgent(agent, filter)
		agent.Recommendation = &RecommendationMetadata{
			Score:  score,
			Reason: strings.Join(reasons, "; "),
		}
	}
}

func recommendationMetadataForAgent(agent *PublishedAgent, filter MarketplaceSearchFilter) (float64, []string) {
	var score float64
	var reasons []string

	query := strings.TrimSpace(filter.Query)
	if query != "" && containsFold(agent.Name+" "+agent.Description, query) {
		score += 0.30
		reasons = append(reasons, fmt.Sprintf("Matches %q", query))
	}

	category := strings.TrimSpace(firstNonEmptyString(agent.CategoryName, agent.CategoryID))
	if strings.TrimSpace(filter.CategorySlug) != "" && category != "" {
		score += 0.15
		reasons = append(reasons, fmt.Sprintf("%s category", category))
	}

	if len(filter.Tags) > 0 {
		matchedTags := matchingTags(agent.Tags, filter.Tags)
		if len(matchedTags) > 0 {
			score += 0.15
			reasons = append(reasons, strings.Join(matchedTags, ", ")+" tag")
		}
	}

	if agent.RatingAvg > 0 {
		ratingScore := agent.RatingAvg / 5.0 * 0.25
		score += ratingScore
		reasons = append(reasons, fmt.Sprintf("%.1f rating", agent.RatingAvg))
	}

	if agent.InstallCount > 0 {
		installScore := float64(agent.InstallCount)
		if installScore > 100 {
			installScore = 100
		}
		score += installScore / 100 * 0.15
		reasons = append(reasons, fmt.Sprintf("%d installs", agent.InstallCount))
	}

	if len(reasons) == 0 {
		score = 0.10
		reasons = append(reasons, "Exploration pick")
	}
	if score > 1 {
		score = 1
	}
	return score, reasons
}

func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func matchingTags(agentTags []string, filterTags []string) []string {
	var matches []string
	seen := map[string]bool{}
	for _, filterTag := range filterTags {
		normalizedFilter := strings.ToLower(strings.TrimSpace(filterTag))
		if normalizedFilter == "" || seen[normalizedFilter] {
			continue
		}
		for _, agentTag := range agentTags {
			if strings.ToLower(strings.TrimSpace(agentTag)) == normalizedFilter {
				matches = append(matches, strings.TrimSpace(agentTag))
				seen[normalizedFilter] = true
				break
			}
		}
	}
	return matches
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func recommendationSignals(hasQuery, hasCategory, hasTags bool) string {
	var signals []string
	if hasQuery {
		signals = append(signals, "rank")
	}
	if hasCategory {
		signals = append(signals, "category_match")
	}
	if hasTags {
		signals = append(signals, "tag_match")
	}
	return strings.Join(signals, ",")
}

// buildOrderBy returns the SQL ORDER BY clause for the given sort option (D-29, D-30).
func buildOrderBy(sort string, signals string) string {
	switch sort {
	case "popular", "installs":
		return "ORDER BY a.install_count DESC, a.rating_avg DESC, a.id ASC"
	case "rating":
		return "ORDER BY a.rating_avg DESC, a.rating_count DESC, a.id ASC"
	case "newest":
		return "ORDER BY a.created_at DESC, a.id ASC"
	case "recommended":
		rankScore := "0"
		if strings.Contains(signals, "rank") {
			rankScore = "ts_rank(to_tsvector('english', a.name || ' ' || a.description), plainto_tsquery('english', $1))"
		}
		categoryScore := "0"
		if strings.Contains(signals, "category_match") {
			categoryScore = "1"
		}
		tagScore := "0"
		if strings.Contains(signals, "tag_match") {
			tagScore = "1"
		}

		// Hybrid recommendation score:
		// hot score (installs + rating), content score (query/category/tag),
		// marketplace_agent_ranking_signals operational score, deterministic
		// exploration score, and stable fallback tie breakers.
		return `ORDER BY (
			(
				/* hot_score */ (
					(COALESCE(a.rating_avg, 0)::float / 5.0) * 0.45 +
					(COALESCE(a.install_count, 0)::float / GREATEST((SELECT COALESCE(MAX(install_count), 0) FROM published_agents WHERE status = 'approved' AND visibility = 'public'), 1)) * 0.55
				) * 0.45 +
				/* content_score rank category_match tag_match */ (
					(` + rankScore + `) * 0.60 +
					(` + categoryScore + `) * 0.20 +
					(` + tagScore + `) * 0.20
				) * 0.30 +
				/* operational_score from marketplace_agent_ranking_signals: click_count/impression_count and install_conversion_count/impression_count */ (
					(LEAST(COALESCE(mars.click_count, 0)::float / GREATEST(COALESCE(mars.impression_count, 0), 1), 1.0)) * 0.45 +
					(LEAST(COALESCE(mars.install_conversion_count, 0)::float / GREATEST(COALESCE(mars.impression_count, 0), 1), 1.0)) * 0.55
				) * 0.15 +
				/* exploration_score: deterministic 10% exploration, no runtime RNG needed. */ (
					(('x' || substr(md5(a.id), 1, 8))::bit(32)::bigint)::float / 4294967295.0
				) * 0.10
			) * GREATEST(COALESCE(mars.curated_weight, 1.0), 0.0) * GREATEST(COALESCE(mars.governance_weight, 1.0), 0.0)
		) DESC, a.rating_avg DESC, a.install_count DESC, a.id ASC`
	default:
		// Default: relevance (FTS rank) then rating
		if strings.Contains(signals, "rank") {
			return "ORDER BY ts_rank(to_tsvector('english', a.name || ' ' || a.description), plainto_tsquery('english', $1)) DESC, a.rating_avg DESC, a.id ASC"
		}
		return "ORDER BY a.rating_avg DESC, a.rating_count DESC, a.id ASC"
	}
}
