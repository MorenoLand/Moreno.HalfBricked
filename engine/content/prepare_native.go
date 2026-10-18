//go:build !js

package content

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func PrepareAssets(root string) (string, error) {
	if strings.EqualFold(filepath.Ext(root), ".apk") {
		return prepareAPK(root)
	}
	if root == "" {
		// No location given: the cache made earlier (next to the program, or the developer checkout's).
		if cache := FindCache(); cache != "" {
			return cache, nil
		}
		return "", fmt.Errorf("the game's files have not been prepared yet: start the program and choose your Age of Zombies APK (or pass --assets <apk>)")
	}
	if info, err := os.Stat(root); err == nil && !info.IsDir() {
		return "", fmt.Errorf("assets path %q is not a directory or APK", root)
	}
	if _, err := os.Stat(filepath.Join(root, "pack.json")); err == nil {
		return root, nil
	}
	output := DefaultCacheDir()
	if _, err := os.Stat(filepath.Join(output, "pack.json")); err == nil {
		if _, err := os.Stat(filepath.Join(root, "assets", "World0", "Levels", "lab.xml")); err != nil || hasScriptLevelCache(output) {
			return output, nil
		}
	}
	if err := Import(root, output); err != nil {
		return "", err
	}
	return output, nil
}
