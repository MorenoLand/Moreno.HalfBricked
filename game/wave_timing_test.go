package game

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

// useNativeWaveEnd selects the recovered FUN_000bf120 end rule for one test (the default is the all-dead option).
func useNativeWaveEnd(t *testing.T) {
	t.Helper()
	old := waveEndRule
	waveEndRule = waveEndNative
	t.Cleanup(func() { waveEndRule = old })
}

func runWaveEnd(p *playState, wave formats.Wave, spawnersLeft func(frame int) bool, alive func(frame int) int, max int) int {
	for frame := 0; frame < max; frame++ {
		p.waveElapsed += 1000.0 / 60.0
		if p.waveEndStep(wave, spawnersLeft(frame), alive(frame)) {
			return frame + 1
		}
	}
	return -1
}

// A timed wave (end_wave_time 20001) ends after (20001 - 10) / 17 ms steps once the
// spawners are empty, however many zombies are still alive.
func TestWaveWithEndTimeIgnoresAliveZombies(t *testing.T) {
	useNativeWaveEnd(t)
	p := &playState{}
	wave := formats.Wave{RunTime: 20000, EndWaveTime: 20001, EndWaveZombies: 10}
	frames := runWaveEnd(p, wave, func(int) bool { return false }, func(int) int { return 500 }, 3000)
	want := (20001 - 10 + 16) / 17 // ceil(19991 / 17)
	if frames != want {
		t.Fatalf("wave ended after %d frames, want %d (about 19.6 s)", frames, want)
	}
}

// While spawners still have zombies to deliver the wave waits (re-checking every 100 ms).
func TestWaveWaitsForTheSpawnersAfterTheTimerExpires(t *testing.T) {
	useNativeWaveEnd(t)
	p := &playState{}
	wave := formats.Wave{RunTime: 5000, EndWaveTime: 3000}
	frames := runWaveEnd(p, wave, func(frame int) bool { return frame < 400 }, func(int) int { return 0 }, 3000)
	if frames < 400 || frames > 400+8 {
		t.Fatalf("wave ended after %d frames, want right after the last spawn at 400 (plus at most the 100 ms re-check)", frames)
	}
}

// end_wave_time 0 waves are gated by alive < end_wave_zombies (strict) once the spawners are empty.
func TestKillGatedWaveNeedsFewerZombiesThanTheLimit(t *testing.T) {
	useNativeWaveEnd(t)
	p := &playState{}
	wave := formats.Wave{RunTime: 20000, EndWaveTime: 0, EndWaveZombies: 10}
	if p.waveEndStep(wave, true, 0) {
		t.Fatal("must not end while spawners remain")
	}
	if p.waveEndStep(wave, false, 10) {
		t.Fatal("10 alive is not below 10")
	}
	if !p.waveEndStep(wave, false, 9) {
		t.Fatal("9 alive is below 10")
	}
	zero := formats.Wave{EndWaveTime: 3000}
	q := &playState{}
	q.waveEndInit, q.waveEndTimer = true, 0
	if q.waveEndStep(zero, false, 0) {
		t.Fatal("end_wave_zombies 0 needs alive < 0, never true natively once the timer is not positive")
	}
}

// FUN_000bec48: tile index rnd(n - 1) (the last one is unreachable), point (tile + 0.1 + rnd(.8)) * 32.
func TestNativeSpawnPointIsRandomInsideTheTile(t *testing.T) {
	rng := weapons.NewNativeRNG()
	p := &playState{rng: &rng, tileSize: 32}
	points := []formats.Vec2{{X: 48, Y: 48}, {X: 144, Y: 48}, {X: 240, Y: 80}}
	seen := map[int]bool{}
	for i := 0; i < 400; i++ {
		pt := p.nativeSpawnPoint(points)
		tx, ty := int(pt.X)/32, int(pt.Y)/32
		localX, localY := pt.X-float64(tx*32), pt.Y-float64(ty*32)
		if localX < 3.19 || localX > 28.81 || localY < 3.19 || localY > 28.81 {
			t.Fatalf("point %v is not inside the inset tile", pt)
		}
		for index, candidate := range points {
			if int(candidate.X)/32 == tx && int(candidate.Y)/32 == ty {
				seen[index] = true
			}
		}
	}
	if !seen[0] || !seen[1] || seen[2] {
		t.Fatalf("tiles used %v: the first two must appear and the last must never be picked", seen)
	}
}

