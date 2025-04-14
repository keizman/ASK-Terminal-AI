package terminal

import (
	"ask_terminal/config"
	"ask_terminal/dto"
	"ask_terminal/relay"
	"ask_terminal/service"
	"ask_terminal/utils"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fatih/color"
)

// CommandSuggestion represents a command with its description
type CommandSuggestion struct {
	Command     string
	Description string
}

// VirtualTerminalModel represents the model for the virtual terminal
type VirtualTerminalModel struct {
	query           string
	input           textinput.Model
	suggestions     []CommandSuggestion
	selected        int
	editMode        bool
	loading         bool
	err             error
	config          *config.Config
	logger          *utils.Logger
	adapter         relay.AIAdapter
	executionMode   bool
	executionOutput string
}

// NewVirtualTerminalModel creates a new virtual terminal model
func NewVirtualTerminalModel(conf *config.Config) *VirtualTerminalModel {
	// Initialize text input
	ti := textinput.New()
	ti.Placeholder = "Type your command query here..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 80

	// Initialize logger
	logger := utils.NewLogger()

	// Create AI adapter
	adapter, err := relay.NewAdapter(conf)
	if err != nil {
		return &VirtualTerminalModel{
			input:  ti,
			err:    err,
			config: conf,
			logger: logger,
		}
	}

	return &VirtualTerminalModel{
		input:   ti,
		config:  conf,
		logger:  logger,
		adapter: adapter,
	}
}

// Init initializes the model
func (m VirtualTerminalModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles the model updates
func (m VirtualTerminalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle special keys first before passing to textinput
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "down":
			if !m.loading && len(m.suggestions) > 0 {
				if msg.String() == "up" {
					m.selected = (m.selected - 1 + len(m.suggestions)) % len(m.suggestions)
				} else {
					m.selected = (m.selected + 1) % len(m.suggestions)
				}
			}
			return m, nil

		case "e":
			// Debug log to verify key press is detected
			utils.LogInfo("Edit key pressed")

			if !m.loading && len(m.suggestions) > 0 && !m.editMode && !m.executionMode {
				// Enter edit mode for the selected command
				m.editMode = true
				m.input.SetValue(m.suggestions[m.selected].Command)
				m.input.CursorEnd()
				return m, nil
			}

		case "enter":
			if m.editMode {
				// Exit edit mode and update the command
				m.editMode = false
				m.suggestions[m.selected].Command = m.input.Value()
				m.input.SetValue("")
			} else if m.executionMode {
				// Exit execution mode
				m.executionMode = false
				m.executionOutput = ""
			} else if len(m.suggestions) > 0 {
				// Execute the selected command
				command := m.suggestions[m.selected].Command
				m.executionMode = true
				return m, executeCommand(command)
			} else if !m.loading {
				// Submit the query to get suggestions
				m.query = m.input.Value()
				if m.query != "" {
					m.input.SetValue("")
					m.loading = true
					return m, getCommandSuggestions(m.query, m.config, m.adapter)
				}
			}
			return m, nil
		}

		// Only pass keyboard input to the textinput when not in execution mode
		// and not handling special keys for navigation
		if !m.executionMode {
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}

	// The rest of your existing case handling...

	return m, nil
}

// View renders the UI
func (m VirtualTerminalModel) View() string {
	var s strings.Builder

	// Title
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Render("ASK Terminal AI")
	s.WriteString(title + "\n\n")

	if m.err != nil {
		s.WriteString(color.RedString("Error: %v\n\n", m.err))
	}

	if m.executionMode {
		// Show command execution results
		s.WriteString(color.CyanString("Executing: %s\n\n", m.suggestions[m.selected].Command))
		s.WriteString(m.executionOutput + "\n\n")
		s.WriteString(color.YellowString("Press Enter to continue...\n"))
		return s.String()
	}

	// Input field or query display
	if m.loading {
		s.WriteString(fmt.Sprintf("> %s\n\n", m.query))
		s.WriteString("Loading suggestions...\n\n")
	} else if m.editMode {
		s.WriteString(color.YellowString("Editing command (press Enter when done):\n"))
		s.WriteString(fmt.Sprintf("%s\n\n", m.input.View()))
	} else {
		s.WriteString(fmt.Sprintf("> %s\n\n", m.input.View()))
	}

	// Command suggestions
	if len(m.suggestions) > 0 && !m.loading {
		for i, suggestion := range m.suggestions {
			// Render each suggestion
			if i == m.selected {
				s.WriteString(color.GreenString("→ %d. %s\n", i+1, suggestion.Command))
				s.WriteString(color.New(color.FgHiBlack).Sprintf("  %s\n", suggestion.Description))
			} else {
				s.WriteString(fmt.Sprintf("%d. %s\n", i+1, suggestion.Command))
				s.WriteString(color.New(color.FgHiBlack).Sprintf("  %s\n", suggestion.Description))
			}
		}

		// Instructions
		s.WriteString("\n" + color.YellowString("Use ↑/↓ to navigate, e to edit, Enter to execute, q to quit\n"))
	}

	return s.String()
}

