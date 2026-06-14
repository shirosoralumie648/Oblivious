package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/types"
)

func TestProductionModelsRouteRequiresTrustedIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	modelsHandler := &countingHandler{}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeModels: modelsHandler,
	}, RouteRegistrationOptions{Production: true})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "relay_identity_required") {
		t.Fatalf("expected relay_identity_required response, got %s", rec.Body.String())
	}
	if modelsHandler.calls != 0 {
		t.Fatalf("unauthenticated models route reached handler %d times", modelsHandler.calls)
	}
}

func TestProductionModelsRouteReachesHandlerWithTrustedIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	modelsHandler := &countingHandler{}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeModels: modelsHandler,
	}, RouteRegistrationOptions{Production: true})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	addConfiguredTrustedRelayHeaders(t, req)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if modelsHandler.calls != 1 {
		t.Fatalf("trusted models route reached handler %d times, want 1", modelsHandler.calls)
	}
}

func TestModelsHandlerListsEnabledChannelModels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	pool := &fakeModelsPool{
		channels: []*types.Channel{
			{ID: "ch_1", Enabled: true, Models: []string{"gpt-4o", "text-embedding-3-small", "gpt-4o"}},
			{ID: "ch_2", Enabled: false, Models: []string{"disabled-model"}},
			{ID: "ch_3", Enabled: true, Models: []string{"gpt-4o-mini", ""}},
		},
	}
	var poolInterface types.ChannelPoolInterface = pool
	handler := NewModelsHandler(&poolInterface)
	engine := gin.New()
	engine.GET("/v1/models", func(c *gin.Context) {
		_ = handler.Handle(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	body := rec.Body.String()
	for _, model := range []string{"gpt-4o", "gpt-4o-mini", "text-embedding-3-small"} {
		if !strings.Contains(body, `"id":"`+model+`"`) {
			t.Fatalf("expected model %q in response, got %s", model, body)
		}
	}
	if strings.Contains(body, "disabled-model") {
		t.Fatalf("disabled channel model leaked in response: %s", body)
	}
	if strings.Count(body, `"id":"gpt-4o"`) != 1 {
		t.Fatalf("expected gpt-4o once after de-duplication, got %s", body)
	}
}

func TestModelsHandlerScopesModelsToTrustedOrganization(t *testing.T) {
	gin.SetMode(gin.TestMode)

	pool := &fakeModelsPool{
		channels: []*types.Channel{
			{ID: "ch_org_a", OrganizationID: "org_a", Enabled: true, Models: []string{"gpt-org-a", "shared-model"}},
			{ID: "ch_org_b", OrganizationID: "org_b", Enabled: true, Models: []string{"gpt-org-b", "shared-model"}},
			{ID: "ch_global", Enabled: true, Models: []string{"legacy-global"}},
		},
	}
	var poolInterface types.ChannelPoolInterface = pool
	handler := NewModelsHandler(&poolInterface)
	engine := gin.New()
	engine.GET("/v1/models", func(c *gin.Context) {
		_ = handler.Handle(c)
	})

	ctx := types.WithTrustedOrganizationID(context.Background(), "org_a")
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	body := rec.Body.String()
	for _, model := range []string{"gpt-org-a", "shared-model"} {
		if !strings.Contains(body, `"id":"`+model+`"`) {
			t.Fatalf("expected org_a model %q in response, got %s", model, body)
		}
	}
	for _, leaked := range []string{"gpt-org-b", "legacy-global"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("model %q from another scope leaked in response: %s", leaked, body)
		}
	}
}

type fakeModelsPool struct {
	channels []*types.Channel
}

func (p *fakeModelsPool) GetChannel(id string) (*types.Channel, bool) {
	return nil, false
}

func (p *fakeModelsPool) GetChannelsByModel(model string) []*types.RouteChannel {
	return nil
}

func (p *fakeModelsPool) GetStats(channelID string) (*types.ChannelStats, bool) {
	return nil, false
}

func (p *fakeModelsPool) UpdateChannel(ch *types.Channel) {}

func (p *fakeModelsPool) UpdateRoute(route *types.ModelRoute) {}

func (p *fakeModelsPool) ListChannels() []*types.Channel {
	return p.channels
}

func (p *fakeModelsPool) ListChannelsForOrganization(organizationID string) []*types.Channel {
	filtered := make([]*types.Channel, 0, len(p.channels))
	for _, ch := range p.channels {
		if ch != nil && ch.OrganizationID == organizationID {
			filtered = append(filtered, ch)
		}
	}
	return filtered
}

func (p *fakeModelsPool) SetChannelHealthy(channelID string, healthy bool) {}

func (p *fakeModelsPool) GetAllStats() map[string]*types.ChannelStats {
	return nil
}