// The two data caches and the build each one was imported from (the wave rule follows the build).
var waveRuleCaches = []struct{ root, build string }{
	{"bin/data-cache", content.WaveBuild125},
	{"bin/web/data/data-cache", content.WaveBuildV7},
}

// waveCachePlay opens a level of one data cache as the game does (TestMain runs the tests from the repository root)
// and returns its play state with the scripts done, god mode on and no cheats beyond it.
func waveCachePlay(t *testing.T, root, id string) *playState {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, "pack.json")); err != nil {
		t.Skipf("data cache %s is not present", root)
	}
	a := newHeadlessApp(t, root)
	if err := a.selectCaptureLevel(id); err != nil {
		t.Fatal(err)
	}
	if err := a.openPlay(); err != nil {
		t.Fatal(err)
	}
	p := a.play
	p.closeScript()
	p.dialogueIndex = len(p.dialogue)
	p.cheats.god = true
	return p
}

// waveFrameAdvance runs updateWaves frame by frame until the wave index changes. It reports the frame, the seconds
// that had passed and the living zombies on that frame.
func waveFrameAdvance(p *playState, maxFrames int) (seconds float64, alive int, ok bool) {
	start := p.waveIndex
	for frame := 1; frame <= maxFrames; frame++ {
		p.updateWaves()
		if p.waveIndex != start {
			return float64(frame) / 60, p.livingZombies(), true
		}
	}
	return 0, p.livingZombies(), false
}

// The cache's marker names the build it was imported from, and the wave rule follows that build.
func TestWaveBuildComesFromTheCacheMarker(t *testing.T) {
	for _, c := range waveRuleCaches {
		if _, err := os.Stat(filepath.Join(c.root, "pack.json")); err != nil {
			t.Skipf("data cache %s is not present", c.root)
		}
		pack, err := content.NewPack(content.NewSource(c.root))
		if err != nil {
			t.Fatal(err)
		}
		if got := pack.Manifest().WaveBuild; got != c.build {
			t.Fatalf("%s: wave build %q, want %q", c.root, got, c.build)
		}
		level, err := pack.Load("World0Level1")
		if err != nil {
			t.Fatal(err)
		}
		if level.WaveBuild != c.build {
			t.Fatalf("%s: level wave build %q, want %q", c.root, level.WaveBuild, c.build)
		}
	}
}

// Enemy limits of the wave gate: 65 in the 1.2.1 build (v7), 95 in 1.2.5 (story; survival is 95 unless its
// entity lists 10..15 are occupied, which is not ported).
func TestWaveZombieLimitPerBuild(t *testing.T) {
	cases := []struct {
		build    string
		survival bool
		want     int
	}{
		{content.WaveBuildV7, false, 65},
		{content.WaveBuildV7, true, 65},
		{content.WaveBuild125, false, 95},
		{content.WaveBuild125, true, 95},
		{"", false, 95},
	}
	for _, c := range cases {
		if got := waveZombieLimit(c.build, c.survival); got != c.want {
			t.Fatalf("waveZombieLimit(%q, survival=%v) = %d, want %d", c.build, c.survival, got, c.want)
		}
	}
}

