// Package crypto provides AES-GCM encryption for sensitive fields stored in the database.
//
// Encrypted values are prefixed with "enc:v1:" so the application can detect
// legacy plaintext values and handle them gracefully during the migration window.
// Once all rows are re-encrypted the legacy path can be removed.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

const encPrefix = "enc:v1:"

// Encrypt encrypts plaintext with AES-256-GCM using the provided 32-byte key.
// A random 12-byte nonce is prepended to the ciphertext before base64url encoding.
// The returned string always begins with "enc:v1:".
func Encrypt(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: generate nonce: %w", err)
	}
	// Seal appends the ciphertext+tag to nonce, giving us [nonce | ciphertext | tag].
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encPrefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts a value produced by Encrypt.
// If the value does not begin with "enc:v1:" it is returned as-is — this
// supports legacy plaintext rows during the migration window.
func Decrypt(value string, key []byte) (string, error) {
	if !strings.HasPrefix(value, encPrefix) {
		// Legacy plaintext — callers should re-encrypt on next write.
		return value, nil
	}

	data, err := base64.RawURLEncoding.DecodeString(value[len(encPrefix):])
	if err != nil {
		return "", fmt.Errorf("crypto: base64 decode: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("crypto: ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: decrypt failed (wrong key or corrupted data): %w", err)
	}
	return string(plaintext), nil
}

// ParseKey decodes a 64-character hex string into a 32-byte AES-256 key.
// Generate one with: openssl rand -hex 32
func ParseKey(hexKey string) ([]byte, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be a 64-character hex string: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must decode to 32 bytes (got %d)", len(key))
	}
	return key, nil
}
