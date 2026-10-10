package game

import (
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

// riseZombies ends the 2 s spawn rise of every rising wave zombie in the rig. The
// native grid lists a zombie only after its rise (see zombieIsRising), so a hit
// test that wants a zombie in the grid has to finish the rise first.
func riseZombies(p *playState) {
	for index := range p.zombies {
		if zombieIsRising(&p.zombies[index]) {
			p.zombies[index].native.ai.state, p.zombies[index].native.ai.riseMS = 1, 0
		}
	}
}

// A rising zombie is invisible to the projectile pass (state 0 is never inserted
// into the grid), so a bullet on top of it does nothing until the rise ends.
func TestRisingZombieIsNotHitUntilItRises(t *testing.T) {
	p := modelRig()
	p.spawnZombieAt(waveEntry("zombie", 100, formats.Vec2{X: 35, Y: 35}), formats.Vec2{X: 400, Y: 400})
	z := &p.zombies[0]
	if !zombieIsRising(z) {
		t.Fatalf("a freshly spawned zombie must be rising (state %d)", z.native.ai.state)
	}
	p.bullets = append(p.bullets, pistolBullet(z.x, z.y))
	p.updateBulletsAndKills()
	if z.health != 100 {
		t.Fatalf("a bullet hit a rising zombie: health %v, want 100", z.health)
	}
	if p.bulletReachesZombie(pistolBullet(z.x, z.y), z) {
		t.Fatal("a rising zombie is listed in the grid")
	}

	// The rise is 1.5 s invisible plus 0.5 s of squash: after 2 s of 60 Hz frames the
	// zombie is walking and the grid lists it.
	const dt = 1.0 / 60.0
	for frame := 0; frame < 180 && zombieIsRising(z); frame++ {
		p.stepZombieAI(z, 0, 0, z.x, z.y, dt)
	}
	if zombieIsRising(z) {
		t.Fatalf("zombie still rising after 3 s: state %d riseMS %v", z.native.ai.state, z.native.ai.riseMS)
	}
	if !p.bulletReachesZombie(pistolBullet(z.x, z.y), z) {
		t.Fatal("a risen zombie must be hit by a bullet on top of it")
	}
}

// The grid registration skips a rising zombie, so it has no cells to be visited in.
func TestRisingZombieHasNoGridRegistration(t *testing.T) {
	p := modelRig()
	p.spawnZombieAt(waveEntry("zombie", 100, formats.Vec2{X: 35, Y: 35}), formats.Vec2{X: 400, Y: 400})
	z := &p.zombies[0]
	p.refreshEntityGrid()
	if z.grid.valid {
		t.Fatal("refreshEntityGrid registered a rising zombie")
	}
	riseZombies(p)
	p.refreshEntityGrid()
	if !z.grid.valid {
		t.Fatal("a risen zombie must be registered by the grid refresh")
	}
}

// Contact damage and the push-out are inside the candidate block of FUN_000a1b08,
// which a rising zombie never reaches: standing on a rising zombie hurts nobody.
func TestRisingZombieDoesNotHurtThePlayer(t *testing.T) {
	p := modelRig()
	p.spawnZombieAt(waveEntry("zombie", 100, formats.Vec2{X: 35, Y: 35}), formats.Vec2{X: 400, Y: 400})
	z := &p.zombies[0]
	prey := playerTarget{index: 0, x: z.x, y: z.y}
	p.health = 1
	p.zombieContact(z, prey, 1.0/60.0)
	if p.health != 1 {
		t.Fatalf("a rising zombie hurt the player: health %v", p.health)
	}
	riseZombies(p)
	p.zombieContact(z, prey, 1.0/60.0)
	if p.health >= 1 {
		t.Fatal("a risen zombie on top of the player must hurt him")
	}
}
