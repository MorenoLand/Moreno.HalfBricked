//go:build js && wasm

package platform

import (
	"strings"
	"syscall/js"
)

func init() {
	defer func() { _ = recover() }()
	search := js.Global().Get("location").Get("search").String()
	PerfOverlay = strings.Contains(search, "perf")
	AudioDisabled = strings.Contains(search, "noaudio")
}
