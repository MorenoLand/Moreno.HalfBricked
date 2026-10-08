package game

import (
	"encoding/xml"
	"fmt"
	"image"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
)

// Western train hazard. Evidence (see Research/native/train-hazard-2026-10-08.md and
// hazards-followup-2026-10-08.md):
//   - DLC1/XML/specialCharacters.xml holds the original <Train> record (timing,
//     movement, visual scale, hitbox scale/shift); the 1.2.5 constructor FUN_000fd144
//     reads exactly those attributes.
//   - FUN_000fd7a4 (vtable slot 4) is the per-frame update, FUN_000fd5e8 the
//     whistle/chug sound state, FUN_000fd71c the wait countdown, FUN_000fda6c the
//     collision pass and FUN_000fde94 (vtable slot 6) the draw.
//   - The spawn type "train" resolves to entity type 0x1F in the FUN_001033e4 table.

// trainSpec is the <Train> record of specialCharacters.xml.
type trainSpec struct {
	StartTime, WaitTime                        float64
	Speed                                      float64
	StartX, StartY, EndX, EndY                 float64
	ScaleX, ScaleY                             float64
	HitShiftX, HitShiftY, HitScaleX, HitScaleY float64
}

// defaultTrainSpec mirrors DLC1/XML/specialCharacters.xml and is used when the
// pack does not carry that file (unit tests, stripped caches).
func defaultTrainSpec() trainSpec {
	return trainSpec{StartTime: 10, WaitTime: 30, Speed: -200, StartX: 1800, StartY: 450, EndX: -200, EndY: 450, ScaleX: 1, ScaleY: 1, HitShiftX: -20, HitScaleX: 1.25, HitScaleY: 1}
}

type trainAttrs struct {
	Attrs []xml.Attr `xml:",any,attr"`
}

func (a trainAttrs) get(name string, def float64) float64 {
	for _, attr := range a.Attrs {
		if strings.EqualFold(attr.Name.Local, name) {
			if v, err := strconv.ParseFloat(strings.TrimSpace(attr.Value), 64); err == nil {
				return v
			}
		}
	}
	return def
}

// parseTrainSpec reads the <Train> node of specialCharacters.xml.
func parseTrainSpec(r io.Reader) (trainSpec, error) {
	var doc struct {
		Train struct {
			Timing   trainAttrs `xml:"Timing"`
			Movement trainAttrs `xml:"Movement"`
			Visual   trainAttrs `xml:"Visual"`
			Hitbox   trainAttrs `xml:"Hitbox"`
		} `xml:"Train"`
	}
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return trainSpec{}, err
	}
	d := defaultTrainSpec()
	t := doc.Train
	return trainSpec{
		StartTime: t.Timing.get("startTime", d.StartTime), WaitTime: t.Timing.get("waitTime", d.WaitTime),
		Speed:  t.Movement.get("speed", d.Speed),
		StartX: t.Movement.get("startX", d.StartX), StartY: t.Movement.get("startY", d.StartY),
		EndX: t.Movement.get("endX", d.EndX), EndY: t.Movement.get("endY", d.EndY),
		ScaleX: t.Visual.get("scaleX", d.ScaleX), ScaleY: t.Visual.get("scaleY", d.ScaleY),
		HitShiftX: t.Hitbox.get("hitboxShiftX", d.HitShiftX), HitShiftY: t.Hitbox.get("hitboxShiftY", d.HitShiftY),
		HitScaleX: t.Hitbox.get("hitboxScaleX", d.HitScaleX), HitScaleY: t.Hitbox.get("hitboxScaleY", d.HitScaleY),
	}, nil
}

// loadTrainSpec reads the original record from the content pack, falling back to
// the built-in copy of the same values.
func (a *app) loadTrainSpec() trainSpec {
	if a == nil || a.pack == nil {
		return defaultTrainSpec()
	}
	path, ok := a.pack.SourcePath("DLC1/XML/specialCharacters.xml")
	if !ok {
		return defaultTrainSpec()
	}
	r, err := a.pack.Open(path)
	if err != nil {
		return defaultTrainSpec()
	}
	defer r.Close()
	spec, err := parseTrainSpec(r)
	if err != nil {
		return defaultTrainSpec()
	}
	return spec
}

