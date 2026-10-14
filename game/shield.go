package game

import (
	"image"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
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
// FUN_0009f6e8 (a zombie within 4.8 px of a shielded player is pushed away by 0.025 of its overlap, zombie_ai.go
// zombieContact) and the HUD shield icon (FUN_0009532c, below) are ported (G14). The zombie-hit velocity factor .985
// instead of .9 (FUN_00094c6c) scales the player's +0x1c/+0x20/+0x24, which FUN_00096818 rewrites from the controls
// every frame before use (lines 276-291 vs 414-416): a native no-op, not applied.
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

// HUD shield icon (FUN_0009532c, v7): while the shield timer (+0x3e4) is above 0 and the HUD is visible, the HUD
// batch submits cell 1 of the 8-cell item sheet FUN_000b1a5c(0x11) returns (Common0/Textures/Weapons_Pickups: 1UP,
// shield, gift, ...) as a 32x32 quad at (134, 20) (literals 0x956cc = 134.0, 0x956d0 = 20.0), white, alpha =
// int(clamp(2 * timer, 0, 1) * 255): it is fully opaque until the last 0.5 s of the shield and fades out then. The source cell is 32 px wide in the SD sheet, 64 in HD.
const (
	shieldIconX    = 134.0
	shieldIconY    = 20.0
	shieldIconSize = 32.0
	shieldIconCell = 1
	shieldIconCols = 8
)

// shieldIconAlpha is the quad alpha for a timer value (0 when the shield is over).
func shieldIconAlpha(timer float64) float64 {
	if timer <= 0 {
		return 0
	}
	a := float32(timer) + float32(timer)
	if a >= 1 {
		a = 1
	}
	return float64(int(a*255)&0xff) / 255
}

// drawShieldHUD draws the HUD shield icon.
func (a *app) drawShieldHUD(screen *ebiten.Image) {
	p := a.play
	// v7 only: the 1.2.5 HUD functions (FUN_00132d9c kill meter, FUN_001330f8 lives, FUN_001333ec / FUN_00133624
	// weapon icons, FUN_00133884 hearts, FUN_00133b0c score) draw no shield icon.
	if p == nil || !p.hudVisible || !p.shielded() || p.waveBuild() != content.WaveBuildV7 {
		return
	}
	alpha := shieldIconAlpha(p.achieve.shieldTimer)
	if alpha <= 0 {
		return
	}
	texture, err := a.Texture("Common0/Textures/Weapons_Pickups_SD")
	if err != nil {
		if texture, err = a.Texture("Common0/Textures/Weapons_Pickups"); err != nil {
			return
		}
	}
	bounds := texture.Bounds()
	cell := bounds.Dx() / shieldIconCols
	if cell <= 0 || bounds.Dy() <= 0 {
		return
	}
	source := texture.SubImage(image.Rect(bounds.Min.X+shieldIconCell*cell, bounds.Min.Y, bounds.Min.X+(shieldIconCell+1)*cell, bounds.Max.Y)).(*ebiten.Image)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Scale(shieldIconSize/float64(source.Bounds().Dx()), shieldIconSize/float64(source.Bounds().Dy()))
	options.GeoM.Translate(shieldIconX, shieldIconY)
	options.ColorScale.ScaleAlpha(float32(alpha))
	a.drawImage(screen, source, options)
}
