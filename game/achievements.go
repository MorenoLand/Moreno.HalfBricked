package game

import (
	"fmt"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/achievements"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"image/color"
	"math"
)

func achievementTracked(entry formats.Achievement) bool {
	return entry.Type == "STORY" && entry.Check == "e" || entry.Type == "SPECIFIC" && entry.Check == "ge" && (entry.SpecificType == "total" || entry.SpecificType == "wave") || achievements.AchievementTrackingSupported(entry.Type, entry.Check, entry.SpecificType) || achievementEventTracked(entry)
}
func (a *app) updateGameplayAchievements(p *playState) error {
	if p == nil || p.paused {
		return nil
	}
	sample := p.achievementTracking.Update(achievements.AchievementTrackingFrame{PlayerPresent: true, Kills: p.combatKillCount, Health: float32(p.health), MotionX: p.achievementMotionX, MotionY: p.achievementMotionY, ScriptActive: p.scriptRuntime != nil && !p.scriptRuntime.Done(), Dt: float32(1.0 / 60.0)})
	changed := false
	for _, entry := range a.achievements {
		if a.achievementUnlocks[entry.ID] || (!sample.Meets(entry.Type, entry.Check, entry.SpecificType, entry.Total) && !a.playAchievementMet(p, entry)) {
			continue
		}
		a.unlockAchievement(entry)
		changed = true
	}
	if changed {
		return a.savePlayerProfile()
	}
	return nil
}
func (a *app) awardLocalAchievements(info *formats.LevelInfo) bool {
	a.bankLevelAchievementProgress(info)
	changed := false
	for _, entry := range a.achievements {
		if a.achievementUnlocks[entry.ID] || !achievementTracked(entry) {
			continue
		}
		met := false
		if entry.Type == "SPECIFIC" && (entry.SpecificType == "pistol_only" || entry.SpecificType == "continue" || entry.SpecificType == "death" || entry.SpecificType == "accuracy") {
			met = a.completionAchievementMet(entry, info)
		} else if entry.Type == "STORY" {
			met = info != nil && hasLevelFlag(*info, "ENDWORLD") && info.WorldIndex == entry.Total
		} else if entry.SpecificType == "total" {
			met = a.statistics.Available["Zombies Killed"] && a.statistics.ZombiesKilled >= int32(entry.Total)
		} else if entry.SpecificType == "wave" {
			met = a.statistics.Available["Highest Survival Wave"] && a.statistics.HighestSurvivalWave >= int32(entry.Total)
		}
		if met {
			a.unlockAchievement(entry)
			changed = true
		}
	}
	return changed
}
func (a *app) openAchievements() { a.achievementsBackPage, a.achievementOffset, a.page = a.page, 0, 6 }
func (a *app) closeAchievements() error {
	a.page = a.achievementsBackPage
	return a.savePlayerProfile()
}
func (a *app) updateAchievements() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return a.closeAchievements()
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := a.pointer()
		if x >= 380 && y >= 286 {
			return a.closeAchievements()
		}
	}
	if a.updateAchievementScrollbar() {
		return nil
	}
	delta := 0
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		delta++
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		delta--
	}
	_, wheel := ebiten.Wheel()
	if wheel < 0 {
		delta++
	} else if wheel > 0 {
		delta--
	}
	a.achievementOffset = clamp(a.achievementOffset+delta, 0, max(0, len(a.achievements)-5))
	return nil
}
func (a *app) drawAchievements(screen *ebiten.Image) {
	a.drawBackdrop(screen)
	a.textCentered(screen, "ACHIEVEMENTS", 14, .75)
	for row := 0; row < 5 && a.achievementOffset+row < len(a.achievements); row++ {
		entry := a.achievements[a.achievementOffset+row]
		y := float64(48 + row*45)
		a.drawRect(screen, 12, y-4, 440, 40, color.RGBA{0, 0, 0, 160})
		status := "UNTRACKED"
		if a.achievementUnlocks[entry.ID] {
			status = "UNLOCKED"
		} else if achievementTracked(entry) {
			status = "LOCKED"
		}
		a.drawAchievementIcon(screen, entry, 18, y-1, 32, a.achievementUnlocks[entry.ID])
		a.text(screen, entry.Name, 58, y, .45)
		description := entry.DisplayDescription(a.achievementUnlocks[entry.ID])
		if description == "" {
			description = fmt.Sprintf("%s %s %d %s", entry.Type, entry.Check, entry.Total, entry.SpecificType)
		}
		a.text(screen, a.wrapDialogue(description, 300, .3), 58, y+16, .3)
		a.text(screen, status, 365, y+19, .35)
	}
	a.drawAchievementScrollbar(screen)
	a.text(screen, fmt.Sprintf("%d-%d / %d", min(a.achievementOffset+1, len(a.achievements)), min(a.achievementOffset+5, len(a.achievements)), len(a.achievements)), 12, 294, .4)
	a.text(screen, "BACK", 400, 294, .5)
}

