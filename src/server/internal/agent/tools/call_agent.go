package tools

import (
	"context"
	"fmt"
)

type CallAgentTool struct {
	service AgentService
}

type AgentService interface {
	CreateSubAgentRun(ctx context.Context, req *SubAgentRunRequest) (*SubAgentRunResult, error)
}

type SubAgentRunRequest struct {
	AgentID        string
	RequestText    string
	RecursionDepth int
	MaxDepth       int
}

type SubAgentRunResult struct {
	FinalResponse string
	UsedTokens    int
}

func NewCallAgentTool(svc AgentService) *CallAgentTool {
	return &CallAgentTool{service: svc}
}

func (t *CallAgentTool) Execute(ctx context.Context, input CallAgentInput) (CallAgentOutput, error) {
	if input.RecursionDepth >= input.MaxDepth {
		return CallAgentOutput{}, fmt.Errorf("max sub-agent depth %d reached", input.MaxDepth)
	}

	result, err := t.service.CreateSubAgentRun(ctx, &SubAgentRunRequest{
		AgentID:        input.AgentID,
		RequestText:    input.RequestText,
		RecursionDepth: input.RecursionDepth + 1,
		MaxDepth:       input.MaxDepth,
	})
	if err != nil {
		return CallAgentOutput{}, err
	}

	return CallAgentOutput{
		Result:     result.FinalResponse,
		TokensUsed: result.UsedTokens,
	}, nil
}

type CallAgentInput struct {
	AgentID        string
	RequestText    string
	RecursionDepth int
	MaxDepth       int
}

type CallAgentOutput struct {
	Result     string
	TokensUsed int
}