// Message types for the update function
type suggestionsMsg struct {
	suggestions []CommandSuggestion
	err         error
}

type executionResultMsg struct {
	output string
	err    error
}

// Function to get command suggestions from the AI
func getCommandSuggestions(query string, conf *config.Config, adapter relay.AIAdapter) tea.Cmd {
	return func() tea.Msg {
		// Build the request
		request := utils.BuildPrompt(query, conf, "terminal")

		// Send the request
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Convert AIAdapter to Adapter to access ChatCompletion
		adapterImpl, ok := adapter.(relay.Adapter)
		if !ok {
			return suggestionsMsg{nil, fmt.Errorf("adapter does not implement required interface")}
		}

		response, err := adapterImpl.ChatCompletion(ctx, request)
		if err != nil {
			return suggestionsMsg{nil, err}
		}

		if len(response.Choices) == 0 {
			return suggestionsMsg{nil, fmt.Errorf("no suggestions received")}
		}

		// Get the response content
		content := response.Choices[0].Message.StringContent()

		// Parse the JSON response
		var rawSuggestions []map[string]map[string]string
		if err := json.Unmarshal([]byte(content), &rawSuggestions); err != nil {
			return suggestionsMsg{nil, fmt.Errorf("failed to parse suggestions: %w", err)}
		}

		// Convert to CommandSuggestion objects
		var suggestions []CommandSuggestion
		for _, item := range rawSuggestions {
			for _, cmdMap := range item {
				for cmd, desc := range cmdMap {
					suggestions = append(suggestions, CommandSuggestion{
						Command:     cmd,
						Description: desc,
					})
				}
			}
		}

		return suggestionsMsg{suggestions, nil}
	}
}

// Function to execute a command
func executeCommand(command string) tea.Cmd {
	return func() tea.Msg {
		// Split the command into parts
		parts := strings.Fields(command)
		if len(parts) == 0 {
			return executionResultMsg{"", fmt.Errorf("empty command")}
		}

		// Create the command
		cmd := exec.Command(parts[0], parts[1:]...)
		cmd.Env = os.Environ()

		// Capture output
		output, err := cmd.CombinedOutput()

		return executionResultMsg{string(output), err}
	}
}

// StartVirtualTerminalMode starts the virtual terminal mode
func StartVirtualTerminalMode(conf *config.Config) {
	p := tea.NewProgram(NewVirtualTerminalModel(conf))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running virtual terminal: %v\n", err)
		os.Exit(1)
	}
}

// StartCommandMode starts the command mode with a query
func StartCommandMode(query string, conf *config.Config) {
	// Get the adapter
	adapter, err := relay.NewAdapter(conf)
	if err != nil {
		fmt.Printf("Error initializing adapter: %v\n", err)
		os.Exit(1)
	}

	// Create service
	aiService := service.NewAIService(adapter)

	// Create command mode
	cmdMode := NewCommandMode(aiService, conf.ModelName)

	// Process the query
	err = cmdMode.ProcessQuery(query, conf.SysPrompt, true)
	if err != nil {
		fmt.Printf("Error processing query: %v\n", err)
		os.Exit(1)
	}
}

// NewCommandMode creates a new command mode
func NewCommandMode(aiService *service.AIService, model string) *CommandMode {
	return &CommandMode{
		aiService: aiService,
		model:     model,
	}
}

// CommandMode handles command processing
type CommandMode struct {
	aiService *service.AIService
	model     string
}

// ProcessQuery processes a command query
func (c *CommandMode) ProcessQuery(query string, systemPrompt string, stream bool) error {
	messages := []dto.Message{
		{
			Role: "system",
		},
		{
			Role: "user",
		},
	}

	messages[0].SetStringContent(systemPrompt)
	messages[1].SetStringContent(query)

	ctx := context.Background()

	if stream {
		return c.handleStreamingResponse(ctx, messages)
	}
	return c.handleNonStreamingResponse(ctx, messages)
}

// handleNonStreamingResponse handles non-streaming response
func (c *CommandMode) handleNonStreamingResponse(ctx context.Context, messages []dto.Message) error {
	response, err := c.aiService.SendChatRequest(ctx, messages, c.model)
	if err != nil {
		return err
	}

	if len(response.Choices) > 0 {
		content := response.Choices[0].Message.StringContent()
		fmt.Print(content)
	}
	return nil
}

// handleStreamingResponse handles streaming response
func (c *CommandMode) handleStreamingResponse(ctx context.Context, messages []dto.Message) error {
	responseStream, err := c.aiService.SendStreamingChatRequest(ctx, messages, c.model)
	if err != nil {
		return err
	}

	for response := range responseStream {
		if len(response.Choices) > 0 && response.Choices[0].Delta.Content != nil {
			fmt.Print(*response.Choices[0].Delta.Content)
			// Flush stdout to ensure immediate display
			os.Stdout.Sync()
		}
	}
	fmt.Println() // Add final newline
	return nil
}
