// Package netwire is the wire format of online co-op. The host runs the whole simulation and sends Snapshots of
// what the guests need to draw; guests send only their controls (Input). Everything here is plain data with
// exported fields (JSON encoded), so it has no dependency on the game package; the game converts its own state
// to and from these types in net_wire.go.
package netwire

import (
	"encoding/json"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

type Msg struct {
	T       string    `json:"t"`
	Level   string    `json:"level,omitempty"`
	Mode    int       `json:"mode,omitempty"`
	Self    int       `json:"self,omitempty"`
	Players int       `json:"players,omitempty"`
	In      *Input    `json:"in,omitempty"`
	Snap    *Snapshot `json:"snap,omitempty"`
	Res     *Results  `json:"res,omitempty"`
}

type Input struct {
	MoveX, MoveY, AimX, AimY float64
	Secondary                bool
	SecondaryCount           int // increments per throw so a dropped packet cannot lose one
}

type Player struct {
	X, Y                float64
	Angle               int
	FlipX, Moving, Dead bool
	Health, MaxHealth   float64
	Flash               float64
	Hurt                float64
	Age                 float64
	Gun                 string
	Ammo                int
	Grenades            int
	SecondaryType       string
	Joined              bool
	// Buzzsaw / dual pistol weapon object (GunVisual): spin or flash timer, blink
	// flag, hand flag and the offset of the hand that fired last.
	GunSpin, GunHandX, GunHandY float64
	GunBlink, GunRight          bool
}

type Zombie struct {
	X, Y, Frame, Alpha, HitFlash, DeathAge, Health, Fps, RexRage float64  `json:",omitempty"`
	Lift                                                         float64  `json:",omitempty"` // rex leap lift (rex_shockwave.go)
	W, H                                                         float64  `json:",omitempty"`
	Tex, Anim                                                    int      `json:",omitempty"` // indexes into the snapshot's string table
	Angle                                                        int      `json:",omitempty"`
	FlipX, FlipY, Dying, BossRage, AnimTime                      bool     `json:",omitempty"`
	Exploding                                                    bool     `json:",omitempty"`
	Brightness                                                   float64  `json:",omitempty"` // +0x2b8 tint (zombie_model.go)
	Speedy                                                       bool     `json:",omitempty"` // afterimage trail
	Ghosts                                                       [2]Ghost `json:",omitempty"`
	// Gib state (zombie_gib.go): native death state 1/2, the Disintegrate body flag and its age.
	DeathState int     `json:",omitempty"`
	Gibbed     bool    `json:",omitempty"`
	PresentAge float64 `json:",omitempty"`
	// Optional fields (absent from an older host, ignored by an older guest): the boss meter reads Boss and
	// MaxHealth; Robot is the future boss's drawn state.
	MaxHealth float64    `json:",omitempty"`
	Boss      bool       `json:",omitempty"`
	Robot     *RobotBody `json:",omitempty"`
}

// RobotBody is the part of the future boss (robot_boss.go) a guest needs to draw it: the sprite lift factor and
// the size its lift scales with, the falling body's draw offset, the orbs' colour and their positions.
type RobotBody struct {
	SizeZ, Lift, ShakeX, ShakeY float64
	Colour                      [4]uint8
	Orbs                        [3][3]float64
}

// Particle is one live particle (particles.go) as the guests draw it: type, position and height, the age the size,
// frame and colour follow from, and the sign of its horizontal direction (the quad is mirrored for -1).
type Particle struct {
	X, Y, Z, Size, Age float64
	Kind               int
	Left               bool
}

// RobotShot is one live projectile of the future boss (robot_projectiles.go): a flyer (kind 0x19), a rocket (0x13)
// or a minigun beam (0x11), as the guests draw it.
type RobotShot struct {
	X, Y, Lift, Size float64
	Heading, Alpha   int
}

type Bullet struct {
	X, Y, VX, VY, Life, Angle float64
	Kind                      string
	Proj                      *weapons.NativeWeaponProjectile
}

type Pickup struct {
	ID      int
	X, Y    float64
	Texture string
	Lift    float64
}

// Actor is a scripted cutscene entity (a character created by the level script).
type Actor struct {
	ID             int
	X, Y, Rot      float64
	ScaleX, ScaleY float64
	Alpha          float64
	Texture        string
	FlipX, FlipY   bool `json:",omitempty"`
	Sized          bool `json:",omitempty"`
	RotSet         bool `json:",omitempty"`
	Screen, Feet   bool `json:",omitempty"`
	Angle          int
	Anim, Frame    int
}

type Snapshot struct {
	Seq        uint32
	Time       float64
	Strings    []string
	Players    []Player
	Zombies    []Zombie
	GibBodies  []Zombie // Disintegrate bodies (playState.gibBodies)
	Bullets    []Bullet
	Pickups    []Pickup
	Mines      [][3]float64
	Sentries   []Sentry
	Explosions [][3]float64
	Blood      []Blood
	Portals    []Portal
	Score      int
	Lives      int
	Multiplier int
	WaveIndex  int
	Kills      int
	ZombieSum  int
	Finished   bool
	WaveLive   bool
	Train      *Train
	Progress   float64
	Zoom       float64
	HasProg    bool
	Over       bool
	RexWaves   [][4]float64 // x, y, age, max size of each rex shockwave
	RexVenom   []Venom
	Sfx        []string
	Combo      []Combo
	HudFlash   float64
	HudColor   [3]float64

	// Actors are the scripted cutscene characters, so the guest sees the scene the host's script is playing.
	Actors []Actor `json:",omitempty"`

	// Paused is set while the host has the game paused: the world is frozen, so the guest shows a notice.
	Paused bool `json:",omitempty"`

	// Say is the line of cutscene dialogue on the host's screen right now; the guest draws it and the host alone skips it.
	Say *Say `json:",omitempty"`

	// Played are the one-shot sounds the host started since the last snapshot (so the guest hears the same game).
	Played []Play `json:",omitempty"`

	// Optional fields, absent from an older host and ignored by an older guest: the live particles
	// (see Particle) and the future boss's projectiles (see RobotShot).
	Particles    []Particle  `json:",omitempty"`
	RobotFlyers  []RobotShot `json:",omitempty"`
	RobotRockets []RobotShot `json:",omitempty"`
	RobotBeams   []RobotShot `json:",omitempty"`
}

// Say is one spoken line: its text and the cameo (portrait) texture path, empty for none.
type Say struct {
	T string
	C string `json:",omitempty"`
}

// Play is one sound the host started.
type Play struct {
	P string
	V float64 `json:",omitempty"`
}

// Venom is one rex venom projectile (rex_ai.go) as the guest draws it.
type Venom struct {
	X, Y, Heading, Height, Size, Age, Life float64
	State                                  int
}

type Sentry struct {
	X, Y, Age, Cooldown float64
	Gun                 string
	Column              int
	FlipX               bool
	Lift, Shake         float64
}

type Blood struct {
	X, Y, Age float64
	Variant   int
}

// Portal mirrors portalState with exported fields.
type Portal struct {
	X, Y, Age, Size, Rotation, Speed float64
	CellX, CellY, Frame              int
	Alpha                            float64
}

// Results is what the guests' results screen shows; only the host can pick
// Replay or Menu.
type Results struct {
	Score, Kills, Highscore int32
	LevelStart              int32
	Flags                   uint32
	Survival, Dead          bool
}

// Train is the level's train as the guests draw it.
type Train struct {
	X, Y   float64
	Active bool
	Frame  int
}

// Ghost is a speedy zombie afterimage as it travels in a snapshot.
type Ghost struct {
	X, Y, W, H, Frame float64 `json:",omitempty"`
	Angle, Alpha      int     `json:",omitempty"`
	FlipX             bool    `json:",omitempty"`
}

// Combo is one tracker popup in the online snapshot (screen space).
type Combo struct {
	Text       string
	X, Y, Size float64
	R, G, B, A uint8
}

func Encode(m Msg) []byte {
	raw, _ := json.Marshal(m)
	return raw
}

func Decode(raw []byte) (Msg, bool) {
	var m Msg
	if json.Unmarshal(raw, &m) != nil || m.T == "" {
		return Msg{}, false
	}
	return m, true
}
