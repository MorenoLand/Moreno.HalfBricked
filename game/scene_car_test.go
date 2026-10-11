package game

import "testing"

func TestSceneCarIsCreatedOnTheMap(t *testing.T) {
	p := modelRig()
	p.y = 187
	height := float64(p.world.Level.Height * p.tileSize)
	if got := p.sceneCarY("Characters/gangstercar", height+125); got != 251 {
		t.Fatalf("car y %v, want playerY + 64", got)
	}
	if got := p.sceneCarY("Characters/gangstercar", 100); got != 100 {
		t.Fatal("an on-map y must be kept")
	}
	if got := p.sceneCarY("Characters/other", height+125); got != height+125 {
		t.Fatal("other sprites are not touched")
	}
	portSceneCarFix = false
	defer func() { portSceneCarFix = true }()
	if got := p.sceneCarY("Characters/gangstercar", height+125); got != height+125 {
		t.Fatal("the switch must restore the original y")
	}
}
