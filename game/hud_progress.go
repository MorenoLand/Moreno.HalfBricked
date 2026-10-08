package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"image"
	"math"
)

var nativeZombieTypes = map[string]int{"zombie": 2, "shield_zombie": 3, "exploding_zombie": 4, "speedy_zombie": 5, "charging_zombie": 6, "armed_zombie": 7, "smart_zombie": 8, "prospector_zombie": 9, "boss_rex": 10, "boss_gangster": 11, "boss_egyptian": 12, "boss_samurai": 13, "boss_robot": 14, "boss_west": 15}

func levelZombieCount(waves []formats.Wave, survival bool) int {
	total, blocked := int32(0), survival
	for _, wave := range waves {
		for _, spawner := range wave.Spawners {
			eligible := true
			for _, entry := range spawner.Types {
				if blocked {
					eligible = false
					continue
				}
				kind, ok := nativeZombieTypes[entry.Name]
				if !ok || kind < 2 || kind > 15 {
					eligible = blocked
					continue
				}
				if kind >= 10 {
					eligible, blocked = false, true
				}
			}
			if eligible {
				total += int32(spawner.Count)
			}
		}
	}
	return int(total)
}
func (p *playState) updateProgressOpacity(dt float64) {
	opacity := float32(p.progressOpacity)
	if p.hudVisible {
		opacity = min(float32(1), opacity+3.5*float32(dt))
	} else {
		opacity = max(float32(0), opacity-2*float32(dt))
	}
	p.progressOpacity = float64(opacity)
}
func (p *playState) levelProgress() (float64, uint8) {
	alpha := uint8(int(float32(clampFloat(p.progressOpacity, 0, 1)) * 255))
	if p.levelZombieTotal <= 0 {
		// The original has no survival meter (its zombie total is zero there).
		// The port shows how much of the current wave is left instead.
		if fraction, ok := p.waveRemaining(); ok {
			return fraction, alpha
		}
		return 0, 0
	}
	remaining := 1 - min(float32(1), max(float32(0), float32(p.levelKills)/float32(p.levelZombieTotal)))
	return float64(remaining), alpha
}

func (p *playState) isSurvival() bool {
	for _, flag := range p.levelInfo.Flags {
		if flag == "SURVIVAL" {
			return true
		}
	}
	return false
}

// waveRemaining is the share of the current survival wave still to be beaten:
// zombies not yet spawned plus those still alive, out of the wave's total.
func (p *playState) waveRemaining() (float64, bool) {
	if p.progressMirror {
		return p.progressValue, true
	}
	if !p.isSurvival() || p.world == nil || p.waveIndex >= len(p.world.Level.Waves) {
		return 0, false
	}
	total, unspawned := 0, 0
	for index, spawner := range p.world.Level.Waves[p.waveIndex].Spawners {
		if spawner.Index < 1 || spawner.Index > 13 || spawner.Count <= 0 || len(spawner.Types) == 0 {
			continue
		}
		spawned := 0
		if index < len(p.waveSpawned) {
			spawned = p.waveSpawned[index]
		}
		total += spawner.Count
		unspawned += max(0, spawner.Count-spawned)
	}
	if total == 0 {
		return 0, false
	}
	alive := 0
	for _, zombie := range p.zombies {
		if !zombie.dying && !zombie.spawnAway && zombie.health > 0 {
			alive++
		}
	}
	return clampFloat(float64(unspawned+alive)/float64(total), 0, 1), true
}
func (a *app) drawLevelProgress(screen *ebiten.Image) {
	remaining, alpha := a.play.levelProgress()
	if alpha == 0 {
		return
	}
	scaleX, scaleY := a.renderScale()
	rotation := float64(0xbffd) * 2 * math.Pi / 65536
	border, err := a.Texture("Common0/Textures/healthBarBorder_SD")
	if err != nil {
		return
	}
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	op.ColorScale.ScaleAlpha(float32(alpha) / 255)
	op.GeoM.Translate(-float64(border.Bounds().Dx())/2, -float64(border.Bounds().Dy())/2)
	op.GeoM.Scale(16*scaleX/float64(border.Bounds().Dx()), 110*scaleX/float64(border.Bounds().Dy()))
	op.GeoM.Rotate(rotation)
	op.GeoM.Translate(240*scaleX, 40*scaleY)
	screen.DrawImage(border, op)
	if remaining > 0 {
		if fill, err := a.Texture("Common0/Textures/healthBar_SD"); err == nil {
			bounds := fill.Bounds()
			height := int(remaining * float64(bounds.Dy()))
			if height == 0 {
				height = bounds.Dy()
			}
			part := fill.SubImage(image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Max.X, bounds.Min.Y+height)).(*ebiten.Image)
			op.GeoM.Reset()
			op.GeoM.Translate(-float64(bounds.Dx())/2, -float64(height)/2)
			op.GeoM.Scale(16*scaleX/float64(bounds.Dx()), 110*remaining*scaleX/float64(height))
			op.GeoM.Rotate(rotation)
			op.GeoM.Translate((240+110*(remaining*.5-.5))*scaleX, 40*scaleY)
			screen.DrawImage(part, op)
		}
	}
	if icons, err := a.Texture("Common0/Textures/bossicons_SD"); err == nil {
		bounds := icons.Bounds()
		width := bounds.Dx() / 8
		if width > 0 {
			part := icons.SubImage(image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+width, bounds.Max.Y)).(*ebiten.Image)
			op.GeoM.Reset()
			op.GeoM.Translate(-float64(width)/2, -float64(bounds.Dy())/2)
			op.GeoM.Scale(31*scaleX/float64(width), 32*scaleX/float64(bounds.Dy()))
			op.GeoM.Translate((240+110*(remaining-.5))*scaleX, 40*scaleY)
			screen.DrawImage(part, op)
		}
	}
}
