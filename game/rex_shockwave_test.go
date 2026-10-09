package game

import (
	"image"
	"math"
	"strings"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
)

const rexTestID = 7

// newRexRig builds a play state with one boss_rex (health 25000, render size
// 128, as spawned by world0_level2) at (x, y). The player stands far away.
func newRexRig(t *testing.T, catalog formats.AchievementCatalog, x, y float64) *achievementRig {
	t.Helper()
	r := newAchievementRig(t, catalog, formats.WeaponCatalog{})
	r.p.scriptEntities = map[int]*scriptEntity{rexTestID: {id: rexTestID, kind: "zombie", entityType: "boss_rex"}}
	r.p.zombies = append(r.p.zombies, zombieState{x: x, y: y, health: 25000, size: formats.Vec2{X: 128, Y: 128}, scriptID: rexTestID, alpha: 1, speed: 170})
	r.p.x, r.p.y = 100, 100
	return r
}

func (r *achievementRig) rex() *zombieState {
	for index := range r.p.zombies {
		if r.p.zombies[index].scriptID == rexTestID {
			return &r.p.zombies[index]
		}
	}
	return nil
}

func hasSFX(p *playState, name string) bool {
	for _, queued := range p.sfxQueue {
		if queued == name {
			return true
		}
	}
	return false
}

func TestRexShockwaveGeometryMatchesNative(t *testing.T) {
	if got := rexShockwaveSize(0.5); got != 300 {
		t.Fatalf("size at half life = %v, want 300 (0.5 * 600)", got)
	}
	if rexShockwaveSize(1) != 600 {
		t.Fatal("full-life size is not 600")
	}
	// alpha = (1 - t) * 255 * 5 clamped to a byte: opaque until 80% of the life.
	if rexShockwaveAlpha(0.5) != 1 || rexShockwaveAlpha(0.79) != 1 {
		t.Fatal("wave should stay opaque for the first 80% of its life")
	}
	if a := rexShockwaveAlpha(0.9); math.Abs(a-127.0/255) > 1e-9 {
		t.Fatalf("alpha at 0.9 = %v, want 127/255", a)
	}
	// Zombie ellipse test (FUN_000a5f74): reach = wave size (whole) + 0.3 * target
	// size, vertical axis divided by 0.667 (0x3f2a7efa).
	if !rexShockwaveHitsZombie(0, 0, 200, 199, 0, 0) || rexShockwaveHitsZombie(0, 0, 200, 201, 0, 0) {
		t.Fatal("horizontal zombie reach should be the whole wave size, 200")
	}
	if !rexShockwaveHitsZombie(0, 0, 200, 0, 133, 0) || rexShockwaveHitsZombie(0, 0, 200, 0, 134, 0) {
		t.Fatal("vertical zombie reach should be 200*0.667 = 133.4")
	}
	if !rexShockwaveHitsZombie(0, 0, 0, 14, 0, 48) || rexShockwaveHitsZombie(0, 0, 0, 15, 0, 48) {
		t.Fatal("target size should add 0.3*size to the reach")
	}
	// Player test: reach = 64*0.3 + 0.5 * wave size.
	if !rexShockwaveHitsPlayer(0, 0, 200, 119, 0) || rexShockwaveHitsPlayer(0, 0, 200, 120, 0) {
		t.Fatal("player reach should be 19.2 + 100")
	}
	if !rexShockwaveHitsPlayer(0, 0, 200, 0, 79) || rexShockwaveHitsPlayer(0, 0, 200, 0, 80) {
		t.Fatal("vertical player reach should be 119.2*0.667 = 79.5")
	}
	vertices, ok := rexShockwaveVertices(0.5, 100, 100, 1, 2, 2, 256, 256)
	if !ok {
		t.Fatal("no quad at half life")
	}
	if width, height := vertices[1].DstX-vertices[0].DstX, vertices[2].DstY-vertices[0].DstY; math.Abs(float64(width)-600) > .01 || math.Abs(float64(height)-300*2*rexShockwaveAspect) > .01 {
		t.Fatalf("quad %vx%v, want 600x400.2 (300 wide * frontend 2, aspect 0.667)", width, height)
	}
	if _, ok := rexShockwaveVertices(1, 0, 0, 1, 1, 1, 256, 256); ok {
		t.Fatal("wave should be gone at the end of its life")
	}
}

