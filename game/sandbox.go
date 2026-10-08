package game

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The F1 sandbox menu: toggles and one-shot actions over the running level.
// It is a port addition (the original has nothing like it). The game holds
// still while it is open.

type cheatFlags struct {
	god, freezeWaves, freezeZombies, infiniteAmmo bool
}

type sandboxItem struct {
	label string
	on    func(p *playState) bool  // toggle state; nil for one-shot actions
	do    func(a *app, shift bool) // click (shift = bulk / instant variant)
	hint  string
}

var sandboxTabs = [...]string{"GENERAL", "ZOMBIES", "PICKUPS"}

// sandboxPickups are the crates and weapons the sandbox can drop or hand over.
var sandboxPickups = []string{"p_health", "p_shotgun", "p_uzi", "p_flamer", "p_sniper", "p_minigun", "p_buzzsaw", "p_dual_pistol", "p_grenade", "p_mine", "p_sentry", "p_bazooka", "p_cow_pat"}

// spawnSpot returns a free-ish point in front of the player; repeated calls fan
// the spawns out so crates and zombies do not stack.
func (p *playState) spawnSpot(ordinal int) formats.Vec2 {
	dx, dy := barryAimDirection(p.angle, p.flipX)
	if dx == 0 && dy == 0 {
		dx = 1
	}
	base := math.Atan2(dy, dx) + float64(ordinal%7-3)*.35
	distance := 80.0 + float64(ordinal/7%3)*28
	x, y := p.x+math.Cos(base)*distance, p.y+math.Sin(base)*distance
	p.resolveBody(&x, &y)
	return formats.Vec2{X: x, Y: y}
}

// sandboxZombieTypes gathers every zombie type the game's levels define, so the
// menu can spawn them with their real sprites, speeds and health.
func (a *app) sandboxZombieTypes() []formats.SpawnType {
	if a.sandboxTypes != nil {
		return a.sandboxTypes
	}
	found := map[string]formats.SpawnType{} // one entry per type and skin
	add := func(waves []formats.Wave) {
		for _, wave := range waves {
			for _, spawner := range wave.Spawners {
				for _, entry := range spawner.Types {
					if strings.HasPrefix(entry.Name, "p_") || isBossType(entry.Name) || entry.Name == "train" {
						continue
					}
					key := entry.Name + "|" + strings.ToLower(entry.Texture)
					if _, ok := found[key]; !ok {
						found[key] = entry
					}
				}
			}
		}
	}
	if a.play != nil && a.play.world != nil {
		add(a.play.world.Level.Waves)
	}
	for _, info := range a.pack.List() {
		if level, err := a.pack.Load(info.ID); err == nil {
			add(level.Waves)
		}
	}
	list := make([]formats.SpawnType, 0, len(found))
	for _, entry := range found {
		list = append(list, entry)
	}
	sort.Slice(list, func(i, j int) bool {
		if nativeZombieTypes[list[i].Name] != nativeZombieTypes[list[j].Name] {
			return nativeZombieTypes[list[i].Name] < nativeZombieTypes[list[j].Name]
		}
		if list[i].Name != list[j].Name {
			return list[i].Name < list[j].Name
		}
		return list[i].Texture < list[j].Texture
	})
	a.sandboxTypes = list
	return list
}

