package game

import (
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
)

func TestWaveBannerCompletesThenAnnouncesTheNextWave(t *testing.T) {
	p := script125CachedHost(t, "world0_level1").play
	p.closeScript()
	if len(p.world.Level.Waves) < 2 {
		t.Skip("level has a single wave")
	}
	p.noteWave()
	if !p.banner.active || p.banner.from != 0 || p.banner.to != 1 {
		t.Fatalf("the first wave should announce itself: %+v", p.banner)
	}
	line, _ := p.banner.line()
	if line.text != "wave 1" {
		t.Fatalf("first line %q (the corner shows wave 1 at the level start)", line.text)
	}
	p.waveIndex = 1
	p.noteWave()
	if p.banner.from != 1 || p.banner.to != 2 {
		t.Fatalf("a wave change should banner 1 -> 2: %+v", p.banner)
	}
	line, _ = p.banner.line()
	if line.text != "WAVE 1 COMPLETE" {
		t.Fatalf("the finished wave is announced first, got %q", line.text)
	}
	for frame := 0; frame < int((bannerCompleteTime+.1)*60); frame++ {
		p.banner.advance(1.0 / 60.0)
	}
	line, _ = p.banner.line()
	if line.text != "wave 2" {
		t.Fatalf("then the next wave, got %q", line.text)
	}
	for frame := 0; frame < 60*10; frame++ {
		p.banner.advance(1.0 / 60.0)
	}
	if p.banner.active {
		t.Fatal("the banner should go away by itself")
	}
}

func TestWaveBannerAfterTheLastWave(t *testing.T) {
	p := script125CachedHost(t, "world0_level1").play
	p.closeScript()
	p.noteWave()
	p.waveIndex = len(p.world.Level.Waves)
	p.noteWave()
	line, ok := p.banner.line()
	if !ok || line.text == "" || !p.banner.finished {
		t.Fatalf("clearing every wave should say so: %+v", p.banner)
	}
}

// The native timer runs from 2.0 down: the wave text is blue for the first second and whitens over the next.
func TestNativeWaveTextTintsFromBlueToWhite(t *testing.T) {
	b := waveBanner{active: true, from: 0, to: 2, count: 1, timer: nativeBannerSeconds, native: true}
	for _, c := range []struct{ age, want float64 }{{0, 1}, {1, 1}, {1.5, .5}, {2, 0}, {3, 0}} {
		b.age = c.age
		if got := b.nativeT(); math.Abs(got-c.want) > 1e-9 {
			t.Fatalf("age %.1f: t = %.2f, want %.2f", c.age, got, c.want)
		}
	}
	b.age = 1.5
	line, ok := b.line()
	if !ok || !line.native || line.text != "wave 1" || math.Abs(float64(line.r)-.5) > 1e-6 || line.g != line.r || line.b != 1 {
		t.Fatalf("main line at 1.5 s: %+v", line)
	}
}

// The v7 timer changes nothing in the text, so the SD cache draws no main wave line; the sub line stays.
func TestV7BannerDrawsNoMainWaveLine(t *testing.T) {
	b := waveBanner{active: true, from: 0, to: 2, count: 1, timer: nativeBannerSeconds, native: false, age: 1.5, incoming: 4}
	line, ok := b.line()
	if !ok || line.text != "" || line.native || line.sub != "4 ZOMBIES INCOMING" || line.subY != bannerSubY {
		t.Fatalf("v7 banner: %+v", line)
	}
}

func TestWaveBannerNumbersFollowTheNativeCounter(t *testing.T) {
	p := script125CachedHost(t, "world0_level1").play
	p.closeScript()
	if len(p.world.Level.Waves) < 3 {
		t.Skip("level has fewer than three waves")
	}
	p.noteWave()
	if p.banner.count != 1 || p.banner.timer != 0 || !p.banner.native {
		t.Fatalf("level start: %+v", p.banner)
	}
	p.waveIndex = 1
	p.noteWave()
	if p.banner.count != 2 || p.banner.timer != nativeBannerSeconds {
		t.Fatalf("first advance: %+v", p.banner)
	}
	p.waveIndex = 2
	p.noteWave()
	if p.banner.count != 3 {
		t.Fatalf("second advance: %+v", p.banner)
	}
}

func TestV7WaveBannerDrawsNoMainLineOnItsCache(t *testing.T) {
	p := script125CachedHost(t, "world0_level1").play
	p.closeScript()
	if len(p.world.Level.Waves) < 2 {
		t.Skip("level has a single wave")
	}
	p.world.Level.WaveBuild = content.WaveBuildV7
	p.noteWave()
	p.waveIndex = 1
	p.noteWave()
	p.banner.age = bannerCompleteTime + .1
	line, ok := p.banner.line()
	if !ok || p.banner.native || line.text != "" {
		t.Fatalf("v7 banner: %+v native=%v", line, p.banner.native)
	}
}
