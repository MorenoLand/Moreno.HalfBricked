package game

import (
	"fmt"
	"image/color"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The wave editor: a level is a list of waves, a wave is a list of spawners, and a spawner drops a number of zombies (or
// pickups, or a boss) at one spawn zone of the collision layer, chosen by weight from its kinds.
//
//	wave:     spawn time (ms, the spawners' counts are spread over it), minimum length (ms), the "fewer than N alive" end of
//	          the native rule, and the wave to go to next (0 = the following one in the list)
//	spawner:  zone 1..13 (the collision layer marks zone n with the value n+2), count, start delay (ms), kinds
//	kind:     name (zombie, speedy_zombie, armed_zombie, a boss, a pickup name...), look (texture), weapon, weight, health,
//	          speed range, size

// editorVocab is every kind, look and weapon the game's own levels use, to cycle through.
type editorVocab struct {
	names, textures, weapons []string
}

func (a *app) editorVocab() editorVocab {
	e := a.editor
	if e.vocab != nil {
		return *e.vocab
	}
	counts := map[string]int{}
	textures, weapons := map[string]bool{"": true}, map[string]bool{"": true}
	for _, info := range a.levels {
		if info.ID == editorTestID {
			continue
		}
		level, err := a.pack.Load(info.ID)
		if err != nil {
			continue
		}
		for _, wave := range level.Waves {
			for _, spawner := range wave.Spawners {
				for _, kind := range spawner.Types {
					counts[kind.Name]++
					textures[kind.Texture] = true
					weapons[kind.Weapon] = true
				}
			}
		}
	}
	vocab := editorVocab{}
	for name := range counts {
		vocab.names = append(vocab.names, name)
	}
	sort.Slice(vocab.names, func(i, j int) bool {
		if counts[vocab.names[i]] != counts[vocab.names[j]] {
			return counts[vocab.names[i]] > counts[vocab.names[j]]
		}
		return vocab.names[i] < vocab.names[j]
	})
	for texture := range textures {
		vocab.textures = append(vocab.textures, texture)
	}
	for weapon := range weapons {
		vocab.weapons = append(vocab.weapons, weapon)
	}
	sort.Strings(vocab.textures)
	sort.Strings(vocab.weapons)
	e.vocab = &vocab
	return vocab
}

func defaultSpawner() formats.Spawner {
	return formats.Spawner{Index: 1, Count: 10, Types: []formats.SpawnType{defaultKind()}}
}

func defaultKind() formats.SpawnType {
	return formats.SpawnType{Name: "zombie", Chance: 1, Speed: formats.Vec2{X: 70, Y: 75}, Strength: 100, Size: formats.Vec2{X: 29, Y: 31}, TurnSpeed: 12}
}

func defaultWave() formats.Wave {
	return formats.Wave{RunTime: 10000, EndWaveTime: 15000, EndWaveZombies: 10, Spawners: []formats.Spawner{defaultSpawner()}}
}

func copyWave(wave formats.Wave) formats.Wave {
	copied := wave
	copied.Spawners = make([]formats.Spawner, len(wave.Spawners))
	for index, spawner := range wave.Spawners {
		copied.Spawners[index] = spawner
		copied.Spawners[index].Types = append([]formats.SpawnType(nil), spawner.Types...)
	}
	return copied
}

func copyWaves(waves []formats.Wave) []formats.Wave {
	copied := make([]formats.Wave, len(waves))
	for index, wave := range waves {
		copied[index] = copyWave(wave)
	}
	return copied
}

// waveZombieCount is how many zombies a wave sends: the counts of its spawners that do not drop pickups.
func waveZombieCount(wave formats.Wave) int {
	total := 0
	for _, spawner := range wave.Spawners {
		if len(spawner.Types) > 0 && !strings.HasPrefix(spawner.Types[0].Name, "p_") && spawner.Types[0].Name != "train" {
			total += spawner.Count
		}
	}
	return total
}

// zoneCells is how many cells of the collision layer mark the spawn zone.
func (e *levelEditor) zoneCells(zone int) int {
	count := 0
	for _, value := range e.level.Layers[formats.LayerC] {
		if value == uint32(zone+2) {
			count++
		}
	}
	return count
}

// waveChanged records an undo step for a change made in the wave editor.
func (e *levelEditor) waveChanged() {
	e.push()
}

func (e *levelEditor) clampWaveSelection() {
	if len(e.level.Waves) == 0 {
		e.waveSel, e.spawnSel, e.typeSel = 0, 0, 0
		return
	}
	e.waveSel = max(0, min(len(e.level.Waves)-1, e.waveSel))
	spawners := e.level.Waves[e.waveSel].Spawners
	e.spawnSel = max(0, min(max(0, len(spawners)-1), e.spawnSel))
	if len(spawners) > 0 {
		e.typeSel = max(0, min(max(0, len(spawners[e.spawnSel].Types)-1), e.typeSel))
	} else {
		e.typeSel = 0
	}
}

// ---- layout --------------------------------------------------------------------------------------------------------

type waveText struct {
	x, y, size float64
	text       string
	clr        color.RGBA
	bold       bool
}

type waveField struct {
	x, y, w, h float64
	id         string
	value      string
	tip        string
	commit     func(string) bool
}

type waveLayout struct {
	texts   []waveText
	buttons []edButton
	fields  []waveField
	rows    []waveRow // selectable list rows
}

type waveRow struct {
	x, y, w, h float64
	selected   bool
	title, sub string
	swatch     *color.RGBA
	tip        string
	do         func()
}

func shiftHeld() bool {
	return ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)
}

const (
	waveColA = 10.0
	waveColB = 266.0
	waveColC = 650.0
)

// waveLayout builds this frame's widgets from the level's waves; drawing and clicking both use it.
func (a *app) waveLayout() waveLayout {
	e := a.editor
	e.clampWaveSelection()
	var layout waveLayout
	text := func(x, y, size float64, s string, clr color.RGBA, bold bool) {
		layout.texts = append(layout.texts, waveText{x: x, y: y, size: size, text: s, clr: clr, bold: bold})
	}
	button := func(label, tip string, x, y, w, h float64, do func()) {
		layout.buttons = append(layout.buttons, edButton{label: label, tip: tip, x: x, y: y, w: w, h: h, do: do})
	}
	// step draws "label  [-] value [+]" for a number and lets the value be typed.
	step := func(x, y, w float64, label, tip, id string, get func() float64, set func(float64), unit float64, format func(float64) string) {
		text(x, y+5, 15, label, editorText, false)
		cx := x + w - 150
		change := func(direction float64) {
			amount := unit
			if shiftHeld() {
				amount *= 10
			}
			e.waveChanged()
			set(get() + direction*amount)
		}
		button("-", tip, cx, y, 26, 26, func() { change(-1) })
		layout.fields = append(layout.fields, waveField{x: cx + 29, y: y, w: 92, h: 26, id: id, value: format(get()), tip: tip + " Click to type a value.",
			commit: func(input string) bool {
				value, err := strconv.ParseFloat(strings.TrimSpace(input), 64)
				if err != nil {
					return false
				}
				e.waveChanged()
				set(value)
				return true
			}})
		button("+", tip, cx+124, y, 26, 26, func() { change(1) })
	}
	// cycle draws "label  [<] name [>]" over a list of choices.
	cycle := func(x, y, w float64, label, tip string, options []string, current string, set func(string)) {
		text(x, y+5, 15, label, editorText, false)
		cx := x + w - 168
		index := 0
		for i, option := range options {
			if option == current {
				index = i
			}
		}
		move := func(direction int) {
			if len(options) == 0 {
				return
			}
			e.waveChanged()
			set(options[(index+direction+len(options))%len(options)])
		}
		button("<", tip, cx, y, 26, 26, func() { move(-1) })
		button("", tip, cx+29, y, 110, 26, func() { move(1) }) // the name itself steps forward
		button(">", tip, cx+142, y, 26, 26, func() { move(1) })
	}
	seconds := func(v float64) string { return fmt.Sprintf("%.1f s", v) }
	whole := func(v float64) string { return fmt.Sprintf("%d", int(math.Round(v))) }

	waves := e.level.Waves
	// ---- column A: the waves ----
	text(waveColA, 48, 17, "Waves", editorText, true)
	total := 0
	for _, wave := range waves {
		total += waveZombieCount(wave)
	}
	const rowH, listTop, visible = 38.0, 76.0, 11
	e.waveTop = max(0, min(max(0, len(waves)-visible), e.waveTop))
	if e.waveSel < e.waveTop {
		e.waveTop = e.waveSel
	}
	if e.waveSel >= e.waveTop+visible {
		e.waveTop = e.waveSel - visible + 1
	}
	for row := e.waveTop; row < len(waves) && row < e.waveTop+visible; row++ {
		index := row
		layout.rows = append(layout.rows, waveRow{x: waveColA, y: listTop + float64(row-e.waveTop)*rowH, w: 240, h: rowH - 3, selected: row == e.waveSel,
			title: fmt.Sprintf("Wave %d", row+1), sub: fmt.Sprintf("%d zombies   %.0f s", waveZombieCount(waves[row]), waves[row].RunTime/1000),
			tip: "Select this wave", do: func() { e.waveSel, e.spawnSel, e.typeSel = index, 0, 0 }})
	}
	by := listTop + visible*rowH + 6
	button("Add", "Add a wave after the selected one (a copy of it)", waveColA, by, 54, 28, func() {
		e.waveChanged()
		fresh := defaultWave()
		if len(waves) > 0 {
			fresh = copyWave(waves[e.waveSel])
		}
		at := min(len(waves), e.waveSel+1)
		e.level.Waves = append(e.level.Waves[:at:at], append([]formats.Wave{fresh}, e.level.Waves[at:]...)...)
		e.waveSel, e.spawnSel, e.typeSel = at, 0, 0
	})
	button("Delete", "Delete the selected wave", waveColA+58, by, 62, 28, func() {
		if len(waves) == 0 {
			return
		}
		e.waveChanged()
		e.level.Waves = append(e.level.Waves[:e.waveSel:e.waveSel], e.level.Waves[e.waveSel+1:]...)
		e.clampWaveSelection()
	})
	button("Up", "Move the selected wave earlier", waveColA+124, by, 54, 28, func() {
		if e.waveSel > 0 && e.waveSel < len(waves) {
			e.waveChanged()
			waves[e.waveSel-1], waves[e.waveSel] = waves[e.waveSel], waves[e.waveSel-1]
			e.waveSel--
		}
	})
	button("Down", "Move the selected wave later", waveColA+182, by, 58, 28, func() {
		if e.waveSel >= 0 && e.waveSel < len(waves)-1 {
			e.waveChanged()
			waves[e.waveSel+1], waves[e.waveSel] = waves[e.waveSel], waves[e.waveSel+1]
			e.waveSel++
		}
	})
	text(waveColA, by+38, 14, fmt.Sprintf("%d waves, %d zombies in the level", len(waves), total), editorDim, false)
	if len(waves) > 0 {
		stats := waveNumbers(waves[e.waveSel])
		text(waveColA, by+60, 13, fmt.Sprintf("Wave %d: %d zombies, about %.0f health", e.waveSel+1, stats.zombies, stats.health), editorText, false)
		text(waveColA, by+77, 13, fmt.Sprintf("up to %.0f appear in one second", stats.peak), editorDim, false)
		text(waveColA, by+94, 13, fmt.Sprintf("the last one appears at %.1f s", stats.last/1000), editorDim, false)
	}
	if len(waves) == 0 {
		text(waveColB, 90, 16, "This level has no waves. Add one with the Add button.", editorDim, false)
		return layout
	}
	wave := &e.level.Waves[e.waveSel]

	// ---- column B: the wave and its spawners ----
	const bw = 370.0
	text(waveColB, 48, 17, fmt.Sprintf("Wave %d", e.waveSel+1), editorText, true)
	step(waveColB, 76, bw, "Spawn time", "How long the spawners spread their zombies over.", "run", func() float64 { return wave.RunTime / 1000 }, func(v float64) { wave.RunTime = math.Max(0, v) * 1000 }, .5, seconds)
	step(waveColB, 108, bw, "Minimum length", "The wave lasts at least this long before the next one can start.", "end", func() float64 { return wave.EndWaveTime / 1000 }, func(v float64) { wave.EndWaveTime = math.Max(0, v) * 1000 }, .5, seconds)
	step(waveColB, 140, bw, "Ends below (original rule)", "The original game ends the wave when fewer than this many are alive. This port waits for all of them.", "alive", func() float64 { return float64(wave.EndWaveZombies) }, func(v float64) { wave.EndWaveZombies = max(0, int(math.Round(v))) }, 1, whole)
	step(waveColB, 172, bw, "Next wave", "Following goes on to the next wave in the list; otherwise it jumps to the wave shown.", "next", func() float64 { return float64(wave.NextWave) }, func(v float64) { wave.NextWave = max(0, int(math.Round(v))) }, 1,
		func(v float64) string {
			if int(v) < 1 {
				return "following"
			}
			return fmt.Sprintf("jump to %d", int(v)+1)
		})

	text(waveColB, 214, 17, "Spawners", editorText, true)
	spawners := wave.Spawners
	const spawnRow, spawnTop, spawnVisible = 36.0, 242.0, 7
	e.spawnTop = max(0, min(max(0, len(spawners)-spawnVisible), e.spawnTop))
	if e.spawnSel < e.spawnTop {
		e.spawnTop = e.spawnSel
	}
	if e.spawnSel >= e.spawnTop+spawnVisible {
		e.spawnTop = e.spawnSel - spawnVisible + 1
	}
	for row := e.spawnTop; row < len(spawners) && row < e.spawnTop+spawnVisible; row++ {
		index := row
		s := spawners[row]
		swatch := collisionSwatch(uint32(s.Index + 2))
		kinds := "no kinds"
		if len(s.Types) > 0 {
			kinds = s.Types[0].Name
			if len(s.Types) > 1 {
				kinds += fmt.Sprintf(" and %d more", len(s.Types)-1)
			}
		}
		layout.rows = append(layout.rows, waveRow{x: waveColB, y: spawnTop + float64(row-e.spawnTop)*spawnRow, w: bw, h: spawnRow - 3, selected: row == e.spawnSel,
			title: fmt.Sprintf("Zone %d    x%d", s.Index, s.Count), sub: fmt.Sprintf("from %.1f s    %s", s.DelayTime/1000, kinds), swatch: &swatch,
			tip: "Select this spawner", do: func() { e.spawnSel, e.typeSel = index, 0 }})
	}
	sy := spawnTop + spawnVisible*spawnRow + 6
	button("Add", "Add a spawner (a copy of the selected one)", waveColB, sy, 70, 28, func() {
		e.waveChanged()
		fresh := defaultSpawner()
		if len(wave.Spawners) > 0 {
			fresh = copyWave(formats.Wave{Spawners: []formats.Spawner{wave.Spawners[e.spawnSel]}}).Spawners[0]
		}
		wave.Spawners = append(wave.Spawners, fresh)
		e.spawnSel, e.typeSel = len(wave.Spawners)-1, 0
	})
	button("Delete", "Delete the selected spawner", waveColB+74, sy, 80, 28, func() {
		if len(wave.Spawners) == 0 {
			return
		}
		e.waveChanged()
		wave.Spawners = append(wave.Spawners[:e.spawnSel:e.spawnSel], wave.Spawners[e.spawnSel+1:]...)
		e.clampWaveSelection()
	})
	if len(spawners) == 0 {
		return layout
	}
	spawner := &wave.Spawners[e.spawnSel]

	// ---- column C: the spawner and its kinds ----
	const cw = 300.0
	text(waveColC, 48, 17, fmt.Sprintf("Spawner %d", e.spawnSel+1), editorText, true)
	step(waveColC, 76, cw, "Zone", "Which spawn zone drops the zombies (1 to 13). Paint the zone on the collision layer.", "zone", func() float64 { return float64(spawner.Index) }, func(v float64) { spawner.Index = max(1, min(13, int(math.Round(v)))) }, 1, whole)
	step(waveColC, 108, cw, "Count", "How many it drops.", "count", func() float64 { return float64(spawner.Count) }, func(v float64) { spawner.Count = max(0, int(math.Round(v))) }, 1, whole)
	step(waveColC, 140, cw, "Starts at", "Seconds into the wave before it starts.", "delay", func() float64 { return spawner.DelayTime / 1000 }, func(v float64) { spawner.DelayTime = math.Max(0, v) * 1000 }, .5, seconds)
	zone := e.zoneCells(spawner.Index)
	if zone == 0 {
		text(waveColC, 172, 13, fmt.Sprintf("Zone %d is not painted on the map: nothing can spawn.", spawner.Index), color.RGBA{240, 120, 100, 255}, false)
	} else {
		text(waveColC, 172, 13, fmt.Sprintf("Zone %d covers %d cells of the map.", spawner.Index, zone), editorDim, false)
	}

	button("On map", "Go to this zone on the map: it is selected on the collision layer so you can paint or extend it", waveColC+cw-70, 168, 70, 24, func() { a.editorShowZone(spawner.Index) })
	if len(spawner.Types) > 0 {
		switch name := spawner.Types[0].Name; {
		case strings.HasPrefix(name, "boss_"):
			text(waveColC, 187, 12, "A boss appears in this zone: paint it where the fight should start.", editorDim, false)
		case name == "train":
			text(waveColC, 187, 12, "The train runs its own track from the game's data; it cannot be moved.", editorDim, false)
		}
	}
	text(waveColC, 200, 17, "Kinds", editorText, true)
	text(waveColC+60, 204, 13, "chosen by weight", editorDim, false)
	kindsTop, kindRow, kindVisible := 226.0, 32.0, 4
	e.typeTop = max(0, min(max(0, len(spawner.Types)-kindVisible), e.typeTop))
	if e.typeSel < e.typeTop {
		e.typeTop = e.typeSel
	}
	if e.typeSel >= e.typeTop+kindVisible {
		e.typeTop = e.typeSel - kindVisible + 1
	}
	for row := e.typeTop; row < len(spawner.Types) && row < e.typeTop+kindVisible; row++ {
		index := row
		k := spawner.Types[row]
		sub := "weight " + strconv.FormatFloat(k.Chance, 'f', -1, 64)
		if k.Texture != "" {
			sub += "    " + k.Texture
		}
		layout.rows = append(layout.rows, waveRow{x: waveColC, y: kindsTop + float64(row-e.typeTop)*kindRow, w: cw, h: kindRow - 3, selected: row == e.typeSel,
			title: k.Name, sub: sub, tip: "Select this kind", do: func() { e.typeSel = index }})
	}
	ky := kindsTop + float64(kindVisible)*kindRow + 4
	button("Add", "Add a kind (a copy of the selected one)", waveColC, ky, 60, 26, func() {
		e.waveChanged()
		fresh := defaultKind()
		if len(spawner.Types) > 0 {
			fresh = spawner.Types[e.typeSel]
		}
		spawner.Types = append(spawner.Types, fresh)
		e.typeSel = len(spawner.Types) - 1
	})
	button("Delete", "Delete the selected kind", waveColC+64, ky, 70, 26, func() {
		if len(spawner.Types) == 0 {
			return
		}
		e.waveChanged()
		spawner.Types = append(spawner.Types[:e.typeSel:e.typeSel], spawner.Types[e.typeSel+1:]...)
		e.clampWaveSelection()
	})
	if len(spawner.Types) == 0 {
		return layout
	}
	kind := &spawner.Types[e.typeSel]
	vocab := a.editorVocab()
	fy := ky + 36
	label := func(s string) string { // the empty choice reads "none"
		if s == "" {
			return "none"
		}
		return s
	}
	cycle(waveColC, fy, cw, "Kind", "What this drops: a zombie type, a boss, or a pickup.", vocab.names, kind.Name, func(s string) { kind.Name = s })
	cycle(waveColC, fy+30, cw, "Look", "The zombie's art (none uses the level's default).", vocab.textures, kind.Texture, func(s string) { kind.Texture = s })
	cycle(waveColC, fy+60, cw, "Weapon", "The weapon an armed zombie carries.", vocab.weapons, kind.Weapon, func(s string) { kind.Weapon = s })
	step(waveColC, fy+90, cw, "Weight", "How likely this kind is next to the others of the spawner.", "weight", func() float64 { return kind.Chance }, func(v float64) { kind.Chance = math.Max(0, v) }, 1, whole)
	step(waveColC, fy+120, cw, "Health", "Zombie health (100 is normal).", "health", func() float64 { return kind.Strength }, func(v float64) { kind.Strength = math.Max(0, v) }, 10, whole)
	step(waveColC, fy+150, cw, "Speed from", "Slowest walking speed.", "speedmin", func() float64 { return kind.Speed.X }, func(v float64) { kind.Speed.X = math.Max(0, v) }, 5, whole)
	step(waveColC, fy+180, cw, "Speed to", "Fastest walking speed.", "speedmax", func() float64 { return kind.Speed.Y }, func(v float64) { kind.Speed.Y = math.Max(0, v) }, 5, whole)
	// Show the names of the none-able choices as text over the cycle buttons.
	layout.texts = append(layout.texts,
		waveText{x: waveColC + cw - 168 + 33, y: fy + 5, size: 14, text: label(kind.Name), clr: editorText},
		waveText{x: waveColC + cw - 168 + 33, y: fy + 35, size: 14, text: label(kind.Texture), clr: editorText},
		waveText{x: waveColC + cw - 168 + 33, y: fy + 65, size: 14, text: label(kind.Weapon), clr: editorText})
	return layout
}

// ---- draw ----------------------------------------------------------------------------------------------------------

// drawWaveEditor draws the wave editor and returns the hover help of what the pointer is over.
func (a *app) drawWaveEditor(screen *ebiten.Image, px, py float64) string {
	e := a.editor
	layout := a.waveLayout()
	tip := ""
	a.vRect(screen, 0, editorTopBar, editorW, editorMapBottom-editorTopBar, editorBar)
	a.vRect(screen, waveColB-8, editorTopBar+8, 1, editorMapBottom-editorTopBar-16, color.RGBA{255, 255, 255, 18})
	a.vRect(screen, waveColC-8, editorTopBar+8, 1, editorMapBottom-editorTopBar-16, color.RGBA{255, 255, 255, 18})
	for _, row := range layout.rows {
		shade := editorBtn
		hover := px >= row.x && px < row.x+row.w && py >= row.y && py < row.y+row.h
		switch {
		case row.selected:
			shade = editorBtnOn
		case hover:
			shade = editorBtnHover
		}
		a.vRound(screen, row.x, row.y, row.w, row.h, 5, shade)
		left := row.x + 10
		if row.swatch != nil {
			a.vRound(screen, row.x+8, row.y+row.h/2-6, 12, 12, 3, *row.swatch)
			left = row.x + 28
		}
		a.vText(screen, row.title, left, row.y+3, 14, editorText, true)
		if row.h < 34 { // short rows keep the detail on the same line, to the right
			a.vText(screen, row.sub, row.x+row.w-a.vTextWidth(row.sub, 11, false)-10, row.y+7, 11, editorDim, false)
		} else {
			a.vText(screen, row.sub, left, row.y+row.h-15, 11, editorDim, false)
		}
		if hover {
			tip = row.tip
		}
	}
	for _, button := range layout.buttons {
		a.drawEditorButton(screen, button, px, py)
		if button.contains(px, py) {
			tip = button.tip
		}
	}
	for _, field := range layout.fields {
		hover := px >= field.x && px < field.x+field.w && py >= field.y && py < field.y+field.h
		shade := color.RGBA{28, 33, 46, 255}
		if hover {
			shade = color.RGBA{44, 52, 72, 255}
		}
		value := field.value
		editing := e.field == field.id
		if editing {
			shade = color.RGBA{20, 26, 40, 255}
			value = e.input + "|"
		}
		a.vRound(screen, field.x, field.y, field.w, field.h, 4, shade)
		if editing {
			a.vOutline(screen, field.x, field.y, field.w, field.h, 1.5, editorPlay)
		}
		a.vText(screen, value, field.x+(field.w-a.vTextWidth(value, 15, true))/2, field.y+4, 15, editorText, true)
		if hover {
			tip = field.tip
		}
	}
	for _, t := range layout.texts {
		a.vText(screen, t.text, t.x, t.y, t.size, t.clr, t.bold)
	}
	if len(e.level.Waves) > 0 {
		e.clampWaveSelection()
		a.drawWaveChart(screen, e.level.Waves[e.waveSel], waveColB, 536, 370, 74)
	}
	return tip
}

// ---- update --------------------------------------------------------------------------------------------------------

func (a *app) updateWaves(px, py float64) {
	e := a.editor
	layout := a.waveLayout()

	// Typing into a value.
	if e.field != "" {
		for _, r := range ebiten.AppendInputChars(nil) {
			if (r >= '0' && r <= '9') || r == '.' || r == '-' {
				e.input += string(r)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(e.input) > 0 {
			e.input = e.input[:len(e.input)-1]
		}
		commit := func() {
			for _, field := range layout.fields {
				if field.id == e.field && !field.commit(e.input) {
					e.say("Not a number: " + e.input)
				}
			}
			e.field = ""
		}
		switch {
		case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
			e.field = ""
		case inpututil.IsKeyJustPressed(ebiten.KeyEnter), inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter), inpututil.IsKeyJustPressed(ebiten.KeyTab):
			commit()
		case inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft):
			commit() // a click elsewhere commits, then acts
		default:
			return
		}
	}
	if _, wheel := ebiten.Wheel(); wheel != 0 {
		step := -int(math.Round(wheel * 2))
		switch {
		case px < waveColB-8:
			e.waveTop += step
		case px < waveColC-8:
			e.spawnTop += step
		default:
			e.typeTop += step
		}
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	for _, button := range layout.buttons {
		if button.contains(px, py) {
			button.do()
			return
		}
	}
	for _, field := range layout.fields {
		if px >= field.x && px < field.x+field.w && py >= field.y && py < field.y+field.h {
			e.field, e.input = field.id, ""
			return
		}
	}
	for _, row := range layout.rows {
		if px >= row.x && px < row.x+row.w && py >= row.y && py < row.y+row.h {
			row.do()
			return
		}
	}
}

// updateWaveMode runs the editor while the wave page is up: its own keys, the toolbar, and the page itself.
func (a *app) updateWaveMode(px, py float64, ctrl, shift bool) error {
	e := a.editor
	pressed := inpututil.IsKeyJustPressed
	if e.field == "" {
		switch {
		case pressed(ebiten.KeyEscape), pressed(ebiten.KeyE):
			e.wavePage = false
			return nil
		case pressed(ebiten.KeyF1):
			e.help = !e.help
			return nil
		case ctrl && pressed(ebiten.KeyZ) && shift, ctrl && pressed(ebiten.KeyY):
			e.doRedo()
			return nil
		case ctrl && pressed(ebiten.KeyZ):
			e.doUndo()
			return nil
		case ctrl && pressed(ebiten.KeyS) && shift:
			a.editorSaveToCache()
			return nil
		case ctrl && pressed(ebiten.KeyS):
			a.editorSaveAs()
			return nil
		case pressed(ebiten.KeyT):
			a.editorTest()
			return nil
		}
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && py < editorTopBar && e.field == "" {
		a.editorClickButtons(px, py)
		return nil
	}
	a.updateWaves(px, py)
	return nil
}

// editorShowZone leaves the wave editor for the map, with the zone's collision value selected and the view on its first cell.
func (a *app) editorShowZone(zone int) {
	e := a.editor
	e.wavePage, e.propMode = false, false
	e.layer, e.collision = formats.LayerC, uint32(zone+2)
	e.view.Layers[formats.LayerC] = true
	a.editorSetTool(toolPaint)
	for index, value := range e.level.Layers[formats.LayerC] {
		if value == uint32(zone+2) {
			v := e.view
			v.CameraX = float64(index%e.level.Width)*editorTile - (editorPanelX-editorToolStrip)/4/v.Zoom
			v.CameraY = float64(index/e.level.Width)*editorTile - (editorMapBottom-editorTopBar)/4/v.Zoom
			e.say(fmt.Sprintf("Zone %d is selected: paint more of it, or erase it and paint it elsewhere", zone))
			return
		}
	}
	e.say(fmt.Sprintf("Zone %d is not painted yet: click the map to paint it", zone))
}
