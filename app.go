package main

import (
	"context"
	"log"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/rac-developer/omnifloat/pkg/apps"
	"github.com/rac-developer/omnifloat/pkg/config"
	"github.com/rac-developer/omnifloat/pkg/history"
	"github.com/rac-developer/omnifloat/pkg/hotkey"
	"github.com/rac-developer/omnifloat/pkg/ipc"
	pkgMath "github.com/rac-developer/omnifloat/pkg/math"
	"github.com/rac-developer/omnifloat/pkg/system"
	"github.com/rac-developer/omnifloat/pkg/updater"
)

const (
	AppVersion       = "1.0.0"
	LauncherWidth    = 720
	LauncherHeight   = 520
	IslandWidth      = 360
	IslandHeight     = 44
	IslandMenuWidth  = 220
	IslandMenuHeight = 194
)

// App struct holds application context, configuration, and services.
type App struct {
	ctx              context.Context
	cfgMgr           *config.Manager
	evaluator        *pkgMath.Evaluator
	monitor          *system.Monitor
	updater          *updater.Updater
	hotkeyMgr        *hotkey.Manager
	ipcServer        *ipc.Server
	appsIdx          *apps.Indexer
	histMgr          *history.Manager
	backdrop         *system.BackdropCapturer
	islandCmd        *exec.Cmd
	stopFocusWatcher func()
	mu               sync.Mutex
	isVisible        bool
	startHidden      bool
	isIslandMode     bool
	settingsOpen      bool
	islandMenuOpen    bool
	islandCustomWidth int
	lastShownAt       time.Time
}

// NewApp creates a new App application struct.
func NewApp(isIslandMode bool) *App {
	return &App{
		cfgMgr:       config.NewManager(),
		evaluator:    pkgMath.NewEvaluator(),
		monitor:      system.NewMonitor(1200 * time.Millisecond),
		updater:      updater.NewUpdater(AppVersion),
		appsIdx:      apps.NewIndexer(),
		histMgr:      history.NewManager(),
		backdrop:     system.NewBackdropCapturer(),
		isVisible:    false,
		startHidden:  false,
		isIslandMode: isIslandMode,
	}
}

// SetStartHidden configures initial visibility.
func (a *App) SetStartHidden(hidden bool) {
	a.startHidden = hidden
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Start system monitoring goroutine
	a.monitor.Start(ctx)

	if a.isIslandMode {
		// Island Mode: always stays visible in the corner
		a.isVisible = true
		go a.watchConfigChanges(ctx)

		// Watch clicks outside the Island window to close its context menu
		a.stopFocusWatcher = system.WatchClickOutside("OmniFloat Island", func() {
			a.mu.Lock()
			menuOpen := a.islandMenuOpen
			a.mu.Unlock()
			if menuOpen {
				a.SetIslandMenuOpen(false)
				wailsRuntime.EventsEmit(ctx, "island:close_menu")
			}
		})
		return
	}

	// Launcher Mode: Register global Alt+Space hotkey
	a.hotkeyMgr = hotkey.NewManager(func() {
		a.ToggleLauncher()
	})
	if err := a.hotkeyMgr.Start(); err != nil {
		wailsRuntime.LogWarningf(ctx, "Hotkey registration notice: %v", err)
	}

	// Start click-outside & focus watcher: hide launcher when clicking anywhere outside "OmniFloat"
	a.stopFocusWatcher = system.WatchClickOutside("OmniFloat", func() {
		a.mu.Lock()
		visible := a.isVisible
		sinceShown := time.Since(a.lastShownAt)
		a.mu.Unlock()
		if visible && sinceShown > 250*time.Millisecond {
			a.HideLauncher()
		}
	})

	// Trigger non-blocking GitHub release check
	a.updater.CheckAndNotify(ctx)

	// If Floating Island HUD is enabled in settings, start its companion process
	cfg := a.cfgMgr.Get()
	if cfg.Island.Enabled {
		go a.startIslandProcess()
	}
}

