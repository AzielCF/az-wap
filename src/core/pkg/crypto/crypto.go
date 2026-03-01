package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
)

var encryptionKey []byte

// SetEncryptionKey sets the global encryption key. It must be 16, 24, or 32 bytes.
func SetEncryptionKey(key string) error {
	// If the key provided is less than 32 bytes, we pad it or hash it.
	// For simplicity, let's just use the first 32 bytes if it's long enough,
	// or pad with 0s if it's short.
	// In production, use a proper KDF like Argon2 or PBKDF2.

	finalKey := make([]byte, 32) // AES-256
	copy(finalKey, []byte(key))
	encryptionKey = finalKey
	return nil
}

// Encrypt encrypts a plain text string using AES-GCM and returns a base64 encoded string.
func Encrypt(plainText string) (string, error) {
	if len(encryptionKey) == 0 {
		return plainText, nil // Encryption not configured, return as is (WARN: insecure)
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plainText), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64 encoded string using AES-GCM.
func Decrypt(cipherText string) (string, error) {
	if len(encryptionKey) == 0 {
		return cipherText, nil // No key, assume plain text (could fail if it IS encrypted)
	}

	// If it doesn't look like base64, maybe it's legacy plain text
	data, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return cipherText, nil // Fallback to plain text
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return cipherText, nil // Too short to be encrypted with nonce
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
