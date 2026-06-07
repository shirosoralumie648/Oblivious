package channel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrRetryMessageStoreRequired = errors.New("retry message store is required")

const ChannelHealthThreshold = 3

type ReceiveRequest struct {
	ChannelID string
	Type      ChannelType
	Raw       json.RawMessage
	Now       time.Time
}

type SendRequest struct {
	ChannelID string
	Type      ChannelType
	Config    map[string]any
	Message   InternalMessage
	Now       time.Time
}

type ProcessDueRetryMessagesResult struct {
	Claimed           int
	Succeeded         int
	Failed            int
	PermanentFailures int
}

type RetryProcessResult = ProcessDueRetryMessagesResult

type ChannelHealthEvent struct {
	OrganizationID string
	ChannelID      string
	ChannelName    string
	ChannelType    ChannelType
	Status         ChannelStatus
	MessageLogID   string
	Reason         string
	OccurredAt     time.Time
}

type ChannelHealthNotifier func(ctx context.Context, event ChannelHealthEvent)

type Service struct {
	registry       *AdapterRegistry
	healthNotifier ChannelHealthNotifier
}

type outboundDeliverer interface {
	DeliverOutbound(ctx context.Context, config map[string]any, raw json.RawMessage) error
}

type ServiceOption func(*Service)

func WithChannelHealthNotifier(notifier ChannelHealthNotifier) ServiceOption {
	return func(s *Service) {
		s.healthNotifier = notifier
	}
}

func NewService(registry *AdapterRegistry) *Service {
	return NewServiceWithOptions(registry)
}

