package main

import (
	_ "embed"

	"github.com/MorenoLand/Moreno.HalfBricked/game"
)

//go:embed resources/icon_16.png
var icon16 []byte

//go:embed resources/icon_32.png
var icon32 []byte

//go:embed resources/icon_48.png
var icon48 []byte

//go:embed resources/icon_64.png
var icon64 []byte

//go:embed resources/icon_128.png
var icon128 []byte

//go:embed resources/icon_256.png
var icon256 []byte

func main() {
	game.IconPNGs = [][]byte{icon16, icon32, icon48, icon64, icon128, icon256}
	game.Run()
}
