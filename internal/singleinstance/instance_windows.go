//go:build windows

package singleinstance

import (
	"strings"
	"syscall"
	"unsafe"
)

const (
	mutexName   = `Local\YTUI_SingleInstance`
	windowTitle = "YTUI - YouTube Downloader"
	dlgTitle    = "YTD-UI"
	dlgContent  = "YTD-UI telah dijalankan.\r\nProgram YTD-UI sedang berjalan."
	btnOKText   = "OK"
	btnFindText = "Masalah YTD-UI tidak muncul"
	className   = "YTUI_SingleInstance_Dialog"

	errAlreadyExists uintptr = 183

	swShow    uintptr = 5
	swRestore uintptr = 9

	vkMenu     = 0x12
	keyEventUp = 0x0002

	mbOK            = 0x00000000
	mbIconInform    = 0x00000040
	mbSetForeground = 0x00010000

	idOK      = 1
	idRestore = 2

	wmCommand = 0x0111
	wmClose   = 0x0010
	wmKeyDown = 0x0100
	wmSetFont = 0x0030

	wsPopup     = 0x80000000
	wsVisible   = 0x10000000
	wsChild     = 0x40000000
	wsCaption   = 0x00C00000
	wsSysMenu   = 0x00080000
	wsTabStop   = 0x00010000
	wsClipSibs  = 0x04000000
	wsExTopMost = 0x00000008

	bsPushButton    = 0x00000000
	bsDefPushButton = 0x00000001
	ssLeft          = 0x00000000

	colorBtnFace = 15 + 1

	idcArrow = 32512

	smCxScreen = 0
	smCyScreen = 1

	vkEscape = 0x1B
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")

	procCreateMutexW = kernel32.NewProc("CreateMutexW")
	procCloseHandle  = kernel32.NewProc("CloseHandle")
	procGetModuleW   = kernel32.NewProc("GetModuleHandleW")

	procFindWindowW         = user32.NewProc("FindWindowW")
	procEnumWindows         = user32.NewProc("EnumWindows")
	procGetWindowTextW      = user32.NewProc("GetWindowTextW")
	procIsIconic            = user32.NewProc("IsIconic")
	procShowWindow          = user32.NewProc("ShowWindow")
	procBringWindowToTop    = user32.NewProc("BringWindowToTop")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procKeybdEvent          = user32.NewProc("keybd_event")

	procRegisterClassW   = user32.NewProc("RegisterClassW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procGetStockObject   = user32.NewProc("GetStockObject")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	procSendMessageW     = user32.NewProc("SendMessageW")
	procLoadCursorW      = user32.NewProc("LoadCursorW")
	procMessageBoxW      = user32.NewProc("MessageBoxW")
)

var mutexHandle uintptr

// wndClassEx mirrors WNDCLASSEXW (64-bit alignment).
type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  uintptr
	lpszClassName uintptr
	hIconSm       uintptr
}

// msg mirrors MSG (64-bit layout).
type msg struct {
	hwnd    uintptr
	message uint32
	_       uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	_       uint32
	ptX     int32
	ptY     int32
}

var wndProcPtr = syscall.NewCallback(wndProc)

// Acquire returns true if another instance already owns the named mutex.
// The OS releases the mutex automatically when the owning process exits,
// including crashes, so it never stays stale.
func Acquire() bool {
	name, err := syscall.UTF16PtrFromString(mutexName)
	if err != nil {
		return false
	}

	h, _, lastErr := procCreateMutexW.Call(0, 1, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return false
	}
	if lastErr == syscall.Errno(errAlreadyExists) {
		return true
	}
	mutexHandle = h
	return false
}

// Release closes the mutex handle.
func Release() {
	if mutexHandle != 0 {
		procCloseHandle.Call(mutexHandle)
		mutexHandle = 0
	}
}

