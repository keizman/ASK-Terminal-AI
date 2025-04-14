package terminal

import (
	"ask_terminal/config"
	"ask_terminal/dto"
	"ask_terminal/service"
	"ask_terminal/utils"
	"context"
	"fmt"
	"os"
	"strings"

	"ask_terminal/relay"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
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
		} else {
			m.content = msg.content
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
	p := tea.NewProgram(NewChatModel(query, conf))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running conversation UI: %v\n", err)
		os.Exit(1)
	}
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
func (c *ChatMode) ProcessQuery(query string, systemPrompt string, stream bool) error {
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

	if stream {
		return c.handleStreamingResponse(ctx, messages)
	}
	return c.handleNonStreamingResponse(ctx, messages)
}

// handleNonStreamingResponse processes a non-streaming response
func (c *ChatMode) handleNonStreamingResponse(ctx context.Context, messages []dto.Message) error {
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

// handleStreamingResponse processes a streaming response
func (c *ChatMode) handleStreamingResponse(ctx context.Context, messages []dto.Message) error {
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
