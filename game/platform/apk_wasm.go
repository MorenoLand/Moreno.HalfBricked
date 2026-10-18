//go:build js && wasm

package platform

// The browser build gets its package from the page (web/index.html), never from a remembered desktop path.
func RememberedAPK() string    { return "" }
func RememberAPK(string) error { return nil }
func PickAPK() (string, error) { return "", nil }
