//go:build windows

package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

// The Windows build is a GUI program (no console window), so errors that would have gone to the console are shown in a
// message box and the log is kept in a file.

// ShowError tells the player something went wrong, in a message box.
func ShowError(title, text string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	box := user32.NewProc("MessageBoxW")
	t, _ := syscall.UTF16PtrFromString(text)
	c, _ := syscall.UTF16PtrFromString(title)
	const mbOK, mbIconError = 0x0, 0x10
	box.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), mbOK|mbIconError)
}

// LogFilePath is where the log goes when the program has no console.
func LogFilePath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "HalfBricked", "halfbricked.log")
}

// HasConsole reports whether the process was started from a console (go run, a terminal).
func HasConsole() bool {
	window, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow").Call()
	return window != 0
}

// ShowNotice shows a message box without waiting for it and returns the function that closes it again. It is used for
// "this takes a moment" while the program works.
func ShowNotice(title, text string) (closeNotice func()) {
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		t, _ := syscall.UTF16PtrFromString(text)
		c, _ := syscall.UTF16PtrFromString(title)
		const mbOK, mbIconInformation = 0x0, 0x40
		syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), mbOK|mbIconInformation)
	}()
	return func() {
		user32 := syscall.NewLazyDLL("user32.dll")
		c, _ := syscall.UTF16PtrFromString(title)
		window, _, _ := user32.NewProc("FindWindowW").Call(0, uintptr(unsafe.Pointer(c)))
		if window != 0 {
			const wmClose = 0x0010
			user32.NewProc("PostMessageW").Call(window, wmClose, 0, 0)
		}
	}
}
