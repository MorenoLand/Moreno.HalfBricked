// Package platform holds the thin glue between the game and the machine it runs on: the system clipboard, the
// profile storage backend (bin/profile.json natively, localStorage in the browser), "quit to desktop" and the
// browser-only debugging switches (?perf, ?noaudio). Every file has a native twin and a js/wasm twin selected by
// build tags (*_native.go / *_other.go and *_wasm.go) that export the same identifiers, so callers never need a
// build tag themselves. It imports nothing from the game package.
package platform

// PerfOverlay is the frame-time readout of the browser build: open the page with ?perf (or set it from the
// console) to show the update and draw CPU times and the frame rates in the top-left corner. It is for finding
// where a slow machine loses its time; it is off otherwise and costs nothing. Only the wasm build sets it.
var PerfOverlay bool

// AudioDisabled (?noaudio in the browser) starts the game with no sound system at all, to tell whether audio
// decoding is what holds the frame rate down. Only the wasm build sets it.
var AudioDisabled bool
