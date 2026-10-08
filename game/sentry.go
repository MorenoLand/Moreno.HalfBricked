package game

import (
	"image"
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"github.com/hajimehoshi/ebiten/v2"
)

// The sentry gun actor.
//
// Sources: libmortargame.so(v7) FUN_0009a988 (init), FUN_0009b238 (update),
// FUN_0009acb8 (dying), FUN_0009ac74 (death), FUN_0009ade4 (target search),
// FUN_0009b320 (draw), cross-checked against the same functions in
// libmortargame.so (1.2.5): FUN_000f8dc8, FUN_000f96d8, FUN_000f9150,
// FUN_000f910c. The 1.2.5 actor matches the v7 one in every constant read.
// Research/native/secondary-weapons-2026-10-08.md has the full evidence.
//
// Behaviour:
//   - Lifetime is NOT a timer. The turret's gun is a clone of one of the player's
//     weapons (1.2.5: FUN_00112c28 copies the shotgun, uzi, flamer or bazooka
//     weapon object of weapon manager 4 and re-attaches it, so its ammo is that
//     weapon's catalog Ammo and its rate, life, speed and spread are that
//     weapon's). Every shot costs one ammo and the actor dies the update after
//     the ammo reaches 0 (the vtable +0x40 check at the end of FUN_000f96d8).
//   - Range is Life * Speed * .5 of the cloned weapon (FUN_000f8dc8 literal .5).
//   - The target is a held reference: it is searched only while none is held
//     (nearest living zombie-class entity strictly inside the range circle,
//     FUN_0009ade4) and dropped when it dies or leaves the range.
//   - The turret turns by 10% of the remaining angle each frame (truncated to an
//     integer, so it stops within 9 units of the goal) and shoots only when the
//     16-way facing column equals the column of the angle to the target.
//   - Without a target it picks one random 16-bit angle and turns toward it.
//   - Dying lasts 1 s with the head shaking, then a harmless mine-class blast
//     (a life-0 grenade owned by the sentry) goes off and the actor is removed.
const (
	sentryDropSpeed   = 24.0                // FUN_000f96d8 literal 0x000f9104
	sentryRestLift    = -18.0               // 0x000f9108
	sentryTurnRate    = float32(.1)         // 0x000f96c4
	sentryStartAngle  = 0x47ce              // FUN_000f8dc8
	sentryRangeScale  = float32(.5)         // 0x000f9008
	sentryDyingTime   = 1.0                 // FUN_000f910c
	sentryStandSize   = 40.0                // FUN_0009b320 literal 0x0009b4d8
	sentryHeadWidth   = 68.0                // 0x0009b4e0
	sentryHeadHeight  = 40.0                // 0x0009b4dc
	sentryCellPitch   = 112.0 / 1024.0      // 0x0009b4d4 = 0x3de00000
	sentryColumnScale = float32(1.0 / 4096) // 0x000f96c0 = 0x39800000
	sentryUziHold     = float32(.5)         // FUN_000aef38 literal 0x000af008
	sentryBlastSound  = "SFX_MINE_EXPLODE"  // FUN_000a3ca8 plays sound id 8
	sentryDeathSound  = "SFX_SENTRY_DEATH"  // FUN_000f910c / FUN_0009ac74
	sentrySpawnSound  = "SFX_SENTRY_SPAWN"  // FUN_000f8dc8 / FUN_0009a988
	sentryStateDeploy = 0
	sentryStateActive = 1
	sentryStateDying  = 2
)

// sentryDyingShake is the head x offset per dying frame (v7 table 0x005672ec).
var sentryDyingShake = [10]float64{-1, -1, 2, 2, 0, 0, -2, -2, 1, 1}

type sentryState struct {
	x, y, age, cooldown float64
	id                  int
	// weapon is the player weapon record the gun is cloned from.
	weapon formats.Weapon
	// column is the native 16-way facing column (+0x58); flipX is unused by the
	// native draw (columns 9..15 mirror cells 7..1) and kept for the wire format.
	column int
	flipX  bool

	state       int
	lift        float64 // +0x64: 0 -> -18 while deploying
	angle, want uint16  // +0x32 and +0x54
	ammo        int
	rangePx     float64
	hasTarget   bool
	targetIdx   int
	targetX     float64
	targetY     float64
	dying       float64
	shake       float64
	shakeIdx    int
	uziIdleHold bool
}

