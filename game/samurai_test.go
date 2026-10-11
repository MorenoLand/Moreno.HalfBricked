package game

import (
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/stats"
)

// samuraiRig spawns the Japan boss on an open level. The player stands 250 px to its left.
func samuraiRig() (*playState, *zombieState) {
	p := modelRig()
	p.zombieWeapons = formats.ZombieWeaponCatalog{{GunType: "CURVER", RecoilSeconds: 1, RateOfFire: 1, SpreadUnits: 17, Ammo: 6, AmmoPerShot: 1, Life: 1.7, Speed: 130, BulletType: "SPINNER"}}
	entry := formats.SpawnType{Name: "boss_samurai", Chance: 1, Speed: formats.Vec2{X: 130, Y: 130}, Strength: 20000, Size: formats.Vec2{X: 55, Y: 55}, TurnSpeed: 12, Texture: "samurai"}
	p.spawnZombieAt(entry, formats.Vec2{X: 800, Y: 800})
	z := &p.zombies[0]
	z.native.ai.state = 1
	z.size = formats.Vec2{X: zombieBodyWidth(z.native.sizeZ), Y: z.native.sizeZ}
	z.alpha = 1
	p.x, p.y = z.x-250, z.y
	p.health, p.maxHealth = 100, 100
	return p, z
}

// forceRoll advances the shared generator until the next rnd(10) returns want.
func forceRoll(t *testing.T, p *playState, want uint32) {
	t.Helper()
	for i := 0; i < 10000; i++ {
		probe := *p.rng
		if probe.Bounded(10) == want {
			return
		}
		p.rng.Bounded(10)
	}
	t.Fatalf("no roll of %d", want)
}

// enterState5 puts the boss in state 5 on the frame the AI notices it (previous state 1) and runs that frame.
func enterState5(t *testing.T, p *playState, z *zombieState, roll uint32) {
	t.Helper()
	forceRoll(t, p, roll)
	z.native.ai.state = 5
	z.native.samurai.prevState = 1
	z.native.ai.speedFactor = 4
	p.samuraiAI(z, p.nearestPlayer(z.x, z.y), 1.0/60)
}

// The boss is built from the spawn record: kind 13, 87.7 x 110 body (size 55 is the half size), 20000 health,
// turn factor 12/120, three blades loaded and the range sight^2 * 5.
func TestSamuraiSpawnSizeAndKind(t *testing.T) {
	p := modelRig()
	p.zombieWeapons = formats.ZombieWeaponCatalog{{GunType: "CURVER", RecoilSeconds: 1, RateOfFire: 1, SpreadUnits: 17, Ammo: 6, AmmoPerShot: 1, Life: 1.7, Speed: 130, BulletType: "SPINNER"}}
	entry := formats.SpawnType{Name: "boss_samurai", Chance: 1, Speed: formats.Vec2{X: 130, Y: 130}, Strength: 20000, Size: formats.Vec2{X: 55, Y: 55}, TurnSpeed: 12, Texture: "samurai"}
	p.spawnZombieAt(entry, formats.Vec2{X: 800, Y: 800})
	z := &p.zombies[0]
	if z.native.kind != zombieKindSamurai || !z.native.samurai.active {
		t.Fatalf("kind %d active %v, want the samurai class", z.native.kind, z.native.samurai.active)
	}
	if z.native.sizeZ != 110 || z.health != 20000 || z.native.maxHealth != 20000 || z.speed != 130 {
		t.Fatalf("sizeZ %v health %v speed %v, want 110 / 20000 / 130", z.native.sizeZ, z.health, z.speed)
	}
	if got := z.native.ai.turn; math.Abs(got-.1) > 1e-6 {
		t.Fatalf("turn %v, want 0.1", got)
	}
	if z.native.ai.state != 0 {
		t.Fatalf("state %d, want 0 (rising)", z.native.ai.state)
	}
	gun := z.native.ai.gun
	if gun.rounds != 3 || gun.speed != 250 || gun.interval != .2 || gun.spread != 0 {
		t.Fatalf("gun %+v, want 3 rounds, speed 250, interval 0.2, spread 0", gun)
	}
	if want := z.native.ai.sightBase * 5; z.native.samurai.rangeSq != want {
		t.Fatalf("range %v, want sight^2 * 5 = %v", z.native.samurai.rangeSq, want)
	}
	for frame := 0; frame < 200 && z.native.ai.state == 0; frame++ {
		p.updateZombies()
	}
	if z.native.ai.state == 0 {
		t.Fatal("the rise never ended")
	}
	if math.Abs(z.size.X-87.65625) > .01 || z.size.Y != 110 {
		t.Fatalf("body %.2f x %.2f, want 87.66 x 110", z.size.X, z.size.Y)
	}
}

