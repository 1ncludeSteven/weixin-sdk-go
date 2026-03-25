// Package messaging provides inbound message processing.
// This is a complete Go reimplementation of the TypeScript inbound.ts module.
package messaging

import (
	"strings"

	"github.com/1ncludeSteven/weixin-sdk-go/pkg/api"
)

// MsgContext represents inbound message context for the core pipeline.
type MsgContext struct {
	Body               string
	From               string
	To                 string
	AccountID          string
	OriginatingChannel string
	OriginatingTo      string
	MessageSid         string
	Timestamp          int64
	Provider           string
	ChatType           string
	SessionKey         string
	ContextToken       string
	MediaPath          string
	MediaType          string
	CommandBody        string
	CommandAuthorized  bool
}

// ExtractTextBody extracts text body from message items.
func ExtractTextBody(itemList []*api.MessageItem) string {
	if len(itemList) == 0 {
		return ""
	}

	for _, item := range itemList {
		if item.Type == int(api.MessageItemTypeText) && item.TextItem != nil && item.TextItem.Text != "" {
			return item.TextItem.Text
		}
		if item.Type == int(api.MessageItemTypeVoice) && item.VoiceItem != nil && item.VoiceItem.Text != "" {
			return item.VoiceItem.Text
		}
	}

	return ""
}

// IsMediaItem checks if a message item is a media type.
func IsMediaItem(item *api.MessageItem) bool {
	return item.Type == int(api.MessageItemTypeImage) ||
		item.Type == int(api.MessageItemTypeVideo) ||
		item.Type == int(api.MessageItemTypeFile) ||
		item.Type == int(api.MessageItemTypeVoice)
}

// HasMedia checks if a message has downloadable media.
func HasMedia(item *api.MessageItem) bool {
	switch item.Type {
	case int(api.MessageItemTypeImage):
		return item.ImageItem != nil && item.ImageItem.Media != nil && item.ImageItem.Media.EncryptQueryParam != ""
	case int(api.MessageItemTypeVideo):
		return item.VideoItem != nil && item.VideoItem.Media != nil && item.VideoItem.Media.EncryptQueryParam != ""
	case int(api.MessageItemTypeFile):
		return item.FileItem != nil && item.FileItem.Media != nil && item.FileItem.Media.EncryptQueryParam != ""
	case int(api.MessageItemTypeVoice):
		return item.VoiceItem != nil && item.VoiceItem.Media != nil && item.VoiceItem.Media.EncryptQueryParam != ""
	}
	return false
}

// GetMediaInfo extracts media info from a message item.
type MediaInfo struct {
	Type              string // "image", "video", "file", "voice"
	EncryptQueryParam string
	AESKey            string
	FileName          string
	FileSize          int64
	Duration          int // voice only
}

// GetMediaInfo extracts media info from a message item.
func GetMediaInfo(item *api.MessageItem) *MediaInfo {
	if item == nil {
		return nil
	}

	switch item.Type {
	case int(api.MessageItemTypeImage):
		if item.ImageItem != nil && item.ImageItem.Media != nil {
			aesKey := item.ImageItem.AESKey
			if aesKey == "" {
				aesKey = item.ImageItem.Media.AESKey
			}
			return &MediaInfo{
				Type:              "image",
				EncryptQueryParam: item.ImageItem.Media.EncryptQueryParam,
				AESKey:            aesKey,
			}
		}
	case int(api.MessageItemTypeVideo):
		if item.VideoItem != nil && item.VideoItem.Media != nil {
			return &MediaInfo{
				Type:              "video",
				EncryptQueryParam: item.VideoItem.Media.EncryptQueryParam,
				AESKey:            item.VideoItem.Media.AESKey,
			}
		}
	case int(api.MessageItemTypeFile):
		if item.FileItem != nil && item.FileItem.Media != nil {
			return &MediaInfo{
				Type:              "file",
				EncryptQueryParam: item.FileItem.Media.EncryptQueryParam,
				AESKey:            item.FileItem.Media.AESKey,
				FileName:          item.FileItem.FileName,
			}
		}
	case int(api.MessageItemTypeVoice):
		if item.VoiceItem != nil && item.VoiceItem.Media != nil {
			return &MediaInfo{
				Type:              "voice",
				EncryptQueryParam: item.VoiceItem.Media.EncryptQueryParam,
				AESKey:            item.VoiceItem.Media.AESKey,
				Duration:          item.VoiceItem.Playtime,
			}
		}
	}

	return nil
}

