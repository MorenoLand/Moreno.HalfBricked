package game

import "github.com/hajimehoshi/ebiten/v2"

// Speedy zombie afterimages: the type-5 hook FUN_000a05a4 (vtable slot +0x34,
// runs every frame) keeps two ghost records of 0x70 bytes at +0x324 and +0x394.
// Every record's alpha byte (+0x363) drops by 6 per frame down to 0; while the
// zombie is alive and out of its spawn state a counter (+0x31c) counts down from
// 10 and, at 0, the next record (alternating) is overwritten with the zombie's
// current position, size, sprite state, heading and colour and alpha 0x80. A live
// record is drawn with FUN_0006ed18 behind the zombie.
const (
	zombieGhostInterval = 10
	zombieGhostAlpha    = 0x80
	zombieGhostFade     = 6
)

type zombieGhost struct {
	x, y, w, h, frame float64
	angle             int
	flipX             bool
	alpha             int
}

type zombieTrail struct {
	ghosts  [2]zombieGhost
	counter int
	next    int
}

// stepZombieTrail is FUN_000a05a8 for one frame of a speedy zombie.
func stepZombieTrail(z *zombieState) {
	t := &z.native.trail
	for i := range t.ghosts {
		if t.ghosts[i].alpha != 0 {
			t.ghosts[i].alpha = max(0, t.ghosts[i].alpha-zombieGhostFade)
		}
	}
	if !z.dying && z.native.ai.state != 0 {
		t.counter--
	}
	if t.counter < 1 {
		t.counter = zombieGhostInterval
		t.ghosts[t.next] = zombieGhost{x: z.x, y: z.y, w: z.size.X, h: z.size.Y, frame: z.frame, angle: z.angle, flipX: z.flipX, alpha: zombieGhostAlpha}
		t.next = (t.next + 1) % 2
	}
}

// drawZombieTrail draws the live ghosts of a speedy zombie before its body.
func (a *app) drawZombieTrail(screen *ebiten.Image, zombie zombieState) {
	if zombie.native.kind != zombieKindSpeedy {
		return
	}
	for _, g := range zombie.native.trail.ghosts {
		if g.alpha <= 0 {
			continue
		}
		ghost := zombie
		ghost.x, ghost.y, ghost.frame, ghost.angle, ghost.flipX = g.x, g.y, g.frame, g.angle, g.flipX
		ghost.size.X, ghost.size.Y = g.w, g.h
		ghost.alpha = float64(g.alpha) / 255
		ghost.hitFlash = 0
		ghost.native.trail = zombieTrail{}
		a.drawZombie(screen, ghost)
	}
}
