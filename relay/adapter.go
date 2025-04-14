package relay

import (
	"ask_terminal/dto"
	"context"
)

// AIAdapter defines the interface for AI service adapters
type AIAdapter interface {
	ProcessQuery(query string) (string, error)
}

// Adapter defines the complete adapter interface for API interactions
type Adapter interface {
	// Initialize the adapter with configuration
	Init(baseURL, apiKey string) error

	// Send a chat completion request
	ChatCompletion(ctx context.Context, request *dto.GeneralOpenAIRequest) (*dto.OpenAITextResponse, error)

	// Send a streaming chat completion request
	ChatCompletionStream(ctx context.Context, request *dto.GeneralOpenAIRequest) (chan *dto.ChatCompletionsStreamResponse, error)

	// Process a simple query (for AIAdapter compatibility)
	ProcessQuery(query string) (string, error)
}
