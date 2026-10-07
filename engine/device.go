//go:build !js || !wasm

package engine

import "runtime"

func IsMobileDevice() bool { return runtime.GOOS == "android" || runtime.GOOS == "ios" }
