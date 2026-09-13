package formats

import (
	"strings"
	"testing"
)

func TestParseSpritesKeepsAnimationMetadata(t *testing.T) {
	catalog, err := ParseSprites(strings.NewReader(`<SpriteLibrary><Sprites><Sprite name="Characters/cavezombie"><Anim name="Idle" texture="Textures/Characters/cavezombie_SD" numFrames="4" numAngles="5" fps="8" loop="1"/></Sprite><Sprite name="rexwalk_1"><Anim name="Run" texture="Textures/Characters/rexwalk_SD" numFrames="4" numAngles="9" fps="8" loop="1"/><Anim name="Rage" texture="Textures/Characters/rexrage_SD" numFrames="2" numAngles="2" fps="8" loop="1"/></Sprite></Sprites></SpriteLibrary>`))
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

func TestParseSpritesAcceptsReferenceCommentSeparators(t *testing.T) {
	catalog, err := ParseSprites(strings.NewReader(`<SpriteLibrary><Sprites><!-- ------------------------------------------- PLAYER ---------------------------------------------------------- --><Sprite name="characters/gangster"><Anim name="Idle" texture="Textures/Characters/gangster_SD" numFrames="4" numAngles="5" fps="8" loop="1"/></Sprite></Sprites></SpriteLibrary>`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := catalog.Find("characters/gangster"); !ok {
		t.Fatal("gangster definition missing")
	}
}

func TestParseSpritesLoadsReferenceDeathCatalog(t *testing.T) {
	catalog, err := ParseSprites(strings.NewReader(`<SpriteLibrary><Sprites><Sprite name="ZombieDeaths"><Anim name="Pop_0" texture="Textures/ZombiePop_0_SD" numFrames="4" loop="0"/></Sprite></Sprites></SpriteLibrary>`))
	if err != nil {
		t.Fatal(err)
	}
	animation, ok := catalog.Find("ZombieDeaths")
	if !ok {
		t.Fatal("ZombieDeaths definition missing")
	}
	pop, ok := animation.Animation("Pop_0")
	if !ok || pop.Frames != 4 || pop.FPS != 8 || pop.Loop {
		t.Fatalf("death animation = %#v, found %t, want reference defaults", pop, ok)
	}
}
