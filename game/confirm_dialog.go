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
	// native marks the QuitPrompt.uiscreen dialog: Quit / Back buttons, Back
	// focused by default (QuitPrompt.txt: Back IsDefault IsBack), Left/Right moves
	// focus and Enter presses the focused button.
	native          bool
	yesText, noText string
	focusYes        bool
}

func (a *app) askConfirm(title string, yes func() error) {
	a.confirm = &confirmDialog{title: title, yes: yes}
}

// askNativeQuit opens the QuitPrompt screen's dialog: the text and button labels
// come from QuitPrompt.uiscreen ("Quit Age of Zombies?", "Quit", "Back").
func (a *app) askNativeQuit(yes func() error) {
	title, yesText, noText := nativeQuitPromptText(a.loadQuitPromptScreen())
	a.confirm = &confirmDialog{title: title, yes: yes, native: true, yesText: yesText, noText: noText}
}

// nativeQuitPromptText reads the dialog strings out of QuitPrompt.uiscreen,
// falling back to the shipped English text when the pack has no screen.
func nativeQuitPromptText(screen *formats.UIScreen) (title, yes, no string) {
	title, yes, no = "Quit Age of Zombies?", "Quit", "Back"
	if screen == nil {
		return
	}
	if c := screen.Find("ComponentText"); c != nil && c.String("text") != "" {
		title = c.String("text")
	}
	if c := screen.Find("QuitYesButton"); c != nil && c.String("text") != "" {
		yes = c.String("text")
	}
	if c := screen.Find("QuitNoButton"); c != nil && c.String("text") != "" {
		no = c.String("text")
	}
	return
}

func (a *app) loadQuitPromptScreen() *formats.UIScreen {
	if a.pack == nil {
		return nil
	}
	path, ok := a.pack.SourcePath("Common0/UserInterface/screens/QuitPrompt.uiscreen")
	if !ok {
		return nil
	}
	reader, err := a.pack.Open(path)
	if err != nil {
		return nil
	}
	defer reader.Close()
	data, err := ioReadAll(reader)
	if err != nil {
		return nil
	}
	screen, err := formats.ParseUIScreen(data)
	if err != nil {
		return nil
	}
	return screen
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
	accept := uiKeyJustPressed(ebiten.KeyEnter) || uiKeyJustPressed(ebiten.KeyKPEnter) || uiKeyJustPressed(ebiten.KeyY)
	cancel := uiKeyJustPressed(ebiten.KeyEscape) || uiKeyJustPressed(ebiten.KeyN)
	if c.native {
		// Enter presses the focused button: Back unless focus moved to Quit.
		enter := uiKeyJustPressed(ebiten.KeyEnter) || uiKeyJustPressed(ebiten.KeyKPEnter)
		accept = uiKeyJustPressed(ebiten.KeyY) || (enter && c.focusYes)
		cancel = cancel || (enter && !c.focusYes)
		if uiKeyJustPressed(ebiten.KeyLeft) {
			c.focusYes = true // Back Left:Quit
		}
		if uiKeyJustPressed(ebiten.KeyRight) {
			c.focusYes = false // Quit Right:Back
		}
	}
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
	if !a.confirm.native {
		a.textCentered(screen, "Are you sure?", 142, .38)
	}
	pointerX, pointerY := a.pointer()
	yesButton, noButton := confirmButtons()
	if a.confirm.native {
		yesButton.label, noButton.label = a.confirm.yesText, a.confirm.noText
	}
	for _, button := range []lobbyButton{yesButton, noButton} {
		focused := a.confirm.native && (button.action == "yes") == a.confirm.focusYes
		a.drawLobbyButton(screen, button, focused || image.Pt(pointerX, pointerY).In(button.rect))
	}
}
