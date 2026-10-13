//go:build !js

package platform

import (
	"os"
	"path/filepath"
)

func ReadProfile() ([]byte, error) {
	data, err := os.ReadFile(filepath.Join("bin", "profile.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}
func WriteProfile(data []byte) error {
	if err := os.MkdirAll("bin", 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join("bin", "profile.json"), data, 0600)
}
