package game

import (
	"image"
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
)

func markerTestPlay(build string) *playState {
	p := trainTestPlay()
	p.world.Zoom = 1
	p.world.Level.WaveBuild = build
	p.scriptEntities = map[int]*scriptEntity{}
	p.hudVisible = true
	p.x, p.y = 240, 160
	return p
}

func addMarkerZombie(p *playState, x, y float64) *zombieState {
	p.zombies = append(p.zombies, zombieState{x: x, y: y, health: 100, size: formats.Vec2{X: 38, Y: 48}, native: zombieNative{sizeZ: 48}})
	return &p.zombies[len(p.zombies)-1]
}

func TestMarkerAlphaArithmeticMatchesNative(t *testing.T) {
	dt := 1.0 / 60.0
	if got := markerAlphaRises(0, dt); got != 33 {
		t.Fatalf("rise from 0: %d, want int(2000/60) = 33", got)
	}
	if got := markerAlphaRises(140, dt); got != markerAlphaCap {
		t.Fatalf("rise is capped at 0x96: %d", got)
	}
	if got := markerAlphaFalls(33, dt); got != 16 {
		t.Fatalf("fall from 33: %d, want uint(33-16.67) = 16", got)
	}
	if got := markerAlphaFalls(10, dt); got != 0 {
		t.Fatalf("fall clamps at zero: %d", got)
	}
}

func TestZombieMarkerRisesOnlyWhileOffscreenAndPlayerAlive(t *testing.T) {
	p := markerTestPlay(content.WaveBuild125)
	off := addMarkerZombie(p, 900, 160)
	on := addMarkerZombie(p, 200, 160)
	// addMarkerZombie appends: re-take the pointers after both appends.
	off, on = &p.zombies[0], &p.zombies[1]
	for frame := 0; frame < 30; frame++ {
		p.stepZombieMarkers(1.0 / 60)
	}
	if off.native.markerAlpha != markerAlphaCap {
		t.Fatalf("off-screen zombie alpha %d, want the 150 cap", off.native.markerAlpha)
	}
	if on.native.markerAlpha != 0 {
		t.Fatalf("on-screen zombie alpha %d", on.native.markerAlpha)
	}
	// It fades at 1000/s once the zombie is visible: 150 -> 0 in ten frames.
	off.x = 220
	for frame := 0; frame < 12; frame++ {
		p.stepZombieMarkers(1.0 / 60)
	}
	if off.native.markerAlpha != 0 {
		t.Fatalf("visible zombie kept alpha %d", off.native.markerAlpha)
	}
	// A dead player stops the health rule.
	p.health = 0
	off.x = 900
	p.stepZombieMarkers(1.0 / 60)
	if off.native.markerAlpha != 0 {
		t.Fatalf("alpha rose with the player dead: %d", off.native.markerAlpha)
	}
}

func TestZombieMarkerBossRuleUsesLevelBounds(t *testing.T) {
	p := markerTestPlay(content.WaveBuild125)
	p.health = 0 // the boss rule ignores the player's health
	addMarkerZombie(p, 900, 160)
	boss := &p.zombies[0]
	boss.scriptID = 7
	p.scriptEntities[7] = &scriptEntity{id: 7, kind: "zombie", entityType: "boss_egyptian"}
	p.stepZombieMarkers(1.0 / 60)
	if boss.native.markerAlpha == 0 {
		t.Fatal("a boss inside the level bounds did not raise its alpha")
	}
	boss.native.markerAlpha = 0
	boss.x = -97
	p.stepZombieMarkers(1.0 / 60)
	if boss.native.markerAlpha != 0 {
		t.Fatal("a boss left of -96 raised its alpha")
	}
	boss.x = float64(p.world.Level.Width+3)*32 + 1
	p.stepZombieMarkers(1.0 / 60)
	if boss.native.markerAlpha != 0 {
		t.Fatal("a boss right of (width+3)*32 raised its alpha")
	}
	// The rex and the western boss use the health rule.
	addMarkerZombie(p, 900, 160)
	rex := &p.zombies[1]
	rex.scriptID = 8
	p.scriptEntities[8] = &scriptEntity{id: 8, kind: "zombie", entityType: "boss_rex"}
	p.stepZombieMarkers(1.0 / 60)
	if rex.native.markerAlpha != 0 {
		t.Fatal("rex followed the boss bound rule while the player is dead")
	}
}

func TestZombieMarkerPlacementClampsSizesAndRotates(t *testing.T) {
	x, y, side, angle := zombieMarkerPlacement(900, -40, 48, 1200, 100, 240, 160)
	if x != logicalWidth || y != 0 {
		t.Fatalf("marker at %v,%v is not clamped to the 480x320 screen", x, y)
	}
	if math.Abs(side-16.8) > 1e-4 {
		t.Fatalf("side %v, want 48 * 0.35", side)
	}
	if want := math.Atan2(160-100, 240-1200); !near(angle, want) {
		t.Fatalf("angle %v want %v", angle, want)
	}
	// Art points left; a zombie to the right (cam - zombie points left) therefore needs the half turn.
	if math.Abs(math.Abs(angle)-math.Pi) > .3 {
		t.Fatalf("rotation %v does not turn the arrow toward a zombie on the right", angle)
	}
}

