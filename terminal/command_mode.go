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
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fatih/color"
)

// CommandSuggestion represents a command with its description
type CommandSuggestion struct {
	Command        string // The original command
	EditedCommand  string // The edited version of the command
	Description    string
	CursorPosition int // Track cursor position for each command
}

// VirtualTerminalModel represents the model for the virtual terminal
type VirtualTerminalModel struct {
	query         string
	input         textinput.Model
	suggestions   []CommandSuggestion
	selected      int
	loading       bool
	cursorVisible bool
	queryMode     bool // true when entering a query, false when editing commands
	err           error
	config        *config.Config
	logger        *utils.Logger
	adapter       relay.AIAdapter
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
			input:     ti,
			err:       err,
			config:    conf,
			logger:    logger,
			queryMode: true,
		}
	}

	return &VirtualTerminalModel{
		input:         ti,
		config:        conf,
		logger:        logger,
		adapter:       adapter,
		queryMode:     true,
		cursorVisible: true,
	}
}

// Init initializes the model
func (m VirtualTerminalModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		blinkCursor(),
	)
}

// Update function with DEL key support
func (m VirtualTerminalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle special keys first
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "down":
			if !m.loading && len(m.suggestions) > 0 && !m.queryMode {
				// Navigate between commands
				if msg.String() == "up" {
					m.selected = (m.selected - 1 + len(m.suggestions)) % len(m.suggestions)
				} else {
					m.selected = (m.selected + 1) % len(m.suggestions)
				}
				return m, nil
			}

		case "enter":
			if !m.loading {
				if len(m.suggestions) > 0 && !m.queryMode {
					// Return the selected (possibly edited) command
					command := m.suggestions[m.selected].EditedCommand
					return m, tea.Sequence(
						returnCommandToShell(command),
						tea.Quit,
					)
				} else if m.queryMode {
					// Submit the query to get suggestions
					m.query = m.input.Value()
					if m.query != "" {
						m.loading = true
						m.input.SetValue("")
						m.queryMode = false
						return m, getCommandSuggestions(m.query, m.config, m.adapter)
					}
				}
			}

		case "backspace":
			if !m.loading && len(m.suggestions) > 0 && !m.queryMode {
				// Handle backspace for direct command editing
				cmd := &m.suggestions[m.selected]
				if cmd.CursorPosition > 0 {
					// Delete the character before the cursor
					before := cmd.EditedCommand[:cmd.CursorPosition-1]
					after := cmd.EditedCommand[cmd.CursorPosition:]
					cmd.EditedCommand = before + after
					cmd.CursorPosition--
				}
				return m, nil
			}

		case "delete": // Add DEL key support
			if !m.loading && len(m.suggestions) > 0 && !m.queryMode {
				cmd := &m.suggestions[m.selected]
				if cmd.CursorPosition < len(cmd.EditedCommand) {
					// Delete the character at the cursor position
					before := cmd.EditedCommand[:cmd.CursorPosition]
					after := cmd.EditedCommand[cmd.CursorPosition+1:]
					cmd.EditedCommand = before + after
				}
				return m, nil
			}

		case "left":
			if !m.loading && len(m.suggestions) > 0 && !m.queryMode {
				// Move cursor left in the command
				cmd := &m.suggestions[m.selected]
				if cmd.CursorPosition > 0 {
					cmd.CursorPosition--
				}
				return m, nil
			}

		case "right":
			if !m.loading && len(m.suggestions) > 0 && !m.queryMode {
				// Move cursor right in the command
				cmd := &m.suggestions[m.selected]
				if cmd.CursorPosition < len(cmd.EditedCommand) {
					cmd.CursorPosition++
				}
				return m, nil
			}

		case "esc":
			// Switch back to query mode if editing commands
			if !m.loading && len(m.suggestions) > 0 && !m.queryMode {
				// Reset edited commands to originals
				for i := range m.suggestions {
					m.suggestions[i].EditedCommand = m.suggestions[i].Command
					m.suggestions[i].CursorPosition = len(m.suggestions[i].Command)
				}
				m.queryMode = true
				m.input.Focus()
				return m, nil
			}

		default:
			// Handle regular key inputs for command editing
			if !m.loading && len(m.suggestions) > 0 && !m.queryMode && msg.Type == tea.KeyRunes {
				cmd := &m.suggestions[m.selected]
				// Insert the character at cursor position
				before := cmd.EditedCommand[:cmd.CursorPosition]
				after := cmd.EditedCommand[cmd.CursorPosition:]
				cmd.EditedCommand = before + string(msg.Runes) + after
				cmd.CursorPosition += len(msg.Runes)
				return m, nil
			}
		}

		// Pass inputs to textinput when in query mode
		if m.queryMode {
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}

	case suggestionsMsg:
		// Set loading to false when suggestions are received
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.queryMode = true // Go back to query mode on error
			return m, nil
		}

		// Initialize suggestions with both original and edited commands
		m.suggestions = make([]CommandSuggestion, len(msg.suggestions))
		for i, sugg := range msg.suggestions {
			m.suggestions[i] = CommandSuggestion{
				Command:        sugg.Command,
				EditedCommand:  sugg.Command,
				Description:    sugg.Description,
				CursorPosition: len(sugg.Command), // Start cursor at end
			}
		}

		m.selected = 0
		m.queryMode = false
		return m, nil

	case cursorBlinkMsg:
		m.cursorVisible = !m.cursorVisible
		return m, blinkCursor()
	}

	return m, nil
}

