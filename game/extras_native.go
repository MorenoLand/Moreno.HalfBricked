package game

import (
	"fmt"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// Port addition (HD cache only): the EXTRA page and its link drawn like the native menus. The link is a grey plate in
// the bottom-left corner of the options page (like the main menu's Quit plate) that lights under the pointer; the page
// is the NinePatchTest window with the heading, a slider for FOV, an ON/OFF button for Discord and a BACK button in a
// box below, sliding in and out with the other screens.
const (
	extraNatPanelX, extraNatPanelY, extraNatPanelW, extraNatPanelH = 60.0, 24.0, 360.0, 206.0
	extraNatFOVY, extraNatDiscordY                                 = 98.0, 142.0 // row centres
	extraNatTrackX0, extraNatTrackX1                               = 170.0, 340.0
	extraNatValueX                                                 = 392.0
)

// extraNative reports whether the page uses the native layout.
func (a *app) extraNative() bool { return a.pack != nil && a.titleNative() }

// extraLinkRectNative is the corner plate on the options page.
func extraLinkRectNative() (x0, y0, x1, y1 float64) { return 0, 298, 58, 320 }

func (a *app) extraRowTop(row int) float64 {
	if a.extraNative() {
		switch row {
		case extraRowFOV:
			return extraNatFOVY - 11
		case extraRowDiscord:
			return extraNatDiscordY - 11
		}
		return 400 // BACK is hit by its rectangle
	}
	return extraRowY(row)
}

func (a *app) extraTrack() (x0, x1 float64) {
	if a.extraNative() {
		return extraNatTrackX0, extraNatTrackX1
	}
	return extraTrackX0, extraTrackX1
}

func extraNatToggleRect() image.Rectangle {
	return image.Rect(300, int(extraNatDiscordY)-12, 396, int(extraNatDiscordY)+12)
}

func extraNatBackRect() image.Rectangle { return languagesBackRect() }

func (a *app) drawExtraLinkNative(screen *ebiten.Image) {
	x0, y0, x1, y1 := extraLinkRectNative()
	px, py := a.pointer()
	brighten := float32(0)
	if float64(px) >= x0 && float64(px) <= x1 && float64(py) >= y0 && float64(py) <= y1 {
		brighten = .2
	}
	// Drawn larger than the corner and clipped by the screen edge, so only the top-right corner shows.
	a.drawNativeNinePatchRect(screen, "Textures/HUD_ButtonNinePatch.tex", x0-12, y0, x1-x0+12, y1-y0+12, brighten)
	a.drawNativeSans(screen, "EXTRA", 24, y0+11, 44, "", 11, 64, 64, 64, nativeTextStyle{bold: true, em: 1.2})
}

func (a *app) drawExtrasNative(screen *ebiten.Image) {
	progress := lobbyNatEase(a.nativeSlide("extras", lobbyNatSlide))
	dy := -(1 - progress) * (extraNatPanelY + extraNatPanelH + 40)
	rightX := (1 - progress) * 520
	px, py := extraNatPanelX, extraNatPanelY+dy
	boxX := (logicalWidth - langBoxW) / 2
	bx, by := boxX+rightX, langBoxY

	a.drawBackdrop(screen)
	a.drawLobbyArt(screen, "Textures/HUD_Connector.tex", px-4, py+44, 52, 27, false)
	a.drawLobbyArt(screen, "Textures/HUD_Connector.tex", px-4, py+132, 52, 27, false)
	a.drawLobbyArt(screen, "Textures/HUD_WiresSmall2.tex", px+250, py-12, 40, 64, false)
	a.drawLobbyArt(screen, "Textures/HUD_WiresSmall1.tex", bx+langBoxW-8, by+14, 36, 50, false)
	a.drawLobbyArt(screen, "Textures/HUD_Connector.tex", bx+langBoxW+4, by+46, 40, 21, true)
	a.drawNativeNinePatchRect(screen, "Textures/NinePatchTest.tex", px, py, extraNatPanelW, extraNatPanelH, 0)
	a.drawNativeNinePatchRect(screen, "Textures/HUD_BoxNinePatch.tex", bx, by, langBoxW, langBoxH, 0)

	a.drawNativeSans(screen, "EXTRA", px+extraNatPanelW/2, py+22, extraNatPanelW, "", 18, nativeCyan[0], nativeCyan[1], nativeCyan[2], nativeHeadingStyle)
	a.drawLobbyArt(screen, "Textures/HUD_GreenLine.tex", px+extraNatPanelW/2, py+37, 300, 3, false)

	// Focus bar of the keyboard row.
	if a.extraRow == extraRowFOV || (a.extraRow == extraRowDiscord && discordAvailable()) {
		centre := extraNatFOVY
		if a.extraRow == extraRowDiscord {
			centre = extraNatDiscordY
		}
		a.drawRect(screen, px+14, centre-15+dy, extraNatPanelW-28, 30, color.RGBA{40, 70, 40, 140})
	}

	label := func(text string, cy float64) {
		a.drawNativeSans(screen, text, 80+80, cy+dy, 160, "CenterLeft", 13, 255, 255, 255, lobbyNatStepStyle)
	}
	// FOV: a slider with a plate for the knob.
	label("FOV", extraNatFOVY)
	t := float64(a.extras.fov()-fovMin) / float64(fovMax-fovMin)
	trackY := extraNatFOVY + dy
	a.drawRect(screen, extraNatTrackX0, trackY-2, extraNatTrackX1-extraNatTrackX0, 4, color.RGBA{14, 16, 16, 255})
	knobX := extraNatTrackX0 + t*(extraNatTrackX1-extraNatTrackX0)
	a.drawRect(screen, extraNatTrackX0, trackY-2, knobX-extraNatTrackX0, 4, color.RGBA{100, 228, 100, 255})
	a.drawNativeNinePatchRect(screen, "Textures/HUD_ButtonNinePatch.tex", knobX-6, trackY-10, 12, 20, 0)
	a.drawNativeSans(screen, fmt.Sprintf("%d", a.extras.fov()), extraNatValueX, trackY, 40, "", 13, 100, 228, 100, nativeTextStyle{bold: true, em: 1.2})

	pointerX, pointerY := a.pointer()
	if discordAvailable() { // the web build has no Discord option (discord_support.go)
		// Discord: an ON / OFF button.
		label("DISCORD RPC", extraNatDiscordY)
		toggle := extraNatToggleRect()
		hover := progress >= 1 && image.Pt(pointerX, pointerY).In(toggle)
		brighten := float32(0)
		if hover {
			brighten = .25
		}
		a.drawNativeNinePatchRect(screen, "Textures/HUD_ButtonNinePatch.tex", float64(toggle.Min.X), float64(toggle.Min.Y)+dy, float64(toggle.Dx()), float64(toggle.Dy()), brighten)
		state, r, g, b := "OFF", 150.0, 30.0, 30.0
		if a.extras.DiscordRPC {
			state, r, g, b = "ON", 10.0, 90.0, 10.0
		}
		a.drawNativeSans(screen, state, float64(toggle.Min.X)+float64(toggle.Dx())/2, float64(toggle.Min.Y)+float64(toggle.Dy())/2+dy, float64(toggle.Dx()), "", 12, r, g, b, nativeTextStyle{bold: true, em: 1.2})
	}

	a.drawNativeSans(screen, "FOV 90 is the original view.", px+extraNatPanelW/2, py+extraNatPanelH-18, extraNatPanelW-20, "", 10, 190, 190, 190, lobbyNatStepStyle)

	back := extraNatBackRect()
	backBrighten := float32(0)
	if progress >= 1 && image.Pt(pointerX, pointerY).In(back) {
		backBrighten = .25
	}
	x, y, w, h := float64(back.Min.X)+rightX, float64(back.Min.Y), float64(back.Dx()), float64(back.Dy())
	a.drawNativeNinePatchRect(screen, "Textures/HUD_ButtonNinePatch.tex", x, y, w, h, backBrighten)
	a.drawNativeSans(screen, "BACK", x+w/2, y+h/2, w, "", 11, 0, 0, 0, nativeTextStyle{em: 1.2, bold: true})
}