func TestPickupMarkerAlphaBuildsDiffer(t *testing.T) {
	for _, build := range []string{content.WaveBuildV7, content.WaveBuild125} {
		p := markerTestPlay(build)
		e := &scriptEntity{id: 1, kind: "pickup", texture: "p_shotgun", x: 1000, y: 160}
		p.scriptEntities[1] = e
		p.stepPickupMarkers(1.0 / 60)
		// Resting pickup: the update decays first (nothing yet), then the rise adds 2000/60.
		want := 33.0
		if build == content.WaveBuild125 {
			want = 2000.0 / 60
		}
		if math.Abs(e.markerAlpha-want) > .01 {
			t.Fatalf("%s: first frame alpha %v want %v", build, e.markerAlpha, want)
		}
		for frame := 0; frame < 20; frame++ {
			p.stepPickupMarkers(1.0 / 60)
		}
		if e.markerAlpha != markerAlphaCap {
			t.Fatalf("%s: alpha %v, want the 150 cap", build, e.markerAlpha)
		}
		// Back on screen: every resting frame subtracts dt*1000.
		e.x = 240
		for frame := 0; frame < 12; frame++ {
			p.stepPickupMarkers(1.0 / 60)
		}
		if e.markerAlpha != 0 {
			t.Fatalf("%s: on-screen pickup kept alpha %v", build, e.markerAlpha)
		}
		// HUDSetVisible(false) stops the rise (v7 scene +0x34dfc, 1.2.5 per player +0x17).
		e.x, p.hudVisible = 1000, false
		p.stepPickupMarkers(1.0 / 60)
		if e.markerAlpha != 0 {
			t.Fatalf("%s: alpha rose with the HUD hidden", build)
		}
	}
}

func TestPickupMarkerDroppingPickupDoesNotDecay(t *testing.T) {
	p := markerTestPlay(content.WaveBuildV7)
	e := &scriptEntity{id: 1, kind: "pickup", texture: "p_shotgun", x: 240, y: 160, drop: true, lift: 250, markerAlpha: 100}
	p.scriptEntities[1] = e
	p.stepPickupMarkers(1.0 / 60) // in the air: no bounce branch, on screen: no rise
	if e.markerAlpha != 100 {
		t.Fatalf("alpha changed in the air: %v", e.markerAlpha)
	}
	e.landed = true
	p.stepPickupMarkers(1.0 / 60)
	if e.markerAlpha != 83 {
		t.Fatalf("alpha after the bounce %v, want uint(100-16.67) = 83", e.markerAlpha)
	}
}

func TestPickupMarkerPlacementFollowsNativeFormula(t *testing.T) {
	// Landed pickup, pulse cosine 1: side = (1 + 0.15) * 0.5 * 50.
	g := pickupMarkerPlacement(0, 50, 600, 100, 0x8000, 1)
	if !near(g.side, 28.75) {
		t.Fatalf("side %v", g.side)
	}
	// Direction 0x8000 (player to the left): arrow and icon move left of the clamped edge point.
	if !near(g.arrowX, logicalWidth-28.75*.5) || !near(g.arrowY, 100) {
		t.Fatalf("arrow at %v,%v", g.arrowX, g.arrowY)
	}
	if !near(g.iconX, logicalWidth-28.75*.6) || !near(g.iconSide, 28.75*.8) {
		t.Fatalf("icon at %v side %v", g.iconX, g.iconSide)
	}
	if !near(g.rotation, math.Pi) {
		t.Fatalf("rotation %v", g.rotation)
	}
	// Falling from 250: the pulse term vanishes and the size grows to 1.25 * size.
	g = pickupMarkerPlacement(250, 50, 100, 100, 0, 1)
	if !near(g.side, 62.5) {
		t.Fatalf("falling side %v", g.side)
	}
	// The pulse cosine is cos(2*phase + 0x8000): a negated double-rate cosine of the glow phase.
	if c := pickupMarkerPulseCos(0); !near(c, -1) {
		t.Fatalf("pulse cosine at phase 0 is %v", c)
	}
}

func TestPickupShadowGeometry(t *testing.T) {
	w, h, a := pickupShadowGeometry(50, 0)
	if !near(w, 50) || !near(h, 33.3) || a != 254 {
		t.Fatalf("landed shadow %v %v %d", w, h, a)
	}
	w, _, a = pickupShadowGeometry(50, 250)
	if !near(w, 100) || a != 1 {
		t.Fatalf("high shadow %v %d", w, a)
	}
}

func TestMarkerTexturesExistInEveryCache(t *testing.T) {
	roots := achievementCaches()
	if len(roots) == 0 {
		t.Skip("original asset cache unavailable")
	}
	for _, root := range roots {
		pack, err := content.NewPack(content.NewSource(root))
		if err != nil {
			t.Fatal(err)
		}
		a := &app{pack: pack, images: map[string]*ebiten.Image{}, sources: map[string]image.Image{}}
		for _, name := range []string{"Common0/Textures/markerzombie_SD", pickupMarkerTexture, pickupShadowTexture} {
			if _, err := a.Texture(name); err != nil {
				t.Fatalf("%s: %s: %v", root, name, err)
			}
		}
	}
}
