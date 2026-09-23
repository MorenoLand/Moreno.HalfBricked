package formats

import (
	"encoding/xml"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

func ParseLevelCatalogXML(reader io.Reader, sourceXML string) ([]LevelInfo, error) {
	var document levelsDocument
	if err := xml.NewDecoder(reader).Decode(&document); err != nil {
		return nil, err
	}
	result := make([]LevelInfo, 0, len(document.Levels))
	for _, item := range document.Levels {
		result = append(result, levelInfoFromXML(item, sourceXML))
	}
	return result, nil
}

func levelInfoFromXML(item levelXML, sourceXML string) LevelInfo {
	world, _ := strconv.Atoi(item.WorldIndex)
	return LevelInfo{ID: item.LevelName, DisplayName: item.DisplayName, BaseFile: item.BaseFile, NextLevel: strings.TrimSpace(item.NextLevel), UnlockLevels: splitFlags(item.UnlockLevels), Music: item.Music, VoiceoverPrefix: item.VoiceoverPrefix, ConversationXMLs: splitFlags(item.ConversationXMLs), LeaderboardID: item.LeaderboardID, LeaderboardIDSD: item.LeaderboardIDSD, LeaderboardIDHD: item.LeaderboardIDHD, WorldIndex: world, Flags: splitFlags(item.Flags), Description: item.Description, PostcardImage: item.PostcardImage, SourceXML: filepath.ToSlash(sourceXML)}
}
