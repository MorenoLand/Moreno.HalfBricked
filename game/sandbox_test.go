package game

import "testing"

func TestSandboxSpawnsZombiesAndCratesAndCheatsWork(t *testing.T) {
	host := script125CachedHost(t, "world0_level1")
	a, p := host.app, host.play
	p.closeScript()
	a.play = p
	p.maxHealth, p.health = 1, 1

	a.sandboxTab = 1
	types := a.sandboxZombieTypes()
	if len(types) < 3 {
		t.Fatalf("only %d zombie types found", len(types))
	}
	before := len(p.zombies)
	a.sandboxItems()[0].do(a, true)
	if len(p.zombies) != before+5 {
		t.Fatalf("shift-click should spawn 5 zombies, got %d", len(p.zombies)-before)
	}

	a.sandboxTab = 2
	crates := len(p.scriptEntities)
	a.sandboxItems()[1].do(a, false)
	if len(p.scriptEntities) != crates+1 {
		t.Fatal("clicking a pickup should drop a crate")
	}
	a.sandboxItems()[2].do(a, true)
	if p.weapon.GunType == "PISTOL" {
		t.Fatal("shift-click should hand the weapon over")
	}

	a.sandboxTab = 0
	for _, item := range a.sandboxItems() {
		if item.label == "GOD MODE" {
			item.do(a, false)
		}
	}
	p.damagePlayer(0, .5)
	if p.health != 1 {
		t.Fatal("god mode should block damage")
	}
	p.cheats.freezeWaves = true
	elapsed := p.waveElapsed
	p.updateWaves()
	if p.waveElapsed != elapsed {
		t.Fatal("frozen waves should not advance")
	}
}
