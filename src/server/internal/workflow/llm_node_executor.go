package workflow

import (
	"context"
	"fmt"
	"strings"
)

type LLMChatGateway interface {
	Chat(ctx context.Context, request LLMChatRequest) (*LLMChatResponse, error)
}

type LLMChatRequest struct {
	Model          string
	Prompt         string
	Messages       []LLMChatMessage
	Options        map[string]any
	OrganizationID string
	UserID         string
	WorkspaceID    string
	RequestID      string
	FeatureType    string
}

type LLMChatMessage struct {
	Role    string
	Content string
}

type LLMChatResponse struct {
	Text    string
	Content string
	Model   string
	Usage   map[string]any
	Raw     map[string]any
}

type LLMNodeExecutor struct {
	gateway LLMChatGateway
}

func NewLLMNodeExecutor(gateway LLMChatGateway) *LLMNodeExecutor {
	return &LLMNodeExecutor{gateway: gateway}
}

func (e *LLMNodeExecutor) Type() string { return "llm" }

func (e *LLMNodeExecutor) Execute(ctx context.Context, input NodeExecutorInput) (map[string]any, error) {
	if e == nil || e.gateway == nil {
		return nil, fmt.Errorf("%w: llm gateway is required", ErrInvalidInput)
	}
	request, err := llmChatRequestFromInput(input.Input)
	if err != nil {
		return nil, err
	}
	applyLLMChatRequestAttribution(&request, input)
	response, err := e.gateway.Chat(ctx, request)
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, fmt.Errorf("%w: llm gateway response is required", ErrInvalidInput)
	}
	text := strings.TrimSpace(response.Text)
	if text == "" {
		text = strings.TrimSpace(response.Content)
	}
	output := map[string]any{
		"text":    text,
		"content": text,
		"model":   response.Model,
	}
	if len(response.Usage) > 0 {
		output["usage"] = mergeWorkflowMaps(response.Usage, nil)
	}
	if len(response.Raw) > 0 {
		output["raw"] = mergeWorkflowMaps(response.Raw, nil)
	}
	return output, nil
}

const workflowLLMFeatureType = "workflow"

func applyLLMChatRequestAttribution(request *LLMChatRequest, input NodeExecutorInput) {
	if request == nil {
		return
	}
	request.FeatureType = workflowLLMFeatureType
	if input.Execution != nil {
		request.OrganizationID = strings.TrimSpace(input.Execution.OrganizationID)
		request.UserID = firstWorkflowString(input.Execution.Context, "userId", "userID", "user_id")
		request.WorkspaceID = firstWorkflowString(input.Execution.Context, "workspaceId", "workspaceID", "workspace_id")
		request.RequestID = firstWorkflowString(input.Execution.Context, "requestId", "requestID", "request_id")
	}
	if request.OrganizationID == "" && input.Workflow != nil {
		request.OrganizationID = strings.TrimSpace(input.Workflow.OrganizationID)
	}
}

func llmChatRequestFromInput(input map[string]any) (LLMChatRequest, error) {
	config := llmNodeConfig(input)
	request := LLMChatRequest{
		Model:   strings.TrimSpace(stringFromWorkflowValue(config["model"])),
		Prompt:  strings.TrimSpace(stringFromWorkflowValue(config["prompt"])),
		Options: llmNodeOptions(config),
	}
	messages, err := llmNodeMessages(config["messages"])
	if err != nil {
		return LLMChatRequest{}, err
	}
	request.Messages = messages
	if request.Prompt == "" && len(request.Messages) == 0 {
		return LLMChatRequest{}, fmt.Errorf("%w: llm node prompt or messages are required", ErrInvalidInput)
	}
	if len(request.Messages) == 0 && request.Prompt != "" {
		request.Messages = []LLMChatMessage{{Role: "user", Content: request.Prompt}}
	}
	return request, nil
}

func llmNodeConfig(input map[string]any) map[string]any {
	config := mergeWorkflowMaps(input, nil)
	for _, key := range []string{"config", "data"} {
		if nested, ok := mapStringAnyFromAny(input[key]); ok {
			config = mergeWorkflowMaps(nested, config)
		}
	}
	return config
}

func llmNodeOptions(config map[string]any) map[string]any {
	options, _ := mapStringAnyFromAny(config["options"])
	return mergeWorkflowMaps(options, nil)
}

func llmNodeMessages(value any) ([]LLMChatMessage, error) {
	if value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case []LLMChatMessage:
		return append([]LLMChatMessage(nil), typed...), nil
	case []map[string]any:
		messages := make([]LLMChatMessage, 0, len(typed))
		for _, item := range typed {
			messages = append(messages, llmNodeMessageFromMap(item))
		}
		return messages, nil
	case []any:
		messages := make([]LLMChatMessage, 0, len(typed))
		for _, item := range typed {
			mapped, ok := mapStringAnyFromAny(item)
			if !ok {
				return nil, fmt.Errorf("%w: llm node messages must be objects", ErrInvalidInput)
			}
			messages = append(messages, llmNodeMessageFromMap(mapped))
		}
		return messages, nil
	default:
		return nil, fmt.Errorf("%w: llm node messages must be an array", ErrInvalidInput)
	}
}

func llmNodeMessageFromMap(input map[string]any) LLMChatMessage {
	role := strings.TrimSpace(stringFromWorkflowValue(input["role"]))
	if role == "" {
		role = "user"
	}
	return LLMChatMessage{
		Role:    role,
		Content: stringFromWorkflowValue(input["content"]),
	}
}
