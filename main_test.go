package main

import (
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
)

func TestPortalCellMatchesNativeBounds(t *testing.T) {
	for _, test := range []struct {
		x, y         float64
		wantX, wantY int
	}{
		{-1, -1, 0, 0},
		{364, 530, 5, 8},
		{730, 660, 11, 10},
		{9999, 9999, 0x22, 0x12},
	} {
		gotX, gotY := portalCell(test.x, test.y)
		if gotX != test.wantX || gotY != test.wantY {
			t.Fatalf("portalCell(%.1f,%.1f)=(%d,%d), want (%d,%d)", test.x, test.y, gotX, gotY, test.wantX, test.wantY)
		}
	}
}

func TestBarryAimDirectionMatchesFacingColumns(t *testing.T) {
	for _, test := range []struct {
		angle int
		dx    float64
		dy    float64
	}{
		{0, 0, 1},
		{4, 1, 0},
		{8, 0, -1},
	} {
		dx, dy := barryAimDirection(test.angle, false)
		if dx < test.dx-.001 || dx > test.dx+.001 || dy < test.dy-.001 || dy > test.dy+.001 {
			t.Fatalf("barryAimDirection(%d)=(%.3f,%.3f), want (%.3f,%.3f)", test.angle, dx, dy, test.dx, test.dy)
		}
	}
}

func TestSetZombieTextureUsesLoadedNumericSlot(t *testing.T) {
	play := &playState{
		zombies:        []zombieState{{scriptID: 7, texture: "zombie"}},
		scriptEntities: map[int]*scriptEntity{7: {id: 7, kind: "zombie", texture: "zombie"}},
		scriptTextures: map[int]*scriptTexture{2: {id: 2, name: "Characters/professoridle"}},
	}
	host := &playScriptHost{play: play}
	if _, err := host.zombieProperty("SetZombieTexture", []scripting.Value{7, 2}); err != nil {
		t.Fatal(err)
	}
	if got := play.zombies[0].texture; got != "Characters/professoridle" {
		t.Fatalf("zombie texture = %q, want loaded slot texture", got)
	}
	if got := play.scriptEntities[7].texture; got != "Characters/professoridle" {
		t.Fatalf("script entity texture = %q, want loaded slot texture", got)
	}
}

func TestSetEntityRotationPreservesScriptFacing(t *testing.T) {
	play := &playState{
		zombies:        []zombieState{{scriptID: 7}},
		scriptEntities: map[int]*scriptEntity{7: {id: 7, kind: "zombie"}},
	}
	host := &playScriptHost{play: play}
	if _, err := host.Call("SetEntityRotation", []scripting.Value{7, 180}); err != nil {
		t.Fatal(err)
	}
	entity := play.scriptEntities[7]
	if !entity.rotationSet || entity.angle != 4 || !entity.flipX {
		t.Fatalf("script rotation state = set:%t angle:%d flipX:%t, want set:true angle:4 flipX:true", entity.rotationSet, entity.angle, entity.flipX)
	}
	if got := play.zombies[0]; got.angle != 4 || !got.flipX {
		t.Fatalf("zombie rotation state = angle:%d flipX:%t, want angle:4 flipX:true", got.angle, got.flipX)
	}
}

func TestMakeZombieInvulnerableSetsDamageGate(t *testing.T) {
	play := &playState{
		zombies:        []zombieState{{scriptID: 7, health: 100}},
		scriptEntities: map[int]*scriptEntity{7: {id: 7, kind: "zombie"}},
	}
	host := &playScriptHost{play: play}
	if _, err := host.Call("MakeZombieInvulnerable", []scripting.Value{7, true}); err != nil {
		t.Fatal(err)
	}
	if !play.zombies[0].invulnerable {
		t.Fatal("zombie invulnerability was not enabled")
	}
}

func TestInvulnerableZombieIgnoresGrenadeDamage(t *testing.T) {
	play := &playState{zombies: []zombieState{{x: 0, y: 0, health: 100, invulnerable: true}, {x: 64, y: 0, health: 100}}}
	play.detonateGrenade(0, 0)
	if got := play.zombies[0].health; got != 100 {
		t.Fatalf("invulnerable zombie health = %.1f, want 100", got)
	}
	if got := play.zombies[1].health; got >= 100 {
		t.Fatalf("normal zombie health = %.1f, want damage", got)
	}
}

func TestGetPlatformMatchesNativeValue(t *testing.T) {
	host := &playScriptHost{play: &playState{}}
	result, err := host.Call("GetPlatform", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Values) != 1 || result.Values[0] != 5 {
		t.Fatalf("GetPlatform() = %#v, want numeric 5", result.Values)
	}
}

func TestDrawScriptTextUsesNativeFlags(t *testing.T) {
	play := &playState{}
	host := &playScriptHost{play: play}
	if _, err := host.drawScriptText([]scripting.Value{240, 20, "large"}, false); err != nil {
		t.Fatal(err)
	}
	if play.scriptText1Y != 136 || play.scriptText1Size != 30 {
		t.Fatalf("default DrawText1 state = y %.1f size %.1f, want y 136 size 30", play.scriptText1Y, play.scriptText1Size)
	}
	if _, err := host.drawScriptText([]scripting.Value{240, 123, "large", true, true}, false); err != nil {
		t.Fatal(err)
	}
	if play.scriptText1Y != 123 || play.scriptText1Size != 30 {
		t.Fatalf("large flagged DrawText1 state = y %.1f size %.1f, want y 123 size 30", play.scriptText1Y, play.scriptText1Size)
	}
	if _, err := host.drawScriptText([]scripting.Value{240, 149, "small", false}, true); err != nil {
		t.Fatal(err)
	}
	if play.scriptText2Y != 149 || play.scriptText2Size != 24 {
		t.Fatalf("regular DrawText2 state = y %.1f size %.1f, want y 149 size 24", play.scriptText2Y, play.scriptText2Size)
	}
}

func TestWalkPlayerToUsesNativeRangeCheck(t *testing.T) {
	play := &playState{x: 70, y: 0, scriptEntities: map[int]*scriptEntity{}}
	host := &playScriptHost{play: play}
	if _, err := host.Call("WalkPlayerTo", []scripting.Value{100, 0, 32}); err != nil {
		t.Fatal(err)
	}
	if play.scriptWalkRange != 32 || !play.scriptWalking {
		t.Fatalf("WalkPlayerTo state = range %.1f walking %t, want range 32 walking true", play.scriptWalkRange, play.scriptWalking)
	}
	play.updateScriptWalk()
	if play.scriptWalking {
		t.Fatal("WalkPlayerTo remained active inside native range check")
	}
	if play.x != 70 || play.y != 0 {
		t.Fatalf("WalkPlayerTo moved player inside range to (%.1f,%.1f), want unchanged (70,0)", play.x, play.y)
	}
}
