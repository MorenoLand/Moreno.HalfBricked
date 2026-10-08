package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/stats"
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
)

func explodingRig() (*app, *playState) {
	a := &app{statistics: stats.NewStatsData()}
	p := &playState{
		world: &viewer.Viewer{Level: formats.Level{Width: 64, Height: 64, Layers: map[formats.LayerKind][]uint32{formats.LayerC: openLayer(64 * 64)}}},
		x:     1200, y: 1200, tileSize: 32, health: 1, maxHealth: 1, lives: 3, hudVisible: true, multiplier: 1,
		scriptEntities: map[int]*scriptEntity{}, scriptNextEntity: 1,
	}
	a.play = p
	return a, p
}

// addZombie places a 100 health zombie; kind "exploding_zombie" gives it the type-4 behaviour.
func addZombie(p *playState, kind string, x, y float64) int {
	id := p.scriptNextEntity
	p.scriptNextEntity++
	p.scriptEntities[id] = &scriptEntity{id: id, kind: "zombie", entityType: kind}
	p.zombies = append(p.zombies, zombieState{x: x, y: y, health: 100, rawPoints: 100, size: formats.Vec2{X: 48, Y: 48}, scriptID: id, alpha: 1})
	return len(p.zombies) - 1
}

func killNow(p *playState, index int) {
	p.zombies[index].health = 0
	p.zombies[index].dying = true
	p.zombies[index].deathAge = 0
}

// tickWorld runs the parts of a frame that matter for zombies and blasts.
func tickWorld(p *playState, frames int) {
	for i := 0; i < frames; i++ {
		p.updateZombies()
		p.updateExplosions()
		p.updateBulletsAndKills()
	}
}

func TestZombieBlastRadiusFollowsNativeShrink(t *testing.T) {
	if got := zombieBlastRadiusAt(0); got != 160 {
		t.Fatalf("radius at 0 = %v, want 160", got)
	}
	if got, want := zombieBlastRadiusAt(.1), 160-.1/.75*80; math.Abs(got-want) > .01 {
		t.Fatalf("radius at .1 = %v, want %v", got, want)
	}
	if got := zombieBlastRadiusAt(.15); got != 0 {
		t.Fatalf("radius at .15 = %v, want 0", got)
	}
	if got := zombieBlastRadiusAt(.5); got != 0 {
		t.Fatalf("radius at .5 = %v, want 0", got)
	}
}

func TestExplodingZombieDetonatesAtDeathTransition(t *testing.T) {
	_, p := explodingRig()
	exploder := addZombie(p, explodingZombieType, 400, 400)
	plain := addZombie(p, "zombie", 900, 900)
	killNow(p, exploder)
	killNow(p, plain)
	if got, want := p.zombieDeathDelayFor(p.zombies[exploder]), zombieDeathDelay/2; got != want {
		t.Fatalf("exploding death delay = %v, want half of the shared delay %v", got, want)
	}
	tickWorld(p, 1)
	if len(p.zombieBlasts) != 0 {
		t.Fatal("blast fired before the (halved) death delay elapsed")
	}
	tickWorld(p, 3)
	if len(p.zombieBlasts) != 1 {
		t.Fatalf("blasts after delay = %d, want exactly one", len(p.zombieBlasts))
	}
	if blast := p.zombieBlasts[0]; blast.x != 400 || blast.y != 400 {
		t.Fatalf("blast at (%v,%v), want the zombie's position", blast.x, blast.y)
	}
	if len(p.explosions) != 1 {
		t.Fatalf("explosion effects = %d, want 1", len(p.explosions))
	}
	sound := false
	for _, name := range p.sfxQueue {
		sound = sound || name == zombieBlastSound
	}
	if !sound {
		t.Fatalf("blast sound %q not queued: %v", zombieBlastSound, p.sfxQueue)
	}
	// The plain zombie's longer delay has also passed by now and must not blast.
	tickWorld(p, 10)
	if len(p.zombies) != 0 {
		t.Fatalf("zombies left = %d", len(p.zombies))
	}
	if p.levelKills != 2 {
		t.Fatalf("kills = %d, want both removed zombies counted", p.levelKills)
	}
	if got := countBlastsEver(p); got != 1 {
		t.Fatalf("blasts ever = %d, want one (only the exploding zombie)", got)
	}
}

// countBlastsEver is the number of blast shot ids handed out.
func countBlastsEver(p *playState) int { return p.achieve.shotSerial }

