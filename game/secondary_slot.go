package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// The secondary (grenade) slot holds one of: grenades, mines, bazooka rockets or
// a deployable sentry gun. Picking up another replaces the slot contents.
const (
	secondaryGrenade = "GRENADE"
	secondaryMine    = "MINE"
	secondaryBazooka = "BAZOOKA"
	secondarySentry  = "SENTRY"
	// DLC1 dynamite (COWPATBOMB): thrown like a grenade. Its native class (entity
	// 0x1d) was not decoded, so it uses the grenade blast.
	secondaryCowpat = "COWPATBOMB"
)

// mineFuse documents where the mine timer comes from: FUN_000a5e7c detonates a
// planted mine when its age reaches the weapon's XML Life (MINE: 1.5 s). There is
// no proximity trigger and no arming delay in the native class (the hit pass's
// handler FUN_000a4cac ignores a mine that is not exploding yet).
type mineState struct {
	x, y, age float64
	life      float64
	id        int
}

// secondaryForPickup maps a pickup name to the slot it fills and the weapon whose
// catalog entry selects the turret's gun for sentries.
func secondaryForPickup(name string) (slot, weapon string, ok bool) {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "P_GRENADE":
		return secondaryGrenade, "GRENADE", true
	case "P_MINE":
		return secondaryMine, "MINE", true
	case "P_BAZOOKA":
		return secondaryBazooka, "BAZOOKA", true
	case "P_COW_PAT":
		return secondaryCowpat, "COWPATBOMB", true
	case "P_SENTRY":
		// FUN_000f8dc8: the plain pickup carries the random marker 0x12; the
		// variant is rolled when the turret is placed.
		return secondarySentry, "SENTRY", true
	case "P_SENTRYUZI":
		return secondarySentry, "SENTRY_UZI", true
	case "P_SENTRYSHOTGUN":
		return secondarySentry, "SENTRY_SHOTGUN", true
	case "P_SENTRYFLAMER":
		return secondarySentry, "SENTRY_FLAMER", true
	case "P_SENTRYBAZOOKA":
		return secondarySentry, "SENTRY_BAZOOKA", true
	}
	return "", "", false
}

func (p *playState) fillSecondarySlot(slot, weaponName string) {
	weapon, ok := p.weapons.Find(slot)
	if !ok {
		return
	}
	p.secondaryType, p.secondaryWeapon, p.grenades = slot, weaponName, weapon.Ammo
	p.combo.secondarySerial++
}

func secondaryIconCell(slot string) int {
	switch slot {
	case secondaryMine:
		return 1
	case secondaryBazooka:
		return 2
	case secondarySentry:
		return 3
	}
	return 0
}

func (b bullet) explodes() bool { return b.kind == "grenade" || b.kind == "rocket" }

// explosionSound: every native explosion state starts through FUN_000a3ca8, which
// plays sound id 8, "mine_explode" (the rocket class vtable +0x54 is that same
// function). SFX_GRENADE_EXPLODE (id 12) belongs to the unused kind 0x1d
// (FUN_000a4050), and "rocket_explode" is never played by the v7 bazooka.
func (b bullet) explosionSound() string { return zombieBlastSound }

