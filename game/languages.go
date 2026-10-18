package game

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Port addition (HD cache only): the Languages screen behind the main menu's LANGUAGES board, which the iPad build
// has. The Android 1.2.5 data holds no Languages screen and no translated game text (only the BrickNet account
// string tables, one per language below), so the screen lists those languages, marks the chosen one and saves it in
// the profile ("language"); every game string stays English until translations exist. The engine's own lookup
// (DEVICE_LANGUAGE, "languageSuffix", "languagemappings" in libmortargame.so) is not ported.
const languagesPage = 9

type languagesState struct {
	cursor int
	age    float64
}

type languageEntry struct{ code, name string }

// languageList follows the BrickNet_*.str table names of the 1.2.5 APK.
var languageList = []languageEntry{
	{"English_US", "English"}, {"French", "French"}, {"German", "German"}, {"Italian", "Italian"},
	{"Spanish", "Spanish"}, {"pt-br", "Portuguese (Brazil)"}, {"pl", "Polish"}, {"ru", "Russian"},
	{"tr", "Turkish"}, {"ar", "Arabic"}, {"Japanese", "Japanese"}, {"Korean", "Korean"},
	{"Chinese (Simplified)", "Chinese (Simplified)"}, {"zh-Hant", "Chinese (Traditional)"},
}

const (
	langPanelX, langPanelY, langPanelW, langPanelH = 60.0, 22.0, 360.0, 212.0
	langBoxW, langBoxH                             = 150.0, 64.0
	langBoxY                                       = 248.0
	langColumnX                                    = 80.0
	langRowTop, langRowH                           = 60.0, 20.0
	langColumnW                                    = 160.0
	langRows                                       = 7
)

func (a *app) openLanguages() {
	cursor := 0
	for i, entry := range languageList {
		if entry.code == a.language {
			cursor = i
		}
	}
	a.languages = &languagesState{cursor: cursor}
	a.page = languagesPage
	a.playSound("audio/sound/sfx/menu_transition_in.ogg", .7)
}

func (a *app) closeLanguages() error {
	a.languages, a.page = nil, 0
	a.playSound("audio/sound/sfx/menu_back.ogg", .8)
	a.playSound("audio/sound/sfx/menu_transition_out.ogg", .7)
	return a.savePlayerProfile()
}

// languageRect is the click rectangle of entry i at rest.
func languageRect(i int) image.Rectangle {
	column, row := i/langRows, i%langRows
	x := int(langColumnX) + column*int(langColumnW)
	y := int(langRowTop) + row*int(langRowH)
	return image.Rect(x, y, x+int(langColumnW)-6, y+int(langRowH)-1)
}

func languagesBackRect() image.Rectangle {
	w := 98
	left := (logicalWidth - w) / 2
	return image.Rect(left, 262, left+w, 290)
}

