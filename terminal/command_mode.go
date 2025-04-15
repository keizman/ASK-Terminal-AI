package terminal

import (
	"ask_terminal/config"
	"ask_terminal/dto"
	"ask_terminal/relay"
	"ask_terminal/service"
	"ask_terminal/utils"
	"bytes"
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
	Command        string // The original command
	EditedCommand  string // The edited version of the command
	Description    string
	CursorPosition int // Track cursor position for each command
}

// VirtualTerminalModel represents the model for the virtual terminal
type VirtualTerminalModel struct {
	query             string
	input             textinput.Model
	suggestions       []CommandSuggestion
	selected          int
	loading           bool
	cursorVisible     bool
	queryMode         bool // true when entering a query, false when editing commands
	directCommandMode bool // true when directly entering commands to execute
	err               error
	config            *config.Config
	logger            *utils.Logger
	adapter           relay.AIAdapter
	commandResult     string // stores the result of executed commands
	showResult        bool   // whether to show command result
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
		input:             ti,
		config:            conf,
		logger:            logger,
		adapter:           adapter,
		queryMode:         true,
		directCommandMode: false,
		cursorVisible:     true,
		showResult:        false,
	}
}

// Init initializes the model
func (m VirtualTerminalModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		blinkCursor(),
	)
}

