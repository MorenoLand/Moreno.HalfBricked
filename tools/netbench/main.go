//go:build js && wasm

// netbench times the online snapshot codec inside WebAssembly (run it under Node like tools/webimport): the guest
// decodes a snapshot 30 times a second on the browser's main thread, so this is the cost that matters.
package main

import (
	"fmt"
	"time"

	"github.com/MorenoLand/Moreno.HalfBricked/game/netwire"
)

func busy() netwire.Snapshot {
	s := netwire.Snapshot{Seq: 1, Time: 123.456, Score: 18400, Lives: 3, Multiplier: 4, Kills: 120, Strings: []string{"zombie_a", "run", "zombie_b", "walk"}}
	s.Players = []netwire.Player{{X: 100.25, Y: 200.5, Health: .75, MaxHealth: 1, Gun: "SHOTGUN", Ammo: 12, Joined: true}, {X: 140.5, Y: 210.25, Health: 1, MaxHealth: 1, Gun: "PISTOL", Joined: true}}
	for i := 0; i < 60; i++ {
		s.Zombies = append(s.Zombies, netwire.Zombie{X: 100.5 + float64(i)*7.25, Y: 80.75 + float64(i)*5.5, Frame: float64(i % 4), Alpha: 1, Health: 55.5, Fps: 8, W: 48, H: 48, Tex: i % 4, Anim: (i + 1) % 4, Angle: i % 8, FlipX: i%2 == 0, Brightness: 1})
	}
	for i := 0; i < 40; i++ {
		s.Bullets = append(s.Bullets, netwire.Bullet{X: 150.5 + float64(i), Y: 160.25, VX: 6.5, VY: -2.25, Life: .8, Angle: 1.5, Kind: "bullet"})
	}
	for i := 0; i < 120; i++ {
		s.Particles = append(s.Particles, netwire.Particle{X: 200.5 + float64(i), Y: 100.25, Z: 4.5, Size: 12.5, Age: .35, Kind: i % 5})
	}
	for i := 0; i < 20; i++ {
		s.Blood = append(s.Blood, netwire.Blood{X: 90.5 + float64(i), Y: 70.25, Age: .5, Variant: i % 3})
	}
	return s
}

func main() {
	s := busy()
	raw := netwire.Encode(netwire.Msg{T: "snap", Snap: &s})
	const rounds = 100
	start := time.Now()
	for i := 0; i < rounds; i++ {
		netwire.Encode(netwire.Msg{T: "snap", Snap: &s})
	}
	enc := time.Since(start)
	start = time.Now()
	for i := 0; i < rounds; i++ {
		netwire.Decode(raw)
	}
	dec := time.Since(start)
	fmt.Printf("snapshot %d bytes; encode %.2f ms, decode %.2f ms each (in WebAssembly)\n", len(raw), float64(enc.Microseconds())/rounds/1000, float64(dec.Microseconds())/rounds/1000)
}
