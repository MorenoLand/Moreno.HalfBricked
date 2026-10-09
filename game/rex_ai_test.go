package game

import (
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

// engagedRex builds a rex rig whose player stands `distance` px to the left of
// the rex, inside the sight range, and primes the AI state.
func engagedRex(t *testing.T, distance float64) *achievementRig {
	t.Helper()
	r := newRexRig(t, nil, 600, 600)
	r.p.x, r.p.y = 600-distance, 600
	r.p.updateRexBoss() // creates the state
	state := r.p.rex.bosses[rexTestID]
	state.ai.rangeSq = 3 * 110 * 110 // the smallest native sight: 3 * (110 + rnd(50))^2
	return r
}

// FUN_000b94e4: three FUN_00094c6c calls of dt * 0.05 per tick while the player is
// inside 0.3 * (width + 64); nothing while the rex leaps or a script runs.
func TestRexContactDamageIsThreeTimesPointZeroFivePerSecond(t *testing.T) {
	r := engagedRex(t, 40) // reach 0.3 * (128 + 64) = 57.6
	p := r.p
	state := p.rex.bosses[rexTestID]
	p.health = 1
	for tick := 0; tick < 60; tick++ {
		p.rexContact(r.rex(), state, 1.0/60)
	}
	if lost := 1 - p.health; math.Abs(lost-0.15) > 1e-9 {
		t.Fatalf("contact cost %.6f health in one second, want 0.15 (3 x 0.05)", lost)
	}
	p.health = 1
	r.rex().x = 600 + 58 // just outside 0.3 * (128 + 64)
	p.rexContact(r.rex(), state, 1.0/60)
	if p.health != 1 {
		t.Fatal("contact damage outside the native reach")
	}
	r.rex().x = 600
	state.leaping = true
	p.rexContact(r.rex(), state, 1.0/60)
	if p.health != 1 {
		t.Fatal("a leaping rex must not hurt by contact")
	}
	state.leaping = false
	p.scriptRuntime = nil
	p.health = 1
	p.cheats.god = true
	p.rexContact(r.rex(), state, 1.0/60)
	if p.health != 1 {
		t.Fatal("god mode ignored")
	}
}

// FUN_0009f32c state 3 with ammo: a volley of four VENOM projectiles every 0.8 s
// (record 10 = VOMIT), three volleys until the ammo of FUN_000ba174 is spent.
func TestRexFiresThreeVolleysOfFourVenomThenTheGateOpens(t *testing.T) {
	r := engagedRex(t, 150)
	p := r.p
	state := p.rex.bosses[rexTestID]
	if state.ai.ammo != 3 || state.gateOpen() {
		t.Fatalf("initial ammo %d gate %v, want 3 / closed", state.ai.ammo, state.gateOpen())
	}
	var fires []int
	for tick := 0; tick < 900 && state.ai.ammo > 0; tick++ {
		before := state.ai.ammo
		p.rex.venom = nil
		p.health = 1 // venom damage is covered by its own test
		p.updateRexBoss()
		if state.ai.ammo != before {
			fires = append(fires, tick)
			if len(p.rex.venom) != 4 {
				t.Fatalf("volley %d spawned %d projectiles, want 4", len(fires), len(p.rex.venom))
			}
		}
	}
	if len(fires) != 3 || state.ai.ammo != 0 || !state.gateOpen() {
		t.Fatalf("fired %v, ammo %d: want three volleys and an open gate", fires, state.ai.ammo)
	}
	for i := 1; i < len(fires); i++ {
		if gap := fires[i] - fires[i-1]; gap < 48 {
			t.Fatalf("volleys %d ticks apart, want at least 0.8 s (48 ticks)", gap)
		}
	}
}

// Spread: heading = facing +- 2 * 17 * 182 units (about 34 degrees).
func TestRexVenomVolleyStaysInsideTheNativeSpread(t *testing.T) {
	r := engagedRex(t, 150)
	p := r.p
	state := p.rex.bosses[rexTestID]
	rng := weapons.NewNativeRNG()
	p.rng = &rng
	for volley := 0; volley < 40; volley++ {
		p.rex.venom = nil
		state.ai.ammo = 3
		p.fireRexVenom(r.rex(), state, -150, 0)
		for _, v := range p.rex.venom {
			off := math.Abs(math.Remainder(v.heading-math.Pi, 2*math.Pi)) * 180 / math.Pi
			if off > 34.01 {
				t.Fatalf("heading %.2f degrees off the facing, want <= 34", off)
			}
			if v.height != r.rex().size.Y*0.3 || v.speed != 250 {
				t.Fatalf("venom height %v speed %v", v.height, v.speed)
			}
		}
	}
}

// FUN_0009f32c state 3 with the gate open: wait 0.95 s, then lunge (state 5,
// factor 4.0) with a roar; the factor decays at 4/s back to 1.0.
func TestRexLungesWhenTheAmmoIsSpentAndReloadsOnContact(t *testing.T) {
	r := engagedRex(t, 150)
	p := r.p
	state := p.rex.bosses[rexTestID]
	state.ai.ammo, state.ai.state, state.ai.factor = 0, 3, 0
	state.ai.timer, state.ai.interval = 0, 0.8
	lunge := -1
	for tick := 0; tick < 200 && lunge < 0; tick++ {
		p.updateRexBoss()
		if state.ai.state == 5 {
			lunge = tick
		}
	}
	if lunge < 55 || lunge > 60 {
		t.Fatalf("lunge began after %d ticks, want about 0.95 s (57)", lunge)
	}
	if state.ai.factor != rexLungeFactor && state.ai.factor < 3.9 {
		t.Fatalf("lunge speed factor %v, want 4", state.ai.factor)
	}
	if !hasSFX(p, "SFX_T_REX_ROAR_1") && !hasSFX(p, "SFX_T_REX_ROAR_2") {
		t.Fatalf("no roar at the lunge start: %v", p.sfxQueue)
	}
	if animation := r.rex().animation; animation != "Charge" {
		t.Fatalf("animation %q during the lunge, want Charge", animation)
	}
	ticks := 0
	for state.ai.state == 5 && ticks < 200 {
		p.updateRexBoss()
		ticks++
	}
	if ticks < 43 || ticks > 47 {
		t.Fatalf("lunge lasted %d more ticks, want 0.75 s (45)", ticks)
	}
	if state.ai.factor != 1 || r.rex().animation != "" {
		t.Fatalf("after the lunge factor %v animation %q", state.ai.factor, r.rex().animation)
	}
	// Touching the player while in state 5 reloads the weapon record with ammo 5.
	state.ai.ammo, state.ai.state, state.ai.factor = 0, 5, 2
	r.rex().x = p.x + 40
	p.updateRexBoss()
	if state.ai.ammo != 5 || state.gateOpen() {
		t.Fatalf("ammo %d after touching the player in state 5, want 5", state.ai.ammo)
	}
}

// FUN_000b9df0: the counter lets the AI engage only for 69 of every 100 calls, and
// the sight range is 3 * (110 + rnd(50))^2.
func TestRexAIEngagementWindowAndRange(t *testing.T) {
	r := engagedRex(t, 150)
	p := r.p
	state := p.rex.bosses[rexTestID]
	state.ai.state, state.ai.counter = 1, 0
	engaged := 0
	for tick := 0; tick < 100; tick++ {
		state.ai.state, state.ai.factor = 1, 1
		p.updateRexAI(r.rex(), state, 1.0/60)
		if state.ai.state == 2 {
			engaged++
		}
	}
	if engaged != 70 { // counter values 100..31 engage, 30..1 do not, the refresh call engages
		t.Fatalf("engaged on %d of 100 calls, want 70", engaged)
	}
	r.p.x = 600 - 400 // beyond 3 * (160)^2
	state.ai.state, state.ai.counter = 1, 100
	p.updateRexAI(r.rex(), state, 1.0/60)
	if state.ai.state != 1 {
		t.Fatal("rex engaged a player outside its sight range")
	}
}

// FUN_000b9624: the health-fraction rage needs state 3 with the gate open, i.e.
// spent ammo; +0x338 starts at 0 and is stored on the first open-gate frame.
func TestRexHealthRageNeedsSpentAmmoAndCrossesTheThresholds(t *testing.T) {
	r := engagedRex(t, 150)
	p := r.p
	state := p.rex.bosses[rexTestID]
	r.rex().health = 25000 * .60
	for tick := 0; tick < 30; tick++ {
		p.updateRexBoss()
	}
	if r.rex().rexRageTimer != 0 || state.lastRatio != 0 {
		t.Fatal("rage / ratio stored while the ammo is not spent")
	}
	// Spend the ammo: at the first open-gate frame the fraction is stored, no rage.
	state.ai.ammo, state.ai.state = 0, 3
	p.updateRexBoss()
	if r.rex().rexRageTimer != 0 || math.Abs(state.lastRatio-.60) > 1e-9 {
		t.Fatalf("first open-gate frame: timer %v ratio %v", r.rex().rexRageTimer, state.lastRatio)
	}
	// A fresh rex at full health crosses 66%.
	state.lastRatio, r.rex().health = 1, 25000*.65
	state.ai.state = 3
	p.updateRexBoss()
	if r.rex().rexRageTimer < 900 || (!hasSFX(p, "SFX_T_REX_ROAR_1") && !hasSFX(p, "SFX_T_REX_ROAR_2")) {
		t.Fatalf("no rage/roar at 66%%: timer %v sfx %v", r.rex().rexRageTimer, p.sfxQueue)
	}
	// Windup 1 s, leap, landing: the record is reloaded (ammo 3) and a shockwave of
	// 3 x width (600 * 128 / 200) appears.
	for tick := 0; tick < 400 && len(p.rex.waves) == 0; tick++ {
		p.updateZombies()
	}
	if len(p.rex.waves) == 0 {
		t.Fatal("rex never landed")
	}
	if got := p.rex.waves[0].max; math.Abs(got-384) > 1e-9 {
		t.Fatalf("shockwave max size %v, want 600 * 128/200 = 384", got)
	}
	if state.ai.ammo != 3 || state.gateOpen() {
		t.Fatalf("ammo %d after landing, want the reload to 3", state.ai.ammo)
	}
	// With ammo 3 a drop to 30% does nothing until the volleys are spent.
	r.rex().health = 25000 * .30
	for tick := 0; tick < 20; tick++ {
		p.updateRexBoss()
	}
	if r.rex().rexRageTimer != 0 {
		t.Fatal("rage through the closed gate")
	}
	state.ai.ammo, state.ai.state = 0, 3
	p.updateRexBoss()
	if r.rex().rexRageTimer == 0 {
		t.Fatal("no rage when crossing 33% with the gate open")
	}
}

// The intro script calls MakeRexRage every frame, which keeps the timer between
// 500 and 1000 ms (ungated by health); the leap only follows once the script stops.
func TestRexLeapWaitsForTheScriptToStopCallingMakeRexRage(t *testing.T) {
	r := newRexRig(t, nil, 800, 800)
	p := r.p
	h := &playScriptHost{app: r.app, play: p}
	for tick := 0; tick < 200; tick++ {
		if _, err := h.Call("MakeRexRage", nil); err != nil {
			t.Fatal(err)
		}
		p.updateZombies()
		if len(p.rex.waves) != 0 || p.rexLeaping(rexTestID) {
			t.Fatalf("rex leaped at tick %d while the script kept calling MakeRexRage", tick)
		}
	}
	if r.rex().rexRageTimer < 500 {
		t.Fatalf("rage timer %v, want it held between 500 and 1000", r.rex().rexRageTimer)
	}
	stopped := 0
	for len(p.rex.waves) == 0 && stopped < 400 {
		p.updateZombies()
		stopped++
	}
	if stopped < 30+60 || stopped > 60+90 { // 0.5..1 s of windup, then about 80 ticks of flight
		t.Fatalf("landing %d ticks after the script stopped, want windup (30..60) + flight (~80)", stopped)
	}
}

// Health is the strength attribute; every bullet hit takes 101 (boss classes do
// not override FUN_000a0304 beyond the dead-player check FUN_000b4a98).
func TestRexHitsToKillAtOneHundredAndOnePerBullet(t *testing.T) {
	for _, tc := range []struct {
		health float64
		hits   int
	}{{25000, 248}, {35000, 347}} {
		p := &playState{}
		z := zombieState{health: tc.health}
		hits := 0
		for !p.hurtZombie(&z, zombieHit{damage: zombieBulletDamage, kind: 0x10}) {
			hits++
			if hits > 1000 {
				t.Fatal("never died")
			}
		}
		hits++ // the killing hit
		if hits != tc.hits {
			t.Fatalf("health %v: %d hits, want %d (ceil(%v/101))", tc.health, hits, tc.hits, tc.health)
		}
	}
}

// FUN_000a4288 / FUN_000a4210 / FUN_000a69c8: a venom drop is lobbed, lands after
// ~0.45 s as a puddle (x5.5) with four droplets, hurts 0.0005 per tick while the
// player overlaps it and fades after 1.4..1.9 s.
func TestRexVenomLandsAsAPuddleAndHurtsTheBarelyAtAll(t *testing.T) {
	r := engagedRex(t, 600)
	p := r.p
	p.x, p.y = 1400, 1400 // far from the rex and from the drop
	v := p.newRexVenom(900, 900, 0, 38.4, 0, false)
	v.speed = 0 // lands where it stands
	p.rex.venom = []rexVenom{v}
	landed := -1
	for tick := 0; tick < 120 && landed < 0; tick++ {
		p.health = 1
		p.updateRexVenom()
		if len(p.rex.venom) > 0 && p.rex.venom[0].state == 2 {
			landed = tick
		}
	}
	if landed < 20 || landed > 40 {
		t.Fatalf("venom landed after %d ticks, want about 0.44 s", landed)
	}
	if len(p.rex.venom) != 5 {
		t.Fatalf("%d venom entries after the splat, want the puddle plus 4 droplets", len(p.rex.venom))
	}
	if main := p.rex.venom[0]; math.Abs(main.size-55) > 1e-9 || main.age != 0 {
		t.Fatalf("puddle size %v age %v, want 55 / 0", main.size, main.age)
	}
	p.health = 1
	p.rex.venom = p.rex.venom[:1]
	p.x, p.y = 900, 900 // step into the puddle
	p.rex.venom[0].life = 1.5
	p.updateRexVenom()
	if lost := 1 - p.health; math.Abs(lost-rexVenomDamage) > 1e-12 {
		t.Fatalf("puddle tick cost %v, want 0.0005", lost)
	}
	for tick := 0; tick < 100 && len(p.rex.venom) > 0; tick++ {
		p.updateRexVenom()
	}
	if len(p.rex.venom) != 0 {
		t.Fatal("puddle outlived its life")
	}
}

// FUN_000ba094: the rex arrives 80..100 px from the player, airborne at height 10,
// and lands with a shockwave.
func TestRexDropsInNextToThePlayerAndLandsWithAShockwave(t *testing.T) {
	r := newRexRig(t, nil, 2000, 2000)
	p := r.p
	p.x, p.y = 1000, 1000
	p.markRexSpawn(rexTestID, "boss_rex")
	p.updateRexBoss()
	rex := r.rex()
	distance := math.Hypot(rex.x-p.x, rex.y-p.y)
	if distance < 79.99 || distance > 100.01 {
		t.Fatalf("rex dropped in %.1f px from the player, want 80..100", distance)
	}
	state := p.rex.bosses[rexTestID]
	if !state.leaping || p.rexLift(*rex) < 9*rex.size.Y {
		t.Fatalf("rex not airborne after the spawn reset: leaping %v lift %v", state.leaping, p.rexLift(*rex))
	}
	ticks := 0
	for len(p.rex.waves) == 0 && ticks < 400 {
		p.updateRexBoss()
		ticks++
	}
	if len(p.rex.waves) == 0 || ticks < 60 || ticks > 100 {
		t.Fatalf("landing after %d ticks (waves %d), want about 1.2 s", ticks, len(p.rex.waves))
	}
	if wave := p.rex.waves[0]; math.Abs(wave.x-rex.x) > 0 || wave.owner != rexTestID {
		t.Fatalf("wave %+v not at the landing point", wave)
	}
}

// A scripted scene (scene state != 1) shields the player from the shockwave.
func TestRexShockwaveDoesNotHurtDuringAScript(t *testing.T) {
	h := script125CachedHost(t, "world0_level2")
	p := h.play
	if p.scriptRuntime == nil || p.scriptRuntime.Done() {
		t.Skip("no running script")
	}
	p.health = 1
	p.rex.waves = []rexShockwave{{x: p.x, y: p.y, age: .5, owner: 0}}
	p.rexShockwaveHit(p.rex.waves[0])
	if p.health != 1 {
		t.Fatalf("player hurt by a shockwave during a script: %v", p.health)
	}
}

var _ = formats.Vec2{}

// Guests draw the rex leap, the shockwave and the venom from the snapshot.
func TestGuestSeesTheRexLeapShockwaveAndVenom(t *testing.T) {
	host := coopTestPlay(t)
	guest := coopTestPlay(t)
	host.scriptEntities = map[int]*scriptEntity{rexTestID: {id: rexTestID, kind: "zombie", entityType: "boss_rex"}}
	host.zombies = []zombieState{{x: 10, y: 20, health: 25000, size: formats.Vec2{X: 128, Y: 128}, scriptID: rexTestID}}
	host.rex.bosses = map[int]*rexBossState{rexTestID: {leaping: true, height: .5}}
	host.rex.waves = []rexShockwave{{x: 100, y: 120, age: .25, max: 384}}
	host.rex.venom = []rexVenom{{x: 50, y: 60, heading: 1, height: 12, size: 55, age: .5, life: 1.5, state: 2}}
	raw := encodeWire(wireMsg{T: "snap", Snap: host.snapshot(3)})
	msg, ok := decodeWire(raw)
	if !ok || msg.Snap == nil {
		t.Fatal("snapshot did not decode")
	}
	guest.applySnapshot(msg.Snap)
	if len(guest.zombies) != 1 || math.Abs(guest.rexLift(guest.zombies[0])-64) > 1e-9 {
		t.Fatalf("guest rex lift %v, want 0.5 * 128", guest.rexLift(guest.zombies[0]))
	}
	if len(guest.rex.waves) != 1 || guest.rex.waves[0].max != 384 || guest.rex.waves[0].age != .25 {
		t.Fatalf("guest waves %+v", guest.rex.waves)
	}
	if len(guest.rex.venom) != 1 || guest.rex.venom[0].state != 2 || guest.rex.venom[0].size != 55 {
		t.Fatalf("guest venom %+v", guest.rex.venom)
	}
}
