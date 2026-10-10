package game

import (
	"image"
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"github.com/hajimehoshi/ebiten/v2"
)

// gibBuilds pairs each data cache with the build it follows. The HD cache is 1.2.5 and
// the SD cache is v7 (1.2.1); the two are never mixed.
var gibBuilds = []struct {
	name   string
	root   string
	build  string
	consts zombieGibConstants
}{
	{name: "hd-1.2.5", root: "bin/data-cache", build: content.WaveBuild125, consts: zombieGib125},
	{name: "sd-1.2.1", root: "bin/web/data/data-cache", build: content.WaveBuildV7, consts: zombieGibV7},
}

func gibCatalog(t *testing.T, root string) formats.SpriteCatalog {
	t.Helper()
	pack, err := content.NewPack(content.NewSource(root))
	if err != nil {
		t.Fatalf("pack %s: %v", root, err)
	}
	catalog, err := pack.Sprites()
	if err != nil {
		t.Fatalf("sprites %s: %v", root, err)
	}
	return catalog
}

func TestGibDeathStateChoice(t *testing.T) {
	for _, build := range gibBuilds {
		c := zombieGibConstantsFor(build.build)
		cases := []struct {
			name     string
			kind     uint8
			distance float64
			want     int
		}{
			{"flame on the victim", 0x16, 0, zombieDeathGib},
			{"flame far away", 0x16, 300, zombieDeathGib},
			{"bazooka beyond 48 px", 0x13, 49, zombieDeathGib},
			{"grenade far", 0x14, 160, zombieDeathGib},
			{"mine beyond 48 px", 0x15, 48.5, zombieDeathGib},
			{"blast exactly 48 px", 0x15, 48, zombieDeathNormal},
			{"grenade near", 0x14, 12, zombieDeathNormal},
			{"bazooka near", 0x13, 0, zombieDeathNormal},
			{"bullet far away", 0x10, 300, zombieDeathNormal},
			{"second bullet kind far", 0x11, 300, zombieDeathNormal},
			{"dynamite (0x1e) far", 0x1e, 300, zombieDeathNormal},
		}
		for _, tc := range cases {
			if got := zombieDeathStateFor(tc.kind, tc.distance, c); got != tc.want {
				t.Fatalf("%s: %s = death state %d, want %d", build.name, tc.name, got, tc.want)
			}
		}
	}
}

func TestGibHitTimeWindowAndDraw(t *testing.T) {
	for _, build := range gibBuilds {
		c := build.consts
		low, high := c.hitBase*(1-c.hitSpread), c.hitBase
		rng := weapons.NewNativeRNG()
		lo, hi := math.Inf(1), math.Inf(-1)
		for i := 0; i < 2000; i++ {
			snapshot := rng
			got := zombieGibHitTime(&rng, c)
			lo, hi = math.Min(lo, got), math.Max(hi, got)
			if got <= low || got > high {
				t.Fatalf("%s: draw %d = %v outside (%v, %v]", build.name, i, got, low, high)
			}
			bounded := snapshot.Bounded(0x7ffff)
			want := (1 - float64(float32(bounded)/float32(524287.5)*float32(c.hitSpread))) * c.hitBase
			if math.Abs(got-want) > 1e-5 {
				t.Fatalf("%s: draw %d = %v, want %v from Bounded(0x7ffff)=%d", build.name, i, got, want, bounded)
			}
		}
		if hi-lo < 0.1 {
			t.Fatalf("%s: hit timers do not spread: %v..%v", build.name, lo, hi)
		}
		var zero weapons.NativeRNG
		if got := zombieGibHitTime(&zero, c); math.Abs(got-c.hitBase) > 1e-6 {
			t.Fatalf("%s: zero draw = %v, want the base %v", build.name, got, c.hitBase)
		}
	}
}

