// Package messaging provides message sending functionality.
// This is a complete Go reimplementation of the TypeScript send.ts module.
package messaging

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/openclaw/weixin-sdk-go/pkg/api"
	"github.com/openclaw/weixin-sdk-go/pkg/cdn"
)

// GenerateClientID generates a unique client ID for messages.
func GenerateClientID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("openclaw-weixin:%d-%s", time.Now().UnixMilli(), hex.EncodeToString(b))
}

// MarkdownToPlainText converts markdown-formatted text to plain text.
// Preserves newlines; strips markdown syntax.
func MarkdownToPlainText(text string) string {
	result := text

	// Code blocks: strip fences, keep code content
	codeBlockRegex := regexp.MustCompile("```[^\\n]*\\n?([\\s\\S]*?)```")
	result = codeBlockRegex.ReplaceAllStringFunc(result, func(match string) string {
		submatches := codeBlockRegex.FindStringSubmatch(match)
		if len(submatches) > 1 {
			return strings.TrimSpace(submatches[1])
		}
		return ""
	})

	// Images: remove entirely
	result = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`).ReplaceAllString(result, "")

	// Links: keep display text only
	result = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`).ReplaceAllString(result, "$1")

	// Tables: remove separator rows
	result = regexp.MustCompile(`^\|[\s:|-]+\|$`).ReplaceAllString(result, "")

	// Table cells: strip pipes and join with spaces
	result = regexp.MustCompile(`^\|(.+)\|$`).ReplaceAllStringFunc(result, func(match string) string {
		inner := regexp.MustCompile(`^\|(.+)\|$`).FindStringSubmatch(match)
		if len(inner) > 1 {
			cells := strings.Split(inner[1], "|")
			parts := make([]string, 0, len(cells))
			for _, cell := range cells {
				trimmed := strings.TrimSpace(cell)
				if trimmed != "" {
					parts = append(parts, trimmed)
				}
			}
			return strings.Join(parts, "  ")
		}
		return match
	})

	// Strip remaining markdown syntax
	result = stripMarkdown(result)

	return result
}

