// Package cdn provides CDN download utilities.
// This is a complete Go reimplementation of the TypeScript pic-decrypt.ts module.
package cdn

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Downloader handles CDN downloads with AES-128-ECB decryption.
type Downloader struct {
	cdnBaseURL string
	httpClient *http.Client
}

// NewDownloader creates a new downloader.
func NewDownloader(cdnBaseURL string) *Downloader {
	if cdnBaseURL == "" {
		cdnBaseURL = DefaultCDNBaseURL
	}
	return &Downloader{
		cdnBaseURL: cdnBaseURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// DownloadAndDecrypt downloads and decrypts a file from CDN.
func (d *Downloader) DownloadAndDecrypt(ctx context.Context, encryptedQueryParam, aesKeyBase64 string) ([]byte, error) {
	key, err := parseAESKey(aesKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AES key: %w", err)
	}

	encrypted, err := d.download(ctx, encryptedQueryParam)
	if err != nil {
		return nil, err
	}

	decrypted, err := DecryptAES128ECB(encrypted, key)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return decrypted, nil
}

// DownloadPlain downloads unencrypted data from CDN.
func (d *Downloader) DownloadPlain(ctx context.Context, encryptedQueryParam string) ([]byte, error) {
	return d.download(ctx, encryptedQueryParam)
}

// download fetches raw bytes from CDN.
func (d *Downloader) download(ctx context.Context, encryptedQueryParam string) ([]byte, error) {
	cdnURL := d.buildDownloadURL(encryptedQueryParam)

	req, err := http.NewRequestWithContext(ctx, "GET", cdnURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("CDN download failed: %w", err)
	}
	defer resp.Body.Close()

	if !isSuccess(resp.StatusCode) {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("CDN download %s: %s", resp.Status, string(body))
	}

	return io.ReadAll(resp.Body)
}

// isSuccess checks if HTTP status is successful.
func isSuccess(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

// buildDownloadURL builds the CDN download URL.
func (d *Downloader) buildDownloadURL(encryptedQueryParam string) string {
	return fmt.Sprintf("%s/download?encrypted_query_param=%s",
		d.cdnBaseURL,
		url.QueryEscape(encryptedQueryParam))
}

// parseAESKey parses the AES key from base64 encoding.
// Supports two formats:
//   - base64(raw 16 bytes) → images (from image_item.aeskey or media.aes_key)
//   - base64(hex string of 16 bytes) → file/voice/video
func parseAESKey(aesKeyBase64 string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(aesKeyBase64)
	if err != nil {
		// Try URL-safe base64
		decoded, err = base64.RawURLEncoding.DecodeString(aesKeyBase64)
		if err != nil {
			return nil, fmt.Errorf("failed to base64 decode AES key: %w", err)
		}
	}

	// Raw 16 bytes
	if len(decoded) == 16 {
		return decoded, nil
	}

	// Hex-encoded key (32 ASCII chars)
	if len(decoded) == 32 && isHexString(string(decoded)) {
		key, err := hex.DecodeString(string(decoded))
		if err != nil {
			return nil, fmt.Errorf("failed to hex decode AES key: %w", err)
		}
		return key, nil
	}

	return nil, fmt.Errorf("AES key must decode to 16 raw bytes or 32-char hex string, got %d bytes", len(decoded))
}

// isHexString checks if a string is a valid hex string.
func isHexString(s string) bool {
	s = strings.ToLower(s)
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// DownloadImage downloads and decrypts an image from CDN.
// The aesKeyHex is the hex-encoded AES key from ImageItem.AESKey.
func (d *Downloader) DownloadImage(ctx context.Context, encryptedQueryParam, aesKeyHex string) ([]byte, error) {
	// Convert hex to base64
	aesKey, err := hex.DecodeString(aesKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to hex decode AES key: %w", err)
	}
	aesKeyBase64 := base64.StdEncoding.EncodeToString(aesKey)
	return d.DownloadAndDecrypt(ctx, encryptedQueryParam, aesKeyBase64)
}

// DownloadMedia downloads and decrypts any media type from CDN.
// Uses the appropriate key format based on the key string format.
func (d *Downloader) DownloadMedia(ctx context.Context, encryptedQueryParam, aesKey string) ([]byte, error) {
	// Try to detect key format
	if isHexString(aesKey) && len(aesKey) == 32 {
		// Hex-encoded key (from ImageItem.AESKey)
		return d.DownloadImage(ctx, encryptedQueryParam, aesKey)
	}

	// Base64-encoded key
	return d.DownloadAndDecrypt(ctx, encryptedQueryParam, aesKey)
}