func TestGibClassifyFollowsTheKillSource(t *testing.T) {
	_, p := explodingRig()
	rng := weapons.NewNativeRNG()
	p.rng = &rng
	flame := addZombie(p, "zombie", 1000, 1000)
	farBlast := addZombie(p, "zombie", 1100, 1000)
	nearBlast := addZombie(p, "zombie", 1200, 1000)
	bullet := addZombie(p, "zombie", 1300, 1000)
	boss := addZombie(p, "boss_rex", 1400, 1000)
	alive := addZombie(p, "zombie", 1500, 1000)
	for _, index := range []int{flame, farBlast, nearBlast, bullet, boss} {
		killNow(p, index)
	}
	p.classifyZombieDeath(&p.zombies[flame], 0x16, 1000, 1000)
	p.classifyZombieDeath(&p.zombies[farBlast], 0x15, 1100+80, 1000)
	p.classifyZombieDeath(&p.zombies[nearBlast], 0x15, 1200+10, 1000)
	p.classifyZombieDeath(&p.zombies[bullet], 0x10, 1300+300, 1000)
	p.classifyZombieDeath(&p.zombies[boss], 0x15, 1400+200, 1000)
	p.classifyZombieDeath(&p.zombies[alive], 0x16, 1500, 1000)

	if z := p.zombies[flame]; z.deathState != zombieDeathGib || z.deathDelay < 1.28 || z.deathDelay > 1.6 {
		t.Fatalf("flame kill: state %d delay %v, want a gib with a 1.28..1.6 s hit timer", z.deathState, z.deathDelay)
	}
	if z := p.zombies[farBlast]; z.deathState != zombieDeathGib || z.deathDelay < 1.28 || z.deathDelay > 1.6 {
		t.Fatalf("far blast kill: state %d delay %v, want a gib", z.deathState, z.deathDelay)
	}
	if z := p.zombies[nearBlast]; z.deathState != zombieDeathNormal || z.deathDelay != zombieDeathDelay {
		t.Fatalf("near blast kill: state %d delay %v, want a normal death with %v", z.deathState, z.deathDelay, zombieDeathDelay)
	}
	if z := p.zombies[bullet]; z.deathState != zombieDeathNormal || z.deathDelay != zombieDeathDelay {
		t.Fatalf("bullet kill: state %d delay %v, want a normal death", z.deathState, z.deathDelay)
	}
	if z := p.zombies[boss]; z.deathState != zombieDeathNormal {
		t.Fatalf("boss blast kill: state %d, want normal (bosses do not run takeDamage)", z.deathState)
	}
	if z := p.zombies[alive]; z.deathState != 0 || z.deathDelay != 0 {
		t.Fatalf("living zombie classified: state %d delay %v", z.deathState, z.deathDelay)
	}
	if got := p.zombieDeathDelayFor(p.zombies[nearBlast]); got != zombieDeathDelay {
		t.Fatalf("normal death delay = %v, want %v", got, zombieDeathDelay)
	}
}

func TestGibPresentationTimingOnBothCaches(t *testing.T) {
	for _, build := range gibBuilds {
		catalog := gibCatalog(t, build.root)
		hit, ok := zombieDeathClip(catalog, zombieGibHitClip)
		if !ok || hit.Frames <= 0 || hit.FPS <= 0 {
			t.Fatalf("%s: Charred clip missing from ZombieDeaths", build.name)
		}
		death, ok := zombieDeathClip(catalog, zombieGibDeathClip)
		if !ok || death.Frames <= 0 || death.FPS <= 0 {
			t.Fatalf("%s: Disintegrate clip missing from ZombieDeaths", build.name)
		}
		if death.Loop {
			t.Fatalf("%s: Disintegrate must be one shot", build.name)
		}

		a := &app{sprites: catalog}
		z := zombieState{x: 1000, y: 1000, dying: true, deathState: zombieDeathGib, deathDelay: 1.5, alpha: 1, size: formats.Vec2{X: 48, Y: 48}}
		for _, tc := range []struct {
			age   float64
			frame int
		}{
			{0, 0},
			{1.5 / hit.FPS, 1},
			{(float64(hit.Frames) + .5) / hit.FPS, 0},
			{(float64(hit.Frames) + 2.5) / hit.FPS, 2},
		} {
			z.deathAge = tc.age
			animation, frame, ok := a.zombieGibClip(z)
			if !ok || animation.Texture != hit.Texture || frame != tc.frame {
				t.Fatalf("%s: Charred at %.4f s = frame %d (ok %v, %s), want frame %d of %s", build.name, tc.age, frame, ok, animation.Texture, tc.frame, hit.Texture)
			}
		}

		body := z
		body.gibbed, body.presentAge = true, 0
		animation, frame, ok := a.zombieGibClip(body)
		if !ok || animation.Texture != death.Texture || frame != 0 {
			t.Fatalf("%s: Disintegrate at 0 s = frame %d (ok %v), want frame 0 of %s", build.name, frame, ok, death.Texture)
		}
		body.presentAge = float64(death.Frames+3) / death.FPS
		if _, frame, _ := a.zombieGibClip(body); frame != death.Frames-1 {
			t.Fatalf("%s: Disintegrate past its end = frame %d, want the last frame %d", build.name, frame, death.Frames-1)
		}

		p := &playState{sprites: catalog}
		p.addZombieGibBody(z)
		length := zombieGibClipLength(catalog)
		if math.Abs(length-float64(death.Frames)/death.FPS) > 1e-9 {
			t.Fatalf("%s: presentation length %v, want frames/fps %v", build.name, length, float64(death.Frames)/death.FPS)
		}
		const dt = 1.0 / 60
		ticks := 0
		for len(p.gibBodies) > 0 && ticks < 600 {
			p.updateZombieGibBodies(dt)
			ticks++
		}
		if len(p.gibBodies) != 0 {
			t.Fatalf("%s: gib body never left after %d ticks", build.name, ticks)
		}
		if float64(ticks)*dt < length || float64(ticks-1)*dt >= length {
			t.Fatalf("%s: body removed after %d ticks (%.4f s), want the first tick at or past %.4f s", build.name, ticks, float64(ticks)*dt, length)
		}
	}
}

