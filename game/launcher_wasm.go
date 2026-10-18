//go:build js && wasm

package game

import "github.com/hajimehoshi/ebiten/v2"

// The browser has no launcher: the page prepares the files and starts the game (web/index.html).
type launchOptions struct {
	assets                string
	explicit, chooseAPK   bool
	debug, mobile, silent bool
}

func newLauncher(options launchOptions, setup func(*app) error) (ebiten.Game, func()) { return nil, nil }

// switchAPK does nothing in the browser (the page has its own "Change APK" button).
func switchAPK() error { return nil }
