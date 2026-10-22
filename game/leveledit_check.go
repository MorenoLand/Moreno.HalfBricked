package game

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Checking a level before it is played: the problems list, the test options, and the numbers and chart of the waves.

// ---- problems ------------------------------------------------------------------------------------------------------

type editorProblem struct {
	severe     bool
	text       string
	cellX      int // a place on the map, -1 for none
	cellY      int
	wave       int // a spot in the wave editor, -1 for none
	spawner    int
	paintSpawn bool // fix: pick the player spawn to paint it
}

// passableCell is whether a walker can stand in the cell: the collision layer marks walls with 0 and 1.
func (e *levelEditor) passableCell(x, y int) bool {
	if x < 0 || y < 0 || x >= e.level.Width || y >= e.level.Height {
		return false
	}
	value := e.level.Layers[formats.LayerC][y*e.level.Width+x]
	return value != 0 && value != 1
}

// reachable is every cell the player can walk to from the spawn.
func (e *levelEditor) reachable() (map[[2]int]bool, bool) {
	spawn := [2]int{-1, -1}
	for index, value := range e.level.Layers[formats.LayerC] {
		if value == 2 {
			spawn = [2]int{index % e.level.Width, index / e.level.Width}
		}
	}
	if spawn[0] < 0 {
		return nil, false
	}
	seen := map[[2]int]bool{spawn: true}
	queue := [][2]int{spawn}
	for len(queue) > 0 {
		cell := queue[0]
		queue = queue[1:]
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			next := [2]int{cell[0] + d[0], cell[1] + d[1]}
			if !seen[next] && e.passableCell(next[0], next[1]) {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return seen, true
}

func (e *levelEditor) computeProblems() []editorProblem {
	var problems []editorProblem
	add := func(severe bool, text string, cellX, cellY, wave, spawner int) {
		problems = append(problems, editorProblem{severe: severe, text: text, cellX: cellX, cellY: cellY, wave: wave, spawner: spawner})
	}
	reach, hasSpawn := e.reachable()
	if !hasSpawn {
		problems = append(problems, editorProblem{severe: true, text: "There is no player spawn. Paint one on the collision layer.", cellX: -1, cellY: -1, wave: -1, paintSpawn: true})
	}
	if len(e.level.Waves) == 0 {
		add(true, "The level has no waves, so nothing will come at the player.", -1, -1, -1, -1)
	}
	zoneReached := func(zone int) (cells, reachedCells int, first [2]int) {
		first = [2]int{-1, -1}
		for index, value := range e.level.Layers[formats.LayerC] {
			if value == uint32(zone+2) {
				x, y := index%e.level.Width, index/e.level.Width
				cells++
				if first[0] < 0 {
					first = [2]int{x, y}
				}
				if reach[[2]int{x, y}] {
					reachedCells++
				}
			}
		}
		return
	}
	for w, wave := range e.level.Waves {
		if len(wave.Spawners) == 0 {
			add(false, fmt.Sprintf("Wave %d has no spawners.", w+1), -1, -1, w, -1)
		}
		if wave.NextWave >= len(e.level.Waves) {
			add(false, fmt.Sprintf("Wave %d goes on to wave %d, which does not exist.", w+1, wave.NextWave+1), -1, -1, w, -1)
		}
		for s, spawner := range wave.Spawners {
			where := fmt.Sprintf("Wave %d, spawner %d", w+1, s+1)
			switch {
			case spawner.Index < 1 || spawner.Index > 13:
				add(true, fmt.Sprintf("%s: zone %d is not between 1 and 13.", where, spawner.Index), -1, -1, w, s)
				continue
			case len(spawner.Types) == 0:
				add(false, where+" has no kinds, so it does nothing (the game's own last waves have such spawners).", -1, -1, w, s)
				continue
			}
			cells, reached, first := zoneReached(spawner.Index)
			switch {
			case cells == 0:
				add(true, fmt.Sprintf("%s: zone %d is not painted on the map, so nothing spawns.", where, spawner.Index), -1, -1, w, s)
			case hasSpawn && reached == 0:
				add(false, fmt.Sprintf("%s: zone %d is sealed off from the player by walls.", where, spawner.Index), first[0], first[1], w, s)
			}
			weight := 0.0
			for _, kind := range spawner.Types {
				weight += kind.Chance
				if kind.Name == "" {
					add(true, where+" has a kind with no name.", -1, -1, w, s)
				}
			}
			if weight <= 0 {
				add(false, where+": every kind has weight 0, so none is ever chosen.", -1, -1, w, s)
			}
		}
	}
	width, height := float64(e.level.Width)*editorTile, float64(e.level.Height)*editorTile
	for _, prop := range e.level.Props {
		if prop.X < 0 || prop.Y < 0 || prop.X > width || prop.Y > height {
			add(false, "A prop is outside the map.", int(prop.X/editorTile), int(prop.Y/editorTile), -1, -1)
			break
		}
	}
	for _, pickup := range e.level.Pickups {
		cx, cy := int(pickup.X/editorTile), int(pickup.Y/editorTile)
		if !e.passableCell(cx, cy) {
			add(false, fmt.Sprintf("The %s crate sits in a wall or outside the map.", pickup.Name), cx, cy, -1, -1)
		}
	}
	return problems
}

// problemBadge is the count shown on the Problems button ("" when all is well).
func (a *app) problemBadge() string {
	e := a.editor
	if e.view == nil {
		return ""
	}
	e.problemFrame++
	if e.problemFrame%20 == 1 || e.problemCache == nil {
		e.problemCache = e.computeProblems()
	}
	if len(e.problemCache) == 0 {
		return ""
	}
	return fmt.Sprint(len(e.problemCache))
}

// openProblems lists what is wrong; a click goes to the place.
func (a *app) openProblems(x float64) {
	e := a.editor
	problems := e.computeProblems()
	e.problemCache = problems
	var items []edMenuItem
	if len(problems) == 0 {
		items = []edMenuItem{{label: "No problems found", sub: "all good", disabled: true}}
	}
	for _, problem := range problems {
		pr := problem
		red, amber := color.RGBA{230, 90, 80, 255}, color.RGBA{236, 176, 60, 255}
		swatch := &amber
		if pr.severe {
			swatch = &red
		}
		items = append(items, edMenuItem{label: pr.text, swatch: swatch, sub: "go", tip: "Go to the place", do: func() { a.editorGoto(pr) }})
	}
	popup := a.openPopup("Problems", x-120, 620, items)
	popup.rowH = 34
}

// editorGoto takes the editor to where a problem is.
func (a *app) editorGoto(p editorProblem) {
	e := a.editor
	switch {
	case p.paintSpawn:
		e.wavePage, e.propMode = false, false
		e.layer, e.collision = formats.LayerC, 2
		e.view.Layers[formats.LayerC] = true
		a.editorSetTool(toolPaint)
		e.say("Click the map to place the player spawn")
	case p.wave >= 0:
		e.wavePage, e.waveSel, e.spawnSel = true, p.wave, max(0, p.spawner)
		e.field = ""
	case p.cellX >= 0:
		e.wavePage = false
		v := e.view
		v.CameraX = float64(p.cellX)*editorTile - (editorPanelX-editorToolStrip)/4/v.Zoom
		v.CameraY = float64(p.cellY)*editorTile - (editorMapBottom-editorTopBar)/4/v.Zoom
	}
}

// ---- test options --------------------------------------------------------------------------------------------------

// editorTestStart is where a test play starts when "start at the view" is on (the middle of the map area), nudged out of
// walls.
func (e *levelEditor) viewCentre() (float64, float64) {
	v := e.view
	cx, cy := (editorToolStrip+editorPanelX)/4, (editorTopBar+editorMapBottom)/4
	x, y := (cx-v.ViewportX)/v.Zoom+v.CameraX, (cy-v.ViewportY)/v.Zoom+v.CameraY
	tx, ty := int(math.Floor(x/editorTile)), int(math.Floor(y/editorTile))
	if !e.passableCell(tx, ty) {
		best, bestDistance := [2]int{-1, -1}, 1e9
		for yy := 0; yy < e.level.Height; yy++ {
			for xx := 0; xx < e.level.Width; xx++ {
				if e.passableCell(xx, yy) {
					if d := math.Hypot(float64(xx-tx), float64(yy-ty)); d < bestDistance {
						best, bestDistance = [2]int{xx, yy}, d
					}
				}
			}
		}
		if best[0] >= 0 {
			tx, ty = best[0], best[1]
		}
	}
	return (float64(tx) + .5) * editorTile, (float64(ty) + .5) * editorTile
}

// applyTestOptions sets up the freshly started test game as the options say.
func (a *app) applyTestOptions() {
	e := a.editor
	if a.play == nil {
		return
	}
	if e.testGod {
		a.play.cheats.god = true
	}
	if e.testWave > 1 && e.testWave <= len(e.level.Waves) {
		a.play.waveIndex = e.testWave - 1
		a.play.waveElapsed = 0
		a.play.spawnLevelPickups(e.testWave)
	}
	if e.testHere || e.testAtCursor {
		x, y := e.testX, e.testY
		if !e.testAtCursor {
			x, y = e.viewCentre()
		}
		a.play.x, a.play.y, a.play.spawnX, a.play.spawnY = x, y, x, y
	}
	e.testAtCursor = false
}

func (a *app) testOptionsRect() (x, y, w, h float64) { return 640, editorTopBar + 4, 310, 256 }

func (a *app) testOptionsButtons() []edButton {
	e := a.editor
	x, y, w, _ := a.testOptionsRect()
	check := func(label, tip string, row float64, get func() bool, set func()) edButton {
		return edButton{label: label, tip: tip, x: x + 14, y: y + 40 + row, w: w - 28, h: 30, active: get, do: set}
	}
	return []edButton{
		check("God mode: Barry cannot die", "Test without losing: look at the level instead of surviving it", 0, func() bool { return e.testGod }, func() { e.testGod = !e.testGod }),
		check("Start where the map view is", "Begin at the middle of what you see, not at the player spawn", 36, func() bool { return e.testHere }, func() { e.testHere = !e.testHere }),
		{icon: "minus", tip: "Start at an earlier wave", x: x + 188, y: y + 120, w: 34, h: 30, do: func() { e.testWave = max(1, e.testWave-1) }},
		{icon: "plus", tip: "Start at a later wave", x: x + w - 48, y: y + 120, w: 34, h: 30, do: func() { e.testWave = min(max(1, len(e.level.Waves)), e.testWave+1) }},
		{label: "Test now", icon: "play", tip: "Play the level with these options", x: x + 14, y: y + 208, w: 140, h: 38, accent: true, do: func() { e.testOpen = false; a.editorTest() }},
		{label: "Close", tip: "Close this panel", x: x + w - 114, y: y + 208, w: 100, h: 38, do: func() { e.testOpen = false }},
	}
}

func (a *app) updateTestOptions(px, py float64) {
	e := a.editor
	if e.testWave < 1 {
		e.testWave = 1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		e.testOpen = false
		return
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	for _, button := range a.testOptionsButtons() {
		if button.contains(px, py) {
			button.do()
			return
		}
	}
	x, y, w, h := a.testOptionsRect()
	if px < x || px >= x+w || py < y || py >= y+h {
		e.testOpen = false
	}
}

func (a *app) drawTestOptions(screen *ebiten.Image, px, py float64) {
	e := a.editor
	x, y, w, h := a.testOptionsRect()
	a.vRound(screen, x-1, y-1, w+2, h+2, 9, color.RGBA{255, 255, 255, 30})
	a.vRound(screen, x, y, w, h, 8, color.RGBA{24, 29, 41, 252})
	a.vText(screen, "Test options", x+14, y+10, 16, editorText, true)
	for _, button := range a.testOptionsButtons() {
		a.drawEditorButton(screen, button, px, py)
	}
	a.vText(screen, "Start at wave", x+14, y+126, 15, editorText, false)
	label := fmt.Sprint(max(1, e.testWave))
	a.vText(screen, label, x+222+(w-48-188-34-a.vTextWidth(label, 17, true))/2, y+126, 17, editorText, true)
	for index, line := range a.vWrap("Press T for a normal test; Shift+T starts at the pointer.", w-28, 13) {
		a.vText(screen, line, x+14, y+166+float64(index)*16, 13, editorDim, false)
	}
}

// ---- wave numbers and chart ----------------------------------------------------------------------------------------

// spawnTimes are the moments (ms into the wave) each zombie of a spawner appears, as the game spaces them.
func spawnTimes(wave formats.Wave, spawner formats.Spawner) []float64 {
	if spawner.Count <= 0 || len(spawner.Types) == 0 {
		return nil
	}
	interval := waveSpawnerInterval(wave.RunTime, spawner.DelayTime, spawner.Count)
	times := make([]float64, spawner.Count)
	for k := range times {
		times[k] = spawner.DelayTime + float64(k)*interval
	}
	return times
}

func isPickupName(name string) bool { return strings.HasPrefix(name, "p_") }

// waveStats are the numbers a wave adds up to.
type waveStats struct {
	zombies int
	health  float64
	peak    float64 // the most zombies spawned within one second
	last    float64 // ms of the last spawn
}

func waveNumbers(wave formats.Wave) waveStats {
	var stats waveStats
	var times []float64
	for _, spawner := range wave.Spawners {
		if len(spawner.Types) == 0 || isPickupName(spawner.Types[0].Name) || spawner.Types[0].Name == "train" {
			continue
		}
		weight, health := 0.0, 0.0
		for _, kind := range spawner.Types {
			weight += kind.Chance
			health += kind.Chance * kind.Strength
		}
		if weight > 0 {
			stats.health += float64(spawner.Count) * health / weight
		}
		stats.zombies += spawner.Count
		times = append(times, spawnTimes(wave, spawner)...)
	}
	for _, t := range times {
		stats.last = math.Max(stats.last, t)
		inWindow := 0.0
		for _, other := range times {
			if other >= t && other < t+1000 {
				inWindow++
			}
		}
		stats.peak = math.Max(stats.peak, inWindow)
	}
	return stats
}

// drawWaveChart draws when a wave's zombies appear: bars of how many come each moment, coloured by spawn zone, with the
// wave's minimum length marked.
func (a *app) drawWaveChart(screen *ebiten.Image, wave formats.Wave, x, y, w, h float64) {
	a.vRound(screen, x, y, w, h, 6, color.RGBA{18, 22, 31, 255})
	stats := waveNumbers(wave)
	span := math.Max(wave.EndWaveTime, stats.last+500)
	span = math.Max(span, 1000)
	const bins = 60
	counts := make([]map[int]int, bins)
	peak := 0
	for _, spawner := range wave.Spawners {
		if len(spawner.Types) == 0 || isPickupName(spawner.Types[0].Name) {
			continue
		}
		for _, t := range spawnTimes(wave, spawner) {
			bin := min(bins-1, int(t/span*bins))
			if counts[bin] == nil {
				counts[bin] = map[int]int{}
			}
			counts[bin][spawner.Index]++
		}
	}
	for _, m := range counts {
		total := 0
		for _, n := range m {
			total += n
		}
		peak = max(peak, total)
	}
	plotX, plotY, plotW, plotH := x+8, y+8, w-16, h-30
	barW := plotW / bins
	for bin, m := range counts {
		bottom := plotY + plotH
		for zone := 1; zone <= 13; zone++ {
			n := m[zone]
			if n == 0 {
				continue
			}
			barH := float64(n) / float64(max(1, peak)) * plotH
			bottom -= barH
			swatch := collisionSwatch(uint32(zone + 2))
			a.vRect(screen, plotX+float64(bin)*barW, bottom, math.Max(1, barW-1), barH, swatch)
		}
	}
	if wave.EndWaveTime > 0 {
		mx := plotX + wave.EndWaveTime/span*plotW
		a.vRect(screen, mx, plotY, 2, plotH, color.RGBA{255, 255, 255, 140})
		labelX := mx + 4
		if labelX+a.vTextWidth("minimum length", 11, false) > x+w-4 {
			labelX = mx - 4 - a.vTextWidth("minimum length", 11, false)
		}
		a.vText(screen, "minimum length", labelX, plotY, 11, editorDim, false)
	}
	a.vText(screen, "0 s", plotX, y+h-18, 12, editorDim, false)
	end := fmt.Sprintf("%.0f s", span/1000)
	a.vText(screen, end, plotX+plotW-a.vTextWidth(end, 12, false), y+h-18, 12, editorDim, false)
	a.vText(screen, "When the zombies appear", plotX+plotW/2-80, y+h-18, 12, editorDim, false)
}
