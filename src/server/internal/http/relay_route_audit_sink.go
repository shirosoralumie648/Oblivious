package http

import (
	"context"
	"time"

	"oblivious/server/internal/observability"
	"oblivious/server/internal/relay/handler"
)

type relayRouteAuditRequestLogSink struct {
	sink observability.RequestLogSink
}

func newRelayRouteAuditRequestLogSink(sink observability.RequestLogSink) handler.RouteAuditSink {
	if sink == nil {
		return nil
	}
	return relayRouteAuditRequestLogSink{sink: sink}
}

func (s relayRouteAuditRequestLogSink) RecordRelayRouteDecision(ctx context.Context, event handler.RouteAuditEvent) {
	if s.sink == nil {
		return
	}
	status := 200
	if event.Result == handler.RouteAuditResultRejected {
		status = 403
	}
	timestamp := event.CreatedAt
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	if err := observability.WriteRequestLog(ctx, s.sink, observability.Event{
		Component:       "relay",
		Event:           "relay.route_decision",
		RequestID:       event.RequestID,
		OrganizationID:  event.OrganizationID,
		UserID:          event.UserID,
		Method:          event.Method,
		Route:           event.Path,
		Status:          status,
		RelayRouteClass: string(event.Class),
		RelayAPIType:    event.APIType.String(),
		FailureReason:   event.FailureReason,
		Fields: map[string]any{
			"relay_route_result": string(event.Result),
		},
	}, timestamp); err != nil {
		routeRequestLogSinkAlert(ctx, event.Method, event.Path, timestamp, event.RequestID, err)
	}
}
