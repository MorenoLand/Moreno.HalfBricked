package main

import (
	"fmt"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"image/color"
)

func achievementTracked(entry formats.Achievement) bool {
	return entry.Type == "STORY" && entry.Check == "e" || entry.Type == "SPECIFIC" && entry.Check == "ge" && (entry.SpecificType == "total" || entry.SpecificType == "wave") || achievementTrackingSupported(entry.Type, entry.Check, entry.SpecificType)
}
func (a *app) updateGameplayAchievements(p *playState) error {
	if p == nil || p.paused {
		return nil
	}
	sample := p.achievementTracking.Update(achievementTrackingFrame{playerPresent: true, kills: p.combatKillCount, health: float32(p.health), motionX: p.achievementMotionX, motionY: p.achievementMotionY, scriptActive: p.scriptRuntime != nil && !p.scriptRuntime.Done(), dt: float32(1.0 / 60.0)})
	changed := false
	for _, entry := range a.achievements {
		if a.achievementUnlocks[entry.ID] || !sample.Meets(entry.Type, entry.Check, entry.SpecificType, entry.Total) {
			continue
		}
		if a.achievementUnlocks == nil {
			a.achievementUnlocks = map[string]bool{}
		}
		a.achievementUnlocks[entry.ID], changed = true, true
	}
	if changed {
		return a.savePlayerProfile()
	}
	return nil
}
func (a *app) awardLocalAchievements(info *formats.LevelInfo) bool {
	changed := false
	for _, entry := range a.achievements {
		if a.achievementUnlocks[entry.ID] || !achievementTracked(entry) {
			continue
		}
		met := false
		if entry.Type == "STORY" {
			met = info != nil && hasLevelFlag(*info, "ENDWORLD") && info.WorldIndex == entry.Total
		} else if entry.SpecificType == "total" {
			met = a.statistics.Available["Zombies Killed"] && a.statistics.ZombiesKilled >= int32(entry.Total)
		} else if entry.SpecificType == "wave" {
			met = a.statistics.Available["Highest Survival Wave"] && a.statistics.HighestSurvivalWave >= int32(entry.Total)
		}
		if met {
			if a.achievementUnlocks == nil {
				a.achievementUnlocks = map[string]bool{}
			}
			a.achievementUnlocks[entry.ID], changed = true, true
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
		a.drawRect(screen, 12, y-4, 456, 40, color.RGBA{0, 0, 0, 160})
		status := "UNTRACKED"
		if a.achievementUnlocks[entry.ID] {
			status = "UNLOCKED"
		} else if achievementTracked(entry) {
			status = "LOCKED"
		}
		a.text(screen, entry.Name, 20, y, .45)
		description := entry.Description
		if description == "" {
			description = fmt.Sprintf("%s %s %d %s", entry.Type, entry.Check, entry.Total, entry.SpecificType)
		}
		a.text(screen, a.wrapDialogue(description, 340, .3), 20, y+16, .3)
		a.text(screen, status, 375, y+19, .35)
	}
	a.text(screen, fmt.Sprintf("%d-%d / %d", min(a.achievementOffset+1, len(a.achievements)), min(a.achievementOffset+5, len(a.achievements)), len(a.achievements)), 12, 294, .4)
	a.text(screen, "BACK", 400, 294, .5)
}
