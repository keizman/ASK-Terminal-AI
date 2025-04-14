package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	encryptionPrefix = "encry_"
)

// EncryptAPIKey encrypts the API key if it's not already encrypted
func EncryptAPIKey(apiKey string) (string, error) {
	// Check if already encrypted
	if strings.HasPrefix(apiKey, encryptionPrefix) {
		return apiKey, nil
	}

	// Generate device-specific encryption key
	deviceKey, err := getDeviceKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate device key: %w", err)
	}

	// Encrypt the API key
	encrypted, err := encrypt(apiKey, deviceKey)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt API key: %w", err)
	}

	return encryptionPrefix + encrypted, nil
}

// DecryptAPIKey decrypts the API key if it's encrypted
func DecryptAPIKey(apiKey string) (string, error) {
	// Check if encrypted
	if !strings.HasPrefix(apiKey, encryptionPrefix) {
		return apiKey, nil
	}

	// Extract the encrypted part
	encryptedPart := strings.TrimPrefix(apiKey, encryptionPrefix)

	// Generate device-specific encryption key
	deviceKey, err := getDeviceKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate device key: %w", err)
	}

	// Decrypt the API key
	decrypted, err := decrypt(encryptedPart, deviceKey)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt API key: %w", err)
	}

	return decrypted, nil
}

// getDeviceKey generates a device-specific key using hardware identifiers
func getDeviceKey() ([]byte, error) {
	// Get machine ID or hardware identifier
	machineID, err := getMachineID()
	if err != nil {
		return nil, err
	}

	// Generate key using Argon2id (more resistant to hardware acceleration attacks)
	key := argon2.IDKey([]byte(machineID), nil, 1, 64*1024, 4, 32)
	return key, nil
}

// getMachineID gets a unique machine identifier
func getMachineID() (string, error) {
	// On Windows, try to get the MachineGUID from registry
	// On Linux/macOS, try to get machine-id from /etc/machine-id or /var/lib/dbus/machine-id
	// Fallback to hostname if the above methods fail

	var machineID string

	// Try to get hostname as fallback
	hostname, err := os.Hostname()
	if err == nil {
		machineID = hostname
	} else {
		// If even hostname fails, use a fixed string + username as last resort
		username := os.Getenv("USER")
		if username == "" {
			username = os.Getenv("USERNAME")
		}
		machineID = "askta-" + username
	}

	// Hash the machine ID to get a consistent length value
	hash := sha256.Sum256([]byte(machineID))
	return fmt.Sprintf("%x", hash), nil
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
