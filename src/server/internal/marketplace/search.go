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
	ftsCond := ""
	if strings.TrimSpace(filter.Query) != "" {
		ftsCond = fmt.Sprintf("AND to_tsvector('english', a.name || ' ' || a.description) @@ plainto_tsquery('english', $%d)", paramIdx)
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
	orderBy := buildOrderBy(filter.Sort, ftsCond)

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
	selectCols := selectAgentColumns
	if ftsCond != "" {
		selectCols = selectAgentColumns + `, ts_rank(to_tsvector('english', a.name || ' ' || a.description), plainto_tsquery('english', $1)) AS rank`
	}

	dataQuery := fmt.Sprintf(`
		SELECT %s
		FROM published_agents a
		LEFT JOIN categories c ON a.category_id = c.id
		LEFT JOIN users u ON a.owner_id = u.id
		WHERE %s
		%s
		LIMIT $%d OFFSET $%d
	`, selectCols, whereClause, orderBy, paramIdx, paramIdx+1)

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

	return agents, total, nil
}

// buildOrderBy returns the SQL ORDER BY clause for the given sort option (D-29, D-30).
func buildOrderBy(sort string, ftsCond string) string {
	switch sort {
	case "popular":
		return "ORDER BY a.install_count DESC, a.rating_avg DESC"
	case "rating":
		return "ORDER BY a.rating_avg DESC, a.rating_count DESC"
	case "newest":
		return "ORDER BY a.created_at DESC"
	case "recommended":
		// D-29: Algorithmic composite score
		// rating*0.4 + installs_norm*0.3 + recency*0.3
		return `ORDER BY (
			a.rating_avg * 0.4 +
			(a.install_count::float / NULLIF((SELECT MAX(install_count) FROM published_agents WHERE status='approved' AND visibility='public'), 0)) * 0.3 +
			(EXTRACT(EPOCH FROM a.created_at) / EXTRACT(EPOCH FROM (SELECT MAX(created_at) FROM published_agents WHERE status='approved' AND visibility='public'))) * 0.3
		) DESC`
	default:
		// Default: relevance (FTS rank) then rating
		if ftsCond != "" {
			return "ORDER BY rank DESC, a.rating_avg DESC"
		}
		return "ORDER BY a.rating_avg DESC, a.rating_count DESC"
	}
}
