package win32

import (
	"fmt"
	"syscall"
	"unsafe"
)

var user32 = syscall.NewLazyDLL("user32.dll")
var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var setParent = user32.NewProc("SetParent")
var setWindowLong = user32.NewProc("SetWindowLongPtrW")
var getWindowLong = user32.NewProc("GetWindowLongPtrW")
var setWindowPos = user32.NewProc("SetWindowPos")
var showWindowAsync = user32.NewProc("ShowWindowAsync")
var msgWaitForMultipleObjectsEx = user32.NewProc("MsgWaitForMultipleObjectsEx")
var peekMessage = user32.NewProc("PeekMessageW")
var translateMessage = user32.NewProc("TranslateMessage")
var dispatchMessage = user32.NewProc("DispatchMessageW")
var postQuitMessage = user32.NewProc("PostQuitMessage")
var createEvent = kernel32.NewProc("CreateEventW")
var setEvent = kernel32.NewProc("SetEvent")
var closeHandle = kernel32.NewProc("CloseHandle")
var clearLastError = kernel32.NewProc("SetLastError")

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
		showWindowAsync.Call(hwnd, 0)
		return
	}
	// HWND_TOP places the profile over the shell's full-window WebView2, only in its viewport.
	// SWP_ASYNCWINDOWPOS avoids blocking the workspace UI thread if the child thread is waiting.
	setWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0x0010|0x0020|0x0040|0x4000)
}

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

// WaitWithMessagePump waits until done is closed while pumping Win32 messages
// on the calling UI thread, preventing cross-process parent/child HWND stalls.
func WaitWithMessagePump(done <-chan struct{}) {
	event, _, _ := createEvent.Call(0, 1, 0, 0)
	if event == 0 {
		<-done
		return
	}
	defer closeHandle.Call(event)
	go func() {
		<-done
		setEvent.Call(event)
	}()
	const (
		waitObject0    = 0
		qsAllInput     = 0x04FF
		mwmoInputAvail = 0x0004
		pmRemove       = 0x0001
		wmQuit         = 0x0012
	)
	var msg winMsg
	for {
		r, _, _ := msgWaitForMultipleObjectsEx.Call(1, uintptr(unsafe.Pointer(&event)), 0xFFFFFFFF, qsAllInput, mwmoInputAvail)
		if r == waitObject0 {
			return
		}
		if r != waitObject0+1 {
			return
		}
		for {
			has, _, _ := peekMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, pmRemove)
			if has == 0 {
				break
			}
			if msg.message == wmQuit {
				postQuitMessage.Call(msg.wParam)
				return
			}
			translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}
}
