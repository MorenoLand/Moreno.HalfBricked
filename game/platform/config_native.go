//go:build !js

package platform

// ConfigRead returns a small file of the game's settings (slash separated name, no leading slash).
func ConfigRead(name string) ([]byte, bool) { return nativeConfigRead(name) }

// ConfigWrite stores a small file; it reports whether it could.
func ConfigWrite(name string, data []byte) bool { return nativeConfigWrite(name, data) }

// ConfigDelete removes a stored file.
func ConfigDelete(name string) { nativeConfigDelete(name) }

// ConfigList lists the stored files under a folder name such as "stamps".
func ConfigList(prefix string) []string { return nativeConfigList(prefix) }
