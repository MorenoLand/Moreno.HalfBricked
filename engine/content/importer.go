package content

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MorenoLand/Moreno.AgeofZombies/engine/formats"
)

func Import(referenceRoot, outputRoot string) error {
	if _, err := os.Stat(filepath.Join(referenceRoot, "assets")); err != nil {
		return fmt.Errorf("reference assets: %w", err)
	}
	levels, err := formats.ParseLevelCatalog(referenceRoot)
	if err != nil {
		return err
	}
	tilesets, err := formats.ParseTileSets(referenceRoot)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(outputRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(outputRoot, "levels"), 0755); err != nil {
		return err
	}
	manifest := PackManifest{SchemaVersion: 1, SourceVersion: "Age of Zombies 1.2.1", Levels: levels, TileSets: tilesets, Textures: map[string]string{}, Files: map[string]string{}}
	for _, info := range levels {
		level, err := formats.ParseLevel(referenceRoot, info)
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(level, "", "  ")
		if err != nil {
			return err
		}
		path := filepath.Join(outputRoot, "levels", info.ID+".json")
		if err := os.WriteFile(path, data, 0644); err != nil {
			return err
		}
	}
	if err := importAssets(referenceRoot, outputRoot, &manifest); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputRoot, "pack.json"), data, 0644)
}

func importAssets(root, out string, manifest *PackManifest) error {
	var paths []string
	err := filepath.WalkDir(filepath.Join(root, "assets"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(paths)
	for _, path := range paths {
		rel, _ := filepath.Rel(filepath.Join(root, "assets"), path)
		rel = filepath.ToSlash(rel)
		ext := strings.ToLower(filepath.Ext(path))
		key := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
		switch ext {
		case ".tex":
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			img, err := formats.DecodeTexture(f)
			f.Close()
			if err != nil {
				return fmt.Errorf("%s: %w", rel, err)
			}
			name := filepath.ToSlash(filepath.Join("textures", key+".png"))
			target := filepath.Join(out, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			tf, err := os.Create(target)
			if err != nil {
				return err
			}
			err = png.Encode(tf, img)
			tf.Close()
			if err != nil {
				return err
			}
			if _, ok := manifest.Textures[key]; !ok {
				manifest.Textures[key] = name
			}
			manifest.Files[rel] = name
		case ".ogg":
			name := filepath.ToSlash(filepath.Join("audio", rel))
			target := filepath.Join(out, filepath.FromSlash(name))
			if err := copyFile(path, target); err != nil {
				return err
			}
			manifest.Files[rel] = name
		case ".fnt", ".xml", ".script", ".txt":
			name := filepath.ToSlash(filepath.Join("source", rel))
			target := filepath.Join(out, filepath.FromSlash(name))
			if err := copyFile(path, target); err != nil {
				return err
			}
			manifest.Files[rel] = name
		}
	}
	return nil
}

func copyFile(source, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