// Update function with DEL key support and mode toggling
func (m VirtualTerminalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle special keys first
		switch msg.String() {
		case "ctrl+c", "ctrl+d", "ctrl+z", "ctrl+q":
			return m, tea.Quit

		case "tab":
			// Toggle between modes: query -> direct command -> suggestions (if available)
			if m.loading {
				return m, nil
			}

			if m.queryMode {
				m.queryMode = false
				m.directCommandMode = true
				m.input.Placeholder = "Enter command to execute directly..."
				return m, nil
			} else if m.directCommandMode {
				m.directCommandMode = false
				if len(m.suggestions) > 0 {
					m.queryMode = false // Go to suggestion selection mode
				} else {
					m.queryMode = true // Go back to query mode if no suggestions
				}
				m.input.Placeholder = "Type your command query here..."
				return m, nil
			} else if len(m.suggestions) > 0 {
				// From suggestion mode to query mode
				m.queryMode = true
				m.directCommandMode = false
				m.input.Placeholder = "Type your command query here..."
				return m, nil
			}

		case "up", "down":
			if !m.loading && len(m.suggestions) > 0 && !m.queryMode && !m.directCommandMode {
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
				if m.showResult {
					// Start a new query session instead of just hiding the result
					m.showResult = false
					m.commandResult = ""
					m.suggestions = nil
					m.queryMode = true
					m.directCommandMode = false
					m.input.SetValue("")
					m.input.Focus()
					m.input.Placeholder = "Type your command query here..."
					m.query = ""
					return m, nil
				} else if len(m.suggestions) > 0 && !m.queryMode && !m.directCommandMode {
					// Execute the selected command
					command := m.suggestions[m.selected].EditedCommand
					return m, tea.Sequence(
						executeCommand(command),
						func() tea.Msg { return executeResultMsg{} },
					)
				} else if m.directCommandMode {
					// Execute direct command
					command := m.input.Value()
					if command != "" {
						m.input.SetValue("")
						return m, tea.Sequence(
							executeCommand(command),
							func() tea.Msg { return executeResultMsg{} },
						)
					}
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
			// New behavior for ESC key when showing results
			if !m.loading && m.showResult {
				// Hide result and go back to suggestion mode without losing suggestions
				m.showResult = false
				m.commandResult = ""
				if len(m.suggestions) > 0 {
					m.queryMode = false
					m.directCommandMode = false
				} else {
					m.queryMode = true
				}
				return m, nil
			}

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

		// Pass inputs to textinput when in appropriate modes
		if m.queryMode || m.directCommandMode {
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
		return blinkCursor()

	case executeResultMsg:
		// Show the command result instead of quitting
		m.showResult = true
		if m.directCommandMode {
			// Stay in direct command mode
			m.input.Focus()
		} else {
			// Go back to query mode after executing a suggestion
			m.queryMode = true
			m.input.Focus()
			m.input.SetValue("")
		}
		return m, nil

	case commandOutputMsg:
		m.commandResult = string(msg)
		return m, nil
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

// Execute command
func executeCommand(command string) tea.Cmd {
	return func() tea.Msg {
		// Split the command into executable and arguments
		parts := strings.Fields(command)
		if len(parts) == 0 {
			return commandOutputMsg("Error: Empty command")
		}

		// Create a command with captured output
		cmd := exec.Command(parts[0], parts[1:]...)

		// Capture both stdout and stderr
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()

		// Build the output
		var output strings.Builder
		output.WriteString("\n")

		if stdout.Len() > 0 {
			output.WriteString(stdout.String())
		}

		if stderr.Len() > 0 {
			output.WriteString("\nError output:\n")
			output.WriteString(stderr.String())
		}

		if err != nil && stderr.Len() == 0 {
			output.WriteString(fmt.Sprintf("\nCommand error: %v", err))
		}

		output.WriteString("\n")
		return commandOutputMsg(output.String())
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

	// Show command result if available
	if m.showResult && m.commandResult != "" {
		s.WriteString(color.CyanString("Command Output:"))
		s.WriteString(m.commandResult)
		s.WriteString(color.YellowString("\n[Press Enter for new query or ESC to return to suggestions]\n\n"))
		return s.String()
	}

	// Input field or query display based on mode
	if m.loading {
		s.WriteString(fmt.Sprintf("> %s\n\n", m.query))
		s.WriteString("Loading suggestions...\n\n")
		return s.String()
	}

	// Display current mode
	if m.directCommandMode {
		s.WriteString(color.GreenString("[DIRECT COMMAND MODE] "))
		s.WriteString(fmt.Sprintf("> %s\n\n", m.input.View()))
	} else if m.queryMode {
		s.WriteString(color.BlueString("[QUERY MODE] "))
		if len(m.suggestions) > 0 {
			s.WriteString(fmt.Sprintf("> %s\n\n", m.query))
		} else {
			s.WriteString(fmt.Sprintf("> %s\n\n", m.input.View()))
		}
	} else {
		s.WriteString(color.MagentaString("[SUGGESTION MODE] "))
		s.WriteString(fmt.Sprintf("> %s\n\n", m.query))
	}

	// Command suggestions with direct editing
	if len(m.suggestions) > 0 && !m.directCommandMode {
		for i, suggestion := range m.suggestions {
			// Highlight selected suggestion
			prefix := "  "
			if i == m.selected {
				prefix = "> "
			}

			// Display command with cursor
			commandDisplay := suggestion.EditedCommand
			if i == m.selected {
				// Insert cursor at the right position
				if m.cursorVisible {
					pos := suggestion.CursorPosition
					if pos >= 0 && pos <= len(commandDisplay) {
						commandDisplay = commandDisplay[:pos] + "|" + commandDisplay[pos:]
					}
				}

				// Highlight selected command
				commandStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFF00")).Bold(true)
				s.WriteString(prefix + commandStyle.Render(commandDisplay) + "\n")
			} else {
				s.WriteString(prefix + commandDisplay + "\n")
			}

			// Display description with a different color
			descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA")).Italic(true)
			s.WriteString("    " + descStyle.Render(suggestion.Description) + "\n\n")
		}
	}

	// Instructions based on current state
	if m.directCommandMode {
		s.WriteString("\n" + color.YellowString("Type a command and press Enter to execute, [Tab] to switch modes, [q] to quit\n"))
	} else if !m.queryMode {
		s.WriteString("\n" + color.YellowString("Edit directly, use ↑/↓ to switch commands, Enter to execute, [Tab] to switch modes, [Esc] to cancel, [q] to quit\n"))
	} else {
		s.WriteString("\n" + color.YellowString("Type a query for command suggestions, [Tab] to switch to direct command mode, [q] to quit\n"))
	}

	return s.String()
}

// Message types for the update function
type suggestionsMsg struct {
	suggestions []CommandSuggestion
	err         error
}

type executeResultMsg struct{}

type commandOutputMsg string

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
