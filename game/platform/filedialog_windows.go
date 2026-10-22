//go:build windows

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const (
	ofnOverwritePrompt = 0x00000002
	ofnNoChangeDir     = 0x00000008
)

// fileDialog shows the system open or save dialog. filter is "Description\x00*.ext\x00" pairs (no final double NUL).
func fileDialog(save bool, title, filter, defaultName, defaultExt string) (string, error) {
	buffer := make([]uint16, 32768)
	if defaultName != "" {
		copy(buffer, utf16.Encode([]rune(defaultName)))
	}
	encoded := utf16.Encode([]rune(filter + "All files (*.*)\x00*.*\x00\x00"))
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	ofn := openFileName{
		filter:      &encoded[0],
		file:        &buffer[0],
		maxFile:     uint32(len(buffer)),
		title:       titlePtr,
		flags:       ofnPathMustExist | ofnExplorer | ofnNoChangeDir,
		filterIndex: 1,
	}
	if defaultExt != "" {
		ofn.defExt, _ = syscall.UTF16PtrFromString(defaultExt)
	}
	proc := "GetOpenFileNameW"
	if save {
		proc = "GetSaveFileNameW"
		ofn.flags |= ofnOverwritePrompt
	} else {
		ofn.flags |= ofnFileMustExist
	}
	ofn.structSize = uint32(unsafe.Sizeof(ofn))
	ret, _, _ := syscall.NewLazyDLL("comdlg32.dll").NewProc(proc).Call(uintptr(unsafe.Pointer(&ofn)))
	if ret == 0 {
		failure := syscall.NewLazyDLL("comdlg32.dll").NewProc("CommDlgExtendedError")
		if code, _, _ := failure.Call(); code != 0 {
			return "", fmt.Errorf("file dialog failed (code %d)", code)
		}
		return "", nil // cancelled
	}
	return syscall.UTF16ToString(buffer), nil
}

// OpenFileDialog asks for an existing file ("" when cancelled). desc and pattern are the filter, e.g. "Level (*.json)", "*.json".
func OpenFileDialog(title, desc, pattern string) (string, error) {
	return fileDialog(false, title, desc+"\x00"+pattern+"\x00", "", "")
}

// SaveFileAs asks where to save and writes data there. It returns the chosen path ("" when cancelled).
func SaveFileAs(title, defaultName, desc, pattern string, data []byte) (string, error) {
	path, err := fileDialog(true, title, desc+"\x00"+pattern+"\x00", defaultName, filepath.Ext(defaultName)[1:])
	if err != nil || path == "" {
		return "", err
	}
	return path, os.WriteFile(path, data, 0o644)
}
