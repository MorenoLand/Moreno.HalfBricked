package game

import "testing"

// Everything the sandbox lists can be dropped and taken without trouble, and the list is complete for the
// pickups the port implements (shield, hover and the random crates were missing).
func TestSandboxPickupsDropAndCollect(t *testing.T) {
	a := newHeadlessApp(t, "bin/data-cache")
	if err := a.selectCaptureLevel("World0Level0"); err != nil {
		t.Fatal(err)
	}
	if err := a.openPlay(); err != nil {
		t.Fatal(err)
	}
	p := a.play
	p.closeScript()
	have := map[string]bool{}
	for _, name := range sandboxPickups {
		have[name] = true
		p.spawnPickup(name, p.spawnSpot(0))
		p.collectPickup(name)
	}
	for _, want := range []string{"p_shield", "p_hover", "p_health", "p_rand_1", "p_rand_8", "p_rand_all", "p_sentry"} {
		if !have[want] {
			t.Fatalf("sandbox is missing %s", want)
		}
	}
}

// Every zombie type the sandbox lists, bosses included, spawns and survives a few frames.
func TestSandboxSpawnsEveryZombieType(t *testing.T) {
	a := newHeadlessApp(t, "bin/data-cache")
	if err := a.selectCaptureLevel("World0Level0"); err != nil {
		t.Fatal(err)
	}
	if err := a.openPlay(); err != nil {
		t.Fatal(err)
	}
	p := a.play
	p.closeScript()
	p.cheats.god = true
	bosses := 0
	for _, entry := range a.sandboxZombieTypes() {
		if isBossType(entry.Name) {
			bosses++
		}
		p.spawnZombieAt(entry, p.spawnSpot(a.sandboxSpawned))
		a.sandboxSpawned++
	}
	if bosses == 0 {
		t.Fatal("the sandbox lists no bosses")
	}
	for i := 0; i < 120; i++ {
		p.updateZombies()
	}
}
