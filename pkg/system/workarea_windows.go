//go:build windows

package system

import (
	"syscall"
	"unsafe"
)

var (
	user32                   = syscall.NewLazyDLL("user32.dll")
	procSystemParametersInfo = user32.NewProc("SystemParametersInfoW")
)

type rect struct {
	Left, Top, Right, Bottom int32
}

const spiGetWorkArea = 0x0030

// WorkArea represents the usable desktop area excluding the taskbar.
type WorkArea struct {
	X      int
	Y      int
	Width  int
	Height int
	Bottom int
	Right  int
}

// GetPrimaryWorkArea returns the work area of the primary display (excluding the taskbar).
func GetPrimaryWorkArea() WorkArea {
	var r rect
	ret, _, _ := procSystemParametersInfo.Call(
		uintptr(spiGetWorkArea),
		0,
		uintptr(unsafe.Pointer(&r)),
		0,
	)

	if ret == 0 {
		// Fallback
		return WorkArea{
			X:      0,
			Y:      0,
			Width:  1920,
			Height: 1040,
			Bottom: 1040,
			Right:  1920,
		}
	}

	return WorkArea{
		X:      int(r.Left),
		Y:      int(r.Top),
		Width:  int(r.Right - r.Left),
		Height: int(r.Bottom - r.Top),
		Bottom: int(r.Bottom),
		Right:  int(r.Right),
	}
}
