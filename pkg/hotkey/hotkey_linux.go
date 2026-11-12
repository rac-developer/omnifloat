//go:build linux

package hotkey

import (
	"context"
	"fmt"
	"log"
	"sync"

	"golang.design/x/hotkey"
)

// Manager manages registration and listening of the global hotkey (Alt+Space).
type Manager struct {
	hk       *hotkey.Hotkey
	onToggle func()
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	active   bool
}

// NewManager creates a new hotkey manager configured for Alt+Space.
func NewManager(onToggle func()) *Manager {
	return &Manager{
		onToggle: onToggle,
	}
}

// Start registers the global Alt+Space hotkey on Linux.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.active {
		return nil
	}

	hk := hotkey.New([]hotkey.Modifier{hotkey.ModAlt}, hotkey.KeySpace)
	err := hk.Register()
	if err != nil {
		return fmt.Errorf("failed to register global hotkey Alt+Space: %w", err)
	}

	m.hk = hk
	m.active = true
	m.ctx, m.cancel = context.WithCancel(context.Background())

	go m.listen()
	log.Println("[OmniFloat Hotkey] Global shortcut Alt+Space registered on Linux")
	return nil
}

func (m *Manager) listen() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.hk.Keydown():
			if m.onToggle != nil {
				go m.onToggle()
			}
		}
	}
}

// Stop unregisters the hotkey and releases system resources.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.active {
		return
	}

	if m.cancel != nil {
		m.cancel()
	}

	if m.hk != nil {
		_ = m.hk.Unregister()
		m.hk = nil
	}

	m.active = false
	log.Println("[OmniFloat Hotkey] Global shortcut unregistered")
}