// World5Level2 wave 1 (end_wave_time 2000, end_wave_zombies 0): its two zombies spawn and the timer ends the wave
// about 2 s in with zombies still alive. Neither build looks at the alive count on the timed path, so this is the
// native rule and the banner moves on that frame.
func TestWorld5Level2WaveOneEndsOnItsTimer(t *testing.T) {
	useNativeWaveEnd(t)
	for _, c := range waveRuleCaches {
		t.Run(c.build, func(t *testing.T) {
			p := waveCachePlay(t, c.root, "World5Level2")
			wave := p.world.Level.Waves[0]
			if wave.EndWaveTime != 2000 || wave.EndWaveZombies != 0 {
				t.Fatalf("level data changed: end_wave_time %v, end_wave_zombies %d", wave.EndWaveTime, wave.EndWaveZombies)
			}
			seconds, alive, ok := waveFrameAdvance(p, 60*10)
			if !ok {
				t.Fatal("wave 1 never ended")
			}
			if seconds < 1.9 || seconds > 2.3 {
				t.Fatalf("wave 1 ended after %.2f s, want the 2 s timer", seconds)
			}
			if alive < 1 {
				t.Fatalf("wave 1 ended with %d zombies alive; the timed rule does not wait for them", alive)
			}
			if !p.banner.active || p.banner.from != 1 {
				t.Fatalf("banner %+v does not announce WAVE 1 COMPLETE on the advance frame", p.banner)
			}
		})
	}
}

// World0Level1 wave 1 (end_wave_time 20001, end_wave_zombies 10). Its zombies keep coming while the living count is
// below the build's limit. 1.2.5 (limit 95) lets the 20 s timer end it with the zombies alive. 1.2.1 (limit 65)
// holds the wave once 65 are alive: nothing spawns and the timer does not run until kills bring the count back.
func TestWorld0Level1WaveOneHoldsAtTheEnemyLimit(t *testing.T) {
	useNativeWaveEnd(t)
	for _, c := range waveRuleCaches {
		t.Run(c.build, func(t *testing.T) {
			p := waveCachePlay(t, c.root, "World0Level1")
			wave := p.world.Level.Waves[0]
			if wave.EndWaveTime != 20001 || wave.EndWaveZombies != 10 {
				t.Fatalf("level data changed: end_wave_time %v, end_wave_zombies %d", wave.EndWaveTime, wave.EndWaveZombies)
			}
			limit := waveZombieLimit(p.waveBuild(), p.isSurvival())
			if c.build == content.WaveBuild125 {
				seconds, alive, ok := waveFrameAdvance(p, 60*40)
				if !ok {
					t.Fatal("1.2.5: wave 1 did not end within 40 s")
				}
				if seconds < 19.5 || seconds > 21 {
					t.Fatalf("1.2.5: wave 1 ended after %.2f s, want the 20 s timer", seconds)
				}
				if alive >= limit {
					t.Fatalf("1.2.5: wave 1 ended with %d alive, at or above the limit %d", alive, limit)
				}
				return
			}
			var aliveAt30 int
			for frame := 1; frame <= 60*40; frame++ {
				p.updateWaves()
				if p.waveIndex != 0 {
					t.Fatalf("1.2.1: wave 1 ended at %.1f s with %d alive, above the limit %d", float64(frame)/60, p.livingZombies(), limit)
				}
				if frame == 60*30 {
					aliveAt30 = p.livingZombies()
				}
			}
			alive := p.livingZombies()
			if alive < limit {
				t.Fatalf("1.2.1: %d alive after 40 s, the wave should hold at the limit %d", alive, limit)
			}
			if alive != aliveAt30 {
				t.Fatalf("1.2.1: spawning did not stop at the limit: %d alive at 30 s, %d at 40 s", aliveAt30, alive)
			}
			// Kills below the limit reopen the gate: the zombies that are still due spawn again.
			killed := 0
			for i := range p.zombies {
				if alive-killed > limit-6 && !p.zombies[i].dying {
					p.zombies[i].dying = true
					killed++
				}
			}
			for frame := 0; frame < 60*5; frame++ {
				p.updateWaves()
			}
			if p.livingZombies() <= limit-6 {
				t.Fatalf("1.2.1: no zombie spawned again after kills dropped the count below the limit (%d alive)", p.livingZombies())
			}
		})
	}
}
