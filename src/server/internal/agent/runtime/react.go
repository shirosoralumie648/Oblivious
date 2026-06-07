package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/chat"
)

// ReActStep represents a single step in the ReAct loop.
type ReActStep struct {
	Iteration   int            `json:"iteration"`
	Thought     string         `json:"thought"`
	Action      string         `json:"action,omitempty"`
	ActionInput map[string]any `json:"actionInput,omitempty"`
	Observation string         `json:"observation,omitempty"`
	TokensUsed  int            `json:"tokensUsed,omitempty"`
}

// ReActResult is the outcome of a complete ReAct run.
type ReActResult struct {
	FinalAnswer    string     `json:"finalAnswer"`
	Steps          []ReActStep `json:"steps"`
	TotalTokens    int        `json:"totalTokens"`
	StopReason     string     `json:"stopReason"`
	IterationCount int        `json:"iterationCount"`
}

// ReActConfig controls the ReAct loop behaviour.
type ReActConfig struct {
	MaxIterations int // upper bound on Thought-Action-Observation cycles
	TokenBudget   int // total token budget across all iterations (0 = unlimited)
}

// DefaultReActConfig returns sensible defaults.
func DefaultReActConfig() ReActConfig {
	return ReActConfig{
		MaxIterations: 10,
		TokenBudget:   0,
	}
}

// NormalizeReActConfig applies limits and defaults.
func NormalizeReActConfig(cfg ReActConfig) ReActConfig {
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 10
	}
	if cfg.MaxIterations > 100 {
		cfg.MaxIterations = 100
	}
	if cfg.TokenBudget < 0 {
		cfg.TokenBudget = 0
	}
	if cfg.TokenBudget > 0 && cfg.TokenBudget < 1000 {
		cfg.TokenBudget = 1000
	}
	if cfg.TokenBudget > 1_000_000 {
		cfg.TokenBudget = 1_000_000
	}
	return cfg
}

// ToolRunner abstracts tool execution so the engine is testable.
type ToolRunner interface {
	ExecuteTool(ctx context.Context, agentInstance *agent.Agent, toolCall *agent.ToolCall) (*agent.ExecuteResult, error)
	BuildOpenAITools(ctx context.Context, agentInstance *agent.Agent) ([]map[string]any, error)
}

// ChatGateway abstracts LLM calls.
type ChatGateway interface {
	GenerateStructuredReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, tools []map[string]any) (*chat.CompletionResponse, error)
}

// ReActEngine runs the Thought-Action-Observation loop.
type ReActEngine struct {
	gateway ChatGateway
	runner  ToolRunner
}

// NewReActEngine creates a new engine.
func NewReActEngine(gateway ChatGateway, runner ToolRunner) *ReActEngine {
	return &ReActEngine{gateway: gateway, runner: runner}
}

// reactSystemPrompt is injected before the conversation to request ReAct
// style output from the model.
const reactSystemPrompt = `You are a ReAct agent. For each user request, respond in this EXACT JSON format:
{"thought":"your reasoning about what to do next","action":"tool_name","actionInput":{"key":"value"}}
OR when you have enough information to answer:
{"thought":"your reasoning","finalAnswer":"the final answer to the user"}

Rules:
- Always start with a thought.
- Use an action+actionInput only when you need tool output.
- After observing tool results, reason again before deciding the next action.
- When you can answer without more tools, use finalAnswer.
- Be concise. Do not repeat observations.`

