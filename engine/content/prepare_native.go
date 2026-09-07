//go:build !js

package content

import (
	"os"
	"path/filepath"
)

func PrepareAssets(root string) (string, error) {
	if _, err := os.Stat(filepath.Join(root, "pack.json")); err == nil {
		return root, nil
	}
	output := filepath.Join("bin", "data-cache")
	if _, err := os.Stat(filepath.Join(output, "pack.json")); err == nil {
		return output, nil
	}
	if err := Import(root, output); err != nil {
		return "", err
	}
	return output, nil
}