func (a *app) sandboxItems() []sandboxItem {
	p := a.play
	switch a.sandboxTab {
	case 1:
		var items []sandboxItem
		for _, entry := range a.sandboxZombieTypes() {
			entry := entry
			label := strings.ReplaceAll(strings.ToUpper(entry.Name), "_", " ")
			if entry.Texture != "" {
				label += " - " + strings.ToUpper(strings.TrimPrefix(entry.Texture, "characters/"))
			}
			items = append(items, sandboxItem{label: label, hint: "click: 1   shift: 5", do: func(a *app, shift bool) {
				count := 1
				if shift {
					count = 5
				}
				for i := 0; i < count; i++ {
					a.play.spawnZombieAt(entry, a.play.spawnSpot(a.sandboxSpawned))
					a.sandboxSpawned++
				}
			}})
		}
		return items
	case 2:
		var items []sandboxItem
		for _, name := range sandboxPickups {
			name := name
			label := strings.ReplaceAll(strings.ToUpper(strings.TrimPrefix(name, "p_")), "_", " ")
			items = append(items, sandboxItem{label: label, hint: "click: drop a crate   shift: take it now", do: func(a *app, shift bool) {
				if shift {
					a.play.collectPickup(name)
					return
				}
				a.play.spawnPickup(name, a.play.spawnSpot(a.sandboxSpawned))
				a.sandboxSpawned++
			}})
		}
		return items
	}
	toggle := func(label string, flag func(c *cheatFlags) *bool) sandboxItem {
		return sandboxItem{label: label, on: func(p *playState) bool { return *flag(&p.cheats) }, do: func(a *app, _ bool) {
			value := flag(&a.play.cheats)
			*value = !*value
		}}
	}
	action := func(label string, do func(p *playState)) sandboxItem {
		return sandboxItem{label: label, do: func(a *app, _ bool) { do(a.play) }}
	}
	_ = p
	return []sandboxItem{
		toggle("GOD MODE", func(c *cheatFlags) *bool { return &c.god }),
		toggle("FREEZE WAVES", func(c *cheatFlags) *bool { return &c.freezeWaves }),
		toggle("FREEZE ZOMBIES", func(c *cheatFlags) *bool { return &c.freezeZombies }),
		toggle("INFINITE AMMO", func(c *cheatFlags) *bool { return &c.infiniteAmmo }),
		action("FULL HEALTH", func(p *playState) { p.healEveryone() }),
		action("+1 LIFE", func(p *playState) { p.lives++ }),
		action("+1000 SCORE", func(p *playState) { p.score += 1000 }),
		action("KILL ALL ZOMBIES", func(p *playState) {
			for i := range p.zombies {
				if z := &p.zombies[i]; !z.dying && !z.spawnAway && z.health > 0 {
					z.health = -1
				}
			}
		}),
		action("REMOVE ALL ZOMBIES", func(p *playState) {
			for i := range p.zombies {
				p.zombies[i].health, p.zombies[i].spawnAway = 0, true
			}
		}),
		action("CLOSE PORTALS", func(p *playState) { p.portals = nil }),
		action("NEXT WAVE", func(p *playState) { p.skipWave() }),
		action("CLEAR CRATES", func(p *playState) {
			for id, e := range p.scriptEntities {
				if e != nil && e.kind == "pickup" {
					delete(p.scriptEntities, id)
				}
			}
		}),
		action("CLEAR MINES+SENTRIES", func(p *playState) { p.mines, p.sentries = nil, nil }),
		action("RESTOCK AMMO", func(p *playState) { p.restockAmmo() }),
	}
}

func (p *playState) healEveryone() {
	p.health = p.maxHealth
	if p.coopActive() {
		for _, c := range p.coop.players {
			if c.joined {
				c.body.health, c.dead = c.body.maxHealth, false
			}
		}
	}
}

func (p *playState) skipWave() {
	if p.world == nil || p.waveIndex+1 >= len(p.world.Level.Waves) {
		return
	}
	p.waveIndex++
	p.waveElapsed, p.waveSpawned = 0, nil
}

func (p *playState) restockAmmo() {
	if gun, ok := p.weapons.Find(p.weapon.GunType); ok && p.weapon.Ammo > 0 {
		p.weapon.Ammo = gun.Ammo
	}
	if p.grenades > 0 {
		if gun, ok := p.weapons.Find("GRENADE"); ok {
			p.grenades = max(p.grenades, gun.Ammo)
		}
	}
}

// Layout: three tabs across the top, then buttons in two columns.
const (
	sandboxLeft    = 30
	sandboxTop     = 44
	sandboxColumn  = 205
	sandboxRowH    = 19
	sandboxRows    = 12
	sandboxButtonW = 198
)

func sandboxTabRect(index int) image.Rectangle {
	left := sandboxLeft + index*(sandboxButtonW/2+12)
	return image.Rect(left, 22, left+sandboxButtonW/2+4, 40)
}

