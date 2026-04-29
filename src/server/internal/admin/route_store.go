package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/lib/pq"

	"oblivious/server/internal/auth"
)

// RouteStore defines operations on model routes.
type RouteStore interface {
	ListRoutes(ctx context.Context) ([]*RouteInfo, error)
	GetRoute(ctx context.Context, id string) (*RouteInfo, error)
	CreateRoute(ctx context.Context, input RouteCreateRequest) (*RouteInfo, error)
	UpdateRoute(ctx context.Context, id string, input RouteUpdateRequest) (*RouteInfo, error)
	DeleteRoute(ctx context.Context, id string) error
}

// ListRoutes returns all model routes with their channel weights.
func (s *SQLStore) ListRoutes(ctx context.Context) ([]*RouteInfo, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.model, r.strategy, r.created_at,
		       COALESCE(
		         json_agg(
		           json_build_object(
		             'channelID', w.channel_id,
		             'channelName', COALESCE(c.name, ''),
		             'weight', COALESCE(w.weight, 100),
		             'priority', COALESCE(w.priority, 0),
		             'enabled', COALESCE(w.enabled, true)
		           ) ORDER BY w.priority DESC
		         ) FILTER (WHERE w.id IS NOT NULL),
		         '[]'::json
		       ) AS channels_json
		FROM model_routes r
		LEFT JOIN model_channel_weights w ON r.id = w.route_id
		LEFT JOIN channels c ON w.channel_id = c.id
		GROUP BY r.id
		ORDER BY r.created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	defer rows.Close()

	return scanRoutes(rows)
}

// GetRoute returns a single model route by ID with its channel weights.
func (s *SQLStore) GetRoute(ctx context.Context, id string) (*RouteInfo, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT r.id, r.model, r.strategy, r.created_at,
		       COALESCE(
		         json_agg(
		           json_build_object(
		             'channelID', w.channel_id,
		             'channelName', COALESCE(c.name, ''),
		             'weight', COALESCE(w.weight, 100),
		             'priority', COALESCE(w.priority, 0),
		             'enabled', COALESCE(w.enabled, true)
		           ) ORDER BY w.priority DESC
		         ) FILTER (WHERE w.id IS NOT NULL),
		         '[]'::json
		       ) AS channels_json
		FROM model_routes r
		LEFT JOIN model_channel_weights w ON r.id = w.route_id
		LEFT JOIN channels c ON w.channel_id = c.id
		WHERE r.id = $1
		GROUP BY r.id
	`, id)

	var route RouteInfo
	var channelsJSON []byte
	if err := row.Scan(&route.ID, &route.Model, &route.Strategy, &route.CreatedAt, &channelsJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get route: %w", err)
	}
	if err := json.Unmarshal(channelsJSON, &route.Channels); err != nil {
		return nil, fmt.Errorf("unmarshal channels: %w", err)
	}
	return &route, nil
}

// CreateRoute creates a new model route with channel weights in a transaction.
func (s *SQLStore) CreateRoute(ctx context.Context, input RouteCreateRequest) (*RouteInfo, error) {
	routeID, err := auth.NewID("route")
	if err != nil {
		return nil, fmt.Errorf("generate route id: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	strategy := input.Strategy
	if strategy == "" {
		strategy = "weighted"
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO model_routes (id, model, strategy, created_at)
		VALUES ($1, $2, $3, NOW())
	`, routeID, input.Model, strategy)
	if err != nil {
		return nil, fmt.Errorf("insert route: %w", err)
	}

	for _, ch := range input.Channels {
		weightID, err := auth.NewID("rtw")
		if err != nil {
			return nil, fmt.Errorf("generate weight id: %w", err)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO model_channel_weights (id, route_id, channel_id, weight, priority, enabled)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, weightID, routeID, ch.ChannelID, ch.Weight, ch.Priority, ch.Enabled)
		if err != nil {
			return nil, fmt.Errorf("insert channel weight: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return s.GetRoute(ctx, routeID)
}

// UpdateRoute updates a model route and replaces its channel weights in a transaction.
func (s *SQLStore) UpdateRoute(ctx context.Context, id string, input RouteUpdateRequest) (*RouteInfo, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Update route fields if provided
	if input.Model != nil || input.Strategy != nil {
		_, err = tx.ExecContext(ctx, `
			UPDATE model_routes SET
				model = COALESCE(NULLIF($2::text,''), model),
				strategy = COALESCE(NULLIF($3::text,''), strategy)
			WHERE id = $1
		`, id, coalesceString(input.Model), coalesceString(input.Strategy))
		if err != nil {
			return nil, fmt.Errorf("update route: %w", err)
		}
	}

	// Replace channel weights if provided
	if input.Channels != nil {
		_, err = tx.ExecContext(ctx, `DELETE FROM model_channel_weights WHERE route_id = $1`, id)
		if err != nil {
			return nil, fmt.Errorf("delete old weights: %w", err)
		}
		for _, ch := range *input.Channels {
			weightID, err := auth.NewID("rtw")
			if err != nil {
				return nil, fmt.Errorf("generate weight id: %w", err)
			}
			_, err = tx.ExecContext(ctx, `
				INSERT INTO model_channel_weights (id, route_id, channel_id, weight, priority, enabled)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, weightID, id, ch.ChannelID, ch.Weight, ch.Priority, ch.Enabled)
			if err != nil {
				return nil, fmt.Errorf("insert channel weight: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return s.GetRoute(ctx, id)
}

// DeleteRoute deletes a model route and its channel weights via CASCADE.
func (s *SQLStore) DeleteRoute(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM model_routes WHERE id = $1`, id)
	return err
}

// scanRoutes is a helper that scans route rows from a *sql.Rows result.
func scanRoutes(rows *sql.Rows) ([]*RouteInfo, error) {
	var routes []*RouteInfo
	for rows.Next() {
		var route RouteInfo
		var channelsJSON []byte
		if err := rows.Scan(&route.ID, &route.Model, &route.Strategy, &route.CreatedAt, &channelsJSON); err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		if err := json.Unmarshal(channelsJSON, &route.Channels); err != nil {
			return nil, fmt.Errorf("unmarshal channels: %w", err)
		}
		routes = append(routes, &route)
	}
	return routes, rows.Err()
}

// Ensure pq is used (compile-time check for unused import in case scanRoutes moves).
var _ = pq.Array
