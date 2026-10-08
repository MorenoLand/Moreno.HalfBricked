package game

import (
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

func sawWeapon() formats.Weapon {
	return formats.Weapon{GunType: "BUZZSAW", Ammo: 700, RateOfFire: .03, Life: .1, Speed: 300, SpreadUnits: 6370, BulletType: "SAW_BLADE"}
}

func dualWeapon() formats.Weapon {
	return formats.Weapon{GunType: "DUALPISTOL", Ammo: 24, RateOfFire: .35, Life: .75, Speed: 700, BulletType: "MULTI", SFXShoot: "SFX_DUAL_PISTOL"}
}

func gunRig(weapon formats.Weapon) *playState {
	_, p := explodingRig()
	p.weapon = weapon
	p.weapons = formats.WeaponCatalog{weapon, {GunType: "PISTOL", RateOfFire: .2, Life: .6, Speed: 600, BulletType: "NORMAL"}}
	rng := weapons.NewNativeRNG()
	p.rng = &rng
	p.angle = 4
	return p
}

func TestBuzzsawSpawnsThreeBladesEveryFrameWithoutTheTrigger(t *testing.T) {
	p := gunRig(sawWeapon())
	p.tickGun(false) // attach
	for frame := 1; frame <= 20; frame++ {
		p.tickGun(false)
		if len(p.bullets) != 3*frame || p.weapon.Ammo != 700-frame {
			t.Fatalf("frame %d: %d blades, %d ammo; want %d blades, %d ammo", frame, len(p.bullets), p.weapon.Ammo, 3*frame, 700-frame)
		}
	}
	if p.bullets[1].projectile.Direction != weapons.NativeWeaponDirection(1, 0) || p.bullets[0].projectile.Direction == p.bullets[2].projectile.Direction {
		t.Fatalf("blades are not fanned around the facing: %+v", p.bullets[0].projectile)
	}
	if p.achieve.shots != 0 || (p.statistics != nil && p.statistics.ShotsFired != 0) {
		t.Fatal("the saw update counts no shots fired (0x000a9f0c has no counter)")
	}
}

func TestBuzzsawFallsBackToThePistolWhenEmpty(t *testing.T) {
	weapon := sawWeapon()
	weapon.Ammo = 2
	p := gunRig(weapon)
	p.tickGun(false)
	p.tickGun(false)
	p.tickGun(false)
	if p.weapon.GunType != "PISTOL" {
		t.Fatalf("weapon %s, want the pistol after the last blade volley", p.weapon.GunType)
	}
}

func bladeAt(p *playState, x, y float64) bullet {
	volley, _ := weapons.NativePrimaryVolley(sawWeapon(), weapons.NativeWeaponDirection(1, 0), p.rng)
	projectile, _ := weapons.NewNativeWeaponProjectile(volley.Shots[1], "SAW_BLADE", x, y, p.rng)
	return bullet{x: x, y: y, vx: projectile.VX, vy: projectile.VY, life: projectile.Life, projectile: &projectile, origin: killOrigin{gun: "BUZZSAW", shot: 1}}
}

func TestSawBladeKillsAnAdjacentZombieAndCredits101(t *testing.T) {
	p := gunRig(sawWeapon())
	p.collectPickup("p_buzzsaw")
	p.syncCombo()
	victim := addZombie(p, "zombie", 1230, 1200)
	b := bladeAt(p, 1200, 1200)
	b.origin = p.primaryOrigin(p.newShot())
	before := p.combo.primary
	if before == nil {
		t.Fatal("no combo tracker after the pickup")
	}
	counter := before.counter
	survives := p.stepSawBlade(&b)
	if p.zombies[victim].health > 0 || !p.zombies[victim].dying {
		t.Fatalf("zombie survived the blade: %+v", p.zombies[victim])
	}
	if got := before.counter - counter; got < 1.0 || got > 1.02 {
		t.Fatalf("combo credit %v, want 1.01 for the single living contact", got)
	}
	if len(p.sfxQueue) != 1 || (p.sfxQueue[0] != "SFX_BUZZSAW_HIT_1" && p.sfxQueue[0] != "SFX_BUZZSAW_HIT_2") {
		t.Fatalf("hit sounds %v, want one chainsaw_rev_1/2", p.sfxQueue)
	}
	if !p.shake.active() || p.shake.total != float64(float32(0.2)) {
		t.Fatalf("a valid contact starts a .2 s camera shake, got %+v", p.shake)
	}
	// The blade survives only when the grid lists the zombie once; a second listing
	// finds a corpse and destroys it (0x000a8534).
	visits := p.zombieGridVisits(&p.zombies[victim], b.x, b.y, 21)
	if survives != (visits <= 1) {
		t.Fatalf("blade survived=%v with %d grid listings", survives, visits)
	}
}

func TestSawBladeIsDestroyedByCorpsesAndInvulnerableTargets(t *testing.T) {
	p := gunRig(sawWeapon())
	corpse := addZombie(p, "zombie", 1230, 1200)
	killNow(p, corpse)
	b := bladeAt(p, 1200, 1200)
	if p.stepSawBlade(&b) {
		t.Fatal("a blade touching a dead zombie must be destroyed")
	}
	if len(p.sfxQueue) != 0 || p.zombies[corpse].health != 0 {
		t.Fatal("a corpse gives no sound and takes no further damage")
	}
	p = gunRig(sawWeapon())
	shielded := addZombie(p, "zombie", 1230, 1200)
	p.zombies[shielded].invulnerable = true
	b = bladeAt(p, 1200, 1200)
	if p.stepSawBlade(&b) {
		t.Fatal("a blade hitting an invulnerable zombie must be destroyed")
	}
	if p.zombies[shielded].health != 100 {
		t.Fatalf("invulnerable zombie lost health: %v", p.zombies[shielded].health)
	}
	if len(p.sfxQueue) == 0 {
		t.Fatal("the contact sound plays before the damage call")
	}
}

func TestSawBladeLivesFiveFramesAndStopsAtWalls(t *testing.T) {
	p := gunRig(sawWeapon())
	b := bladeAt(p, 600, 600)
	frames := 0
	for p.stepSawBlade(&b) {
		frames++
		if frames > 20 {
			t.Fatal("blade never expired")
		}
	}
	if frames != 5 {
		t.Fatalf("blade survived %d updates, want 5 (age reaches .1 s on the 6th, float32)", frames)
	}
	p.world.Level.Layers[formats.LayerC][(600/32)*64+(620/32)] = 0
	b = bladeAt(p, 616, 600)
	if p.stepSawBlade(&b) {
		t.Fatal("blade entering a solid tile must expire")
	}
}

func TestDualPistolAlternatesHandsWithItsOwnTimer(t *testing.T) {
	p := gunRig(dualWeapon())
	p.tickGun(false) // attach
	p.tickGun(false) // released: the timer pegs to half the rate
	if p.gun.Timer != 0.175 {
		t.Fatalf("released timer %v", p.gun.Timer)
	}
	var shots []bullet
	for frame := 0; frame < 80 && len(shots) < 2; frame++ {
		before := len(p.bullets)
		if p.fire(1, 0) && len(p.bullets) > before {
			shots = append(shots, p.bullets[len(p.bullets)-1])
		}
		p.tickGun(false)
	}
	if len(shots) != 2 {
		t.Fatalf("fired %d shots in 80 held frames", len(shots))
	}
	first, second := shots[0], shots[1]
	if first.x != p.x+25 || first.y != p.y+12 || second.x != p.x+32 || second.y != p.y+7 {
		t.Fatalf("hands at (%v,%v) then (%v,%v); want the facing-right entries (+25,+12) then (+32,+7)", first.x-p.x, first.y-p.y, second.x-p.x, second.y-p.y)
	}
	if first.projectile.ChildType != 0x10 || first.projectile.Penetration != 3 {
		t.Fatalf("dual bullet %+v", first.projectile)
	}
	if p.weapon.Ammo != 22 {
		t.Fatalf("ammo %d, want 22", p.weapon.Ammo)
	}
	if p.flash != 0 {
		t.Fatal("the dual pistol uses its flare quad, not the generic flash timer")
	}
	if !weapons.DualFlashActive(p.gun.Spin) && p.gun.Spin != 0 {
		t.Fatalf("flash timer %v", p.gun.Spin)
	}
}

func TestGunStateRidesTheSnapshotForGuests(t *testing.T) {
	host := coopTestPlay(t)
	guest := coopTestPlay(t)
	host.weapon = dualWeapon()
	host.weapons = append(host.weapons, dualWeapon(), sawWeapon())
	host.gun = gunState{kind: "DUALPISTOL", GunVisual: weapons.GunVisual{Spin: 120, BlinkOn: true, Right: true, HandX: 32, HandY: 7}}
	msg, ok := decodeWire(encodeWire(wireMsg{T: "snap", Snap: host.snapshot(3)}))
	if !ok || msg.Snap == nil {
		t.Fatal("snapshot did not decode")
	}
	guest.weapons = host.weapons
	guest.applySnapshot(msg.Snap)
	if guest.weapon.GunType != "DUALPISTOL" || guest.gun.Spin != 120 || !guest.gun.BlinkOn || !guest.gun.Right || guest.gun.HandX != 32 || guest.gun.HandY != 7 {
		t.Fatalf("guest gun state %+v weapon %s", guest.gun, guest.weapon.GunType)
	}
	if !guest.gunBlinks() {
		t.Fatal("the guest does not draw the blink frame")
	}
	// A co-op teammate carries its own saw phase.
	host.coop.players[0].body.weapon = sawWeapon()
	host.coop.players[0].body.gun = gunState{kind: "BUZZSAW", GunVisual: weapons.GunVisual{Spin: 77}}
	msg, _ = decodeWire(encodeWire(wireMsg{T: "snap", Snap: host.snapshot(4)}))
	guest.applySnapshot(msg.Snap)
	if got := guest.coop.players[0].body.gun.Spin; got != 77 {
		t.Fatalf("teammate saw phase %v, want 77", got)
	}
}

func TestSawAudioStartsOnPickupIdlesWhileHeldAndEndsOnDetach(t *testing.T) {
	p := gunRig(sawWeapon())
	p.tickGun(true) // attach: Start
	events := p.spinEvents.Drain()
	if len(events) != 1 || events[0].Sound != "SFX_BUZZSAW_START" || events[0].Loop || events[0].Stop {
		t.Fatalf("attach events %+v", events)
	}
	p.spinEndFinished = false
	p.tickGun(true)
	if len(p.spinEvents) != 0 {
		t.Fatalf("idle started while the Start clip still plays: %+v", p.spinEvents)
	}
	p.spinEndFinished = true
	p.tickGun(true)
	events = p.spinEvents.Drain()
	if len(events) != 1 || events[0].Sound != "SFX_BUZZSAW_IDLE" || !events[0].Loop || events[0].LoopPointSamples != 23924 {
		t.Fatalf("idle events %+v", events)
	}
	p.spinEndFinished = false
	p.tickGun(true)
	if len(p.spinEvents) != 0 {
		t.Fatalf("the idle loop is restarted while it plays: %+v", p.spinEvents)
	}
	pistol, _ := p.weapons.Find("PISTOL")
	p.equipWeapon(pistol)
	p.tickGun(true)
	events = p.spinEvents.Drain()
	if len(events) != 1 || !events[0].Stop || events[0].Sound != "SFX_BUZZSAW_IDLE" {
		t.Fatalf("detach events %+v", events)
	}
	if len(p.sfxQueue) != 1 || p.sfxQueue[0] != "SFX_BUZZSAW_DEATH" {
		t.Fatalf("End clip not queued: %v", p.sfxQueue)
	}
}
