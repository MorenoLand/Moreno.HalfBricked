//go:build !js

package content

// NewSource opens an asset source: on the desktop always a directory.
func NewSource(root string) AssetSource { return fileSource{root: root} }
