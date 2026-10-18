//go:build !js

package platform

import "github.com/atotto/clipboard"

func ClipboardWrite(text string) { _ = clipboard.WriteAll(text) }

// ClipboardRequest asks for the clipboard text; the result is polled with
// ClipboardTake so the browser build can answer asynchronously.
func ClipboardRequest() {
	text, err := clipboard.ReadAll()
	if err == nil {
		clipboardResult.set(text)
	}
}

// ClipboardArm is the browser build's hook for reading inside a user event; the desktop build reads at once.
func ClipboardArm(overPasteButton, keys bool) {}

// ClipboardDenied is always false on the desktop.
func ClipboardDenied() bool { return false }
