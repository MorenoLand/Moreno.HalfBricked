//go:build js && wasm

package platform

// ShowError does nothing in the browser (the console shows the log).
func ShowError(title, text string) {}

// LogFilePath is empty in the browser.
func LogFilePath() string { return "" }

// HasConsole is true in the browser (the developer console).
func HasConsole() bool { return true }

// ShowNotice does nothing in the browser (the page shows its own progress).
func ShowNotice(title, text string) (closeNotice func()) { return func() {} }
