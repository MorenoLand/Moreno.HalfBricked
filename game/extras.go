package game

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// PORT ADDITION (not in the original game): the "EXTRA" page reached from the options page. It is drawn with the
// game font like the rest of the menus and holds the settings the original has no place for:
//
//   - FOV: how much of the level the camera shows. 90 is the original view; a bigger value shows more of the map
//     (the zoom becomes 90 / FOV times the game's own zoom, applied in viewer.ZoomScale so scripted zooms and the
//     camera clamp keep working), a smaller one zooms in.
//   - DISCORD RPC: Discord Rich Presence (discord_rpc.go).
const (
	extraPage  = 8
	fovMin     = 60
	fovMax     = 120
	fovDefault = 90
	fovStep    = 5
)

type extraSettings struct {
	FOV        int  `json:"fov"`
	DiscordRPC bool `json:"discordRpc"`
}

func defaultExtras() extraSettings { return extraSettings{FOV: fovDefault, DiscordRPC: true} }

// fov is the FOV setting clamped to its range (0, as in a profile written before the page existed, is the default).
func (e extraSettings) fov() int {
	switch {
	case e.FOV <= 0:
		return fovDefault
	case e.FOV < fovMin:
		return fovMin
	case e.FOV > fovMax:
		return fovMax
	}
	return e.FOV
}

// fovZoomScale is the multiplier applied to the game's zoom.
func (e extraSettings) fovZoomScale() float64 { return float64(fovDefault) / float64(e.fov()) }

// Rows of the page and their layout in logical pixels.
const (
	extraRowFOV = iota
	extraRowDiscord
	extraRowBack
	extraRows
)

const (
	extraLabelX    = 46.0
	extraTrackX0   = 236.0
	extraTrackX1   = 396.0
	extraValueX    = 410.0
	extraToggleX   = 330.0
	extraRowHeight = 30.0
)

func extraRowY(row int) float64 {
	switch row {
	case extraRowFOV:
		return 104
	case extraRowDiscord:
		return 158
	}
	return 262
}

// extraButtonRect is the "EXTRA" button on the options page: inside the exit box, under DEFAULT and BACK.
func extraButtonRect() (x0, y0, x1, y1 float64) { return 318, 280, 418, 304 }

func (a *app) drawExtraButton(screen *ebiten.Image) {
	x0, y0, x1, _ := a.optionsExtraRect()
	if l := a.optionsLayout(); l != nil {
		// beside the native layout the link is a grey panel button with a bold black label
		if path := creditsCleanTexture(l.backButton.Texture()); path != "" {
			_, y0, _, y1 := a.optionsExtraRect()
			a.drawNativeNinePatchRect(screen, path, x0, y0, x1-x0, y1-y0, 0)
			a.drawNativeSans(screen, "Extra", (x0+x1)/2, (y0+y1)/2, x1-x0-6, "", 10, 0, 0, 0, nativeTextStyle{bold: true, em: 1.15})
			return
		}
	}
	label := "EXTRA"
	scale := .42
	a.text(screen, label, (x0+x1)/2-a.fontTextWidth(label, scale)/2, y0+2, scale)
}

// applyExtras pushes the settings to the running level and to Discord.
func (a *app) applyExtras() {
	if a.play != nil && a.play.world != nil {
		a.play.world.SetZoomScale(a.extras.fovZoomScale())
	}
	a.discord.Configure(a.extras.DiscordRPC)
}

func (a *app) leaveExtras() error {
	a.page = 3
	a.applyExtras()
	return a.savePlayerProfile()
}

func (a *app) setFOV(value int) {
	if value < fovMin {
		value = fovMin
	}
	if value > fovMax {
		value = fovMax
	}
	a.extras.FOV = value
	if a.play != nil && a.play.world != nil {
		a.play.world.SetZoomScale(a.extras.fovZoomScale()) // live preview behind the menu
	}
}

