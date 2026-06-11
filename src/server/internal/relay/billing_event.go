package relay

import (
	"context"
	"time"

	eventpb "oblivious/server/pkg/event/proto"
	"oblivious/server/pkg/event"
)

type Service struct {
	billingProducer *event.Producer
}

func (s *Service) publishBillingEvent(ctx context.Context, orgID, userID string, tokens int, cost float64) {
	evt := &eventpb.BillingEvent{
		OrgId:       orgID,
		UserId:      userID,
		TokenCount:  int32(tokens),
		CostUsd:     cost,
		FeatureType: "llm_call",
		Timestamp:   time.Now().Unix(),
	}
	s.billingProducer.Publish(ctx, orgID, evt)
}
