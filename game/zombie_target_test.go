package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
	"testing"
)

func TestZombieTargetFollowsPlayerAfterScript(t *testing.T) {
	p := &playState{world: &viewer.Viewer{Level: formats.Level{Width: 20, Height: 20, Layers: map[formats.LayerKind][]uint32{formats.LayerC: make([]uint32, 400)}}}, tileSize: 32, x: 320, y: 320, health: 1, scriptHasZombieTarget: true, scriptZombieTargetX: 64, scriptZombieTargetY: 64, scriptEntities: map[int]*scriptEntity{1: {id: 1}}, zombies: []zombieState{{x: 64, y: 64, speed: 60, health: 100, size: formats.Vec2{X: 48, Y: 48}, scriptID: 1}}}
	p.updateZombies()
	if p.zombies[0].x <= 64 || p.zombies[0].y <= 64 || p.health != 1 {
		t.Fatalf("stale target retained or remote damage: zombie %+v health %g", p.zombies[0], p.health)
	}
	runtime, err := scripting.New("return", &playScriptHost{play: p}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if err := runtime.Step(); err != nil {
		t.Fatal(err)
	}
	p.scriptRuntime = runtime
	p.zombies[0].x, p.zombies[0].y = 64, 64
	p.updateZombies()
	if p.zombies[0].x <= 64 || p.zombies[0].y <= 64 || p.health != 1 {
		t.Fatal("completed script retained target or inflicted remote damage")
	}
}

func TestZombieContactUsesPlayerNotScriptTarget(t *testing.T) {
	p := &playState{world: &viewer.Viewer{}, x: 320, y: 320, health: 1, scriptHasZombieTarget: true, scriptZombieTargetX: 64, scriptZombieTargetY: 64, scriptEntities: map[int]*scriptEntity{1: {id: 1}}, zombies: []zombieState{{x: 64, y: 64, health: 100, size: formats.Vec2{X: 48, Y: 48}, scriptID: 1}}}
	runtime, err := scripting.New("coroutine.yield()", &playScriptHost{play: p}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	p.scriptRuntime = runtime
	p.updateZombies()
	if p.health != 1 {
		t.Fatal("movement-target contact damaged remote player")
	}
	p.zombies[0].x, p.zombies[0].y = p.x, p.y
	p.updateZombies()
	if p.health >= 1 {
		t.Fatal("actual player contact did not damage player")
	}
}
