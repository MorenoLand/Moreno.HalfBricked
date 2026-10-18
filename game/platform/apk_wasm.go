//go:build js && wasm

package platform

import "syscall/js"

// The browser build gets its package from the page (web/index.html), never from a remembered desktop path.
func RememberedAPK() string    { return "" }
func RememberAPK(string) error { return nil }
func PickAPK() (string, error) { return "", nil }

// NotifyReady tells the page the game is about to draw, so it can hide its loading panel.
func NotifyReady() {
	defer func() { _ = recover() }()
	if ready := js.Global().Get("aozReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
}