// Roll 0..1 is the plain lunge: phase 0, no special timer. The state machine then rushes at 4.0 and decays to 1.
func TestSamuraiLungeRoll(t *testing.T) {
	p, z := samuraiRig()
	enterState5(t, p, z, 1)
	s := &z.native.samurai
	if s.phase != 0 || s.special != 0 {
		t.Fatalf("phase %d special %v, want a plain lunge", s.phase, s.special)
	}
	if z.native.ai.state != 5 || z.animation != samuraiAnimAttack {
		t.Fatalf("state %d animation %q, want state 5 with the Attack sheet", z.native.ai.state, z.animation)
	}
	z.native.ai.speedFactor = 4
	for frame := 0; frame < 120 && z.native.ai.state == 5; frame++ {
		p.samuraiStateMachine(z, 1.0/60)
	}
	if z.native.ai.state != 1 || z.native.ai.speedFactor != 1 {
		t.Fatalf("after the lunge state %d factor %v, want 1 / 1", z.native.ai.state, z.native.ai.speedFactor)
	}
}

// Roll 2..4 is the teleport: the boss fades out over 1.33 s, then reappears 200..300 px from the player facing him
// with factor 5.0.
func TestSamuraiTeleportRoll(t *testing.T) {
	p, z := samuraiRig()
	enterState5(t, p, z, 3)
	s := &z.native.samurai
	if s.phase != 1 || s.special != 1 {
		t.Fatalf("phase %d special %v, want the teleport", s.phase, s.special)
	}
	player := p.nearestPlayer(z.x, z.y)
	for frame := 0; frame < 200 && s.special > 0; frame++ {
		p.samuraiAI(z, player, 1.0/60)
	}
	if s.special != 0 || z.native.ai.speedFactor != 5 {
		t.Fatalf("special %v factor %v, want 0 / 5", s.special, z.native.ai.speedFactor)
	}
	if d := math.Hypot(z.x-p.x, z.y-p.y); d < 200 || d > 300.01 {
		t.Fatalf("reappeared %.1f px from the player, want 200..300", d)
	}
	if z.alpha != 0 {
		t.Fatalf("alpha %v on the frame it appears, want 0 (it ramps up afterwards)", z.alpha)
	}
	p.samuraiAI(z, player, 1.0/60)
	if got := samuraiAlphaByte(z.alpha); got != 0x10 {
		t.Fatalf("alpha byte %#x one frame later, want 0x10", got)
	}
}

// Roll 5..7 is the spin: 3 s timer, factor = timer + 1.1, the spin heading advances and draws instead of the
// facing.
func TestSamuraiSpinRoll(t *testing.T) {
	p, z := samuraiRig()
	z.native.facing = 1000
	enterState5(t, p, z, 6)
	s := &z.native.samurai
	if s.phase != 2 || s.special <= 0 {
		t.Fatalf("phase %d special %v, want the spin", s.phase, s.special)
	}
	if s.spin < 1000 {
		t.Fatalf("spin heading %d did not start at the facing", s.spin)
	}
	if z.samuraiDrawHeading() != s.spin {
		t.Fatal("the spin must draw with its own heading")
	}
	player := p.nearestPlayer(z.x, z.y)
	before := s.special
	p.samuraiAI(z, player, 1.0/60)
	if s.special >= before {
		t.Fatal("the spin timer did not run")
	}
	if want := s.special + 1.1; math.Abs(z.native.ai.speedFactor-want) > 1e-9 {
		t.Fatalf("factor %v, want timer + 1.1 = %v", z.native.ai.speedFactor, want)
	}
	frames := 0
	for ; s.special > 0 && frames < 400; frames++ {
		p.samuraiAI(z, player, 1.0/60)
	}
	if frames < 170 || frames > 190 {
		t.Fatalf("the spin lasted %d frames, want about 180 (3 s)", frames)
	}
}