// sentryGunRecord maps a sentry variant to the cloned weapon's catalog name.
func sentryGunRecord(variant string) string {
	switch variant {
	case "SENTRY_SHOTGUN":
		return "SHOTGUN"
	case "SENTRY_UZI":
		return "UZI"
	case "SENTRY_FLAMER":
		return "FLAMER"
	case "SENTRY_BAZOOKA":
		return "BAZOOKA"
	}
	return ""
}

// sentryVariants are the four variants in native order (vtable cache ids 0xe..0x11).
var sentryVariants = [4]string{"SENTRY_SHOTGUN", "SENTRY_UZI", "SENTRY_FLAMER", "SENTRY_BAZOOKA"}

// randomSentryVariant is FUN_000f8dc8's pick for the random pickup marker 0x12:
// floor(rand * 5) + 0xe, re-rolled while it equals 0x12.
func (p *playState) randomSentryVariant() string {
	p.ensureRNG()
	for {
		if v := p.rng.Bounded(5); v < 4 {
			return sentryVariants[v]
		}
	}
}

func (p *playState) ensureRNG() {
	if p.rng == nil {
		rng := weapons.NewNativeRNG()
		p.rng = &rng
	}
}

// deploySentry creates the turret at (x, y) with the gun cloned from variant.
func (p *playState) deploySentry(x, y float64, variant string) bool {
	record, ok := p.weapons.Find(sentryGunRecord(variant))
	if !ok {
		return false
	}
	p.achieve.sentrySerial++
	s := sentryState{x: x, y: y, weapon: record, id: p.achieve.sentrySerial, angle: sentryStartAngle, want: sentryStartAngle,
		ammo: record.Ammo, rangePx: float64(float32(record.Life) * float32(record.Speed) * sentryRangeScale), uziIdleHold: record.GunType == "UZI"}
	s.column = sentryColumn(s.angle)
	p.sentries = append(p.sentries, s)
	p.sfxQueue = append(p.sfxQueue, sentrySpawnSound)
	return true
}

// sentryColumn is the 16-way facing column of a 16-bit angle (FUN_000f96d8).
func sentryColumn(angle uint16) int {
	value := float32((sentryStartAngle - int(angle)) & 0xffff)
	column := int(value * sentryColumnScale)
	if column > 15 {
		column = 15
	}
	if column < 0 {
		column = 0
	}
	return column
}

// angleDifference is FUN_00072a40 / FUN_000c7c60: the shortest signed difference.
func angleDifference(a, b uint16) int {
	d := int(int16(a - b))
	return d
}

func (p *playState) sentryTargetValid(s *sentryState) bool {
	if s.targetIdx < 0 || s.targetIdx >= len(p.zombies) {
		return false
	}
	z := p.zombies[s.targetIdx]
	return z.health > 0 && !z.dying && !z.spawnAway && math.Abs(z.x-s.targetX) < 8 && math.Abs(z.y-s.targetY) < 8
}

// sentryRelocate finds the held target again after the zombie list changed.
func (p *playState) sentryRelocate(s *sentryState) bool {
	for index, z := range p.zombies {
		if z.health > 0 && !z.dying && !z.spawnAway && math.Abs(z.x-s.targetX) < 8 && math.Abs(z.y-s.targetY) < 8 {
			s.targetIdx = index
			return true
		}
	}
	return false
}

// sentryAcquire is FUN_0009ade4: the nearest living zombie strictly inside the
// range circle.
func (p *playState) sentryAcquire(s *sentryState) (int, bool) {
	best, bestD2 := -1, s.rangePx*s.rangePx
	for index, z := range p.zombies {
		if z.health <= 0 || z.dying || z.spawnAway {
			continue
		}
		dx, dy := z.x-s.x, z.y-s.y
		if d2 := dx*dx + dy*dy; d2 < bestD2 {
			best, bestD2 = index, d2
		}
	}
	return best, best >= 0
}

