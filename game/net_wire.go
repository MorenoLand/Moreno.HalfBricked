package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/game/netwire"
)

// Online co-op is host-authoritative: the host runs the whole simulation and
// sends snapshots of what the guests need to draw; guests send only their
// controls. The wire types themselves live in game/netwire; this file converts the
// game's state to and from them.

func round(v float64) float64 { return math.Round(v*10) / 10 }

func (p *playState) wireBody(b bodyState, joined, dead bool) netwire.Player {
	return netwire.Player{X: round(b.x), Y: round(b.y), Angle: b.angle, FlipX: b.flipX, Moving: b.moving, Dead: dead, Health: b.health, MaxHealth: b.maxHealth, Flash: b.flash, Hurt: b.hurt, Gun: b.weapon.GunType, Ammo: b.weapon.Ammo, Grenades: b.grenades, SecondaryType: b.secondaryType, Joined: joined, GunSpin: float64(b.gun.Spin), GunHandX: float64(b.gun.HandX), GunHandY: float64(b.gun.HandY), GunBlink: b.gun.BlinkOn, GunRight: b.gun.Right}
}

// snapshot captures everything a guest needs to draw this frame.
func (p *playState) snapshot(seq uint32) *netwire.Snapshot {
	s := &netwire.Snapshot{Seq: seq, Time: p.time, Score: p.score, Lives: p.lives, Multiplier: p.multiplier, WaveIndex: p.waveIndex, WaveLive: p.wavesStarted, Kills: p.levelKills, ZombieSum: p.levelZombieTotal, Finished: p.wavesFinished, Sfx: append([]string(nil), p.sfxQueue...)}
	s.Paused = p.paused
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
		s.Zombies = append(s.Zombies, p.zombieToWire(z, intern))
	}
	for _, z := range p.gibBodies {
		s.GibBodies = append(s.GibBodies, p.zombieToWire(z, intern))
	}
	for _, shot := range p.zombieShots {
		projectile := shot.projectile
		s.Bullets = append(s.Bullets, netwire.Bullet{X: round(projectile.X), Y: round(projectile.Y), Proj: &projectile})
	}
	for _, b := range p.bullets {
		wb := netwire.Bullet{X: round(b.x), Y: round(b.y), VX: b.vx, VY: b.vy, Life: b.life, Angle: b.angle, Kind: b.kind}
		if b.projectile != nil {
			projectile := *b.projectile
			wb.Proj = &projectile
		}
		s.Bullets = append(s.Bullets, wb)
	}
	for id, e := range p.scriptEntities {
		if e == nil {
			continue
		}
		if e.kind == "pickup" {
			s.Pickups = append(s.Pickups, netwire.Pickup{ID: id, X: round(e.x), Y: round(e.y), Texture: e.texture, Lift: round(e.lift)})
		} else if e.kind == "sprite" && e.texture != "" { // script-created characters; script zombies travel as zombies
			s.Actors = append(s.Actors, netwire.Actor{ID: id, X: round(e.x), Y: round(e.y), Rot: e.rotation, ScaleX: e.scaleX, ScaleY: e.scaleY, Alpha: e.alpha, Texture: e.texture, FlipX: e.flipX, FlipY: e.flipY, Sized: e.sized, RotSet: e.rotationSet, Screen: e.screen, Feet: e.feet, Angle: e.angle, Anim: e.animation, Frame: e.frame})
		}
	}
	for _, b := range p.thrown {
		// Guests draw a thrown bomb as a grenade bullet at its lifted position.
		s.Bullets = append(s.Bullets, netwire.Bullet{X: round(b.x), Y: round(b.y - b.lift + nativeProjectileRenderAnchor), Angle: math.Atan2(b.dirY, b.dirX) + math.Pi/2, Kind: "grenade"})
	}
	for _, w := range p.rex.waves {
		s.RexWaves = append(s.RexWaves, [4]float64{round(w.x), round(w.y), w.age, w.max})
	}
	for _, v := range p.rex.venom {
		s.RexVenom = append(s.RexVenom, netwire.Venom{X: round(v.x), Y: round(v.y), Heading: v.heading, Height: v.height, Size: v.size, Age: v.age, Life: v.life, State: v.state})
	}
	for _, pt := range p.particles {
		s.Particles = append(s.Particles, netwire.Particle{X: round(float64(pt.x)), Y: round(float64(pt.y)), Z: round(float64(pt.z)), Size: round(float64(pt.size)), Age: float64(pt.age), Kind: pt.kind, Left: pt.dirX < 0})
	}
	for _, f := range p.robotShots.flyers {
		s.RobotFlyers = append(s.RobotFlyers, netwire.RobotShot{X: round(f.x), Y: round(f.y), Lift: round(f.lift), Size: f.size, Heading: int(f.heading), Alpha: f.alpha})
	}
	for _, r := range p.robotShots.rockets {
		s.RobotRockets = append(s.RobotRockets, netwire.RobotShot{X: round(r.x), Y: round(r.y), Lift: round(r.lift), Heading: int(r.heading)})
	}
	for _, b := range p.robotShots.beams {
		s.RobotBeams = append(s.RobotBeams, netwire.RobotShot{X: round(b.x), Y: round(b.y), Lift: round(b.lift), Size: b.size, Heading: int(b.heading), Alpha: int(b.alpha)})
	}
	for _, m := range p.mines {
		s.Mines = append(s.Mines, [3]float64{round(m.x), round(m.y), m.age})
	}
	for _, t := range p.sentries {
		s.Sentries = append(s.Sentries, netwire.Sentry{X: round(t.x), Y: round(t.y), Age: t.age, Cooldown: t.cooldown, Gun: t.weapon.GunType, Column: t.column, FlipX: t.flipX, Lift: t.lift, Shake: t.shake})
	}
	for _, e := range p.explosions {
		s.Explosions = append(s.Explosions, [3]float64{round(e.x), round(e.y), e.age})
	}
	for _, b := range p.bloodPops {
		s.Blood = append(s.Blood, netwire.Blood{X: round(b.x), Y: round(b.y), Age: b.age, Variant: b.variant})
	}
	for _, o := range p.portals {
		s.Portals = append(s.Portals, netwire.Portal{X: o.x, Y: o.y, Age: o.age, Size: o.size, Rotation: o.rotationUnits, Speed: o.rotationSpeed, CellX: o.cellX, CellY: o.cellY, Frame: o.frame, Alpha: o.alpha})
	}
	s.Combo, s.HudFlash, s.HudColor = p.comboWire()
	s.Over = p.allPlayersDown()
	if t := p.train; t != nil {
		s.Train = &netwire.Train{X: round(t.x), Y: round(t.y), Active: t.active, Frame: t.frame}
	}
	if p.world != nil {
		s.Zoom = p.world.BaseZoom()
	}
	if p.levelZombieTotal <= 0 && !p.progressMirror {
		s.Progress, s.HasProg = p.waveRemaining()
	}
	return s
}

