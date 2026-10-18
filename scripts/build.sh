#!/usr/bin/env sh
set -eu
project=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
target=${1:-all}
# Second argument, for wasm: "hosted" (default) builds bin/web with the extracted game data, for hosting it yourself;
# "apk" builds bin/web-apk with no game data, every visitor picks their own APK and the game builds itself from it.
webmode=${2:-hosted}
mkdir -p "$project/bin"
staged_resource="$project/icon_windows_amd64.syso"
cleanup() { rm -f "$staged_resource"; }
case "$target" in
  all) "$0" windows; "$0" wasm "$webmode" ;;
  windows)
    if command -v windres >/dev/null 2>&1; then (cd "$project" && windres -i resources/icon.rc -o resources/icon_windows_amd64.syso); fi
    if [ ! -f "$project/resources/icon_windows_amd64.syso" ]; then echo "Windows icon resource not found" >&2; exit 1; fi
    cp "$project/resources/icon_windows_amd64.syso" "$staged_resource"
    trap cleanup EXIT
    GOOS=windows GOARCH=amd64 go build -ldflags "-H=windowsgui" -o "$project/bin/aoz.exe" "$project" ;;
  linux) GOOS=linux GOARCH=amd64 go build -o "$project/bin/aoz-linux" "$project" ;;
  darwin) GOOS=darwin GOARCH=amd64 go build -o "$project/bin/aoz-darwin" "$project" ;;
  wasm) GOOS=js GOARCH=wasm go build -o "$project/bin/aoz.wasm" "$project" ;;
*) echo "target must be windows, linux, darwin, wasm, or all" >&2; exit 2 ;;
esac
if [ "$target" = wasm ]; then
  web="$project/bin/web"
  if [ "$webmode" = apk ]; then web="$project/bin/web-apk"; fi
  mkdir -p "$web"
  cp "$project/bin/aoz.wasm" "$web/aoz.wasm"
  cp "$project/web/index.html" "$web/index.html"
  if [ -f "$project/web/favicon.png" ]; then cp "$project/web/favicon.png" "$web/favicon.png"; fi
  if [ -f "$project/web/favicon.ico" ]; then cp "$project/web/favicon.ico" "$web/favicon.ico"; fi
  wasm_exec="$(go env GOROOT)/lib/wasm/wasm_exec.js"
  if [ ! -f "$wasm_exec" ]; then wasm_exec="$(go env GOROOT)/misc/wasm/wasm_exec.js"; fi
  if [ ! -f "$wasm_exec" ]; then echo "wasm_exec.js not found below GoROOT" >&2; exit 1; fi
  cp "$wasm_exec" "$web/wasm_exec.js"
  if [ "$webmode" = apk ]; then
    cp "$project/web/memfs.js" "$web/memfs.js"
    version=$(sha256sum "$project/bin/aoz.wasm" 2>/dev/null | cut -c1-16 || shasum -a 256 "$project/bin/aoz.wasm" | cut -c1-16)
    printf '{"mode":"apk","cacheVersion":"%s"}\n' "$version" > "$web/config.json"
    rm -rf "$web/data"
  else
    rm -f "$web/config.json"
    if [ -d "$project/bin/data-cache" ]; then rm -rf "$web/data"; cp -R "$project/bin/data-cache" "$web/data"; fi
  fi
fi
