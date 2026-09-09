package formats

import (
	"strings"
	"testing"
)

func TestParseSpritesKeepsAnimationMetadata(t *testing.T) {
	catalog, err := ParseSprites(strings.NewReader(`<Sprites><Sprite name="Characters/cavezombie"><Anim name="Idle" texture="Textures/Characters/cavezombie_SD" numFrames="4" numAngles="5" fps="8" loop="1"/></Sprite><Sprite name="rexwalk_1"><Anim name="Run" texture="Textures/Characters/rexwalk_SD" numFrames="4" numAngles="9" fps="8" loop="1"/><Anim name="Rage" texture="Textures/Characters/rexrage_SD" numFrames="2" numAngles="2" fps="8" loop="1"/></Sprite></Sprites>`))
	if err != nil {
		t.Fatal(err)
	}
	zombie, ok := catalog.Find("characters/cavezombie")
	if !ok {
		t.Fatal("cavezombie definition missing")
	}
	animation, ok := zombie.Animation("idle")
	if !ok || animation.Frames != 4 || animation.Angles != 5 || animation.FPS != 8 || !animation.Loop {
		t.Fatalf("unexpected zombie animation: %+v", animation)
	}
	rex, ok := catalog.Find("rexwalk_1")
	if !ok {
		t.Fatal("rex definition missing")
	}
	if animation, ok := rex.Animation("rage"); !ok || animation.Frames != 2 || animation.Angles != 2 {
		t.Fatalf("unexpected rage animation: %+v", animation)
	}
	if animation, ok := rex.AnimationByIndex(1); !ok || animation.Name != "Rage" {
		t.Fatalf("unexpected animation order: %+v", animation)
	}
}
