package game

import (
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
)

// drawDepthSorted draws the entities of the current depth band in the native order: ascending y-bucket of the raw
// position y (Research/native/prop-entity-draw-order-2026-10-12.md), ties in entity-list order. The list order is
// PORT-chosen where the note leaves it open (per-type creation order is unresolved): Barry, script entities and
// sentries, zombies with their dead bodies and blood pops, then other players. barry draws Barry (nil when he is not
// in this band); lastBand puts the thrown objects and bullets next to him (they are not mapped to a native class).
func (a *app) drawDepthSorted(target *ebiten.Image, zombies []zombieState, barry func(*ebiten.Image)) {
	p := a.play
	type item struct {
		bucket int
		order  int // list order class at equal bucket
		zombie *zombieState
		entity *scriptEntity
		player *coopPlayer
		sentry *sentryState
		pop    *bloodPop
		barry  bool
	}
	var items []item
	if barry != nil {
		items = append(items, item{bucket: a.depthBucket(p.y), order: 0, barry: true})
	}
	for _, entity := range p.scriptEntities {
		if entity != nil && entity.kind != "zombie" && a.inDepthBand(entity.y) {
			items = append(items, item{bucket: a.depthBucket(entity.y), order: 1, entity: entity})
		}
	}
	for index := range p.sentries {
		if a.inDepthBand(p.sentries[index].y) {
			items = append(items, item{bucket: a.depthBucket(p.sentries[index].y), order: 1, sentry: &p.sentries[index]})
		}
	}
	for index := range zombies {
		if a.inDepthBand(zombies[index].y) {
			items = append(items, item{bucket: a.depthBucket(zombies[index].y), order: 2, zombie: &zombies[index]})
		}
	}
	for index := range p.gibBodies {
		if a.inDepthBand(p.gibBodies[index].y) {
			items = append(items, item{bucket: a.depthBucket(p.gibBodies[index].y), order: 2, zombie: &p.gibBodies[index]})
		}
	}
	for index := range p.bloodPops {
		if a.inDepthBand(p.bloodPops[index].y) {
			items = append(items, item{bucket: a.depthBucket(p.bloodPops[index].y), order: 2, pop: &p.bloodPops[index]})
		}
	}
	if p.coopActive() {
		for _, c := range p.coop.players {
			if a.inDepthBand(c.body.y) {
				items = append(items, item{bucket: a.depthBucket(c.body.y), order: 3, player: c})
			}
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].bucket != items[j].bucket {
			return items[i].bucket < items[j].bucket
		}
		if items[i].order != items[j].order {
			return items[i].order < items[j].order
		}
		return entityID(items[i].entity) < entityID(items[j].entity)
	})
	lastBand := !a.depthBandActive || a.depthBand.Last
	beside := lastBand && barry != nil
	for _, it := range items {
		switch {
		case it.barry:
			if beside {
				a.drawThrown(target)
			}
			barry(target)
			if beside {
				a.drawBullets(target)
			}
		case it.zombie != nil:
			a.drawZombie(target, *it.zombie)
		case it.player != nil:
			a.drawCoopPlayer(target, it.player)
		case it.sentry != nil:
			a.drawSentry(target, *it.sentry)
		case it.pop != nil:
			a.drawBloodPop(target, *it.pop)
		default:
			a.drawScriptEntity(target, it.entity)
		}
	}
	if lastBand && !beside {
		a.drawThrown(target)
		a.drawBullets(target)
	}
}

func entityID(e *scriptEntity) int {
	if e == nil {
		return 0
	}
	return e.id
}
