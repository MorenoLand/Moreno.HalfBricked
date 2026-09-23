package formats

import (
	"encoding/xml"
	"testing"
)

func TestParsePropsKeepsAnimatedXMLRecords(t *testing.T) {
	var document levelXMLData
	if err := xml.Unmarshal([]byte(`<level><props><prop texture="rock" position="10,20" height="1" scale="2,2" uv1="0,0" uv2="32,32"/><animated_prop texture="campfiresmoke" position="30,40" height="3" scale="2,2" x_frames="1" y_frames="4" frame_time="80"/></props></level>`), &document); err != nil {
		t.Fatal(err)
	}
	props, animated, err := parseProps(document.Props)
	if err != nil {
		t.Fatal(err)
	}
	if len(props) != 1 || len(animated) != 1 {
		t.Fatalf("props = %d animated = %d, want 1 and 1", len(props), len(animated))
	}
	if animated[0].Texture != "campfiresmoke" || animated[0].X != 30 || animated[0].Y != 40 || animated[0].Height != 3 || animated[0].XFrames != 1 || animated[0].YFrames != 4 || animated[0].FrameTime != 80 {
		t.Fatalf("animated prop = %#v", animated[0])
	}
}

func TestParseWavesKeepsNativeSpawnAttributes(t *testing.T) {
	var document waveList
	if err := xml.Unmarshal([]byte(`<waves><wave next_wave="0" run_time="1000" end_wave_time="100" end_wave_zombies="0"><spawner delay_time="0" count="1" index="3"><type name="armed_zombie" chance="2" speed="80" strength="300" size="40" turnSpeed="12" texture="cyborg" weapon="PISTOL" alertRadius="120" deviateCycleSpeed="1.5" deviateAmount="4"/></spawner></wave></waves>`), &document); err != nil {
		t.Fatal(err)
	}
	waves, err := parseWaves(document)
	if err != nil {
		t.Fatal(err)
	}
	spawn := waves[0].Spawners[0].Types[0]
	if spawn.Weapon != "PISTOL" || spawn.AlertRadius != 120 || spawn.DeviateCycleSpeed != 1.5 || spawn.DeviateAmount != 4 {
		t.Fatalf("spawn attributes = %#v", spawn)
	}
}

func TestLevelXMLKeepsProgressionMetadata(t *testing.T) {
	var document levelsDocument
	if err := xml.Unmarshal([]byte(`<Levels><Level levelName="World0Level2" nextLevel="World1Level0" unlockLevels="World0Survival0;World0Survival1" levelFlags="STORY|ENDWORLD" music="Music_Caveman" voiceoverPrefix="VO_Caveman" conversationXmls="banter_000;chat_000;chat_cutscene_000" leaderboardIdSD="383483" leaderboardIdHD="990246"/></Levels>`), &document); err != nil {
		t.Fatal(err)
	}
	level := document.Levels[0]
	unlockLevels, flags := splitFlags(level.UnlockLevels), splitFlags(level.Flags)
	conversations := splitFlags(level.ConversationXMLs)
	if level.NextLevel != "World1Level0" || len(unlockLevels) != 2 || unlockLevels[0] != "World0Survival0" || unlockLevels[1] != "World0Survival1" || len(flags) != 2 || flags[0] != "STORY" || flags[1] != "ENDWORLD" || level.Music != "Music_Caveman" || level.VoiceoverPrefix != "VO_Caveman" || len(conversations) != 3 || conversations[0] != "banter_000" || conversations[2] != "chat_cutscene_000" || level.LeaderboardIDSD != "383483" || level.LeaderboardIDHD != "990246" {
		t.Fatalf("progression metadata = %#v", level)
	}
}
