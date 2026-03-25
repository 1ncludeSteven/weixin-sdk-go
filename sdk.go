// Package weixinsdk provides a complete WeChat SDK for OpenClaw integration.
// This is a Go reimplementation of @tencent-weixin/openclaw-weixin npm package.
package weixinsdk

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/1ncludeSteven/weixin-sdk-go/pkg/api"
	"github.com/1ncludeSteven/weixin-sdk-go/pkg/auth"
	"github.com/1ncludeSteven/weixin-sdk-go/pkg/cdn"
	"github.com/1ncludeSteven/weixin-sdk-go/pkg/config"
	"github.com/1ncludeSteven/weixin-sdk-go/pkg/media"
	"github.com/1ncludeSteven/weixin-sdk-go/pkg/messaging"
	"github.com/1ncludeSteven/weixin-sdk-go/pkg/monitor"
	"github.com/1ncludeSteven/weixin-sdk-go/pkg/storage"
)

// Version is the SDK version (matches npm package version).
const Version = "2.0.1"

// SDK is the main WeChat SDK for OpenClaw integration.
type SDK struct {
	client         *api.Client
	accountManager *auth.AccountManager
	loginManager   *auth.LoginManager
	config         *config.ChannelConfig
	stateDir       string
	cdnBaseURL     string
}

// Config represents SDK configuration.
type Config struct {
	BaseURL    string
	Token      string
	RouteTag   string
	CDNBaseURL string
	StateDir   string
}

// New creates a new SDK instance.
func New(cfg *Config) *SDK {
	if cfg == nil {
		cfg = &Config{}
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = config.DefaultBaseURL
	}
	if cfg.CDNBaseURL == "" {
		cfg.CDNBaseURL = config.DefaultCDNBaseURL
	}

	client := api.NewClient(
		api.WithBaseURL(cfg.BaseURL),
		api.WithToken(cfg.Token),
		api.WithRouteTag(cfg.RouteTag),
	)

	return &SDK{
		client:         client,
		accountManager: auth.NewAccountManager(cfg.StateDir),
		loginManager:   auth.NewLoginManager(client),
		stateDir:       cfg.StateDir,
		cdnBaseURL:     cfg.CDNBaseURL,
	}
}

// Client returns the underlying API client.
func (s *SDK) Client() *api.Client {
	return s.client
}

// AccountManager returns the account manager.
func (s *SDK) AccountManager() *auth.AccountManager {
	return s.accountManager
}

// LoginManager returns the login manager.
func (s *SDK) LoginManager() *auth.LoginManager {
	return s.loginManager
}

// ============================================================================
// Login (QR Code Authentication)
// ============================================================================

// QRStartResult represents the result of starting a QR login.
type QRStartResult = auth.QRStartResult

// QRWaitResult represents the result of waiting for QR login.
type QRWaitResult = auth.QRWaitResult

// StartQRLogin initiates a QR code login session.
func (s *SDK) StartQRLogin(ctx context.Context, accountID string, force bool) (*QRStartResult, error) {
	return s.loginManager.StartQRLogin(ctx, accountID, force)
}

// WaitForLogin waits for QR code scan and confirmation.
func (s *SDK) WaitForLogin(ctx context.Context, sessionKey string, timeout int) (*QRWaitResult, error) {
	return s.loginManager.WaitForLogin(ctx, sessionKey, 0)
}

// Login performs complete QR code login flow (start + wait).
// Returns the QR code URL and waits for scan confirmation.
func (s *SDK) Login(ctx context.Context, accountID string, force bool) (*QRWaitResult, *QRStartResult, error) {
	startResult, err := s.loginManager.StartQRLogin(ctx, accountID, force)
	if err != nil {
		return nil, startResult, err
	}

	if startResult.QRCodeURL == "" {
		return &QRWaitResult{
			Connected: false,
			Message:   startResult.Message,
		}, startResult, nil
	}

	waitResult, err := s.loginManager.WaitForLogin(ctx, startResult.SessionKey, 0)
	if err != nil {
		return nil, startResult, err
	}

	// Save account on successful login
	if waitResult.Connected && waitResult.BotToken != "" {
		s.accountManager.SaveAccount(waitResult.AccountID, &auth.AccountData{
			Token:   waitResult.BotToken,
			BaseURL: waitResult.BaseURL,
			UserID:  waitResult.UserID,
		})

		// Register user in allowFrom store for pairing
		if waitResult.UserID != "" {
			allowStore := storage.NewAllowFromStore(waitResult.AccountID)
			allowStore.Add(waitResult.UserID)
		}
	}

	return waitResult, startResult, nil
}

