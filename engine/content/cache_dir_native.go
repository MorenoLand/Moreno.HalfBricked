//go:build !js

package content

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultCacheDir is where the game's files are made from the APK: a "data" folder next to the program, so the program
// and its files move together. A program started with "go run" lives in a temporary directory that is gone afterwards
// (the files would be rebuilt on every run), so there, and when the program's own location is unknown, it is "data" in
// the working directory.
func DefaultCacheDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "data"
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)
	temp := filepath.Clean(os.TempDir())
	if strings.HasPrefix(strings.ToLower(dir), strings.ToLower(temp)+string(os.PathSeparator)) && strings.Contains(strings.ToLower(dir), "go-build") {
		return "data"
	}
	return filepath.Join(dir, "data")
}

// legacyCacheDir is where earlier versions (and the developer checkout) kept the files.
func legacyCacheDir() string { return filepath.Join("bin", "data-cache") }

// FindCache returns the first existing prepared cache: the default folder, then the legacy one. "" when there is none.
func FindCache() string {
	for _, dir := range []string{DefaultCacheDir(), legacyCacheDir()} {
		if _, err := os.Stat(filepath.Join(dir, "pack.json")); err == nil {
			return dir
		}
	}
	return ""
}
