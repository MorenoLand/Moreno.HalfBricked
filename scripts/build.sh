#!/usr/bin/env sh
set -eu
project=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
target=${1:-linux}
mkdir -p "$project/bin"
case "$target" in
  windows) GOOS=windows GOARCH=amd64 go build -o "$project/bin/aoz.exe" "$project/frontend" ;;
  linux) GOOS=linux GOARCH=amd64 go build -o "$project/bin/aoz-linux" "$project/frontend" ;;
  darwin) GOOS=darwin GOARCH=amd64 go build -o "$project/bin/aoz-darwin" "$project/frontend" ;;
  wasm) GOOS=js GOARCH=wasm go build -o "$project/bin/aoz.wasm" "$project/frontend" ;;
  *) echo "target must be windows, linux, darwin, or wasm" >&2; exit 2 ;;
esac
