package channel

import (
	"context"
	"time"

	"oblivious/server/internal/relay/types"
)

type ProviderProbeResult struct {
	Provider     string
	Models       []string
	Balance      *ProviderBalance
	BalanceError string
	Health       ProviderHealth
	CheckedAt    time.Time
}

type ProviderBalance struct {
	Amount   float64
	Currency string
	Source   string
}

type ProviderHealth struct {
	Status    string
	Message   string
	CheckedAt time.Time
}

type modelLister interface {
	ListModels(ctx context.Context) ([]string, error)
}

type balanceChecker interface {
	CheckBalance(ctx context.Context) (*ProviderBalance, error)
}

func ProbeChannel(ctx context.Context, ch *types.Channel) (*ProviderProbeResult, error) {
	adapter, err := AdapterForChannel(ch)
	if err != nil {
		return nil, err
	}

	result := &ProviderProbeResult{
		Provider:  adapter.Provider(),
		CheckedAt: time.Now().UTC(),
	}

	if lister, ok := adapter.(modelLister); ok {
		models, err := lister.ListModels(ctx)
		if err != nil {
			return nil, err
		}
		result.Models = models
	} else if err := adapter.HealthCheck(ctx); err != nil {
		return nil, err
	}

	result.Health = ProviderHealth{
		Status:    "online",
		CheckedAt: result.CheckedAt,
	}

	if checker, ok := adapter.(balanceChecker); ok {
		balance, err := checker.CheckBalance(ctx)
		if err != nil {
			result.BalanceError = err.Error()
		} else {
			result.Balance = balance
		}
	}

	return result, nil
}
