package admin

import (
	"context"
	"reflect"
	"testing"

	"oblivious/server/internal/auth"
)

func TestServiceUpdateRelayPricingSettingsPersistsAuditsAndApplies(t *testing.T) {
	store := &pricingSettingsStoreFake{}
	var applied RelayPricingSettings
	service := NewService(store, WithRelayPricingSettingsApplier(func(settings RelayPricingSettings) {
		applied = settings
	}))

	settings := RelayPricingSettings{
		ModelMultipliers: map[string]float64{
			"gpt-4o":      1.5,
			"gpt-4o-mini": 0.4,
		},
		GroupMultipliers: map[string]float64{
			"default": 1,
			"vip":     0.8,
		},
	}

	got, err := service.UpdateRelayPricingSettings(context.Background(), auth.Session{
		User: auth.User{
			ID:    "admin_1",
			Email: "admin@example.com",
		},
	}, settings, "127.0.0.1")
	if err != nil {
		t.Fatalf("update relay pricing settings: %v", err)
	}

	if got == nil || !reflect.DeepEqual(*got, settings) {
		t.Fatalf("returned settings = %#v, want %#v", got, settings)
	}
	if !reflect.DeepEqual(store.updated, settings) {
		t.Fatalf("persisted settings = %#v, want %#v", store.updated, settings)
	}
	if !reflect.DeepEqual(applied, settings) {
		t.Fatalf("applied settings = %#v, want %#v", applied, settings)
	}
	if store.audit == nil || store.audit.Action != "settings.relay_pricing.update" || store.audit.ResourceType != "settings" || store.audit.ResourceID != "relay_pricing" {
		t.Fatalf("expected relay pricing audit entry, got %#v", store.audit)
	}
}

func TestServiceUpdateRelayPricingSettingsRejectsNegativeMultipliers(t *testing.T) {
	store := &pricingSettingsStoreFake{}
	service := NewService(store)

	_, err := service.UpdateRelayPricingSettings(context.Background(), auth.Session{}, RelayPricingSettings{
		ModelMultipliers: map[string]float64{"gpt-4o": -1},
		GroupMultipliers: map[string]float64{"vip": 1},
	}, "")
	if err == nil {
		t.Fatalf("expected negative model multiplier to be rejected")
	}
	if store.updated.ModelMultipliers != nil || store.updated.GroupMultipliers != nil {
		t.Fatalf("settings should not be persisted after validation failure: %#v", store.updated)
	}
}

type pricingSettingsStoreFake struct {
	Store
	updated RelayPricingSettings
	audit   *AuditEntry
}

func (s *pricingSettingsStoreFake) GetRelayPricingSettings(ctx context.Context) (*RelayPricingSettings, error) {
	return &s.updated, nil
}

func (s *pricingSettingsStoreFake) UpdateRelayPricingSettings(ctx context.Context, settings RelayPricingSettings) (*RelayPricingSettings, error) {
	s.updated = settings
	return &settings, nil
}

func (s *pricingSettingsStoreFake) CreateAuditEntry(ctx context.Context, entry *AuditEntry) error {
	s.audit = entry
	return nil
}