func (p *playState) updateSentries() {
	const dt = 1.0 / 60.0
	p.sentryLoopHold = math.Max(0, p.sentryLoopHold-dt)
	p.ensureRNG()
	alive := len(p.livingPlayers()) > 0
	kept := p.sentries[:0]
	for _, s := range p.sentries {
		s.age += dt
		switch s.state {
		case sentryStateDeploy:
			s.lift -= dt * sentryDropSpeed
			if s.lift <= sentryRestLift {
				s.lift, s.state = sentryRestLift, sentryStateActive
			}
			kept = append(kept, s)
			continue
		case sentryStateDying:
			s.dying -= dt
			if s.dying <= 1e-9 {
				// FUN_0009acb8: a life-0 grenade owned by the sentry detonates here.
				p.spawnBlast(s.x, s.y, sentryBlastSound, killOrigin{gun: "SENTRY", sentry: s.id}, true)
				continue
			}
			s.shake = sentryDyingShake[s.shakeIdx]
			s.shakeIdx = (s.shakeIdx + 1) % len(sentryDyingShake)
			kept = append(kept, s)
			continue
		}
		p.updateActiveSentry(&s, alive)
		if s.state == sentryStateActive || s.state == sentryStateDying {
			kept = append(kept, s)
		}
	}
	p.sentries = kept
}

func (p *playState) updateActiveSentry(s *sentryState, alive bool) {
	const dt = 1.0 / 60.0
	// Target management (only while the player lives).
	if alive {
		if !s.hasTarget {
			if index, ok := p.sentryAcquire(s); ok {
				s.hasTarget, s.targetIdx = true, index
				s.targetX, s.targetY = p.zombies[index].x, p.zombies[index].y
			}
		} else if !p.sentryTargetValid(s) && !p.sentryRelocate(s) {
			s.hasTarget = false
		}
	} else {
		s.hasTarget = false
	}
	if !s.hasTarget {
		if s.angle == s.want {
			s.want = uint16(p.rng.Bounded(0xffff))
		}
	} else {
		z := p.zombies[s.targetIdx]
		s.targetX, s.targetY = z.x, z.y
		if math.Hypot(z.x-s.x, z.y-s.y) <= s.rangePx {
			s.want = weapons.NativeWeaponDirection(z.x-s.x, z.y-s.y)
		} else {
			s.hasTarget = false
		}
	}
	// Turn 10% of the way, truncated toward zero.
	step := int(float32(angleDifference(s.want, s.angle)) * sentryTurnRate)
	s.angle = uint16(int(s.angle) + step)
	s.column = sentryColumn(s.angle)
	s.flipX = s.column > 8
	// The gun's update(dt, hasTarget): the uzi pins its timer at rate*.5 while idle.
	if s.uziIdleHold && !s.hasTarget {
		s.cooldown = float64(float32(s.weapon.RateOfFire) * sentryUziHold)
	} else {
		s.cooldown += dt
	}
	if s.cooldown >= s.weapon.RateOfFire && s.hasTarget && s.column == sentryColumn(s.want) {
		p.sentryFire(s)
	}
	if s.ammo < 1 {
		s.state, s.dying, s.shakeIdx = sentryStateDying, sentryDyingTime, 0
		p.sfxQueue = append(p.sfxQueue, sentryDeathSound)
	}
}

// sentryMuzzle is the muzzle position for the current column: the sentry's
// position, the head lift and the same per-column offset table the player's
// weapons use (v7 0x005da460).
func (s *sentryState) muzzle() (float64, float64) {
	offset := barryMuzzleOffsets[s.column]
	return s.x + offset.x, s.y + s.lift + offset.y
}

