package relay

import (
	"fmt"

	"ask_terminal/config"
)

// NewAdapter returns the appropriate adapter based on the provider configuration
func NewAdapter(conf *config.Config) (Adapter, error) { // Use the Adapter type from adapter.go
	// For now, we only support OpenAI-compatible adapter
	if conf.Provider == "openai-compatible" || conf.Provider == "" {
		adapter := NewOpenAIAdapter()
		err := adapter.Init(conf.BaseURL, conf.APIKey)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize adapter: %w", err) // Wrap error for context
		}
		return adapter, nil // Return nil error on success
	}

	return nil, fmt.Errorf("unsupported provider: %s", conf.Provider)
}
