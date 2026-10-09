package game

import (
	"encoding/json"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

// Online co-op is host-authoritative: the host runs the whole simulation and
// sends snapshots of what the guests need to draw; guests send only their
// controls. Everything here is the wire format between the two.

type wireMsg struct {
	T       string        `json:"t"`
	Level   string        `json:"level,omitempty"`
	Mode    int           `json:"mode,omitempty"`
	Self    int           `json:"self,omitempty"`
	Players int           `json:"players,omitempty"`
	In      *wireInput    `json:"in,omitempty"`
	Snap    *wireSnapshot `json:"snap,omitempty"`
	Res     *wireResults  `json:"res,omitempty"`
}

type wireInput struct {
	MoveX, MoveY, AimX, AimY float64
	Secondary                bool
	SecondaryCount           int // increments per throw so a dropped packet cannot lose one
}

type wirePlayer struct {
	X, Y                float64
	Angle               int
	FlipX, Moving, Dead bool
	Health, MaxHealth   float64
	Flash               float64
	Hurt                float64
	Age                 float64
	Gun                 string
	Ammo                int
	Grenades            int
	SecondaryType       string
	Joined              bool
	// Buzzsaw / dual pistol weapon object (GunVisual): spin or flash timer, blink
	// flag, hand flag and the offset of the hand that fired last.
	GunSpin, GunHandX, GunHandY float64
	GunBlink, GunRight          bool
}

type wireZombie struct {
	X, Y, Frame, Alpha, HitFlash, DeathAge, Health, Fps, RexRage float64
	Lift                                                         float64 // rex leap lift (rex_shockwave.go)
	W, H                                                         float64
	Tex, Anim                                                    int // indexes into the snapshot's string table
	Angle                                                        int
	FlipX, FlipY, Dying, BossRage, AnimTime                      bool
	Exploding                                                    bool
	Brightness                                                   float64 // +0x2b8 tint (zombie_model.go)
	Speedy                                                       bool    // afterimage trail
	Ghosts                                                       [2]wireGhost
}

type wireBullet struct {
	X, Y, VX, VY, Life, Angle float64
	Kind                      string
	Proj                      *weapons.NativeWeaponProjectile
}

type wirePickup struct {
	ID      int
	X, Y    float64
	Texture string
}

type wireSnapshot struct {
	Seq        uint32
	Time       float64
	Strings    []string
	Players    []wirePlayer
	Zombies    []wireZombie
	Bullets    []wireBullet
	Pickups    []wirePickup
	Mines      [][3]float64
	Sentries   []wireSentry
	Explosions [][3]float64
	Blood      []wireBlood
	Portals    []portalState2
	Score      int
	Lives      int
	Multiplier int
	WaveIndex  int
	Kills      int
	ZombieSum  int
	Finished   bool
	WaveLive   bool
	Train      *wireTrain
	Progress   float64
	Zoom       float64
	HasProg    bool
	Over       bool
	RexWaves   [][4]float64 // x, y, age, max size of each rex shockwave
	RexVenom   []wireVenom
	Sfx        []string
	Combo      []wireCombo
	HudFlash   float64
	HudColor   [3]float64
}

// wireVenom is one rex venom projectile (rex_ai.go) as the guest draws it.
type wireVenom struct {
	X, Y, Heading, Height, Size, Age, Life float64
	State                                  int
}

type wireSentry struct {
	X, Y, Age, Cooldown float64
	Gun                 string
	Column              int
	FlipX               bool
	Lift, Shake         float64
}

type wireBlood struct {
	X, Y, Age float64
	Variant   int
}

// portalState2 mirrors portalState with exported fields.
type portalState2 struct {
	X, Y, Age, Size, Rotation, Speed float64
	CellX, CellY, Frame              int
}

func round(v float64) float64 { return math.Round(v*10) / 10 }

func (p *playState) wireBody(b bodyState, joined, dead bool) wirePlayer {
	return wirePlayer{X: round(b.x), Y: round(b.y), Angle: b.angle, FlipX: b.flipX, Moving: b.moving, Dead: dead, Health: b.health, MaxHealth: b.maxHealth, Flash: b.flash, Hurt: b.hurt, Gun: b.weapon.GunType, Ammo: b.weapon.Ammo, Grenades: b.grenades, SecondaryType: b.secondaryType, Joined: joined, GunSpin: float64(b.gun.Spin), GunHandX: float64(b.gun.HandX), GunHandY: float64(b.gun.HandY), GunBlink: b.gun.BlinkOn, GunRight: b.gun.Right}
}

// snapshot captures everything a guest needs to draw this frame.
func (p *playState) snapshot(seq uint32) *wireSnapshot {
	s := &wireSnapshot{Seq: seq, Time: p.time, Score: p.score, Lives: p.lives, Multiplier: p.multiplier, WaveIndex: p.waveIndex, WaveLive: p.wavesStarted, Kills: p.levelKills, ZombieSum: p.levelZombieTotal, Finished: p.wavesFinished, Sfx: append([]string(nil), p.sfxQueue...)}
	s.Players = append(s.Players, p.wireBody(p.body(), true, p.health <= 0))
	if p.coopActive() {
		for _, c := range p.coop.players {
			w := p.wireBody(c.body, c.joined, c.dead)
			w.Age = c.spawnAge
			s.Players = append(s.Players, w)
		}
	}
	index := map[string]int{}
	intern := func(v string) int {
		if i, ok := index[v]; ok {
			return i
		}
		index[v] = len(s.Strings)
		s.Strings = append(s.Strings, v)
		return index[v]
	}
	for _, z := range p.zombies {
		if z.spawnAway {
			continue
		}
		s.Zombies = append(s.Zombies, wireZombie{X: round(z.x), Y: round(z.y), Frame: z.frame, Alpha: z.alpha, HitFlash: z.hitFlash, DeathAge: z.deathAge, Health: z.health, Fps: z.fps, RexRage: z.rexRageTimer, Lift: p.rexLift(z), W: z.size.X, H: z.size.Y, Tex: intern(z.texture), Anim: intern(z.animation), Angle: z.angle, FlipX: z.flipX, FlipY: z.flipY, Dying: z.dying, BossRage: z.bossRage, AnimTime: z.animTimeMode, Exploding: p.isExplodingZombie(z), Brightness: z.native.brightness, Speedy: z.native.kind == zombieKindSpeedy, Ghosts: ghostsToWire(z.native.trail.ghosts)})
	}
	for _, shot := range p.zombieShots {
		projectile := shot.projectile
		s.Bullets = append(s.Bullets, wireBullet{X: round(projectile.X), Y: round(projectile.Y), Proj: &projectile})
	}
	for _, b := range p.bullets {
		wb := wireBullet{X: round(b.x), Y: round(b.y), VX: b.vx, VY: b.vy, Life: b.life, Angle: b.angle, Kind: b.kind}
		if b.projectile != nil {
			projectile := *b.projectile
			wb.Proj = &projectile
		}
		s.Bullets = append(s.Bullets, wb)
	}
	for id, e := range p.scriptEntities {
		if e != nil && e.kind == "pickup" {
			s.Pickups = append(s.Pickups, wirePickup{ID: id, X: round(e.x), Y: round(e.y), Texture: e.texture})
		}
	}
	for _, b := range p.thrown {
		// Guests draw a thrown bomb as a grenade bullet at its lifted position.
		s.Bullets = append(s.Bullets, wireBullet{X: round(b.x), Y: round(b.y - b.lift + nativeProjectileRenderAnchor), Angle: math.Atan2(b.dirY, b.dirX) + math.Pi/2, Kind: "grenade"})
	}
	for _, w := range p.rex.waves {
		s.RexWaves = append(s.RexWaves, [4]float64{round(w.x), round(w.y), w.age, w.max})
	}
	for _, v := range p.rex.venom {
		s.RexVenom = append(s.RexVenom, wireVenom{X: round(v.x), Y: round(v.y), Heading: v.heading, Height: v.height, Size: v.size, Age: v.age, Life: v.life, State: v.state})
	}
	for _, m := range p.mines {
		s.Mines = append(s.Mines, [3]float64{round(m.x), round(m.y), m.age})
	}
	for _, t := range p.sentries {
		s.Sentries = append(s.Sentries, wireSentry{X: round(t.x), Y: round(t.y), Age: t.age, Cooldown: t.cooldown, Gun: t.weapon.GunType, Column: t.column, FlipX: t.flipX, Lift: t.lift, Shake: t.shake})
	}
	for _, e := range p.explosions {
		s.Explosions = append(s.Explosions, [3]float64{round(e.x), round(e.y), e.age})
	}
	for _, b := range p.bloodPops {
		s.Blood = append(s.Blood, wireBlood{X: round(b.x), Y: round(b.y), Age: b.age, Variant: b.variant})
	}
	for _, o := range p.portals {
		s.Portals = append(s.Portals, portalState2{X: o.x, Y: o.y, Age: o.age, Size: o.size, Rotation: o.rotationUnits, Speed: o.rotationSpeed, CellX: o.cellX, CellY: o.cellY, Frame: o.frame})
	}
	s.Combo, s.HudFlash, s.HudColor = p.comboWire()
	s.Over = p.allPlayersDown()
	if t := p.train; t != nil {
		s.Train = &wireTrain{X: round(t.x), Y: round(t.y), Active: t.active, Frame: t.frame}
	}
	if p.world != nil {
		s.Zoom = p.world.Zoom
	}
	if p.levelZombieTotal <= 0 && !p.progressMirror {
		s.Progress, s.HasProg = p.waveRemaining()
	}
	return s
}

func (p *playState) bodyFromWire(w wirePlayer, previous bodyState) bodyState {
	b := previous
	b.x, b.y, b.angle, b.flipX, b.moving = w.X, w.Y, w.Angle, w.FlipX, w.Moving
	b.health, b.maxHealth, b.flash, b.hurt = w.Health, w.MaxHealth, w.Flash, w.Hurt
	b.grenades, b.secondaryType = w.Grenades, w.SecondaryType
	if b.weapon.GunType != w.Gun {
		if gun, ok := p.weapons.Find(w.Gun); ok {
			b.weapon = gun
		}
	}
	b.weapon.Ammo = w.Ammo
	b.gun.kind = w.Gun
	b.gun.Spin, b.gun.HandX, b.gun.HandY = float32(w.GunSpin), float32(w.GunHandX), float32(w.GunHandY)
	b.gun.BlinkOn, b.gun.Right = w.GunBlink, w.GunRight
	return b
}

// applySnapshot overwrites the guest's mirror of the host's world.
func (p *playState) applySnapshot(s *wireSnapshot) {
	p.time, p.score, p.lives, p.multiplier = s.Time, s.Score, s.Lives, s.Multiplier
	p.applyComboWire(s.Combo, s.HudFlash, s.HudColor)
	p.waveIndex, p.levelKills, p.levelZombieTotal, p.wavesFinished = s.WaveIndex, s.Kills, s.ZombieSum, s.Finished
	if s.WaveLive {
		p.noteWave() // guests raise the same banner when the host's wave changes
	}
	if len(s.Players) > 0 {
		p.setBody(p.bodyFromWire(s.Players[0], p.body()))
	}
	if p.coopActive() {
		for i, c := range p.coop.players {
			if i+1 >= len(s.Players) {
				break
			}
			w := s.Players[i+1]
			c.joined, c.dead = w.Joined, w.Dead
			c.spawnAge = w.Age
			c.body = p.bodyFromWire(w, c.body)
		}
	}
	str := func(i int) string {
		if i >= 0 && i < len(s.Strings) {
			return s.Strings[i]
		}
		return ""
	}
	p.zombies = p.zombies[:0]
	for _, z := range s.Zombies {
		p.zombies = append(p.zombies, zombieState{x: z.X, y: z.Y, frame: z.Frame, alpha: z.Alpha, hitFlash: z.HitFlash, deathAge: z.DeathAge, health: z.Health, fps: z.Fps, rexRageTimer: z.RexRage, lift: z.Lift, size: formats.Vec2{X: z.W, Y: z.H}, texture: str(z.Tex), animation: str(z.Anim), angle: z.Angle, flipX: z.FlipX, flipY: z.FlipY, dying: z.Dying, bossRage: z.BossRage, animTimeMode: z.AnimTime, mirrorExploding: z.Exploding, native: zombieNative{brightness: z.Brightness, kind: speedyKind(z.Speedy), trail: zombieTrail{ghosts: ghostsFromWire(z.Ghosts)}}})
	}
	p.bullets = p.bullets[:0]
	for _, b := range s.Bullets {
		nb := bullet{x: b.X, y: b.Y, vx: b.VX, vy: b.VY, life: b.Life, angle: b.Angle, kind: b.Kind}
		if b.Proj != nil {
			projectile := *b.Proj
			nb.projectile = &projectile
		}
		p.bullets = append(p.bullets, nb)
	}
	p.scriptEntities = map[int]*scriptEntity{}
	for _, e := range s.Pickups {
		p.scriptEntities[e.ID] = &scriptEntity{id: e.ID, kind: "pickup", texture: e.Texture, x: e.X, y: e.Y, scaleX: 1, scaleY: 1, alpha: 1}
	}
	p.mines = p.mines[:0]
	for _, m := range s.Mines {
		p.mines = append(p.mines, mineState{x: m[0], y: m[1], age: m[2]})
	}
	p.sentries = p.sentries[:0]
	for _, t := range s.Sentries {
		turret := sentryState{x: t.X, y: t.Y, age: t.Age, cooldown: t.Cooldown, column: t.Column, flipX: t.FlipX, lift: t.Lift, shake: t.Shake, state: sentryStateActive}
		if gun, ok := p.weapons.Find(t.Gun); ok {
			turret.weapon = gun
		}
		p.sentries = append(p.sentries, turret)
	}
	p.explosions = p.explosions[:0]
	for _, e := range s.Explosions {
		p.explosions = append(p.explosions, explosionState{x: e[0], y: e[1], age: e[2]})
	}
	p.bloodPops = p.bloodPops[:0]
	for _, b := range s.Blood {
		p.bloodPops = append(p.bloodPops, bloodPop{x: b.X, y: b.Y, age: b.Age, variant: b.Variant})
	}
	p.portals = p.portals[:0]
	for _, o := range s.Portals {
		p.portals = append(p.portals, portalState{x: o.X, y: o.Y, age: o.Age, size: o.Size, rotationUnits: o.Rotation, rotationSpeed: o.Speed, cellX: o.CellX, cellY: o.CellY, frame: o.Frame})
	}
	p.progressMirror, p.progressValue = s.HasProg, s.Progress
	switch {
	case s.Train == nil:
		p.train = nil
	default:
		if p.train == nil {
			spec := p.trainSpec
			if spec.Speed == 0 {
				spec = defaultTrainSpec()
			}
			p.train = &trainHazard{spec: spec}
		}
		p.train.x, p.train.y, p.train.active, p.train.frame = s.Train.X, s.Train.Y, s.Train.Active, s.Train.Frame
	}
	if p.world != nil && s.Zoom > 0 && p.world.Zoom != s.Zoom {
		p.world.SetZoom(s.Zoom)
	}
	p.rex.waves, p.rex.venom = nil, nil
	for _, w := range s.RexWaves {
		p.rex.waves = append(p.rex.waves, rexShockwave{x: w[0], y: w[1], age: w[2], max: w[3]})
	}
	for _, v := range s.RexVenom {
		p.rex.venom = append(p.rex.venom, rexVenom{x: v.X, y: v.Y, heading: v.Heading, height: v.Height, size: v.Size, age: v.Age, life: v.Life, state: v.State})
	}
	p.sfxQueue = append(p.sfxQueue, s.Sfx...)
}

func encodeWire(m wireMsg) []byte {
	raw, _ := json.Marshal(m)
	return raw
}

func decodeWire(raw []byte) (wireMsg, bool) {
	var m wireMsg
	if json.Unmarshal(raw, &m) != nil || m.T == "" {
		return wireMsg{}, false
	}
	return m, true
}

// wireResults is what the guests' results screen shows; only the host can pick
// Replay or Menu.
type wireResults struct {
	Score, Kills, Highscore int32
	LevelStart              int32
	Flags                   uint32
	Survival, Dead          bool
}

func (a *app) wireResultsFrom(menu *resultsMenu) *wireResults {
	d := menu.Data
	res := &wireResults{Score: d.Score, LevelStart: d.LevelStartScore, Flags: d.Flags, Survival: d.Survival, Dead: d.Dead}
	if d.Kills != nil {
		res.Kills = *d.Kills
	}
	if d.Highscore != nil {
		res.Highscore = *d.Highscore
	}
	return res
}

func (r *wireResults) data() resultsData {
	kills, best := r.Kills, r.Highscore
	return resultsData{Survival: r.Survival, Dead: r.Dead, Flags: r.Flags, Score: r.Score, LevelStartScore: r.LevelStart, Kills: &kills, Highscore: &best}
}

// wireTrain is the level's train as the guests draw it.
type wireTrain struct {
	X, Y   float64
	Active bool
	Frame  int
}

// speedyKind restores the one native kind that changes how a guest draws a zombie.
func speedyKind(speedy bool) int {
	if speedy {
		return zombieKindSpeedy
	}
	return 0
}

// wireGhost is a speedy zombie afterimage as it travels in a snapshot.
type wireGhost struct {
	X, Y, W, H, Frame float64
	Angle, Alpha      int
	FlipX             bool
}

func ghostsToWire(ghosts [2]zombieGhost) (out [2]wireGhost) {
	for i, g := range ghosts {
		out[i] = wireGhost{X: g.x, Y: g.y, W: g.w, H: g.h, Frame: g.frame, Angle: g.angle, Alpha: g.alpha, FlipX: g.flipX}
	}
	return out
}

func ghostsFromWire(ghosts [2]wireGhost) (out [2]zombieGhost) {
	for i, g := range ghosts {
		out[i] = zombieGhost{x: g.X, y: g.Y, w: g.W, h: g.H, frame: g.Frame, angle: g.Angle, alpha: g.Alpha, flipX: g.FlipX}
	}
	return out
}
