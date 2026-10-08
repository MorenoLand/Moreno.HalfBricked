package game

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

// The p_shield pickup.
//
// Sources (libmortargame.so v7, image base 0x10000; Research/native/
// secondary-weapons-2026-10-08.md):
//
//   - Pickup update FUN_000928f4, type byte '$' (0x24, entry 30 of the entity
//     name table at 0x005cf1b8 = "p_shield"): the shield timer player+0x3e4 is
//     set (not added) to DAT_00092d34 = 0x41700000 = 15.0 s, the shield pickup
//     statistic (+0x130) is incremented, the text string 0x0056213d "Shield" is
//     posted and voice id 8 (SFX_VO_SHIELD) plays.
//   - Timer: FUN_00096818 lowers it by the frame time every update, floored at 0.
//   - Blocking: the player damage routine FUN_00094c6c does nothing to the health
//     while the timer is above 0, whatever hurt the player (zombie contact, enemy
//     and exploding-zombie blasts, the t-rex shockwave...). The port funnels all
//     player damage through damagePlayer, which already honours shielded().
//   - Draw: FUN_00097a24 draws the "Characters/shield" sheet (512x128, four
//     frames) as a 64x64 quad centred 25 above the player, frame
//     int(frac(timer*2)*4), and in the last 3 s alternates between visible and
//     hidden by ((int)(timer*10) & 1) == 0 (literals 0x00097b8c..9c).
//
// Not ported: FUN_0009f6e8 (a zombie within 4.8 px of a shielded player is pushed
// away by 0.025*dt of its contact vector), the zombie-hit velocity factor
// .985 instead of .9 (FUN_00094c6c), and the HUD shield gauge (FUN_0009532c).
const (
	shieldTexture     = "Common0/Textures/Characters/shield_SD"
	shieldSize        = 64.0
	shieldLiftOffset  = 25.0
	shieldBlinkWindow = 3.0
	shieldBlinkRate   = 10.0
	shieldFrames      = 4
	shieldFrameRate   = 2.0
)

// shieldFrame is the animation frame for a remaining timer.
func shieldFrame(timer float64) int {
	scaled := float32(timer) * shieldFrameRate
	frac := scaled - float32(int(scaled))
	return int(frac * shieldFrames)
}

// shieldVisible is false on the hidden half of the last-3-seconds blink.
func shieldVisible(timer float64) bool {
	if timer <= shieldBlinkWindow {
		return int(float32(timer)*shieldBlinkRate)&1 == 0
	}
	return true
}

// drawShield draws the shield bubble over Barry while the timer runs.
func (a *app) drawShield(screen *ebiten.Image) {
	p := a.play
	if p == nil || p.world == nil || !p.shielded() || !shieldVisible(p.achieve.shieldTimer) {
		return
	}
	texture, err := a.Texture(shieldTexture)
	if err != nil || texture.Bounds().Dx() < shieldFrames {
		return
	}
	cell := texture.Bounds().Dx() / shieldFrames
	frame := shieldFrame(p.achieve.shieldTimer)
	source := texture.SubImage(image.Rect(frame*cell, 0, (frame+1)*cell, texture.Bounds().Dy())).(*ebiten.Image)
	zoom := p.world.Zoom
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(cell)/2, -float64(texture.Bounds().Dy())/2)
	options.GeoM.Scale(shieldSize*zoom/float64(cell), shieldSize*zoom/float64(texture.Bounds().Dy()))
	options.GeoM.Translate((p.x-p.world.CameraX)*zoom+p.world.ViewportX, (p.y-shieldLiftOffset-p.world.CameraY)*zoom+p.world.ViewportY)
	a.drawImage(screen, source, options)
}