// Native constants recovered from FUN_000fd144 (doubles at 0x000fd518..0x000fd540).
const (
	trainSpriteW     = 250.0 // |-250| * Visual scaleX
	trainSpriteH     = 220.0 // |-220| * Visual scaleY
	trainBoxHalfX    = 0.464
	trainBoxTopY     = 0.13636
	trainBoxBottomY  = 0.43636
	trainFrameTime   = 0.1   // DAT_000fd9d8
	trainFrames      = 4     // uv step 0.25 (DAT_000fd9dc) wrapping at 1.0
	trainWarnWindow  = 3.5   // rumble window (DAT_000fd794)
	trainWhistleOne  = 2.5   // first whistle (ctor 0xb0)
	trainWhistleTwo  = 1.3   // second whistle (DAT_000fd678)
	trainDamage      = 500.0 // vtable +0x68 call on zombies
	trainPlayerBlow  = 1.0   // FUN_000f2208 argument; the callee halves it (DAT_000f258c = 0.5)
	trainPlayerScale = 0.5

	// FUN_000fd5e8 starts SFX 0x3a (train_chug) once through the 8-slot sound API
	// FUN_0012d618 and keeps the returned handle until the train is no longer
	// active (FUN_0012d9c4). Looping is a property of the OGG: train_chug.ogg
	// carries the Vorbis comment LOOPSAMPLES=1988, which the decoder (v7
	// 0x003927c0) turns into an intro + repeated tail, so the port holds one
	// looping player instead of restarting the clip.
	trainChugLoopSamples = 1988

	// FUN_000fda6c: a projectile (entity type 0x10..0x1e) inside the box picks
	// SFX id 0x2d + floor(3 * rng32 / 2^32) (the carry sum at 0x000fdbd4, i.e.
	// Bounded(3)) and plays it only when FUN_0012da24(id) says that id is not
	// already held by a sound slot. A slot stays held for the length of the clip
	// (OGG headers: Car_hit_1/2/3 = 15584/15008/11712 samples at 22050 Hz).
	trainCarHitCount = 3

	// FUN_000fd680(dt*2, 2.5, 1.5) -> FUN_00095d38(camera, playerPos, 2*dt, 2.5,
	// 1.5, 0.8): the shake setup of FUN_000be1fc re-armed every frame of the last
	// 3.5 s of the countdown for every player (angle multiplier DAT_000fd718 =
	// 0x3f4ccccd).
	trainShakeAmpX = 2.5
	trainShakeAmpY = 1.5

	// FUN_000fde94 marker: size 50 (DAT_000fe1d8 = 0x42480000), placed at the
	// train's screen position with y += |size.y| / 2.1 (DAT_000fe1b0 =
	// 0x40066666) clamped to the viewport, rotated by atan2(cameraCentre - trainPos).
	trainMarkerSize = 50.0
	trainMarkerDrop = 2.1
)

var trainShakeAngle = float64(float32(0.8))

var trainCarHitSeconds = [trainCarHitCount]float64{15584.0 / 22050, 15008.0 / 22050, 11712.0 / 22050}

// trainHazard is the single train object of a level.
type trainHazard struct {
	spec                         trainSpec
	x, y                         float64
	countdown                    float64
	active                       bool
	collisionOff                 bool // native +0xa5, cleared the first frame the scene state is not 1
	whistleThreshold             float64
	whistleArmed                 bool
	animTimer                    float64
	frame                        int
	chugWanted                   bool
	chugStarts                   int
	passKills, passSerial, total int
	carHitUntil                  [trainCarHitCount]float64
}

// box is the narrow-phase hitbox relative to the train position (FUN_000fda6c/FUN_000fd9f4).
func (s trainSpec) box() (left, top, right, bottom float64) {
	w0, h0 := -trainSpriteW*s.ScaleX, -trainSpriteH*s.ScaleY
	return w0 * trainBoxHalfX * s.HitScaleX, h0 * -trainBoxTopY * s.HitScaleY, w0 * -trainBoxHalfX * s.HitScaleX, h0 * -trainBoxBottomY * s.HitScaleY
}

