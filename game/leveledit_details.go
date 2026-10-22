package game

import (
	"fmt"
	"image/color"
	"sort"
	"unicode/utf8"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The level details dialog: the name, music, mode, world and what comes next, and the conversation the level opens with.

// speakers are the portraits an intro line can show (the id is the cameo texture's name).
var speakers = []struct{ id, label string }{
	{"", "No portrait"}, {"barrycameo", "Barry"}, {"barrycameosunnies", "Barry (sunnies)"}, {"zombiecameo", "Zombie"},
	{"cavezombiecameo", "Cave zombie"}, {"professorbrainscameo", "Professor Brains"}, {"hqcameo", "HQ"}, {"rexcameo", "Rex"},
	{"samuraicameo", "Samurai"}, {"pharohcameo", "Pharaoh"}, {"egyptianprincecameo", "Egyptian prince"},
	{"princeworkercameo", "Prince worker"}, {"japaneseboycameo", "Japanese boy"}, {"washington_zombiecameo", "Washington zombie"},
	{"finalbosscameo", "Final boss"},
}

// cameoTexture is the texture the dialogue box draws for a speaker id.
func cameoTexture(id string) string {
	if id == "" {
		return ""
	}
	return commonSDTexture("Cameos/" + id)
}

func speakerLabel(id string) string {
	for _, s := range speakers {
		if s.id == id {
			return s.label
		}
	}
	return id
}

type detailField struct {
	x, y, w, h float64
	id         string
	get        func() string
	set        func(string)
	max        int
	filter     func(rune) bool
	tip        string
}

type detailsLayout struct {
	texts   []waveText
	buttons []edButton
	fields  []detailField
}

func (a *app) openDetails() {
	e := a.editor
	e.detailsOpen, e.detailField, e.introSel = true, "", 0
}

func (a *app) detailsRect() (x, y, w, h float64) { return 50, 52, 860, 540 }

func idRune(r rune) bool {
	return r == '_' || r == '-' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// cycleChoice moves through a list of choices from the current one.
func cycleChoice(options []string, current string, step int) string {
	if len(options) == 0 {
		return current
	}
	index := 0
	for i, option := range options {
		if option == current {
			index = i
		}
	}
	return options[(index+step+len(options))%len(options)]
}

func (a *app) detailsLayout() detailsLayout {
	e := a.editor
	x, y, _, h := a.detailsRect()
	info := &e.level.Info
	var layout detailsLayout
	text := func(tx, ty, size float64, s string, clr color.RGBA, bold bool) {
		layout.texts = append(layout.texts, waveText{x: tx, y: ty, size: size, text: s, clr: clr, bold: bold})
	}
	field := func(label string, fx, fy, fw float64, id string, get func() string, set func(string), max int, filter func(rune) bool, tip string) {
		text(fx, fy-18, 13, label, editorDim, true)
		layout.fields = append(layout.fields, detailField{x: fx, y: fy, w: fw, h: 28, id: id, get: get, set: set, max: max, filter: filter, tip: tip})
	}
	chooser := func(label string, fx, fy, fw float64, options []string, get func() string, set func(string), show func(string) string, tip string) {
		text(fx, fy-18, 13, label, editorDim, true)
		layout.buttons = append(layout.buttons,
			edButton{icon: "minus", tip: tip, x: fx, y: fy, w: 30, h: 28, do: func() { e.waveChanged(); set(cycleChoice(options, get(), -1)) }},
			edButton{label: show(get()), tip: tip, x: fx + 33, y: fy, w: fw - 66, h: 28, do: func() { e.waveChanged(); set(cycleChoice(options, get(), 1)) }},
			edButton{icon: "plus", tip: tip, x: fx + fw - 30, y: fy, w: 30, h: 28, do: func() { e.waveChanged(); set(cycleChoice(options, get(), 1)) }})
	}

	left := x + 28
	text(left, y+18, 22, "Level details", editorText, true)
	field("Name shown in level select", left, y+72, 380, "name", func() string { return info.DisplayName }, func(v string) { info.DisplayName = v }, 40, nil, "The title of the level card")
	field("Description", left, y+128, 380, "desc", func() string { return info.Description }, func(v string) { info.Description = v }, 90, nil, "The line under the title")
	field("Id (the file name, letters and digits)", left, y+184, 380, "id", func() string { return info.ID }, func(v string) { info.ID = v }, 36, idRune, "Names the file when the level is added to the game")

	// mode
	text(left, y+222, 13, "Mode", editorDim, true)
	survival := false
	for _, flag := range info.Flags {
		if flag == "SURVIVAL" {
			survival = true
		}
	}
	setMode := func(on bool) {
		e.waveChanged()
		var flags []string
		for _, flag := range info.Flags {
			if flag != "STORY" && flag != "SURVIVAL" {
				flags = append(flags, flag)
			}
		}
		if on {
			flags = append(flags, "SURVIVAL")
		} else {
			flags = append(flags, "STORY")
		}
		info.Flags = flags
	}
	layout.buttons = append(layout.buttons,
		edButton{label: "Story", tip: "Plays in the story levels of the world", x: left, y: y + 244, w: 186, h: 30, active: func() bool { return !survival }, do: func() { setMode(false) }},
		edButton{label: "Survival", tip: "Plays in the survival maps of the world", x: left + 194, y: y + 244, w: 186, h: 30, active: func() bool { return survival }, do: func() { setMode(true) }})

	// world, music, postcard, next level
	var worldNames, music, postcards, levels []string
	for _, world := range a.worlds() {
		worldNames = append(worldNames, fmt.Sprint(world))
	}
	seen := map[string]bool{}
	levels = []string{""}
	for _, level := range a.editorLevels() {
		if level.ID != info.ID {
			levels = append(levels, level.ID)
		}
		if level.Music != "" && !seen["m"+level.Music] {
			seen["m"+level.Music] = true
			music = append(music, level.Music)
		}
		if level.PostcardImage != "" && !seen["p"+level.PostcardImage] {
			seen["p"+level.PostcardImage] = true
			postcards = append(postcards, level.PostcardImage)
		}
	}
	sort.Strings(music)
	sort.Strings(postcards)
	music = append([]string{""}, music...)
	postcards = append([]string{""}, postcards...)
	none := func(v string) string {
		if v == "" {
			return "none"
		}
		return v
	}
	chooser("World", left, y+312, 380, worldNames, func() string { return fmt.Sprint(info.WorldIndex) }, func(v string) { fmt.Sscan(v, &info.WorldIndex) },
		func(v string) string { var n int; fmt.Sscan(v, &n); return fmt.Sprintf("World %d", n+1) }, "Which world's level list it appears in")
	chooser("Music", left, y+368, 380, music, func() string { return info.Music }, func(v string) { info.Music = v }, none, "The track that plays during the level")
	chooser("Level card picture", left, y+424, 380, postcards, func() string { return info.PostcardImage }, func(v string) { info.PostcardImage = v }, none, "The picture on the level's card")
	chooser("Next level (story)", left, y+480, 380, levels, func() string { return info.NextLevel }, func(v string) { info.NextLevel = v }, none, "The level that unlocks after this one")

	// the intro conversation
	rx := x + 450
	text(rx, y+18, 22, "Opening conversation", editorText, true)
	intro := &e.level.Intro
	e.introSel = max(0, min(max(0, len(*intro)-1), e.introSel))
	const rowH = 62.0
	for index := 0; index < len(*intro) && index < 5; index++ {
		i := index
		rowY := y + 84 + float64(index)*rowH
		selected := i == e.introSel
		layout.buttons = append(layout.buttons, edButton{label: speakerLabel((*intro)[i].Cameo), tip: "Who is speaking: click to change the portrait", x: rx, y: rowY, w: 150, h: 26, active: func() bool { return selected },
			do: func() {
				e.introSel = i
				ids := make([]string, len(speakers))
				for n, s := range speakers {
					ids[n] = s.id
				}
				e.waveChanged()
				(*intro)[i].Cameo = cycleChoice(ids, (*intro)[i].Cameo, 1)
			}})
		layout.buttons = append(layout.buttons, edButton{label: "Select", tip: "Select this line", x: rx + 156, y: rowY, w: 70, h: 26, active: func() bool { return selected }, do: func() { e.introSel = i }})
		layout.fields = append(layout.fields, detailField{x: rx, y: rowY + 28, w: 386, h: 28, id: fmt.Sprintf("intro%d", i), get: func() string { return (*intro)[i].Text }, set: func(v string) { (*intro)[i].Text = v }, max: 200, tip: "What they say"})
	}
	by := y + 84 + 5*rowH + 4
	layout.buttons = append(layout.buttons,
		edButton{label: "Add line", icon: "plus", tip: "Add a line after the selected one", x: rx, y: by, w: 120, h: 30, do: func() {
			e.waveChanged()
			at := min(len(*intro), e.introSel+1)
			if len(*intro) == 0 {
				at = 0
			}
			fresh := formats.IntroLine{Cameo: "barrycameo", Text: "Here they come!"}
			*intro = append((*intro)[:at:at], append([]formats.IntroLine{fresh}, (*intro)[at:]...)...)
			e.introSel = at
		}},
		edButton{label: "Delete", icon: "trash", tip: "Remove the selected line", x: rx + 126, y: by, w: 110, h: 30, do: func() {
			if len(*intro) == 0 {
				return
			}
			e.waveChanged()
			*intro = append((*intro)[:e.introSel:e.introSel], (*intro)[e.introSel+1:]...)
		}},
		edButton{label: "Close", icon: "close", tip: "Done (Esc)", x: x + 860 - 150, y: y + h - 50, w: 124, h: 34, accent: true, do: func() { e.detailsOpen, e.detailField = false, "" }})
	return layout
}

// editorDialogs runs the dialogs of the map editor; it reports whether one took the frame.
func (a *app) editorDialogs(px, py float64) bool {
	e := a.editor
	switch {
	case e.detailsOpen:
		a.updateDetails(px, py)
		return true
	case e.testOpen:
		a.updateTestOptions(px, py)
		return true
	case e.propLibOpen:
		a.updatePropLibrary(px, py)
		return true
	case e.stampNaming:
		a.updateStampNaming()
		return true
	case e.shortcutsOpen:
		a.updateShortcuts(px, py)
		return true
	}
	return false
}

func (a *app) drawEditorDialogs(screen *ebiten.Image, px, py float64) {
	e := a.editor
	switch {
	case e.detailsOpen:
		a.drawEditorDetails(screen, px, py)
	case e.testOpen:
		a.drawTestOptions(screen, px, py)
	case e.propLibOpen:
		a.drawPropLibrary(screen, px, py)
	case e.stampNaming:
		a.drawStampNaming(screen)
	case e.shortcutsOpen:
		a.drawShortcuts(screen, px, py)
	}
}

func (a *app) updateDetails(px, py float64) {
	e := a.editor
	layout := a.detailsLayout()
	if e.detailField != "" {
		for _, field := range layout.fields {
			if field.id != e.detailField {
				continue
			}
			value := field.get()
			for _, r := range ebiten.AppendInputChars(nil) {
				if r < 32 || r == 127 || (field.filter != nil && !field.filter(r)) {
					continue
				}
				if utf8.RuneCountInString(value) < field.max {
					value += string(r)
				}
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && value != "" {
				runes := []rune(value)
				value = string(runes[:len(runes)-1])
			}
			if value != field.get() {
				field.set(value)
				e.dirty = true
			}
		}
		switch {
		case inpututil.IsKeyJustPressed(ebiten.KeyEscape), inpututil.IsKeyJustPressed(ebiten.KeyEnter), inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter):
			e.detailField = ""
			return
		case inpututil.IsKeyJustPressed(ebiten.KeyTab):
			for index, field := range layout.fields {
				if field.id == e.detailField {
					e.detailField = layout.fields[(index+1)%len(layout.fields)].id
					break
				}
			}
			return
		}
	} else if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		e.detailsOpen = false
		return
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	for _, button := range layout.buttons {
		if button.contains(px, py) {
			e.detailField = ""
			button.do()
			return
		}
	}
	for _, field := range layout.fields {
		if px >= field.x && px < field.x+field.w && py >= field.y && py < field.y+field.h {
			e.detailField = field.id
			return
		}
	}
	e.detailField = ""
}

func (a *app) drawEditorDetails(screen *ebiten.Image, px, py float64) {
	e := a.editor
	layout := a.detailsLayout()
	x, y, w, h := a.detailsRect()
	a.vRect(screen, 0, 0, editorW, editorH, color.RGBA{0, 0, 0, 175})
	a.vRound(screen, x-1, y-1, w+2, h+2, 11, color.RGBA{255, 255, 255, 30})
	a.vRound(screen, x, y, w, h, 10, color.RGBA{24, 29, 41, 255})
	a.vRect(screen, x+430, y+16, 1, h-70, color.RGBA{255, 255, 255, 20})
	for _, t := range layout.texts {
		a.vText(screen, t.text, t.x, t.y, t.size, t.clr, t.bold)
	}
	for _, button := range layout.buttons {
		a.drawEditorButton(screen, button, px, py)
	}
	for _, field := range layout.fields {
		focused := e.detailField == field.id
		shade := color.RGBA{28, 33, 46, 255}
		if focused {
			shade = color.RGBA{20, 26, 40, 255}
		}
		a.vRound(screen, field.x, field.y, field.w, field.h, 5, shade)
		if focused {
			a.vOutline(screen, field.x, field.y, field.w, field.h, 1.5, editorPlay)
		}
		value := field.get()
		if focused {
			value += "|"
		}
		for a.vTextWidth(value, 15, false) > field.w-16 && len(value) > 1 { // long text scrolls: its end stays visible
			value = value[1:]
		}
		a.vText(screen, value, field.x+8, field.y+5, 15, editorText, false)
	}
	rx := x + 450
	for index, line := range a.vWrap("Shown before the first wave, with the portrait of the speaker. Click or press Enter to move on.", 380, 13) {
		a.vText(screen, line, rx, y+46+float64(index)*15, 13, editorDim, false)
	}
	if len(e.level.Intro) == 0 {
		a.vText(screen, "No opening conversation. Add a line to start one.", rx, y+130, 15, editorDim, false)
	}
}
