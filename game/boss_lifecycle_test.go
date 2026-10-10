package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
	"testing"
)

func TestBossCorpseRemainsAvailableToOriginalOutro(t *testing.T) {
	for _, present := range []bool{true, false} {
		p := &playState{scriptEntities: map[int]*scriptEntity{9: {id: 9, kind: "zombie", entityType: "boss_rex", x: 200, y: 300}}}
		if present {
			p.zombies = []zombieState{{scriptID: 9, health: 0, dying: true}}
		}
		host := &playScriptHost{play: p}
		result, err := host.Call("GetFirstEntityOfType", []scripting.Value{"boss_rex"})
		if err != nil || len(result.Values) != 1 || result.Values[0] != 9 {
			t.Fatalf("retained corpse unavailable: %+v %v", result, err)
		}
		result, err = host.Call("GetEntityXPos", []scripting.Value{9})
		if err != nil || result.Values[0] != float64(200) {
			t.Fatalf("corpse position unavailable: %+v %v", result, err)
		}
		result, err = host.Call("ZombieExists", []scripting.Value{9})
		if err != nil || result.Values[0] != false {
			t.Fatal("dead corpse considered a live enemy")
		}
		delete(p.scriptEntities, 9)
		// Native 1.2.5 closure 0x0013e574 pushes the null handle 0 when no entity of the type is left.
		result, err = host.Call("GetFirstEntityOfType", []scripting.Value{"boss_rex"})
		if err != nil || len(result.Values) != 1 || result.Values[0] != 0 {
			t.Fatalf("destroyed registry entry returned: %+v %v", result, err)
		}
	}
}
