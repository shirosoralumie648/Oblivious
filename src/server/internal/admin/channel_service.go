package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"oblivious/server/internal/auth"
)

// ListChannels returns channels for admin display, applying limit bounds.
func (s *Service) ListChannels(ctx context.Context, filter ChannelFilter) ([]*ChannelInfo, error) {
	if filter.Limit < 1 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	return s.store.ListChannels(ctx, filter)
}

// GetChannel returns a single channel by ID.
func (s *Service) GetChannel(ctx context.Context, id string) (*ChannelInfo, error) {
	if id == "" {
		return nil, fmt.Errorf("channel id is required")
	}
	return s.store.GetChannel(ctx, id)
}

// CreateChannel creates a new channel and records an audit entry.
func (s *Service) CreateChannel(ctx context.Context, actor auth.Session, input ChannelCreateRequest, r *http.Request) (*ChannelInfo, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("channel name is required")
	}
	if input.Provider == "" {
		return nil, fmt.Errorf("channel provider is required")
	}

	result, err := s.store.CreateChannel(ctx, input)
	if err != nil {
		return nil, err
	}

	changes, _ := json.Marshal(input)
	ip := extractIP(r)
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "channel.create", "channel", result.ID, string(changes), ip)

	return result, nil
}

// UpdateChannel updates a channel and records an audit entry.
func (s *Service) UpdateChannel(ctx context.Context, actor auth.Session, id string, input ChannelUpdateRequest, r *http.Request) (*ChannelInfo, error) {
	if id == "" {
		return nil, fmt.Errorf("channel id is required")
	}

	result, err := s.store.UpdateChannel(ctx, id, input)
	if err != nil {
		return nil, err
	}

	changes, _ := json.Marshal(input)
	ip := extractIP(r)
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "channel.update", "channel", id, string(changes), ip)

	return result, nil
}

// DeleteChannel deletes a channel and records an audit entry.
func (s *Service) DeleteChannel(ctx context.Context, actor auth.Session, id string, r *http.Request) error {
	if err := s.store.DeleteChannel(ctx, id); err != nil {
		return err
	}

	ip := extractIP(r)
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "channel.delete", "channel", id, "", ip)
	return nil
}

// TestChannel performs a connectivity test on the channel.
func (s *Service) TestChannel(ctx context.Context, id string) (*ChannelTestResult, error) {
	return s.store.TestChannel(ctx, id)
}

// BatchUpdateChannels enables or disables multiple channels and records an audit entry.
func (s *Service) BatchUpdateChannels(ctx context.Context, actor auth.Session, ids []string, action string, r *http.Request) error {
	if action != "enable" && action != "disable" {
		return fmt.Errorf("action must be 'enable' or 'disable'")
	}

	if err := s.store.BatchUpdateChannels(ctx, ids, action); err != nil {
		return err
	}

	changes, _ := json.Marshal(map[string]interface{}{"ids": ids, "action": action})
	ip := extractIP(r)
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "channel.batch_"+action, "channel", "", string(changes), ip)
	return nil
}

// extractIP extracts the client IP address from an HTTP request.
func extractIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	// Check X-Forwarded-For header first (for proxied requests)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
