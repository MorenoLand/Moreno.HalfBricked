//go:build !js

package game

import "github.com/atotto/clipboard"

func clipboardWrite(text string) { _ = clipboard.WriteAll(text) }

// clipboardRequest asks for the clipboard text; the result is polled with
// clipboardTake so the browser build can answer asynchronously.
func clipboardRequest() {
	text, err := clipboard.ReadAll()
	if err == nil {
		clipboardResult.set(text)
	}
}