// ShowDialog shows the second-instance warning dialog with two buttons:
//   - "OK"                              -> close only, never creates a new instance
//   - "Masalah YTD-UI tidak muncul"     -> restore/focus the running instance
func ShowDialog() {
	hwnd := createDialogWindow()
	if hwnd == 0 {
		fallbackMessageBox()
		return
	}
	defer procDestroyWindow.Call(hwnd)

	// Process a modal message loop until the dialog closes itself.
	var m msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if r == 0 { // WM_QUIT
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// createDialogWindow builds the popup dialog window (in pixels).
func createDialogWindow() uintptr {
	classNamePtr, _ := syscall.UTF16PtrFromString(className)

	// Register a minimal window class (registration is idempotent).
	hInstance, _, _ := procGetModuleW.Call(0)
	arrowPtr, _, _ := procLoadCursorW.Call(0, idcArrow)
	wc := wndClassEx{
		lpfnWndProc:   wndProcPtr,
		hInstance:     hInstance,
		hbrBackground: colorBtnFace,
		hCursor:       arrowPtr,
		lpszClassName: uintptr(unsafe.Pointer(classNamePtr)),
	}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	r, _, errProc := procRegisterClassW.Call(uintptr(unsafe.Pointer(&wc)))
	if r == 0 && errProc != syscall.Errno(1410) { // ERROR_CLASS_ALREADY_EXISTS
		return 0
	}

	// Center the window on the primary screen.
	sw, _, _ := procGetSystemMetrics.Call(smCxScreen)
	sh, _, _ := procGetSystemMetrics.Call(smCyScreen)
	width := uintptr(480)
	height := uintptr(150)
	x := (sw - width) / 2
	y := (sh - height) / 3

	titlePtr, _ := syscall.UTF16PtrFromString(dlgTitle)
	hwnd, _, _ := procCreateWindowExW.Call(
		wsExTopMost,
		uintptr(unsafe.Pointer(classNamePtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		wsPopup|wsCaption|wsSysMenu|wsVisible|wsClipSibs,
		x, y, width, height,
		0, // parent
		0, // menu
		hInstance,
		0, // lpParam
	)
	if hwnd == 0 {
		return 0
	}

	// Default GUI font for a native dialog look.
	font, _, _ := procGetStockObject.Call(17) // DEFAULT_GUI_FONT

	// Message text (static label).
	contentPtr, _ := syscall.UTF16PtrFromString(dlgContent)
	staticCls, _ := syscall.UTF16PtrFromString("Static")
	staticHwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(staticCls)),
		uintptr(unsafe.Pointer(contentPtr)),
		wsChild|wsVisible|ssLeft,
		16, 14, width-32, 40,
		hwnd, 0xFFFF, 0, 0,
	)
	if staticHwnd != 0 {
		procSendMessageW.Call(staticHwnd, wmSetFont, font, 1)
	}

	// "Masalah YTD-UI tidak muncul" button.
	btnFindTextPtr, _ := syscall.UTF16PtrFromString(btnFindText)
	buttonCls, _ := syscall.UTF16PtrFromString("Button")
	btnFind, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(buttonCls)),
		uintptr(unsafe.Pointer(btnFindTextPtr)),
		wsChild|wsVisible|wsTabStop|bsPushButton,
		16, 74, 260, 26,
		hwnd, idRestore, 0, 0,
	)
	if btnFind != 0 {
		procSendMessageW.Call(btnFind, wmSetFont, font, 1)
	}

	// "OK" button.
	okTextPtr, _ := syscall.UTF16PtrFromString(btnOKText)
	btnOK, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(buttonCls)),
		uintptr(unsafe.Pointer(okTextPtr)),
		wsChild|wsVisible|wsTabStop|bsPushButton|bsDefPushButton,
		384, 74, 80, 26,
		hwnd, idOK, 0, 0,
	)
	if btnOK != 0 {
		procSendMessageW.Call(btnOK, wmSetFont, font, 1)
	}

	procSendMessageW.Call(hwnd, wmSetFont, font, 1)
	procSetForegroundWindow.Call(hwnd)
	return hwnd
}

// wndProc is the window procedure of the single-instance dialog.
func wndProc(hwnd uintptr, uMsg uint32, wParam, lParam uintptr) uintptr {
	switch uMsg {
	case wmCommand:
		switch wParam & 0xFFFF {
		case idRestore:
			FindAndRestore()
			procPostQuitMessage.Call(0)
			return 0
		case idOK:
			procPostQuitMessage.Call(0)
			return 0
		}
	case wmKeyDown:
		if wParam == vkEscape {
			procPostQuitMessage.Call(0)
			return 0
		}
	case wmClose:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(uMsg), wParam, lParam)
	return r
}

// fallbackMessageBox is used only if window creation fails.
func fallbackMessageBox() {
	title, _ := syscall.UTF16PtrFromString(dlgTitle)
	content, _ := syscall.UTF16PtrFromString(dlgContent)
	procMessageBoxW.Call(0,
		uintptr(unsafe.Pointer(content)),
		uintptr(unsafe.Pointer(title)),
		mbOK|mbIconInform|mbSetForeground)
}

// FindAndRestore locates the first YTUI window, restores it if minimized,
// then brings it to the foreground and gives it focus. It never creates a
// new instance and silently does nothing when the window is not found.
func FindAndRestore() {
	title, _ := syscall.UTF16PtrFromString(windowTitle)
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(title)))
	if hwnd == 0 {
		hwnd = findWindowByTitleContains("YTUI")
	}
	if hwnd == 0 {
		return
	}

	if r, _, _ := procIsIconic.Call(hwnd); r != 0 {
		procShowWindow.Call(hwnd, swRestore)
	}
	procShowWindow.Call(hwnd, swShow)
	procBringWindowToTop.Call(hwnd)

	// Bypass the Windows foreground-activation lock so a process that is
	// not in the foreground can still activate the target window.
	procKeybdEvent.Call(vkMenu, 0, 0, 0)
	procKeybdEvent.Call(vkMenu, 0, keyEventUp, 0)
	procSetForegroundWindow.Call(hwnd)
}

// --- Window search fallback --------------------------

func findWindowByTitleContains(substr string) uintptr {
	var found uintptr
	want := strings.ToLower(substr)
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		var buf [512]uint16
		n, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 512)
		if n > 0 {
			title := strings.ToLower(syscall.UTF16ToString(buf[:n]))
			if strings.Contains(title, want) {
				found = hwnd
				return 0
			}
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
	return found
}
