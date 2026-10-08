package game

import "testing"

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
	if line.text != "WAVE 1" {
		t.Fatalf("first line %q", line.text)
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
	if line.text != "WAVE 2" && line.text != "FINAL WAVE" {
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