func (p *playState) sentryFire(s *sentryState) {
	s.ammo--
	s.cooldown = 0
	muzzleX, muzzleY := s.muzzle()
	dx, dy := s.targetX-muzzleX, s.targetY-muzzleY
	distance := math.Hypot(dx, dy)
	if distance < .0001 {
		return
	}
	dirX, dirY := dx/distance, dy/distance
	weapon := s.weapon
	origin := killOrigin{gun: "SENTRY", sentry: s.id}
	if weapon.GunType == "BAZOOKA" {
		// The rocket starts at age Life*.5 (FUN_000a5900), so it flies Life*.5.
		p.bullets = append(p.bullets, bullet{x: muzzleX, y: muzzleY, vx: dirX * weapon.Speed, vy: dirY * weapon.Speed, life: weapon.Life * .5, angle: math.Atan2(dirY, dirX) + math.Pi/2, kind: "rocket", origin: origin})
		p.sfxQueue = append(p.sfxQueue, weapon.SFXShoot)
		return
	}
	weapon.Ammo = 1 << 20
	volley, err := weapons.NativePrimaryVolley(weapon, weapons.NativeWeaponDirection(dirX, dirY), p.rng)
	if err != nil {
		return
	}
	for _, shot := range volley.Shots {
		projectile, ok := weapons.NewNativeWeaponProjectile(shot, weapon.BulletType, muzzleX, muzzleY, p.rng)
		if !ok {
			return
		}
		p.bullets = append(p.bullets, bullet{x: muzzleX, y: muzzleY, vx: projectile.VX, vy: projectile.VY, life: projectile.Life, projectile: &projectile, origin: origin})
	}
	if weapon.SFXShoot != "" && weapon.SFXShoot != "0" {
		p.sfxQueue = append(p.sfxQueue, weapon.SFXShoot)
	} else if weapon.SFXStart != "" && weapon.SFXStart != "0" {
		p.sfxQueue = append(p.sfxQueue, weapons.NativeWeaponSoundRange(weapon.SFXStart, weapon.SFXEnd, p.rng.Bounded(0)))
	}
}

func (a *app) drawSentries(screen *ebiten.Image) {
	for _, sentry := range a.play.sentries {
		a.drawSentry(screen, sentry)
	}
}

// sentryCell maps a native column to the head-sheet cell and mirroring: columns
// 0..8 are cells 0..8, columns 9..15 are cells 7..1 mirrored (FUN_0009b320).
func sentryCell(column int) (int, bool) {
	if column <= 8 {
		return column, false
	}
	return 16 - column, true
}

// drawSentry draws the stand (40x40 at the actor) and then the head (68x40,
// lifted by the deploy/rest lift, shaken while dying), as FUN_0009b320 does. The
// native draw submits no shadow.
func (a *app) drawSentry(screen *ebiten.Image, sentry sentryState) {
	stand, standErr := a.Texture("Common2/Textures/Turret_Stand_SD")
	gun, gunErr := a.Texture("Common2/Textures/Turret_Gun_SD")
	if standErr != nil || gunErr != nil || a.play == nil || a.play.world == nil {
		return
	}
	zoom := a.play.world.Zoom
	x := (sentry.x-a.play.world.CameraX)*zoom + a.play.world.ViewportX
	y := (sentry.y-a.play.world.CameraY)*zoom + a.play.world.ViewportY
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(stand.Bounds().Dx())/2, -float64(stand.Bounds().Dy())/2)
	options.GeoM.Scale(zoom*sentryStandSize/float64(stand.Bounds().Dx()), zoom*sentryStandSize/float64(stand.Bounds().Dy()))
	options.GeoM.Translate(x, y)
	a.drawImage(screen, stand, options)

	cellW := int(math.Round(float64(gun.Bounds().Dx()) * sentryCellPitch))
	cellIndex, mirrored := sentryCell(sentry.column)
	cell := image.Rect(cellIndex*cellW, 0, (cellIndex+1)*cellW, gun.Bounds().Dy())
	options = &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(cell.Dx())/2, -float64(cell.Dy())/2)
	flip := 1.0
	if mirrored {
		flip = -1
	}
	options.GeoM.Scale(flip*zoom*sentryHeadWidth/float64(cell.Dx()), zoom*sentryHeadHeight/float64(cell.Dy()))
	options.GeoM.Translate(x+sentry.shake*zoom, y+sentry.lift*zoom)
	a.drawImage(screen, gun.SubImage(cell).(*ebiten.Image), options)
}
