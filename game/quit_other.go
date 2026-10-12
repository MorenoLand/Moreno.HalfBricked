//go:build !(js && wasm)

package game

import "github.com/hajimehoshi/ebiten/v2"

// quitGame is "quit to desktop": end the game loop.
func quitGame() error { return ebiten.Termination }