// domReady is called after front-end resources are loaded.
func (a *App) domReady(ctx context.Context) {
	if a.isIslandMode {
		// Hide Island from the Windows taskbar so only the system tray icon is used
		system.HideFromTaskbar("OmniFloat Island")
		// Reposition island to the configured corner right above the taskbar
		cfg := a.cfgMgr.Get()
		a.PositionIsland(cfg.Island.Position)
		go func() {
			time.Sleep(80 * time.Millisecond)
			if uri, ok := a.backdrop.CaptureBehind("OmniFloat Island", true); ok && uri != "" {
				wailsRuntime.EventsEmit(ctx, "backdrop:updated", uri)
			}
		}()
		// Keep island permanently pinned on top without stealing focus & refresh live glass backdrop
		go func() {
			pinTicker := time.NewTicker(1500 * time.Millisecond)
			glassTicker := time.NewTicker(220 * time.Millisecond)
			defer pinTicker.Stop()
			defer glassTicker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-pinTicker.C:
					curCfg := a.cfgMgr.Get()
					a.PositionIsland(curCfg.Island.Position)
				case <-glassTicker.C:
					if uri, ok := a.backdrop.CaptureBehind("OmniFloat Island", false); ok && uri != "" {
						wailsRuntime.EventsEmit(ctx, "backdrop:updated", uri)
					}
				}
			}
		}()
		return
	}

	// Ensure main launcher window also stays out of the taskbar (living in System Tray)
	system.HideFromTaskbar("OmniFloat")

	// Start live glass backdrop updater for the Launcher when visible
	go func() {
		ticker := time.NewTicker(140 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.mu.Lock()
				vis := a.isVisible
				a.mu.Unlock()
				if vis {
					if uri, ok := a.backdrop.CaptureBehind("OmniFloat", false); ok && uri != "" {
						wailsRuntime.EventsEmit(ctx, "backdrop:updated", uri)
					}
				}
			}
		}
	}()

	// Launcher Mode: Center and display if not started hidden
	if !a.startHidden {
		a.mu.Lock()
		a.lastShownAt = time.Now()
		a.isVisible = true
		a.mu.Unlock()

		a.CenterLauncher()
		wailsRuntime.WindowShow(ctx)
		wailsRuntime.WindowUnminimise(ctx)
		wailsRuntime.WindowSetAlwaysOnTop(ctx, true)
		go func() {
			time.Sleep(60 * time.Millisecond)
			system.ForceForegroundWindow("OmniFloat")
			if uri, ok := a.backdrop.CaptureBehind("OmniFloat", true); ok && uri != "" {
				wailsRuntime.EventsEmit(ctx, "backdrop:updated", uri)
			}
		}()
		wailsRuntime.EventsEmit(ctx, "launcher:opened")
	}
}

// shutdown is called at application termination.
func (a *App) shutdown(ctx context.Context) {
	if a.monitor != nil {
		a.monitor.Stop()
	}
	if a.hotkeyMgr != nil {
		a.hotkeyMgr.Stop()
	}
	if a.ipcServer != nil {
		_ = a.ipcServer.Close()
	}
	if a.stopFocusWatcher != nil {
		a.stopFocusWatcher()
	}
	a.stopIslandProcess()
}

// SetIPCServer attaches the active IPC server to the app.
func (a *App) SetIPCServer(server *ipc.Server) {
	a.ipcServer = server
}

// ==========================================
// Window Positioning & Geometry
// ==========================================

// CenterLauncher places the command palette in the dead center (slightly golden-ratio elevated).
func (a *App) CenterLauncher() {
	if a.ctx == nil {
		return
	}

	screens, err := wailsRuntime.ScreenGetAll(a.ctx)
	if err == nil && len(screens) > 0 {
		primary := screens[0]
		for _, s := range screens {
			if s.IsPrimary {
				primary = s
				break
			}
		}
		w := LauncherWidth
		h := LauncherHeight
		sw := primary.Size.Width
		sh := primary.Size.Height
		if sw <= 0 {
			sw = primary.Width
		}
		if sh <= 0 {
			sh = primary.Height
		}
		if sw <= 0 {
			sw = 1920
		}
		if sh <= 0 {
			sh = 1080
		}

		x := (sw - w) / 2
		y := (sh - h) / 3
		wailsRuntime.WindowSetPosition(a.ctx, x, y)
		wailsRuntime.WindowSetSize(a.ctx, w, h)
	} else {
		wailsRuntime.WindowCenter(a.ctx)
	}
}

func calcIslandWidth(ic config.IslandConfig) int {
	w := 32 // base padding + pulse dot
	if ic.ShowCPU {
		w += 54
	}
	if ic.ShowRAM {
		w += 56
	}
	if ic.ShowNet {
		w += 116
	}
	if ic.ShowClock {
		w += 58
	}
	if ic.ShowSearchBtn {
		w += 74
	}
	if w < 56 {
		w = 56
	}
	return w
}

