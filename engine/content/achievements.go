package content

import (
	"encoding/xml"
	"fmt"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"path/filepath"
	"sort"
	"strings"
)

func (p *Pack) Achievements() (formats.AchievementCatalog, error) {
	var catalog formats.AchievementCatalog
	keys := []string{}
	for key := range p.manifest.Files {
		if strings.HasSuffix(strings.ToLower(filepath.Base(filepath.ToSlash(key))), "_manifest.xml") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	seenFiles, seenIDs := map[string]bool{}, map[string]bool{}
	for _, key := range keys {
		r, err := p.source.Open(p.manifest.Files[key])
		if err != nil {
			return nil, err
		}
		var manifest struct {
			XMLInfo []struct {
				Achievement string `xml:"achievement,attr"`
			} `xml:"XmlInfo"`
		}
		err = xml.NewDecoder(r).Decode(&manifest)
		r.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		for _, info := range manifest.XMLInfo {
			name := strings.TrimSpace(info.Achievement)
			if name == "" {
				continue
			}
			if filepath.Ext(name) == "" {
				name += ".xml"
			}
			wanted := filepath.ToSlash(filepath.Join(filepath.Dir(key), name))
			path, ok := p.SourcePath(wanted)
			if !ok {
				return nil, fmt.Errorf("achievement catalog %q missing", wanted)
			}
			if seenFiles[path] {
				continue
			}
			seenFiles[path] = true
			r, err := p.source.Open(path)
			if err != nil {
				return nil, err
			}
			entries, err := formats.ParseAchievements(r)
			r.Close()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			for _, entry := range entries {
				if seenIDs[entry.ID] {
					return nil, fmt.Errorf("duplicate achievement ID %q", entry.ID)
				}
				seenIDs[entry.ID] = true
				catalog = append(catalog, entry)
			}
		}
	}
	if path, ok := p.SourcePath("Common0/Xml/Services/iOS/Provider_GameCenter_Achievements.xml"); ok {
		r, err := p.source.Open(path)
		if err != nil {
			return nil, err
		}
		var provider struct {
			XMLName xml.Name              `xml:"Achievements"`
			Entries []formats.Achievement `xml:"achievement"`
		}
		err = xml.NewDecoder(r).Decode(&provider)
		r.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		descriptions := map[string]string{}
		for _, entry := range provider.Entries {
			descriptions[entry.ID] = entry.Description
		}
		for index := range catalog {
			catalog[index].Description = descriptions[catalog[index].ID]
		}
	}
	return catalog, nil
}