// spawnTrain handles the "train" spawn type. The original creates the entity
// from the wave data; its position, timing and speed come from the XML record.
func (p *playState) spawnTrain() {
	if p.train != nil {
		return
	}
	spec := p.trainSpec
	if spec.Speed == 0 {
		spec = defaultTrainSpec()
	}
	p.train = &trainHazard{spec: spec, x: spec.StartX, y: spec.StartY, countdown: spec.StartTime, whistleThreshold: trainWhistleOne, whistleArmed: true}
}

// updateTrain advances the hazard one 60 Hz frame. It runs inside
// updateBulletsAndKills so kills are cleaned up in the same frame.
//
// Native FUN_000fd7a4: every frame the strip animates and FUN_000fd5e8 runs the
// sounds. When the scene state (**(ctx+DAT_000fd9e4)) is 1 the train waits or
// moves; otherwise the per-player predicate (vtable +0x30, the base stub that
// returns 0 for the train) is false for everyone, so the else branch clears the
// collision flag +0xa5, the active flag +0xa4, submits the pass kill count and
// stops the chug. The port maps scene state 1 to "no cutscene script running",
// the same gate it uses for the wave spawner (v7 0x000c0d80).
func (p *playState) updateTrain() {
	t := p.train
	if t == nil {
		return
	}
	const dt = 1.0 / 60.0
	// Animation: a 4-frame strip advanced every 0.1 s.
	t.animTimer += dt
	if t.animTimer >= trainFrameTime {
		t.animTimer -= trainFrameTime
		t.frame = (t.frame + 1) % trainFrames
	}
	t.updateSounds(p, dt)
	if p.scriptRuntime != nil && !p.scriptRuntime.Done() {
		t.collisionOff, t.active = true, false
		t.finishPass(p)
		return
	}
	if !t.active {
		// FUN_000fd71c: the countdown only runs while the scene state is 1.
		t.countdown -= dt
		if t.countdown > 0 {
			if t.countdown <= trainWarnWindow {
				p.trainRumble(dt)
			}
			return
		}
		t.countdown = t.spec.WaitTime
		t.whistleThreshold, t.whistleArmed = trainWhistleOne, true
		t.active = true
		t.passKills = 0
		t.passSerial++
		return
	}
	t.x += t.spec.Speed * dt
	if (t.spec.Speed < 0 && t.x >= t.spec.EndX) || (t.spec.Speed > 0 && t.x <= t.spec.EndX) {
		if !t.collisionOff {
			p.trainCollide(t)
		}
		return
	}
	// Pass finished: back to the start, count submitted.
	t.x = t.spec.StartX
	t.active = false
	t.finishPass(p)
}

// finishPass submits the pass kill count to the achievement manager (FUN_00142e18
// with the 5-character key "train", compared against the catalog threshold as a
// single value) and zeroes it. Native does this when the pass ends and when the
// scene leaves state 1, never per kill.
func (t *trainHazard) finishPass(p *playState) {
	if t.passKills > 0 {
		if p.achieve.best == nil {
			p.achieve.best = map[string]int32{}
		}
		if n := int32(t.passKills); n > p.achieve.best["train"] {
			p.achieve.best["train"] = n
		}
	}
	t.total += t.passKills
	t.passKills = 0
}

// trainRumble is FUN_000fd680 for the local player: a two-frame camera shake
// armed from the player's position every frame of the last 3.5 s of the wait.
func (p *playState) trainRumble(dt float64) {
	for _, target := range p.livingPlayers() {
		p.startCameraShake(target.x, target.y, dt*2, trainShakeAmpX, trainShakeAmpY, trainShakeAngle)
		return // one camera: the local player's (the native loop runs per player camera)
	}
}

// updateSounds mirrors FUN_000fd5e8: two whistles while the train is about to
// arrive, then the looping chug for as long as it is active.
func (t *trainHazard) updateSounds(p *playState, dt float64) {
	if !t.active {
		t.chugWanted = false
		if t.countdown > 0 && t.countdown <= t.whistleThreshold {
			p.sfxQueue = append(p.sfxQueue, "SFX_TRAIN_WHISTLE")
			if t.whistleArmed {
				t.whistleArmed, t.whistleThreshold = false, trainWhistleTwo
			} else {
				t.whistleThreshold = -1
			}
		}
		return
	}
	if !t.chugWanted {
		t.chugWanted = true
		t.chugStarts++
	}
}