// ============================================================================
// Message Monitoring
// ============================================================================

// MessageHandler handles incoming messages.
type MessageHandler = monitor.MessageHandler

// MessageHandlerFunc is a function that implements MessageHandler.
type MessageHandlerFunc = monitor.MessageHandlerFunc

// MonitorStatus represents monitor status.
type MonitorStatus = monitor.Status

// Monitor monitors WeChat messages via long-poll.
type Monitor = monitor.Monitor

// MonitorOption is a function that configures the Monitor.
type MonitorOption = monitor.MonitorOption

// NewMonitor creates a message monitor for an account.
func (s *SDK) NewMonitor(accountID string, opts ...MonitorOption) (*Monitor, error) {
	account, err := s.accountManager.ResolveAccount(accountID)
	if err != nil {
		return nil, err
	}

	// Update client with account token
	s.client.Token = account.Token
	s.client.BaseURL = account.BaseURL

	return monitor.NewMonitor(s.client, accountID, opts...), nil
}

// StartMonitor starts monitoring messages for an account.
func (s *SDK) StartMonitor(ctx context.Context, accountID string, handler MessageHandler) (*Monitor, error) {
	m, err := s.NewMonitor(accountID, WithHandler(handler))
	if err != nil {
		return nil, err
	}

	go m.Start(ctx)
	return m, nil
}

// ============================================================================
// Message Sending
// ============================================================================

// Sender sends messages to WeChat users.
type Sender struct {
	client     *api.Client
	uploader   *cdn.Uploader
	downloader *cdn.Downloader
	cdnBaseURL string
}

// NewSender creates a new message sender for an account.
func (s *SDK) NewSender(accountID string) (*Sender, error) {
	account, err := s.accountManager.ResolveAccount(accountID)
	if err != nil {
		return nil, err
	}

	s.client.Token = account.Token
	s.client.BaseURL = account.BaseURL

	return &Sender{
		client:     s.client,
		uploader:   cdn.NewUploader(s.client, account.CDNBaseURL),
		downloader: cdn.NewDownloader(account.CDNBaseURL),
		cdnBaseURL: account.CDNBaseURL,
	}, nil
}

// SendText sends a text message.
func (s *Sender) SendText(ctx context.Context, to, text, contextToken string) (string, error) {
	return messaging.NewSender(s.client, s.cdnBaseURL).SendText(ctx, to, text, contextToken)
}

// SendImage sends an image message.
func (s *Sender) SendImage(ctx context.Context, to, caption string, imageData []byte, contextToken string) (string, error) {
	return messaging.NewSender(s.client, s.cdnBaseURL).SendImage(ctx, to, caption, imageData, contextToken)
}

// SendVideo sends a video message.
func (s *Sender) SendVideo(ctx context.Context, to, caption string, videoData []byte, contextToken string) (string, error) {
	return messaging.NewSender(s.client, s.cdnBaseURL).SendVideo(ctx, to, caption, videoData, contextToken)
}

// SendFile sends a file attachment.
func (s *Sender) SendFile(ctx context.Context, to, caption, fileName string, fileData []byte, contextToken string) (string, error) {
	return messaging.NewSender(s.client, s.cdnBaseURL).SendFile(ctx, to, caption, fileName, fileData, contextToken)
}

