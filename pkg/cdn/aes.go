// Package cdn provides CDN utilities for WeChat media transfer.
// This is a complete Go reimplementation of the TypeScript cdn module.
package cdn

import (
	"crypto/aes"
	"crypto/rand"
	"crypto/md5"
	"encoding/hex"
	"fmt"
)

// EncryptAES128ECB encrypts plaintext using AES-128-ECB with PKCS7 padding.
func EncryptAES128ECB(plaintext, key []byte) ([]byte, error) {
	if len(key) != 16 {
		// Truncate or pad key to 16 bytes
		if len(key) > 16 {
			key = key[:16]
		} else {
			padded := make([]byte, 16)
			copy(padded, key)
			key = padded
		}
	}

	// PKCS7 padding
	blockSize := aes.BlockSize
	padding := blockSize - len(plaintext)%blockSize
	padded := make([]byte, len(plaintext)+padding)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padding)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, len(padded))
	for i := 0; i < len(padded); i += blockSize {
		block.Encrypt(ciphertext[i:i+blockSize], padded[i:i+blockSize])
	}

	return ciphertext, nil
}

// DecryptAES128ECB decrypts ciphertext using AES-128-ECB with PKCS7 padding.
func DecryptAES128ECB(ciphertext, key []byte) ([]byte, error) {
	if len(key) != 16 {
		if len(key) > 16 {
			key = key[:16]
		} else {
			padded := make([]byte, 16)
			copy(padded, key)
			key = padded
		}
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not a multiple of block size")
	}

	plaintext := make([]byte, len(ciphertext))
	for i := 0; i < len(ciphertext); i += aes.BlockSize {
		block.Decrypt(plaintext[i:i+aes.BlockSize], ciphertext[i:i+aes.BlockSize])
	}

	// Remove PKCS7 padding
	if len(plaintext) > 0 {
		padding := int(plaintext[len(plaintext)-1])
		if padding > 0 && padding <= aes.BlockSize {
			// Validate padding
			for i := len(plaintext) - padding; i < len(plaintext); i++ {
				if plaintext[i] != byte(padding) {
					return plaintext, nil // Invalid padding, return as-is
				}
			}
			plaintext = plaintext[:len(plaintext)-padding]
		}
	}

	return plaintext, nil
}

// AES128ECBPaddedSize returns the ciphertext size for given plaintext size.
func AES128ECBPaddedSize(plaintextSize int) int {
	return ((plaintextSize + aes.BlockSize) / aes.BlockSize) * aes.BlockSize
}

// GenerateAESKey generates a random 16-byte AES key.
func GenerateAESKey() ([]byte, error) {
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

// GenerateFileKey generates a random 16-byte file key (hex-encoded).
func GenerateFileKey() (string, error) {
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}

// MD5Hash computes the MD5 hash of data and returns hex string.
func MD5Hash(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// ECBEncryptor provides AES-128-ECB encryption/decryption.
type ECBEncryptor struct {
	key []byte
}

// NewECBEncryptor creates a new ECB encryptor.
func NewECBEncryptor(key []byte) *ECBEncryptor {
	if len(key) > 16 {
		key = key[:16]
	} else if len(key) < 16 {
		padded := make([]byte, 16)
		copy(padded, key)
		key = padded
	}
	return &ECBEncryptor{key: key}
}

// Encrypt encrypts plaintext.
func (e *ECBEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	return EncryptAES128ECB(plaintext, e.key)
}

// Decrypt decrypts ciphertext.
func (e *ECBEncryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	return DecryptAES128ECB(ciphertext, e.key)
}

// Key returns the encryption key.
func (e *ECBEncryptor) Key() []byte {
	return e.key
}

// KeyHex returns the hex-encoded key.
func (e *ECBEncryptor) KeyHex() string {
	return hex.EncodeToString(e.key)
}
