package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CommandHistoryItem represents a history log entry
type CommandHistoryItem struct {
	Timestamp string            `json:"timestamp"`
	Query     string            `json:"query"`
	Commands  map[string]string `json:"commands"`
}

// Logger provides logging functionality
type Logger struct {
	CommandHistoryPath string
	ApplicationLogPath string
}

// NewLogger creates a new logger instance
func NewLogger() *Logger {
	tempDir := os.TempDir()
	return &Logger{
		CommandHistoryPath: filepath.Join(tempDir, "askta_Chistory.log"),
		ApplicationLogPath: filepath.Join(tempDir, "askta_run.log"),
	}
}

// LogCommand records a command suggestion to history
func (l *Logger) LogCommand(query string, commands map[string]string) error {
	// Create history item
	item := CommandHistoryItem{
		Timestamp: time.Now().Format(time.RFC3339),
		Query:     query,
		Commands:  commands,
	}

	// Marshal to JSON
	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal history item: %w", err)
	}

	// Append to file
	f, err := os.OpenFile(l.CommandHistoryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open history file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(string(data) + "\n"); err != nil {
		return fmt.Errorf("failed to write to history file: %w", err)
	}

	return nil
}

// LogApplication logs application events
func (l *Logger) LogApplication(message string) error {
	logEntry := fmt.Sprintf("[%s] %s\n", time.Now().Format(time.RFC3339), message)

	f, err := os.OpenFile(l.ApplicationLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(logEntry); err != nil {
		return fmt.Errorf("failed to write to log file: %w", err)
	}

	return nil
}

// GetRecentCommands retrieves the most recent command history entries
func (l *Logger) GetRecentCommands(limit int) ([]CommandHistoryItem, error) {
	if limit <= 0 {
		limit = 1000 // Default to 1000 entries if no limit is provided
	}

	// Read the history file
	data, err := os.ReadFile(l.CommandHistoryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []CommandHistoryItem{}, nil // Return empty list if file doesn't exist
		}
		return nil, fmt.Errorf("failed to read history file: %w", err)
	}

	// Parse each line as a JSON object
	lines := strings.Split(string(data), "\n")
	var items []CommandHistoryItem

	for i := len(lines) - 1; i >= 0 && len(items) < limit; i-- {
		if lines[i] == "" {
			continue
		}

		var item CommandHistoryItem
		if err := json.Unmarshal([]byte(lines[i]), &item); err != nil {
			l.LogApplication(fmt.Sprintf("Failed to parse history entry: %v", err))
			continue
		}

		items = append(items, item)
	}

	return items, nil
}

// LogInfo logs an informational message
func LogInfo(message string) {
	logger := NewLogger()
	_ = logger.LogApplication("[INFO] " + message)
}

// LogUserRequest logs a user request
func LogUserRequest(query string, mode string) {
	logger := NewLogger()
	_ = logger.LogApplication(fmt.Sprintf("[USER REQUEST] Mode: %s, Query: %s", mode, query))
}

// LogSystemResponse logs an AI response
func LogSystemResponse(responseLength int, success bool) {
	logger := NewLogger()
	status := "SUCCESS"
	if !success {
		status = "FAILED"
	}
	_ = logger.LogApplication(fmt.Sprintf("[SYSTEM RESPONSE] Status: %s, Response length: %d chars", status, responseLength))
}

// LogCommandExecution logs when a command is executed
func LogCommandExecution(command string) {
	logger := NewLogger()
	_ = logger.LogApplication(fmt.Sprintf("[COMMAND EXECUTED] %s", command))
}
