//go:build !windows

package system

// WorkArea represents the usable desktop area excluding system panels.
type WorkArea struct {
	X      int
	Y      int
	Width  int
	Height int
	Bottom int
	Right  int
}

// GetPrimaryWorkArea returns the fallback work area on non-Windows platforms.
func GetPrimaryWorkArea() WorkArea {
	return WorkArea{
		X:      0,
		Y:      0,
		Width:  1920,
		Height: 1040,
		Bottom: 1040,
		Right:  1920,
	}
}
