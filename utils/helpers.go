package utils

import (
	"log"
	"os"
)

// LogInfo logs an informational message
func LogInfo(message string) {
	logger := NewLogger()
	_ = logger.LogApplication("[INFO] " + message)
}

// LogError logs an error message
func LogError(message string, err error) {
	logger := NewLogger()
	if err != nil {
		_ = logger.LogApplication("[ERROR] " + message + ": " + err.Error())
	} else {
		_ = logger.LogApplication("[ERROR] " + message)
	}
}

// Ptr returns a pointer to the given value
func Ptr[T any](v T) *T {
	return &v
}

// GetDefaultConfigPath returns the default configuration path
func GetDefaultConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Could not determine user home directory: %v", err)
		return "/etc/askta/config.yaml"
	}
	return homeDir + "/.config/askta/config.yaml"
}
