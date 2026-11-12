//go:build !windows

package system

// HideFromTaskbar is a no-op on non-Windows platforms.
func HideFromTaskbar(windowTitle string) {}

// ForceForegroundWindow is a no-op on non-Windows platforms.
func ForceForegroundWindow(windowTitle string) {}

// PinTopMostWindow is a no-op on non-Windows platforms.
func PinTopMostWindow(windowTitle string, x, y, w, h int) bool {
	return false
}

// WatchClickOutside is a no-op on non-Windows platforms (handled by window.blur).
func WatchClickOutside(windowTitle string, onOutside func()) func() {
	return func() {}
}
