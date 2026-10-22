package platform

import (
	"os"
	"path/filepath"
	"strings"
)

// Small files the game keeps for its player: the level editor's settings, the stamp library, its autosave. On the desktop
// they are files in the user's config folder (HalfBricked); in the browser they are entries of local storage.

func configPath(name string) string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "HalfBricked", filepath.FromSlash(name))
}

func nativeConfigRead(name string) ([]byte, bool) {
	path := configPath(name)
	if path == "" {
		return nil, false
	}
	data, err := os.ReadFile(path)
	return data, err == nil
}

func nativeConfigWrite(name string, data []byte) bool {
	path := configPath(name)
	if path == "" {
		return false
	}
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return false
	}
	return os.WriteFile(path, data, 0o644) == nil
}

func nativeConfigDelete(name string) {
	if path := configPath(name); path != "" {
		_ = os.Remove(path)
	}
}

func nativeConfigList(prefix string) []string {
	path := configPath(prefix)
	if path == "" {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			names = append(names, strings.TrimSuffix(prefix, "/")+"/"+entry.Name())
		}
	}
	return names
}
