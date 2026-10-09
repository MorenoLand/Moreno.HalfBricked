package game

import (
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