// Cursor blinking functionality
type cursorBlinkMsg struct{}

func blinkCursor() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return cursorBlinkMsg{}
	})
}

// Return command to shell
func returnCommandToShell(command string) tea.Cmd {
	return func() tea.Msg {
		fmt.Print(command)
		return nil
	}
}

// View function with direct command editing
func (m VirtualTerminalModel) View() string {
	var s strings.Builder

	// Title
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Render("ASK Terminal AI")
	s.WriteString(title + "\n\n")

	if m.err != nil {
		s.WriteString(color.RedString("Error: %v\n\n", m.err))
	}

	// Input field or query display
	if m.loading {
		s.WriteString(fmt.Sprintf("> %s\n\n", m.query))
		s.WriteString("Loading suggestions...\n\n")
		return s.String()
	}

	if m.queryMode {
		if len(m.suggestions) > 0 {
			s.WriteString(fmt.Sprintf("> %s\n\n", m.query))
		} else {
			s.WriteString(fmt.Sprintf("> %s\n\n", m.input.View()))
		}
	} else {
		s.WriteString(fmt.Sprintf("> %s\n\n", m.query))
	}

	// Command suggestions with direct editing
	if len(m.suggestions) > 0 {
		for i, suggestion := range m.suggestions {
			if i == m.selected && !m.queryMode {
				// Render currently selected command with cursor for editing
				cmd := suggestion.EditedCommand
				var displayCmd string

				if m.cursorVisible {
					// Insert cursor at current position
					if suggestion.CursorPosition < len(cmd) {
						displayCmd = cmd[:suggestion.CursorPosition] + "█" + cmd[suggestion.CursorPosition:]
					} else {
						displayCmd = cmd + "█"
					}
				} else {
					// Show underscore when cursor is blinking off
					if suggestion.CursorPosition < len(cmd) {
						displayCmd = cmd[:suggestion.CursorPosition] + "_" + cmd[suggestion.CursorPosition:]
					} else {
						displayCmd = cmd + "_"
					}
				}

				// Use background color for selection with editable command
				selectedStyle := lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("#000000")).
					Background(lipgloss.Color("#00FF00")).
					Padding(0, 1)

				s.WriteString(selectedStyle.Render(fmt.Sprintf(" %d. %s ", i+1, displayCmd)))
				s.WriteString("\n")
				s.WriteString(color.New(color.FgHiBlack).Sprintf("  %s\n", suggestion.Description))
			} else {
				// Render non-selected commands normally
				s.WriteString(fmt.Sprintf(" %d. %s\n", i+1, suggestion.EditedCommand))
				s.WriteString(color.New(color.FgHiBlack).Sprintf("  %s\n", suggestion.Description))
			}
		}

		// Instructions based on current state
		if !m.queryMode {
			s.WriteString("\n" + color.YellowString("Edit directly, use ↑/↓ to switch commands, Enter to execute, Esc to cancel, q to quit\n"))
		} else {
			s.WriteString("\n" + color.YellowString("Type a new query or press Enter to select commands\n"))
		}
	}

	return s.String()
}