// PositionIsland places the floating island HUD docked flush right above the Windows taskbar.
func (a *App) PositionIsland(pos string) {
	if a.ctx == nil {
		return
	}

	a.mu.Lock()
	menuOpen := a.islandMenuOpen
	customW := a.islandCustomWidth
	a.mu.Unlock()

	cfg := a.cfgMgr.Get()
	wa := system.GetPrimaryWorkArea()
	w := calcIslandWidth(cfg.Island)
	if customW >= 50 && customW <= 600 {
		w = customW
	}
	h := IslandHeight
	if menuOpen {
		w = IslandMenuWidth
		h = IslandMenuHeight
	}
	var x, y int

	// Flush against the corner and right above the Windows taskbar
	taskbarGap := 4
	edgeMargin := 6

	switch pos {
	case "bottom-left":
		x = wa.X + edgeMargin
		y = wa.Bottom - h - taskbarGap
	case "top-right":
		x = wa.Right - w - edgeMargin
		y = wa.Y + edgeMargin
	case "top-left":
		x = wa.X + edgeMargin
		y = wa.Y + edgeMargin
	default: // "bottom-right"
		x = wa.Right - w - edgeMargin
		y = wa.Bottom - h - taskbarGap
	}

	if !system.PinTopMostWindow("OmniFloat Island", x, y, w, h) {
		wailsRuntime.WindowSetPosition(a.ctx, x, y)
		wailsRuntime.WindowSetSize(a.ctx, w, h)
		wailsRuntime.WindowSetAlwaysOnTop(a.ctx, true)
	}
}

// watchConfigChanges monitors config file updates on disk and synchronizes the Island.
func (a *App) watchConfigChanges(ctx context.Context) {
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if updatedCfg, changed := a.cfgMgr.Reload(); changed {
				a.PositionIsland(updatedCfg.Island.Position)
				wailsRuntime.EventsEmit(ctx, "config:updated", updatedCfg)
			}
		}
	}
}

// ==========================================
// Island Companion Process Supervision
// ==========================================

func (a *App) startIslandProcess() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.islandCmd != nil && a.islandCmd.Process != nil {
		return // Already running
	}

	exePath, err := os.Executable()
	if err != nil {
		return
	}

	cmd := exec.Command(exePath, "--island")
	if err := cmd.Start(); err != nil {
		log.Printf("[OmniFloat] Could not spawn island HUD: %v", err)
		return
	}
	a.islandCmd = cmd

	// Monitor child in background — auto-restart if it exits unexpectedly while still enabled
	go func() {
		_ = cmd.Wait()
		a.mu.Lock()
		a.islandCmd = nil
		appCtx := a.ctx
		a.mu.Unlock()
		// Restart unless shutdown is in progress or Island was disabled in settings
		if appCtx != nil {
			select {
			case <-appCtx.Done():
				return
			default:
				time.Sleep(500 * time.Millisecond)
				if a.cfgMgr.Get().Island.Enabled {
					go a.startIslandProcess()
				}
			}
		}
	}()
}

func (a *App) stopIslandProcess() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.islandCmd != nil && a.islandCmd.Process != nil {
		_ = a.islandCmd.Process.Kill()
		a.islandCmd = nil
	}
}

// ==========================================
// Wails UI Bindings
// ==========================================

// EvaluateMath safely evaluates arithmetic expressions with standard operators,
// functions, and percentage calculations.
func (a *App) EvaluateMath(input string) string {
	return a.evaluator.Evaluate(input)
}

