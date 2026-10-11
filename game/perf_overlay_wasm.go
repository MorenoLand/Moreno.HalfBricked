//go:build js && wasm

package game

import (
	"strings"
	"syscall/js"
)

func init() {
	defer func() { _ = recover() }()
	search := js.Global().Get("location").Get("search").String()
	perfOverlay = strings.Contains(search, "perf")
	audioDisabled = strings.Contains(search, "noaudio")
}