// Roll 8..9 sets phase 4 (5 - 0.95 * health fraction, truncated); when state 5 ends the boss re-aims at the player,
// dashes at 4.0 and spends one phase step; the gun is disabled meanwhile.
func TestSamuraiDashChainRoll(t *testing.T) {
	p, z := samuraiRig()
	enterState5(t, p, z, 9)
	s := &z.native.samurai
	if s.phase != 4 {
		t.Fatalf("phase %d, want 4", s.phase)
	}
	if !p.samuraiCannotFire(z) {
		t.Fatal("phase above 3 must disable the gun")
	}
	z.health = z.native.maxHealth / 3
	enterState5(t, p, z, 9)
	if s.phase != 4 {
		t.Fatalf("phase %d at one third health, want 4", s.phase)
	}
	// State 5 over: the next AI call sees state 1 with the previous state 5.
	z.native.ai.state = 1
	z.native.facing = 0
	p.samuraiAI(z, p.nearestPlayer(z.x, z.y), 1.0/60)
	if z.native.ai.state != 5 || z.native.ai.speedFactor != 4 || s.phase != 3 {
		t.Fatalf("state %d factor %v phase %d, want a new dash (5 / 4 / 3)", z.native.ai.state, z.native.ai.speedFactor, s.phase)
	}
	want := uint16(0x8000) // the player is straight to the left
	if d := absInt(angleDifference(z.native.facing, want)); d > 0x200 {
		t.Fatalf("facing %#x, want toward the player (%#x)", z.native.facing, want)
	}
	if p.samuraiCannotFire(z) && z.native.ai.gun.rounds > 0 {
		t.Fatal("phase 3 re-enables the gun")
	}
}

// The volley is three shots at 0.2 s spacing; once the rounds are gone the boss waits 0.95 s and lunges; the gun
// reloads 5 s after it emptied.
func TestSamuraiVolleyThenLungeThenReload(t *testing.T) {
	p, z := samuraiRig()
	ai := &z.native.ai
	ai.state = 3
	ai.gun.timer = 1
	z.native.facing = 0x8000 // the player is to the left
	ai.desired = 0x8000
	fired := 0
	frames := 0
	for ; frames < 600 && ai.state != 5; frames++ {
		before := ai.gun.rounds
		p.samuraiStateMachine(z, 1.0/60)
		if ai.gun.rounds < before {
			fired++
		}
	}
	if fired != 3 || ai.state != 5 || ai.speedFactor != 4 {
		t.Fatalf("fired %d state %d factor %v after %d frames, want 3 shots then the lunge", fired, ai.state, ai.speedFactor, frames)
	}
	s := &z.native.samurai
	s.prevRounds = 3
	ai.state = 1
	p.samuraiAI(z, p.nearestPlayer(z.x, z.y), 1.0/60)
	if s.reload <= 0 || s.reload > 5 {
		t.Fatalf("reload %v, want a 5 s countdown", s.reload)
	}
	for frame := 0; frame < 320 && ai.gun.rounds == 0; frame++ {
		p.samuraiAI(z, p.nearestPlayer(z.x, z.y), 1.0/60)
	}
	if ai.gun.rounds != 3 {
		t.Fatalf("rounds %d after the reload, want 3", ai.gun.rounds)
	}
}

// Touching the boss hurts at dt * 0.15 with no depth threshold, and in state 5 it reloads the gun.
func TestSamuraiContactDamage(t *testing.T) {
	p, z := samuraiRig()
	z.native.ai.gun.rounds = 0
	z.native.ai.state = 5
	// Barely inside the 0.3 * (width + 64) reach: the plain zombie would deal nothing here.
	reach := zombieBodyReachFactor*z.size.X + zombieBodyReachFactor*entityGridPlayerBody
	p.x, p.y = z.x+reach-.5, z.y
	prey := playerTarget{index: 0, x: p.x, y: p.y}
	p.zombieContact(z, prey, 1.0/60)
	if got, want := p.health, 100-1.0/60*.15; math.Abs(got-want) > 1e-9 {
		t.Fatalf("health %v, want %v", got, want)
	}
	if z.native.ai.gun.rounds != 3 {
		t.Fatalf("rounds %d, want the state 5 contact to reload", z.native.ai.gun.rounds)
	}
	p.health = 100
	z.native.ai.state = 1
	z.native.ai.gun.rounds = 0
	p.zombieContact(z, prey, 1.0/60)
	if p.health >= 100 || z.native.ai.gun.rounds != 0 {
		t.Fatalf("state 1 contact: health %v rounds %d, want damage without a reload", p.health, z.native.ai.gun.rounds)
	}
	z.native.ai.state = 0
	p.health = 100
	p.zombieContact(z, prey, 1.0/60)
	if p.health != 100 {
		t.Fatal("a rising boss must not hurt")
	}
}

