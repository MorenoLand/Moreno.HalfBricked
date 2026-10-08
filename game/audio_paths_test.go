package game

import (
	"bytes"
	"io"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
)

// The 1.2.5 cache stores audio per package; the legacy paths used by the game
// must still open and decode.
func TestLegacyAudioPathsResolveInPerPackageCache(t *testing.T) {
	pack, err := content.NewPack(content.NewSource("bin/data-cache"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"audio/music/sound/Music_Menu.ogg", "audio/music/sound/Music_Caveman.ogg", "audio/music/sound/Music_western.ogg",
		"audio/sound/sfx/menu_move.ogg", "audio/sound/sfx/menu_select.ogg", "audio/sound/sfx/menu_shotgun_cock_1.ogg",
	} {
		reader, err := pack.Open(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := vorbis.DecodeF32(bytes.NewReader(data)); err != nil {
			t.Fatalf("%s does not decode: %v", path, err)
		}
	}
}