// ToggleLauncher shows or hides the floating launcher window without touching the island.
func (a *App) ToggleLauncher() {
	if a.isIslandMode {
		ipc.SendCommand("TOGGLE")
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.ctx == nil {
		return
	}

	if a.isVisible && !wailsRuntime.WindowIsMinimised(a.ctx) {
		wailsRuntime.WindowHide(a.ctx)
		a.isVisible = false
	} else {
		a.lastShownAt = time.Now()
		a.CenterLauncher()
		wailsRuntime.WindowShow(a.ctx)
		wailsRuntime.WindowUnminimise(a.ctx)
		wailsRuntime.WindowSetAlwaysOnTop(a.ctx, true)
		a.isVisible = true
		go func() {
			time.Sleep(50 * time.Millisecond)
			system.ForceForegroundWindow("OmniFloat")
			if a.ctx != nil {
				if uri, ok := a.backdrop.CaptureBehind("OmniFloat", true); ok && uri != "" {
					wailsRuntime.EventsEmit(a.ctx, "backdrop:updated", uri)
				}
			}
		}()
		wailsRuntime.EventsEmit(a.ctx, "launcher:opened")
	}
}

// ShowLauncher ensures the launcher is visible, centered and focused.
func (a *App) ShowLauncher() {
	if a.isIslandMode {
		// When called from Island, signal main launcher via IPC
		ipc.SendCommand("SHOW")
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.ctx == nil {
		return
	}

	a.lastShownAt = time.Now()
	a.CenterLauncher()
	wailsRuntime.WindowShow(a.ctx)
	wailsRuntime.WindowUnminimise(a.ctx)
	wailsRuntime.WindowSetAlwaysOnTop(a.ctx, true)
	a.isVisible = true
	go func() {
		time.Sleep(50 * time.Millisecond)
		system.ForceForegroundWindow("OmniFloat")
		if a.ctx != nil {
			if uri, ok := a.backdrop.CaptureBehind("OmniFloat", true); ok && uri != "" {
				wailsRuntime.EventsEmit(a.ctx, "backdrop:updated", uri)
			}
		}
	}()
	wailsRuntime.EventsEmit(a.ctx, "launcher:opened")
}

// HideLauncher hides the launcher window to background.
func (a *App) HideLauncher() {
	if a.isIslandMode {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.ctx == nil {
		return
	}

	wailsRuntime.WindowHide(a.ctx)
	a.isVisible = false
}

// SearchApps searches indexed applications and system tools.
func (a *App) SearchApps(query string) []apps.AppItem {
	if a.appsIdx == nil {
		return nil
	}
	return a.appsIdx.Search(query)
}

// LaunchApp launches an indexed application or shortcut and saves it to history.
func (a *App) LaunchApp(path string, name string) bool {
	if err := apps.Launch(path); err != nil {
		return false
	}
	if a.histMgr != nil {
		a.histMgr.Add(history.HistoryItem{
			Type:     "app",
			Title:    name,
			Detail:   "Aplicación",
			RawInput: name,
			Action:   path,
		})
	}
	a.HideLauncher()
	return true
}

// SearchInBrowser opens the user's query (or direct URL) in the system's default web browser.
func (a *App) SearchInBrowser(query string) bool {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return false
	}

	targetURL := ""
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		targetURL = trimmed
	} else if !strings.ContainsAny(trimmed, " \t") && strings.Contains(trimmed, ".") && !strings.HasSuffix(trimmed, ".") {
		targetURL = "https://" + trimmed
	} else {
		targetURL = "https://www.google.com/search?q=" + url.QueryEscape(trimmed)
	}

	if a.ctx != nil {
		wailsRuntime.BrowserOpenURL(a.ctx, targetURL)
	} else {
		_ = apps.Launch(targetURL)
	}

	if a.histMgr != nil {
		a.histMgr.Add(history.HistoryItem{
			Type:     "web",
			Title:    trimmed,
			Detail:   "Búsqueda en el navegador",
			RawInput: trimmed,
			Action:   trimmed,
		})
	}
	a.HideLauncher()
	return true
}

// RecordCalculation saves an arithmetic calculation to history.
func (a *App) RecordCalculation(expression string, result string) bool {
	if a.histMgr != nil {
		a.histMgr.Add(history.HistoryItem{
			Type:     "math",
			Title:    expression,
			Detail:   "= " + result,
			RawInput: expression,
			Action:   result,
		})
	}
	return true
}

// GetHistory returns recent calculations and executions.
func (a *App) GetHistory() []history.HistoryItem {
	if a.histMgr == nil {
		return nil
	}
	return a.histMgr.Get(6)
}

// ClearHistory wipes search and calculation history.
func (a *App) ClearHistory() bool {
	if a.histMgr == nil {
		return false
	}
	a.histMgr.Clear()
	return true
}

// SetIslandPosition updates the island corner position from context menu or settings.
func (a *App) SetIslandPosition(pos string) bool {
	cfg := a.cfgMgr.Get()
	cfg.Island.Position = pos
	_ = a.SaveConfig(cfg)
	if a.isIslandMode {
		a.PositionIsland(pos)
	}
	return true
}

// SetIslandMenuOpen expands or collapses the Island window height so the right-click context menu is never clipped.
func (a *App) SetIslandMenuOpen(open bool) {
	a.mu.Lock()
	a.islandMenuOpen = open
	a.mu.Unlock()
	if a.isIslandMode {
		cfg := a.cfgMgr.Get()
		a.PositionIsland(cfg.Island.Position)
	}
}

// SetIslandContentWidth dynamically resizes the Island window to match the exact measured width of its visible DOM content.
func (a *App) SetIslandContentWidth(width int) {
	if width < 50 || width > 600 {
		return
	}
	a.mu.Lock()
	changed := a.islandCustomWidth != width
	a.islandCustomWidth = width
	a.mu.Unlock()
	if changed && a.isIslandMode {
		cfg := a.cfgMgr.Get()
		a.PositionIsland(cfg.Island.Position)
	}
}

// OpenLauncherSettings signals the main Launcher to open directly in the Settings view.
func (a *App) OpenLauncherSettings() {
	if a.isIslandMode {
		ipc.SendCommand("SHOW_SETTINGS")
		return
	}
	a.ShowLauncher()
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "launcher:open_settings")
	}
}

