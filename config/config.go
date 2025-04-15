package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ask_terminal/security"

	"gopkg.in/yaml.v2"
)

// Config holds application configuration
type Config struct {
	BaseURL     string  `yaml:"base_url"`
	APIKey      string  `yaml:"api_key"`
	ModelName   string  `yaml:"model_name"`
	PrivateMode bool    `yaml:"private_mode"`
	SysPrompt   string  `yaml:"sys_prompt"`
	Provider    string  `yaml:"provider"`
	Temperature float64 `yaml:"temperature"` // Temperature for generation
	MaxTokens   uint    `yaml:"max_tokens"`  // Max tokens for generation
}

// LoadConfig loads configuration from the specified path
func LoadConfig(configPath string) (*Config, error) {
	// If config path is not specified, use default
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		configPath = filepath.Join(homeDir, ".config", "askta", "config.yaml")
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create directory structure if it doesn't exist
		configDir := filepath.Dir(configPath)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create config directory: %w", err)
		}

		// Create a default config with comments
		defaultConfigYaml := `# ASK Terminal AI Configuration
# https://api.openai.com/v1/
# https://generativelanguage.googleapis.com/v1beta/openai/
# https://api.anthropic.com/v1/

# API service configuration
base_url: "https://api.openai.com/v1/"  # API base URL for your provider
api_key: "your-api-key"                 # Your API key (will be encrypted after first run)
model_name: "gpt-4o-mini"               # Default AI model to use

# Model parameters(only use at conversation mode)
temperature: 0.7                        # Temperature for chat mode (0.0-1.0, lower is more deterministic)
max_tokens: 3000                           # Max tokens for chat mode 

# Feature configuration
private_mode: false                     # Set to true to not send directory structure
sys_prompt: ""                          # System prompt, WARNING: Please understand what you're modifying before making changes

# Provider configuration (currently only openai-compatible is supported)
provider: "openai-compatible"           # AI provider type, no other options available yet
`

		if err := os.WriteFile(configPath, []byte(defaultConfigYaml), 0600); err != nil {
			return nil, fmt.Errorf("failed to write default config: %w", err)
		}

		return nil, fmt.Errorf("created default config at %s, please add your API key", configPath)
	}

	// Read config file
	data, err := ioutil.ReadFile(configPath)
	// fmt.Printf("Debug - Read data: %s\n %s", data, err)
	if err != nil {
		return nil, err
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Add debug logging
	// fmt.Printf("Debug - Read config file: %s\n", configPath)
	// fmt.Printf("Debug - Config values: BaseURL=%s, APIKey=%s, Model=%s\n",
	// 	config.BaseURL,
	// 	config.APIKey,
	// 	config.ModelName)

	// Validate required fields
	if config.APIKey == "" {
		return nil, fmt.Errorf("api_key is required in configuration: %s,%s, %s", config.APIKey, config.BaseURL, config.ModelName)
	}

	if config.ModelName == "" {
		// Set default model
		config.ModelName = "gpt-4o-mini"
	}

	// Set default value for temperature if not provided in config
	// Use a default value of 0.7 only if temperature is not set at all
	if config.Temperature == 0 {
		// Check if temperature actually exists in the config file
		var tempFound bool
		yamlData := string(data)
		for _, line := range strings.Split(yamlData, "\n") {
			if strings.Contains(line, "temperature:") {
				tempFound = true
				break
			}
		}

		if !tempFound {
			// Only set default if not found in config at all
			config.Temperature = 0.7
		}
	}

	// MaxTokens of 0 is valid (unlimited) so no default needed

	// Check if API key needs decryption
	decryptedKey := "" // Initialize decryptedKey
	if len(config.APIKey) > 6 && config.APIKey[:6] == "encry_" {
		// Decrypt API key
		decryptedKey, err = security.DecryptAPIKey(config.APIKey)
		if err != nil {
			return nil, err
		}
		config.APIKey = decryptedKey
	} else {
		originalKey := config.APIKey
		// Encrypt API key for future use
		encryptedKey, err := security.EncryptAPIKey(config.APIKey)
		if err != nil {
			return nil, err
		}

		// Update config file with encrypted key
		config.APIKey = encryptedKey
		newData, err := yaml.Marshal(&config)
		if err != nil {
			return nil, err
		}

		// Write updated config back to file
		if err := ioutil.WriteFile(configPath, newData, 0600); err != nil {
			return nil, err
		}

		// Restore unencrypted key for current use
		config.APIKey = originalKey
	}

	return &config, nil
}

// MergeWithArgs merges command line arguments into config
func (c *Config) MergeWithArgs(args map[string]string) {
	// Override config with command line arguments
	if model, ok := args["model"]; ok && model != "" {
		c.ModelName = model
	}

	if baseURL, ok := args["url"]; ok && baseURL != "" {
		c.BaseURL = baseURL
	}

	if apiKey, ok := args["key"]; ok && apiKey != "" {
		c.APIKey = apiKey
	}

	if sysPrompt, ok := args["sys_prompt"]; ok && sysPrompt != "" {
		c.SysPrompt = sysPrompt
	}

	// Only override temperature if explicitly provided
	if tempStr, ok := args["temperature"]; ok {
		if temp, err := strconv.ParseFloat(tempStr, 64); err == nil {
			c.Temperature = temp
		}
	}

	// Only override max_tokens if explicitly provided
	if tokensStr, ok := args["max_tokens"]; ok {
		if tokens, err := strconv.ParseUint(tokensStr, 10, 32); err == nil {
			c.MaxTokens = uint(tokens)
		}
	}

	if _, ok := args["private_mode"]; ok {
		c.PrivateMode = true
	}
}
