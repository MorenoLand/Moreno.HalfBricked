package game

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// Port addition (HD cache only; the legacy lobby stays for SD): the online lobby drawn like the other 1.2.5 menus:
// the NinePatchTest window slides down with AnimateIn, the button box (HUD_BoxNinePatch with HUD_ButtonNinePatch
// buttons) slides in from the right, and the HUD connector / wire art sits behind them. The window is smaller than
// the old full-screen panel. There is no native lobby (the original has no online play); sizes follow the Credits
// and Statistics screens.
const (
	lobbyNatPanelX, lobbyNatPanelY, lobbyNatPanelW, lobbyNatPanelH = 60.0, 30.0, 360.0, 182.0
	lobbyNatBoxX, lobbyNatBoxY, lobbyNatBoxW, lobbyNatBoxH         = 80.0, 226.0, 320.0, 64.0
	lobbyNatSlide                                                  = .5
	lobbyNatTextX                                                  = 84
	lobbyNatCodeW                                                  = 212
	lobbyNatSideL, lobbyNatSideW                                   = 306, 90
	lobbyNatRowH                                                   = 22
	lobbyNatButtonY, lobbyNatButtonH, lobbyNatButtonW, lobbyNatGap = 238, 28, 98, 6
)

var lobbyNatStepStyle = nativeTextStyle{em: 1.2}

func lobbyNatEase(progress float64) float64 { return 1 - math.Pow(1-progress, 3) }

// lobbyButtonsNative are the lobby buttons of the native layout, at their resting positions.
func (n *netSession) lobbyButtonsNative() []lobbyButton {
	row := func(y int) image.Rectangle { return image.Rect(lobbyNatSideL, y, lobbyNatSideL+lobbyNatSideW, y+lobbyNatRowH) }
	copyRow, pasteRow := 100, 150
	if !n.isHost {
		copyRow, pasteRow = 150, 100
	}
	buttons := []lobbyButton{
		{"copy", "COPY", row(copyRow), n.ownCode() != ""},
		{"paste", "PASTE", row(pasteRow), n.isHost && n.pending != nil || !n.isHost && n.link == nil && n.joining == nil},
	}
	bottom := func(index, count int, action, label string, enabled bool) lobbyButton {
		total := count*lobbyNatButtonW + (count-1)*lobbyNatGap
		left := (logicalWidth-total)/2 + index*(lobbyNatButtonW+lobbyNatGap)
		return lobbyButton{action, label, image.Rect(left, lobbyNatButtonY, left+lobbyNatButtonW, lobbyNatButtonY+lobbyNatButtonH), enabled}
	}
	if n.isHost {
		buttons = append(buttons,
			bottom(0, 3, "start", "START", n.readyPeers() > 0),
			bottom(1, 3, "add", "ADD PLAYER", n.pending == nil && n.hosting == nil && len(n.peers) < maxNetGuests),
			bottom(2, 3, "back", "BACK", true))
	} else {
		buttons = append(buttons, bottom(0, 1, "back", "BACK", true))
	}
	return buttons
}

// drawLobbyArt draws a HUD texture centred on (cx, cy), w x h, optionally mirrored horizontally.
func (a *app) drawLobbyArt(screen *ebiten.Image, path string, cx, cy, w, h float64, mirror bool) {
	texture, err := a.Texture(nativeTextureName(path))
	if err != nil {
		return
	}
	bounds := texture.Bounds()
	sx, sy := w/float64(bounds.Dx()), h/float64(bounds.Dy())
	if mirror {
		sx = -sx
	}
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(bounds.Dx())/2, -float64(bounds.Dy())/2)
	options.GeoM.Scale(sx, sy)
	options.GeoM.Translate(cx, cy)
	a.drawImage(screen, texture, options)
}

func (a *app) drawLobbyCodeBox(screen *ebiten.Image, x, y float64, text string, muted bool) {
	a.drawRect(screen, x-1, y-1, lobbyNatCodeW+2, lobbyNatRowH+2, color.RGBA{96, 100, 98, 255})
	a.drawRect(screen, x, y, lobbyNatCodeW, lobbyNatRowH, color.RGBA{26, 28, 28, 255})
	r, g, b := 255.0, 255.0, 255.0
	if muted {
		r, g, b = 150, 154, 152
	}
	a.drawNativeSans(screen, text, x+6+(lobbyNatCodeW-12)/2, y+lobbyNatRowH/2, lobbyNatCodeW-12, "CenterLeft", 11, r, g, b, nativeTextStyle{em: 1.1})
}