// The boss is always alerted: far outside any 200..300 px alert radius it still chases.
func TestSamuraiAlwaysAggro(t *testing.T) {
	p, z := samuraiRig()
	p.x, p.y = z.x-450, z.y
	startX := z.x
	for frame := 0; frame < 60; frame++ {
		p.updateZombies()
	}
	if z.x >= startX-1 {
		t.Fatalf("boss at %.1f did not chase the distant player", z.x)
	}
}

// Dying: Death clip then fade without a blood pop; the wave list ends after 1.5 s; every living zombie of types 2..9
// takes 1000 damage through its normal death; portals close; the boss time becomes the best Japan time.
func TestSamuraiDeathCleanup(t *testing.T) {
	p, z := samuraiRig()
	data := stats.NewStatsData()
	p.statistics = &data
	p.world.Level.Waves = []formats.Wave{{}, {}}
	plain := formats.SpawnType{Name: "zombie", Chance: 1, Speed: formats.Vec2{X: 90, Y: 90}, Strength: 100, Size: formats.Vec2{X: 35, Y: 35}, TurnSpeed: 12, Texture: "cavezombie"}
	for i := 0; i < 2; i++ {
		p.spawnZombieAt(plain, formats.Vec2{X: float64(300 + 40*i), Y: 300})
		p.zombies[len(p.zombies)-1].native.ai.state = 1
	}
	z = &p.zombies[0] // the spawns may have moved the slice
	p.portals = append(p.portals, portalState{x: 10, y: 10, age: .5, size: 100, alpha: 255})
	z.native.samurai.time = 83.7
	z.health, z.dying = 0, true
	p.updateZombies()
	if !p.bossDefeated || !z.native.samurai.dead {
		t.Fatal("the boss death was not processed")
	}
	if p.statistics.BestJapanTime != 83 {
		t.Fatalf("best Japan time %d, want 83", p.statistics.BestJapanTime)
	}
	if p.statistics.BossesKilled != 1 {
		t.Fatalf("bosses killed %d, want 1", p.statistics.BossesKilled)
	}
	if got := 2 - p.portals[0].age; got > -0.6 {
		t.Fatalf("portal remaining %v, want it clamped to -0.6", got)
	}
	if z.animation != samuraiAnimDeath {
		t.Fatalf("animation %q, want Death", z.animation)
	}
	for _, other := range p.zombies[1:] {
		if !other.dying {
			t.Fatal("a regular zombie survived the boss death")
		}
	}
	if p.wavesFinished {
		t.Fatal("the waves end 1.5 s after the death, not at once")
	}
	for frame := 0; frame < 120 && !p.wavesFinished; frame++ {
		p.updateZombies()
	}
	if !p.wavesFinished {
		t.Fatal("the replacement wave never ended")
	}
	pops := len(p.bloodPops)
	alpha := z.alpha
	for frame := 0; frame < 60*8 && len(p.zombies) > 1; frame++ {
		p.updateZombies()
		p.updateBulletsAndKills()
	}
	// The two regular zombies pop and go; the boss body stays for good (native story mode never fades or removes it).
	if len(p.zombies) != 1 || !p.zombies[0].native.samurai.dead {
		t.Fatalf("%d zombies left, want only the samurai body", len(p.zombies))
	}
	for frame := 0; frame < 60*30; frame++ {
		p.updateZombies()
		p.updateBulletsAndKills()
	}
	body := p.zombies[0]
	if len(p.zombies) != 1 || body.alpha != alpha {
		t.Fatalf("body alpha %v (was %v) after 30 s, want it frozen and present", body.alpha, alpha)
	}
	if p.zombiesBlockExit() {
		t.Fatal("the dead samurai's body must not hold the level exit")
	}
	if got := len(p.bloodPops) - pops; got > 2 {
		t.Fatalf("%d pops, want at most the two regular zombies'", got)
	}
}

