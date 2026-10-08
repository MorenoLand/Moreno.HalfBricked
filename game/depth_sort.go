package game

import (
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
)

// drawDepthSorted draws zombies, other players and script entities (crates, NPCs, props) in one
// back-to-front pass, so a crate lies under a zombie standing below it and over
// one standing above it. behind picks the half in front of or behind the player.
func (a *app) drawDepthSorted(target *ebiten.Image, zombies []zombieState, behind bool) {
	p := a.play
	type item struct {
		y      float64
		order  int // entities before zombies at the same depth
		zombie *zombieState
		entity *scriptEntity
		player *coopPlayer
		sentry *sentryState
	}
	var items []item
	for index := range zombies {
		if (zombies[index].y <= p.y) == behind {
			items = append(items, item{y: zombies[index].y, order: 1, zombie: &zombies[index]})
		}
	}
	if p.coopActive() {
		for _, c := range p.coop.players {
			if (c.body.y <= p.y) == behind {
				items = append(items, item{y: c.body.y, order: 2, player: c})
			}
		}
	}
	for _, entity := range p.scriptEntities {
		if entity != nil && entity.kind != "zombie" && (entity.y <= p.y) == behind {
			items = append(items, item{y: entity.y, order: 0, entity: entity})
		}
	}
	for index := range p.sentries {
		if (p.sentries[index].y <= p.y) == behind {
			items = append(items, item{y: p.sentries[index].y, order: 0, sentry: &p.sentries[index]})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].y != items[j].y {
			return items[i].y < items[j].y
		}
		if items[i].order != items[j].order {
			return items[i].order < items[j].order
		}
		return entityID(items[i].entity) < entityID(items[j].entity)
	})
	for _, it := range items {
		switch {
		case it.zombie != nil:
			a.drawZombie(target, *it.zombie)
		case it.player != nil:
			a.drawCoopPlayer(target, it.player)
		case it.sentry != nil:
			a.drawSentry(target, *it.sentry)
		default:
			a.drawScriptEntity(target, it.entity)
		}
	}
}

func entityID(e *scriptEntity) int {
	if e == nil {
		return 0
	}
	return e.id
}
