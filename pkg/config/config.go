package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// IslandConfig defines preferences for the independent floating island HUD.
type IslandConfig struct {
	Enabled       bool   `json:"enabled"`
	Position      string `json:"position"` // "bottom-right", "bottom-left", "top-right", "top-left"
	ShowCPU       bool   `json:"showCPU"`
	ShowRAM       bool   `json:"showRAM"`
	ShowNet       bool   `json:"showNet"`
	ShowClock     bool   `json:"showClock"`
	ShowSearchBtn bool   `json:"showSearchBtn"`
}

// AppConfig defines user customizations for visuals, widgets, and shortcuts.
type AppConfig struct {
	Theme         string       `json:"theme"`         // "dark-glass", "cyan-frost", "violet-aurora", "deep-obsidian"
	Opacity       float64      `json:"opacity"`       // 0.30 to 1.00
	BlurIntensity int          `json:"blurIntensity"` // 10 to 50
	Island        IslandConfig `json:"island"`
}

// Manager handles thread-safe loading and persistent storage of configuration.
type Manager struct {
	mu     sync.RWMutex
	config AppConfig
	path   string
}

// DefaultConfig returns clean, high-contrast default settings.
func DefaultConfig() AppConfig {
	return AppConfig{
		Theme:         "dark-glass",
		Opacity:       0.88,
		BlurIntensity: 28,
		Island: IslandConfig{
			Enabled:       true,
			Position:      "bottom-right",
			ShowCPU:       true,
			ShowRAM:       true,
			ShowNet:       true,
			ShowClock:     true,
			ShowSearchBtn: false,
		},
	}
}

// NewManager initializes the configuration manager and loads saved preferences.
func NewManager() *Manager {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}

	appDir := filepath.Join(configDir, "OmniFloat")
	_ = os.MkdirAll(appDir, 0755)
	cfgPath := filepath.Join(appDir, "config.json")

	m := &Manager{
		config: DefaultConfig(),
		path:   cfgPath,
	}
	m.Load()
	return m
}

// Load reads the JSON configuration from disk if present.
func (m *Manager) Load() {
	_, _ = m.Reload()
}

// Reload re-reads the JSON configuration from disk if present.
// It returns the current configuration and whether it changed compared to in-memory state.
func (m *Manager) Reload() (AppConfig, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if err != nil {
		// File does not exist yet; save defaults
		_ = m.saveLocked(m.config)
		return m.config, false
	}

	var loaded AppConfig
	if err := json.Unmarshal(data, &loaded); err != nil {
		return m.config, false
	}

	// Apply sanity checks
	if loaded.Opacity > 1.0 && loaded.Opacity <= 100.0 {
		loaded.Opacity = loaded.Opacity / 100.0
	}
	if loaded.Opacity < 0.0 || loaded.Opacity > 1.0 {
		loaded.Opacity = 0.88
	}
	if loaded.BlurIntensity < 5 || loaded.BlurIntensity > 60 {
		loaded.BlurIntensity = 28
	}
	if loaded.Theme == "" {
		loaded.Theme = "dark-glass"
	}
	if loaded.Island.Position == "" {
		loaded.Island.Position = "bottom-right"
	}

	if loaded == m.config {
		return m.config, false
	}

	m.config = loaded
	return m.config, true
}

// Get returns a copy of current configuration.
func (m *Manager) Get() AppConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// Save writes new configuration to disk and updates in-memory state.
func (m *Manager) Save(cfg AppConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validation
	if cfg.Opacity > 1.0 && cfg.Opacity <= 100.0 {
		cfg.Opacity = cfg.Opacity / 100.0
	}
	if cfg.Opacity < 0.0 {
		cfg.Opacity = 0.0
	}
	if cfg.Opacity > 1.0 {
		cfg.Opacity = 1.0
	}
	if cfg.BlurIntensity < 5 {
		cfg.BlurIntensity = 5
	}
	if cfg.BlurIntensity > 60 {
		cfg.BlurIntensity = 60
	}
	if cfg.Theme == "" {
		cfg.Theme = "dark-glass"
	}
	if cfg.Island.Position == "" {
		cfg.Island.Position = "bottom-right"
	}

	m.config = cfg
	return m.saveLocked(cfg)
}

func (m *Manager) saveLocked(cfg AppConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0644)
}

// Reset restores factory default settings.
func (m *Manager) Reset() AppConfig {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = DefaultConfig()
	_ = m.saveLocked(m.config)
	return m.config
}
