package http

import (
	"context"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"oblivious/server/internal/observability"
)

type observabilityAlertHandler struct {
	store         observability.AlertStateStore
	routingStore  observability.AlertRoutingRuleStore
	providerStore observability.AlertProviderConfigStore
	now           func() time.Time
}

type latencySLOProof struct {
	LatencySLOTrigger        string                       `json:"latencySLOTrigger"`
	LatencySLOAlertDelivery  string                       `json:"latencySLOAlertDelivery"`
	LatencySLORecoveryAction string                       `json:"latencySLORecoveryAction"`
	Window                   string                       `json:"window"`
	TriggeredAlerts          int                          `json:"triggeredAlerts"`
	AlertDelivery            latencySLOAlertDeliveryProof `json:"alertDelivery"`
	RecoveryAudit            latencySLORecoveryAuditProof `json:"recoveryAudit"`
}

type latencySLOAlertDeliveryProof struct {
	ConfiguredProviders int      `json:"configuredProviders"`
	DeliveredAlerts     int      `json:"deliveredAlerts"`
	FailedDeliveries    int      `json:"failedDeliveries"`
	Channels            []string `json:"channels"`
	LastDeliveryID      string   `json:"lastDeliveryId"`
}

type latencySLORecoveryAuditProof struct {
	AuditRecords  int    `json:"auditRecords"`
	FailedActions int    `json:"failedActions"`
	LastRecordID  string `json:"lastRecordId"`
}

func newObservabilityAlertHandler(store observability.AlertStateStore, routingStores ...observability.AlertRoutingRuleStore) observabilityAlertHandler {
	var routingStore observability.AlertRoutingRuleStore
	if len(routingStores) > 0 {
		routingStore = routingStores[0]
	}
	return newObservabilityAlertHandlerWithStores(store, routingStore, nil)
}

func newObservabilityAlertHandlerWithStores(
	store observability.AlertStateStore,
	routingStore observability.AlertRoutingRuleStore,
	providerStore observability.AlertProviderConfigStore,
) observabilityAlertHandler {
	if routingStore == nil {
		routingStore = observability.NewInMemoryAlertRoutingRuleStore(observability.DefaultAlertRoutingRules())
	}
	if providerStore == nil {
		providerStore = observability.NewInMemoryAlertProviderConfigStore()
	}
	return observabilityAlertHandler{
		store:         store,
		routingStore:  routingStore,
		providerStore: providerStore,
		now:           func() time.Time { return time.Now().UTC() },
	}
}

func (h observabilityAlertHandler) getAlertRoutingRules(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if h.routingStore == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "observability_unavailable", "observability alert routing rule store is unavailable")
		return
	}
	rules, err := h.routingStore.GetRoutingRules(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, rules)
}

func (h observabilityAlertHandler) updateAlertRoutingRules(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if h.routingStore == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "observability_unavailable", "observability alert routing rule store is unavailable")
		return
	}
	var request struct {
		Rules observability.AlertRoutingRules `json:"rules"`
	}
	if !decodeObservabilityAlertJSON(w, r, &request) {
		return
	}
	rules, err := h.routingStore.UpdateRoutingRules(r.Context(), request.Rules)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, rules)
}

type alertProviderConfigRequest struct {
	Kind   observability.AlertProviderKind   `json:"kind"`
	Name   string                            `json:"name"`
	Status observability.AlertProviderStatus `json:"status"`
	Config map[string]string                 `json:"config"`
}

func (h observabilityAlertHandler) listAlertProviderConfigs(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if h.providerStore == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "observability_unavailable", "observability alert provider config store is unavailable")
		return
	}
	configs, err := h.providerStore.ListAlertProviderConfigs(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, observability.AlertProviderConfigsToViews(configs))
}

func (h observabilityAlertHandler) createAlertProviderConfig(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if h.providerStore == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "observability_unavailable", "observability alert provider config store is unavailable")
		return
	}
	var request alertProviderConfigRequest
	if !decodeObservabilityAlertJSON(w, r, &request) {
		return
	}
	config, err := h.providerStore.SaveAlertProviderConfig(r.Context(), observability.AlertProviderConfig{
		ID:     "alert_provider_" + uuid.NewString(),
		Kind:   request.Kind,
		Name:   request.Name,
		Status: request.Status,
		Config: request.Config,
	})
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusCreated, observability.AlertProviderConfigToView(config))
}

