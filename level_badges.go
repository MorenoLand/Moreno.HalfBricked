package main

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"image"
	"math"
	"strings"
)

type levelBadgeState struct {
	Locked, NewDismissed bool
	Flags                uint32
}
type levelBadge struct {
	Texture                       string
	X, Y, Width, Height, Rotation float64
	Source                        image.Rectangle
}

func nativeCatalogLevelFlags(flags []string) uint32 {
	value, mask := strings.Join(flags, ";"), uint32(0)
	for _, flag := range []struct {
		name string
		mask uint32
	}{{"STORY", 1}, {"SURVIVAL", 2}, {"ENDWORLD", 4}, {"RATEONFINISH", 8}, {"ENDSTORY", 16}, {"STARTUNLOCKED", 32}, {"BEGINSTORY", 64}, {"SHOWCREDITS", 128}} {
		if strings.Contains(value, flag.name) {
			mask |= flag.mask
		}
	}
	return mask
}

func (a *app) drawCatalogLevelBadges(screen *ebiten.Image, item formats.LevelInfo, x, y, width, height float64) {
	a.drawLevelBadges(screen, a.catalogLevelBadgeState(item), x, y, width, height, float64(float32(a.menuTime)*3))
}
func (a *app) catalogLevelBadgeState(item formats.LevelInfo) levelBadgeState {
	return levelBadgeState{Locked: !a.levelUnlocked(item), NewDismissed: a.newDismissed[item.ID], Flags: nativeCatalogLevelFlags(item.Flags)}
}

func levelBadges(state levelBadgeState, x, y, width, height, tutorialWidth, tutorialHeight, phase float64) []levelBadge {
	badges := make([]levelBadge, 0, 3)
	locked := state.Locked && state.Flags&0x200 == 0
	if locked {
		badges = append(badges, levelBadge{Texture: "Common0/Textures/blank_SD", X: x, Y: y, Width: width / 1.5, Height: height, Source: image.Rect(0, 0, 64, 64)})
	}
	pulse := (.5 + math.Pow(math.Abs(math.Sin(phase))*.75, 2)) * .45
	if !state.NewDismissed && !locked && state.Flags&0x200 == 0 {
		badges = append(badges, levelBadge{Texture: "Common0/Textures/NewIcon_SD", X: x + width*float64(float32(.26)), Y: y - height*float64(float32(.4)), Width: width * float64(float32(1.1)) * pulse, Height: height * pulse, Rotation: float64(float32(.55))})
	}
	if state.Flags&0x400 != 0 {
		badges = append(badges, levelBadge{Texture: "Common0/Textures/TutorialIcon_SD", X: x + width*float64(float32(-.34)), Y: y - height*float64(float32(.32)), Width: tutorialWidth * float64(float32(1.9)) * pulse, Height: tutorialHeight * float64(float32(1.9)) * pulse, Rotation: float64(float32(-.55))})
	}
	return badges
}

func (a *app) drawLevelBadges(screen *ebiten.Image, state levelBadgeState, x, y, width, height, phase float64) {
	tutorialWidth, tutorialHeight := 0., 0.
	if state.Flags&0x400 != 0 {
		if texture, err := a.Texture("Common0/Textures/TutorialIcon_SD"); err == nil {
			resolution := a.pack.TextureSourceScale("Common0/Textures/TutorialIcon_SD")
			tutorialWidth, tutorialHeight = float64(texture.Bounds().Dx())/resolution, float64(texture.Bounds().Dy())/resolution
		}
	}
	for _, badge := range levelBadges(state, x, y, width, height, tutorialWidth, tutorialHeight, phase) {
		texture, err := a.Texture(badge.Texture)
		if err != nil {
			continue
		}
		if !badge.Source.Empty() {
			texture = texture.SubImage(resolutionRect(badge.Source, a.pack.TextureSourceScale(badge.Texture))).(*ebiten.Image)
		}
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Translate(-float64(texture.Bounds().Dx())/2, -float64(texture.Bounds().Dy())/2)
		options.GeoM.Scale(badge.Width/float64(texture.Bounds().Dx()), badge.Height/float64(texture.Bounds().Dy()))
		options.GeoM.Rotate(badge.Rotation)
		options.GeoM.Translate(badge.X, badge.Y)
		a.drawImage(screen, texture, options)
	}
}