func (p *playState) fireSecondaryOther(dx, dy float64) bool {
	weapon, ok := p.weapons.Find(p.secondaryType)
	if !ok || p.grenades <= 0 || weapon.RateOfFire <= 0 {
		return false
	}
	dist := math.Hypot(dx, dy)
	if dist < .0001 {
		return false
	}
	dirX, dirY := dx/dist, dy/dist
	switch p.secondaryType {
	case secondaryMine:
		// FUN_000ada64: the mine appears at the muzzle position (speed 0) and
		// goes off when it is Life old.
		offsetX, offsetY, _, _ := muzzleTransform(dirX, dirY)
		p.mines = append(p.mines, mineState{x: p.x + offsetX, y: p.y + offsetY, life: weapon.Life, id: p.newShot()})
	case secondaryCowpat:
		offsetX, offsetY, _, _ := muzzleTransform(dirX, dirY)
		dirX, dirY = p.projectileDirection(dirX, dirY, offsetX, offsetY)
		p.throwBomb(p.x+offsetX, p.y+offsetY, dirX, dirY, weapon.Speed, weapon.Life, true, killOrigin{gun: "COWPAT", shot: p.newShot()})
	case secondaryBazooka:
		offsetX, offsetY, _, _ := muzzleTransform(dirX, dirY)
		dirX, dirY = p.projectileDirection(dirX, dirY, offsetX, offsetY)
		// FUN_000a5900 starts the rocket at age Life*.5, so it flies for Life*.5.
		p.bullets = append(p.bullets, bullet{x: p.x + offsetX, y: p.y + offsetY, vx: dirX * weapon.Speed, vy: dirY * weapon.Speed, life: weapon.Life * .5, angle: math.Atan2(dirY, dirX) + math.Pi/2, kind: "rocket", origin: killOrigin{gun: "ROCKET", shot: p.newShot()}})
	case secondarySentry:
		// 1.2.5 FUN_0010e0f8: the sentry weapon creates the turret at the
		// weapon's own position (the player) at once; the v7 SENTRY_SPAWN lob
		// (Speed 250, Life .4) is not part of this fire path.
		variant := p.secondaryWeapon
		if variant == secondarySentry {
			variant = p.randomSentryVariant()
		}
		if !p.deploySentry(p.x, p.y, variant) {
			return false
		}
		if p.statistics != nil {
			p.statistics.SentryGunsUsed++
			if p.statistics.Available == nil {
				p.statistics.Available = map[string]bool{}
			}
			p.statistics.Available["Sentry Guns Used"] = true
		}
	default:
		return false
	}
	p.achieve.nonPistol = true
	p.grenades--
	if p.secondaryType != secondarySentry {
		p.shotSound = weapon.SFXShoot
	}
	p.secondaryShootCooldown = weapon.RateOfFire
	return true
}

func (p *playState) updateMines() {
	const dt = 1.0 / 60.0
	remaining := p.mines[:0]
	for _, mine := range p.mines {
		mine.age += dt
		if mine.life > 0 && mine.age >= mine.life-1e-9 {
			p.spawnBlast(mine.x, mine.y, zombieBlastSound, killOrigin{gun: "MINE", shot: mine.id}, true)
			continue
		}
		remaining = append(remaining, mine)
	}
	p.mines = remaining
}

func (p *playState) nearestZombie(x, y, within float64) (int, bool) {
	best, bestDistance := -1, within
	for index, zombie := range p.zombies {
		if zombie.health <= 0 || zombie.dying || zombie.spawnAway {
			continue
		}
		if distance := math.Hypot(zombie.x-x, zombie.y-y); distance <= bestDistance {
			best, bestDistance = index, distance
		}
	}
	return best, best >= 0
}

func (a *app) drawMines(screen *ebiten.Image) {
	if len(a.play.mines) == 0 {
		return
	}
	texture, err := a.Texture("Common0/Textures/mine_SD")
	if err != nil {
		return
	}
	zoom := a.play.world.Zoom
	const logicalSize = 40.0
	for _, mine := range a.play.mines {
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Translate(-float64(texture.Bounds().Dx())/2, -float64(texture.Bounds().Dy())/2)
		options.GeoM.Scale(zoom*logicalSize/float64(texture.Bounds().Dx()), zoom*logicalSize/float64(texture.Bounds().Dy()))
		options.GeoM.Translate((mine.x-a.play.world.CameraX)*zoom+a.play.world.ViewportX, (mine.y-a.play.world.CameraY)*zoom+a.play.world.ViewportY)
		a.drawImage(screen, texture, options)
	}
}

// flushSfxQueue plays queued non-player sounds (sentry spawn/death/fire). Looping
// weapon sounds are started once and held while the sentry keeps firing.
func (a *app) flushSfxQueue(p *playState) {
	for _, symbol := range p.sfxQueue {
		if symbol == "" || symbol == "0" {
			continue
		}
		if weapons.NativeWeaponSoundLoopSamples(symbol) >= 0 || symbol == "SFX_FLAMETHROWER" {
			p.sentryLoopHold = .12
			if a.weaponPlayback.player == nil {
				a.playWeaponSound(symbol)
			}
			continue
		}
		a.playSound(a.scriptSoundPath(symbol), .8)
	}
	p.sfxQueue = nil
}