func (h observabilityAlertHandler) updateAlertProviderConfig(w stdhttp.ResponseWriter, r *stdhttp.Request, id string) {
	if h.providerStore == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "observability_unavailable", "observability alert provider config store is unavailable")
		return
	}
	existing, ok, err := h.providerStore.GetAlertProviderConfig(r.Context(), id)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if !ok {
		writeError(w, stdhttp.StatusNotFound, "not_found", "alert provider config not found")
		return
	}
	var request alertProviderConfigRequest
	if !decodeObservabilityAlertJSON(w, r, &request) {
		return
	}
	if request.Kind == "" {
		request.Kind = existing.Kind
	}
	if strings.TrimSpace(request.Name) == "" {
		request.Name = existing.Name
	}
	if request.Status == "" {
		request.Status = existing.Status
	}
	config, err := h.providerStore.SaveAlertProviderConfig(r.Context(), observability.AlertProviderConfig{
		ID:     existing.ID,
		Kind:   request.Kind,
		Name:   request.Name,
		Status: request.Status,
		Config: mergeAlertProviderConfigUpdate(existing.Config, request.Config),
	})
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, observability.AlertProviderConfigToView(config))
}

func (h observabilityAlertHandler) testAlertProviderConfig(w stdhttp.ResponseWriter, r *stdhttp.Request, id string) {
	if h.providerStore == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "observability_unavailable", "observability alert provider config store is unavailable")
		return
	}
	config, ok, err := h.providerStore.GetAlertProviderConfig(r.Context(), id)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if !ok {
		writeError(w, stdhttp.StatusNotFound, "not_found", "alert provider config not found")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, observability.ProbeAlertProviderConfig(r.Context(), config, h.currentTime(), nil))
}

func (h observabilityAlertHandler) listAlerts(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if h.store == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "observability_unavailable", "observability alert state store is unavailable")
		return
	}
	filter := observability.AlertStateFilter{
		Status:    observability.AlertStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
		Severity:  observability.AlertSeverity(strings.TrimSpace(r.URL.Query().Get("severity"))),
		Component: strings.TrimSpace(r.URL.Query().Get("component")),
		KeyPrefix: strings.TrimSpace(r.URL.Query().Get("keyPrefix")),
		From:      parseOptionalObservabilityAlertTimeQuery(r, "from"),
		To:        parseOptionalObservabilityAlertTimeQuery(r, "to"),
		Limit:     parseObservabilityAlertQueryInt(r, "limit", 50, 100),
		Offset:    parseObservabilityAlertQueryInt(r, "offset", 0, 0),
	}
	states, err := h.store.ListAlertStates(r.Context(), filter)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, states)
}

func (h observabilityAlertHandler) listRecoveryActions(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if h.store == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "observability_unavailable", "observability alert state store is unavailable")
		return
	}
	actions, err := h.store.ListRecoveryActions(r.Context(), observability.RecoveryActionFilter{
		AlertKey:   strings.TrimSpace(r.URL.Query().Get("alertKey")),
		KeyPrefix:  strings.TrimSpace(r.URL.Query().Get("keyPrefix")),
		PolicyName: strings.TrimSpace(r.URL.Query().Get("policyName")),
		Component:  strings.TrimSpace(r.URL.Query().Get("component")),
		Type:       observability.RecoveryActionType(strings.TrimSpace(r.URL.Query().Get("type"))),
		From:       parseOptionalObservabilityAlertTimeQuery(r, "from"),
		To:         parseOptionalObservabilityAlertTimeQuery(r, "to"),
		Limit:      parseObservabilityAlertQueryInt(r, "limit", 50, 100),
		Offset:     parseObservabilityAlertQueryInt(r, "offset", 0, 0),
	})
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, actions)
}

