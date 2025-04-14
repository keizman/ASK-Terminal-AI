package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/charmbracelet/lipgloss"

	"ask_terminal/config"
	"ask_terminal/dto"
	"ask_terminal/utils"
)

// CommandOption represents a single command suggestion
type CommandOption struct {
	Command     string
	Description string
}

// ExecuteCommand runs a shell command
func ExecuteCommand(command string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("bash", "-c", command)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// LogCommand logs command to history file
func LogCommand(query string, command string) {
	historyFile := "/tmp/askta_Chistory.log"
	entry := fmt.Sprintf("%s|%s\n", query, command)

	f, err := os.OpenFile(historyFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		utils.LogError("Error opening history file", err)
		return
	}
	defer f.Close()

	_, err = f.WriteString(entry)
	if err != nil {
		utils.LogError("Error writing to history file", err)
	}
}

// RenderTitle creates a styled title for terminal UIs
func RenderTitle(title string) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA"))
	return titleStyle.Render(title)
}

// RenderQueryInfo creates a styled query display
func RenderQueryInfo(query string) string {
	if query == "" {
		return ""
	}
	queryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	return queryStyle.Render("Query: "+query) + "\n\n"
}

// RenderError creates a styled error message
func RenderError(err error) string {
	if err == nil {
		return ""
	}
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	return errorStyle.Render(fmt.Sprintf("Error: %v\n", err))
}

// RenderHelpText creates styled help text
func RenderHelpText(text string) string {
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	return helpStyle.Render(text)
}

// BuildTerminalModePrompt creates a prompt for terminal mode
func BuildTerminalModePrompt(query string, conf *config.Config) *dto.GeneralOpenAIRequest {
	return &dto.GeneralOpenAIRequest{
		Model: conf.ModelName,
		Messages: []dto.Message{
			{
				Role:    "system",
				Content: []byte(`"You are a command line expert. Format responses as JSON."`),
			},
			{
				Role:    "user",
				Content: []byte(`"` + query + `"`),
			},
		},
		ResponseFormat: &dto.ResponseFormat{
			Type: "json_object",
		},
	}
}

// BuildConversationModePrompt creates a prompt for conversation mode
func BuildConversationModePrompt(query string, conf *config.Config) *dto.GeneralOpenAIRequest {
	return &dto.GeneralOpenAIRequest{
		Model: conf.ModelName,
		Messages: []dto.Message{
			{
				Role:    "system",
				Content: []byte(`"You are a helpful assistant."`),
			},
			{
				Role:    "user",
				Content: []byte(`"` + query + `"`),
			},
		},
	}
}
