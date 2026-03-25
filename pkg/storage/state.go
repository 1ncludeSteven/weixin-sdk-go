// Package storage provides state persistence utilities.
package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// StateDir returns the state directory.
func StateDir() string {
	if dir := os.Getenv("OPENCLAW_STATE_DIR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("CLAWDBOT_STATE_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".openclaw")
}

// WeixinStateDir returns the weixin-specific state directory.
func WeixinStateDir() string {
	return filepath.Join(StateDir(), "openclaw-weixin")
}

// SyncBufData represents persisted sync buffer.
type SyncBufData struct {
	GetUpdatesBuf string `json:"get_updates_buf,omitempty"`
}

// LoadSyncBuf loads the get_updates_buf for an account.
func LoadSyncBuf(accountID string) (string, error) {
	path := filepath.Join(WeixinStateDir(), "accounts", accountID+".sync.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil
	}

	var buf SyncBufData
	if err := json.Unmarshal(data, &buf); err != nil {
		return "", err
	}

	return buf.GetUpdatesBuf, nil
}

// SaveSyncBuf saves the get_updates_buf for an account.
func SaveSyncBuf(accountID, getUpdatesBuf string) error {
	path := filepath.Join(WeixinStateDir(), "accounts", accountID+".sync.json")
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	buf := SyncBufData{GetUpdatesBuf: getUpdatesBuf}
	data, err := json.Marshal(buf)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// ContextTokenStore manages context tokens.
type ContextTokenStore struct {
	mu       sync.RWMutex
	tokens   map[string]string // key: accountId:userId
	filePath string
}

// NewContextTokenStore creates a new context token store.
func NewContextTokenStore(accountID string) *ContextTokenStore {
	return &ContextTokenStore{
		tokens:   make(map[string]string),
		filePath: filepath.Join(WeixinStateDir(), "accounts", accountID+".context-tokens.json"),
	}
}

// Load loads persisted tokens from disk.
func (s *ContextTokenStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return nil
	}

	var tokens map[string]string
	if err := json.Unmarshal(data, &tokens); err != nil {
		return err
	}

	s.tokens = tokens
	return nil
}

// Save persists tokens to disk.
func (s *ContextTokenStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.Marshal(s.tokens)
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

// Get retrieves a context token.
func (s *ContextTokenStore) Get(accountID, userID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tokens[accountID+":"+userID]
}

// Set stores a context token.
func (s *ContextTokenStore) Set(accountID, userID, token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[accountID+":"+userID] = token
	s.Save()
}

// Clear removes all tokens for an account.
func (s *ContextTokenStore) Clear(accountID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prefix := accountID + ":"
	for k := range s.tokens {
		if strings.HasPrefix(k, prefix) {
			delete(s.tokens, k)
		}
	}
	s.Save()
}

// FindAccountIDsByContextToken finds account IDs that have a context token for a user.
func FindAccountIDsByContextToken(accountIDs []string, userID string) []string {
	// This would require loading all context token stores
	// For simplicity, return empty for now
	return []string{}
}

// DebugModeState represents debug mode state.
type DebugModeState struct {
	Accounts map[string]bool `json:"accounts"`
}

// DebugModeManager manages debug mode state.
type DebugModeManager struct {
	mu      sync.RWMutex
	state   *DebugModeState
	path    string
}

// NewDebugModeManager creates a new debug mode manager.
func NewDebugModeManager() *DebugModeManager {
	return &DebugModeManager{
		state: &DebugModeState{Accounts: make(map[string]bool)},
		path:  filepath.Join(WeixinStateDir(), "debug-mode.json"),
	}
}

// Load loads debug mode state from disk.
func (m *DebugModeManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if err != nil {
		return nil
	}

	return json.Unmarshal(data, m.state)
}

// Save saves debug mode state to disk.
func (m *DebugModeManager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.path, data, 0644)
}

// Toggle toggles debug mode for an account.
func (m *DebugModeManager) Toggle(accountID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	current := m.state.Accounts[accountID]
	m.state.Accounts[accountID] = !current
	m.Save()
	return !current
}

// IsEnabled checks if debug mode is enabled for an account.
func (m *DebugModeManager) IsEnabled(accountID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.Accounts[accountID]
}

// AllowFromStore manages the allowFrom list for pairing.
type AllowFromStore struct {
	mu      sync.RWMutex
	path    string
	content *AllowFromFileContent
}

// AllowFromFileContent represents the allowFrom file content.
type AllowFromFileContent struct {
	Version   int      `json:"version"`
	AllowFrom []string `json:"allowFrom"`
}

// NewAllowFromStore creates a new allowFrom store.
func NewAllowFromStore(accountID string) *AllowFromStore {
	home, _ := os.UserHomeDir()
	return &AllowFromStore{
		path:    filepath.Join(home, ".openclaw", "credentials", "openclaw-weixin-"+accountID+"-allowFrom.json"),
		content: &AllowFromFileContent{Version: 1, AllowFrom: []string{}},
	}
}

// Load loads the allowFrom list from disk.
func (s *AllowFromStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil
	}

	return json.Unmarshal(data, s.content)
}

// Save saves the allowFrom list to disk.
func (s *AllowFromStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.content, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0644)
}

// GetList returns the allowFrom list.
func (s *AllowFromStore) GetList() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string{}, s.content.AllowFrom...)
}

// Add adds a user ID to the allowFrom list.
func (s *AllowFromStore) Add(userID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range s.content.AllowFrom {
		if id == userID {
			return false
		}
	}

	s.content.AllowFrom = append(s.content.AllowFrom, userID)
	s.Save()
	return true
}

// Remove removes a user ID from the allowFrom list.
func (s *AllowFromStore) Remove(userID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	newList := []string{}
	changed := false
	for _, id := range s.content.AllowFrom {
		if id != userID {
			newList = append(newList, id)
		} else {
			changed = true
		}
	}

	if changed {
		s.content.AllowFrom = newList
		s.Save()
	}
	return changed
}

// Contains checks if a user ID is in the allowFrom list.
func (s *AllowFromStore) Contains(userID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, id := range s.content.AllowFrom {
		if id == userID {
			return true
		}
	}
	return false
}