func (a *app) drawNetLobbyNative(screen *ebiten.Image) {
	n := a.net
	progress := lobbyNatEase(a.nativeSlide("netlobby", lobbyNatSlide))
	dy := -(1 - progress) * (lobbyNatPanelY + lobbyNatPanelH + 40)
	rightX := (1 - progress) * 520
	px, py := lobbyNatPanelX, lobbyNatPanelY+dy
	boxX, boxW := lobbyNatBoxX, lobbyNatBoxW
	if !n.isHost { // one BACK button: a smaller box
		boxW = 150
		boxX = (logicalWidth - boxW) / 2
	}
	bx, by := boxX+rightX, lobbyNatBoxY

	a.drawBackdrop(screen)
	// Wires and connectors sit behind the windows.
	a.drawLobbyArt(screen, "Textures/HUD_Connector.tex", px-4, py+44, 52, 27, false)
	a.drawLobbyArt(screen, "Textures/HUD_Connector.tex", px-4, py+132, 52, 27, false)
	a.drawLobbyArt(screen, "Textures/HUD_WiresSmall2.tex", px+250, py-12, 40, 64, false)
	a.drawLobbyArt(screen, "Textures/HUD_WiresSmall1.tex", bx+boxW-8, by+14, 36, 50, false)
	a.drawLobbyArt(screen, "Textures/HUD_Connector.tex", bx+boxW+4, by+46, 40, 21, true)

	a.drawNativeNinePatchRect(screen, "Textures/NinePatchTest.tex", px, py, lobbyNatPanelW, lobbyNatPanelH, 0)
	a.drawNativeNinePatchRect(screen, "Textures/HUD_BoxNinePatch.tex", bx, by, boxW, lobbyNatBoxH, 0)

	title := "HOST ONLINE GAME"
	if !n.isHost {
		title = "JOIN ONLINE GAME"
	}
	a.drawNativeSans(screen, title, px+lobbyNatPanelW/2, py+22, lobbyNatPanelW, "", 18, nativeCyan[0], nativeCyan[1], nativeCyan[2], nativeHeadingStyle)
	a.drawLobbyArt(screen, "Textures/HUD_GreenLine.tex", px+lobbyNatPanelW/2, py+37, 300, 3, false)

	step := func(y float64, text string) {
		a.drawNativeSans(screen, text, lobbyNatTextX+150, y+dy, 300, "CenterLeft", 12, 255, 255, 255, lobbyNatStepStyle)
	}
	codeText := shortCode(n.ownCode())
	pasteText := "(nothing pasted yet)"
	if n.pasted != "" {
		pasteText = shortCode(n.pasted)
	}
	if n.isHost {
		step(88, "1.  Send this code to your friend")
		if codeText == "" {
			codeText = "making your code..."
		}
		a.drawLobbyCodeBox(screen, lobbyNatTextX, 100+dy, codeText, false)
		step(138, "2.  Paste the reply code they send back")
		a.drawLobbyCodeBox(screen, lobbyNatTextX, 150+dy, pasteText, n.pasted == "")
	} else {
		step(88, "1.  Paste the code your host sent you")
		a.drawLobbyCodeBox(screen, lobbyNatTextX, 100+dy, pasteText, n.pasted == "")
		step(138, "2.  Send this reply code back to the host")
		if codeText == "" {
			codeText = "(appears after you paste the host's code)"
		}
		a.drawLobbyCodeBox(screen, lobbyNatTextX, 150+dy, codeText, false)
	}

	pointerX, pointerY := a.pointer()
	for _, button := range n.lobbyButtonsNative() {
		rect := button.rect
		offX, offY := 0.0, dy
		if button.action != "copy" && button.action != "paste" {
			offX, offY = rightX, 0
		}
		hover := button.enabled && image.Pt(pointerX, pointerY).In(rect) && progress >= 1
		brighten := float32(0)
		switch {
		case !button.enabled:
			brighten = -.55
		case hover:
			brighten = .25
		}
		x, y, w, h := float64(rect.Min.X)+offX, float64(rect.Min.Y)+offY, float64(rect.Dx()), float64(rect.Dy())
		a.drawNativeNinePatchRect(screen, "Textures/HUD_ButtonNinePatch.tex", x, y, w, h, brighten)
		shade := 0.0 // the original's Back label is black on the plate
		if !button.enabled {
			shade = 90
		}
		a.drawNativeSans(screen, button.label, x+w/2, y+h/2, w, "", 11, shade, shade, shade, nativeTextStyle{em: 1.2, bold: true})
	}
	if n.copied > 0 {
		a.drawNativeSans(screen, "Copied!", lobbyNatSideL+lobbyNatSideW/2, 140+dy, 90, "", 10, 120, 255, 120, nativeTextStyle{em: 1.1})
	}

	status := n.note
	if n.isHost {
		switch ready := n.readyPeers(); {
		case ready > 0:
			status = fmt.Sprintf("%d player connected - press START when everyone is in", ready)
			if ready > 1 {
				status = fmt.Sprintf("%d players connected - press START when everyone is in", ready)
			}
		case n.pending != nil && n.pasted == "":
			status = "Your code is copied to the clipboard. Send it, then paste the reply."
		}
	}
	if status != "" {
		a.drawNativeSans(screen, status, px+lobbyNatPanelW/2, 188+dy, lobbyNatPanelW-20, "", 10, 255, 255, 255, lobbyNatStepStyle)
	}
	if n.problem != "" {
		a.drawNativeSans(screen, n.problem, px+lobbyNatPanelW/2, 201+dy, lobbyNatPanelW-20, "", 10, 255, 120, 110, lobbyNatStepStyle)
	}
}
