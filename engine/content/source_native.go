//go:build !js

package content

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type fileSource struct{ root string }

func NewSource(root string) AssetSource { return fileSource{root: root} }
func (s fileSource) Open(path string) (io.ReadCloser, error) {
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return nil, os.ErrPermission
	}
	return os.Open(filepath.Join(s.root, clean))
}
func (s fileSource) Manifest() (PackManifest, error) {
	r, err := s.Open("pack.json")
	if err != nil {
		return PackManifest{}, err
	}
	defer r.Close()
	var manifest PackManifest
	err = json.NewDecoder(r).Decode(&manifest)
	return manifest, err
}
