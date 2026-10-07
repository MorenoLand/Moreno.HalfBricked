//go:build js && wasm

package engine

import "syscall/js"

func IsMobileDevice() bool {
	navigator := js.Global().Get("navigator")
	if !navigator.Truthy() {
		return false
	}
	touchPoints, mobile := 0, false
	if value := navigator.Get("maxTouchPoints"); value.Type() == js.TypeNumber {
		touchPoints = value.Int()
	}
	if data := navigator.Get("userAgentData"); data.Truthy() {
		if value := data.Get("mobile"); value.Type() == js.TypeBoolean {
			mobile = value.Bool()
		}
	}
	return mobileBrowser(navigator.Get("userAgent").String(), navigator.Get("platform").String(), touchPoints, mobile)
}
