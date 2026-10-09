package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
	"math"
	"testing"
)

func scriptZombieActivityHost(t *testing.T) *playScriptHost {
	t.Helper()
	p := &playState{world: &viewer.Viewer{Level: formats.Level{Width: 20, Height: 20, Layers: map[formats.LayerKind][]uint32{formats.LayerC: openLayer(400)}}}, tileSize: 32, x: 320, y: 320, health: 1, scriptEntities: map[int]*scriptEntity{}, zombies: []zombieState{{x: 64, y: 64, speed: 60, health: 100, fps: 8, size: formats.Vec2{X: 48, Y: 48}}}}
	h := &playScriptHost{play: p}
	var err error
	p.scriptRuntime, err = scripting.New("Idle()", h, scriptCallbacks)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.closeScript)
	if err := p.scriptRuntime.Step(); err != nil {
		t.Fatal(err)
	}
	return h
}
func TestScriptZombieActivityGatesAutomaticMovement(t *testing.T) {
	h := scriptZombieActivityHost(t)
	p := h.play
	p.updateZombies()
	if p.zombies[0].x != 64 || p.zombies[0].y != 64 || p.zombies[0].frame <= 0 {
		t.Fatalf("inactive movement or animation: %+v", p.zombies[0])
	}
	for _, value := range []int{1, 2, -1} {
		if _, err := h.Call("SetZombiesActiveDuringScripts", []scripting.Value{value}); err != nil {
			t.Fatal(err)
		}
		x := p.zombies[0].x
		p.updateZombies()
		if !p.scriptZombiesActive || p.zombies[0].x <= x {
			t.Fatalf("activity %d did not enable movement", value)
		}
	}
	if _, err := h.Call("SetZombiesActiveDuringScripts", []scripting.Value{0}); err != nil {
		t.Fatal(err)
	}
	x, y := p.zombies[0].x, p.zombies[0].y
	p.updateZombies()
	if p.scriptZombiesActive || p.zombies[0].x != x || p.zombies[0].y != y {
		t.Fatal("zero did not stop automatic movement")
	}
	for _, value := range []scripting.Value{true, "1", .5, math.NaN(), math.Inf(1)} {
		if _, err := h.Call("SetZombiesActiveDuringScripts", []scripting.Value{value}); err == nil || p.scriptZombiesActive {
			t.Fatalf("invalid activity %v mutated state", value)
		}
	}
	if err := p.scriptRuntime.Step(); err != nil {
		t.Fatal(err)
	}
	p.updateZombies()
	if !p.scriptRuntime.Done() || p.zombies[0].x <= x {
		t.Fatal("completed script retained movement gate")
	}
}
func TestScriptZombieActivityPreservesDamageAndDeathAnimation(t *testing.T) {
	h := scriptZombieActivityHost(t)
	p := h.play
	p.zombies[0].x, p.zombies[0].y, p.zombies[0].hitFlash = p.x, p.y, .1
	p.zombies = append(p.zombies, zombieState{dying: true, deathAge: .1})
	p.updateZombies()
	if p.health >= 1 || p.zombies[0].frame <= 0 || p.zombies[0].hitFlash >= .1 || p.zombies[1].deathAge <= .1 {
		t.Fatalf("movement gate suppressed damage or animation: health=%v zombies=%+v", p.health, p.zombies)
	}
}
func TestScriptZombieActivityPreservesExplicitWalkZombieTo(t *testing.T) {
	h := scriptZombieActivityHost(t)
	p := h.play
	p.zombies[0].scriptID, p.zombies[0].scriptControlled = 1, true
	p.scriptEntities[1] = &scriptEntity{id: 1, kind: "zombie", x: 64, y: 64}
	if _, err := h.Call("WalkZombieTo", []scripting.Value{1, 320, 320, 4}); err != nil {
		t.Fatal(err)
	}
	p.updateZombies()
	if p.scriptZombiesActive || p.zombies[0].x <= 64 || p.scriptEntities[1].x != p.zombies[0].x {
		t.Fatal("automatic movement gate blocked explicit scripted walking")
	}
}
