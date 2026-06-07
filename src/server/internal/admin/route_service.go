package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"oblivious/server/internal/auth"
)

// ListRoutes returns all model routes.
func (s *Service) ListRoutes(ctx context.Context) ([]*RouteInfo, error) {
	return s.store.ListRoutes(ctx)
}

// GetRoute returns a single route by ID.
func (s *Service) GetRoute(ctx context.Context, id string) (*RouteInfo, error) {
	if id == "" {
		return nil, fmt.Errorf("route id is required")
	}
	return s.store.GetRoute(ctx, id)
}

// CreateRoute creates a new model route and records an audit entry.
func (s *Service) CreateRoute(ctx context.Context, actor auth.Session, input RouteCreateRequest, r *http.Request) (*RouteInfo, error) {
	if input.Model == "" {
		return nil, fmt.Errorf("route model is required")
	}
	if len(input.Channels) == 0 {
		return nil, fmt.Errorf("at least one channel is required")
	}

	result, err := s.store.CreateRoute(ctx, input)
	if err != nil {
		return nil, err
	}
	if err := s.applyRelayConfigChange(ctx, RelayConfigChange{Kind: RelayConfigChangeRoute, Action: RelayConfigActionUpsert, ID: result.ID}); err != nil {
		return nil, err
	}

	changes, _ := json.Marshal(input)
	ip := extractIP(r)
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "route.create", "route", result.ID, string(changes), ip)

	return result, nil
}

// UpdateRoute updates a model route and records an audit entry.
func (s *Service) UpdateRoute(ctx context.Context, actor auth.Session, id string, input RouteUpdateRequest, r *http.Request) (*RouteInfo, error) {
	if id == "" {
		return nil, fmt.Errorf("route id is required")
	}

	result, err := s.store.UpdateRoute(ctx, id, input)
	if err != nil {
		return nil, err
	}
	if err := s.applyRelayConfigChange(ctx, RelayConfigChange{Kind: RelayConfigChangeRoute, Action: RelayConfigActionUpsert, ID: result.ID}); err != nil {
		return nil, err
	}

	changes, _ := json.Marshal(input)
	ip := extractIP(r)
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "route.update", "route", id, string(changes), ip)

	return result, nil
}

// DeleteRoute deletes a model route and records an audit entry.
func (s *Service) DeleteRoute(ctx context.Context, actor auth.Session, id string, r *http.Request) error {
	if err := s.store.DeleteRoute(ctx, id); err != nil {
		return err
	}
	if err := s.applyRelayConfigChange(ctx, RelayConfigChange{Kind: RelayConfigChangeRoute, Action: RelayConfigActionDelete, ID: id}); err != nil {
		return err
	}

	ip := extractIP(r)
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "route.delete", "route", id, "", ip)
	return nil
}
