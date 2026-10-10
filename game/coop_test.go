package game

import (
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

func TestCoopAvailabilityFollowsTheNativeRule(t *testing.T) {
	for _, tc := range []struct {
		pads, touch int
		desktopPad  bool
		available   bool
		players     int
		split       bool
	}{
		{0, 0, true, false, 0, false},
		{1, 0, false, false, 0, false}, // native: one pad is rejected
		{2, 0, false, true, 2, false},
		{3, 0, false, true, 3, false},
		{6, 0, false, true, 4, false}, // capped at four players
		{0, 2, false, true, 2, true},  // touch co-op is split screen
		{1, 2, false, false, 0, false},
		{1, 0, true, true, 2, false}, // port extension: keyboard/mouse + one pad
	} {
		if got := coopAvailable(tc.pads, tc.touch, tc.desktopPad); got != tc.available {
			t.Fatalf("%+v available=%v", tc, got)
		}
		if tc.available {
			if players, split := coopRequest(tc.pads, tc.touch, tc.desktopPad); players != tc.players || split != tc.split {
				t.Fatalf("%+v request (%d,%v)", tc, players, split)
			}
		}
	}
}

func coopTestPlay(t *testing.T) *playState {
	t.Helper()
	p := script125CachedHost(t, "world0_level1").play
	p.closeScript()
	p.hudVisible = true
	p.startCoop(2, false)
	p.maxHealth = 1
	return p
}

func TestCoopPlayerJoinsMovesAndFires(t *testing.T) {
	p := coopTestPlay(t)
	player := p.coop.players[0]
	p.updateCoopPlayers([]playerInput{{}})
	if !player.joined {
		t.Fatal("player 2 should join once the entry script is done")
	}
	startX := player.body.x
	for frame := 0; frame < 30; frame++ {
		p.updateCoopPlayers([]playerInput{{moveX: 1}})
	}
	if player.body.x <= startX+20 {
		t.Fatalf("player 2 did not move right: %v -> %v", startX, player.body.x)
	}
	if p.x == player.body.x && p.y == player.body.y {
		t.Fatal("players share a body")
	}
	before := len(p.bullets)
	p.updateCoopPlayers([]playerInput{{aimX: 1}})
	if len(p.bullets) <= before {
		t.Fatal("aiming with the right stick should fire")
	}
}

func TestCoopWaitsWhileAScriptHoldsSecondaryPlayers(t *testing.T) {
	p := coopTestPlay(t)
	p.secondaryPlayersWait = true
	runtime := script125CachedHost(t, "world0_level1").play.scriptRuntime
	p.scriptRuntime = runtime
	p.updateCoopPlayers([]playerInput{{moveX: 1}})
	if p.coop.players[0].joined {
		t.Fatal("secondary players must wait while the entry script runs")
	}
}

func TestZombiesChaseAndHurtTheNearestPlayer(t *testing.T) {
	p := coopTestPlay(t)
	p.updateCoopPlayers([]playerInput{{}})
	player := p.coop.players[0]
	player.body.x, player.body.y = p.x+400, p.y
	p.zombies = []zombieState{{x: player.body.x - 20, y: player.body.y, speed: 0, health: 100, size: formats.Vec2{X: 48, Y: 48}, collision: 15}}
	prey := p.nearestPlayer(p.zombies[0].x, p.zombies[0].y)
	if prey.index != 1 {
		t.Fatalf("zombie picked player %d, want the nearby player 2", prey.index)
	}
	for frame := 0; frame < 120; frame++ {
		p.updateZombies()
	}
	if player.body.health >= p.maxHealth {
		t.Fatal("player 2 took no damage from the adjacent zombie")
	}
	if p.health != p.maxHealth {
		t.Fatal("player 1 was hurt by a zombie chasing player 2")
	}
}

func TestCoopCameraFollowsTheMidpoint(t *testing.T) {
	p := coopTestPlay(t)
	p.updateCoopPlayers([]playerInput{{}})
	player := p.coop.players[0]
	p.x, p.y = 200, 200
	player.body.x, player.body.y = 400, 300
	x, y := p.cameraFocus()
	if math.Abs(x-300) > .01 || math.Abs(y-250) > .01 {
		t.Fatalf("camera focus %v,%v want 300,250", x, y)
	}
	player.dead = true
	if x, _ := p.cameraFocus(); math.Abs(x-200) > .01 {
		t.Fatal("a dead player no longer pulls the camera")
	}
}

func TestCoopPlayerDeathSpendsALifeAndReturns(t *testing.T) {
	p := coopTestPlay(t)
	p.lives = 2
	p.updateCoopPlayers([]playerInput{{}})
	player := p.coop.players[0]
	player.body.health = 0
	p.updateCoopPlayers([]playerInput{{}})
	if !player.dead || p.lives != 1 {
		t.Fatalf("death: dead=%v lives=%d", player.dead, p.lives)
	}
	for frame := 0; frame < 60*3; frame++ {
		p.updateCoopPlayers([]playerInput{{}})
	}
	if player.dead || player.body.health <= 0 {
		t.Fatal("player 2 never returned")
	}
	if math.Hypot(player.body.x-p.x, player.body.y-p.y) > 80 {
		t.Fatalf("player 2 returned far from player 1: %v,%v vs %v,%v", player.body.x, player.body.y, p.x, p.y)
	}
}

func TestCoopWithNoLivesLeavesPlayersDownAndEndsTheGame(t *testing.T) {
	p := coopTestPlay(t)
	p.lives = 0
	p.updateCoopPlayers([]playerInput{{}})
	player := p.coop.players[0]
	player.body.health = 0
	p.updateCoopPlayers([]playerInput{{}})
	for frame := 0; frame < 60*4; frame++ {
		p.updateCoopPlayers([]playerInput{{}})
	}
	if !player.dead {
		t.Fatal("with no lives the player stays down")
	}
	if p.allPlayersDown() {
		t.Fatal("player 1 is still alive")
	}
	p.health = 0
	if !p.allPlayersDown() {
		t.Fatal("everyone is down; the game should end")
	}
}

func TestCoopPlayerPickupsAreTheirOwn(t *testing.T) {
	p := coopTestPlay(t)
	p.updateCoopPlayers([]playerInput{{}})
	player := p.coop.players[0]
	p.spawnPickup("p_shotgun", formats.Vec2{X: player.body.x, Y: player.body.y})
	for i := 0; i < 120; i++ {
		// the crate falls from 250 px and is only taken on a ground frame (pickup_drop.go)
		p.stepPickupDrops(1.0 / 60)
		p.updatePickups()
	}
	if player.body.weapon.GunType != "SHOTGUN" {
		t.Fatalf("player 2 holds %s after touching a shotgun", player.body.weapon.GunType)
	}
	if p.weapon.GunType == "SHOTGUN" {
		t.Fatal("player 1's weapon changed")
	}
}

func TestBeingHurtFlashesRedAndWearsOff(t *testing.T) {
	p := coopTestPlay(t)
	p.updateCoopPlayers([]playerInput{{}})
	p.damagePlayer(0, .1)
	if p.hurt <= 0 {
		t.Fatal("player 1 should flash when hurt")
	}
	p.damagePlayer(1, .1)
	if p.coop.players[0].body.hurt <= 0 {
		t.Fatal("player 2 should flash when hurt")
	}
	if full, half := hurtTint(hurtFlashDuration, 0, [3]float32{}), hurtTint(hurtFlashDuration/2, 0, [3]float32{}); full[0] <= half[0] || half[0] <= 1 || hurtTint(0, 0, [3]float32{}) != ([3]float32{}) {
		t.Fatal("the tint should fade from red back to normal")
	}
	for frame := 0; frame < 30; frame++ {
		p.stepBody(0, 0, false, false, true)
	}
	if p.hurt != 0 {
		t.Fatalf("the flash never wore off: %v", p.hurt)
	}
}
