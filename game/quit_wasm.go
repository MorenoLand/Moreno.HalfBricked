//go:build js && wasm

package game

import (
	"syscall/js"

	"github.com/hajimehoshi/ebiten/v2"
)

// quitGame is "quit to desktop". In a browser tab ending the game loop leaves a frozen page, so the tab is closed
// as well: window.close() works for tabs the page opened itself; a tab the user opened refuses it, and then the
// page is replaced by a blank one so the game is really gone.
func quitGame() error {
	defer func() { _ = recover() }()
	window := js.Global()
	window.Call("close")
	if location := window.Get("location"); location.Truthy() {
		location.Call("replace", "about:blank")
	}
	return ebiten.Termination
}