func (a *app) updateLanguages() error {
	s := a.languages
	s.age += 1.0 / 60.0
	a.menuTime += 1.0 / 60.0
	if uiKeyJustPressed(ebiten.KeyEscape) || uiKeyJustPressed(ebiten.KeyBackspace) {
		return a.closeLanguages()
	}
	if uiKeyJustPressed(ebiten.KeyUp) {
		s.cursor = (s.cursor + len(languageList) - 1) % len(languageList)
	}
	if uiKeyJustPressed(ebiten.KeyDown) {
		s.cursor = (s.cursor + 1) % len(languageList)
	}
	if uiKeyJustPressed(ebiten.KeyLeft) && s.cursor >= langRows {
		s.cursor -= langRows
	}
	if uiKeyJustPressed(ebiten.KeyRight) && s.cursor+langRows < len(languageList) {
		s.cursor += langRows
	}
	if uiKeyJustPressed(ebiten.KeyEnter) || uiKeyJustPressed(ebiten.KeyKPEnter) {
		a.chooseLanguage(s.cursor)
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && lobbyNatEase(a.nativeSlide("languages", lobbyNatSlide)) >= 1 {
		x, y := a.pointer()
		point := image.Pt(x, y)
		if point.In(languagesBackRect()) {
			return a.closeLanguages()
		}
		for i := range languageList {
			if point.In(languageRect(i)) {
				s.cursor = i
				a.chooseLanguage(i)
			}
		}
	}
	return nil
}

func (a *app) chooseLanguage(i int) {
	if i < 0 || i >= len(languageList) || a.language == languageList[i].code {
		return
	}
	a.language = languageList[i].code
	a.playSound("audio/sound/sfx/menu_select.ogg", .8)
	_ = a.savePlayerProfile()
}

func (a *app) drawLanguages(screen *ebiten.Image) {
	s := a.languages
	progress := lobbyNatEase(a.nativeSlide("languages", lobbyNatSlide))
	dy := -(1 - progress) * (langPanelY + langPanelH + 40)
	rightX := (1 - progress) * 520
	px, py := langPanelX, langPanelY+dy
	boxX := (logicalWidth - langBoxW) / 2
	bx, by := boxX+rightX, langBoxY

	a.drawBackdrop(screen)
	a.drawLobbyArt(screen, "Textures/HUD_Connector.tex", px-4, py+44, 52, 27, false)
	a.drawLobbyArt(screen, "Textures/HUD_Connector.tex", px-4, py+140, 52, 27, false)
	a.drawLobbyArt(screen, "Textures/HUD_WiresSmall2.tex", px+250, py-12, 40, 64, false)
	a.drawLobbyArt(screen, "Textures/HUD_WiresSmall1.tex", bx+langBoxW-8, by+14, 36, 50, false)
	a.drawLobbyArt(screen, "Textures/HUD_Connector.tex", bx+langBoxW+4, by+46, 40, 21, true)
	a.drawNativeNinePatchRect(screen, "Textures/NinePatchTest.tex", px, py, langPanelW, langPanelH, 0)
	a.drawNativeNinePatchRect(screen, "Textures/HUD_BoxNinePatch.tex", bx, by, langBoxW, langBoxH, 0)

	a.drawNativeSans(screen, "LANGUAGES", px+langPanelW/2, py+22, langPanelW, "", 18, nativeCyan[0], nativeCyan[1], nativeCyan[2], nativeHeadingStyle)
	a.drawLobbyArt(screen, "Textures/HUD_GreenLine.tex", px+langPanelW/2, py+37, 300, 3, false)

	pointerX, pointerY := a.pointer()
	for i, entry := range languageList {
		rect := languageRect(i)
		hover := progress >= 1 && image.Pt(pointerX, pointerY).In(rect)
		if i == s.cursor || hover {
			a.drawRect(screen, float64(rect.Min.X), float64(rect.Min.Y)+dy, float64(rect.Dx()), float64(rect.Dy()), color.RGBA{40, 70, 40, 140})
		}
		r, g, b := 255.0, 255.0, 255.0
		if entry.code == a.language || (a.language == "" && i == 0) {
			r, g, b = 100, 228, 100
		}
		a.drawNativeSans(screen, entry.name, float64(rect.Min.X)+float64(rect.Dx())/2+6, float64(rect.Min.Y)+float64(rect.Dy())/2+dy, float64(rect.Dx())-12, "CenterLeft", 12, r, g, b, lobbyNatStepStyle)
	}
	a.drawNativeSans(screen, "Translated text is not installed. Your choice is saved.", px+langPanelW/2, py+langPanelH-14, langPanelW-20, "", 10, 190, 190, 190, lobbyNatStepStyle)

	back := languagesBackRect()
	brighten := float32(0)
	if progress >= 1 && image.Pt(pointerX, pointerY).In(back) {
		brighten = .25
	}
	x, y, w, h := float64(back.Min.X)+rightX, float64(back.Min.Y), float64(back.Dx()), float64(back.Dy())
	a.drawNativeNinePatchRect(screen, "Textures/HUD_ButtonNinePatch.tex", x, y, w, h, brighten)
	a.drawNativeSans(screen, "BACK", x+w/2, y+h/2, w, "", 11, 0, 0, 0, nativeTextStyle{em: 1.2, bold: true})
}
