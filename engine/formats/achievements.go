package formats

import (
	"encoding/xml"
	"fmt"
	"io"
)

type Achievement struct {
	ID                  string `xml:"id,attr"`
	Name                string `xml:"name,attr"`
	Description         string `xml:"description,attr"`
	DescriptionLocked   string `xml:"description_locked,attr"`
	DescriptionUnlocked string `xml:"description_unlocked,attr"`
	Type                string `xml:"type,attr"`
	Check               string `xml:"check,attr"`
	SpecificType        string `xml:"specific_type,attr"`
	Texture             string `xml:"texture,attr"`
	Score               int    `xml:"score,attr"`
	Total               int    `xml:"total,attr"`
}

// DisplayDescription is the text for the list: the locked or unlocked text from
// the achievement definition, falling back to the other, then the provider text.
func (a Achievement) DisplayDescription(unlocked bool) string {
	first, second := a.DescriptionLocked, a.DescriptionUnlocked
	if unlocked {
		first, second = second, first
	}
	for _, text := range []string{first, a.Description, second} {
		if text != "" {
			return text
		}
	}
	return ""
}

type AchievementCatalog []Achievement

func ParseAchievements(r io.Reader) (AchievementCatalog, error) {
	var file struct {
		XMLName xml.Name           `xml:"achievementManagerFile"`
		Entries AchievementCatalog `xml:"achievement"`
	}
	if err := xml.NewDecoder(r).Decode(&file); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, entry := range file.Entries {
		if entry.ID == "" || entry.Name == "" || seen[entry.ID] {
			return nil, fmt.Errorf("invalid or duplicate achievement %q", entry.ID)
		}
		seen[entry.ID] = true
	}
	return file.Entries, nil
}
