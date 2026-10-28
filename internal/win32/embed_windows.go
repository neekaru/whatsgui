package win32

import (
	"fmt"
	"syscall"
)

var user32 = syscall.NewLazyDLL("user32.dll")
var setParent = user32.NewProc("SetParent")
var setWindowLong = user32.NewProc("SetWindowLongPtrW")
var getWindowLong = user32.NewProc("GetWindowLongPtrW")
var setWindowPos = user32.NewProc("SetWindowPos")
var showWindow = user32.NewProc("ShowWindow")
var clearLastError = syscall.NewLazyDLL("kernel32.dll").NewProc("SetLastError")

func Attach(child, parent uintptr) error {
	if child == 0 || parent == 0 {
		return fmt.Errorf("handle jendela tidak valid")
	}
	const styleIndex = ^uintptr(15) // GWL_STYLE = -16
	style, _, _ := getWindowLong.Call(child, styleIndex)
	style = (style &^ uintptr(0x80000000|0x00C00000|0x00040000|0x00080000|0x00020000|0x00010000)) | 0x40000000 | 0x04000000
	clearLastError.Call(0)
	if result, _, err := setWindowLong.Call(child, styleIndex, style); result == 0 && err != syscall.Errno(0) {
		return fmt.Errorf("mengubah gaya jendela: %w", err)
	}
	clearLastError.Call(0)
	if result, _, err := setParent.Call(child, parent); result == 0 && err != syscall.Errno(0) {
		return fmt.Errorf("menanam jendela profil: %w", err)
	}
	return nil
}

func Place(hwnd uintptr, x, y, width, height int, active bool) {
	if !active || width < 1 || height < 1 {
		showWindow.Call(hwnd, 0)
		return
	}
	// HWND_TOP places the profile over the shell's full-window WebView2, only in its viewport.
	setWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0x0010|0x0020|0x0040)
}