// sandboxVisible is how many items fit on screen; longer lists scroll with the wheel.
const sandboxVisible = sandboxRows * 2

func sandboxItemRect(index int) image.Rectangle {
	column, row := index/sandboxRows, index%sandboxRows
	left := sandboxLeft + column*sandboxColumn
	top := sandboxTop + row*sandboxRowH
	return image.Rect(left, top, left+sandboxButtonW, top+sandboxRowH-2)
}

// updateSandbox runs the F1 menu; it returns true while the menu owns the frame.
func (a *app) updateSandbox() bool {
	if a.play == nil {
		a.sandboxOpen = false
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		a.sandboxOpen = !a.sandboxOpen
		return a.sandboxOpen
	}
	if !a.sandboxOpen {
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		a.sandboxOpen = false
		return true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		a.sandboxTab, a.sandboxScroll = (a.sandboxTab+1)%len(sandboxTabs), 0
	}
	if _, wheel := ebiten.Wheel(); wheel != 0 {
		a.sandboxScroll = max(0, min(a.sandboxScroll-int(math.Copysign(math.Ceil(math.Abs(wheel)), wheel))*4, max(0, len(a.sandboxItems())-sandboxVisible)))
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return true
	}
	x, y := a.pointer()
	point := image.Pt(x, y)
	for index := range sandboxTabs {
		if point.In(sandboxTabRect(index)) {
			a.sandboxTab, a.sandboxScroll = index, 0
			return true
		}
	}
	shift := ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)
	for index, item := range a.sandboxItems() {
		if slot := index - a.sandboxScroll; slot >= 0 && slot < sandboxVisible && point.In(sandboxItemRect(slot)) {
			item.do(a, shift)
			a.playSound("audio/sound/sfx/menu_select.ogg", .6)
			return true
		}
	}
	return true
}

func (a *app) drawSandbox(screen *ebiten.Image) {
	if !a.sandboxOpen || a.play == nil {
		return
	}
	a.drawRect(screen, 0, 0, logicalWidth, logicalHeight, color.RGBA{0, 0, 0, 175})
	pointerX, pointerY := a.pointer()
	pointer := image.Pt(pointerX, pointerY)
	button := func(rect image.Rectangle, label string, lit, hover bool) {
		fill := color.RGBA{34, 38, 44, 255}
		if lit {
			fill = color.RGBA{36, 92, 52, 255}
		}
		a.drawRect(screen, float64(rect.Min.X), float64(rect.Min.Y), float64(rect.Dx()), float64(rect.Dy()), color.RGBA{120, 130, 140, 255})
		a.drawRect(screen, float64(rect.Min.X+1), float64(rect.Min.Y+1), float64(rect.Dx()-2), float64(rect.Dy()-2), fill)
		if hover {
			a.drawRect(screen, float64(rect.Min.X+1), float64(rect.Min.Y+1), float64(rect.Dx()-2), float64(rect.Dy()-2), color.RGBA{40, 40, 40, 40})
		}
		a.text(screen, label, float64(rect.Min.X+6), float64(rect.Min.Y+4), .3)
	}
	for index, name := range sandboxTabs {
		rect := sandboxTabRect(index)
		button(rect, name, index == a.sandboxTab, pointer.In(rect))
	}
	a.text(screen, "F1 closes", 380, 28, .33)
	hint := ""
	items := a.sandboxItems()
	if len(items) > sandboxVisible {
		a.text(screen, "mouse wheel scrolls", 30, 292, .3)
	}
	for index, item := range items {
		slot := index - a.sandboxScroll
		if slot < 0 || slot >= sandboxVisible {
			continue
		}
		rect := sandboxItemRect(slot)
		hover := pointer.In(rect)
		label := item.label
		lit := false
		if item.on != nil {
			lit = item.on(a.play)
			label = fmt.Sprintf("[%s] %s", map[bool]string{true: "x", false: " "}[lit], item.label)
		}
		button(rect, label, lit, hover)
		if hover {
			hint = item.hint
		}
	}
	if hint != "" {
		a.textCentered(screen, hint, 300, .32)
	}
}
