package main

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
	if p.levelZombieTotal <= 0 {
		return 0, 0
	}
	remaining := 1 - min(float32(1), max(float32(0), float32(p.levelKills)/float32(p.levelZombieTotal)))
	return float64(remaining), uint8(int(float32(clampFloat(p.progressOpacity, 0, 1)) * 255))
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
