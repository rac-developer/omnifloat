//go:build windows

package system

import (
	"bytes"
	"encoding/base64"
	"hash/fnv"
	"image"
	"image/jpeg"
	"syscall"
	"unsafe"
)

var (
	gdi32                  = syscall.NewLazyDLL("gdi32.dll")
	procGetDC              = user32.NewProc("GetDC")
	procReleaseDC          = user32.NewProc("ReleaseDC")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procStretchBlt         = gdi32.NewProc("StretchBlt")
	procSetStretchBltMode  = gdi32.NewProc("SetStretchBltMode")
)

const (
	biRGB               = 0
	dibRGBColors        = 0
	srccopy             = 0x00CC0020
	stretchColorOnColor = 3
)

type bitmapInfoHeader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type bitmapInfo struct {
	BmiHeader bitmapInfoHeader
	BmiColors [1]uint32
}

// BackdropCapturer captures the live desktop/window pixels directly behind a target window
// using WDA_EXCLUDEFROMCAPTURE so CSS backdrop-filter/blur always has real screen content to blur.
type BackdropCapturer struct {
	lastHash uint64
}

func NewBackdropCapturer() *BackdropCapturer {
	return &BackdropCapturer{}
}

// CaptureBehind captures a downscaled image of the screen rectangle behind windowTitle.
// It returns the base64 data URI and true if the screen content changed since the last capture.
func (bc *BackdropCapturer) CaptureBehind(windowTitle string, force bool) (string, bool) {
	hwnd := findWindowByTitle(windowTitle)
	if hwnd == 0 {
		return "", false
	}

	vis, _, _ := procIsWindowVisible.Call(hwnd)
	if vis == 0 {
		return "", false
	}

	// Ensure target window is excluded from screen capture so StretchBlt sees right through it
	_, _, _ = procSetWindowDisplayAffinity.Call(hwnd, uintptr(wdaExcludeFromCapture))

	var r rect
	ok, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	if ok == 0 {
		return "", false
	}

	srcW := int(r.Right - r.Left)
	srcH := int(r.Bottom - r.Top)
	if srcW <= 8 || srcH <= 8 {
		return "", false
	}

	// Downscale by 3x for Island or 4x for Launcher (fast & ideal for Gaussian blur)
	scale := 4
	if srcH < 120 {
		scale = 2
	}
	dstW := srcW / scale
	dstH := srcH / scale
	if dstW < 16 {
		dstW = 16
	}
	if dstH < 8 {
		dstH = 8
	}

	hdcScreen, _, _ := procGetDC.Call(0)
	if hdcScreen == 0 {
		return "", false
	}
	defer procReleaseDC.Call(0, hdcScreen)

	hdcMem, _, _ := procCreateCompatibleDC.Call(hdcScreen)
	if hdcMem == 0 {
		return "", false
	}
	defer procDeleteDC.Call(hdcMem)

	var bmi bitmapInfo
	bmi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bmi.BmiHeader))
	bmi.BmiHeader.BiWidth = int32(dstW)
	bmi.BmiHeader.BiHeight = -int32(dstH) // top-down DIB
	bmi.BmiHeader.BiPlanes = 1
	bmi.BmiHeader.BiBitCount = 32
	bmi.BmiHeader.BiCompression = biRGB

	var bitsPtr *byte
	hBitmap, _, _ := procCreateDIBSection.Call(
		hdcMem,
		uintptr(unsafe.Pointer(&bmi)),
		dibRGBColors,
		uintptr(unsafe.Pointer(&bitsPtr)),
		0,
		0,
	)
	if hBitmap == 0 || bitsPtr == nil {
		return "", false
	}
	defer procDeleteObject.Call(hBitmap)

	oldObj, _, _ := procSelectObject.Call(hdcMem, hBitmap)
	defer procSelectObject.Call(hdcMem, oldObj)

	_, _, _ = procSetStretchBltMode.Call(hdcMem, stretchColorOnColor)
	bltOk, _, _ := procStretchBlt.Call(
		hdcMem,
		0, 0, uintptr(dstW), uintptr(dstH),
		hdcScreen,
		uintptr(r.Left), uintptr(r.Top), uintptr(srcW), uintptr(srcH),
		srccopy,
	)
	if bltOk == 0 {
		return "", false
	}

	byteLen := dstW * dstH * 4
	rawBytes := unsafe.Slice(bitsPtr, byteLen)

	// Compute fast FNV-1a hash to skip encoding if nothing behind the window changed
	h := fnv.New64a()
	_, _ = h.Write(rawBytes)
	sum := h.Sum64()
	if !force && sum == bc.lastHash {
		return "", false
	}
	bc.lastHash = sum

	img := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	dstPix := img.Pix
	for i := 0; i < byteLen; i += 4 {
		dstPix[i] = rawBytes[i+2]   // R
		dstPix[i+1] = rawBytes[i+1] // G
		dstPix[i+2] = rawBytes[i]   // B
		dstPix[i+3] = 255           // A
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 78}); err != nil {
		return "", false
	}

	dataURI := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	return dataURI, true
}
