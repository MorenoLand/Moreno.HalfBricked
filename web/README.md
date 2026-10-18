# Web host files

Run `scripts/build.ps1 -Target wasm` (or `scripts/build.sh wasm`) from the project root. There are two kinds of bundle:

| Mode | Command | Output | For |
| --- | --- | --- | --- |
| hosted (default) | `build.ps1 -Target wasm` | `bin/web`: `index.html`, `wasm_exec.js`, `aoz.wasm` and the game's extracted `data/` | Hosting it yourself (your iPad, a server): visitors just open the page. |
| bring your own APK | `build.ps1 -Target wasm -WebMode apk` (`build.sh wasm apk`) | `bin/web-apk`: the same page and `aoz.wasm`, plus `memfs.js` and `config.json`, and **no game data** | Sharing the site without the game's files: each visitor picks their own Age of Zombies APK. |

Serve the directory through an HTTP server (`https` or `localhost`: browsers restrict WebAssembly, IndexedDB and the clipboard elsewhere). Opening `index.html` from a file URL is not supported.

## Bring-your-own-APK mode

`config.json` (`{"mode":"apk","cacheVersion":"..."}`) makes the page ask for the APK instead of fetching `data/`. The page:

1. asks for the APK (1.2.5 or 1.2.1; file picker or drop) and keeps a copy in the browser's IndexedDB;
2. hands it to the game, which unpacks it and converts the assets with the same importer the desktop build uses, inside an in-memory file system (`memfs.js` implements the `fs` calls Go's WASM runtime makes), showing progress;
3. keeps the converted game files in IndexedDB, so the next visit starts straight away;
4. rebuilds them from the stored APK when `cacheVersion` (the hash of `aoz.wasm`) changes, so an update never needs the APK again.

Nothing is uploaded: the APK and the converted files stay in the visitor's browser. The small "Change APK" button at the bottom-left of the loading screen forgets both.

The import takes about 15 to 50 seconds (the browser runs it on one thread) and happens once per browser and per update.

## Checking the import without a browser

`tools/webimport` is a headless check of the browser import: build it with `GOOS=js GOARCH=wasm go build -o importtest.wasm ./tools/webimport`, then run it under Node with `memfs.js` loaded before `wasm_exec.js`, the APK written to `/game.apk` through `aozFS.writeFile`, and `process.cwd` returning `/`. It prints the file counts and opens the result as a content pack.
