//go:build !js

package platform

import (
	"os"
	"path/filepath"
	"strings"
)

// The desktop build remembers where the game's Android package is, so the first launch asks once and later launches
// just use it. The path lives in the user's config directory (not next to the executable, not in the repository).

func apkSettingsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "HalfBricked", "apk-path.txt"), nil
}

// RememberedAPK is the saved APK path, or "" when none is saved or the file no longer exists.
func RememberedAPK() string {
	path, err := apkSettingsPath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	apk := strings.TrimSpace(string(data))
	if info, err := os.Stat(apk); err != nil || info.IsDir() {
		return ""
	}
	return apk
}

// RememberAPK saves the APK path for the next launch.
func RememberAPK(apk string) error {
	path, err := apkSettingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(apk+"\n"), 0o600)
}
