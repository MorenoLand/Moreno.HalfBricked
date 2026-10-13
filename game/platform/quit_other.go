//go:build !(js && wasm)

package platform

import "github.com/hajimehoshi/ebiten/v2"

// QuitGame is "quit to desktop": end the game loop.
func QuitGame() error { return ebiten.Termination }