func (h observabilityAlertHandler) getAlert(w stdhttp.ResponseWriter, r *stdhttp.Request, key string) {
	if h.store == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "observability_unavailable", "observability alert state store is unavailable")
		return
	}
	state, ok, err := h.store.GetAlertState(r.Context(), strings.TrimSpace(key))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if !ok {
		writeError(w, stdhttp.StatusNotFound, "not_found", "alert state not found")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, state)
}

func (h observabilityAlertHandler) acknowledgeAlert(w stdhttp.ResponseWriter, r *stdhttp.Request, key string) {
	h.updateAlertState(w, r, key, h.store.AcknowledgeAlert)
}

func (h observabilityAlertHandler) resolveAlert(w stdhttp.ResponseWriter, r *stdhttp.Request, key string) {
	h.updateAlertState(w, r, key, h.store.ResolveAlert)
}

func (h observabilityAlertHandler) listDeliveryAttempts(w stdhttp.ResponseWriter, r *stdhttp.Request, key string) {
	if h.store == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "observability_unavailable", "observability alert state store is unavailable")
		return
	}
	attempts, err := h.store.ListDeliveryAttempts(r.Context(), observability.AlertDeliveryHistoryFilter{
		AlertKey: strings.TrimSpace(key),
		From:     parseOptionalObservabilityAlertTimeQuery(r, "from"),
		To:       parseOptionalObservabilityAlertTimeQuery(r, "to"),
		Limit:    parseObservabilityAlertQueryInt(r, "limit", 50, 100),
		Offset:   parseObservabilityAlertQueryInt(r, "offset", 0, 0),
	})
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, attempts)
}

func (h observabilityAlertHandler) getLatencySLOProof(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if h.store == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "observability_unavailable", "observability alert state store is unavailable")
		return
	}
	from, ok := parseObservabilityAlertTimeQuery(w, r, "from")
	if !ok {
		return
	}
	to, ok := parseObservabilityAlertTimeQuery(w, r, "to")
	if !ok {
		return
	}
	if to.Before(from) {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "to must be at or after from")
		return
	}
	keyPrefix := strings.TrimSpace(r.URL.Query().Get("keyPrefix"))
	if keyPrefix == "" {
		keyPrefix = "http-slo:"
	}

	proof, err := h.buildLatencySLOProof(r.Context(), from, to, keyPrefix)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, proof)
}

func (h observabilityAlertHandler) buildLatencySLOProof(ctx context.Context, from, to time.Time, keyPrefix string) (latencySLOProof, error) {
	states, err := h.store.ListAlertStates(ctx, observability.AlertStateFilter{
		KeyPrefix: keyPrefix,
		From:      from,
		To:        to,
	})
	if err != nil {
		return latencySLOProof{}, err
	}
	attempts, err := h.store.ListDeliveryAttempts(ctx, observability.AlertDeliveryHistoryFilter{
		KeyPrefix: keyPrefix,
		From:      from,
		To:        to,
	})
	if err != nil {
		return latencySLOProof{}, err
	}
	actions, err := h.store.ListRecoveryActions(ctx, observability.RecoveryActionFilter{
		KeyPrefix: keyPrefix,
		From:      from,
		To:        to,
	})
	if err != nil {
		return latencySLOProof{}, err
	}

	triggeredAlerts := len(states)
	deliveryProof := buildLatencySLOAlertDeliveryProof(attempts)
	recoveryProof := buildLatencySLORecoveryAuditProof(actions)

	return latencySLOProof{
		LatencySLOTrigger:        observabilityProofPassFail(triggeredAlerts > 0),
		LatencySLOAlertDelivery:  observabilityProofPassFail(deliveryProof.ConfiguredProviders > 0 && deliveryProof.DeliveredAlerts >= triggeredAlerts && deliveryProof.FailedDeliveries == 0),
		LatencySLORecoveryAction: observabilityProofPassFail(recoveryProof.AuditRecords >= triggeredAlerts && recoveryProof.FailedActions == 0),
		Window:                   from.UTC().Format(time.RFC3339Nano) + "/" + to.UTC().Format(time.RFC3339Nano),
		TriggeredAlerts:          triggeredAlerts,
		AlertDelivery:            deliveryProof,
		RecoveryAudit:            recoveryProof,
	}, nil
}

