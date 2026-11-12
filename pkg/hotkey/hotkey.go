//go:build !windows && !linux

package hotkey

import "sync"

// Manager manages registration and listening of the global hotkey (Alt+Space).
type Manager struct {
	onToggle func()
	mu       sync.Mutex
}

// NewManager creates a new hotkey manager configured for Alt+Space.
func NewManager(onToggle func()) *Manager {
	return &Manager{onToggle: onToggle}
}

// Start registers the global Alt+Space hotkey.
func (m *Manager) Start() error {
	return nil
}

// Stop unregisters the hotkey.
func (m *Manager) Stop() {}
