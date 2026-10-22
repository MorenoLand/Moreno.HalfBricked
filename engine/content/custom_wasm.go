//go:build js

package content

// CustomDir is empty in the browser: there is no folder to keep custom levels in (the editor registers them for the visit).
func CustomDir() string { return "" }