func NewServiceWithOptions(registry *AdapterRegistry, options ...ServiceOption) *Service {
	if registry == nil {
		registry = NewAdapterRegistry(nil)
	}
	service := &Service{registry: registry}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

func (s *Service) Receive(ctx context.Context, req ReceiveRequest) (ChannelMessageLog, error) {
	_ = ctx
	now := channelRequestTime(req.Now)
	log := ChannelMessageLog{
		ChannelID:  req.ChannelID,
		Direction:  DirectionInbound,
		RawMessage: append(json.RawMessage(nil), req.Raw...),
		Status:     MessageStatusRecorded,
		CreatedAt:  now,
	}
	adapter, err := s.adapter(req.Type)
	if err != nil {
		log.TransformError = err.Error()
		return log, nil
	}
	message, err := adapter.TransformInbound(req.Raw)
	if err != nil {
		log.TransformError = err.Error()
		return log, nil
	}
	log.ConversationID = message.ConversationID
	log.TransformedMessage = message
	log.TransformSuccess = true
	return log, nil
}

func (s *Service) Send(ctx context.Context, req SendRequest) (ChannelMessageLog, error) {
	now := channelRequestTime(req.Now)
	log := ChannelMessageLog{
		ChannelID:          req.ChannelID,
		ConversationID:     req.Message.ConversationID,
		Direction:          DirectionOutbound,
		TransformedMessage: req.Message,
		Status:             MessageStatusRecorded,
		CreatedAt:          now,
	}
	adapter, err := s.adapter(req.Type)
	if err != nil {
		log.TransformError = err.Error()
		return log, nil
	}
	raw, err := adapter.TransformOutbound(req.Message)
	if err != nil {
		log.RawMessage = auditableRawMessage(req.Message)
		log.TransformError = err.Error()
		return log, nil
	}
	log.RawMessage = append(json.RawMessage(nil), raw...)
	if deliverer, ok := adapter.(outboundDeliverer); ok {
		if err := deliverer.DeliverOutbound(ctx, req.Config, raw); err != nil {
			failed := s.MarkDeliveryFailed(log, err.Error(), now)
			return failed, nil
		}
	}
	log.TransformSuccess = true
	return log, nil
}

func (s *Service) Test(ctx context.Context, config ChannelConfig) (TestConnectionResult, error) {
	adapter, err := s.adapter(config.Type)
	if err != nil {
		return TestConnectionResult{}, err
	}
	result := TestConnectionResult{
		ChannelID: config.ID,
		Type:      string(config.Type),
		Status:    "success",
		Message:   "channel adapter is available",
	}
	if deliverer, ok := adapter.(outboundDeliverer); ok {
		if config.Type == ChannelTypeWebhook && webhookOutboundURL(config.Config) == "" {
			return TestConnectionResult{
				ChannelID: config.ID,
				Type:      string(config.Type),
				Status:    "failed",
				Message:   "webhook_url is required",
			}, nil
		}
		raw := json.RawMessage(`{"test":true}`)
		if err := deliverer.DeliverOutbound(ctx, config.Config, raw); err != nil {
			return TestConnectionResult{
				ChannelID: config.ID,
				Type:      string(config.Type),
				Status:    "failed",
				Message:   err.Error(),
			}, nil
		}
	}
	return result, nil
}

func (s *Service) MarkDeliveryFailed(log ChannelMessageLog, reason string, now time.Time) ChannelMessageLog {
	now = channelRequestTime(now)
	log.TransformSuccess = false
	log.TransformError = reason
	log.FailureReason = reason
	log.RetryCount++
	delay := NextRetryDelay(log.RetryCount)
	if delay <= 0 {
		log.Status = MessageStatusPermanentFailure
		log.NextRetryAt = nil
		return log
	}
	nextRetryAt := now.Add(delay).UTC()
	log.Status = MessageStatusRetryPending
	log.NextRetryAt = &nextRetryAt
	return log
}

func NextRetryDelay(retryCount int) time.Duration {
	switch retryCount {
	case 1:
		return time.Minute
	case 2:
		return 5 * time.Minute
	case 3:
		return 15 * time.Minute
	case 4:
		return 30 * time.Minute
	case 5:
		return time.Hour
	default:
		return 0
	}
}

func (s *Service) ListDueRetryMessages(ctx context.Context, store RetryMessageStore, input ClaimDueRetryMessagesInput) ([]*ChannelMessageLog, error) {
	if store == nil {
		return nil, ErrRetryMessageStoreRequired
	}
	return store.ListDueRetryMessages(ctx, input)
}

func (s *Service) ClaimDueRetryMessages(ctx context.Context, store RetryMessageStore, input ClaimDueRetryMessagesInput) ([]*ChannelMessageLog, error) {
	if store == nil {
		return nil, ErrRetryMessageStoreRequired
	}
	return store.ClaimDueRetryMessages(ctx, input)
}

func (s *Service) ProcessDueRetryMessages(ctx context.Context, store RetryWorkerStore, input ClaimDueRetryMessagesInput) (ProcessDueRetryMessagesResult, error) {
	if store == nil {
		return ProcessDueRetryMessagesResult{}, ErrRetryMessageStoreRequired
	}
	now := channelRequestTime(input.Now)
	claimed, err := store.ClaimDueRetryMessages(ctx, input)
	if err != nil {
		return ProcessDueRetryMessagesResult{}, err
	}
	result := ProcessDueRetryMessagesResult{Claimed: len(claimed)}
	for _, claimedLog := range claimed {
		if claimedLog == nil {
			continue
		}
		updated := *claimedLog
		targetChannelID := claimedLog.ChannelID
		if input.FallbackChannelID != "" {
			targetChannelID = input.FallbackChannelID
		}
		config, err := store.GetConfigByID(ctx, targetChannelID)
		if err == nil && config != nil {
			updated.ChannelID = config.ID
			sendLog, sendErr := s.Send(ctx, SendRequest{
				ChannelID: config.ID,
				Type:      config.Type,
				Config:    config.Config,
				Message:   claimedLog.TransformedMessage,
				Now:       now,
			})
			if sendErr == nil && sendLog.TransformSuccess {
				updated.RawMessage = sendLog.RawMessage
				updated.TransformSuccess = true
				updated.TransformError = ""
				updated.FailureReason = ""
				updated.NextRetryAt = nil
				updated.Status = MessageStatusRecorded
				result.Succeeded++
				_, err = store.UpdateRetryMessageLog(ctx, &updated)
				if err != nil {
					return result, err
				}
				s.updateRetryChannelHealth(ctx, store, config, &updated, now)
				continue
			}
			if sendErr != nil {
				err = sendErr
			} else {
				err = errors.New(sendLog.FailureReason)
			}
		} else if err == nil {
			err = fmt.Errorf("channel config %q not found", targetChannelID)
		}
		failed := s.MarkDeliveryFailed(updated, err.Error(), now)
		result.Failed++
		if failed.Status == MessageStatusPermanentFailure {
			result.PermanentFailures++
		}
		if _, err := store.UpdateRetryMessageLog(ctx, &failed); err != nil {
			return result, err
		}
		if config != nil {
			s.updateRetryChannelHealth(ctx, store, config, &failed, now)
		}
	}
	return result, nil
}

func (s *Service) updateRetryChannelHealth(ctx context.Context, store RetryWorkerStore, config *ChannelConfig, logEntry *ChannelMessageLog, occurredAt time.Time) {
	if store == nil || config == nil || logEntry == nil {
		return
	}
	if !logEntry.TransformSuccess || logEntry.Status == MessageStatusRetryPending || logEntry.Status == MessageStatusPermanentFailure {
		count, err := store.CountConsecutiveDeliveryFailures(ctx, config.ID, ChannelHealthThreshold)
		if err == nil && count >= ChannelHealthThreshold && config.Status != ChannelStatusDegraded {
			if _, updateErr := store.UpdateConfigStatus(ctx, config.OrganizationID, config.ID, ChannelStatusDegraded); updateErr == nil {
				s.notifyChannelHealth(ctx, config, logEntry, ChannelStatusDegraded, occurredAt)
			}
		}
		return
	}
	if config.Status == ChannelStatusDegraded {
		count, err := store.CountConsecutiveSuccessfulDeliveries(ctx, config.ID, ChannelHealthThreshold)
		if err == nil && count >= ChannelHealthThreshold {
			_, _ = store.UpdateConfigStatus(ctx, config.OrganizationID, config.ID, ChannelStatusActive)
		}
	}
}

func (s *Service) notifyChannelHealth(ctx context.Context, config *ChannelConfig, logEntry *ChannelMessageLog, status ChannelStatus, occurredAt time.Time) {
	if s == nil || s.healthNotifier == nil || config == nil {
		return
	}
	reason := ""
	messageLogID := ""
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	if logEntry != nil {
		reason = strings.TrimSpace(logEntry.FailureReason)
		if reason == "" {
			reason = strings.TrimSpace(logEntry.TransformError)
		}
		messageLogID = logEntry.ID
	}
	s.healthNotifier(ctx, ChannelHealthEvent{
		OrganizationID: config.OrganizationID,
		ChannelID:      config.ID,
		ChannelName:    config.Name,
		ChannelType:    config.Type,
		Status:         status,
		MessageLogID:   messageLogID,
		Reason:         reason,
		OccurredAt:     occurredAt,
	})
}

func (s *Service) adapter(channelType ChannelType) (ChannelAdapter, error) {
	if s == nil || s.registry == nil {
		return nil, fmt.Errorf("channel adapter registry is nil")
	}
	return s.registry.Adapter(channelType)
}

func channelRequestTime(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}

func auditableRawMessage(message InternalMessage) json.RawMessage {
	raw, err := json.Marshal(message)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}
