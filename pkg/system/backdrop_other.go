//go:build !windows

package system

type BackdropCapturer struct{}

func NewBackdropCapturer() *BackdropCapturer {
	return &BackdropCapturer{}
}

func (bc *BackdropCapturer) CaptureBehind(windowTitle string, force bool) (string, bool) {
	return "", false
}
