package terminal

import (
	"ask_terminal/config"
	"ask_terminal/dto"
	"ask_terminal/service"
	"ask_terminal/utils"
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"ask_terminal/relay"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// ChatModel represents the state for conversation mode
type ChatModel struct {
	query     string
	content   string
	viewport  viewport.Model
	isLoading bool
	config    *config.Config
	err       error
}

// NewChatModel creates the initial state for chat mode
func NewChatModel(query string, conf *config.Config) ChatModel {
	// Configure viewport for scrollable content
	vp := viewport.New(80, 20)
	// Use an empty style not nil
	vp.Style = lipgloss.Style{}

	// Log query
	utils.LogInfo("Conversation query: " + query)

	return ChatModel{
		query:     query,
		content:   "Loading response...",
		viewport:  vp,
		isLoading: true,
		config:    conf,
	}
}

// Init initializes the TUI model
func (m ChatModel) Init() tea.Cmd {
	return fetchAIResponse(m.query, m.config)
}

// ChatResponseMsg represents a message with AI response content
type ChatResponseMsg struct {
	content string
	err     error
}

// fetchAIResponse sends a request to the AI service and returns the response
func fetchAIResponse(query string, conf *config.Config) tea.Cmd {
	return func() tea.Msg {
		// Get the appropriate adapter
		adapter, err := relay.NewAdapter(conf)
		if err != nil {
			return ChatResponseMsg{"Error initializing AI adapter: " + err.Error(), err}
		}

		// Build request using the utils package
		request := utils.BuildPrompt(query, conf, "chat")
		// Execute request
		ctx := context.Background()
		response, err := adapter.ChatCompletion(ctx, request)
		if err != nil {
			return ChatResponseMsg{"Error communicating with AI: " + err.Error(), err}
		}

		if len(response.Choices) == 0 {
			return ChatResponseMsg{"No response content received from AI.", nil}
		}

		return ChatResponseMsg{response.Choices[0].Message.StringContent(), nil}
	}
}

// Update handles UI updates
func (m ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}

		// Handle viewport scrolling
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case tea.WindowSizeMsg:
		// Adjust viewport size when window is resized
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 6
		return m, nil

	case ChatResponseMsg:
		m.isLoading = false
		if msg.err != nil {
			m.err = msg.err
			m.content = fmt.Sprintf("Error: %v", msg.err)
			utils.LogSystemResponse(0, false, m.content)
		} else {
			m.content = msg.content
			utils.LogSystemResponse(len(m.content), true, m.content)
		}

		// Set content in viewport for scrolling
		m.viewport.SetContent(m.content)
		return m, nil
	}

	return m, nil
}

// View renders the UI
func (m ChatModel) View() string {
	var s strings.Builder

	// Title using shared function
	s.WriteString(RenderTitle("ASK Terminal AI - Conversation Mode") + "\n\n")

	// Query display using shared function
	s.WriteString(RenderQueryInfo(m.query))

	if m.isLoading {
		s.WriteString("Loading response...\n")
	} else if m.err != nil {
		// Error display using shared function
		s.WriteString(RenderError(m.err))
	} else {
		// Content display in viewport
		s.WriteString(m.viewport.View() + "\n\n")

		// Help text using shared function
		s.WriteString(RenderHelpText("Press q to exit • ↑/↓ to scroll\n"))
	}

	return s.String()
}