// MessageToContext converts a WeixinMessage to MsgContext.
func MessageToContext(msg *api.WeixinMessage, accountID string, mediaPath, mediaType string) *MsgContext {
	fromUserID := msg.FromUserID
	if fromUserID == "" {
		fromUserID = ""
	}

	return &MsgContext{
		Body:               ExtractTextBody(msg.ItemList),
		From:               fromUserID,
		To:                 fromUserID,
		AccountID:          accountID,
		OriginatingChannel: "openclaw-weixin",
		OriginatingTo:      fromUserID,
		MessageSid:         GenerateClientID(),
		Timestamp:          msg.CreateTimeMs,
		Provider:           "openclaw-weixin",
		ChatType:           "direct",
		ContextToken:       msg.ContextToken,
		MediaPath:          mediaPath,
		MediaType:          mediaType,
	}
}

// BodyFromItemList extracts body text from item list, handling quoted messages.
func BodyFromItemList(itemList []*api.MessageItem) string {
	if len(itemList) == 0 {
		return ""
	}

	for _, item := range itemList {
		if item.Type == int(api.MessageItemTypeText) && item.TextItem != nil {
			text := item.TextItem.Text
			ref := item.RefMsg
			if ref == nil {
				return text
			}

			// Quoted media is passed as MediaPath; only include current text
			if ref.MessageItem != nil && IsMediaItem(ref.MessageItem) {
				return text
			}

			// Build quoted context
			parts := []string{}
			if ref.Title != "" {
				parts = append(parts, ref.Title)
			}
			if ref.MessageItem != nil {
				refBody := BodyFromItemList([]*api.MessageItem{ref.MessageItem})
				if refBody != "" {
					parts = append(parts, refBody)
				}
			}

			if len(parts) == 0 {
				return text
			}

			return "[引用: " + strings.Join(parts, " | ") + "]\n" + text
		}

		// Voice transcription
		if item.Type == int(api.MessageItemTypeVoice) && item.VoiceItem != nil && item.VoiceItem.Text != "" {
			return item.VoiceItem.Text
		}
	}

	return ""
}

// FirstMediaItem returns the first downloadable media item in the list.
// Priority: IMAGE > VIDEO > FILE > VOICE
func FirstMediaItem(itemList []*api.MessageItem) *api.MessageItem {
	// Check images first
	for _, item := range itemList {
		if item.Type == int(api.MessageItemTypeImage) && HasMedia(item) {
			return item
		}
	}

	// Then videos
	for _, item := range itemList {
		if item.Type == int(api.MessageItemTypeVideo) && HasMedia(item) {
			return item
		}
	}

	// Then files
	for _, item := range itemList {
		if item.Type == int(api.MessageItemTypeFile) && HasMedia(item) {
			return item
		}
	}

	// Then voice (only if no text transcription)
	for _, item := range itemList {
		if item.Type == int(api.MessageItemTypeVoice) && HasMedia(item) && (item.VoiceItem == nil || item.VoiceItem.Text == "") {
			return item
		}
	}

	return nil
}

// GetRefMediaItem extracts a referenced media item from a quoted message.
func GetRefMediaItem(item *api.MessageItem) *api.MessageItem {
	if item == nil || item.Type != int(api.MessageItemTypeText) {
		return nil
	}

	ref := item.RefMsg
	if ref == nil || ref.MessageItem == nil {
		return nil
	}

	if IsMediaItem(ref.MessageItem) {
		return ref.MessageItem
	}

	return nil
}
