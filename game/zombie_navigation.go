package game

import "math"

// navField is a breadth-first distance field over walkable collision tiles,
// measured from the tile the player stands on. Zombies whose straight line to
// the player is blocked follow it downhill around walls.
type navField struct {
	width, height int
	dist          []int32
	goalX, goalY  int
	age           int
}

const navRefreshFrames = 20

func (p *playState) tileWalkable(tileX, tileY int) bool {
	return !playerCollisionBlocks(p.collisionValue(tileX, tileY))
}

// refreshNav keeps one path field per living player.
func (p *playState) refreshNav() {
	if p.world == nil || p.tileSize <= 0 {
		return
	}
	targets := p.livingPlayers()
	if len(targets) == 0 {
		targets = []playerTarget{{index: 0, x: p.x, y: p.y}}
	}
	for _, target := range targets {
		p.refreshNavField(&p.navs[target.index], target.x, target.y)
	}
}

func (p *playState) refreshNavField(nav *navField, playerX, playerY float64) {
	level := p.world.Level
	goalX, goalY := int(playerX)/p.tileSize, int(playerY)/p.tileSize
	if nav.dist != nil && nav.width == level.Width && nav.height == level.Height && nav.goalX == goalX && nav.goalY == goalY && nav.age < navRefreshFrames {
		nav.age++
		return
	}
	if len(nav.dist) != level.Width*level.Height {
		nav.dist = make([]int32, level.Width*level.Height)
	}
	nav.width, nav.height, nav.goalX, nav.goalY, nav.age = level.Width, level.Height, goalX, goalY, 0
	for index := range nav.dist {
		nav.dist[index] = -1
	}
	if goalX < 0 || goalY < 0 || goalX >= level.Width || goalY >= level.Height {
		return
	}
	queue := make([]int, 0, 256)
	nav.dist[goalY*level.Width+goalX] = 0
	queue = append(queue, goalY*level.Width+goalX)
	for head := 0; head < len(queue); head++ {
		current := queue[head]
		cx, cy := current%level.Width, current/level.Width
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				nx, ny := cx+dx, cy+dy
				if nx < 0 || ny < 0 || nx >= level.Width || ny >= level.Height || !p.tileWalkable(nx, ny) {
					continue
				}
				if dx != 0 && dy != 0 && (!p.tileWalkable(cx+dx, cy) || !p.tileWalkable(cx, cy+dy)) {
					continue // no cutting wall corners
				}
				next := ny*level.Width + nx
				if nav.dist[next] >= 0 {
					continue
				}
				nav.dist[next] = nav.dist[current] + 1
				queue = append(queue, next)
			}
		}
	}
}

// navDirection returns a unit vector toward the next tile on the shortest walkable
// route to the player, or false when there is no route (or none is needed).
func (p *playState) navDirection(x, y float64, player int) (float64, float64, bool) {
	nav := &p.navs[player]
	if nav.dist == nil || p.tileSize <= 0 {
		return 0, 0, false
	}
	tileX, tileY := int(x)/p.tileSize, int(y)/p.tileSize
	if tileX < 0 || tileY < 0 || tileX >= nav.width || tileY >= nav.height {
		return 0, 0, false
	}
	here := nav.dist[tileY*nav.width+tileX]
	if here <= 0 {
		return 0, 0, false
	}
	bestX, bestY, best := 0, 0, here
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			nx, ny := tileX+dx, tileY+dy
			if nx < 0 || ny < 0 || nx >= nav.width || ny >= nav.height {
				continue
			}
			if dx != 0 && dy != 0 && (!p.tileWalkable(tileX+dx, tileY) || !p.tileWalkable(tileX, tileY+dy)) {
				continue
			}
			if value := nav.dist[ny*nav.width+nx]; value >= 0 && value < best {
				bestX, bestY, best = nx, ny, value
			}
		}
	}
	if best == here {
		return 0, 0, false
	}
	targetX := (float64(bestX) + .5) * float64(p.tileSize)
	targetY := (float64(bestY) + .5) * float64(p.tileSize)
	dx, dy := targetX-x, targetY-y
	length := math.Hypot(dx, dy)
	if length < .0001 {
		return 0, 0, false
	}
	return dx / length, dy / length, true
}

// lineClear reports whether a body of the given radius can walk straight from
// one point to the other without touching blocked tiles.
func (p *playState) lineClear(x0, y0, x1, y1, radius float64) bool {
	distance := math.Hypot(x1-x0, y1-y0)
	steps := int(distance/8) + 1
	for step := 1; step <= steps; step++ {
		t := float64(step) / float64(steps)
		if _, _, hit := p.collisionDisplacement(x0+(x1-x0)*t, y0+(y1-y0)*t, radius, p.tileSize); hit {
			return false
		}
	}
	return true
}

func (p *playState) resolveZombieCollision(x, y, radius float64) (float64, float64) {
	for resolve := 0; resolve < 4; resolve++ {
		pushX, pushY, hit := p.collisionDisplacement(x, y, radius, p.tileSize)
		if !hit {
			break
		}
		x += pushX
		y += pushY
	}
	return x, y
}

// moveZombie advances a zombie along (dirX, dirY) by step, sliding along walls
// instead of being pushed back when the full move is blocked.
func (p *playState) moveZombie(zombie *zombieState, dirX, dirY, step, radius float64) {
	progress := func(x, y float64) float64 { return (x-zombie.x)*dirX + (y-zombie.y)*dirY }
	fullX, fullY := p.resolveZombieCollision(zombie.x+dirX*step, zombie.y+dirY*step, radius)
	if progress(fullX, fullY) >= step*.7 {
		zombie.x, zombie.y = fullX, fullY
		return
	}
	bestX, bestY, best := zombie.x, zombie.y, 0.0
	for _, candidate := range [][2]float64{{fullX, fullY}, {zombie.x + dirX*step, zombie.y}, {zombie.x, zombie.y + dirY*step}} {
		x, y := p.resolveZombieCollision(candidate[0], candidate[1], radius)
		if gain := progress(x, y); gain > best {
			bestX, bestY, best = x, y, gain
		}
	}
	zombie.x, zombie.y = bestX, bestY
}