// updateExtras runs one frame of the page.
func (a *app) updateExtras() error {
	if uiKeyJustPressed(ebiten.KeyEscape) {
		return a.leaveExtras()
	}
	if uiKeyJustPressed(ebiten.KeyArrowUp) {
		a.extraRow = (a.extraRow + extraRows - 1) % extraRows
	}
	if uiKeyJustPressed(ebiten.KeyArrowDown) {
		a.extraRow = (a.extraRow + 1) % extraRows
	}
	if a.extraRow == extraRowFOV {
		if uiKeyJustPressed(ebiten.KeyArrowLeft) {
			a.setFOV(a.extras.fov() - fovStep)
		}
		if uiKeyJustPressed(ebiten.KeyArrowRight) {
			a.setFOV(a.extras.fov() + fovStep)
		}
	}
	activate := uiKeyJustPressed(ebiten.KeyEnter) || uiKeyJustPressed(ebiten.KeyKPEnter) || uiKeyJustPressed(ebiten.KeySpace)
	if a.extraRow == extraRowDiscord && (activate || uiKeyJustPressed(ebiten.KeyArrowLeft) || uiKeyJustPressed(ebiten.KeyArrowRight)) {
		a.extras.DiscordRPC = !a.extras.DiscordRPC
	}
	if a.extraRow == extraRowBack && activate {
		return a.leaveExtras()
	}
	px, py := a.pointer()
	x, y := float64(px), float64(py)
	pressed := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	if pressed {
		for row := 0; row < extraRows; row++ {
			if y >= extraRowY(row)-4 && y <= extraRowY(row)+extraRowHeight-4 {
				a.extraRow = row
				switch row {
				case extraRowFOV:
					a.extraDrag = x >= extraTrackX0-10 && x <= extraTrackX1+10
				case extraRowDiscord:
					a.extras.DiscordRPC = !a.extras.DiscordRPC
				case extraRowBack:
					return a.leaveExtras()
				}
			}
		}
	}
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		a.extraDrag = false
	}
	if a.extraDrag {
		t := (x - extraTrackX0) / (extraTrackX1 - extraTrackX0)
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		a.setFOV(fovMin + int(t*float64(fovMax-fovMin)+.5))
	}
	return nil
}

// drawExtras draws the page.
func (a *app) drawExtras(screen *ebiten.Image) {
	title := "EXTRA"
	a.text(screen, title, 240-a.fontTextWidth(title, .9)/2, 22, .9)
	row := func(index int, label string) {
		prefix := "  "
		if a.extraRow == index {
			prefix = "> "
		}
		a.text(screen, prefix+label, extraLabelX-a.fontTextWidth("> ", .5), extraRowY(index), .5)
	}
	row(extraRowFOV, "FOV")
	y := extraRowY(extraRowFOV)
	a.drawRect(screen, extraTrackX0, y+10, extraTrackX1-extraTrackX0, 4, color.RGBA{255, 255, 255, 110})
	t := float64(a.extras.fov()-fovMin) / float64(fovMax-fovMin)
	knob := extraTrackX0 + t*(extraTrackX1-extraTrackX0)
	a.drawRect(screen, extraTrackX0, y+10, knob-extraTrackX0, 4, color.RGBA{255, 220, 90, 255})
	a.drawRect(screen, knob-4, y+3, 8, 18, color.RGBA{255, 255, 255, 255})
	a.text(screen, fmt.Sprintf("%d", a.extras.fov()), extraValueX, y, .5)

	row(extraRowDiscord, "DISCORD RPC")
	state := "OFF"
	if a.extras.DiscordRPC {
		state = "ON"
	}
	a.text(screen, state, extraToggleX, extraRowY(extraRowDiscord), .5)

	row(extraRowBack, "BACK")
	hint := strings.ToUpper("FOV 90 is the original view")
	a.text(screen, hint, 240-a.fontTextWidth(hint, .3)/2, 296, .3)
}

// updateDiscordPresence refreshes the activity text about twice a second of game time.
func (a *app) updateDiscordPresence() {
	a.discordClock++
	if a.discordClock%30 != 1 {
		return
	}
	a.discord.Configure(a.extras.DiscordRPC)
	if !a.extras.DiscordRPC {
		return
	}
	if a.play == nil {
		a.discord.Set("In the menus", "")
		return
	}
	name := levelTextFlat(a.play.levelInfo.DisplayName, " - ")
	if name == "" {
		name = a.play.levelInfo.ID
	}
	state := fmt.Sprintf("Score %d", a.play.score)
	if a.mode == 1 {
		wave := a.play.waveIndex + 1
		state = fmt.Sprintf("Survival wave %d - score %d", wave, a.play.score)
	}
	a.discord.Set(name, state)
}
