package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	encryptionPrefix = "encry_"
)

// EncryptAPIKey encrypts the API key if it's not already encrypted
func EncryptAPIKey(apiKey string) (string, error) {
	// Check if already encrypted
	if len(apiKey) > 6 && apiKey[:6] == encryptionPrefix {
		return apiKey, nil
	}

	// Get or create encryption key
	deviceKey, err := getOrCreateDeviceKey()
	if err != nil {
		return "", fmt.Errorf("failed to get device key: %w", err)
	}

	// Encrypt the API key
	encrypted, err := encrypt(apiKey, deviceKey)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt API key: %w", err)
	}

	return encryptionPrefix + encrypted, nil
}

// DecryptAPIKey decrypts the API key if it's encrypted
func DecryptAPIKey(encryptedKey string) (string, error) {
	// Check if encrypted with our prefix
	if len(encryptedKey) <= 6 || encryptedKey[:6] != encryptionPrefix {
		return encryptedKey, nil
	}

	// Extract the encrypted part
	encryptedPart := encryptedKey[6:]

	// Get the encryption key (should be the same key used for encryption)
	deviceKey, err := getOrCreateDeviceKey()
	if err != nil {
		return "", fmt.Errorf("failed to get device key: %w", err)
	}

	// Decrypt the API key
	decrypted, err := decrypt(encryptedPart, deviceKey)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt API key: %w", err)
	}

	return decrypted, nil
}

// getOrCreateDeviceKey gets an existing key or creates and stores a new one
func getOrCreateDeviceKey() ([]byte, error) {
	// Get path to store the encryption key
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	keyDir := filepath.Join(homeDir, ".config", "askta")
	keyPath := filepath.Join(keyDir, ".encryption-key")

	// Try to read existing key
	if keyData, err := os.ReadFile(keyPath); err == nil && len(keyData) >= 32 {
		// Use existing key
		return keyData[:32], nil
	}

	// Generate new random key
	key := make([]byte, 32) // 256-bit key
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}

	// Ensure directory exists
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return nil, err
	}

	// Store the key for future use
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return nil, err
	}

	return key, nil
}

// encrypt encrypts data using AES-GCM
func encrypt(plaintext string, key []byte) (string, error) {
	// Create a new AES cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Create a GCM cipher mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Generate a random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Encrypt and authenticate the data
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Return base64-encoded ciphertext
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts data using AES-GCM
func decrypt(encodedCiphertext string, key []byte) (string, error) {
	// Decode base64
	ciphertext, err := base64.StdEncoding.DecodeString(encodedCiphertext)
	if err != nil {
		return "", err
	}

	// Create a new AES cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Create a GCM cipher mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Extract the nonce from the beginning of the ciphertext
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt and verify the ciphertext
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
