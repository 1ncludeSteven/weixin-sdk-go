// Package auth provides account management.
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// AccountData represents stored account credentials.
type AccountData struct {
	Token    string `json:"token,omitempty"`
	SavedAt  string `json:"savedAt,omitempty"`
	BaseURL  string `json:"baseUrl,omitempty"`
	UserID   string `json:"userId,omitempty"`
}

// ResolvedAccount represents a resolved account with all configuration.
type ResolvedAccount struct {
	AccountID  string
	BaseURL    string
	CDNBaseURL string
	Token      string
	Enabled    bool
	Configured bool
	Name       string
}

// AccountConfig represents per-account configuration.
type AccountConfig struct {
	Name       string `json:"name,omitempty"`
	Enabled    *bool  `json:"enabled,omitempty"`
	CDNBaseURL string `json:"cdnBaseUrl,omitempty"`
	RouteTag   string `json:"routeTag,omitempty"`
}

// AccountManager manages WeChat accounts.
type AccountManager struct {
	mu            sync.RWMutex
	stateDir      string
	accountIndex  []string
}

// NewAccountManager creates a new account manager.
func NewAccountManager(stateDir string) *AccountManager {
	if stateDir == "" {
		stateDir = defaultStateDir()
	}
	am := &AccountManager{
		stateDir: stateDir,
	}
	am.loadAccountIndex()
	return am
}

// defaultStateDir returns the default state directory.
func defaultStateDir() string {
	if dir := os.Getenv("OPENCLAW_STATE_DIR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("CLAWDBOT_STATE_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".openclaw")
}

// weixinStateDir returns the weixin-specific state directory.
func (am *AccountManager) weixinStateDir() string {
	return filepath.Join(am.stateDir, "openclaw-weixin")
}

// accountsDir returns the accounts directory.
func (am *AccountManager) accountsDir() string {
	return filepath.Join(am.weixinStateDir(), "accounts")
}

// accountIndexPath returns the path to the account index file.
func (am *AccountManager) accountIndexPath() string {
	return filepath.Join(am.weixinStateDir(), "accounts.json")
}

// loadAccountIndex loads the account index from disk.
func (am *AccountManager) loadAccountIndex() {
	path := am.accountIndexPath()
	data, err := os.ReadFile(path)
	if err != nil {
		am.accountIndex = []string{}
		return
	}

	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		am.accountIndex = []string{}
		return
	}

	am.accountIndex = ids
}

// saveAccountIndex saves the account index to disk.
func (am *AccountManager) saveAccountIndex() error {
	dir := filepath.Dir(am.accountIndexPath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(am.accountIndex, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(am.accountIndexPath(), data, 0644)
}

// ListAccountIDs returns all registered account IDs.
func (am *AccountManager) ListAccountIDs() []string {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return append([]string{}, am.accountIndex...)
}

// RegisterAccountID adds an account ID to the index.
func (am *AccountManager) RegisterAccountID(accountID string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	for _, id := range am.accountIndex {
		if id == accountID {
			return
		}
	}

	am.accountIndex = append(am.accountIndex, accountID)
	am.saveAccountIndex()
}

// UnregisterAccountID removes an account ID from the index.
func (am *AccountManager) UnregisterAccountID(accountID string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	newIndex := []string{}
	for _, id := range am.accountIndex {
		if id != accountID {
			newIndex = append(newIndex, id)
		}
	}

	if len(newIndex) != len(am.accountIndex) {
		am.accountIndex = newIndex
		am.saveAccountIndex()
	}
}

// LoadAccount loads account data by ID.
func (am *AccountManager) LoadAccount(accountID string) (*AccountData, error) {
	path := am.accountPath(accountID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var account AccountData
	if err := json.Unmarshal(data, &account); err != nil {
		return nil, err
	}

	return &account, nil
}

// SaveAccount saves account data.
func (am *AccountManager) SaveAccount(accountID string, update *AccountData) error {
	dir := am.accountsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	existing, _ := am.LoadAccount(accountID)
	if existing == nil {
		existing = &AccountData{}
	}

	// Merge updates
	if update.Token != "" {
		existing.Token = update.Token
	}
	if update.BaseURL != "" {
		existing.BaseURL = update.BaseURL
	}
	if update.UserID != "" {
		existing.UserID = update.UserID
	}
	existing.SavedAt = timeNow()

	path := am.accountPath(accountID)
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}

	am.RegisterAccountID(accountID)
	return nil
}

// DeleteAccount removes all data for an account.
func (am *AccountManager) DeleteAccount(accountID string) error {
	// Delete account file
	accountPath := am.accountPath(accountID)
	os.Remove(accountPath)

	// Delete sync buf file
	syncPath := am.syncBufPath(accountID)
	os.Remove(syncPath)

	// Delete context tokens file
	ctxPath := am.contextTokensPath(accountID)
	os.Remove(ctxPath)

	// Remove from index
	am.UnregisterAccountID(accountID)

	return nil
}

// accountPath returns the path to an account file.
func (am *AccountManager) accountPath(accountID string) string {
	return filepath.Join(am.accountsDir(), accountID+".json")
}

// syncBufPath returns the path to a sync buffer file.
func (am *AccountManager) syncBufPath(accountID string) string {
	return filepath.Join(am.accountsDir(), accountID+".sync.json")
}

// contextTokensPath returns the path to a context tokens file.
func (am *AccountManager) contextTokensPath(accountID string) string {
	return filepath.Join(am.accountsDir(), accountID+".context-tokens.json")
}

// ResolveAccount resolves an account with all configuration.
func (am *AccountManager) ResolveAccount(accountID string) (*ResolvedAccount, error) {
	if accountID == "" {
		return nil, fmt.Errorf("accountId is required")
	}

	accountData, err := am.LoadAccount(accountID)
	if err != nil {
		accountData = &AccountData{}
	}

	baseURL := accountData.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	token := strings.TrimSpace(accountData.Token)
	configured := token != ""

	return &ResolvedAccount{
		AccountID:  accountID,
		BaseURL:    baseURL,
		CDNBaseURL: DefaultCDNBaseURL,
		Token:      token,
		Enabled:    true,
		Configured: configured,
	}, nil
}

// NormalizeAccountID normalizes an account ID for filesystem use.
func NormalizeAccountID(rawID string) string {
	// Replace @ with - and . with -
	return strings.ReplaceAll(strings.ReplaceAll(rawID, "@", "-"), ".", "-")
}

// DeriveRawAccountID reverses normalization for known suffixes.
func DeriveRawAccountID(normalizedID string) string {
	if strings.HasSuffix(normalizedID, "-im-bot") {
		return normalizedID[:len(normalizedID)-7] + "@im.bot"
	}
	if strings.HasSuffix(normalizedID, "-im-wechat") {
		return normalizedID[:len(normalizedID)-10] + "@im.wechat"
	}
	return ""
}

// timeNow returns the current time in ISO format.
func timeNow() string {
	return formatTime(timeNowFunc())
}

// These are variables for testing.
var (
	timeNowFunc = func() time.Time { return time.Now() }
	formatTime  = func(t time.Time) string { return t.Format(time.RFC3339) }
)