// stripMarkdown strips remaining markdown syntax.
func stripMarkdown(text string) string {
	// Bold
	text = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`__([^_]+)__`).ReplaceAllString(text, "$1")

	// Italic
	text = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`_([^_]+)_`).ReplaceAllString(text, "$1")

	// Strikethrough
	text = regexp.MustCompile(`~~([^~]+)~~`).ReplaceAllString(text, "$1")

	// Headers
	text = regexp.MustCompile(`(?m)^#+\s+`).ReplaceAllString(text, "")

	// Inline code
	text = regexp.MustCompile("`([^`]+)`").ReplaceAllString(text, "$1")

	// Lists
	text = regexp.MustCompile(`(?m)^[\s]*[-*+]\s+`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?m)^[\s]*\d+\.\s+`).ReplaceAllString(text, "")

	// Blockquotes
	text = regexp.MustCompile(`(?m)^>\s*`).ReplaceAllString(text, "")

	// Horizontal rules
	text = regexp.MustCompile(`(?m)^[-*_]{3,}\s*$`).ReplaceAllString(text, "")

	return text
}

// Sender handles sending messages to WeChat users.
type Sender struct {
	client     *api.Client
	uploader   *cdn.Uploader
	cdnBaseURL string
}

// NewSender creates a new message sender.
func NewSender(client *api.Client, cdnBaseURL string) *Sender {
	return &Sender{
		client:     client,
		uploader:   cdn.NewUploader(client, cdnBaseURL),
		cdnBaseURL: cdnBaseURL,
	}
}

// SendText sends a text message.
func (s *Sender) SendText(ctx context.Context, to, text, contextToken string) (string, error) {
	clientID := GenerateClientID()

	msg := &api.WeixinMessage{
		ToUserID:     to,
		ClientID:     clientID,
		MessageType:  int(api.MessageTypeBot),
		MessageState: int(api.MessageStateFinish),
		ContextToken: contextToken,
		ItemList: []*api.MessageItem{
			{
				Type:     int(api.MessageItemTypeText),
				TextItem: &api.TextItem{Text: text},
			},
		},
	}

	if err := s.client.SendMessage(ctx, msg); err != nil {
		return "", fmt.Errorf("failed to send text message: %w", err)
	}

	return clientID, nil
}

// SendImage sends an image message using previously uploaded file info.
func (s *Sender) SendImage(ctx context.Context, to, caption string, imageData []byte, contextToken string) (string, error) {
	// Upload image to CDN
	uploaded, err := s.uploader.UploadFile(ctx, imageData, to, api.UploadMediaTypeImage)
	if err != nil {
		return "", fmt.Errorf("failed to upload image: %w", err)
	}

	// Convert hex AES key to base64
	aesKey, err := hex.DecodeString(uploaded.AESKey)
	if err != nil {
		return "", fmt.Errorf("failed to decode AES key: %w", err)
	}
	aesKeyBase64 := base64.StdEncoding.EncodeToString(aesKey)

	clientID := GenerateClientID()

	// Build image message item
	imageItem := &api.MessageItem{
		Type: int(api.MessageItemTypeImage),
		ImageItem: &api.ImageItem{
			Media: &api.CDNMedia{
				EncryptQueryParam: uploaded.DownloadEncryptedQueryParam,
				AESKey:            aesKeyBase64,
				EncryptType:       1,
			},
			MidSize: uploaded.FileSizeCiphertext,
		},
	}

	// Send caption first if present
	if caption != "" {
		if _, err := s.SendText(ctx, to, caption, contextToken); err != nil {
			return "", err
		}
	}

	// Send image
	msg := &api.WeixinMessage{
		ToUserID:     to,
		ClientID:     clientID,
		MessageType:  int(api.MessageTypeBot),
		MessageState: int(api.MessageStateFinish),
		ContextToken: contextToken,
		ItemList:     []*api.MessageItem{imageItem},
	}

	if err := s.client.SendMessage(ctx, msg); err != nil {
		return "", fmt.Errorf("failed to send image message: %w", err)
	}

	return clientID, nil
}

// SendImageWithUploaded sends an image using pre-uploaded file info.
func (s *Sender) SendImageWithUploaded(ctx context.Context, to, caption string, uploaded *cdn.UploadedFileInfo, contextToken string) (string, error) {
	aesKey, _ := hex.DecodeString(uploaded.AESKey)
	aesKeyBase64 := base64.StdEncoding.EncodeToString(aesKey)

	clientID := GenerateClientID()

	imageItem := &api.MessageItem{
		Type: int(api.MessageItemTypeImage),
		ImageItem: &api.ImageItem{
			Media: &api.CDNMedia{
				EncryptQueryParam: uploaded.DownloadEncryptedQueryParam,
				AESKey:            aesKeyBase64,
				EncryptType:       1,
			},
			MidSize: uploaded.FileSizeCiphertext,
		},
	}

	if caption != "" {
		if _, err := s.SendText(ctx, to, caption, contextToken); err != nil {
			return "", err
		}
	}

	msg := &api.WeixinMessage{
		ToUserID:     to,
		ClientID:     clientID,
		MessageType:  int(api.MessageTypeBot),
		MessageState: int(api.MessageStateFinish),
		ContextToken: contextToken,
		ItemList:     []*api.MessageItem{imageItem},
	}

	if err := s.client.SendMessage(ctx, msg); err != nil {
		return "", fmt.Errorf("failed to send image message: %w", err)
	}

	return clientID, nil
}

// SendVideo sends a video message.
func (s *Sender) SendVideo(ctx context.Context, to, caption string, videoData []byte, contextToken string) (string, error) {
	uploaded, err := s.uploader.UploadFile(ctx, videoData, to, api.UploadMediaTypeVideo)
	if err != nil {
		return "", fmt.Errorf("failed to upload video: %w", err)
	}

	aesKey, _ := hex.DecodeString(uploaded.AESKey)
	aesKeyBase64 := base64.StdEncoding.EncodeToString(aesKey)

	clientID := GenerateClientID()

	videoItem := &api.MessageItem{
		Type: int(api.MessageItemTypeVideo),
		VideoItem: &api.VideoItem{
			Media: &api.CDNMedia{
				EncryptQueryParam: uploaded.DownloadEncryptedQueryParam,
				AESKey:            aesKeyBase64,
				EncryptType:       1,
			},
			VideoSize: uploaded.FileSizeCiphertext,
		},
	}

	if caption != "" {
		if _, err := s.SendText(ctx, to, caption, contextToken); err != nil {
			return "", err
		}
	}

	msg := &api.WeixinMessage{
		ToUserID:     to,
		ClientID:     clientID,
		MessageType:  int(api.MessageTypeBot),
		MessageState: int(api.MessageStateFinish),
		ContextToken: contextToken,
		ItemList:     []*api.MessageItem{videoItem},
	}

	if err := s.client.SendMessage(ctx, msg); err != nil {
		return "", fmt.Errorf("failed to send video message: %w", err)
	}

	return clientID, nil
}

// SendFile sends a file attachment.
func (s *Sender) SendFile(ctx context.Context, to, caption, fileName string, fileData []byte, contextToken string) (string, error) {
	uploaded, err := s.uploader.UploadFile(ctx, fileData, to, api.UploadMediaTypeFile)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	aesKey, _ := hex.DecodeString(uploaded.AESKey)
	aesKeyBase64 := base64.StdEncoding.EncodeToString(aesKey)

	clientID := GenerateClientID()

	fileItem := &api.MessageItem{
		Type: int(api.MessageItemTypeFile),
		FileItem: &api.FileItem{
			Media: &api.CDNMedia{
				EncryptQueryParam: uploaded.DownloadEncryptedQueryParam,
				AESKey:            aesKeyBase64,
				EncryptType:       1,
			},
			FileName: fileName,
			Len:      fmt.Sprintf("%d", uploaded.FileSize),
		},
	}

	if caption != "" {
		if _, err := s.SendText(ctx, to, caption, contextToken); err != nil {
			return "", err
		}
	}

	msg := &api.WeixinMessage{
		ToUserID:     to,
		ClientID:     clientID,
		MessageType:  int(api.MessageTypeBot),
		MessageState: int(api.MessageStateFinish),
		ContextToken: contextToken,
		ItemList:     []*api.MessageItem{fileItem},
	}

	if err := s.client.SendMessage(ctx, msg); err != nil {
		return "", fmt.Errorf("failed to send file message: %w", err)
	}

	return clientID, nil
}

// SendVoice sends a voice message.
func (s *Sender) SendVoice(ctx context.Context, to string, voiceData []byte, duration int, contextToken string) (string, error) {
	uploaded, err := s.uploader.UploadFile(ctx, voiceData, to, api.UploadMediaTypeVoice)
	if err != nil {
		return "", fmt.Errorf("failed to upload voice: %w", err)
	}

	aesKey, _ := hex.DecodeString(uploaded.AESKey)
	aesKeyBase64 := base64.StdEncoding.EncodeToString(aesKey)

	clientID := GenerateClientID()

	voiceItem := &api.MessageItem{
		Type: int(api.MessageItemTypeVoice),
		VoiceItem: &api.VoiceItem{
			Media: &api.CDNMedia{
				EncryptQueryParam: uploaded.DownloadEncryptedQueryParam,
				AESKey:            aesKeyBase64,
				EncryptType:       1,
			},
			Playtime: duration,
		},
	}

	msg := &api.WeixinMessage{
		ToUserID:     to,
		ClientID:     clientID,
		MessageType:  int(api.MessageTypeBot),
		MessageState: int(api.MessageStateFinish),
		ContextToken: contextToken,
		ItemList:     []*api.MessageItem{voiceItem},
	}

	if err := s.client.SendMessage(ctx, msg); err != nil {
		return "", fmt.Errorf("failed to send voice message: %w", err)
	}

	return clientID, nil
}