func (p *playState) bodyFromWire(w netwire.Player, previous bodyState) bodyState {
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
func (p *playState) applySnapshot(s *netwire.Snapshot) {
	p.hostPaused = s.Paused
	p.guestSay = s.Say
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
		p.zombies = append(p.zombies, zombieFromWire(z, str))
	}
	p.gibBodies = p.gibBodies[:0]
	for _, z := range s.GibBodies {
		p.gibBodies = append(p.gibBodies, zombieFromWire(z, str))
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
		p.scriptEntities[e.ID] = &scriptEntity{id: e.ID, kind: "pickup", texture: e.Texture, x: e.X, y: e.Y, scaleX: 1, scaleY: 1, alpha: 1, lift: e.Lift}
	}
	for _, e := range s.Actors {
		p.scriptEntities[e.ID] = &scriptEntity{id: e.ID, kind: "sprite", entityType: e.Texture, texture: e.Texture, x: e.X, y: e.Y, rotation: e.Rot, scaleX: e.ScaleX, scaleY: e.ScaleY, alpha: e.Alpha, flipX: e.FlipX, flipY: e.FlipY, sized: e.Sized, rotationSet: e.RotSet, screen: e.Screen, feet: e.Feet, angle: e.Angle, animation: e.Anim, frame: e.Frame}
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
		p.portals = append(p.portals, portalState{x: o.X, y: o.Y, age: o.Age, size: o.Size, rotationUnits: o.Rotation, rotationSpeed: o.Speed, cellX: o.CellX, cellY: o.CellY, frame: o.Frame, alpha: o.Alpha})
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
	p.particles = p.particles[:0]
	for _, pt := range s.Particles {
		if pt.Kind < 0 || pt.Kind >= len(particleSpecs) {
			continue
		}
		dirX := float32(1)
		if pt.Left {
			dirX = -1
		}
		p.particles = append(p.particles, particle{x: float32(pt.X), y: float32(pt.Y), z: float32(pt.Z), size: float32(pt.Size), age: float32(pt.Age), kind: pt.Kind, dirX: dirX})
	}
	p.robotShots = robotShots{}
	for _, f := range s.RobotFlyers {
		p.robotShots.flyers = append(p.robotShots.flyers, robotFlyer{x: f.X, y: f.Y, lift: f.Lift, size: f.Size, heading: uint16(f.Heading), alpha: f.Alpha})
	}
	for _, r := range s.RobotRockets {
		p.robotShots.rockets = append(p.robotShots.rockets, robotRocket{x: r.X, y: r.Y, lift: r.Lift, heading: uint16(r.Heading)})
	}
	for _, b := range s.RobotBeams {
		p.robotShots.beams = append(p.robotShots.beams, robotBeam{x: b.X, y: b.Y, lift: b.Lift, size: b.Size, heading: uint16(b.Heading), alpha: uint8(b.Alpha)})
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

func (a *app) wireResultsFrom(menu *resultsMenu) *netwire.Results {
	d := menu.Data
	res := &netwire.Results{Score: d.Score, LevelStart: d.LevelStartScore, Flags: d.Flags, Survival: d.Survival, Dead: d.Dead}
	if d.Kills != nil {
		res.Kills = *d.Kills
	}
	if d.Highscore != nil {
		res.Highscore = *d.Highscore
	}
	return res
}

func resultsDataFromWire(r *netwire.Results) resultsData {
	kills, best := r.Kills, r.Highscore
	return resultsData{Survival: r.Survival, Dead: r.Dead, Flags: r.Flags, Score: r.Score, LevelStartScore: r.LevelStart, Kills: &kills, Highscore: &best}
}

// speedyKind restores the one native kind that changes how a guest draws a zombie.
func speedyKind(speedy bool) int {
	if speedy {
		return zombieKindSpeedy
	}
	return 0
}

func ghostsToWire(ghosts [2]zombieGhost) (out [2]netwire.Ghost) {
	for i, g := range ghosts {
		out[i] = netwire.Ghost{X: g.x, Y: g.y, W: g.w, H: g.h, Frame: g.frame, Angle: g.angle, Alpha: g.alpha, FlipX: g.flipX}
	}
	return out
}

func ghostsFromWire(ghosts [2]netwire.Ghost) (out [2]zombieGhost) {
	for i, g := range ghosts {
		out[i] = zombieGhost{x: g.X, y: g.Y, w: g.W, h: g.H, frame: g.Frame, angle: g.Angle, alpha: g.Alpha, flipX: g.FlipX}
	}
	return out
}

// zombieToWire is the snapshot form of one zombie (or gib body).
func (p *playState) zombieToWire(z zombieState, intern func(string) int) netwire.Zombie {
	w := netwire.Zombie{X: round(z.x), Y: round(z.y), Frame: z.frame, Alpha: z.alpha, HitFlash: z.hitFlash, DeathAge: z.deathAge, Health: z.health, Fps: z.fps, RexRage: z.rexRageTimer, Lift: p.rexLift(z), W: z.size.X, H: z.size.Y, Tex: intern(z.texture), Anim: intern(z.animation), Angle: z.angle, FlipX: z.flipX, FlipY: z.flipY, Dying: z.dying, BossRage: z.bossRage, AnimTime: z.animTimeMode, Exploding: p.isExplodingZombie(z), Brightness: z.native.brightness, Speedy: z.native.kind == zombieKindSpeedy, Ghosts: ghostsToWire(z.native.trail.ghosts), DeathState: z.deathState, Gibbed: z.gibbed, PresentAge: z.presentAge}
	w.MaxHealth, w.Boss = z.native.maxHealth, z.native.boss || p.zombieIsBoss(z)
	if r := &z.native.robot; r.active {
		body := &netwire.RobotBody{SizeZ: z.native.sizeZ, Lift: r.lift, ShakeX: r.shakeX, ShakeY: r.shakeY, Colour: r.colour}
		for i, orb := range r.orbs {
			body.Orbs[i] = [3]float64{round(orb.x), round(orb.y), round(orb.z)}
		}
		w.Robot = body
	}
	return w
}

// zombieFromWire rebuilds a zombie from its snapshot form.
func zombieFromWire(z netwire.Zombie, str func(int) string) zombieState {
	state := zombieState{x: z.X, y: z.Y, frame: z.Frame, alpha: z.Alpha, hitFlash: z.HitFlash, deathAge: z.DeathAge, health: z.Health, fps: z.Fps, rexRageTimer: z.RexRage, lift: z.Lift, size: formats.Vec2{X: z.W, Y: z.H}, texture: str(z.Tex), animation: str(z.Anim), angle: z.Angle, flipX: z.FlipX, flipY: z.FlipY, dying: z.Dying, bossRage: z.BossRage, animTimeMode: z.AnimTime, mirrorExploding: z.Exploding, deathState: z.DeathState, gibbed: z.Gibbed, presentAge: z.PresentAge, native: zombieNative{brightness: z.Brightness, kind: speedyKind(z.Speedy), trail: zombieTrail{ghosts: ghostsFromWire(z.Ghosts)}}}
	state.native.maxHealth, state.native.boss = z.MaxHealth, z.Boss
	if z.Robot != nil {
		// The guest draws the future boss with the host's sprite lift, falling-body offset and orbs.
		r := &state.native.robot
		state.native.kind, state.native.sizeZ = zombieKindRobot, z.Robot.SizeZ
		r.active, r.lift, r.shakeX, r.shakeY, r.colour = true, z.Robot.Lift, z.Robot.ShakeX, z.Robot.ShakeY, z.Robot.Colour
		for i, orb := range z.Robot.Orbs {
			r.orbs[i] = robotOrb{x: orb[0], y: orb[1], z: orb[2]}
		}
	}
	return state
}