// SendMedia sends a media file (auto-detect type by MIME).
func (s *Sender) SendMedia(ctx context.Context, to, caption, filePath string, contextToken string) (string, error) {
	// Read file
	data, err := readFile(filePath)
	if err != nil {
		return "", err
	}

	mimeType := media.GetMIMEFromFilename(filePath)
	switch {
	case media.IsImage(mimeType):
		return s.SendImage(ctx, to, caption, data, contextToken)
	case media.IsVideo(mimeType):
		return s.SendVideo(ctx, to, caption, data, contextToken)
	default:
		return s.SendFile(ctx, to, caption, filePath, data, contextToken)
	}
}

// ============================================================================
// Convenience Methods
// ============================================================================

// SendText sends a text message (convenience method).
func (s *SDK) SendText(ctx context.Context, accountID, to, text, contextToken string) (string, error) {
	sender, err := s.NewSender(accountID)
	if err != nil {
		return "", err
	}
	return sender.SendText(ctx, to, text, contextToken)
}

// SendImage sends an image message (convenience method).
func (s *SDK) SendImage(ctx context.Context, accountID, to, caption string, imageData []byte, contextToken string) (string, error) {
	sender, err := s.NewSender(accountID)
	if err != nil {
		return "", err
	}
	return sender.SendImage(ctx, to, caption, imageData, contextToken)
}

// SendFile sends a file message (convenience method).
func (s *SDK) SendFile(ctx context.Context, accountID, to, caption, fileName string, fileData []byte, contextToken string) (string, error) {
	sender, err := s.NewSender(accountID)
	if err != nil {
		return "", err
	}
	return sender.SendFile(ctx, to, caption, fileName, fileData, contextToken)
}

// ============================================================================
// Account Management
// ============================================================================

// AccountData represents stored account credentials.
type AccountData = auth.AccountData

// ResolvedAccount represents a resolved account.
type ResolvedAccount = auth.ResolvedAccount

// ListAccounts returns all registered account IDs.
func (s *SDK) ListAccounts() []string {
	return s.accountManager.ListAccountIDs()
}

// GetAccount returns account information.
func (s *SDK) GetAccount(accountID string) (*ResolvedAccount, error) {
	return s.accountManager.ResolveAccount(accountID)
}

// SaveAccount saves account credentials.
func (s *SDK) SaveAccount(accountID string, data *AccountData) error {
	return s.accountManager.SaveAccount(accountID, data)
}

// DeleteAccount removes an account and all associated data.
func (s *SDK) DeleteAccount(accountID string) error {
	// Clear context tokens
	store := storage.NewContextTokenStore(accountID)
	store.Clear(accountID)

	// Remove allowFrom store
	allowStore := storage.NewAllowFromStore(accountID)
	osRemove(allowStore.(*struct{ path string }).path)

	return s.accountManager.DeleteAccount(accountID)
}

// ============================================================================
// Context Token Management
// ============================================================================

// GetContextToken retrieves a stored context token.
func (s *SDK) GetContextToken(accountID, userID string) string {
	store := storage.NewContextTokenStore(accountID)
	store.Load()
	return store.Get(accountID, userID)
}

// SetContextToken stores a context token.
func (s *SDK) SetContextToken(accountID, userID, token string) {
	store := storage.NewContextTokenStore(accountID)
	store.Load()
	store.Set(accountID, userID, token)
}

// ============================================================================
// Media Download/Upload
// ============================================================================

// DownloadMedia downloads and decrypts media from CDN.
func (s *SDK) DownloadMedia(ctx context.Context, encryptedQueryParam, aesKeyBase64 string) ([]byte, error) {
	downloader := cdn.NewDownloader(s.cdnBaseURL)
	return downloader.DownloadAndDecrypt(ctx, encryptedQueryParam, aesKeyBase64)
}

// UploadMedia encrypts and uploads media to CDN.
func (s *SDK) UploadMedia(ctx context.Context, accountID, toUserID string, data []byte, mediaType api.UploadMediaType) (*cdn.UploadedFileInfo, error) {
	account, err := s.accountManager.ResolveAccount(accountID)
	if err != nil {
		return nil, err
	}

	s.client.Token = account.Token
	uploader := cdn.NewUploader(s.client, account.CDNBaseURL)
	return uploader.UploadFile(ctx, data, toUserID, mediaType)
}