// SetSettingsOpen notifies the backend whether the settings panel is visible.
func (a *App) SetSettingsOpen(open bool) {
	a.mu.Lock()
	a.settingsOpen = open
	a.mu.Unlock()
}

// GetConfig returns the full application settings.
func (a *App) GetConfig() config.AppConfig {
	return a.cfgMgr.Get()
}

// ReloadConfigFromDisk reloads the configuration file from disk and notifies the frontend without triggering an IPC loop.
func (a *App) ReloadConfigFromDisk() config.AppConfig {
	cfg, _ := a.cfgMgr.Reload()
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "config:updated", cfg)
	}
	return cfg
}

// SaveConfig saves updated settings, applies them in real time, and synchronizes the Island.
func (a *App) SaveConfig(cfg config.AppConfig) bool {
	prev := a.cfgMgr.Get()
	if err := a.cfgMgr.Save(cfg); err != nil {
		return false
	}

	// Reload normalized config so frontend receives exact clamped values
	savedCfg := a.cfgMgr.Get()

	// Emit event to update frontends immediately
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "config:updated", savedCfg)
	}

	if !a.isIslandMode {
		// Manage Island process state (Island watches config.json changes automatically)
		if savedCfg.Island.Enabled && !prev.Island.Enabled {
			go a.startIslandProcess()
		} else if !savedCfg.Island.Enabled && prev.Island.Enabled {
			a.stopIslandProcess()
		}
	} else {
		// Island re-positions itself if position changed and notifies main launcher
		if savedCfg.Island.Position != prev.Island.Position {
			a.PositionIsland(savedCfg.Island.Position)
		}
		ipc.SendCommand("CONFIG_UPDATED")
	}

	return true
}

// ResetConfig restores default settings.
func (a *App) ResetConfig() config.AppConfig {
	cfg := a.cfgMgr.Reset()
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "config:updated", cfg)
	}
	return cfg
}

// GetSystemStats returns the current hardware metrics snapshot.
func (a *App) GetSystemStats() system.Stats {
	return a.monitor.GetLastStats()
}

// CheckForUpdates manually triggers an update check and returns info.
func (a *App) CheckForUpdates() (*updater.UpdateInfo, error) {
	return a.updater.CheckForUpdates()
}

// ExecuteAction executes quick OS level operations (Terminal, Calculator, etc.).
func (a *App) ExecuteAction(action string) string {
	go func() {
		switch action {
		case "terminal":
			if runtime.GOOS == "windows" {
				_ = exec.Command("cmd", "/c", "start", "wt").Run()
			} else {
				_ = exec.Command("x-terminal-emulator").Run()
			}
		case "calculator":
			if runtime.GOOS == "windows" {
				_ = exec.Command("calc.exe").Run()
			} else {
				_ = exec.Command("gnome-calculator").Run()
			}
		case "lock":
			if runtime.GOOS == "windows" {
				_ = exec.Command("rundll32.exe", "user32.dll,LockWorkStation").Run()
			} else {
				_ = exec.Command("xdg-screensaver", "lock").Run()
			}
		}
	}()
	return "OK"
}

// QuitApp shuts down the application and its companion processes.
func (a *App) QuitApp() {
	a.stopIslandProcess()
	if a.ctx != nil {
		wailsRuntime.Quit(a.ctx)
	}
}
