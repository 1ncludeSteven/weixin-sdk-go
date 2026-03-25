// Package monitor provides long-poll monitoring for WeChat messages.
package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/1ncludeSteven/weixin-sdk-go/pkg/api"
	"github.com/1ncludeSteven/weixin-sdk-go/pkg/storage"
)

const (
	DefaultLongPollTimeout = 35 * time.Second
	MaxConsecutiveFailures = 3
	BackoffDelay           = 30 * time.Second
	RetryDelay             = 2 * time.Second
	SessionPauseDuration   = 60 * time.Minute
	SessionExpiredErrCode  = -14
)

// MessageHandler handles incoming messages.
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg *api.WeixinMessage, accountID string) error
}

// MessageHandlerFunc is a function that implements MessageHandler.
type MessageHandlerFunc func(ctx context.Context, msg *api.WeixinMessage, accountID string) error

// HandleMessage implements MessageHandler.
func (f MessageHandlerFunc) HandleMessage(ctx context.Context, msg *api.WeixinMessage, accountID string) error {
	return f(ctx, msg, accountID)
}

// StatusCallback is called on status updates.
type StatusCallback func(status *Status)

// Status represents monitor status.
type Status struct {
	AccountID    string
	Running      bool
	LastStartAt  int64
	LastEventAt  int64
	LastError    string
	LastInbound  int64
	LastOutbound int64
}

// Monitor monitors WeChat messages via long-poll.
type Monitor struct {
	client         *api.Client
	accountID      string
	cdnBaseURL     string
	handler        MessageHandler
	statusCallback StatusCallback
	tokenStore     *storage.ContextTokenStore

	mu            sync.RWMutex
	running       bool
	paused        bool
	pauseUntil    time.Time
	getUpdatesBuf string
	status        *Status
}

// MonitorOption is a function that configures the Monitor.
type MonitorOption func(*Monitor)

// WithHandler sets the message handler.
func WithHandler(handler MessageHandler) MonitorOption {
	return func(m *Monitor) {
		m.handler = handler
	}
}

// WithStatusCallback sets the status callback.
func WithStatusCallback(cb StatusCallback) MonitorOption {
	return func(m *Monitor) {
		m.statusCallback = cb
	}
}

// WithCDNBaseURL sets the CDN base URL.
func WithCDNBaseURL(url string) MonitorOption {
	return func(m *Monitor) {
		m.cdnBaseURL = url
	}
}

// NewMonitor creates a new monitor.
func NewMonitor(client *api.Client, accountID string, opts ...MonitorOption) *Monitor {
	m := &Monitor{
		client:     client,
		accountID:  accountID,
		cdnBaseURL: api.DefaultCDNBaseURL,
		status: &Status{
			AccountID: accountID,
		},
		tokenStore: storage.NewContextTokenStore(accountID),
	}

	for _, opt := range opts {
		opt(m)
	}

	// Load persisted sync buffer
	if buf, _ := storage.LoadSyncBuf(accountID); buf != "" {
		m.getUpdatesBuf = buf
	}

	// Load context tokens
	m.tokenStore.Load()

	return m
}

// Start begins the long-poll loop.
func (m *Monitor) Start(ctx context.Context) error {
	m.mu.Lock()
	m.running = true
	m.status.Running = true
	m.status.LastStartAt = time.Now().Unix()
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.running = false
		m.status.Running = false
		m.mu.Unlock()
	}()

	consecutiveFailures := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check if paused
		if m.isPaused() {
			remaining := time.Until(m.pauseUntil)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(remaining):
				m.mu.Lock()
				m.paused = false
				m.mu.Unlock()
				continue
			}
		}

		resp, err := m.client.GetUpdates(ctx, m.getUpdatesBuf)
		if err != nil {
			m.handleError(err, &consecutiveFailures)
			continue
		}

		// Check for API errors
		if resp.Ret != 0 || resp.ErrCode != 0 {
			if resp.ErrCode == SessionExpiredErrCode || resp.Ret == SessionExpiredErrCode {
				m.pauseSession()
				continue
			}
			m.handleAPIError(resp.Ret, resp.ErrCode, resp.ErrMsg, &consecutiveFailures)
			continue
		}

		consecutiveFailures = 0
		m.updateStatus(func(s *Status) { s.LastEventAt = time.Now().Unix() })

		// Update sync buffer
		if resp.GetUpdatesBuf != "" {
			m.getUpdatesBuf = resp.GetUpdatesBuf
			storage.SaveSyncBuf(m.accountID, resp.GetUpdatesBuf)
		}

		// Process messages
		for _, msg := range resp.Msgs {
			m.updateStatus(func(s *Status) {
				s.LastInbound = time.Now().Unix()
				s.LastEventAt = time.Now().Unix()
			})

			// Store context token
			if msg.ContextToken != "" && msg.FromUserID != "" {
				m.tokenStore.Set(m.accountID, msg.FromUserID, msg.ContextToken)
			}

			// Handle message
			if m.handler != nil {
				if err := m.handler.HandleMessage(ctx, msg, m.accountID); err != nil {
					// Log error but continue
					m.updateStatus(func(s *Status) { s.LastError = err.Error() })
				}
			}
		}
	}
}

// Stop stops the monitor.
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
}

// IsRunning returns whether the monitor is running.
func (m *Monitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetStatus returns the current status.
func (m *Monitor) GetStatus() *Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// GetContextToken returns the context token for a user.
func (m *Monitor) GetContextToken(userID string) string {
	return m.tokenStore.Get(m.accountID, userID)
}

// SetContextToken sets a context token for a user.
func (m *Monitor) SetContextToken(userID, token string) {
	m.tokenStore.Set(m.accountID, userID, token)
}

// isPaused checks if the session is paused.
func (m *Monitor) isPaused() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.paused {
		return false
	}
	if time.Now().After(m.pauseUntil) {
		m.paused = false
		return false
	}
	return true
}

// pauseSession pauses the monitor for the session expiration cooldown.
func (m *Monitor) pauseSession() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.paused = true
	m.pauseUntil = time.Now().Add(SessionPauseDuration)
	m.updateStatusLocked(func(s *Status) {
		s.LastError = "session expired, pausing for 60 minutes"
	})
}

// handleError handles a polling error.
func (m *Monitor) handleError(err error, consecutiveFailures *int) {
	*consecutiveFailures++
	m.updateStatus(func(s *Status) { s.LastError = err.Error() })

	delay := RetryDelay
	if *consecutiveFailures >= MaxConsecutiveFailures {
		delay = BackoffDelay
		*consecutiveFailures = 0
	}
	time.Sleep(delay)
}

// handleAPIError handles an API error response.
func (m *Monitor) handleAPIError(ret, errCode int, errMsg string, consecutiveFailures *int) {
	*consecutiveFailures++
	m.updateStatus(func(s *Status) { s.LastError = errMsg })

	delay := RetryDelay
	if *consecutiveFailures >= MaxConsecutiveFailures {
		delay = BackoffDelay
		*consecutiveFailures = 0
	}
	time.Sleep(delay)
}

// updateStatus updates the status with a function.
func (m *Monitor) updateStatus(fn func(*Status)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fn(m.status)
	if m.statusCallback != nil {
		m.statusCallback(m.status)
	}
}

// updateStatusLocked updates the status with a function (must hold lock).
func (m *Monitor) updateStatusLocked(fn func(*Status)) {
	fn(m.status)
	if m.statusCallback != nil {
		m.statusCallback(m.status)
	}
}
