//go:build js && wasm

package content

import (
	"fmt"
	"os"
	"strings"
	"syscall/js"
)

// DefaultCacheDir in the browser is the in-memory cache folder.
func DefaultCacheDir() string { return webCacheRoot }

// webCacheRoot is where the browser build keeps the cache it builds from the player's own APK (web/memfs.js).
const webCacheRoot = "/cache"

// PrepareAssets in the browser. Hosted mode passes nothing and reads the exported data over HTTP. "Bring your own
// APK" mode passes the APK the page put in the in-memory file system (e.g. /game.apk): it is unpacked and converted
// here, with progress shown on the page, and the page is told when the cache is ready so it can keep it between
// visits. A directory in the in-memory file system that already holds a cache (restored by the page) is used as is.
func PrepareAssets(root string) (string, error) {
	if root == "" {
		return "data", nil
	}
	if strings.EqualFold(root[max(0, len(root)-4):], ".apk") {
		if _, err := os.Stat(root); err != nil {
			return "", fmt.Errorf("the APK was not handed to the game: %w", err)
		}
		if err := ValidateAPK(root); err != nil {
			return "", err
		}
		ImportProgress = webImportProgress
		if _, err := PrepareAPK(root, webCacheRoot); err != nil {
			return "", err
		}
		_ = os.Remove(root) // free the memory the package held
		if done := js.Global().Get("aozImportDone"); done.Type() == js.TypeFunction {
			done.Invoke()
		}
		return webCacheRoot, nil
	}
	return root, nil
}

// webImportProgress shows the import on the page and gives the browser a turn every few files, so the page keeps
// drawing while the (single-threaded) import runs.
func webImportProgress(stage string, done, total int) {
	if done%12 != 0 {
		return
	}
	if status := js.Global().Get("aozStatus"); status.Type() == js.TypeFunction {
		status.Invoke(stage, done, total)
	}
	turn := make(chan struct{})
	callback := js.FuncOf(func(this js.Value, args []js.Value) any {
		close(turn)
		return nil
	})
	js.Global().Call("setTimeout", callback, 0)
	<-turn
	callback.Release()
}
