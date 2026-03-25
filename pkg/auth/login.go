// Package auth provides authentication and account management.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/openclaw/weixin-sdk-go/pkg/api"
)

const (
	DefaultILINKBotType   = "3"
	LoginTimeout          = 480 * time.Second
	ActiveLoginTTL        = 5 * time.Minute
	MaxQRRefreshCount     = 3
	QRLongPollTimeout     = 35 * time.Second
)

// QRStartResult represents the result of starting a QR login.
type QRStartResult struct {
	QRCodeURL  string `json:"qrcode_url,omitempty"`
	Message    string `json:"message"`
	SessionKey string `json:"session_key"`
}

// QRWaitResult represents the result of waiting for QR login.
type QRWaitResult struct {
	Connected bool   `json:"connected"`
	BotToken  string `json:"bot_token,omitempty"`
	AccountID string `json:"account_id,omitempty"`
	BaseURL   string `json:"base_url,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	Message   string `json:"message"`
}

// ActiveLogin tracks an in-progress QR login session.
type ActiveLogin struct {
	SessionKey string
	ID         string
	QRCode     string
	QRCodeURL  string
	StartedAt  time.Time
	BotToken   string
	Status     string // "wait", "scaned", "confirmed", "expired"
	Error      string
}

// LoginManager manages QR login sessions.
type LoginManager struct {
	mu          sync.RWMutex
	activeLogins map[string]*ActiveLogin
	client      *api.Client
}

// NewLoginManager creates a new login manager.
func NewLoginManager(client *api.Client) *LoginManager {
	return &LoginManager{
		activeLogins: make(map[string]*ActiveLogin),
		client:       client,
	}
}

// isLoginFresh checks if a login session is still fresh.
func isLoginFresh(login *ActiveLogin) bool {
	return time.Since(login.StartedAt) < ActiveLoginTTL
}

// purgeExpiredLogins removes expired login sessions.
func (lm *LoginManager) purgeExpiredLogins() {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	for id, login := range lm.activeLogins {
		if !isLoginFresh(login) {
			delete(lm.activeLogins, id)
		}
	}
}

// generateSessionKey generates a unique session key.
func generateSessionKey() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// StartQRLogin initiates a QR code login session.
func (lm *LoginManager) StartQRLogin(ctx context.Context, accountID string, force bool) (*QRStartResult, error) {
	lm.purgeExpiredLogins()

	sessionKey := accountID
	if sessionKey == "" {
		sessionKey = generateSessionKey()
	}

	lm.mu.RLock()
	existing, exists := lm.activeLogins[sessionKey]
	lm.mu.RUnlock()

	if !force && exists && isLoginFresh(existing) && existing.QRCodeURL != "" {
		return &QRStartResult{
			QRCodeURL:  existing.QRCodeURL,
			Message:    "二维码已就绪，请使用微信扫描。",
			SessionKey: sessionKey,
		}, nil
	}

	botType := DefaultILINKBotType
	qrResp, err := lm.client.GetQRCode(ctx, botType)
	if err != nil {
		return &QRStartResult{
			Message:    fmt.Sprintf("获取二维码失败: %v", err),
			SessionKey: sessionKey,
		}, err
	}

	login := &ActiveLogin{
		SessionKey: sessionKey,
		ID:         uuid.New().String(),
		QRCode:     qrResp.QRCode,
		QRCodeURL:  qrResp.QRCodeImgContent,
		StartedAt:  time.Now(),
		Status:     "wait",
	}

	lm.mu.Lock()
	lm.activeLogins[sessionKey] = login
	lm.mu.Unlock()

	return &QRStartResult{
		QRCodeURL:  qrResp.QRCodeImgContent,
		Message:    "使用微信扫描以下二维码，以完成连接。",
		SessionKey: sessionKey,
	}, nil
}

// WaitForLogin waits for QR code scan and confirmation.
func (lm *LoginManager) WaitForLogin(ctx context.Context, sessionKey string, timeout time.Duration) (*QRWaitResult, error) {
	lm.mu.RLock()
	activeLogin, exists := lm.activeLogins[sessionKey]
	lm.mu.RUnlock()

	if !exists {
		return &QRWaitResult{
			Connected: false,
			Message:   "当前没有进行中的登录，请先发起登录。",
		}, nil
	}

	if !isLoginFresh(activeLogin) {
		lm.mu.Lock()
		delete(lm.activeLogins, sessionKey)
		lm.mu.Unlock()
		return &QRWaitResult{
			Connected: false,
			Message:   "二维码已过期，请重新生成。",
		}, nil
	}

	if timeout == 0 {
		timeout = LoginTimeout
	}

	deadline := time.Now().Add(timeout)
	scannedPrinted := false
	qrRefreshCount := 1

	for time.Now().Before(deadline) {
		statusResp, err := lm.client.PollQRStatus(ctx, activeLogin.QRCode)
		if err != nil {
			lm.mu.Lock()
			delete(lm.activeLogins, sessionKey)
			lm.mu.Unlock()
			return &QRWaitResult{
				Connected: false,
				Message:   fmt.Sprintf("登录失败: %v", err),
			}, err
		}

		lm.mu.Lock()
		activeLogin.Status = statusResp.Status
		lm.mu.Unlock()

		switch statusResp.Status {
		case "wait":
			// Continue polling
			time.Sleep(1 * time.Second)

		case "scaned":
			if !scannedPrinted {
				fmt.Println("\n👀 已扫码，在微信继续操作...")
				scannedPrinted = true
			}
			time.Sleep(1 * time.Second)

		case "expired":
			qrRefreshCount++
			if qrRefreshCount > MaxQRRefreshCount {
				lm.mu.Lock()
				delete(lm.activeLogins, sessionKey)
				lm.mu.Unlock()
				return &QRWaitResult{
					Connected: false,
					Message:   "登录超时：二维码多次过期，请重新开始登录流程。",
				}, nil
			}

			fmt.Printf("\n⏳ 二维码已过期，正在刷新...(%d/%d)\n", qrRefreshCount, MaxQRRefreshCount)

			// Get new QR code
			qrResp, err := lm.client.GetQRCode(ctx, DefaultILINKBotType)
			if err != nil {
				lm.mu.Lock()
				delete(lm.activeLogins, sessionKey)
				lm.mu.Unlock()
				return &QRWaitResult{
					Connected: false,
					Message:   fmt.Sprintf("刷新二维码失败: %v", err),
				}, err
			}

			lm.mu.Lock()
			activeLogin.QRCode = qrResp.QRCode
			activeLogin.QRCodeURL = qrResp.QRCodeImgContent
			activeLogin.StartedAt = time.Now()
			lm.mu.Unlock()

			scannedPrinted = false
			fmt.Println("🔄 新二维码已生成，请重新扫描")
			fmt.Println(qrResp.QRCodeImgContent)

		case "confirmed":
			if statusResp.ILinkBotID == "" {
				lm.mu.Lock()
				delete(lm.activeLogins, sessionKey)
				lm.mu.Unlock()
				return &QRWaitResult{
					Connected: false,
					Message:   "登录失败：服务器未返回 ilink_bot_id。",
				}, nil
			}

			lm.mu.Lock()
			activeLogin.BotToken = statusResp.BotToken
			delete(lm.activeLogins, sessionKey)
			lm.mu.Unlock()

			return &QRWaitResult{
				Connected: true,
				BotToken:  statusResp.BotToken,
				AccountID: statusResp.ILinkBotID,
				BaseURL:   statusResp.BaseURL,
				UserID:    statusResp.ILinkUserID,
				Message:   "✅ 与微信连接成功！",
			}, nil
		}
	}

	lm.mu.Lock()
	delete(lm.activeLogins, sessionKey)
	lm.mu.Unlock()

	return &QRWaitResult{
		Connected: false,
		Message:   "登录超时，请重试。",
	}, nil
}
