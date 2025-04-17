package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"ask_terminal/config"
	"ask_terminal/terminal"
	"ask_terminal/utils"
)

const version = "1.0.0"

func main() {
	// Parse command line flags
	configPath := flag.String("c", "", "Path to configuration file")
	modelName := flag.String("m", "", "Model name to use")
	provider := flag.String("p", "", "AI provider to use")
	baseURL := flag.String("u", "", "API base URL")
	apiKey := flag.String("k", "", "API key")
	sysPrompt := flag.String("s", "", "System prompt")
	proxyURL := flag.String("x", "", "Proxy URL (e.g., http://user:pass@host:port)")

	// Define temperature and maxTokens flags
	var temperatureFlag float64
	var temperatureProvided bool
	flag.Float64Var(&temperatureFlag, "temp", 0, "Temperature (0.0-1.0)")

	var maxTokensFlag uint
	var maxTokensProvided bool
	flag.UintVar(&maxTokensFlag, "max-tokens", 0, "Max tokens (0 for unlimited)")

	privateMode := flag.Bool("private-mode", false, "Enable private mode")
	showVersion := flag.Bool("v", false, "Show version information")
	showHelp := flag.Bool("h", false, "Show help information")
	showHistory := flag.Bool("show", false, "Show command history")
	interactiveMode := flag.Bool("i", false, "Use interactive conversation mode")

	// Custom flag parsing to detect if flags were actually provided
	oldUsage := flag.CommandLine.Usage
	flag.CommandLine.Usage = func() {}

	flag.Parse()
	flag.CommandLine.Usage = oldUsage

	// Check which flags were provided by examining original args
	for _, arg := range os.Args {
		if arg == "-temp" || arg == "--temp" {
			temperatureProvided = true
		}
		if arg == "-max-tokens" || arg == "--max-tokens" {
			maxTokensProvided = true
		}
	}

	// Show version and exit if requested
	if *showVersion {
		fmt.Printf("ASK Terminal AI version %s\n", version)
		os.Exit(0)
	}

	// Show help and exit if requested
	if *showHelp {
		showHelpMessage()
		os.Exit(0)
	}

	// Show command history and exit if requested
	if *showHistory {
		showCommandHistory()
		os.Exit(0)
	}

	// Load configuration
	conf, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Override configuration with command line flags
	args := make(map[string]string)
	if *modelName != "" {
		args["model"] = *modelName
	}
	if *provider != "" {
		args["provider"] = *provider
	}
	if *baseURL != "" {
		args["url"] = *baseURL
	}
	if *apiKey != "" {
		args["key"] = *apiKey
	}
	if *sysPrompt != "" {
		args["sys_prompt"] = *sysPrompt
	}
	if *proxyURL != "" {
		args["proxy"] = *proxyURL
	}

	// Only include temperature if it was explicitly provided
	if temperatureProvided {
		args["temperature"] = strconv.FormatFloat(temperatureFlag, 'f', -1, 64)
	}

	// Only include max_tokens if it was explicitly provided
	if maxTokensProvided {
		args["max_tokens"] = strconv.FormatUint(uint64(maxTokensFlag), 10)
	}

	if *privateMode {
		args["private_mode"] = "true"
	}

	conf.MergeWithArgs(args)

	// Get query from command line arguments
	query := strings.Join(flag.Args(), " ")

	// If no query provided and not in interactive mode, start virtual terminal mode
	if query == "" && !*interactiveMode {
		terminal.StartVirtualTerminalMode(conf)
		os.Exit(0)
	}

	// Log application start
	utils.LogInfo("ASK Terminal AI started")

	// Process query based on mode
	if *interactiveMode {
		terminal.StartConversationMode(query, conf)
	} else {
		terminal.StartCommandMode(query, conf)
	}

	utils.LogInfo("ASK Terminal AI completed")
}

// showHelpMessage prints the help message
func showHelpMessage() {
	fmt.Println(`ASK Terminal AI - Help Guide

Usage: ask [options] ["query"]

Options:
  -c, --config FILE       Specify configuration file location
  -m, --model NAME        Temporarily specify model to use
  -p, --provider NAME     Temporarily specify AI provider
  -u, --url URL           Temporarily specify API base URL
  -k, --key KEY           Temporarily specify API key
  -s, --sys-prompt TEXT   Temporarily specify system prompt
  --temp FLOAT            Temporarily specify temperature (0.0-1.0)
  --max-tokens INT        Temporarily specify max tokens (0 for unlimited)
  --private-mode          Enable privacy mode
  -v, --version           Show version information
  -h, --help              Show this help message
  -show                   Show command history
  -i                      Use interactive conversation mode
  -x, --proxy URL         Specify proxy URL (e.g., http://user:pass@host:port)

Examples:
  ask "how to find large files"
  ask -i "explain docker volumes"
  ask --model gpt-4 --temp 0.8 "optimize Postgres query"`)
}

// showCommandHistory displays the command history
func showCommandHistory() {
	// Use the Logger instance for proper cross-platform path handling
	logger := utils.NewLogger()

	// Get properly formatted command history using existing method
	items, err := logger.GetRecentCommands(1000)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving command history: %v\n", err)
		return
	}

	if len(items) == 0 {
		fmt.Println("No command history found.")
		return
	}

	fmt.Printf("Recent commands (showing %d entries):\n\n", len(items))
	for i, item := range items {
		fmt.Printf("%d. [%s] Query: %s\n", i+1, item.Timestamp, item.Query)
		for cmd, desc := range item.Commands {
			fmt.Printf("   - Command: %s\n     Description: %s\n", cmd, desc)
		}
		fmt.Println()
	}
}