// A better stored time is kept; a worse one is replaced.
func TestSamuraiBestTimeKeepsTheLower(t *testing.T) {
	p, _ := samuraiRig()
	data := stats.NewStatsData()
	p.statistics = &data
	p.recordJapanBossTime(90.9)
	if data.BestJapanTime != 90 {
		t.Fatalf("first time %d, want 90", data.BestJapanTime)
	}
	p.recordJapanBossTime(120.2)
	if data.BestJapanTime != 90 {
		t.Fatalf("slower time replaced the best: %d", data.BestJapanTime)
	}
	p.recordJapanBossTime(60.1)
	if data.BestJapanTime != 60 {
		t.Fatalf("faster time %d, want 60", data.BestJapanTime)
	}
}

// One fire call of the boss makes one blade, draws exactly four random numbers (aim jitter x and y, then the setup's
// offset and side) and the blade starts on a circle that also holds the aim point.
func TestSamuraiBladeSetupGeometry(t *testing.T) {
	p, z := samuraiRig()
	ai := &z.native.ai
	ai.gun.timer = 1
	probe := *p.rng
	for i := 0; i < 3; i++ {
		probe.Bounded(0x7ffff)
	}
	probe.Bounded(2)
	p.zombieFire(z)
	if len(p.spinners) != 1 {
		t.Fatalf("%d blades, want 1", len(p.spinners))
	}
	if *p.rng != probe {
		t.Fatal("the fire call did not draw rnd, rnd, rnd, rnd(2) in that order")
	}
	b := p.spinners[0]
	if math.Abs(float64(b.w)-26.5) > 1e-4 {
		t.Fatalf("blade width %v, want 26.5", b.w)
	}
	// Shooter and aim point (the player, no velocity, spread 0) lie on the circle.
	if d := math.Hypot(float64(b.x-b.cx), float64(b.y-b.cy)); math.Abs(d-float64(b.radius)) > .01 {
		t.Fatalf("start %.2f from the centre, radius %v", d, b.radius)
	}
	if d := math.Hypot(p.x-float64(b.cx), p.y-float64(b.cy)); math.Abs(d-float64(b.radius)) > .01 {
		t.Fatalf("aim point %.2f from the centre, radius %v", d, b.radius)
	}
	period := 2 * math.Pi * float64(b.radius) / 250
	if math.Abs(float64(b.life)-.55*period) > .01 {
		t.Fatalf("life %v, want 0.55 * %v", b.life, period)
	}
	if math.Abs(math.Abs(float64(b.omega))-65536/period) > 1 {
		t.Fatalf("omega %v, want +-%v", b.omega, 65536/period)
	}
}

// A blade inside the 0.3 * 64 + 0.5 * 26.5 ellipse hurts once and is removed: 0.333 in v7, halved in 1.2.5.
func TestSamuraiBladeHitsOnce(t *testing.T) {
	for _, c := range []struct {
		build string
		want  float64
	}{{content.WaveBuildV7, .333}, {content.WaveBuild125, .333 * .5}} {
		p, _ := samuraiRig()
		p.world.Level.WaveBuild = c.build
		p.spinners = []spinnerBlade{{x: float32(p.x) + 20, y: float32(p.y), w: 26.5, life: 10, speed: 250, height: 100, radius: 1, cx: float32(p.x) + 20, cy: float32(p.y)}}
		before := p.health
		p.updateSpinners(1.0 / 60)
		if got := before - p.health; math.Abs(got-c.want) > 1e-6 {
			t.Fatalf("%s: damage %v, want %v", c.build, got, c.want)
		}
		if len(p.spinners) != 0 {
			t.Fatalf("%s: the blade survived its hit", c.build)
		}
	}
}

// A blade ends silently on its life, on a height below 0, and a clockwise blade whose angle reaches 0 sticks there
// (ARM unsigned conversion).
func TestSamuraiBladeEndsAndSticks(t *testing.T) {
	p, _ := samuraiRig()
	p.spinners = []spinnerBlade{
		{x: 5000, y: 5000, w: 26.5, life: .01, speed: 250, height: 100, radius: 10, omega: 100},
		{x: 5000, y: 5000, w: 26.5, life: 10, speed: 250, height: .001, fall: 100, radius: 10, omega: 100},
		{x: 5000, y: 5000, w: 26.5, life: 10, speed: 250, height: 100, radius: 10, omega: -60000, orbit: 100, cx: 5000, cy: 5000},
	}
	for i := 0; i < 5; i++ {
		p.updateSpinners(1.0 / 60)
	}
	if len(p.spinners) != 1 {
		t.Fatalf("%d blades left, want only the clockwise one", len(p.spinners))
	}
	if b := p.spinners[0]; b.orbit != 0 || math.Abs(float64(b.x)-5010) > .01 {
		t.Fatalf("orbit %d x %v, want it stuck at angle 0 (centre + radius)", b.orbit, b.x)
	}
	if p.health != 100 {
		t.Fatal("expired blades must not hurt")
	}
}

