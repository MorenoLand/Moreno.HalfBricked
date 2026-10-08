package game

import (
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
)

func wallTestPlay(t *testing.T, wall func(x, y int) bool) *playState {
	t.Helper()
	const width, height = 16, 9
	level := formats.Level{Width: width, Height: height, Layers: map[formats.LayerKind][]uint32{}}
	for _, kind := range formats.LayerKinds {
		cells := make([]uint32, width*height)
		for index := range cells {
			cells[index] = ^uint32(0)
		}
		level.Layers[kind] = cells
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if wall(x, y) {
				level.Layers[formats.LayerC][y*width+x] = 0 // collision value 1
			}
		}
	}
	return &playState{world: viewer.New(level, formats.TileSet{}, nil, nil), tileSize: 32, health: 1, maxHealth: 1}
}

func TestZombiesWalkAroundWallsToReachThePlayer(t *testing.T) {
	// A solid wall column with a gap at the bottom separates zombie and player.
	p := wallTestPlay(t, func(x, y int) bool { return x == 8 && y < 7 })
	p.x, p.y = 13*32+16, 3*32+16
	p.zombies = []zombieState{{x: 3*32 + 16, y: 3*32 + 16, speed: 90, health: 100, size: formats.Vec2{X: 48, Y: 48}, collision: 15}}
	for frame := 0; frame < 60*14; frame++ {
		p.updateZombies()
	}
	zombie := p.zombies[0]
	if distance := math.Hypot(zombie.x-p.x, zombie.y-p.y); distance > 50 {
		t.Fatalf("zombie stuck behind the wall at %.0f,%.0f (distance %.0f)", zombie.x, zombie.y, distance)
	}
}

func TestZombiesSlideAlongWallsInsteadOfStalling(t *testing.T) {
	// The zombie pushes diagonally into a horizontal wall; it must keep sliding.
	p := wallTestPlay(t, func(x, y int) bool { return y == 4 && x < 15 })
	p.x, p.y = 3*32+16, 6*32+16
	p.zombies = []zombieState{{x: 2*32 + 16, y: 3*32 + 16, speed: 90, health: 100, size: formats.Vec2{X: 48, Y: 48}, collision: 15}}
	for frame := 0; frame < 60*12; frame++ {
		p.updateZombies()
	}
	zombie := p.zombies[0]
	if distance := math.Hypot(zombie.x-p.x, zombie.y-p.y); distance > 50 {
		t.Fatalf("zombie stalled at %.0f,%.0f (distance %.0f)", zombie.x, zombie.y, distance)
	}
}

func TestSpawnCollisionRadiusIsIndependentOfRenderSize(t *testing.T) {
	declared := formats.Vec2{X: 29, Y: 31}
	if nativeSpawnCollisionRadius(declared) >= nativeSpawnRenderSize(declared)/2 {
		t.Fatal("collision body must be smaller than the rendered sprite")
	}
	zombie := zombieState{size: formats.Vec2{X: 48, Y: 48}, collision: 15.5}
	if zombieCollisionRadius(zombie) != 15.5 {
		t.Fatalf("radius %v", zombieCollisionRadius(zombie))
	}
}
