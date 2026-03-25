// Package media provides media handling utilities.
package media

import (
	"path/filepath"
	"strings"
)

// ExtensionToMIME maps file extensions to MIME types.
var ExtensionToMIME = map[string]string{
	".pdf":  "application/pdf",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".ppt":  "application/vnd.ms-powerpoint",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".txt":  "text/plain",
	".csv":  "text/csv",
	".zip":  "application/zip",
	".tar":  "application/x-tar",
	".gz":   "application/gzip",
	".mp3":  "audio/mpeg",
	".ogg":  "audio/ogg",
	".wav":  "audio/wav",
	".mp4":  "video/mp4",
	".mov":  "video/quicktime",
	".webm": "video/webm",
	".mkv":  "video/x-matroska",
	".avi":  "video/x-msvideo",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
}

// MIMEToExtension maps MIME types to file extensions.
var MIMEToExtension = map[string]string{
	"image/jpeg":      ".jpg",
	"image/jpg":       ".jpg",
	"image/png":       ".png",
	"image/gif":       ".gif",
	"image/webp":      ".webp",
	"image/bmp":       ".bmp",
	"video/mp4":       ".mp4",
	"video/quicktime": ".mov",
	"video/webm":      ".webm",
	"video/x-matroska": ".mkv",
	"video/x-msvideo": ".avi",
	"audio/mpeg":      ".mp3",
	"audio/ogg":       ".ogg",
	"audio/wav":       ".wav",
	"application/pdf": ".pdf",
	"application/zip": ".zip",
	"application/x-tar": ".tar",
	"application/gzip": ".gz",
	"text/plain":      ".txt",
	"text/csv":        ".csv",
}

// GetMIMEFromFilename returns the MIME type for a filename.
func GetMIMEFromFilename(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if mime, ok := ExtensionToMIME[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// GetExtensionFromMIME returns the file extension for a MIME type.
func GetExtensionFromMIME(mimeType string) string {
	ct := strings.ToLower(strings.Split(mimeType, ";")[0])
	ct = strings.TrimSpace(ct)
	if ext, ok := MIMEToExtension[ct]; ok {
		return ext
	}
	return ".bin"
}

// IsImage checks if a MIME type is an image.
func IsImage(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}

// IsVideo checks if a MIME type is a video.
func IsVideo(mimeType string) bool {
	return strings.HasPrefix(mimeType, "video/")
}

// IsAudio checks if a MIME type is audio.
func IsAudio(mimeType string) bool {
	return strings.HasPrefix(mimeType, "audio/")
}
