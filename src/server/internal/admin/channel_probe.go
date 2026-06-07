package admin

import (
	"context"
	"regexp"
	"strings"
	"time"

	relaychannel "oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

var apiKeyPattern = regexp.MustCompile(`sk-[A-Za-z0-9._-]+`)

func testRelayChannel(ctx context.Context, ch *types.Channel) *ChannelTestResult {
	start := time.Now()
	probe, err := relaychannel.ProbeChannel(ctx, ch)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return &ChannelTestResult{
			Success: false,
			Latency: latency,
			Health: &ChannelHealthDetail{
				Status:    "offline",
				CheckedAt: time.Now().UTC(),
			},
			Error: sanitizeProbeError(err.Error(), ch),
		}
	}

	checkedAt := probe.CheckedAt
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}

	return &ChannelTestResult{
		Success:      true,
		Latency:      latency,
		Provider:     probe.Provider,
		Models:       probe.Models,
		Balance:      channelBalanceFromProbe(probe.Balance),
		BalanceError: sanitizeProbeError(probe.BalanceError, ch),
		Health: &ChannelHealthDetail{
			Status:    firstNonEmpty(probe.Health.Status, "online"),
			Message:   sanitizeProbeError(probe.Health.Message, ch),
			CheckedAt: checkedAt,
		},
	}
}

func channelBalanceFromProbe(balance *relaychannel.ProviderBalance) *ChannelBalance {
	if balance == nil {
		return nil
	}
	return &ChannelBalance{
		Amount:   balance.Amount,
		Currency: firstNonEmpty(balance.Currency, "USD"),
		Source:   balance.Source,
	}
}

func sanitizeProbeError(message string, ch *types.Channel) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	if ch != nil && strings.TrimSpace(ch.APIKey) != "" {
		message = strings.ReplaceAll(message, ch.APIKey, "[redacted]")
	}
	return apiKeyPattern.ReplaceAllString(message, "[redacted]")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
