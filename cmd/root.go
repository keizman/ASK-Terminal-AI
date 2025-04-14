// cmd/root.go
package cmd

import (
	"fmt"
	"os"
	"strings"

	"ask_terminal/config"
	"ask_terminal/terminal"
	"ask_terminal/utils"

	"github.com/spf13/cobra"
)

var (
	cfgFile     string
	modelName   string
	provider    string
	baseURL     string
	apiKey      string
	sysPrompt   string
	privateMode bool
	showHistory bool
)

var rootCmd = &cobra.Command{
	Use:   "ask [query]",
	Short: "ASK Terminal AI - AI assistant for your terminal",
	Run: func(cmd *cobra.Command, args []string) {
		// Initialize logger
		logger := utils.NewLogger()

		// If -show flag is present, display command history
		if showHistory {
			displayCommandHistory(logger)
			return
		}

		// Load configuration
		conf, err := config.LoadConfig(cfgFile)
		if err != nil {
			logger.LogApplication(fmt.Sprintf("Error loading config: %v", err))
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Apply command line overrides
		applyConfigOverrides(conf)

		// Check if a query is provided
		if len(args) > 0 {
			// Join all args to form the query
			query := strings.Join(args, " ")
			// Conversation mode
			terminal.StartConversationMode(query, conf)
		} else {
			// Virtual terminal mode
			terminal.StartVirtualTerminalMode(conf)
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "Config file path")
	rootCmd.PersistentFlags().StringVarP(&modelName, "model", "m", "", "Model name to use")
	rootCmd.PersistentFlags().StringVarP(&provider, "provider", "p", "", "AI provider (openai-compatible)")
	rootCmd.PersistentFlags().StringVarP(&baseURL, "url", "u", "", "API base URL")
	rootCmd.PersistentFlags().StringVarP(&apiKey, "key", "k", "", "API key")
	rootCmd.PersistentFlags().StringVarP(&sysPrompt, "sys-prompt", "s", "", "System prompt")
	rootCmd.PersistentFlags().BoolVar(&privateMode, "private-mode", false, "Enable private mode")
	rootCmd.PersistentFlags().BoolVar(&showHistory, "show", false, "Show recent command history")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// applyConfigOverrides applies command line overrides to the config
func applyConfigOverrides(conf *config.Config) {
	if modelName != "" {
		conf.ModelName = modelName
	}
	if provider != "" {
		conf.Provider = provider
	}
	if baseURL != "" {
		conf.BaseURL = baseURL
	}
	if apiKey != "" {
		conf.APIKey = apiKey
	}
	if sysPrompt != "" {
		conf.SysPrompt = sysPrompt
	}
	if privateMode {
		conf.PrivateMode = true
	}
}

// displayCommandHistory shows the recent command history
func displayCommandHistory(logger *utils.Logger) {
	items, err := logger.GetRecentCommands(1000)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving command history: %v\n", err)
		os.Exit(1)
	}

	if len(items) == 0 {
		fmt.Println("No command history found.")
		return
	}

	fmt.Printf("Recent commands (showing %d entries):\n\n", len(items))
	for i, item := range items {
		fmt.Printf("%d. [%s] Query: %s\n", i+1, item.Timestamp, item.Query)
		for cmd := range item.Commands {
			fmt.Printf("   - %s\n", cmd)
		}
		fmt.Println()
	}
}
