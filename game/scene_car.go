package game

import "strings"

// PORT ADDITION (not in the original binary; -scene-car-fix=false restores the original): the opening of
// World1Level2 (world1_level2_entry.script) creates its passing car with CreateEntity(playerX + 400, playerX + 64,
// "Characters/gangstercar", ...). The y argument is a typo for playerY + 64: with Barry at x = 797 it is 861, below
// the 736 px map, and the native CreateEntity (FUN_000daaac, entity 0x3d, drawn through FUN_000f21f4 with its world
// position) has no clamp, so the original draws the car off the map and only its screech and camera shake play.
// With the fix the car is created where the script clearly meant it, 64 px below Barry.
var portSceneCarFix = true

// sceneCarY returns the y a scripted gangster car is created at.
func (p *playState) sceneCarY(texture string, y float64) float64 {
	if !portSceneCarFix || p.world == nil || !strings.EqualFold(texture, "Characters/gangstercar") {
		return y
	}
	tileSize := p.tileSize
	if tileSize <= 0 {
		tileSize = 32
	}
	if y > float64(p.world.Level.Height*tileSize) {
		return p.y + 64
	}
	return y
}
