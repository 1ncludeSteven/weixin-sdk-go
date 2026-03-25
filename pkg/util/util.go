// Package util provides utility functions.
package util

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// GenerateID generates a prefixed unique ID.
func GenerateID(prefix string) string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%s:%d-%s", prefix, time.Now().Unix(), hex.EncodeToString(b))
}

// TempFileName generates a temporary file name.
func TempFileName(prefix, ext string) string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%s-%d-%s%s", prefix, time.Now().Unix(), hex.EncodeToString(b), ext)
}

// Truncate truncates a string with ellipsis.
func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// RedactToken redacts a token for logging.
func RedactToken(token string, prefixLen int) string {
	if token == "" {
		return "(none)"
	}
	if prefixLen <= 0 {
		prefixLen = 6
	}
	if len(token) <= prefixLen {
		return "****"
	}
	return token[:prefixLen] + "..."
}
