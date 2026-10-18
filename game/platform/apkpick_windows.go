//go:build windows

package platform

import (
	"fmt"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

// openFileName is OPENFILENAMEW (commdlg.h).
type openFileName struct {
	structSize      uint32
	owner           uintptr
	instance        uintptr
	filter          *uint16
	customFilter    *uint16
	maxCustomFilter uint32
	filterIndex     uint32
	file            *uint16
	maxFile         uint32
	fileTitle       *uint16
	maxFileTitle    uint32
	initialDir      *uint16
	title           *uint16
	flags           uint32
	fileOffset      uint16
	fileExtension   uint16
	defExt          *uint16
	custData        uintptr
	hook            uintptr
	templateName    *uint16
	reserved        uintptr
	reserved2       uint32
	flagsEx         uint32
}

const (
	ofnFileMustExist = 0x00001000
	ofnPathMustExist = 0x00000800
	ofnExplorer      = 0x00080000
)

// PickAPK shows the system "open file" dialog for an .apk. It returns "" when the player cancels.
func PickAPK() (string, error) {
	buffer := make([]uint16, 32768)
	// The filter is a list of NUL-separated pairs ending in a double NUL, which UTF16FromString refuses to encode.
	filter := utf16.Encode([]rune("Android package (*.apk)\x00*.apk\x00All files (*.*)\x00*.*\x00\x00"))
	title, _ := syscall.UTF16PtrFromString("Select the Age of Zombies APK")
	ofn := openFileName{
		filter:      &filter[0],
		file:        &buffer[0],
		maxFile:     uint32(len(buffer)),
		title:       title,
		flags:       ofnFileMustExist | ofnPathMustExist | ofnExplorer,
		filterIndex: 1,
	}
	ofn.structSize = uint32(unsafe.Sizeof(ofn))
	dialog := syscall.NewLazyDLL("comdlg32.dll").NewProc("GetOpenFileNameW")
	ret, _, _ := dialog.Call(uintptr(unsafe.Pointer(&ofn)))
	if ret == 0 {
		failure := syscall.NewLazyDLL("comdlg32.dll").NewProc("CommDlgExtendedError")
		if code, _, _ := failure.Call(); code != 0 {
			return "", fmt.Errorf("file dialog failed (code %d)", code)
		}
		return "", nil // cancelled
	}
	return syscall.UTF16ToString(buffer), nil
}
