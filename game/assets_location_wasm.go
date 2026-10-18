//go:build js && wasm

package game

// resolveAssets in the browser: the page decides (hosted data or the player's own APK, web/index.html).
func resolveAssets(value string, explicit, choose, capture bool) (string, error) { return value, nil }
