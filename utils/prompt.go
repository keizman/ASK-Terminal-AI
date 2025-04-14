package utils

import (
	"ask_terminal/config"
	"ask_terminal/dto"
	"os"
)

// BuildPrompt constructs a suitable prompt based on the mode
func BuildPrompt(userQuery string, conf *config.Config, mode string) *dto.GeneralOpenAIRequest {
	// Build system context based on environment and configuration
	systemPrompt := buildSystemContext(conf, mode)

	// Create system message
	systemMessage := dto.Message{}
	systemMessage.Role = "system"
	systemMessage.SetStringContent(systemPrompt)

	// Create user message
	userMessage := dto.Message{}
	userMessage.Role = "user"

	if mode == "terminal" {
		userMessage.SetStringContent("User request: " + userQuery)
	} else {
		userMessage.SetStringContent(userQuery)
	}

	// Set different temperatures based on mode
	var temperature float64 = 0.7 // Default for conversation mode
	maxTokens := uint(0)          // Unlimited for conversation mode

	if mode == "terminal" {
		temperature = 0.1 // Lower temperature for more deterministic command suggestions
		maxTokens = 500   // Limit token usage for command suggestions
	}

	// Create JSON response format for terminal mode
	var responseFormat *dto.ResponseFormat
	if mode == "terminal" {
		responseFormat = &dto.ResponseFormat{
			Type: "json_object",
		}
	}

	// Build the request
	request := &dto.GeneralOpenAIRequest{
		Model:          conf.ModelName,
		Messages:       []dto.Message{systemMessage, userMessage},
		Temperature:    &temperature,
		MaxTokens:      maxTokens,
		ResponseFormat: responseFormat,
	}

	return request
}

// buildSystemContext creates a system prompt with environment information
func buildSystemContext(conf *config.Config, mode string) string {
	var systemPrompt string

	if mode == "terminal" {
		systemPrompt = `You are a command line expert. Help the user with their terminal commands.`
	} else {
		systemPrompt = `You are a command line assistant helping with terminal tasks and questions.`
	}

	// Add OS info
	systemPrompt += "\nCurrent environment:\n- Operating system: " + GetSystemInfo()

	// Add current directory if not in private mode
	if !conf.PrivateMode {
		cwd, err := os.Getwd()
		if err == nil {
			systemPrompt += "\n- Working directory: " + cwd

			// Add directory structure
			systemPrompt += "\nDirectory structure:\n" + GetDirectoryStructure(1)
		}
	}

	// Add user's system prompt if any
	if conf.SysPrompt != "" {
		systemPrompt += "\nUser's system prompt: " + conf.SysPrompt
	}

	// Add formatting instructions for terminal mode
	if mode == "terminal" {
		systemPrompt += `
Strictly respond with a JSON array of command suggestions formatted as follows:
[
  {"1": {"ls -la": "ls is a command to view files or folders in the current directory. -l is a parameter to display more detailed information, and -a is to show hidden files."}},
  {"2": {"command": "description"}},
  ...
]
`
	}

	return systemPrompt
}

// GetSystemInfo returns information about the current system
// func GetSystemInfo() string {
// 	return runtime.GOOS + " " + runtime.GOARCH
// }

// // GetDirectoryStructure returns a tree-like structure of the current directory
// func GetDirectoryStructure(root string, maxDepth int) string {
// 	var result strings.Builder
// 	walkDir(root, "", 0, maxDepth, &result)
// 	return result.String()
// }
