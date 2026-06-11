package relay

import (
	"context"
	"encoding/json"
	"io"

	pb "oblivious/server/pkg/relay/proto"
	"oblivious/server/internal/relay"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

type Server struct {
	pb.UnimplementedRelayServiceServer
	router  *relay.Router
	adapter *channel.OpenAIAdapter
}

func NewServer(router *relay.Router, adapter *channel.OpenAIAdapter) *Server {
	return &Server{router: router, adapter: adapter}
}

func (s *Server) Complete(ctx context.Context, req *pb.CompletionRequest) (*pb.CompletionResponse, error) {
	messages := make([]types.Message, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = types.Message{Role: msg.Role, Content: msg.Content}
	}

	provReq := &types.ProviderRequest{
		APIType:  types.APITypeChat,
		Model:    req.Model,
		Messages: messages,
	}
	if req.MaxTokens != nil {
		provReq.MaxTokens = int(*req.MaxTokens)
	}

	usage := s.adapter.EstimateUsage(provReq)
	ch := s.router.SelectChannel(ctx, provReq.APIType.String())
	if ch == nil {
		return nil, types.ErrNoAvailableChannel
	}

	adp := channel.NewOpenAICompatibleAdapter(ch.Channel.Provider, ch.Channel.BaseURL, ch.Channel.APIKey)
	httpResp, err := adp.DoRequest(ctx, provReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return nil, err
	}

	resp := &pb.CompletionResponse{
		Id:    getString(body, "id"),
		Model: getString(body, "model"),
	}

	if choices, ok := body["choices"].([]any); ok && len(choices) > 0 {
		resp.Choices = make([]*pb.Choice, len(choices))
		for i, ch := range choices {
			chMap, _ := ch.(map[string]any)
			msg, _ := chMap["message"].(map[string]any)
			resp.Choices[i] = &pb.Choice{
				Index: int32(i),
				Message: &pb.Message{
					Role:    getString(msg, "role"),
					Content: getString(msg, "content"),
				},
				FinishReason: getString(chMap, "finish_reason"),
			}
		}
	}

	if usage != nil {
		resp.Usage = &pb.Usage{
			PromptTokens:     int32(usage.PromptTokens),
			CompletionTokens: int32(usage.CompletionTokens),
			TotalTokens:      int32(usage.TotalTokens),
		}
	}

	return resp, nil
}

func (s *Server) CompleteStream(req *pb.CompletionRequest, stream pb.RelayService_CompleteStreamServer) error {
	return nil
}

func (s *Server) Embed(ctx context.Context, req *pb.EmbeddingRequest) (*pb.EmbeddingResponse, error) {
	input := ""
	if len(req.Inputs) > 0 {
		input = req.Inputs[0]
	}

	provReq := &types.ProviderRequest{
		APIType: types.APITypeEmbeddings,
		Model:   req.Model,
		Input:   input,
	}

	usage := s.adapter.EstimateUsage(provReq)
	ch := s.router.SelectChannel(ctx, provReq.APIType.String())
	if ch == nil {
		return nil, types.ErrNoAvailableChannel
	}

	adp := channel.NewOpenAICompatibleAdapter(ch.Channel.Provider, ch.Channel.BaseURL, ch.Channel.APIKey)
	httpResp, err := adp.DoRequest(ctx, provReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return nil, err
	}

	resp := &pb.EmbeddingResponse{Model: getString(body, "model")}

	if data, ok := body["data"].([]any); ok {
		resp.Data = make([]*pb.Embedding, len(data))
		for i, d := range data {
			dMap, _ := d.(map[string]any)
			embedding, _ := dMap["embedding"].([]any)
			vec := make([]float32, len(embedding))
			for j, v := range embedding {
				if f, ok := v.(float64); ok {
					vec[j] = float32(f)
				}
			}
			resp.Data[i] = &pb.Embedding{Index: int32(i), Vector: vec}
		}
	}

	if usage != nil {
		resp.Usage = &pb.Usage{
			PromptTokens: int32(usage.PromptTokens),
			TotalTokens:  int32(usage.TotalTokens),
		}
	}

	return resp, nil
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
