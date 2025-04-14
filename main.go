package main

import (
	"flag"
	"fmt"
	"os"
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
	privateMode := flag.Bool("private-mode", false, "Enable private mode")
	showVersion := flag.Bool("v", false, "Show version information")
	showHelp := flag.Bool("h", false, "Show help information")
	showHistory := flag.Bool("show", false, "Show command history")
	interactiveMode := flag.Bool("i", false, "Use interactive conversation mode")

	flag.Parse()

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
	if *privateMode {
		args["private_mode"] = "true"
	}

	conf.MergeWithArgs(args)

	// Get query from command line arguments
	query := strings.Join(flag.Args(), " ")

	// If no query provided, print help message and exit
	if query == "" && !*interactiveMode {
		fmt.Println("Error: No query provided.")
		showHelpMessage()
		os.Exit(1)
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
  --private-mode          Enable privacy mode
  -v, --version           Show version information
  -h, --help              Show this help message
  -show                   Show command history
  -i                      Use interactive conversation mode

Examples:
  ask "how to find large files"
  ask -i "explain docker volumes"
  ask --model gpt-4 "optimize Postgres query"`)
}

// showCommandHistory displays the command history
func showCommandHistory() {
	logFile := "/tmp/askta_Chistory.log"

	data, err := os.ReadFile(logFile)
	if err != nil {
		fmt.Println("No command history found.")
		return
	}

	fmt.Println("Recent Command History:")
	fmt.Println(string(data))
}
