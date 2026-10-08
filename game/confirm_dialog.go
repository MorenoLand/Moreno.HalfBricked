package game

import (
	"image"
	"image/color"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// confirmDialog is the "are you sure?" box in front of the quit buttons (a port
// addition; the original ships a QuitPrompt screen for the desktop quit).
type confirmDialog struct {
	title string
	yes   func() error
}

func (a *app) askConfirm(title string, yes func() error) {
	a.confirm = &confirmDialog{title: title, yes: yes}
}

func confirmButtons() (yes, no lobbyButton) {
	yes = lobbyButton{action: "yes", label: "YES", rect: image.Rect(150, 168, 230, 192), enabled: true}
	no = lobbyButton{action: "no", label: "NO", rect: image.Rect(250, 168, 330, 192), enabled: true}
	return
}

// updateConfirm runs the dialog; it returns true while it owns the frame.
func (a *app) updateConfirm() (bool, error) {
	c := a.confirm
	if c == nil {
		return false, nil
	}
	yesButton, noButton := confirmButtons()
	accept := inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) || inpututil.IsKeyJustPressed(ebiten.KeyY)
	cancel := inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyN)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := a.pointer()
		switch {
		case image.Pt(x, y).In(yesButton.rect):
			accept = true
		case image.Pt(x, y).In(noButton.rect):
			cancel = true
		}
	}
	switch {
	case accept:
		a.confirm = nil
		return true, c.yes()
	case cancel:
		a.confirm = nil
	}
	return true, nil
}

func (a *app) drawConfirm(screen *ebiten.Image) {
	if a.confirm == nil {
		return
	}
	a.drawRect(screen, 0, 0, logicalWidth, logicalHeight, color.RGBA{0, 0, 0, 150})
	if boxes, err := a.Texture("Common0/Textures/TutorialBoxes"); err == nil {
		a.drawOptionsBoxStyle(screen, boxes, formats.Vec2{X: 240, Y: 150}, formats.Vec2{X: 250, Y: 110}, 0)
	} else {
		a.drawRect(screen, 115, 95, 250, 110, color.RGBA{18, 22, 28, 240})
	}
	a.textCentered(screen, a.confirm.title, 122, .5)
	a.textCentered(screen, "Are you sure?", 142, .38)
	pointerX, pointerY := a.pointer()
	yesButton, noButton := confirmButtons()
	for _, button := range []lobbyButton{yesButton, noButton} {
		a.drawLobbyButton(screen, button, image.Pt(pointerX, pointerY).In(button.rect))
	}
}
