//go:build !js

package game

import (
	"os"
	"path/filepath"
)

func readPlayerProfile() ([]byte, error) {
	data, err := os.ReadFile(filepath.Join("bin", "profile.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}
func writePlayerProfile(data []byte) error {
	if err := os.MkdirAll("bin", 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join("bin", "profile.json"), data, 0600)
}
