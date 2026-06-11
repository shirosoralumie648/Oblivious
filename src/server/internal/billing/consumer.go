package billing

import (
	"context"
	"log"

	"google.golang.org/protobuf/proto"
	eventpb "oblivious/server/pkg/event/proto"
	"oblivious/server/pkg/event"
)

type Consumer struct {
	consumer *event.Consumer
	service  *Service
}

func NewConsumer(brokers []string, topic, groupID string, service *Service) *Consumer {
	return &Consumer{
		consumer: event.NewConsumer(brokers, topic, groupID),
		service:  service,
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	return c.consumer.Subscribe(ctx, func(key string, msg []byte) error {
		var evt eventpb.BillingEvent
		if err := proto.Unmarshal(msg, &evt); err != nil {
			log.Printf("unmarshal error: %v", err)
			return err
		}
		return c.handleBillingEvent(ctx, &evt)
	})
}

func (c *Consumer) handleBillingEvent(ctx context.Context, evt *eventpb.BillingEvent) error {
	_, err := c.service.HandleUsageEvent(
		ctx,
		evt.UserId,
		evt.OrgId,
		"",
		"",
		evt.FeatureType,
		int(evt.TokenCount),
		0,
		0,
		0,
		"",
	)
	return err
}

func (c *Consumer) Close() error {
	return c.consumer.Close()
}