// The Death clip is ticked twice per frame, and the last frame stays.
func TestSamuraiDeathClipRunsAtDoubleSpeed(t *testing.T) {
	p, z := samuraiRig()
	z.health, z.dying = 0, true
	p.updateZombies() // starts the death
	if z.animation != samuraiAnimDeath {
		t.Fatalf("animation %q, want Death", z.animation)
	}
	for frame := 0; frame < 30; frame++ { // 0.5 s: 3 fps * 2 = 3 frames
		p.updateZombies()
	}
	if z.frame < 2.9 || z.frame > 3 {
		t.Fatalf("clip frame %.2f after half a second, want 3 (4 frames at 3 fps, doubled)", z.frame)
	}
	for frame := 0; frame < 300; frame++ {
		p.updateZombies()
	}
	if z.frame != 3 {
		t.Fatalf("clip frame %v, want the last frame to stay", z.frame)
	}
}

// Teleport: alpha = timer * 254 during the 1.33 s fade-out, 0 on arrival, then +0x10 per frame to 255.
func TestSamuraiTeleportAlphaFade(t *testing.T) {
	p, z := samuraiRig()
	enterState5(t, p, z, 3)
	player := p.nearestPlayer(z.x, z.y)
	if z.alpha != 1 {
		t.Fatalf("alpha %v on the roll frame, want unchanged", z.alpha)
	}
	p.samuraiAI(z, player, 1.0/60)
	timer := 1 - 1.0/60*float64(float32(.75))
	want := int(timer * 254)
	if got := samuraiAlphaByte(z.alpha); got != want {
		t.Fatalf("alpha byte %d one frame into the fade, want %d", got, want)
	}
	if z.native.ai.speedFactor != 0 {
		t.Fatalf("factor %v during the fade-out, want 0", z.native.ai.speedFactor)
	}
	for z.native.samurai.special > 0 {
		p.samuraiAI(z, player, 1.0/60)
	}
	if z.alpha != 0 {
		t.Fatalf("alpha %v on arrival, want 0", z.alpha)
	}
	frames := 0
	for ; samuraiAlphaByte(z.alpha) != 255 && frames < 40; frames++ {
		p.samuraiAI(z, player, 1.0/60)
	}
	if frames != 16 {
		t.Fatalf("alpha took %d frames to return, want 16", frames)
	}
}

// Sounds: the roar's rnd(2) comes before the position draws; SFX_SHURIKEN plays on the spin only while it is not
// already playing, and once per blade; nothing plays at the lunge start.
func TestSamuraiSounds(t *testing.T) {
	p, z := samuraiRig()
	enterState5(t, p, z, 1)
	if len(p.sfxQueue) != 0 {
		t.Fatalf("lunge start played %v, want silence", p.sfxQueue)
	}
	// Teleport: sound draw first, then heading and distance.
	p, z = samuraiRig()
	enterState5(t, p, z, 3)
	player := p.nearestPlayer(z.x, z.y)
	for z.native.samurai.special > 0 {
		before := *p.rng
		p.samuraiAI(z, player, 1.0/60)
		if z.native.samurai.special <= 0 {
			probe := before
			roar := probe.Bounded(2)
			want := "SFX_MUMMY_ROAR_" + string(rune('1'+roar))
			if len(p.sfxQueue) == 0 || p.sfxQueue[len(p.sfxQueue)-1] != want {
				t.Fatalf("queue %v, want %s first", p.sfxQueue, want)
			}
			heading := uint16(probe.Bounded(0xff3a))
			if z.native.facing != heading {
				t.Fatalf("heading %d, want the draw right after the sound (%d)", z.native.facing, heading)
			}
		}
	}
	// Spin: plays once, then not again until the clip is over.
	p, z = samuraiRig()
	enterState5(t, p, z, 6)
	player = p.nearestPlayer(z.x, z.y)
	count := func() int {
		n := 0
		for _, name := range p.sfxQueue {
			if name == "SFX_SHURIKEN" {
				n++
			}
		}
		return n
	}
	p.samuraiAI(z, player, 1.0/60)
	p.samuraiAI(z, player, 1.0/60)
	if count() != 1 {
		t.Fatalf("%d shuriken sounds in two spin frames, want 1", count())
	}
	p.time += samuraiShurikenSecs + .01
	p.samuraiAI(z, player, 1.0/60)
	if count() != 2 {
		t.Fatalf("%d shuriken sounds after the clip ended, want 2", count())
	}
	// A blade plays it unconditionally.
	p.sfxQueue = nil
	z.native.ai.gun.timer = 1
	p.zombieFire(z)
	if count() != 1 {
		t.Fatalf("%d shuriken sounds for one blade, want 1", count())
	}
}

