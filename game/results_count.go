package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
)

// PORT ADDITION (not in the original binary): results number animation.
//
// The 1.2.5 end screen has no count-up: the enter routine 0x000cf030 sprintf's the
// final totals once and the per-frame update 0x000ce13c only runs the slide state
// machine, so the original shows final values from the first frame. At the user's
// request this port counts Score, Zombie Kills and Highscore up from 0 once the
// screen has slid in (ease-out, kills slightly after score), ticks softly, pops
// each number gold when it lands, and lets the first click / Enter / Escape finish
// the count before any button can be pressed. Set resultsCountUp to false for the
// purist original (final values at once, no tick, no pop, buttons immediately).
// (A var rather than a const only so tests can exercise both modes.)
var resultsCountUp = true

const (
	resultsCountRows  = 3 // score, kills, highscore
	resultsPopSeconds = .45
	resultsPopScale   = .32
	resultsTickVolume = .22
	resultsTickSteps  = 18  // ticks per number over the whole count
	resultsTickGap    = .06 // seconds between two audible ticks
	resultsTickSound  = "audio/sound/sfx/menu_move.ogg"
)

// Per row: start delay and duration in seconds after the slide-in finished.
var resultsCountDelay = [resultsCountRows]float32{0, .2, .4}
var resultsCountLength = [resultsCountRows]float32{1.4, 1.3, 1.2}

type resultsCount struct {
	t        float32 // seconds since the screen finished sliding in
	skipped  bool
	landed   [resultsCountRows]bool
	landedAt [resultsCountRows]float32
	tickDone [resultsCountRows]int32
	ticks    int
	lastTick float32
}

// countFinals returns the final value of each row and whether the row exists.
func (menu *resultsMenu) countFinals() (values [resultsCountRows]int32, present [resultsCountRows]bool) {
	values[0], present[0] = menu.score(), true
	if menu.Data.Kills != nil {
		values[1], present[1] = *menu.Data.Kills, true
	}
	if menu.Data.Highscore != nil {
		values[2], present[2] = *menu.Data.Highscore, true
	}
	return
}

func (menu *resultsMenu) countValue(row int) int32 {
	finals, _ := menu.countFinals()
	final := finals[row]
	if !resultsCountUp || menu.count.skipped {
		return final
	}
	if !menu.ready {
		return 0
	}
	u := math.Max(0, math.Min(1, float64((menu.count.t-resultsCountDelay[row])/resultsCountLength[row])))
	if u >= 1 {
		return final
	}
	eased := 1 - math.Pow(1-u, 3)
	return int32(math.Round(float64(final) * eased))
}

func (menu *resultsMenu) shownScore() int32 { return menu.countValue(0) }

func (menu *resultsMenu) shownKills() int32 {
	if menu.Data.Kills == nil {
		return 0
	}
	return menu.countValue(1)
}

func (menu *resultsMenu) shownHighscore() int32 {
	if menu.Data.Highscore == nil {
		return 0
	}
	return menu.countValue(2)
}

// counting reports whether the numbers are still on their way (including the
// slide-in), i.e. a first press must only finish the count.
func (menu *resultsMenu) counting() bool {
	if !resultsCountUp || !menu.Visible || menu.closing || menu.delivered || menu.count.skipped {
		return false
	}
	_, present := menu.countFinals()
	for row := 0; row < resultsCountRows; row++ {
		if present[row] && !menu.count.landed[row] {
			return true
		}
	}
	return false
}

// finishCount ends the count at once; it reports whether it consumed the press.
func (menu *resultsMenu) finishCount() bool {
	if !menu.counting() {
		return false
	}
	c := &menu.count
	c.skipped, c.ticks = true, 0
	for row := range c.landed {
		if !c.landed[row] {
			c.landed[row], c.landedAt[row] = true, c.t
		}
	}
	return true
}

// advanceCount runs from resultsMenu.update with the frame time.
func (menu *resultsMenu) advanceCount(dt float32) {
	c := &menu.count
	if !resultsCountUp || !menu.ready || dt <= 0 || c.t > 60 {
		return
	}
	c.t += dt
	finals, present := menu.countFinals()
	for row := 0; row < resultsCountRows; row++ {
		if !present[row] {
			continue
		}
		if !c.skipped {
			step := finals[row] / resultsTickSteps
			if step < 1 {
				step = 1
			}
			if done := menu.countValue(row) / step; done > c.tickDone[row] {
				c.tickDone[row] = done
				c.ticks++
			}
		}
		if !c.landed[row] && c.t >= resultsCountDelay[row]+resultsCountLength[row] {
			c.landed[row], c.landedAt[row] = true, c.t
		}
	}
}

// takeTick returns whether a tick sound is due (at most one per resultsTickGap).
func (menu *resultsMenu) takeTick() bool {
	c := &menu.count
	due := c.ticks > 0 && c.t-c.lastTick >= resultsTickGap
	c.ticks = 0
	if due {
		c.lastTick = c.t
	}
	return due
}

// popEffect is the size multiplier and gold amount of a number that just landed.
func (menu *resultsMenu) popEffect(row int) (scale, gold float64) {
	c := &menu.count
	finals, present := menu.countFinals()
	if !resultsCountUp || !present[row] || !c.landed[row] || finals[row] == 0 {
		return 1, 0
	}
	u := float64(c.t-c.landedAt[row]) / resultsPopSeconds
	if u < 0 || u >= 1 {
		return 1, 0
	}
	bump := (1 - u) / .8
	if u < .2 {
		bump = u / .2
	}
	bump *= bump
	return 1 + resultsPopScale*bump, bump
}

// playResultsTick plays the soft count tick for the frame, if one is due.
func (a *app) playResultsTick(menu *resultsMenu) {
	if menu != nil && menu.takeTick() {
		a.playSound(resultsTickSound, resultsTickVolume)
	}
}

func (a *app) drawFontTinted(screen *ebiten.Image, value string, x, y, scale, gold float64) {
	if a.font == nil {
		return
	}
	// Gold (255, 209, 51) blended in by the pop amount.
	r, g, b := float32(1), float32(1-.18*gold), float32(1-.8*gold)
	a.font.DrawScaledTinted(screen, value, x*a.frontendScaleX, y*a.frontendScaleY, scale*a.frontendScaleX, scale*a.frontendScaleY, r, g, b)
}

// drawUITextFX is drawUIText with the landing pop (size multiplier and gold).
func (a *app) drawUITextFX(screen *ebiten.Image, text string, centerX, centerY, width float64, align string, size float64, offset formats.Vec2, mult, gold float64) {
	if a.font == nil || a.font.LineHeight == 0 || text == "" {
		return
	}
	size *= mult
	scale := size / float64(a.font.LineHeight)
	textWidth := a.fontTextWidth(text, scale)
	x := centerX + offset.X
	switch align {
	case "CenterLeft":
		x -= width / 2
	case "CenterRight":
		x += width/2 - textWidth
	default:
		x -= textWidth / 2
	}
	a.drawFontTinted(screen, text, x, centerY+offset.Y-size/2, scale, gold)
}