func TestZombieBlastKillsNeighboursAndCreditsThem(t *testing.T) {
	_, p := explodingRig()
	exploder := addZombie(p, explodingZombieType, 400, 400)
	near := addZombie(p, "zombie", 405, 402)
	far := addZombie(p, "zombie", 400+300, 400)
	killNow(p, exploder)
	scoreBefore := p.score
	tickWorld(p, 90)
	if p.zombies[0].scriptID == 0 {
		t.Fatal("unexpected state")
	}
	remaining := map[int]bool{}
	for _, z := range p.zombies {
		remaining[z.scriptID] = true
	}
	_ = near
	_ = far
	if remaining[2] {
		t.Fatal("zombie at the blast centre survived")
	}
	if !remaining[3] {
		t.Fatal("zombie 300px away was removed")
	}
	if p.achieve.best["exp_zombie"] != 1 {
		t.Fatalf("exp_zombie scope = %d, want 1 blast kill", p.achieve.best["exp_zombie"])
	}
	if p.combatKillCount != 1 {
		t.Fatalf("combat kills = %d, want 1", p.combatKillCount)
	}
	if p.levelKills != 2 {
		t.Fatalf("level kills = %d, want exploder plus the blast victim", p.levelKills)
	}
	if p.score <= scoreBefore {
		t.Fatalf("score did not rise: %d", p.score)
	}
}

func TestZombieBlastHurtsPlayerInsideOnly(t *testing.T) {
	_, p := explodingRig()
	p.x, p.y = 400, 420
	addZombie(p, explodingZombieType, 400, 400)
	killNow(p, 0)
	tickWorld(p, 60)
	// Player-side reach is .5*radius + .3*48 = ~94 early on; 20px away is inside.
	if p.health >= 1 {
		t.Fatal("player next to the blast took no damage")
	}
	want := 1 - 3.0/60*float64(zombieBlastWindowTicks(0, 20))
	if math.Abs(p.health-want) > .02 {
		t.Fatalf("health %.3f, want about %.3f (3 health/s for the shrinking-radius window)", p.health, want)
	}

	_, safe := explodingRig()
	safe.x, safe.y = 400+500, 400
	addZombie(safe, explodingZombieType, 400, 400)
	killNow(safe, 0)
	tickWorld(safe, 60)
	if safe.health != 1 {
		t.Fatalf("distant player lost health: %v", safe.health)
	}
}

// zombieBlastWindowTicks counts the ticks a target at (dx,dy) from the blast is inside the player-side reach.
func zombieBlastWindowTicks(dx, dy float64) int {
	count := 0
	for tick := 1; float64(tick)/60 <= zombieBlastDuration; tick++ {
		radius := zombieBlastRadiusAt(float64(tick) / 60)
		if zombieBlastEllipse(dx, dy, zombieBlastBodyHalf+radius*zombieBlastPlayerScale) {
			count++
		}
	}
	return count
}

func TestExplodingZombieChainReaction(t *testing.T) {
	_, p := explodingRig()
	first := addZombie(p, explodingZombieType, 400, 400)
	addZombie(p, explodingZombieType, 410, 405)
	killNow(p, first)
	tickWorld(p, 200)
	if got := countBlastsEver(p); got != 2 {
		t.Fatalf("blasts = %d, want the first blast to set off the second exploding zombie", got)
	}
	if p.levelKills != 2 {
		t.Fatalf("kills = %d, want both exploding zombies", p.levelKills)
	}
}

func TestCollateralDamageUnlocksFromRealBlast(t *testing.T) {
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
			if candidate.SpecificType == "exp_zombie" {
				entry = candidate
			}
		}
		if entry.ID == "" || entry.Total != 10 {
			t.Fatalf("%s: COLLATERAL DAMAGE entry missing or changed: %+v", root, entry)
		}
		if !achievementTracked(entry) {
			t.Fatalf("%s: %s must be displayed as tracked", root, entry.Name)
		}
		a, p := explodingRig()
		a.achievements = formats.AchievementCatalog{entry}
		p.statistics = &a.statistics
		exploder := addZombie(p, explodingZombieType, 400, 400)
		// Nine victims is one short; the tenth arrives with the exploder's neighbour below.
		for i := 0; i < 9; i++ {
			addZombie(p, "zombie", 400+float64(i%3)*3, 400+float64(i/3)*3)
		}
		killNow(p, exploder)
		for i := 0; i < 120; i++ {
			tickWorld(p, 1)
			if err := a.updateGameplayAchievements(p); err != nil {
				t.Fatal(err)
			}
		}
		if p.achieve.best["exp_zombie"] != 9 || a.achievementUnlocks[entry.ID] {
			t.Fatalf("%s: nine blast kills must not unlock yet (best %d, unlocked %v)", root, p.achieve.best["exp_zombie"], a.achievementUnlocks[entry.ID])
		}
		// A second, separate blast does not add to the first blast's tally.
		extra := addZombie(p, explodingZombieType, 900, 900)
		addZombie(p, "zombie", 902, 901)
		killNow(p, extra)
		for i := 0; i < 120; i++ {
			tickWorld(p, 1)
			if err := a.updateGameplayAchievements(p); err != nil {
				t.Fatal(err)
			}
		}
		if a.achievementUnlocks[entry.ID] {
			t.Fatalf("%s: kills from two blasts were summed", root)
		}
		// Ten victims of one blast unlock it.
		_, q := explodingRig()
		a.play, q.statistics = q, &a.statistics
		boom := addZombie(q, explodingZombieType, 400, 400)
		for i := 0; i < 10; i++ {
			addZombie(q, "zombie", 400+float64(i%3)*3, 400+float64(i/3)*3)
		}
		killNow(q, boom)
		for i := 0; i < 120; i++ {
			tickWorld(q, 1)
			if err := a.updateGameplayAchievements(q); err != nil {
				t.Fatal(err)
			}
		}
		if !a.achievementUnlocks[entry.ID] {
			t.Fatalf("%s: ten blast kills did not unlock COLLATERAL DAMAGE (best %d)", root, q.achieve.best["exp_zombie"])
		}
	}
}