// Run executes the ReAct loop for the given agent and conversation.
func (e *ReActEngine) Run(ctx context.Context, agentInstance *agent.Agent, conversationID string, messages []chat.Message, cfg ReActConfig) (*ReActResult, error) {
	if e.gateway == nil {
		return nil, fmt.Errorf("react engine: gateway not configured")
	}
	cfg = NormalizeReActConfig(cfg)

	tools, err := e.buildTools(ctx, agentInstance)
	if err != nil {
		return nil, fmt.Errorf("react engine: build tools: %w", err)
	}

	config := chat.ConversationConfig{
		ModelID:              agentInstance.Model,
		SystemPromptOverride: agentInstance.SystemPrompt,
		Temperature:          agentInstance.Config.Temperature,
		MaxOutputTokens:      agentInstance.Config.MaxTokens,
		ToolsEnabled:         len(tools) > 0,
	}
	if config.Temperature == 0 {
		config.Temperature = 0.7
	}
	if config.MaxOutputTokens == 0 {
		config.MaxOutputTokens = 2048
	}

	// Prepend ReAct system prompt.
	chatMessages := make([]chat.Message, 0, len(messages)+1)
	chatMessages = append(chatMessages, chat.Message{
		Role:    "system",
		Content: reactSystemPrompt,
	})
	chatMessages = append(chatMessages, messages...)

	result := &ReActResult{}
	totalTokens := 0

	for iteration := 0; iteration < cfg.MaxIterations; iteration++ {
		reply, err := e.gateway.GenerateStructuredReply(ctx, chatMessages, config, tools)
		if err != nil {
			return nil, fmt.Errorf("react engine: iteration %d: %w", iteration+1, err)
		}
		totalTokens += completionTokens(reply)
		result.TotalTokens = totalTokens

		if cfg.TokenBudget > 0 && totalTokens > cfg.TokenBudget {
			result.StopReason = "token_budget_exceeded"
			result.IterationCount = iteration + 1
			return result, nil
		}

		// If the model returned tool calls, execute them and build an observation.
		if len(reply.ToolCalls) > 0 {
			step := ReActStep{
				Iteration: iteration + 1,
				Thought:   extractThought(reply.Content),
				Action:    reply.ToolCalls[0].Function.Name,
			}
			var args map[string]any
			if strings.TrimSpace(reply.ToolCalls[0].Function.Arguments) != "" {
				_ = json.Unmarshal([]byte(reply.ToolCalls[0].Function.Arguments), &args)
			}
			step.ActionInput = args
			step.TokensUsed = completionTokens(reply)

			execResult, err := e.executeTool(ctx, agentInstance, reply.ToolCalls[0])
			if err != nil {
				step.Observation = fmt.Sprintf("Error: %s", err.Error())
			} else if execResult.IsError {
				step.Observation = fmt.Sprintf("Error: %s", execResult.Content)
			} else {
				step.Observation = execResult.Content
			}
			result.Steps = append(result.Steps, step)

			// Append tool call message and observation to context for next iteration.
			chatMessages = append(chatMessages, chat.Message{
				Role:    "assistant",
				Content: reply.Content,
				ToolCalls: []chat.ToolCall{{
					ID:   reply.ToolCalls[0].ID,
					Type: "function",
					Function: chat.ToolFunction{
						Name:      reply.ToolCalls[0].Function.Name,
						Arguments: reply.ToolCalls[0].Function.Arguments,
					},
				}},
			})
			chatMessages = append(chatMessages, chat.Message{
				Role:       "tool",
				ToolCallID: reply.ToolCalls[0].ID,
				Content:    step.Observation,
			})
			continue
		}

		// No tool calls. Parse the response as a ReAct JSON or treat as final answer.
		step := ReActStep{
			Iteration:  iteration + 1,
			TokensUsed: completionTokens(reply),
		}
		reactResp := parseReActResponse(reply.Content)
		step.Thought = reactResp.Thought
		step.Action = reactResp.Action
		step.ActionInput = reactResp.ActionInput
		step.Observation = reactResp.Observation
		result.Steps = append(result.Steps, step)

		if reactResp.FinalAnswer != "" {
			result.FinalAnswer = reactResp.FinalAnswer
			result.StopReason = "final_answer"
			result.IterationCount = iteration + 1
			return result, nil
		}

		// Model produced text but no tool call and no parseable finalAnswer --
		// treat the whole content as the final answer.
		result.FinalAnswer = reply.Content
		result.StopReason = "model_stop"
		result.IterationCount = iteration + 1
		return result, nil
	}

	result.StopReason = "max_iterations_reached"
	result.IterationCount = cfg.MaxIterations
	return result, nil
}

func (e *ReActEngine) buildTools(ctx context.Context, agentInstance *agent.Agent) ([]map[string]any, error) {
	if e.runner == nil {
		return nil, nil
	}
	return e.runner.BuildOpenAITools(ctx, agentInstance)
}

func (e *ReActEngine) executeTool(ctx context.Context, agentInstance *agent.Agent, tc chat.ToolCall) (*agent.ExecuteResult, error) {
	if e.runner == nil {
		return nil, fmt.Errorf("tool runner not configured")
	}
	var args map[string]any
	if strings.TrimSpace(tc.Function.Arguments) != "" {
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
	}
	if args == nil {
		args = map[string]any{}
	}
	agentToolCall := &agent.ToolCall{
		ID:        tc.ID,
		Name:      tc.Function.Name,
		Arguments: args,
	}
	return e.runner.ExecuteTool(ctx, agentInstance, agentToolCall)
}

// reactParsedResponse is the JSON shape the ReAct prompt asks for.
type reactParsedResponse struct {
	Thought     string         `json:"thought"`
	FinalAnswer string         `json:"finalAnswer,omitempty"`
	Action      string         `json:"action,omitempty"`
	ActionInput map[string]any `json:"actionInput,omitempty"`
	Observation string         `json:"observation,omitempty"`
}

func parseReActResponse(content string) reactParsedResponse {
	content = strings.TrimSpace(content)
	var resp reactParsedResponse
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		// Not parseable JSON. Treat entire content as thought.
		resp.Thought = content
		return resp
	}
	return resp
}

func extractThought(content string) string {
	content = strings.TrimSpace(content)
	var resp reactParsedResponse
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		return content
	}
	if resp.Thought != "" {
		return resp.Thought
	}
	return content
}

func completionTokens(reply *chat.CompletionResponse) int {
	if reply == nil || reply.Usage == nil || reply.Usage.TotalTokens <= 0 {
		return 0
	}
	return reply.Usage.TotalTokens
}

// Ensure time import is used.
var _ = time.Now
