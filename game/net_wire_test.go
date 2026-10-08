package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

func TestSnapshotRoundTripsThroughTheWire(t *testing.T) {
	host := coopTestPlay(t)
	host.updateCoopPlayers([]playerInput{{}})
	guest := coopTestPlay(t)
	guest.updateCoopPlayers([]playerInput{{}})

	host.x, host.y = 300, 200
	host.coop.players[0].body.x, host.coop.players[0].body.y = 340, 210
	host.coop.players[0].body.weapon, _ = host.weapons.Find("SHOTGUN")
	host.coop.players[0].body.health = .5
	host.score, host.lives, host.levelKills = 1234, 2, 7
	host.zombies = []zombieState{{x: 10.04, y: 20, texture: "Zombie/a", animation: "walk", size: formats.Vec2{X: 48, Y: 48}, health: 50, frame: 2}, {x: 30, y: 40, texture: "Zombie/a", animation: "die", dying: true, deathAge: .2}}
	host.bullets = []bullet{{x: 5, y: 6, vx: 1, vy: 2, life: 1, kind: "NORMAL", projectile: &weapons.NativeWeaponProjectile{X: 5, Y: 6, EntityType: 0x12, Penetration: 3}}}
	host.mines = []mineState{{x: 50, y: 60, age: 1}}
	host.spawnPickup("p_shotgun", formats.Vec2{X: 70, Y: 80})

	raw := encodeWire(wireMsg{T: "snap", Snap: host.snapshot(9)})
	msg, ok := decodeWire(raw)
	if !ok || msg.Snap == nil {
		t.Fatal("snapshot did not decode")
	}
	guest.applySnapshot(msg.Snap)

	if guest.x != 300 || guest.y != 200 || guest.score != 1234 || guest.lives != 2 || guest.levelKills != 7 {
		t.Fatalf("scalars: %v,%v score %d lives %d kills %d", guest.x, guest.y, guest.score, guest.lives, guest.levelKills)
	}
	other := guest.coop.players[0]
	if math.Abs(other.body.x-340) > .1 || other.body.weapon.GunType != "SHOTGUN" || other.body.health != .5 {
		t.Fatalf("player 2 mirror: %+v", other.body)
	}
	if len(guest.zombies) != 2 || guest.zombies[0].texture != "Zombie/a" || guest.zombies[1].animation != "die" || !guest.zombies[1].dying {
		t.Fatalf("zombies: %+v", guest.zombies)
	}
	if len(guest.bullets) != 1 || guest.bullets[0].projectile == nil || guest.bullets[0].projectile.Penetration != 3 {
		t.Fatalf("bullets: %+v", guest.bullets)
	}
	if len(guest.mines) != 1 || len(guest.scriptEntities) != 1 {
		t.Fatalf("mines %d pickups %d", len(guest.mines), len(guest.scriptEntities))
	}
	t.Logf("snapshot with 2 zombies is %d bytes", len(raw))
}

func TestGuestFollowsTheHostsZoom(t *testing.T) {
	host, guest := coopTestPlay(t), coopTestPlay(t)
	host.world.SetZoom(.7)
	guest.applySnapshot(host.snapshot(1))
	if guest.world.Zoom != .7 {
		t.Fatalf("guest zoom %v, want the host's .7", guest.world.Zoom)
	}
}

func TestGuestSeesTheHostsTrain(t *testing.T) {
	host, guest := coopTestPlay(t), coopTestPlay(t)
	host.spawnTrain()
	host.train.x, host.train.active, host.train.frame = 400, true, 2
	guest.applySnapshot(host.snapshot(1))
	if guest.train == nil || guest.train.x != 400 || !guest.train.active || guest.train.frame != 2 {
		t.Fatalf("guest train %+v", guest.train)
	}
}
