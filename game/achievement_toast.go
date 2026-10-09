package game

import (
	"image"
	"image/color"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
)

// An unlocked achievement slides a banner in from the top of the screen, on any
// screen (in a level, on the results screen, in the menus).
//
// Recovered from libmortargame 1.2.5: the achievement manager (FUN_00142844)
// loads "textures/achievements/banner" for its notifications and the sound
// name SFX_ACHIEVEMENT_UNLOCK / "achievement_unlocked" is registered in
// _INIT_84 (0x0012e848); the shipped clip is Common0/Sound/SFX/Achievement_unlocked.ogg.
// The notification layout, text positions, volume and slide timing live in the
// notification draw code that was not located (UNRESOLVED), so the banner art is
// the original's but the placement, text and timing below are the port's.
type achievementToast struct {
	entry formats.Achievement
	age   float64
}

// achievementUnlockSound is the original's unlock jingle (see above).
const achievementUnlockSound = "Achievement_unlocked"

// The wide banner piece inside Achievements/banner (512x64 SD sheet): columns
// 63..448, rows 0..56. The small badge at columns 0..54 is a second variant.
var achievementBannerSD = image.Rect(63, 0, 449, 57)

const (
	toastSlide = .3
	toastHold  = 3.8
	toastTotal = toastSlide + toastHold + toastSlide
)

// unlockAchievement marks an achievement earned and queues its banner.
func (a *app) unlockAchievement(entry formats.Achievement) bool {
	if a.achievementUnlocks[entry.ID] {
		return false
	}
	if a.achievementUnlocks == nil {
		a.achievementUnlocks = map[string]bool{}
	}
	a.achievementUnlocks[entry.ID] = true
	a.achievementToasts = append(a.achievementToasts, achievementToast{entry: entry})
	if !a.silent && a.options.sound {
		if a.pack != nil {
			a.playSound(a.scriptSoundPath(achievementUnlockSound), .8) // volume UNRESOLVED
		}
	}
	return true
}

func (a *app) updateAchievementToasts(dt float64) {
	if len(a.achievementToasts) == 0 {
		return
	}
	a.achievementToasts[0].age += dt
	if a.achievementToasts[0].age >= toastTotal {
		a.achievementToasts = a.achievementToasts[1:]
	}
}

func (a *app) drawAchievementToasts(screen *ebiten.Image) {
	if len(a.achievementToasts) == 0 {
		return
	}
	toast := a.achievementToasts[0]
	slide := 1.0
	switch {
	case toast.age < toastSlide:
		slide = toast.age / toastSlide
	case toast.age > toastSlide+toastHold:
		slide = (toastTotal - toast.age) / toastSlide
	}
	banner, err := a.Texture("Common0/Textures/Achievements/banner")
	if err != nil || banner == nil {
		a.drawAchievementToastPlate(screen, toast, slide)
		return
	}
	scale := float64(banner.Bounds().Dx()) / 512
	source := image.Rect(int(float64(achievementBannerSD.Min.X)*scale), 0, int(float64(achievementBannerSD.Max.X)*scale), int(float64(achievementBannerSD.Max.Y)*scale))
	width, height := float64(achievementBannerSD.Dx()), float64(achievementBannerSD.Dy())
	x := (logicalWidth - width) / 2
	// The art is a plate that hangs from the top edge of the screen (flat top,
	// rounded bottom), so it rests flush at y=0 instead of floating below it.
	y := -height + height*slide
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	options.GeoM.Scale(1/scale, 1/scale)
	options.GeoM.Translate(x, y)
	a.drawImage(screen, banner.SubImage(source).(*ebiten.Image), options)
	// The art already carries the trophy; the name and the unlocked text sit right of it.
	a.text(screen, toast.entry.Name, x+50, y+9, .42)
	a.text(screen, toast.entry.DisplayDescription(true), x+50, y+31, .3)
}

// drawAchievementToastPlate is the fallback when the banner texture is missing.
func (a *app) drawAchievementToastPlate(screen *ebiten.Image, toast achievementToast, slide float64) {
	const width, height = 250.0, 46.0
	x := (logicalWidth - width) / 2
	y := -height + (height+8)*slide
	a.drawRect(screen, x-1, y-1, width+2, height+2, color.RGBA{190, 200, 210, 255})
	a.drawRect(screen, x, y, width, height, color.RGBA{14, 18, 24, 245})
	a.drawAchievementIcon(screen, toast.entry, x+7, y+7, 32, true)
	a.text(screen, "ACHIEVEMENT UNLOCKED", x+48, y+6, .3)
	a.text(screen, toast.entry.Name, x+48, y+20, .42)
}