// Message types for the update function
type suggestionsMsg struct {
	suggestions []CommandSuggestion
	err         error
}

// Function to get command suggestions from the AI
func getCommandSuggestions(query string, conf *config.Config, adapter relay.AIAdapter) tea.Cmd {
	return func() tea.Msg {
		// Build the request
		request := utils.BuildPrompt(query, conf, "terminal")

		// Send the request with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Convert AIAdapter to Adapter to access ChatCompletion
		adapterImpl, ok := adapter.(relay.Adapter)
		if !ok {
			return suggestionsMsg{nil, fmt.Errorf("adapter does not implement required interface")}
		}

		// Create a response channel and error channel
		responseChan := make(chan *dto.OpenAITextResponse, 1)
		errChan := make(chan error, 1)

		// Execute request in goroutine to allow for timeout handling
		go func() {
			response, err := adapterImpl.ChatCompletion(ctx, request)
			if err != nil {
				errChan <- err
				return
			}
			responseChan <- response
		}()

		// Wait for response or timeout
		select {
		case response := <-responseChan:
			if len(response.Choices) == 0 {
				return suggestionsMsg{nil, fmt.Errorf("no suggestions received")}
			}

			// Get the response content
			content := response.Choices[0].Message.StringContent()

			// Parse the JSON response
			var rawSuggestions []map[string]map[string]string
			if err := json.Unmarshal([]byte(content), &rawSuggestions); err != nil {
				// Try to handle non-JSON formatted responses
				// Log original content for debugging
				utils.LogError("Failed to parse suggestions JSON", fmt.Errorf("content: %s, error: %v", content, err))

				// Try to extract commands using a fallback approach
				suggestions := extractCommandsFromText(content)
				if len(suggestions) > 0 {
					return suggestionsMsg{suggestions, nil}
				}

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

			// Log the successful suggestions
			utils.LogInfo(fmt.Sprintf("Generated %d command suggestions for query: %s", len(suggestions), query))

			return suggestionsMsg{suggestions, nil}

		case err := <-errChan:
			return suggestionsMsg{nil, fmt.Errorf("API error: %w", err)}

		case <-time.After(35 * time.Second):
			// Cancel the context if timeout occurs
			cancel()
			return suggestionsMsg{nil, fmt.Errorf("request timed out after 35 seconds")}
		}
	}
}

// Helper function to extract commands from non-JSON responses
func extractCommandsFromText(content string) []CommandSuggestion {
	var suggestions []CommandSuggestion
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for patterns like "command - description" or "command: description"
		for _, separator := range []string{" - ", ": "} {
			if parts := strings.SplitN(line, separator, 2); len(parts) == 2 {
				cmd := strings.TrimSpace(parts[0])
				desc := strings.TrimSpace(parts[1])

				// Skip if it doesn't look like a command
				if len(cmd) > 0 && !strings.HasPrefix(cmd, "#") && !strings.HasPrefix(cmd, "//") {
					suggestions = append(suggestions, CommandSuggestion{
						Command:     cmd,
						Description: desc,
					})
					break
				}
			}
		}
	}

	return suggestions
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
