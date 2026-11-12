//go:build windows

package system

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	procGetForegroundWindow             = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow             = user32.NewProc("SetForegroundWindow")
	procSetActiveWindow                 = user32.NewProc("SetActiveWindow")
	procSetFocus                        = user32.NewProc("SetFocus")
	procSendMessageW                    = user32.NewProc("SendMessageW")
	procPostMessageW                    = user32.NewProc("PostMessageW")
	procGetWindow                       = user32.NewProc("GetWindow")
	procGetClassNameW                   = user32.NewProc("GetClassNameW")
	procKeybdEvent                      = user32.NewProc("keybd_event")
	procBringWindowToTop                = user32.NewProc("BringWindowToTop")
	procShowWindow               = user32.NewProc("ShowWindow")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procFindWindowW              = user32.NewProc("FindWindowW")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procGetCursorPos             = user32.NewProc("GetCursorPos")
	procGetAsyncKeyState         = user32.NewProc("GetAsyncKeyState")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procIsIconic                 = user32.NewProc("IsIconic")
	procGetWindowLongW                  = user32.NewProc("GetWindowLongW")
	procSetWindowLongW                  = user32.NewProc("SetWindowLongW")
	procSetWindowDisplayAffinity        = user32.NewProc("SetWindowDisplayAffinity")
	procSetWindowCompositionAttribute   = user32.NewProc("SetWindowCompositionAttribute")
	procGetWindowThreadProcessId        = user32.NewProc("GetWindowThreadProcessId")
	procAttachThreadInput               = user32.NewProc("AttachThreadInput")
	kernel32dll                         = syscall.NewLazyDLL("kernel32.dll")
	procGetCurrentThreadId              = kernel32dll.NewProc("GetCurrentThreadId")
	dwmapi                              = syscall.NewLazyDLL("dwmapi.dll")
	procDwmSetWindowAttribute           = dwmapi.NewProc("DwmSetWindowAttribute")
)

type point struct {
	X, Y int32
}

type accentPolicy struct {
	AccentState   uint32
	AccentFlags   uint32
	GradientColor uint32
	AnimationId   uint32
}

type windowCompositionAttribData struct {
	Attrib uint32
	PvData uintptr
	CbData uintptr
}

const (
	swHide           = 0
	swShow           = 5
	swRestore        = 9
	swShowNoActivate = 4
	hwndTopMost      = ^uintptr(0) // -1 (HWND_TOPMOST)
	swpNoSize        = 0x0001
	swpNoMove        = 0x0002
	swpNoZOrder      = 0x0004
	swpNoActivate    = 0x0010
	swpFrameChanged  = 0x0020
	swpShowWindow    = 0x0040
	wsExToolWindow   = 0x00000080
	wsExAppWindow    = 0x00040000

	wsCaption        = 0x00C00000
	wsSysMenu        = 0x00080000
	wsThickFrame     = 0x00040000
	wsMinimizeBox    = 0x00020000
	wsMaximizeBox    = 0x00010000

	vkLButton        = 0x01
	vkRButton        = 0x02
	vkMButton        = 0x04

	wdaExcludeFromCapture       = 0x00000011
	wcaAccentPolicy             = 19
	accentEnableBlurBehind      = 3
	dwmwaUseImmersiveDarkMode   = 20
	dwmwaWindowCornerPreference = 33
	dwmwcpRound                 = 2 // DWMWCP_ROUND
)

func applyDwmGlassAttributes(hwnd uintptr) {
	if hwnd == 0 {
		return
	}

	// Exclude window from GDI BitBlt/StretchBlt so BackdropCapturer sees the desktop behind it
	_, _, _ = procSetWindowDisplayAffinity.Call(hwnd, uintptr(wdaExcludeFromCapture))

	darkMode := int32(1)
	_, _, _ = procDwmSetWindowAttribute.Call(
		hwnd,
		uintptr(dwmwaUseImmersiveDarkMode),
		uintptr(unsafe.Pointer(&darkMode)),
		unsafe.Sizeof(darkMode),
	)

	cornerPref := int32(dwmwcpRound)
	_, _, _ = procDwmSetWindowAttribute.Call(
		hwnd,
		uintptr(dwmwaWindowCornerPreference),
		uintptr(unsafe.Pointer(&cornerPref)),
		unsafe.Sizeof(cornerPref),
	)

	// Enable Win32 Composition BlurBehind
	accent := accentPolicy{
		AccentState: accentEnableBlurBehind,
	}
	data := windowCompositionAttribData{
		Attrib: wcaAccentPolicy,
		PvData: uintptr(unsafe.Pointer(&accent)),
		CbData: unsafe.Sizeof(accent),
	}
	_, _, _ = procSetWindowCompositionAttribute.Call(hwnd, uintptr(unsafe.Pointer(&data)))
}

