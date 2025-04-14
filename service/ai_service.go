package service

import (
	"ask_terminal/dto"
	"ask_terminal/relay"
	"context"
)

type AIService struct {
	adapter relay.Adapter
}

func NewAIService(adapter relay.Adapter) *AIService {
	return &AIService{adapter: adapter}
}

// SendChatRequest sends a chat request to the AI service
func (s *AIService) SendChatRequest(ctx context.Context, messages []dto.Message, model string) (*dto.OpenAITextResponse, error) {
	request := &dto.GeneralOpenAIRequest{
		Model:    model,
		Messages: messages,
	}

	return s.adapter.ChatCompletion(ctx, request)
}

// SendStreamingChatRequest sends a chat request that streams responses
func (s *AIService) SendStreamingChatRequest(ctx context.Context, messages []dto.Message, model string) (chan *dto.ChatCompletionsStreamResponse, error) {
	request := &dto.GeneralOpenAIRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
	}

	return s.adapter.ChatCompletionStream(ctx, request)
}
