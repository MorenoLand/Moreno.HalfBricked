//go:build js && wasm

package engine

import (
	"strings"
	"syscall/js"
)

func IsMobileDevice() bool {
	navigator := js.Global().Get("navigator")
	if !navigator.Truthy() {
		return false
	}
	userAgent := strings.ToLower(navigator.Get("userAgent").String())
	for _, token := range []string{"android", "iphone", "ipad", "ipod", "mobile", "tablet"} {
		if strings.Contains(userAgent, token) {
			return true
		}
	}
	return false
}
