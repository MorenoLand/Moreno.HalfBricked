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