func TestRexShockwaveTextureExistsInBothCaches(t *testing.T) {
	found := false
	for _, root := range achievementCaches() {
		pack, err := content.NewPack(content.NewSource(root))
		if err != nil {
			t.Fatal(err)
		}
		a := &app{pack: pack, silent: true, images: map[string]*ebiten.Image{}, sources: map[string]image.Image{}}
		texture, err := a.Texture(rexShockwaveTexture)
		if err != nil {
			t.Fatalf("%s: %v", root, err)
		}
		if texture.Bounds().Dx() < 64 {
			t.Fatalf("%s: blast_radius texture is %v", root, texture.Bounds())
		}
		found = true
	}
	if !found {
		t.Skip("no asset cache")
	}
}

func TestRexRageHoldsLeapsAndLandsWithAShockwave(t *testing.T) {
	r := newRexRig(t, nil, 800, 800)
	p := r.p
	p.rex.bosses = nil
	rex := r.rex()
	rex.rexRageTimer = 1000
	startX := rex.x
	windup, airborne, landed := 0, 0, 0
	maxLift := 0.0
	for frame := 0; frame < 400 && landed == 0; frame++ {
		p.updateZombies()
		rex = r.rex()
		if rex.x != startX {
			t.Fatalf("frame %d: rex walked while held", frame)
		}
		switch {
		case len(p.rex.waves) > 0:
			landed = frame
		case p.rexLeaping(rexTestID):
			airborne++
			maxLift = math.Max(maxLift, p.rexLift(*rex))
		default:
			if airborne == 0 {
				windup++
				if p.rexLift(*rex) != 0 {
					t.Fatal("rex rose during the windup")
				}
			}
		}
	}
	if landed == 0 {
		t.Fatal("rex never landed")
	}
	if windup < 59 || windup > 61 {
		t.Fatalf("windup lasted %d ticks, want about 60 (1000 ms)", windup)
	}
	// v0 3.5, g 6: apex ~1.02 * render size 128.
	if maxLift < 120 || maxLift > 140 {
		t.Fatalf("peak lift %.1f, want about 130", maxLift)
	}
	if airborne < 60 || airborne > 90 {
		t.Fatalf("leap lasted %d ticks", airborne)
	}
	wave := p.rex.waves[0]
	if wave.x != 800 || wave.y != 800 || wave.owner != rexTestID {
		t.Fatalf("wave %+v not at the rex", wave)
	}
	if !hasSFX(p, "SFX_GRENADE_EXPLODE") || !p.shake.active() || p.shake.total != 1.5 {
		t.Fatalf("landing lacks the explosion sound / camera shake: %v", p.sfxQueue)
	}
	if p.rexLeaping(rexTestID) || p.rexLift(*r.rex()) != 0 {
		t.Fatal("rex still airborne after landing")
	}
	// The rex survives its own wave and walks again afterwards.
	for frame := 0; frame < 70; frame++ {
		p.updateZombies()
	}
	if r.rex().health != 25000 || r.rex().x >= startX {
		t.Fatalf("rex health %v x %v: should be unharmed and chasing the player again", r.rex().health, r.rex().x)
	}
	if len(p.rex.waves) != 0 {
		t.Fatalf("wave outlived its 1 s life: %+v", p.rex.waves)
	}
}

func TestRexShockwaveHurtsButDoesNotPushThePlayer(t *testing.T) {
	r := newRexRig(t, nil, 500, 500)
	p := r.p
	p.x, p.y = 500, 540
	p.spawnRexShockwave(500, 500, rexTestID)
	for frame := 0; frame < 12; frame++ {
		p.updateRexShockwaves()
	}
	// The wave reaches the player (19.2 + size*.5 > 40/.667 = 60) at tick 9
	// (size 90): ticks 9..12 hurt.
	if want := 1 - 4*0.025; math.Abs(p.health-want) > 1e-9 {
		t.Fatalf("health %v, want %v (0.025 per overlapping tick)", p.health, want)
	}
	// The native velocity add is overwritten by the player update before use.
	if p.y != 540 || p.x != 500 {
		t.Fatalf("player moved to %v,%v; the knockback is a native no-op", p.x, p.y)
	}
	if p.hurt <= 0 {
		t.Fatal("player not flagged as hurt")
	}
	// A shielded player takes no damage.
	p.health, p.achieve.shieldTimer = 1, 10
	p.updateRexShockwaves()
	if p.health != 1 {
		t.Fatalf("shield ignored: %v", p.health)
	}
	// Standing in the wave for its whole life is lethal; running to 330 px is safe.
	p.achieve.shieldTimer = 0
	p.x, p.y = 500, 500
	for frame := 0; frame < 70; frame++ {
		p.updateRexShockwaves()
	}
	if p.health > 0 {
		t.Fatalf("player at the wave centre survived with %v", p.health)
	}
	r2 := newRexRig(t, nil, 500, 500)
	r2.p.x, r2.p.y = 500+330, 500
	r2.p.spawnRexShockwave(500, 500, rexTestID)
	for frame := 0; frame < 70; frame++ {
		r2.p.updateRexShockwaves()
	}
	if r2.p.health != 1 {
		t.Fatalf("player outside the 319 px reach was hurt: %v", r2.p.health)
	}
}

