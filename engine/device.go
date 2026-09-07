//go:build !js || !wasm

package engine

func IsMobileDevice() bool { return false }
