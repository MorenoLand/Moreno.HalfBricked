package game

import (
	"fmt"
	"math"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/hajimehoshi/ebiten/v2"
)

// The wave banner. The native evidence is in Research/native/wave-banner-2026-10-09.md.
//   - The native wave text is "wave %d", with the wave counter (1.2.5 +0x450f0, v7 +0x5147c). It is drawn in
//     three passes: black at (-1,-1) and (+1,+1), then the text (1.2.5 FUN_00133f54, format at 0x0085c8a5).
//   - Its colour is 255*(1-t) on R and G and 255 on B, with t = clamp(banner timer, 0, 1). The timer (1.2.5
//     +0x450f4) is 2.0 s after each advance.
//   - In v7 the same timer is read only through its sign, and its decrement clamps it at 0.0, so it never changes
//     the text. The SD cache therefore draws no main banner line of its own.
//
// The "COMPLETE" and "ZOMBIES INCOMING" lines are not in the binary, so they keep their earlier drawing. The main
// line keeps the port's position and size: the native ones depend on a HUD branch (1.2.5 FUN_00134190 draws at
// (240,40) with scale 0.7 or at (400,15) with scale 1.0) and on a runtime font size.
const (
	bannerCompleteTime = 1.5 // seconds the "complete" line is up
	bannerNextTime     = 2.0 // seconds the next-wave line is up
	// nativeBannerSeconds is the native banner timer after an advance (float 0x40000000 = 2.0).
	nativeBannerSeconds = 2.0
	// bannerSubY is where the "ZOMBIES INCOMING" line sits (the main line's old y plus its old offset).
	bannerSubY = 104 + 40*.7 + 6
)

type waveBanner struct {
	active   bool
	age      float64
	from, to int     // wave numbers (1-based); from 0 for the very first wave
	incoming int     // zombies in the next wave
	finished bool    // no next wave: the level's waves are all done
	native   bool    // 1.2.5 (HD): the main line is the native wave text; v7 (SD) draws no main line
	count    int     // the native wave counter: advances so far (0 at the level start)
	timer    float64 // the native banner timer at the start (2.0 after an advance, 0 at the level start)
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
		p.banner = waveBanner{active: true, from: from, finished: true, count: p.banner.count, native: p.banner.native}
		return
	}
	p.startBanner(from, p.waveIndex+1)
}

// startBanner starts the banner for a change of wave. From 0 is the level's first wave, when nothing has
// advanced: the native counter is 0 and no timer store was found for the 1.2.5 level start. Any other banner
// follows an advance, so the native counter goes up by one and the native timer is 2.0 s.
func (p *playState) startBanner(from, to int) {
	// The number is the wave the corner shows (waveIndex+1), so the banner and the corner always agree.
	b := waveBanner{active: true, from: from, to: to, count: to, incoming: p.waveZombieCount(to - 1)}
	if from > 0 {
		b.timer = nativeBannerSeconds
	}
	b.native = p.waveBuild() != content.WaveBuildV7
	p.banner = b
}

// bannerLine is what to draw right now: text, size, colour, alpha and the scale pop (>1 just after a line
// appears). The sub line is drawn at subY. A native main line uses the native wave text passes.
type bannerLine struct {
	text, sub string
	size, pop float64
	r, g, b   float32
	alpha, y  float64
	subY      float64
	native    bool
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
	l := bannerLine{size: 40, r: 1, g: 1, b: 1, alpha: bannerFade(age, bannerNextTime, .15, .4), pop: bannerPop(age, .3), y: 104, subY: bannerSubY}
	if b.native {
		t := b.nativeT()
		l.text, l.r, l.g, l.b, l.native = fmt.Sprintf("wave %d", b.count), float32(1-t), float32(1-t), 1, true
	}
	if b.incoming > 0 {
		l.sub = fmt.Sprintf("%d ZOMBIES INCOMING", b.incoming)
	}
	return l, true
}

// nativeT is the native colour input t: the banner timer counts down from its start and the wave text uses it
// clamped to [0, 1] (1.2.5 FUN_00133f54). The countdown runs with the banner's age. The 1.2.5 decrement itself
// is not located (v7 FUN_0006e8dc subtracts dt each frame).
func (b *waveBanner) nativeT() float64 {
	return math.Max(0, math.Min(1, b.timer-b.age))
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

// drawWaveBanner draws the current line centred on the screen. A native main line uses the native passes (black
// at (-1,-1) and (+1,+1) at full alpha, then the text); the other lines keep their earlier outline.
func (a *app) drawWaveBanner(screen *ebiten.Image) {
	if a.play == nil || a.font == nil || a.font.LineHeight <= 0 {
		return
	}
	line, ok := a.play.banner.line()
	if !ok || line.alpha <= 0 {
		return
	}
	draw := func(text string, size, y float64, r, g, b float32, alpha float64, native bool) {
		if text == "" {
			return
		}
		scale := size / float64(a.font.LineHeight)
		x := (logicalWidth - a.fontTextWidth(text, scale)) / 2
		top := y - size/2
		if native {
			for _, off := range [][2]float64{{-1, -1}, {1, 1}} {
				a.drawComboGlyphs(screen, text, x+off[0], top+off[1], scale, 0, 0, 0, float32(alpha))
			}
		} else {
			for _, off := range [][2]float64{{-1.5, 0}, {1.5, 0}, {0, -1.5}, {0, 1.5}} {
				a.drawComboGlyphs(screen, text, x+off[0], top+off[1], scale, 0, 0, 0, float32(alpha*.8))
			}
		}
		a.drawComboGlyphs(screen, text, x, top, scale, r, g, b, float32(alpha))
	}
	size := line.size * line.pop
	draw(line.text, size, line.y, line.r, line.g, line.b, line.alpha, line.native)
	if line.sub != "" {
		draw(line.sub, 15, line.subY, 1, .55, .5, line.alpha, false)
	}
}