// Animations: Idle outside state 5, Attack in state 5, the spin ends as Attack frame 0 (Spin is replaced in the same
// frame), and a far fast dash keeps the current sheet.
func TestSamuraiAnimationSelection(t *testing.T) {
	p, z := samuraiRig()
	player := p.nearestPlayer(z.x, z.y)
	z.native.ai.state = 1
	p.samuraiAI(z, player, 1.0/60)
	if z.animation != samuraiAnimIdle {
		t.Fatalf("animation %q, want Idle", z.animation)
	}
	enterState5(t, p, z, 6)
	z.frame = 5
	p.samuraiAI(z, player, 1.0/60)
	if z.animation != samuraiAnimAttack || z.frame != 0 {
		t.Fatalf("spin: animation %q frame %v, want Attack frame 0", z.animation, z.frame)
	}
	// State 5 with a fast dash far away keeps whatever plays.
	p, z = samuraiRig()
	player = p.nearestPlayer(z.x, z.y)
	z.animation = samuraiAnimIdle
	z.native.samurai.distSq = 40000
	forceRoll(t, p, 1)
	z.native.ai.state, z.native.samurai.prevState, z.native.ai.speedFactor = 5, 1, 4
	p.samuraiAI(z, player, 1.0/60)
	if z.animation != samuraiAnimIdle {
		t.Fatalf("animation %q, want the sheet kept during a far dash", z.animation)
	}
	// The first frame after state 5 sets no animation.
	z.native.ai.state, z.native.samurai.prevState = 1, 5
	z.native.samurai.phase = 0
	z.animation = samuraiAnimAttack
	p.samuraiAI(z, player, 1.0/60)
	if z.animation != samuraiAnimAttack {
		t.Fatalf("animation %q on the frame after state 5, want it unchanged", z.animation)
	}
}

// Survival: the body fades (v7 6 s at full alpha then 6 s down, 1.2.5 3 s then 3 s) and is then removed; alive, v7
// forces alpha 255, 1.2.5 does not.
func TestSamuraiSurvivalFade(t *testing.T) {
	for _, c := range []struct {
		build   string
		seconds float64
		aliveOK bool
	}{{content.WaveBuildV7, 12, true}, {content.WaveBuild125, 6, false}} {
		p, z := samuraiRig()
		p.world.Level.WaveBuild = c.build
		p.levelInfo.Flags = []string{"SURVIVAL"}
		z.alpha = .5
		if _, forced := p.samuraiDrawAlpha(*z); forced != c.aliveOK {
			t.Fatalf("%s: alive override %v, want %v", c.build, forced, c.aliveOK)
		}
		z.health, z.dying = 0, true
		p.updateZombies()
		z.deathAge = c.seconds / 4
		if a, ok := p.samuraiDrawAlpha(*z); !ok || a != 1 {
			t.Fatalf("%s: alpha %v %v a quarter in, want 1", c.build, a, ok)
		}
		z.deathAge = c.seconds * .75
		if a, ok := p.samuraiDrawAlpha(*z); !ok || math.Abs(a-.5) > .01 {
			t.Fatalf("%s: alpha %v three quarters in, want about 0.5", c.build, a)
		}
		if got := p.samuraiDeathSeconds(*z); math.Abs(got-c.seconds) > 1e-9 {
			t.Fatalf("%s: removal after %v s, want %v", c.build, got, c.seconds)
		}
	}
}
