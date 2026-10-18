//go:build !windows && !js

package platform

import (
	"fmt"
	"os"
)

// ShowError prints the problem (other systems run from a terminal).
func ShowError(title, text string) { fmt.Fprintln(os.Stderr, title+": "+text) }

// LogFilePath is empty: other systems log to the terminal.
func LogFilePath() string { return "" }

// HasConsole is true: other systems log to the terminal.
func HasConsole() bool { return true }

// ShowNotice prints the notice (other systems run from a terminal) and returns a no-op closer.
func ShowNotice(title, text string) (closeNotice func()) {
	fmt.Fprintln(os.Stderr, text)
	return func() {}
}