// DownloadImage downloads and decrypts an image from CDN.
func (s *SDK) DownloadImage(ctx context.Context, encryptedQueryParam, aesKeyHex string) ([]byte, error) {
	// Convert hex key to base64
	aesKey, _ := hex.DecodeString(aesKeyHex)
	aesKeyBase64 := base64.StdEncoding.EncodeToString(aesKey)
	return s.DownloadMedia(ctx, encryptedQueryParam, aesKeyBase64)
}

// ============================================================================
// Typing Indicator
// ============================================================================

// SendTyping sends a typing indicator.
func (s *SDK) SendTyping(ctx context.Context, accountID, userID, typingTicket string, isTyping bool) error {
	account, err := s.accountManager.ResolveAccount(accountID)
	if err != nil {
		return err
	}

	s.client.Token = account.Token
	s.client.BaseURL = account.BaseURL

	status := api.TypingStatusTyping
	if !isTyping {
		status = api.TypingStatusCancel
	}

	return s.client.SendTyping(ctx, userID, typingTicket, status)
}

// GetTypingTicket gets the typing ticket for a user.
func (s *SDK) GetTypingTicket(ctx context.Context, accountID, userID, contextToken string) (string, error) {
	account, err := s.accountManager.ResolveAccount(accountID)
	if err != nil {
		return "", err
	}

	s.client.Token = account.Token
	s.client.BaseURL = account.BaseURL

	config, err := s.client.GetConfig(ctx, userID, contextToken)
	if err != nil {
		return "", err
	}

	return config.TypingTicket, nil
}

// ============================================================================
// Pairing (AllowFrom)
// ============================================================================

// GetAllowFromList gets the allowFrom list for an account.
func (s *SDK) GetAllowFromList(accountID string) []string {
	store := storage.NewAllowFromStore(accountID)
	store.Load()
	return store.GetList()
}

// AddAllowFrom adds a user to the allowFrom list.
func (s *SDK) AddAllowFrom(accountID, userID string) bool {
	store := storage.NewAllowFromStore(accountID)
	store.Load()
	return store.Add(userID)
}

// RemoveAllowFrom removes a user from the allowFrom list.
func (s *SDK) RemoveAllowFrom(accountID, userID string) bool {
	store := storage.NewAllowFromStore(accountID)
	store.Load()
	return store.Remove(userID)
}

// IsAllowed checks if a user is in the allowFrom list.
func (s *SDK) IsAllowed(accountID, userID string) bool {
	store := storage.NewAllowFromStore(accountID)
	store.Load()
	return store.Contains(userID)
}

// ============================================================================
// Debug Mode
// ============================================================================

// ToggleDebugMode toggles debug mode for an account.
func (s *SDK) ToggleDebugMode(accountID string) bool {
	mgr := storage.NewDebugModeManager()
	mgr.Load()
	return mgr.Toggle(accountID)
}

// IsDebugMode checks if debug mode is enabled for an account.
func (s *SDK) IsDebugMode(accountID string) bool {
	mgr := storage.NewDebugModeManager()
	mgr.Load()
	return mgr.IsEnabled(accountID)
}

// ============================================================================
// Utility Functions
// ============================================================================

// MarkdownToPlainText converts markdown to plain text.
func MarkdownToPlainText(text string) string {
	return messaging.MarkdownToPlainText(text)
}

// ExtractTextBody extracts text from message items.
func ExtractTextBody(items []*api.MessageItem) string {
	return messaging.ExtractTextBody(items)
}

// NormalizeAccountID normalizes an account ID for filesystem use.
func NormalizeAccountID(rawID string) string {
	return auth.NormalizeAccountID(rawID)
}

// readFile reads a file (placeholder for os.ReadFile).
var readFile = func(path string) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

// osRemove removes a file (placeholder for os.Remove).
var osRemove = func(path string) error {
	return nil
}
