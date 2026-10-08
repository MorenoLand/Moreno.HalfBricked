package game

import (
	"fmt"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// The wave banner is a port addition: when a wave ends, a "WAVE n COMPLETE"
// line pops in, then the next "WAVE n+1" slams in with how many zombies it
// brings. (The original only changes the small "wave n" text in the corner.)
const (
	bannerCompleteTime = 1.5 // seconds the "complete" line is up
	bannerNextTime     = 2.0 // seconds the next-wave line is up
)

type waveBanner struct {
	active   bool
	age      float64
	from, to int  // wave numbers (1-based); from 0 for the very first wave
	incoming int  // zombies in the next wave
	last     bool // the next wave is the level's final one
	finished bool // no next wave: the level's waves are all done
}

func (b *waveBanner) advance(dt float64) {
	if !b.active {
		return
	}
	b.age += dt
	if b.age >= b.total() {
		b.active = false
	}
}

func (b *waveBanner) total() float64 {
	t := 0.0
	if b.from > 0 {
		t += bannerCompleteTime
	}
	if !b.finished {
		t += bannerNextTime
	}
	return t
}

// waveZombieCount is how many zombies a wave spawns (pickup spawners excluded).
func (p *playState) waveZombieCount(index int) int {
	if p.world == nil || index < 0 || index >= len(p.world.Level.Waves) {
		return 0
	}
	total := 0
	for _, spawner := range p.world.Level.Waves[index].Spawners {
		if spawner.Index < 1 || spawner.Index > 13 || spawner.Count <= 0 || len(spawner.Types) == 0 {
			continue
		}
		if strings.HasPrefix(strings.ToLower(spawner.Types[0].Name), "p_") {
			continue
		}
		total += spawner.Count
	}
	return total
}

// noteWave starts a banner whenever the running wave changes.
func (p *playState) noteWave() {
	if p.world == nil {
		return
	}
	waves := len(p.world.Level.Waves)
	if !p.wavesStarted {
		p.wavesStarted, p.bannerSeen = true, p.waveIndex
		p.startBanner(0, p.waveIndex+1)
		return
	}
	if p.waveIndex == p.bannerSeen {
		return
	}
	from := p.bannerSeen + 1
	p.bannerSeen = p.waveIndex
	if p.waveIndex >= waves {
		p.banner = waveBanner{active: true, from: from, finished: true}
		return
	}
	p.startBanner(from, p.waveIndex+1)
}

func (p *playState) startBanner(from, to int) {
	p.banner = waveBanner{active: true, from: from, to: to, incoming: p.waveZombieCount(to - 1)}
	p.banner.last = p.world != nil && to == len(p.world.Level.Waves) && to > 1
}

// bannerLines is what to draw right now: text, size, colour, alpha and the
// scale pop (>1 just after a line appears).
type bannerLine struct {
	text, sub string
	size, pop float64
	r, g, b   float32
	alpha, y  float64
}

func (b *waveBanner) line() (bannerLine, bool) {
	if !b.active {
		return bannerLine{}, false
	}
	age := b.age
	if b.from > 0 {
		if age < bannerCompleteTime {
			return bannerLine{text: fmt.Sprintf("WAVE %d COMPLETE", b.from), size: 22, r: 1, g: .84, b: .35, alpha: bannerFade(age, bannerCompleteTime, .25, .35), pop: bannerPop(age, .2), y: 96}, true
		}
		age -= bannerCompleteTime
	}
	if b.finished {
		return bannerLine{}, false
	}
	l := bannerLine{text: fmt.Sprintf("WAVE %d", b.to), size: 40, r: 1, g: 1, b: 1, alpha: bannerFade(age, bannerNextTime, .15, .4), pop: bannerPop(age, .3), y: 104}
	switch {
	case b.last:
		l.text, l.r, l.g, l.b = "FINAL WAVE", 1, .35, .3
	}
	if b.incoming > 0 {
		l.sub = fmt.Sprintf("%d ZOMBIES INCOMING", b.incoming)
	}
	return l, true
}

// bannerFade ramps alpha in over fadeIn seconds and out over the last fadeOut.
func bannerFade(age, total, fadeIn, fadeOut float64) float64 {
	a := 1.0
	if age < fadeIn {
		a = age / fadeIn
	}
	if rest := total - age; rest < fadeOut {
		a = math.Min(a, rest/fadeOut)
	}
	return math.Max(0, math.Min(1, a))
}

// bannerPop is the overshoot scale: starts big and settles to 1.
func bannerPop(age, duration float64) float64 {
	if age >= duration {
		return 1
	}
	t := 1 - age/duration
	return 1 + .7*t*t
}

// drawWaveBanner draws the current line centred on the screen, outlined.
func (a *app) drawWaveBanner(screen *ebiten.Image) {
	if a.play == nil || a.font == nil || a.font.LineHeight <= 0 {
		return
	}
	line, ok := a.play.banner.line()
	if !ok || line.alpha <= 0 {
		return
	}
	draw := func(text string, size, y float64, r, g, b float32, alpha float64) {
		scale := size / float64(a.font.LineHeight)
		x := (logicalWidth - a.fontTextWidth(text, scale)) / 2
		top := y - size/2
		for _, off := range [][2]float64{{-1.5, 0}, {1.5, 0}, {0, -1.5}, {0, 1.5}} {
			a.drawComboGlyphs(screen, text, x+off[0], top+off[1], scale, 0, 0, 0, float32(alpha*.8))
		}
		a.drawComboGlyphs(screen, text, x, top, scale, r, g, b, float32(alpha))
	}
	size := line.size * line.pop
	draw(line.text, size, line.y, line.r, line.g, line.b, line.alpha)
	if line.sub != "" {
		draw(line.sub, 15, line.y+line.size*.7+6, 1, .55, .5, line.alpha)
	}
}