// StartConversationMode starts conversation mode with an initial query
func StartConversationMode(query string, conf *config.Config) {
	utils.LogInfo(fmt.Sprintf("Starting Chat Mode with query: %s", query))
	// Get the appropriate adapter
	adapter, err := relay.NewAdapter(conf)
	if err != nil {
		fmt.Printf("Error initializing AI adapter: %v\n", err)
		utils.LogError("Error initializing AI adapter", err)
		os.Exit(1)
	}

	// Build request using the utils package
	request := utils.BuildPrompt(query, conf, "chat")

	// Execute request
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Print a "thinking" message
	fmt.Println("Processing your request...")

	// Use streaming response by default
	stream, err := adapter.ChatCompletionStream(ctx, request)
	if err != nil {
		fmt.Printf("Error communicating with AI: %v\n", err)
		utils.LogError("Error communicating with AI", err)
		os.Exit(1)
	}

	// Initialize markdown renderer
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		// Fall back to plain text if renderer can't be created
		fmt.Println("\nResponse:")
		for response := range stream {
			if len(response.Choices) > 0 && response.Choices[0].Delta.Content != nil {
				fmt.Print(*response.Choices[0].Delta.Content)
				os.Stdout.Sync()
			}
		}
		fmt.Println()
		return
	}

	// Create buffer to collect content
	var buffer bytes.Buffer

	// Process response
	fmt.Println("\nResponse:")

	// Simple streaming output instead of trying to clear the screen
	for response := range stream {
		if len(response.Choices) > 0 && response.Choices[0].Delta.Content != nil {
			content := *response.Choices[0].Delta.Content
			buffer.WriteString(content)
			fmt.Print(content)
			os.Stdout.Sync()
		}
	}

	// Final render with markdown formatting
	fmt.Println("\n\n--- Formatted Response ---")
	rendered, _ := renderer.Render(buffer.String())
	fmt.Println(rendered)
	utils.LogInfo(fmt.Sprintf("End of Chat Mode with answer: %s", rendered))
}

// ChatMode handles conversations with AI
type ChatMode struct {
	aiService *service.AIService
	model     string
}

// NewChatMode creates a new chat mode with the given AI service
func NewChatMode(aiService *service.AIService, model string) *ChatMode {
	return &ChatMode{
		aiService: aiService,
		model:     model,
	}
}

// ProcessQuery sends a query to the AI service and prints the response
// Stream is now true by default
func (c *ChatMode) ProcessQuery(query string, systemPrompt string, stream ...bool) error {
	messages := []dto.Message{
		{
			Role:    "system",
			Content: []byte(`"` + systemPrompt + `"`),
		},
		{
			Role:    "user",
			Content: []byte(`"` + query + `"`),
		},
	}

	ctx := context.Background()

	// Default to streaming if not explicitly set to false
	useStream := true
	if len(stream) > 0 {
		useStream = stream[0]
	}

	if useStream {
		return c.handleStreamingResponse(ctx, messages)
	}
	return c.handleNonStreamingResponse(ctx, messages)
}

// renderMarkdown converts Markdown content to terminal-friendly styled text
func renderMarkdown(markdown string) (string, error) {
	renderer, err := glamour.NewTermRenderer(
		// Use the default dark style
		glamour.WithAutoStyle(),
		// Ensure compatibility with standard terminals
		glamour.WithWordWrap(80),
	)
	if err != nil {
		return markdown, err
	}
	rendered, err := renderer.Render(markdown)
	if err != nil {
		return markdown, err
	}
	return rendered, nil
}

// handleNonStreamingResponse processes a non-streaming response
func (c *ChatMode) handleNonStreamingResponse(ctx context.Context, messages []dto.Message) error {
	response, err := c.aiService.SendChatRequest(ctx, messages, c.model)
	if err != nil {
		return err
	}

	if len(response.Choices) > 0 {
		content := response.Choices[0].Message.StringContent()

		// Render markdown to terminal-friendly output
		rendered, err := renderMarkdown(content)
		if err != nil {
			// If rendering fails, fall back to plain content
			fmt.Print(content)
		} else {
			fmt.Print(rendered)
		}
	}
	return nil
}

// handleStreamingResponse processes a streaming response
func (c *ChatMode) handleStreamingResponse(ctx context.Context, messages []dto.Message) error {
	responseStream, err := c.aiService.SendStreamingChatRequest(ctx, messages, c.model)
	if err != nil {
		return err
	}

	// Set up a buffer for the final rendering
	var buffer bytes.Buffer

	// Print indicator
	fmt.Println("Processing your request...")
	fmt.Println("\nResponse:")

	// Process the streaming response - simple streaming output
	for response := range responseStream {
		if len(response.Choices) > 0 && response.Choices[0].Delta.Content != nil {
			content := *response.Choices[0].Delta.Content
			buffer.WriteString(content)
			fmt.Print(content)
			os.Stdout.Sync()
		}
	}

	// Final render with markdown formatting
	fmt.Println("\n\n--- Formatted Response ---")
	rendered, _ := renderMarkdown(buffer.String())
	fmt.Println(rendered)

	return nil
}
