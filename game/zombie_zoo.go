package game

import "github.com/MorenoLand/Moreno.HalfBricked/engine/formats"

// captureZombieZoo stages one risen zombie of every spawn-record type next to
// the player (capture state play-zoo): strengths 100..1000, sizes 29..45, the
// speedy afterimage and an armed zombie.
func (a *app) captureZombieZoo() error {
	a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
	if err := a.openPlay(); err != nil {
		return err
	}
	p := a.play
	p.closeScript()
	p.dialogueIndex = len(p.dialogue)
	p.hudVisible, p.moveControl, p.shootControl = true, true, true
	p.alertNoise = 1
	types := []formats.SpawnType{
		{Name: "zombie", Speed: formats.Vec2{X: 90, Y: 95}, Strength: 100, Size: formats.Vec2{X: 29, Y: 31}, TurnSpeed: 12, Texture: "cavezombie"},
		{Name: "zombie", Speed: formats.Vec2{X: 90, Y: 95}, Strength: 300, Size: formats.Vec2{X: 35, Y: 35}, TurnSpeed: 12, Texture: "mummy"},
		{Name: "zombie", Speed: formats.Vec2{X: 90, Y: 95}, Strength: 500, Size: formats.Vec2{X: 40, Y: 40}, TurnSpeed: 12, Texture: "cyborg"},
		{Name: "zombie", Speed: formats.Vec2{X: 90, Y: 95}, Strength: 700, Size: formats.Vec2{X: 45, Y: 45}, TurnSpeed: 12, Texture: "cyborg"},
		{Name: "speedy_zombie", Speed: formats.Vec2{X: 130, Y: 160}, Strength: 100, Size: formats.Vec2{X: 29, Y: 31}, TurnSpeed: 12, Texture: "japanzombie"},
		{Name: "armed_zombie", Speed: formats.Vec2{X: 90, Y: 95}, Strength: 100, Size: formats.Vec2{X: 29, Y: 31}, TurnSpeed: 12, Texture: "gungangster", Weapon: "PISTOL"},
	}
	for i, entry := range types {
		p.spawnZombieAt(entry, formats.Vec2{X: p.x - 150 + float64(i)*60, Y: p.y + 110})
		z := &p.zombies[len(p.zombies)-1]
		z.native.ai.riseMS = zombieRiseDoneMS - 1
	}
	return nil
}
