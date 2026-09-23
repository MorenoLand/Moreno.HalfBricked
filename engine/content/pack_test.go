package content

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

type memoryAssetSource struct {
	manifest PackManifest
	files    map[string]string
}

func (s memoryAssetSource) Open(path string) (io.ReadCloser, error) {
	data, ok := s.files[path]
	if !ok {
		return nil, fmt.Errorf("fixture file %q missing", path)
	}
	return io.NopCloser(strings.NewReader(data)), nil
}
func (s memoryAssetSource) Manifest() (PackManifest, error) { return s.manifest, nil }

func TestPackUsesDeclaredLevelAndConversationXML(t *testing.T) {
	levelXML := `<Levels><Level displayName="Prehistoric: Level 1.3" levelName="World0Level2" nextLevel="World1Level0" baseFileName="world0_level2" unlockLevels="World0Survival0;World0Survival1" music="Music_Caveman" voiceoverPrefix="VO_Caveman" conversationXmls="banter_000;chat_000;chat_cutscene_000" levelFlags="STORY;ENDWORLD" worldIndex="0"/></Levels>`
	levelData, err := json.Marshal(formats.Level{Info: formats.LevelInfo{ID: "World0Level2"}, Width: 1, Height: 1, Layers: map[formats.LayerKind][]uint32{formats.LayerG: {0}, formats.LayerD: {0}, formats.LayerH: {0}, formats.LayerHB: {0}, formats.LayerC: {0}}})
	if err != nil {
		t.Fatal(err)
	}
	source := memoryAssetSource{
		manifest: PackManifest{SchemaVersion: 1, Levels: []formats.LevelInfo{{ID: "World0Level2", BaseFile: "world0_level2", SourceXML: "World0/Xml/World0_Levels.xml"}}, Files: map[string]string{"World0/Xml/World0_Levels.xml": "source/World0/Xml/World0_Levels.xml", "Common0/Dialog/banter_000.xml": "source/Common0/Dialog/banter_000.xml", "Common0/Dialog/chat_000.xml": "source/Common0/Dialog/chat_000.xml", "Common0/Dialog/unlisted.xml": "source/Common0/Dialog/unlisted.xml"}},
		files:    map[string]string{"source/World0/Xml/World0_Levels.xml": levelXML, "source/Common0/Dialog/banter_000.xml": `<chat_file><conversation name="entry speech"><speech cameo="2"><text para="Barry, welcome back."/></speech></conversation></chat_file>`, "source/Common0/Dialog/chat_000.xml": `<chat_file><conversation name="tutorial intro"><speech cameo="0"><text para="Move and shoot."/></speech></conversation></chat_file>`, "source/Common0/Dialog/unlisted.xml": `<chat_file><conversation name="not in catalog"><speech cameo="0"><text para="Do not load me."/></speech></conversation></chat_file>`, "levels/World0Level2.json": string(levelData)},
	}
	pack, err := NewPack(source)
	if err != nil {
		t.Fatal(err)
	}
	info := pack.List()[0]
	if info.NextLevel != "World1Level0" || len(info.UnlockLevels) != 2 || info.Music != "Music_Caveman" || info.VoiceoverPrefix != "VO_Caveman" || len(info.ConversationXMLs) != 3 {
		t.Fatalf("catalog metadata = %#v", info)
	}
	level, err := pack.Load("World0Level2")
	if err != nil || level.Info.NextLevel != "World1Level0" || level.Info.Music != "Music_Caveman" {
		t.Fatalf("loaded level info = %#v, error %v", level.Info, err)
	}
	speech, err := pack.Conversation(0, "entry speech")
	if err != nil || speech.Name != "entry speech" || speech.Speeches[0].Cameo != 2 || speech.Speeches[0].Text[0] != "Barry, welcome back." {
		t.Fatalf("declared banter = %#v, error %v", speech, err)
	}
	if _, err := pack.Conversation(0, "not in catalog"); err == nil || !strings.Contains(err.Error(), "chat_cutscene_000") {
		t.Fatalf("missing/unlisted dialog error = %v, want declared chat_cutscene_000 gap", err)
	}
}