func TestRexShockwaveKillsZombiesInReachAndCreditsTheRex(t *testing.T) {
	r := newRexRig(t, nil, 500, 500)
	p := r.p
	for i := 0; i < 5; i++ {
		r.stack(1, 500+float64(i)*2, 500)
	}
	r.stack(1, 500+250, 500) // inside the full reach, but only late in the wave
	r.stack(1, 500+400, 500) // never reached
	invulnerable := zombieState{x: 510, y: 510, health: 1, size: formats.Vec2{X: 32, Y: 32}, invulnerable: true}
	p.zombies = append(p.zombies, invulnerable)
	p.spawnRexShockwave(500, 500, rexTestID)
	p.updateRexShockwaves()
	dead := func() (n int) {
		for _, z := range p.zombies {
			if z.dying {
				n++
			}
		}
		return
	}
	if dead() != 5 {
		t.Fatalf("%d zombies died on the first tick, want the 5 beside the rex", dead())
	}
	if p.achieve.best["t_rex"] != 5 {
		t.Fatalf("t_rex credit %d, want 5", p.achieve.best["t_rex"])
	}
	for frame := 0; frame < 70; frame++ {
		p.updateRexShockwaves()
	}
	if dead() != 6 {
		t.Fatalf("%d dead at the end, want 6 (the far zombie at +250 dies late, +400 never)", dead())
	}
	if r.rex().health != 25000 || r.rex().dying {
		t.Fatal("the rex hurt itself")
	}
	if p.zombies[len(p.zombies)-1].health != 1 {
		t.Fatal("invulnerable zombie lost health")
	}
	if p.achieve.best["t_rex"] != 6 {
		t.Fatalf("t_rex credit %d, want 6", p.achieve.best["t_rex"])
	}
}

func TestRexShockwaveDamageIsPerTickAndPerGridCell(t *testing.T) {
	// Native grid: 16 px cells, every entity registered over +-19.2 around its
	// position (entity_grid.go).
	var reg entityGridRegistration
	reg.register(512, 512)
	if got := reg.cells; got != (entityGridRange{30, 33, 30, 33}) {
		t.Fatalf("registration at (512,512) = %+v, want cells 30..33 on both axes", got)
	}
	big := entityGridBox(512, 512, 150)
	if got := reg.visits(big); got != 16 {
		t.Fatalf("a zombie in a 4x4 cell block is listed %d times, want 16", got)
	}
	var mid entityGridRegistration
	mid.register(520, 520)
	if got := mid.cells; got != (entityGridRange{31, 33, 31, 33}) {
		t.Fatalf("registration at (520,520) = %+v", got)
	}
	if got := mid.visits(entityGridBox(520, 520, 5)); got != 1 {
		t.Fatalf("a 10 px query hits one cell, got %d", got)
	}
	// The cell list is rebuilt only when the centre cell changes (FUN_00091918).
	mid.register(527, 520)
	if mid.cells != (entityGridRange{31, 33, 31, 33}) {
		t.Fatal("registration changed without a centre-cell change")
	}
	mid.register(540, 520)
	if mid.cells == (entityGridRange{31, 33, 31, 33}) {
		t.Fatal("registration did not follow the entity into the next cell")
	}
	r := newRexRig(t, nil, 500, 500)
	p := r.p
	p.zombies = append(p.zombies, zombieState{x: 512, y: 512, health: 100, size: formats.Vec2{X: 48, Y: 48}}, zombieState{x: 520, y: 520, health: 100, size: formats.Vec2{X: 48, Y: 48}})
	p.spawnRexShockwave(512, 512, rexTestID)
	// Age the wave to 0.5 s: size 300, query box +-150 covers both lists.
	p.rex.waves[0].age = .5 - 1.0/60
	p.updateRexShockwaves()
	if got := p.zombies[1].health; got != 100-16 {
		t.Fatalf("4x4 zombie health %v after one tick, want 84 (1 damage x 16 cells)", got)
	}
	if got := p.zombies[2].health; got != 100-9 {
		t.Fatalf("3x3 zombie health %v after one tick, want 91", got)
	}
}