func inTrainBox(t *trainHazard, x, y float64) bool {
	l, top, r, b := t.spec.box()
	return x >= t.x+l && x <= t.x+r && y >= t.y+top && y <= t.y+b
}

// trainCollide is FUN_000fda6c: zombies inside the box take 500 damage each
// frame and the lethal ones are counted for the pass; players take damage;
// projectiles passing through only play SFX_CAR_HIT_n. Kills are counted when the
// zombie was alive before the hit and its death state (+0x330 in 1.2.5, set by
// the takeDamage at 0x000ff16c) is non-zero afterwards. Invulnerable zombies
// (+0x2e4) return from takeDamage without change, so they are skipped.
func (p *playState) trainCollide(t *trainHazard) {
	for index := range p.zombies {
		z := &p.zombies[index]
		if z.health <= 0 || z.dying || z.invulnerable || z.spawnAway || !inTrainBox(t, z.x, z.y) {
			continue
		}
		z.health -= trainDamage
		z.hitFlash = zombieHitFlashDuration
		if z.health <= 0 {
			p.creditKill(killOrigin{gun: "TRAIN", shot: t.passSerial})
			z.dying, z.deathAge = true, 0
			t.passKills++
		}
	}
	damage := trainPlayerBlow * trainPlayerScale
	if p.health > 0 && !p.playerUnspawned && inTrainBox(t, p.x, p.y) {
		p.damagePlayer(0, damage)
	}
	if p.coopActive() {
		for _, c := range p.coop.players {
			if c.body.health > 0 && inTrainBox(t, c.body.x, c.body.y) {
				p.damagePlayer(c.index, damage)
			}
		}
	}
	for _, b := range p.bullets {
		if !inTrainBox(t, b.x, b.y) {
			continue
		}
		var pick uint32
		if p.rng != nil {
			pick = p.rng.Bounded(trainCarHitCount)
		}
		if p.time < t.carHitUntil[pick] {
			continue
		}
		t.carHitUntil[pick] = p.time + trainCarHitSeconds[pick]
		p.sfxQueue = append(p.sfxQueue, fmt.Sprintf("SFX_CAR_HIT_%d", 1+pick))
	}
}

// The train is drawn by FUN_000fde94 (vtable slot 6), which hands its quad to the
// sprite batcher FUN_0009bb08 immediately with the plain submission sequence as
// the sort key. Zombies, players and projectiles instead only register in the
// y-bucket lists in slot 6 (FUN_0006ed18) and are drawn when the buckets are
// flushed, so the train lies under every bucketed entity and ground effect.

// trainOnScreen is FUN_000fdd4c: the sprite rectangle (centre = train position,
// half extents = |size|/2) against the viewport.
func (a *app) trainOnScreen(t *trainHazard) bool {
	w := a.play.world
	zoom := w.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	halfW := trainSpriteW * t.spec.ScaleX / 2 * zoom
	halfH := trainSpriteH * t.spec.ScaleY / 2 * zoom
	sx := (t.x-w.CameraX)*zoom + w.ViewportX
	sy := (t.y-w.CameraY)*zoom + w.ViewportY
	return sx+halfW > 0 && sx-halfW < logicalWidth && sy+halfH > 0 && sy-halfH < logicalHeight
}