func TestGibDeathTransitionKeepsDisintegrateBodyNoPop(t *testing.T) {
	for _, build := range gibBuilds {
		_, p := explodingRig()
		p.world.Level.WaveBuild = build.build
		rng := weapons.NewNativeRNG()
		p.rng = &rng
		p.sprites = gibCatalog(t, build.root)

		gib := addZombie(p, "zombie", 1200, 1200)
		killNow(p, gib)
		p.classifyZombieDeath(&p.zombies[gib], 0x16, 1200, 1200)
		if p.zombies[gib].deathState != zombieDeathGib {
			t.Fatalf("%s: flame kill is not a gib", build.name)
		}
		tickWorld(p, 60*2) // the hit timer (at most 1.6 s) ends inside two seconds
		if len(p.zombies) != 0 {
			t.Fatalf("%s: dying gib still in the live list after its hit timer", build.name)
		}
		if len(p.gibBodies) != 1 || !p.gibBodies[0].gibbed {
			t.Fatalf("%s: gib bodies = %d, want one Disintegrate body", build.name, len(p.gibBodies))
		}
		if len(p.bloodPops) != 0 {
			t.Fatalf("%s: a gib made %d blood pops, want none", build.name, len(p.bloodPops))
		}
		if p.levelKills != 1 {
			t.Fatalf("%s: level kills = %d, want 1", build.name, p.levelKills)
		}
		if !containsSound(p.sfxQueue, "SFX_ZOMBIE_DEATH_") {
			t.Fatalf("%s: a gib death queued no zombie death sound (%v)", build.name, p.sfxQueue)
		}
		tickWorld(p, 60) // Disintegrate lasts 5 frames at 8 fps, under a second
		if len(p.gibBodies) != 0 {
			t.Fatalf("%s: gib body still present after its clip ended", build.name)
		}

		normal := addZombie(p, "zombie", 1400, 1400)
		killNow(p, normal)
		p.classifyZombieDeath(&p.zombies[normal], 0x10, 1400, 1400)
		pops := len(p.bloodPops)
		tickWorld(p, 60)
		if len(p.bloodPops) != pops+1 || len(p.gibBodies) != 0 {
			t.Fatalf("%s: normal death pops = %d (was %d), gib bodies = %d; want one pop and no gib", build.name, len(p.bloodPops), pops, len(p.gibBodies))
		}
	}
}

func TestGibTintIsNotDarkened(t *testing.T) {
	gib := zombieState{dying: true, deathState: zombieDeathGib, hitFlash: .1, native: zombieNative{brightness: .9}}
	if got := zombieTint(gib); got != 1 {
		t.Fatalf("gib tint = %v, want 1 (no hit flash darkening)", got)
	}
	normal := zombieState{dying: true, deathState: zombieDeathNormal, hitFlash: .1, native: zombieNative{brightness: 1}}
	if got := zombieTint(normal); got >= 1 {
		t.Fatalf("normal death tint = %v, want the hit flash darkening", got)
	}
}

func TestGibDisintegrateStripCellsRunAcrossTheTexture(t *testing.T) {
	hd := image.Rect(0, 0, 512, 128)
	sd := image.Rect(0, 0, 256, 64)
	cases := []struct {
		name   string
		bounds image.Rectangle
		frame  int
		want   image.Rectangle
	}{
		{"hd first frame", hd, 0, image.Rect(0, 0, 102, 128)},
		{"hd last frame", hd, 4, image.Rect(410, 0, 512, 128)},
		{"sd third frame", sd, 2, image.Rect(102, 0, 154, 64)},
	}
	for _, tc := range cases {
		if got := zombieGibStripCell(tc.frame, 5, tc.bounds); got != tc.want {
			t.Fatalf("%s: cell = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestGibBodiesDrawOnBothCaches draws a gib in its Charred hit phase and as a
// Disintegrate body on each cache, so the clip cells are checked against the textures.
func TestGibBodiesDrawOnBothCaches(t *testing.T) {
	for _, build := range gibBuilds {
		catalog := gibCatalog(t, build.root)
		pack, err := content.NewPack(content.NewSource(build.root))
		if err != nil {
			t.Fatalf("%s: pack: %v", build.name, err)
		}
		a, p := explodingRig()
		a.pack = pack
		a.sprites = catalog
		a.images = map[string]*ebiten.Image{}
		a.sources = map[string]image.Image{}
		a.frontendScaleX, a.frontendScaleY = 1, 1
		p.sprites = catalog
		screen := ebiten.NewImage(640, 480)
		z := zombieState{
			x: 1200, y: 1200, dying: true, deathState: zombieDeathGib, deathDelay: 1.5, deathAge: .3,
			alpha: 1, size: formats.Vec2{X: 48, Y: 60}, texture: "cavezombie", angle: 5,
			native: zombieNative{brightness: 1},
		}
		a.drawZombie(screen, z)
		body := z
		body.gibbed, body.presentAge = true, .2
		a.drawZombie(screen, body)
	}
}

func containsSound(queue []string, prefix string) bool {
	for _, name := range queue {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