func ensureToolWindowStyle(hwnd uintptr, keepVisible bool) {
	if hwnd == 0 {
		return
	}
	gwlStyle := ^uintptr(15)   // -16 (GWL_STYLE)
	gwlExStyle := ^uintptr(19) // -20 (GWL_EXSTYLE)

	// Strip WS_CAPTION and WS_SYSMENU so Windows never renders the non-functional "X" button
	style, _, _ := procGetWindowLongW.Call(hwnd, gwlStyle)
	captionBits := uintptr(wsCaption | wsSysMenu | wsMinimizeBox | wsMaximizeBox | wsThickFrame)
	styleChanged := false
	if (style & captionBits) != 0 {
		_, _, _ = procSetWindowLongW.Call(hwnd, gwlStyle, style&^captionBits)
		styleChanged = true
	}

	exStyle, _, _ := procGetWindowLongW.Call(hwnd, gwlExStyle)
	if (exStyle&wsExToolWindow) == 0 || (exStyle&wsExAppWindow) != 0 {
		newStyle := (exStyle &^ wsExAppWindow) | wsExToolWindow
		_, _, _ = procSetWindowLongW.Call(hwnd, gwlExStyle, newStyle)
		_, _, _ = procSetWindowPos.Call(
			hwnd,
			0, 0, 0, 0, 0,
			uintptr(swpNoMove|swpNoSize|swpNoZOrder|swpNoActivate|swpFrameChanged),
		)
		applyDwmGlassAttributes(hwnd)
	} else if styleChanged {
		applyDwmGlassAttributes(hwnd)
	}
}

// HideFromTaskbar strips WS_EX_APPWINDOW and applies WS_EX_TOOLWINDOW so the window
// does not appear as an icon in the Windows taskbar (remaining only in the system tray).
func HideFromTaskbar(windowTitle string) {
	hwnd := findWindowByTitle(windowTitle)
	if hwnd == 0 {
		return
	}
	ensureToolWindowStyle(hwnd, false)
}

func findWindowByTitle(title string) uintptr {
	ptr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return 0
	}
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(ptr)))
	return hwnd
}

func isKeyDown(vk int) bool {
	ret, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return (uint16(ret) & 0x8000) != 0
}