func TestShootingAnExplodingZombieIsNotACollateralKill(t *testing.T) {
	_, p := explodingRig()
	p.creditKill(killOrigin{gun: "PISTOL", shot: 1})
	if p.achieve.best["exp_zombie"] != 0 {
		t.Fatal("a pistol kill was counted for the exploding-zombie scope")
	}
}

func TestExplodingZombieSpawnsFromEveryCacheLevel(t *testing.T) {
	roots := achievementCaches()
	if len(roots) == 0 {
		t.Skip("original asset cache unavailable")
	}
	for _, root := range roots {
		pack, err := content.NewPack(content.NewSource(root))
		if err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, info := range pack.List() {
			level, err := pack.Load(info.ID)
			if err != nil {
				continue
			}
			for _, wave := range level.Waves {
				for _, spawner := range wave.Spawners {
					for _, entry := range spawner.Types {
						if entry.Name != explodingZombieType {
							continue
						}
						_, p := explodingRig()
						p.spawnZombieAt(entry, formats.Vec2{X: 300, Y: 300})
						if len(p.zombies) != 1 || !p.isExplodingZombie(p.zombies[0]) {
							t.Fatalf("%s %s: spawned entry is not an exploding zombie", root, info.ID)
						}
						if got := p.zombies[0].health; got != 100 {
							t.Fatalf("%s %s: exploding zombie health %v, want strength 100", root, info.ID, got)
						}
						found++
					}
				}
			}
		}
		if found == 0 {
			t.Fatalf("%s: no exploding_zombie spawn entries found", root)
		}
		t.Logf("%s: %d exploding_zombie spawn entries spawn as type-4 zombies", root, found)
	}
}

func TestExplodingGlowPulsesWithinNativeAlphaBand(t *testing.T) {
	// Scene update FUN_000867e4: phase16 += uint(dt*36400); alpha = uint(70 + 50*sin(phase)).
	if got := explodingGlowPhase(1); got != 606 {
		t.Fatalf("phase after one 1/60 s frame = %d, want 606", got)
	}
	lo, hi := 1.0, 0.0
	for i := 0; i < 200; i++ {
		alpha := explodingGlowAlpha(float64(i) / 100)
		lo, hi = math.Min(lo, alpha), math.Max(hi, alpha)
	}
	if lo < 20.0/255-.01 || lo > 24.0/255 || hi > 120.0/255+.01 || hi < 116.0/255 {
		t.Fatalf("glow alpha band %.3f..%.3f, want 20..120 over 255", lo*255, hi*255)
	}
}

func TestEntityGridMatchesNativeSetup(t *testing.T) {
	// FUN_0009145c(grid, 16, 16): 2240/16 x 1216/16 cells.
	if entityGridCell != 16 || entityGridCols != 140 || entityGridRows != 76 {
		t.Fatal("grid geometry differs from the native constants")
	}
	// Registration extent 0.3 * 64 (player +0x28), whatever the entity size.
	if math.Abs(float64(entityGridRegisterExtent)-19.2) > 1e-5 {
		t.Fatalf("registration extent %v, want 19.2", entityGridRegisterExtent)
	}
	// Clamps: below one cell -> 0, beyond the last cell -> cols-1.
	if got := entityGridBox(2, 2, 19.2); got != (entityGridRange{0, 1, 0, 1}) {
		t.Fatalf("box at the origin = %+v", got)
	}
	if got := entityGridBox(5000, 5000, 19.2); got != (entityGridRange{139, 139, 75, 75}) {
		t.Fatalf("box beyond the map = %+v", got)
	}
}
