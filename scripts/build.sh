#!/usr/bin/env sh
set -eu
project=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
target=${1:-linux}
mkdir -p "$project/bin"
staged_resource="$project/icon_windows_amd64.syso"
cleanup() { rm -f "$staged_resource"; }
case "$target" in
  windows)
    if command -v windres >/dev/null 2>&1; then (cd "$project" && windres -i resources/icon.rc -o resources/icon_windows_amd64.syso); fi
    if [ ! -f "$project/resources/icon_windows_amd64.syso" ]; then echo "Windows icon resource not found" >&2; exit 1; fi
    cp "$project/resources/icon_windows_amd64.syso" "$staged_resource"
    trap cleanup EXIT
    GOOS=windows GOARCH=amd64 go build -o "$project/bin/aoz.exe" "$project" ;;
  linux) GOOS=linux GOARCH=amd64 go build -o "$project/bin/aoz-linux" "$project" ;;
  darwin) GOOS=darwin GOARCH=amd64 go build -o "$project/bin/aoz-darwin" "$project" ;;
  wasm) GOOS=js GOARCH=wasm go build -o "$project/bin/aoz.wasm" "$project" ;;
  *) echo "target must be windows, linux, darwin, or wasm" >&2; exit 2 ;;
esac
if [ "$target" = wasm ]; then
  mkdir -p "$project/bin/web"
  cp "$project/bin/aoz.wasm" "$project/bin/web/aoz.wasm"
  cp "$project/web/index.html" "$project/bin/web/index.html"
  if [ -f "$project/web/favicon.png" ]; then cp "$project/web/favicon.png" "$project/bin/web/favicon.png"; fi
  if [ -f "$project/web/favicon.ico" ]; then cp "$project/web/favicon.ico" "$project/bin/web/favicon.ico"; fi
  wasm_exec="$(go env GOROOT)/lib/wasm/wasm_exec.js"
  if [ ! -f "$wasm_exec" ]; then wasm_exec="$(go env GOROOT)/misc/wasm/wasm_exec.js"; fi
  if [ ! -f "$wasm_exec" ]; then echo "wasm_exec.js not found below GoROOT" >&2; exit 1; fi
  cp "$wasm_exec" "$project/bin/web/wasm_exec.js"
  if [ -d "$project/bin/data-cache" ]; then rm -rf "$project/bin/web/data"; cp -R "$project/bin/data-cache" "$project/bin/web/data"; fi
fi