const (
	achievementRowsVisible = 5
	scrollbarX             = 460.0
	scrollbarWidth         = 8.0
	scrollbarTop           = 44.0
	scrollbarBottom        = 270.0
)

// achievementScrollbarThumb returns the thumb's top and height in logical pixels.
func achievementScrollbarThumb(total, offset int) (float64, float64) {
	track := scrollbarBottom - scrollbarTop
	if total <= achievementRowsVisible {
		return scrollbarTop, track
	}
	height := math.Max(24, track*achievementRowsVisible/float64(total))
	span := float64(total - achievementRowsVisible)
	return scrollbarTop + (track-height)*float64(offset)/span, height
}

// achievementOffsetForThumb maps a pointer y to the offset whose thumb is centred on it.
func achievementOffsetForThumb(total int, y float64) int {
	if total <= achievementRowsVisible {
		return 0
	}
	track := scrollbarBottom - scrollbarTop
	_, height := achievementScrollbarThumb(total, 0)
	fraction := (y - scrollbarTop - height/2) / (track - height)
	fraction = math.Max(0, math.Min(1, fraction))
	return int(math.Round(fraction * float64(total-achievementRowsVisible)))
}

func (a *app) updateAchievementScrollbar() bool {
	total := len(a.achievements)
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		a.achievementDrag = false
		return false
	}
	x, y := a.pointer()
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		a.achievementDrag = float64(x) >= scrollbarX-6 && float64(y) >= scrollbarTop && float64(y) <= scrollbarBottom
	}
	if !a.achievementDrag {
		return false
	}
	a.achievementOffset = achievementOffsetForThumb(total, float64(y))
	return true
}

func (a *app) drawAchievementScrollbar(screen *ebiten.Image) {
	a.drawRect(screen, scrollbarX, scrollbarTop, scrollbarWidth, scrollbarBottom-scrollbarTop, color.RGBA{0, 0, 0, 120})
	top, height := achievementScrollbarThumb(len(a.achievements), a.achievementOffset)
	thumb := color.RGBA{190, 225, 240, 230}
	if a.achievementDrag {
		thumb = color.RGBA{255, 255, 255, 255}
	}
	a.drawRect(screen, scrollbarX, top, scrollbarWidth, height, thumb)
}

// achievementIcon finds the achievement's art; each pack keeps its own under
// <Pack>/Textures/Achievements.
func (a *app) achievementIcon(texture string) *ebiten.Image {
	if texture == "" {
		texture = "Locked"
	}
	for _, pack := range []string{"Common0", "Common1", "Common2", "DLC1", "DLC2"} {
		if image, err := a.Texture(pack + "/Textures/Achievements/" + texture); err == nil {
			return image
		}
	}
	return nil
}

// drawAchievementIcon draws the icon in a size x size box; locked ones are dimmed.
func (a *app) drawAchievementIcon(screen *ebiten.Image, entry formats.Achievement, x, y, size float64, unlocked bool) {
	icon := a.achievementIcon(entry.Texture)
	if icon == nil {
		icon = a.achievementIcon("Locked")
	}
	if icon == nil {
		return
	}
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	options.GeoM.Scale(size/float64(icon.Bounds().Dx()), size/float64(icon.Bounds().Dy()))
	options.GeoM.Translate(x, y)
	if !unlocked {
		options.ColorScale.Scale(.45, .45, .45, 1)
	}
	a.drawImage(screen, icon, options)
}
