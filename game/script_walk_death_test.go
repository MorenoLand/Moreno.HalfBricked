package game

import (
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
)

// IsZombieWalking must end when the walking zombie dies. The exit scripts loop on it, so a stale walk flag would
// hold the level for ever (seen in World5Level0 with a fixed seed: the zombie was killed mid walk).
func TestIsZombieWalkingEndsWhenTheZombieDies(t *testing.T) {
	p := &playState{scriptEntities: map[int]*scriptEntity{5: {id: 5, kind: "zombie", entityType: "zombie", walking: true, targetX: 100, targetY: 100, targetRange: 80}}}
	p.zombies = []zombieState{{scriptID: 5, health: 100}}
	host := &playScriptHost{play: p}
	walking := func() bool {
		t.Helper()
		result, err := host.Call("IsZombieWalking", []scripting.Value{5})
		if err != nil || len(result.Values) != 1 {
			t.Fatalf("IsZombieWalking: %v %+v", err, result)
		}
		value, ok := result.Values[0].(bool)
		if !ok {
			t.Fatalf("IsZombieWalking returned %#v", result.Values[0])
		}
		return value
	}
	if !walking() {
		t.Fatal("a living walking zombie must report walking")
	}
	p.zombies[0].health, p.zombies[0].dying = 0, true
	if walking() {
		t.Fatal("a dying zombie must not report walking")
	}
	p.zombies = nil
	if walking() {
		t.Fatal("a removed zombie must not report walking")
	}
}
