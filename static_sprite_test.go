package main

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"strings"
	"testing"
)

func TestStaticSpritePreservesSelectedFrame(t *testing.T) {
	catalog, err := formats.ParseSprites(strings.NewReader(`<SpriteLibrary><Sprites><Sprite name="Maddog"><Anim name="Still" texture="Textures/Characters/maddog_body_walk_HD" numFrames="4" numAngles="5" fps="0" loop="0"/></Sprite></Sprites></SpriteLibrary>`))
	if err != nil {
		t.Fatal(err)
	}
	entity := &scriptEntity{texture: "Maddog", playing: true, frame: 2, frameTime: .25}
	p := &playState{sprites: catalog, scriptEntities: map[int]*scriptEntity{1: entity}}
	for i := 0; i < 120; i++ {
		p.updateScriptEntities()
	}
	if entity.frame != 2 || entity.frameTime != .25 || p.spriteFPS("Maddog", "Still") != 0 {
		t.Fatal("static FPS was replaced with a default animation rate")
	}
}
