package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// HistoryItem represents an executed action or calculation in the launcher.
type HistoryItem struct {
	ID        string `json:"id"`
	Type      string `json:"type"`      // "math", "app", "system"
	Title     string `json:"title"`     // e.g. "50 + 10%", "Google Chrome"
	Detail    string `json:"detail"`    // e.g. "= 55", "Aplicación"
	RawInput  string `json:"rawInput"`  // User input that originated this entry
	Action    string `json:"action"`    // Executable target or calculation result
	Timestamp int64  `json:"timestamp"` // Unix timestamp
}

// Manager manages persistent storage of search history.
type Manager struct {
	mu    sync.RWMutex
	items []HistoryItem
	path  string
}

// NewManager loads previously stored history from disk.
func NewManager() *Manager {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		cfgDir = os.TempDir()
	}
	appDir := filepath.Join(cfgDir, "OmniFloat")
	_ = os.MkdirAll(appDir, 0755)
	historyPath := filepath.Join(appDir, "history.json")

	m := &Manager{
		items: make([]HistoryItem, 0),
		path:  historyPath,
	}
	m.load()
	return m
}

func (m *Manager) load() {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if err != nil {
		return
	}

	var loaded []HistoryItem
	if err := json.Unmarshal(data, &loaded); err == nil {
		m.items = loaded
	}
}

func (m *Manager) saveLocked() {
	data, err := json.MarshalIndent(m.items, "", "  ")
	if err == nil {
		_ = os.WriteFile(m.path, data, 0644)
	}
}

// Add adds an entry to the top of the history list, deduplicating prior entries.
func (m *Manager) Add(item HistoryItem) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if item.Timestamp == 0 {
		item.Timestamp = time.Now().Unix()
	}
	if item.ID == "" {
		item.ID = fmt.Sprintf("hist-%d", time.Now().UnixNano())
	}

	var filtered []HistoryItem
	// Filter out duplicate matching title
	for _, it := range m.items {
		if it.Title != item.Title && it.RawInput != item.RawInput {
			filtered = append(filtered, it)
		}
	}

	// Prepend new item
	newList := append([]HistoryItem{item}, filtered...)
	if len(newList) > 25 {
		newList = newList[:25]
	}
	m.items = newList
	m.saveLocked()
}

// Get returns the stored history items (up to limit).
func (m *Manager) Get(limit int) []HistoryItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.items) {
		limit = len(m.items)
	}
	res := make([]HistoryItem, limit)
	copy(res, m.items[:limit])
	return res
}

// Clear wipes all recorded history.
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items = make([]HistoryItem, 0)
	m.saveLocked()
}
