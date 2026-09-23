package content

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

func hydrateLevelMetadata(source AssetSource, manifest *PackManifest) error {
	if len(manifest.Levels) == 0 {
		return nil
	}
	catalogs := make([]string, 0)
	for key := range manifest.Files {
		name := strings.ToLower(filepath.ToSlash(key))
		if strings.Contains(name, "/xml/") && strings.HasSuffix(name, "_levels.xml") {
			catalogs = append(catalogs, key)
		}
	}
	if len(catalogs) == 0 {
		return fmt.Errorf("packed level catalog XML missing")
	}
	sort.Strings(catalogs)
	indices, updated := make(map[string]int, len(manifest.Levels)), make(map[string]bool, len(manifest.Levels))
	for index, info := range manifest.Levels {
		indices[strings.ToLower(info.ID)] = index
	}
	for _, key := range catalogs {
		path := manifest.Files[key]
		reader, err := source.Open(path)
		if err != nil {
			return fmt.Errorf("level catalog %q: %w", key, err)
		}
		infos, parseErr := formats.ParseLevelCatalogXML(reader, key)
		closeErr := reader.Close()
		if parseErr != nil {
			return fmt.Errorf("level catalog %q: %w", key, parseErr)
		}
		if closeErr != nil {
			return closeErr
		}
		for _, info := range infos {
			id := strings.ToLower(info.ID)
			index, exists := indices[id]
			if !exists {
				continue
			}
			current := &manifest.Levels[index]
			if current.SourceXML != "" && !strings.EqualFold(filepath.ToSlash(current.SourceXML), info.SourceXML) {
				continue
			}
			*current = info
			updated[id] = true
		}
	}
	for _, info := range manifest.Levels {
		if !updated[strings.ToLower(info.ID)] {
			return fmt.Errorf("level %q has no matching packed catalog XML", info.ID)
		}
	}
	return nil
}