func getWindowClassName(hwnd uintptr) string {
	if hwnd == 0 {
		return ""
	}
	var buf [128]uint16
	n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

// findChromeWidgetChild locates WebView2's keyboard input receiver ("Chrome_WidgetWin_1")
// without descending into "Intermediate D3D Window" (which would unfocus the input).
func findChromeWidgetChild(hwnd uintptr) uintptr {
	const (
		gwChild    = 5
		gwHwndNext = 2
	)
	var fallback uintptr
	queue := []uintptr{hwnd}
	for len(queue) > 0 && len(queue) < 32 {
		curr := queue[0]
		queue = queue[1:]

		child, _, _ := procGetWindow.Call(curr, uintptr(gwChild))
		for child != 0 {
			cls := getWindowClassName(child)
			if cls == "Chrome_WidgetWin_1" {
				return child
			}
			if cls == "Chrome_WidgetWin_0" && fallback == 0 {
				fallback = child
			}
			if cls != "Intermediate D3D Window" {
				queue = append(queue, child)
			}
			child, _, _ = procGetWindow.Call(child, uintptr(gwHwndNext))
		}
	}
	return fallback
}

func focusLauncherHwnd(hwnd uintptr, clickInput bool) {
	if hwnd == 0 {
		return
	}

	ensureToolWindowStyle(hwnd, true)

	fgHwnd, _, _ := procGetForegroundWindow.Call()
	var fgThread uintptr
	if fgHwnd != 0 {
		fgThread, _, _ = procGetWindowThreadProcessId.Call(fgHwnd, 0)
	}
	winThread, _, _ := procGetWindowThreadProcessId.Call(hwnd, 0)
	curThread, _, _ := procGetCurrentThreadId.Call()

	if fgThread != 0 && curThread != 0 && fgThread != curThread {
		_, _, _ = procAttachThreadInput.Call(fgThread, curThread, 1)
		defer procAttachThreadInput.Call(fgThread, curThread, 0)
	}
	if winThread != 0 && curThread != 0 && winThread != curThread {
		_, _, _ = procAttachThreadInput.Call(curThread, winThread, 1)
		defer procAttachThreadInput.Call(curThread, winThread, 0)
	}
	if fgThread != 0 && winThread != 0 && fgThread != winThread {
		_, _, _ = procAttachThreadInput.Call(fgThread, winThread, 1)
		defer procAttachThreadInput.Call(fgThread, winThread, 0)
	}

	// Unlock Windows foreground lock policy when triggered from background hotkey thread
	_, _, _ = procKeybdEvent.Call(0, 0, 0, 0)

	iconic, _, _ := procIsIconic.Call(hwnd)
	if iconic != 0 {
		_, _, _ = procShowWindow.Call(hwnd, swRestore)
	} else {
		_, _, _ = procShowWindow.Call(hwnd, swShow)
	}

	_, _, _ = procBringWindowToTop.Call(hwnd)
	_, _, _ = procSetForegroundWindow.Call(hwnd)
	_, _, _ = procSetActiveWindow.Call(hwnd)

	// Trigger Wails' mainWindow.OnSetFocus() -> f.chromium.Focus()
	const wmSetFocus = 0x0007
	_, _, _ = procSendMessageW.Call(hwnd, uintptr(wmSetFocus), 0, 0)

	// Focus WebView2's Chrome_WidgetWin_1 child window directly and send a synthetic
	// client click at (260, 68) — the center of #search-input — without moving the mouse.
	chromeHwnd := findChromeWidgetChild(hwnd)
	if chromeHwnd != 0 {
		chromeThread, _, _ := procGetWindowThreadProcessId.Call(chromeHwnd, 0)
		if chromeThread != 0 && chromeThread != curThread {
			_, _, _ = procAttachThreadInput.Call(curThread, chromeThread, 1)
			defer procAttachThreadInput.Call(curThread, chromeThread, 0)
		}
		if chromeThread != 0 && chromeThread != winThread {
			_, _, _ = procAttachThreadInput.Call(winThread, chromeThread, 1)
			defer procAttachThreadInput.Call(winThread, chromeThread, 0)
		}
		_, _, _ = procSetFocus.Call(chromeHwnd)

		if clickInput {
			const (
				wmLButtonDown = 0x0201
				wmLButtonUp   = 0x0202
				mkLButton     = 0x0001
			)
			// Coordinates (x=260, y=68) lie directly inside #search-input
			lParam := uintptr((68 << 16) | 260)
			_, _, _ = procPostMessageW.Call(chromeHwnd, uintptr(wmLButtonDown), uintptr(mkLButton), lParam)
			_, _, _ = procPostMessageW.Call(chromeHwnd, uintptr(wmLButtonUp), 0, lParam)
		}
	}
}

// ForceForegroundWindow brings the specified window to the front as the active foreground window
// and focuses WebView2's Chrome_WidgetWin_1 + #search-input immediately (and after Alt release).
func ForceForegroundWindow(windowTitle string) {
	hwnd := findWindowByTitle(windowTitle)
	if hwnd == 0 {
		return
	}

	focusLauncherHwnd(hwnd, true)

	// If the user opened the launcher via Alt+Space, the Alt key (VK_MENU = 0x12) may still
	// be physically held down; releasing Alt sends WM_SYSKEYUP which can steal focus from
	// Chromium's input. Wait for Alt release and re-focus immediately.
	const vkMenu = 0x12
	if isKeyDown(vkMenu) {
		go func() {
			deadline := time.Now().Add(800 * time.Millisecond)
			for time.Now().Before(deadline) {
				if !isKeyDown(vkMenu) {
					time.Sleep(15 * time.Millisecond)
					focusLauncherHwnd(hwnd, true)
					return
				}
				time.Sleep(15 * time.Millisecond)
			}
		}()
	}
}

// PinTopMostWindow positions and sizes a window at exact screen coordinates as always-on-top
// without stealing focus from the user's active window or showing a taskbar button.
func PinTopMostWindow(windowTitle string, x, y, w, h int) bool {
	hwnd := findWindowByTitle(windowTitle)
	if hwnd == 0 {
		return false
	}

	ensureToolWindowStyle(hwnd, true)

	iconic, _, _ := procIsIconic.Call(hwnd)
	if iconic != 0 {
		_, _, _ = procShowWindow.Call(hwnd, swShowNoActivate)
	}

	_, _, _ = procSetWindowPos.Call(
		hwnd,
		hwndTopMost,
		uintptr(x),
		uintptr(y),
		uintptr(w),
		uintptr(h),
		uintptr(swpShowWindow|swpNoActivate),
	)
	return true
}

// WatchClickOutside monitors mouse clicks outside the target window and foreground focus loss.
// When the user clicks outside the window bounds or switches to another window, onOutside is invoked.
func WatchClickOutside(windowTitle string, onOutside func()) func() {
	stop := make(chan struct{})

	go func() {
		ticker := time.NewTicker(35 * time.Millisecond)
		defer ticker.Stop()

		wasDown := false
		hadFocus := false

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				isDown := isKeyDown(vkLButton) || isKeyDown(vkRButton) || isKeyDown(vkMButton)

				hwnd := findWindowByTitle(windowTitle)
				if hwnd == 0 {
					wasDown = isDown
					hadFocus = false
					continue
				}

				vis, _, _ := procIsWindowVisible.Call(hwnd)
				if vis == 0 {
					wasDown = isDown
					hadFocus = false
					continue
				}

				// 1. Check mouse click outside window rectangle
				if isDown && !wasDown {
					var pt point
					var r rect
					ptOk, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
					rOk, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
					if ptOk != 0 && rOk != 0 {
						outside := pt.X < r.Left || pt.X > r.Right || pt.Y < r.Top || pt.Y > r.Bottom
						if outside {
							wasDown = isDown
							hadFocus = false
							onOutside()
							continue
						}
					}
				}
				wasDown = isDown

				// 2. Check foreground window focus loss (e.g. Alt+Tab or Win key)
				fgHwnd, _, _ := procGetForegroundWindow.Call()
				if fgHwnd == hwnd {
					hadFocus = true
				} else if hadFocus && fgHwnd != 0 && fgHwnd != hwnd {
					hadFocus = false
					onOutside()
				}
			}
		}
	}()

	return func() { close(stop) }
}
