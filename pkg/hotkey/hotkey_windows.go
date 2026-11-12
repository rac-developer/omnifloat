//go:build windows

package hotkey

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

const (
	whKeyboardLL = 13
	wmKeyDown    = 0x0100
	wmSysKeyDown = 0x0104
	vkSpace      = 0x20
	vkMenu       = 0x12 // Alt key
	llkhfAltDown = 0x20
	wmQuit       = 0x0012
)

type kbdllHookStruct struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type point struct {
	X, Y int32
}

type msg struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       point
	LPrivate uint32
}

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procPostThreadMessageW  = user32.NewProc("PostThreadMessageW")
	procGetAsyncKeyState    = user32.NewProc("GetAsyncKeyState")
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procGetCurrentThreadId  = kernel32.NewProc("GetCurrentThreadId")

	hookMu            sync.Mutex
	globalHookHandler func()
	hookHandle        uintptr
	hookThreadID      uintptr
	isHookActive      bool
)

func hookCallback(nCode int32, wParam uintptr, lParam unsafe.Pointer) uintptr {
	if nCode >= 0 && (wParam == wmKeyDown || wParam == wmSysKeyDown) {
		kbd := (*kbdllHookStruct)(lParam)
		if kbd.VkCode == vkSpace {
			altDown := (kbd.Flags&llkhfAltDown) != 0
			if !altDown {
				ret, _, _ := procGetAsyncKeyState.Call(uintptr(vkMenu))
				altDown = (int16(ret) < 0)
			}

			if altDown {
				hookMu.Lock()
				handler := globalHookHandler
				hookMu.Unlock()

				if handler != nil {
					go handler()
					// Return 1 to suppress the default Windows window menu
					return 1
				}
			}
		}
	}

	ret, _, _ := procCallNextHookEx.Call(hookHandle, uintptr(nCode), wParam, uintptr(lParam))
	return ret
}

// Manager manages registration and listening of the global hotkey (Alt+Space).
type Manager struct {
	onToggle func()
	mu       sync.Mutex
}

// NewManager creates a new hotkey manager configured for Alt+Space.
func NewManager(onToggle func()) *Manager {
	return &Manager{
		onToggle: onToggle,
	}
}

// Start registers the global Alt+Space hotkey using a Low-Level Keyboard Hook.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	hookMu.Lock()
	if isHookActive {
		hookMu.Unlock()
		return nil
	}
	globalHookHandler = m.onToggle
	hookMu.Unlock()

	ready := make(chan error, 1)

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		threadID, _, _ := procGetCurrentThreadId.Call()
		hookMu.Lock()
		hookThreadID = threadID
		hookMu.Unlock()

		cb := syscall.NewCallback(hookCallback)
		h, _, err := procSetWindowsHookExW.Call(
			uintptr(whKeyboardLL),
			cb,
			0,
			0,
		)

		if h == 0 {
			ready <- fmt.Errorf("failed to install WH_KEYBOARD_LL for Alt+Space: %w", err)
			return
		}

		hookMu.Lock()
		hookHandle = h
		isHookActive = true
		hookMu.Unlock()

		ready <- nil
		log.Println("[OmniFloat Hotkey] Global shortcut Alt+Space registered via Windows Low-Level Hook")

		var m msg
		for {
			ret, _, _ := procGetMessageW.Call(
				uintptr(unsafe.Pointer(&m)),
				0,
				0,
				0,
			)
			if int32(ret) <= 0 {
				break
			}
		}

		hookMu.Lock()
		if hookHandle != 0 {
			_, _, _ = procUnhookWindowsHookEx.Call(hookHandle)
			hookHandle = 0
		}
		isHookActive = false
		hookMu.Unlock()
	}()

	return <-ready
}

// Stop unregisters the hotkey and releases system resources.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	hookMu.Lock()
	defer hookMu.Unlock()

	if !isHookActive {
		return
	}

	if hookThreadID != 0 {
		_, _, _ = procPostThreadMessageW.Call(hookThreadID, wmQuit, 0, 0)
		hookThreadID = 0
	}
	isHookActive = false
	globalHookHandler = nil
	log.Println("[OmniFloat Hotkey] Global shortcut unregistered")
}
