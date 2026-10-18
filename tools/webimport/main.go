//go:build js && wasm

// webimport is a headless check of the browser import (run it under Node with web/memfs.js and wasm_exec.js, see
// web/README.md): it unpacks /game.apk, converts it into /cache and opens the result as a content pack, exactly as
// the game does in "bring your own APK" mode.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
)

func main() {
	start := time.Now()
	root, err := content.PrepareAssets("/game.apk")
	if err != nil {
		fmt.Println("IMPORT FAILED:", err)
		os.Exit(1)
	}
	fmt.Printf("imported in %.1fs into %s\n", time.Since(start).Seconds(), root)
	pack, err := content.NewPack(content.NewSource(root))
	if err != nil {
		fmt.Println("PACK FAILED:", err)
		os.Exit(1)
	}
	weapons, err := pack.Weapons()
	if err != nil {
		fmt.Println("WEAPONS FAILED:", err)
		os.Exit(1)
	}
	fmt.Printf("pack ok: %d levels, build %v, pistol present: %v\n", len(pack.List()), pack.Manifest().WaveBuild, func() bool { _, ok := weapons.Find("PISTOL"); return ok }())
}