// drawTrain draws the current strip frame centered on the train position,
// mirrored horizontally because the art faces right and the train travels left.
func (a *app) drawTrain(target *ebiten.Image, t *trainHazard) {
	if t == nil || a.play == nil || !a.trainOnScreen(t) {
		return
	}
	texture, err := a.Texture("DLC1/Textures/train_SD")
	if err != nil {
		return
	}
	frameH := texture.Bounds().Dy() / trainFrames
	rect := image.Rect(0, t.frame*frameH, texture.Bounds().Dx(), (t.frame+1)*frameH)
	zoom := a.play.world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	sx := (t.x-a.play.world.CameraX)*zoom + a.play.world.ViewportX
	sy := (t.y-a.play.world.CameraY)*zoom + a.play.world.ViewportY
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(rect.Dx())/2, -float64(rect.Dy())/2)
	scaleX := trainSpriteW * t.spec.ScaleX / float64(rect.Dx()) * zoom
	scaleY := trainSpriteH * t.spec.ScaleY / float64(rect.Dy()) * zoom
	if t.spec.Speed < 0 {
		scaleX = -scaleX
	}
	options.GeoM.Scale(scaleX, scaleY)
	options.GeoM.Translate(sx, sy)
	a.drawImage(target, texture.SubImage(rect).(*ebiten.Image), options)
}

// trainMarkerPlacement is the off-screen indicator of FUN_000fde94: the train's
// screen position, moved down by |size.y|/2.1, clamped to the screen, with the
// arrow rotated by atan2(cameraCentre - trainPos) in world space.
func trainMarkerPlacement(t *trainHazard, screenX, screenY, cameraCenterX, cameraCenterY float64) (x, y, angle float64) {
	y = screenY + trainSpriteH*t.spec.ScaleY/trainMarkerDrop
	x = math.Max(0, math.Min(logicalWidth, screenX))
	y = math.Max(0, math.Min(logicalHeight, y))
	angle = math.Atan2(cameraCenterY-t.y, cameraCenterX-t.x)
	return x, y, angle
}

// trainChugPlayback is the single chug handle (native +0xb8).
var trainChugPlayback struct {
	owner  *trainHazard
	player *audio.Player
}

// syncTrainChug starts the looping chug while the train is active and stops it
// afterwards, and silences it when the level that owned it is gone.
func (a *app) syncTrainChug() {
	var wanted *trainHazard
	if a.play != nil && a.play.train != nil && a.play.train.chugWanted {
		wanted = a.play.train
	}
	if trainChugPlayback.player != nil && (wanted == nil || trainChugPlayback.owner != wanted) {
		_ = trainChugPlayback.player.Close()
		trainChugPlayback.player, trainChugPlayback.owner = nil, nil
	}
	if wanted == nil || trainChugPlayback.player != nil || a.silent || !a.options.sound || a.sound == nil {
		return
	}
	player, err := a.sound.PlayTracked(a.scriptSoundPath("SFX_TRAIN_CHUG"), .8, trainChugLoopSamples)
	if err == nil {
		trainChugPlayback.player, trainChugPlayback.owner = player, wanted
	}
}

// drawTrainMarker shows the original's red "markerzombie" arrow at the screen
// edge while the train is on the field but off screen (FUN_000fde94, first half).
func (a *app) drawTrainMarker(screen *ebiten.Image) {
	a.syncTrainChug()
	if a.play == nil || a.play.train == nil || !a.play.train.active || a.play.world == nil {
		return
	}
	t := a.play.train
	if a.trainOnScreen(t) {
		return
	}
	marker, err := a.Texture("Common0/Textures/markerzombie_SD")
	if err != nil {
		return
	}
	w := a.play.world
	zoom := w.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	sx := (t.x-w.CameraX)*zoom + w.ViewportX
	sy := (t.y-w.CameraY)*zoom + w.ViewportY
	centerX := w.CameraX + logicalWidth/(2*zoom)
	centerY := w.CameraY + logicalHeight/(2*zoom)
	px, py, angle := trainMarkerPlacement(t, sx, sy, centerX, centerY)
	b := marker.Bounds()
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	options.GeoM.Translate(-float64(b.Dx())/2, -float64(b.Dy())/2)
	options.GeoM.Scale(trainMarkerSize/float64(b.Dx()), trainMarkerSize/float64(b.Dy()))
	// The art points left (alpha columns grow from the left tip), so rotating by
	// the camera-minus-train direction turns it to face the train.
	options.GeoM.Rotate(angle)
	options.GeoM.Translate(px, py)
	a.drawImage(screen, marker, options)
}

func init() {
	// ALL ABOARD (KILLS/ge/20/train) is earnable from live play.
	achievementKillSpecifics["train"] = true
}