func buildLatencySLOAlertDeliveryProof(attempts []observability.AlertDeliveryAttempt) latencySLOAlertDeliveryProof {
	providers := map[string]struct{}{}
	deliveredChannels := map[string]struct{}{}
	channels := map[string]struct{}{}
	proof := latencySLOAlertDeliveryProof{Channels: []string{}}
	for _, attempt := range attempts {
		channel := strings.TrimSpace(string(attempt.Channel))
		if channel != "" {
			channels[channel] = struct{}{}
		}
		providerKey := strings.TrimSpace(attempt.ProviderID)
		if providerKey == "" && attempt.ProviderKind != "" {
			providerKey = strings.TrimSpace(string(attempt.ProviderKind)) + ":" + channel
		}
		if providerKey != "" {
			providers[providerKey] = struct{}{}
		}
		if attempt.Delivered {
			proof.DeliveredAlerts++
			if channel != "" {
				deliveredChannels[channel] = struct{}{}
			}
		} else {
			proof.FailedDeliveries++
		}
		if proof.LastDeliveryID == "" {
			proof.LastDeliveryID = attempt.ID
		}
	}
	proof.ConfiguredProviders = len(providers)
	if proof.ConfiguredProviders == 0 {
		proof.ConfiguredProviders = len(deliveredChannels)
	}
	for channel := range channels {
		proof.Channels = append(proof.Channels, channel)
	}
	sort.Strings(proof.Channels)
	return proof
}

func buildLatencySLORecoveryAuditProof(actions []observability.RecoveryAction) latencySLORecoveryAuditProof {
	proof := latencySLORecoveryAuditProof{AuditRecords: len(actions)}
	for _, action := range actions {
		if action.Status != observability.RecoveryActionRecorded {
			proof.FailedActions++
		}
		if proof.LastRecordID == "" {
			proof.LastRecordID = action.ID
		}
	}
	return proof
}

func observabilityProofPassFail(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

func (h observabilityAlertHandler) updateAlertState(
	w stdhttp.ResponseWriter,
	r *stdhttp.Request,
	key string,
	update func(ctx context.Context, key string, at time.Time) (observability.AlertState, error),
) {
	if h.store == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "observability_unavailable", "observability alert state store is unavailable")
		return
	}
	state, err := update(r.Context(), strings.TrimSpace(key), h.currentTime())
	if err != nil {
		if errors.Is(err, context.Canceled) {
			writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		if strings.Contains(err.Error(), "alert state not found") {
			writeError(w, stdhttp.StatusNotFound, "not_found", "alert state not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, state)
}

func mergeAlertProviderConfigUpdate(existing map[string]string, incoming map[string]string) map[string]string {
	merged := make(map[string]string, len(existing)+len(incoming))
	for key, value := range existing {
		merged[key] = value
	}
	for key, value := range incoming {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		trimmedValue := strings.TrimSpace(value)
		if observability.IsAlertProviderSecretConfigKey(trimmedKey) &&
			(trimmedValue == "" || trimmedValue == observability.RedactedAlertProviderSecret) {
			continue
		}
		merged[trimmedKey] = trimmedValue
	}
	return merged
}

func (h observabilityAlertHandler) currentTime() time.Time {
	if h.now == nil {
		return time.Now().UTC()
	}
	return h.now()
}

func decodeObservabilityAlertJSON(w stdhttp.ResponseWriter, r *stdhttp.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return false
	}
	return true
}

func parseObservabilityAlertQueryInt(r *stdhttp.Request, key string, defaultValue int, maxValue int) int {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	if parsed < 0 {
		return defaultValue
	}
	if maxValue > 0 && parsed > maxValue {
		return maxValue
	}
	return parsed
}

func parseObservabilityAlertTimeQuery(w stdhttp.ResponseWriter, r *stdhttp.Request, key string) (time.Time, bool) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", key+" is required")
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", key+" must be ISO-8601")
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func parseOptionalObservabilityAlertTimeQuery(r *stdhttp.Request, key string) time.Time {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}
