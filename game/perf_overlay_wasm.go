//go:build js && wasm

package game

import (
	"strings"
	"syscall/js"
)

func init() {
	defer func() { _ = recover() }()
	perfOverlay = strings.Contains(js.Global().Get("location").Get("search").String(), "perf")
}
