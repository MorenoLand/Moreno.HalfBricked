package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"image"
	"math"
	"strings"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
	"github.com/hajimehoshi/ebiten/v2"
)

const originalTrainXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<SpecChar>
	<Train>
		<Timing startTime="10.0" waitTime="30.0"/>
		<Movement speed="-200.0" startX="1800.0" startY="450.0" endX="-200.0" endY="450.0"/>
		<Visual scaleX="1" scaleY="1" />
		<Hitbox hitboxShiftX="-20.0" hitboxShiftY="0.0" hitboxScaleX="1.25" hitboxScaleY="1.0" />
	</Train>
</SpecChar>`

func trainTestPlay() *playState {
	return &playState{
		world: &viewer.Viewer{Level: formats.Level{Width: 64, Height: 64, Layers: map[formats.LayerKind][]uint32{formats.LayerC: openLayer(64 * 64)}}},
		x:     100, y: 200, tileSize: 32, health: 1, maxHealth: 1, lives: 3,
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestTrainSpecParsesOriginalXMLAndMatchesDefault(t *testing.T) {
	spec, err := parseTrainSpec(strings.NewReader(originalTrainXML))
	if err != nil {
		t.Fatal(err)
	}
	if spec != defaultTrainSpec() {
		t.Fatalf("parsed %+v want %+v", spec, defaultTrainSpec())
	}
	left, top, right, bottom := spec.box()
	// Native box: x +-145 around the train, y 30..96 below it.
	if !near(left, -145) || !near(right, 145) || !near(top, 30) || !near(bottom, 96) {
		t.Fatalf("box %v %v %v %v", left, top, right, bottom)
	}
	// The lane (450+30 .. 450+96) is exactly the two rail tile rows 15 and 16.
	if !near(450+top, 15*32) || math.Abs(450+bottom-17*32) > 2 {
		t.Fatalf("lane %v..%v does not cover rail rows", 450+top, 450+bottom)
	}
}

func TestTrainSpecLoadsFromEveryCache(t *testing.T) {
	roots := achievementCaches()
	if len(roots) == 0 {
		t.Skip("original asset cache unavailable")
	}
	for _, root := range roots {
		pack, err := content.NewPack(content.NewSource(root))
		if err != nil {
			t.Fatal(err)
		}
		a := &app{pack: pack, images: map[string]*ebiten.Image{}, sources: map[string]image.Image{}}
		if got := a.loadTrainSpec(); got != defaultTrainSpec() {
			t.Fatalf("%s: spec %+v", root, got)
		}
		if _, ok := pack.TexturePath("DLC1/Textures/train_SD"); !ok {
			t.Fatalf("%s: train texture missing", root)
		}
		if _, err := a.Texture("DLC1/Textures/train_SD"); err != nil {
			t.Fatalf("%s: %v", root, err)
		}
		if _, err := a.Texture("Common0/Textures/markerzombie_SD"); err != nil {
			t.Fatalf("%s: %v", root, err)
		}
	}
}

func TestTrainSpawnTypeCreatesHazardNotZombie(t *testing.T) {
	p := trainTestPlay()
	rng := weapons.NewNativeRNG()
	p.rng = &rng
	layer := p.world.Level.Layers[formats.LayerC]
	layer[5*64+5] = 3 // spawn marker for index 1
	spawner := formats.Spawner{Count: 1, Index: 1, Types: []formats.SpawnType{{Name: "train", Chance: 1}}}
	p.spawnZombie(spawner, 0)
	if p.train == nil || len(p.zombies) != 0 {
		t.Fatalf("train %v zombies %d", p.train, len(p.zombies))
	}
	if p.train.x != 1800 || p.train.y != 450 || p.train.countdown != 10 || p.train.active {
		t.Fatalf("initial state %+v", *p.train)
	}
	first := p.train
	p.spawnZombie(spawner, 0)
	if p.train != first {
		t.Fatal("a second train spawn replaced the hazard")
	}
}

func countSfx(p *playState, name string) int {
	n := 0
	for _, s := range p.sfxQueue {
		if s == name {
			n++
		}
	}
	return n
}

func TestTrainTimingWarningAndPass(t *testing.T) {
	p := trainTestPlay()
	p.spawnTrain()
	tr := p.train
	firstWhistle, secondWhistle, activated := -1, -1, -1
	whistles := 0
	for frame := 0; frame < 60*100 && !(activated >= 0 && !tr.active); frame++ {
		wasActive := tr.active
		p.updateTrain()
		if n := countSfx(p, "SFX_TRAIN_WHISTLE"); n > whistles {
			whistles = n
			if firstWhistle < 0 {
				firstWhistle = frame
			} else if secondWhistle < 0 {
				secondWhistle = frame
			}
		}
		if tr.active && !wasActive && activated < 0 {
			activated = frame
		}
	}
	seconds := func(frame int) float64 { return float64(frame+1) / 60 }
	// Start timer 10 s; whistles with 2.5 s and 1.3 s left; the train runs 2000 px at 200 px/s.
	if math.Abs(seconds(firstWhistle)-7.5) > .05 || math.Abs(seconds(secondWhistle)-8.7) > .05 || math.Abs(seconds(activated)-10) > .05 {
		t.Fatalf("whistles at %.2f and %.2f, activation %.2f", seconds(firstWhistle), seconds(secondWhistle), seconds(activated))
	}
	if whistles != 2 {
		t.Fatalf("whistles %d want 2", whistles)
	}
	// The chug is one held looping handle (train_chug.ogg LOOPSAMPLES=1988), not a restart.
	tr.updateSounds(p, 1.0/60) // FUN_000fd5e8 releases the handle on the frame after the pass ends
	if tr.chugStarts != 1 || tr.chugWanted {
		t.Fatalf("chug starts %d wanted %v after the pass", tr.chugStarts, tr.chugWanted)
	}
	if tr.active || tr.x != 1800 || tr.countdown != 30 {
		t.Fatalf("after pass: active %v x %v countdown %v", tr.active, tr.x, tr.countdown)
	}
	// 10 s on screen: 2000 px / 200 px/s.
	passFrames := 0
	p2 := trainTestPlay()
	p2.spawnTrain()
	startPass(p2)
	for p2.train.active || passFrames == 0 {
		p2.updateTrain()
		passFrames++
		if passFrames > 5000 {
			t.Fatal("train never left")
		}
	}
	if math.Abs(float64(passFrames)/60-10) > .1 {
		t.Fatalf("pass lasted %.2f s", float64(passFrames)/60)
	}
}

func TestTrainCountdownFreezesDuringCutscene(t *testing.T) {
	a := newAchievementRig(t, nil, formats.WeaponCatalog{})
	p := a.p
	p.spawnTrain()
	h := scriptZombieActivityHost(t)
	p.scriptRuntime = h.play.scriptRuntime
	for i := 0; i < 120; i++ {
		p.updateTrain()
	}
	if p.train.countdown != 10 {
		t.Fatalf("countdown ran during a script: %v", p.train.countdown)
	}
}

func startPass(p *playState) {
	for !p.train.active {
		p.updateTrain()
	}
}

func TestTrainKillsZombiesInLaneOnlyAndCreditsThem(t *testing.T) {
	p := trainTestPlay()
	p.hudVisible, p.multiplier = true, 1
	p.spawnTrain()
	startPass(p)
	// Put the train over (1000,500); a lane zombie, one above the lane and one far ahead.
	p.train.x = 1000
	p.zombies = []zombieState{
		{x: 1000, y: 500, health: 100, rawPoints: 100},
		{x: 1100, y: 540, health: 300, rawPoints: 300},
		{x: 1000, y: 470, health: 100}, // above the lane
		{x: 1000, y: 550, health: 100}, // below the lane
		{x: 1200, y: 500, health: 100}, // inside the lane but the train is still 55 px away
	}
	p.updateBulletsAndKills()
	if p.train.passKills != 2 || p.combatKillCount != 2 || p.achieve.best["train"] != 0 {
		t.Fatalf("pass %d combat %d best %v (the count is submitted when the pass ends)", p.train.passKills, p.combatKillCount, p.achieve.best)
	}
	for p.train.active {
		p.updateTrain()
	}
	if p.achieve.best["train"] != 2 {
		t.Fatalf("pass-end submission %v, want 2", p.achieve.best)
	}
	p.train = nil // let the death animation finish
	for i := 0; i < 60; i++ {
		p.updateZombies()
		p.updateBulletsAndKills()
	}
	if p.levelKills != 2 {
		t.Fatalf("kills %d want 2 (only the lane zombies in range)", p.levelKills)
	}
	if p.score <= 0 {
		t.Fatalf("score %d: train kills must be scored like other kills", p.score)
	}
	if len(p.zombies) != 3 {
		t.Fatalf("remaining zombies %d want 3", len(p.zombies))
	}
}

func TestTrainDamagesPlayerAndPlaysHitSoundForProjectiles(t *testing.T) {
	p := trainTestPlay()
	p.spawnTrain()
	startPass(p)
	p.train.x, p.x, p.y = 1000, 1000, 520
	p.bullets = []bullet{{x: 1010, y: 520, life: 1}}
	p.updateTrain()
	if p.health != 0.5 {
		t.Fatalf("first hit left %v health, want 0.5 (damage 1.0 halved)", p.health)
	}
	p.updateTrain()
	if p.health != 0 {
		t.Fatalf("health %v after two frames", p.health)
	}
	hit := 0
	for _, s := range p.sfxQueue {
		if strings.HasPrefix(s, "SFX_CAR_HIT_") {
			hit++
		}
	}
	if hit != 1 || len(p.bullets) != 1 {
		t.Fatalf("hit sounds %d, bullets %d (bullets pass through)", hit, len(p.bullets))
	}
	// A player outside the lane is untouched.
	q := trainTestPlay()
	q.spawnTrain()
	startPass(q)
	q.train.x, q.x, q.y = 1000, 1000, 400
	q.updateTrain()
	if q.health != 1 {
		t.Fatalf("player outside the lane lost health: %v", q.health)
	}
}

func allAboardEntries(t *testing.T) map[string]formats.Achievement {
	t.Helper()
	found := map[string]formats.Achievement{}
	for _, root := range achievementCaches() {
		pack, err := content.NewPack(content.NewSource(root))
		if err != nil {
			t.Fatal(err)
		}
		catalog, err := pack.Achievements()
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range catalog {
			if entry.Name == "ALL ABOARD" {
				found[root] = entry
			}
		}
	}
	return found
}

func TestAllAboardUnlocksFromOnePassOfTheTrain(t *testing.T) {
	entries := allAboardEntries(t)
	if len(entries) == 0 {
		t.Skip("original asset cache unavailable")
	}
	for root, entry := range entries {
		if entry.Type != "KILLS" || entry.SpecificType != "train" || entry.Total != 20 {
			t.Fatalf("%s: unexpected entry %+v", root, entry)
		}
		if !achievementEventTracked(entry) {
			t.Fatalf("%s: ALL ABOARD must be tracked", root)
		}
		rig := newAchievementRig(t, formats.AchievementCatalog{entry}, formats.WeaponCatalog{})
		p := rig.p
		p.hudVisible, p.multiplier = true, 1
		p.spawnTrain()
		startPass(p)
		// 19 zombies in this pass.
		for i := 0; i < 19; i++ {
			p.zombies = append(p.zombies, zombieState{x: p.train.x + float64(i), y: 520, health: 1})
		}
		rig.tick(1)
		if rig.unlocked(entry.ID) {
			t.Fatalf("%s: unlocked at 19 (best %v)", root, p.achieve.best)
		}
		// Let the pass finish: the count is submitted then; 19 < 20.
		for p.train.active {
			p.updateTrain()
		}
		rig.tick(1)
		if rig.unlocked(entry.ID) || p.achieve.best["train"] != 19 {
			t.Fatalf("%s: best %v after the first pass", root, p.achieve.best)
		}
		// 10 more in the next pass must not add up to 20.
		startPass(p)
		for i := 0; i < 10; i++ {
			p.zombies = append(p.zombies, zombieState{x: p.train.x + float64(i), y: 520, health: 1})
		}
		rig.tick(1)
		for p.train.active {
			p.updateTrain()
		}
		rig.tick(1)
		if rig.unlocked(entry.ID) {
			t.Fatalf("%s: kills from separate passes were added together", root)
		}
		// 20 within one pass unlock it, but only when the pass ends (not at the 20th kill).
		startPass(p)
		for i := 0; i < 20; i++ {
			p.zombies = append(p.zombies, zombieState{x: p.train.x + float64(i), y: 520, health: 1})
		}
		rig.tick(1)
		if rig.unlocked(entry.ID) {
			t.Fatalf("%s: unlocked at the 20th kill instead of at the end of the pass", root)
		}
		for p.train.active {
			p.updateTrain()
		}
		rig.tick(1)
		if !rig.unlocked(entry.ID) {
			t.Fatalf("%s: 20 kills in one pass did not unlock at pass end (best %v)", root, p.achieve.best)
		}
	}
}

func TestTrainMarkerPlacementFollowsNativeFormula(t *testing.T) {
	tr := &trainHazard{spec: defaultTrainSpec(), x: 1800, y: 450}
	// Train far to the right of a camera centred at (240,160): the arrow sits on the
	// right edge, 220/2.1 below the train screen y, turned to face the train.
	x, y, angle := trainMarkerPlacement(tr, 2000, 100, 240, 160)
	if x != logicalWidth || !near(y, 100+220/2.1) {
		t.Fatalf("marker at %v,%v", x, y)
	}
	if want := math.Atan2(160-450, 240-1800); !near(angle, want) {
		t.Fatalf("angle %v want %v", angle, want)
	}
	// The art points left, so a train to the right needs a half turn.
	if math.Abs(math.Abs(angle)-math.Pi) > .3 {
		t.Fatalf("rotation %v does not turn the left-pointing art towards a train on the right", angle)
	}
	// Clamped to the screen on both axes.
	if _, y, _ := trainMarkerPlacement(tr, -50, 5000, 240, 160); y != logicalHeight {
		t.Fatalf("y %v not clamped to the screen", y)
	}
}

func TestTrainCarHitUsesRNGPickAndPerClipThrottle(t *testing.T) {
	p := trainTestPlay()
	rng := weapons.NewNativeRNG()
	p.rng = &rng
	p.spawnTrain()
	startPass(p)
	p.train.x, p.x, p.y = 1000, 100, 100
	p.bullets = []bullet{{x: 1010, y: 520, life: 1}}
	seen := map[string]int{}
	for frame := 0; frame < 600; frame++ {
		p.time = float64(frame) / 60
		p.sfxQueue = nil
		p.trainCollide(p.train)
		for _, s := range p.sfxQueue {
			seen[s]++
		}
	}
	for name := range seen {
		if !strings.HasPrefix(name, "SFX_CAR_HIT_") {
			t.Fatalf("unexpected sound %s", name)
		}
	}
	if len(seen) != 3 {
		t.Fatalf("a long overlap should use all three clips, got %v", seen)
	}
	// 10 s of overlap with clips lasting 0.53..0.71 s: each id at most once per clip length.
	for name, n := range seen {
		if n < 5 || float64(n) > 10/0.53+1 {
			t.Fatalf("%s played %d times", name, n)
		}
	}
	// While one clip is held its id is never queued again.
	p.train.carHitUntil = [3]float64{1e9, 1e9, 1e9}
	p.sfxQueue = nil
	p.trainCollide(p.train)
	if len(p.sfxQueue) != 0 {
		t.Fatalf("held clip replayed: %v", p.sfxQueue)
	}
}

func TestTrainRumbleRunsOnlyInTheLastThreeAndAHalfSeconds(t *testing.T) {
	p := trainTestPlay()
	rng := weapons.NewNativeRNG()
	p.rng = &rng
	p.spawnTrain()
	p.train.countdown = 4
	for i := 0; i < 20; i++ {
		p.updateTrain()
	}
	if p.shake.active() || p.shake.total != 0 {
		t.Fatalf("rumble before the 3.5 s window: %+v", p.shake)
	}
	for i := 0; i < 40 && p.train.countdown > 3.4; i++ {
		p.updateTrain()
	}
	if p.shake.total <= 0 || math.Abs(p.shake.total-2.0/60) > 1e-9 {
		t.Fatalf("shake %+v, want a 2-frame shake (FUN_000fd680 dt*2)", p.shake)
	}
}

func TestTrainGoesInertWhenTheSceneLeavesStateOne(t *testing.T) {
	a := newAchievementRig(t, nil, formats.WeaponCatalog{})
	p := a.p
	p.spawnTrain()
	startPass(p)
	p.train.passKills = 3
	h := scriptZombieActivityHost(t)
	p.scriptRuntime = h.play.scriptRuntime
	p.updateTrain()
	if p.train.active || !p.train.collisionOff || p.achieve.best["train"] != 3 {
		t.Fatalf("active %v collisionOff %v best %v: native clears +0xa4/+0xa5 and submits the count", p.train.active, p.train.collisionOff, p.achieve.best)
	}
}