func TestRexShockwaveIsRemovedInsideAWallTile(t *testing.T) {
	r := newRexRig(t, nil, 500, 500)
	p := r.p
	if p.world == nil || p.tileSize != 32 {
		t.Skip("rig has no 32 px level")
	}
	wallX, wallY := -1, -1
	for y := 0; y < p.world.Level.Height && wallX < 0; y++ {
		for x := 0; x < p.world.Level.Width; x++ {
			if p.collisionValue(x, y) == 1 {
				wallX, wallY = x, y
				break
			}
		}
	}
	if wallX < 0 {
		t.Skip("level has no wall tile")
	}
	p.spawnRexShockwave(float64(wallX*32+16), float64(wallY*32+16), rexTestID)
	p.updateRexShockwaves()
	if len(p.rex.waves) != 0 {
		t.Fatal("a wave centred in a wall tile must be removed (FUN_000a5f74 tile == 1)")
	}
}

func TestMakeRexRageIsIgnoredWhileLeaping(t *testing.T) {
	h := script125CachedHost(t, "world0_level2")
	p := h.play
	p.scriptRuntime.Close()
	p.scriptRuntime = nil
	p.scriptEntities[rexTestID] = &scriptEntity{id: rexTestID, kind: "zombie", entityType: "boss_rex"}
	p.zombies = append(p.zombies, zombieState{x: 400, y: 400, health: 25000, size: formats.Vec2{X: 128, Y: 128}, scriptID: rexTestID})
	index := len(p.zombies) - 1
	p.rex.bosses = map[int]*rexBossState{rexTestID: {leaping: true}}
	if _, err := h.Call("MakeRexRage", nil); err != nil {
		t.Fatal(err)
	}
	if p.zombies[index].rexRageTimer != 0 {
		t.Fatal("MakeRexRage re-armed a leaping rex")
	}
	p.rex.bosses[rexTestID].leaping = false
	if _, err := h.Call("MakeRexRage", nil); err != nil {
		t.Fatal(err)
	}
	if p.zombies[index].rexRageTimer != 1000 {
		t.Fatalf("MakeRexRage timer %v, want 1000", p.zombies[index].rexRageTimer)
	}
}

// CLEVER GIRL: the rex leaps into a crowd and its single shockwave kills 10.
func TestCleverGirlUnlocksFromARealShockwave(t *testing.T) {
	roots := achievementCaches()
	if len(roots) == 0 {
		t.Skip("original asset cache unavailable")
	}
	for _, root := range roots {
		pack, err := content.NewPack(content.NewSource(root))
		if err != nil {
			t.Fatal(err)
		}
		catalog, err := pack.Achievements()
		if err != nil {
			t.Fatal(err)
		}
		var entry formats.Achievement
		for _, candidate := range catalog {
			if strings.EqualFold(candidate.SpecificType, "t_rex") {
				entry = candidate
			}
		}
		if entry.Name != "CLEVER GIRL" || entry.Total != 10 {
			t.Fatalf("%s: unexpected catalog entry %+v", root, entry)
		}
		t.Run(root, func(t *testing.T) {
			r := newRexRig(t, formats.AchievementCatalog{entry}, 800, 800)
			p := r.p
			r.rex().rexRageTimer = rexRageMillis
			for i := 0; i < entry.Total-1; i++ {
				r.stack(1, 800+float64(i%5)*12-24, 840+float64(i/5)*12)
			}
			step := func() {
				p.updateZombies()
				p.updateBulletsAndKills()
				if err := r.app.updateGameplayAchievements(p); err != nil {
					t.Fatal(err)
				}
			}
			for frame := 0; frame < 400 && len(p.rex.waves) == 0; frame++ {
				step()
			}
			if len(p.rex.waves) == 0 {
				t.Fatal("rex never landed")
			}
			for frame := 0; frame < 70; frame++ {
				step()
			}
			if r.unlocked(entry.ID) || p.achieve.best["t_rex"] != 9 {
				t.Fatalf("nine kills must not unlock it: best=%v unlocked=%v", p.achieve.best["t_rex"], r.unlocked(entry.ID))
			}
			// A second leap with ten zombies in reach unlocks it. The nine from
			// before were removed by the kill pass, so the count restarts per wave.
			p.zombies = p.zombies[:1]
			for i := 0; i < entry.Total; i++ {
				r.stack(1, 800+float64(i%5)*12-24, 840+float64(i/5)*12)
			}
			r.rex().rexRageTimer = rexRageMillis
			r.rex().x, r.rex().y = 800, 800 // it chased the player after the first landing
			for frame := 0; frame < 500 && !r.unlocked(entry.ID); frame++ {
				step()
			}
			if !r.unlocked(entry.ID) {
				t.Fatalf("CLEVER GIRL not unlocked after a shockwave killed ten: best=%v", p.achieve.best)
			}
		})
	}
}
