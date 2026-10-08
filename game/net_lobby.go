package game

import (
	"fmt"
	"image"
	"image/color"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The online lobby: a titled panel over the menu backdrop with the code in a
// visible box (COPY) and a box for the other player's code (PASTE).

type lobbyButton struct {
	action  string
	label   string
	rect    image.Rectangle
	enabled bool
}

const (
	lobbyBoxLeft   = 36
	lobbyBoxRight  = 336
	lobbyButtonL   = 346
	lobbyButtonR   = 444
	lobbyButtonH   = 22
	lobbyBottomRow = 262
)

func (n *netSession) readyPeers() int {
	ready := 0
	for _, peer := range n.peers {
		if peer.link.Ready() {
			ready++
		}
	}
	return ready
}

// ownCode is the code this player has to hand over.
func (n *netSession) ownCode() string {
	if n.isHost {
		if n.pending != nil {
			return n.pending.Code
		}
		return ""
	}
	return n.reply
}

func (n *netSession) lobbyButtons() []lobbyButton {
	row := func(y int) image.Rectangle { return image.Rect(lobbyButtonL, y, lobbyButtonR, y+lobbyButtonH) }
	copyRow, pasteRow := 92, 150
	if !n.isHost {
		copyRow, pasteRow = 150, 92
	}
	buttons := []lobbyButton{
		{"copy", "COPY", row(copyRow), n.ownCode() != ""},
		{"paste", "PASTE", row(pasteRow), n.isHost && n.pending != nil || !n.isHost && n.link == nil && n.joining == nil},
	}
	bottom := func(index, count int, action, label string, enabled bool) lobbyButton {
		width, gap := 120, 12
		total := count*width + (count-1)*gap
		left := (logicalWidth-total)/2 + index*(width+gap)
		return lobbyButton{action, label, image.Rect(left, lobbyBottomRow, left+width, lobbyBottomRow+lobbyButtonH+4), enabled}
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

func (a *app) updateLobbyMouse() error {
	n := a.net
	if n.copied > 0 {
		n.copied--
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return nil
	}
	x, y := a.pointer()
	for _, button := range n.lobbyButtons() {
		if !button.enabled || !image.Pt(x, y).In(button.rect) {
			continue
		}
		switch button.action {
		case "copy":
			clipboardWrite(n.ownCode())
			n.copied = 90
		case "paste":
			clipboardRequest()
		case "start":
			return a.startNetHostGame()
		case "add":
			a.requestHostCode()
		case "back":
			a.leaveNet("cancelled")
		}
		return nil
	}
	return nil
}

var (
	lobbyPanelFill   = color.RGBA{18, 22, 28, 235}
	lobbyBoxFill     = color.RGBA{6, 8, 10, 255}
	lobbyBoxEdge     = color.RGBA{120, 130, 140, 255}
	lobbyEnabledEdge = color.RGBA{250, 235, 160, 255}
)

func shortCode(code string) string {
	const head, tail = 30, 8
	if len(code) <= head+tail+3 {
		return code
	}
	return code[:head] + "..." + code[len(code)-tail:]
}

func (a *app) drawLobbyBox(screen *ebiten.Image, y int, text string, muted bool) {
	a.drawRect(screen, lobbyBoxLeft-1, float64(y)-1, lobbyBoxRight-lobbyBoxLeft+2, lobbyButtonH+2, lobbyBoxEdge)
	a.drawRect(screen, lobbyBoxLeft, float64(y), lobbyBoxRight-lobbyBoxLeft, lobbyButtonH, lobbyBoxFill)
	if text == "" {
		return
	}
	scale := .34
	if muted {
		a.text(screen, text, lobbyBoxLeft+6, float64(y)+6, scale)
		return
	}
	a.text(screen, text, lobbyBoxLeft+6, float64(y)+6, scale)
}

func (a *app) drawLobbyButton(screen *ebiten.Image, button lobbyButton, hover bool) {
	if texture, err := a.Texture("Common0/Textures/Backing_Square"); err == nil {
		drawNineSlice(a, screen, texture, image.Rect(0, 0, 64, 64), button.rect)
	}
	if !button.enabled {
		a.drawRect(screen, float64(button.rect.Min.X), float64(button.rect.Min.Y), float64(button.rect.Dx()), float64(button.rect.Dy()), color.RGBA{0, 0, 0, 150})
	} else if hover {
		a.drawRect(screen, float64(button.rect.Min.X), float64(button.rect.Min.Y), float64(button.rect.Dx()), float64(button.rect.Dy()), color.RGBA{40, 40, 40, 40})
	}
	if a.font != nil && a.font.LineHeight > 0 {
		size := 13.0
		scale := size / float64(a.font.LineHeight)
		amount := a.hoverAmount("btn"+button.action+button.label, hover && button.enabled)
		centerX := float64(button.rect.Min.X) + float64(button.rect.Dx())/2
		top := float64(button.rect.Min.Y) + (float64(button.rect.Dy())-size)/2
		a.drawHoverText(screen, button.label, centerX, top, scale, amount)
	}
}

func (a *app) drawNetLobby(screen *ebiten.Image) {
	n := a.net
	screen.Fill(colorDark)
	a.drawBackdrop(screen)
	if boxes, err := a.Texture("Common0/Textures/TutorialBoxes"); err == nil {
		a.drawOptionsBoxStyle(screen, boxes, formats.Vec2{X: 240, Y: 160}, formats.Vec2{X: 440, Y: 292}, 0)
	} else {
		a.drawRect(screen, 20, 14, 440, 292, lobbyPanelFill)
	}
	title := "HOST ONLINE GAME"
	if !n.isHost {
		title = "JOIN ONLINE GAME"
	}
	a.textCentered(screen, title, 34, .6)

	step := func(y int, text string) { a.text(screen, text, lobbyBoxLeft, float64(y), .38) }
	codeText := shortCode(n.ownCode())
	pasteText := "(nothing pasted yet)"
	if n.pasted != "" {
		pasteText = shortCode(n.pasted)
	}
	if n.isHost {
		step(68, "1.  Send this code to your friend")
		if codeText == "" {
			codeText = "making your code..."
		}
		a.drawLobbyBox(screen, 92, codeText, false)
		step(126, "2.  Paste the reply code they send back")
		a.drawLobbyBox(screen, 150, pasteText, n.pasted == "")
	} else {
		step(68, "1.  Paste the code your host sent you")
		a.drawLobbyBox(screen, 92, pasteText, n.pasted == "")
		step(126, "2.  Send this reply code back to the host")
		if codeText == "" {
			codeText = "(appears after you paste the host's code)"
		}
		a.drawLobbyBox(screen, 150, codeText, false)
	}

	pointerX, pointerY := a.pointer()
	for _, button := range n.lobbyButtons() {
		a.drawLobbyButton(screen, button, button.enabled && image.Pt(pointerX, pointerY).In(button.rect))
	}
	if n.copied > 0 {
		a.text(screen, "Copied!", lobbyButtonL+30, 118, .34)
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
		a.textCentered(screen, status, 196, .38)
	}
	if n.problem != "" {
		a.textCentered(screen, n.problem, 218, .36)
	}
}
