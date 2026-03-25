// Package cdn provides CDN upload utilities.
// This is a complete Go reimplementation of the TypeScript upload.ts module.
package cdn

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/openclaw/weixin-sdk-go/pkg/api"
)

const (
	UploadMaxRetries   = 3
	DefaultCDNBaseURL  = "https://novac2c.cdn.weixin.qq.com/c2c"
)

// UploadedFileInfo represents information about an uploaded file.
type UploadedFileInfo struct {
	FileKey                     string
	DownloadEncryptedQueryParam string
	AESKey                      string // hex-encoded
	FileSize                    int    // plaintext size
	FileSizeCiphertext          int    // ciphertext size after encryption
}

// Uploader handles CDN uploads with AES-128-ECB encryption.
type Uploader struct {
	client     *api.Client
	cdnBaseURL string
	httpClient *http.Client
}

// NewUploader creates a new uploader.
func NewUploader(client *api.Client, cdnBaseURL string) *Uploader {
	if cdnBaseURL == "" {
		cdnBaseURL = DefaultCDNBaseURL
	}
	return &Uploader{
		client:     client,
		cdnBaseURL: cdnBaseURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// UploadFile uploads a file to CDN and returns the file info.
// This implements the complete upload flow:
// 1. Calculate plaintext size and MD5
// 2. Generate AES key
// 3. Calculate ciphertext size
// 4. Get upload URL from API
// 5. Encrypt and upload to CDN
func (u *Uploader) UploadFile(ctx context.Context, plaintext []byte, toUserID string, mediaType api.UploadMediaType) (*UploadedFileInfo, error) {
	// Step 1: Calculate hashes
	rawSize := len(plaintext)
	hash := md5.Sum(plaintext)
	rawMD5Hex := hex.EncodeToString(hash[:])

	// Step 2: Generate AES key
	aesKey, err := GenerateAESKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate AES key: %w", err)
	}

	// Step 3: Calculate ciphertext size
	fileSize := AES128ECBPaddedSize(rawSize)

	// Step 4: Generate file key
	fileKey, err := GenerateFileKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate file key: %w", err)
	}

	// Step 5: Get upload URL
	uploadURLResp, err := u.client.GetUploadURL(ctx, &api.GetUploadUrlReq{
		FileKey:     fileKey,
		MediaType:   int(mediaType),
		ToUserID:    toUserID,
		RawSize:     rawSize,
		RawFileMD5:  rawMD5Hex,
		FileSize:    fileSize,
		NoNeedThumb: true,
		AESKey:      hex.EncodeToString(aesKey),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get upload URL: %w", err)
	}

	if uploadURLResp.UploadParam == "" {
		return nil, fmt.Errorf("getUploadUrl returned no upload_param")
	}

	// Step 6: Encrypt and upload
	ciphertext, err := EncryptAES128ECB(plaintext, aesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt: %w", err)
	}

	downloadParam, err := u.uploadToCDN(ctx, uploadURLResp.UploadParam, fileKey, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("CDN upload failed: %w", err)
	}

	return &UploadedFileInfo{
		FileKey:                     fileKey,
		DownloadEncryptedQueryParam: downloadParam,
		AESKey:                      hex.EncodeToString(aesKey),
		FileSize:                    rawSize,
		FileSizeCiphertext:          fileSize,
	}, nil
}

// UploadImage uploads an image to CDN.
func (u *Uploader) UploadImage(ctx context.Context, imageData []byte, toUserID string) (*UploadedFileInfo, error) {
	return u.UploadFile(ctx, imageData, toUserID, api.UploadMediaTypeImage)
}

// UploadVideo uploads a video to CDN.
func (u *Uploader) UploadVideo(ctx context.Context, videoData []byte, toUserID string) (*UploadedFileInfo, error) {
	return u.UploadFile(ctx, videoData, toUserID, api.UploadMediaTypeVideo)
}

// UploadFileAttachment uploads a file attachment to CDN.
func (u *Uploader) UploadFileAttachment(ctx context.Context, fileData []byte, toUserID string) (*UploadedFileInfo, error) {
	return u.UploadFile(ctx, fileData, toUserID, api.UploadMediaTypeFile)
}

// uploadToCDN uploads encrypted data to CDN with retries.
func (u *Uploader) uploadToCDN(ctx context.Context, uploadParam, fileKey string, ciphertext []byte) (string, error) {
	cdnURL := u.buildUploadURL(uploadParam, fileKey)

	var lastErr error
	for attempt := 1; attempt <= UploadMaxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", cdnURL, bytes.NewReader(ciphertext))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/octet-stream")

		resp, err := u.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < UploadMaxRetries {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			return "", lastErr
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			errMsg := resp.Header.Get("x-error-message")
			if errMsg == "" {
				body, _ := io.ReadAll(resp.Body)
				errMsg = string(body)
			}
			return "", fmt.Errorf("CDN client error %d: %s", resp.StatusCode, errMsg)
		}

		if resp.StatusCode != 200 {
			errMsg := resp.Header.Get("x-error-message")
			if errMsg == "" {
				errMsg = fmt.Sprintf("status %d", resp.StatusCode)
			}
			lastErr = fmt.Errorf("CDN server error: %s", errMsg)
			if attempt < UploadMaxRetries {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			return "", lastErr
		}

		downloadParam := resp.Header.Get("x-encrypted-param")
		if downloadParam == "" {
			return "", fmt.Errorf("CDN response missing x-encrypted-param header")
		}

		return downloadParam, nil
	}

	return "", lastErr
}

// buildUploadURL builds the CDN upload URL.
func (u *Uploader) buildUploadURL(uploadParam, fileKey string) string {
	return fmt.Sprintf("%s/upload?encrypted_query_param=%s&filekey=%s",
		u.cdnBaseURL,
		url.QueryEscape(uploadParam),
		url.QueryEscape(fileKey))
}
