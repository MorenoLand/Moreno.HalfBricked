package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Gamepad menu navigation and the "Game Controller Attached" notice
// (Research/native/offscreen-markers-gamepad-2026-10-10.md).
//
// The original drives its front end with the focus graphs in Controller/*.txt (a node,
// Up/Down/Left/Right targets, IsDefault, IsBack; ui_controller.go parses them) and plays
// Common0/Sound/SFX/ControllerMove.ogg when focus moves. The physical buttons arrive
// through NativeGameLib.native_keyEvent (Android key codes, not decoded), so the button
// to direction/press mapping below is the port's convention (UNRESOLVED natively):
//
//	d-pad or left stick  -> Up / Down / Left / Right
//	A  (bottom face)     -> the focused node is pressed (Enter)
//	B  (right face)      -> IsBack (Escape)
//	Start                -> Escape (pause / resume, PauseScreen.txt IsBack)
//
// uiKeyJustPressed is a drop-in for inpututil.IsKeyJustPressed on the menu paths: it
// reports the keyboard key or the gamepad control mapped to it.

type padKey int

const (
	padKeyUp padKey = iota
	padKeyDown
	padKeyLeft
	padKeyRight
	padKeyConfirm
	padKeyBack
	padKeyStart
	padKeyCount
)

// padStickThreshold is how far a stick must be pushed before it counts as a d-pad press.
const padStickThreshold = .5

// padDevices is the gamepad API the navigation reads; tests replace padBackend.
type padDevices interface {
	Connected() []ebiten.GamepadID
	Button(id ebiten.GamepadID, button ebiten.StandardGamepadButton) bool
	Axis(id ebiten.GamepadID, axis ebiten.StandardGamepadAxis) float64
	Tick() int64
}

type ebitenPads struct{}

func (ebitenPads) Connected() []ebiten.GamepadID {
	ids := ebiten.AppendGamepadIDs(nil)
	standard := ids[:0]
	for _, id := range ids {
		if ebiten.IsStandardGamepadLayoutAvailable(id) {
			standard = append(standard, id)
		}
	}
	return standard
}

func (ebitenPads) Button(id ebiten.GamepadID, button ebiten.StandardGamepadButton) bool {
	return ebiten.IsStandardGamepadButtonPressed(id, button)
}

func (ebitenPads) Axis(id ebiten.GamepadID, axis ebiten.StandardGamepadAxis) float64 {
	return ebiten.StandardGamepadAxisValue(id, axis)
}

func (ebitenPads) Tick() int64 { return ebiten.Tick() }

var padBackend padDevices = ebitenPads{}

// padNavState is the per-tick edge detector over every attached pad.
type padNavState struct {
	tick       int64
	started    bool
	held, just [padKeyCount]bool
}

var padNav padNavState

// poll samples the pads once per tick.
func (s *padNavState) poll(dev padDevices) {
	tick := dev.Tick()
	if s.started && tick == s.tick {
		return
	}
	s.started, s.tick = true, tick
	var now [padKeyCount]bool
	for _, id := range dev.Connected() {
		set := func(key padKey, down bool) {
			if down {
				now[key] = true
			}
		}
		set(padKeyUp, dev.Button(id, ebiten.StandardGamepadButtonLeftTop))
		set(padKeyDown, dev.Button(id, ebiten.StandardGamepadButtonLeftBottom))
		set(padKeyLeft, dev.Button(id, ebiten.StandardGamepadButtonLeftLeft))
		set(padKeyRight, dev.Button(id, ebiten.StandardGamepadButtonLeftRight))
		x := dev.Axis(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
		y := dev.Axis(id, ebiten.StandardGamepadAxisLeftStickVertical)
		if math.Max(math.Abs(x), math.Abs(y)) >= padStickThreshold {
			// The dominant axis decides, so a diagonal push never presses two directions.
			if math.Abs(x) >= math.Abs(y) {
				set(padKeyLeft, x < 0)
				set(padKeyRight, x > 0)
			} else {
				set(padKeyUp, y < 0)
				set(padKeyDown, y > 0)
			}
		}
		set(padKeyConfirm, dev.Button(id, ebiten.StandardGamepadButtonRightBottom))
		set(padKeyBack, dev.Button(id, ebiten.StandardGamepadButtonRightRight))
		set(padKeyStart, dev.Button(id, ebiten.StandardGamepadButtonCenterRight))
	}
	for key := padKey(0); key < padKeyCount; key++ {
		s.just[key] = now[key] && !s.held[key]
		s.held[key] = now[key]
	}
}

// padKeyFor maps a keyboard key to the gamepad control that stands in for it.
func padKeyFor(key ebiten.Key) (padKey, bool) {
	switch key {
	case ebiten.KeyUp:
		return padKeyUp, true
	case ebiten.KeyDown:
		return padKeyDown, true
	case ebiten.KeyLeft:
		return padKeyLeft, true
	case ebiten.KeyRight:
		return padKeyRight, true
	case ebiten.KeyEnter, ebiten.KeyKPEnter:
		return padKeyConfirm, true
	case ebiten.KeyEscape, ebiten.KeyBackspace:
		return padKeyBack, true
	}
	return 0, false
}

// padJustPressed reports a mapped pad control that went down this tick. Start doubles as
// Escape (it opens and closes the pause menu).
func padJustPressed(dev padDevices, key padKey) bool {
	padNav.poll(dev)
	if key == padKeyBack {
		return padNav.just[padKeyBack] || padNav.just[padKeyStart]
	}
	return padNav.just[key]
}

// uiKeyJustPressed is inpututil.IsKeyJustPressed plus the mapped gamepad control.
func uiKeyJustPressed(key ebiten.Key) bool {
	if inpututil.IsKeyJustPressed(key) {
		return true
	}
	if mapped, ok := padKeyFor(key); ok {
		return padJustPressed(padBackend, mapped)
	}
	return false
}

// menuFocusMove reports the arrow key or pad direction pressed this tick as -1 (up/left), +1
// (down/right) or 0, for the row lists that walk a focus.
func menuFocusMove() int {
	switch {
	case uiKeyJustPressed(ebiten.KeyUp), uiKeyJustPressed(ebiten.KeyLeft):
		return -1
	case uiKeyJustPressed(ebiten.KeyDown), uiKeyJustPressed(ebiten.KeyRight):
		return 1
	}
	return 0
}

// playControllerMove plays Common0/Sound/SFX/ControllerMove.ogg (shipped for exactly this).
func (a *app) playControllerMove() {
	if a.pack == nil || a.silent {
		return
	}
	a.playSound(a.scriptSoundPath("ControllerMove"), .7) // volume UNRESOLVED
}

// padMenu is the pad focus of the pause menu rows (PauseScreen.txt: Pause IsDefault, so the
// first row, RESUME, starts focused).
type padMenuFocus struct {
	active   bool
	row      int
	lastTick int64
}

// touch drops a stale focus: the menu was not updated on the previous tick, so it was closed
// and is being opened again at its default node.
func (f *padMenuFocus) touch(tick int64) {
	if f.lastTick != 0 && tick-f.lastTick > 1 {
		*f = padMenuFocus{}
	}
	f.lastTick = tick
}

var pausePadFocus padMenuFocus

// step moves the focus along rows (no wrap, like the graph's dead ends) and reports whether
// it moved.
func (f *padMenuFocus) step(move, rows int) bool {
	if rows <= 0 || move == 0 {
		return false
	}
	if !f.active {
		f.active = true
		return true
	}
	next := f.row + move
	if next < 0 || next >= rows {
		return false
	}
	f.row = next
	return true
}

// reset puts the focus back on the default node.
func (f *padMenuFocus) reset() { *f = padMenuFocus{} }

// Controller notice ----------------------------------------------------------------

// ControllerNotification.uiscreen: a 241x116 nine-patch window (NinePatchTest) centred at x = 240,
// starting at y = -200, with the text "Game Controller Attached" (green 100,228,100) 33.5 below its
// centre. The "Show" animation's position track has the keyframes (0 ms, y -80), (250 ms, y -20),
// (1500 ms, y -20), (1800 ms, y -80). The interpolation between them is taken as linear
// (UNRESOLVED).
var controllerNoticeKeys = [...]struct{ t, y float64 }{{0, -80}, {.25, -20}, {1.5, -20}, {1.8, -80}}

const (
	controllerNoticeRestY = -200.0 // the window's authored position
	controllerNoticeTotal = 1.8
)

// controllerNoticeY is the window centre y at `age` seconds into the Show animation.
func controllerNoticeY(age float64) (float64, bool) {
	if age < 0 || age >= controllerNoticeTotal {
		return controllerNoticeRestY, false
	}
	for index := 1; index < len(controllerNoticeKeys); index++ {
		previous, next := controllerNoticeKeys[index-1], controllerNoticeKeys[index]
		if age <= next.t {
			f := (age - previous.t) / (next.t - previous.t)
			return previous.y + (next.y-previous.y)*f, true
		}
	}
	return controllerNoticeRestY, false
}

type controllerNoticeState struct {
	seen   map[ebiten.GamepadID]bool
	active bool
	age    float64
	screen *formats.UIScreen
	pack   any
}

var padNotice controllerNoticeState

// update advances the animation and starts it when a pad that was not seen before is attached
// (pads already attached when the game starts count as attached, as on a device).
func (s *controllerNoticeState) update(dev padDevices, dt float64) {
	if s.seen == nil {
		s.seen = map[ebiten.GamepadID]bool{}
	}
	current := map[ebiten.GamepadID]bool{}
	for _, id := range dev.Connected() {
		current[id] = true
		if !s.seen[id] {
			s.active, s.age = true, 0
		}
	}
	s.seen = current
	if s.active {
		s.age += dt
		if s.age >= controllerNoticeTotal {
			s.active = false
		}
	}
}

func (a *app) updateControllerNotice(dt float64) {
	padNotice.update(padBackend, dt)
}

// drawControllerNotice draws the sliding banner from ControllerNotification.uiscreen.
func (a *app) drawControllerNotice(screen *ebiten.Image) {
	if !padNotice.active || a.pack == nil {
		return
	}
	y, visible := controllerNoticeY(padNotice.age)
	if !visible {
		return
	}
	if padNotice.screen == nil || padNotice.pack != a.pack {
		padNotice.pack = a.pack
		padNotice.screen = nil
		if path, ok := a.pack.SourcePath("Common0/UserInterface/screens/ControllerNotification.uiscreen"); ok {
			if reader, err := a.pack.Open(path); err == nil {
				data, _ := ioReadAll(reader)
				reader.Close()
				padNotice.screen, _ = formats.ParseUIScreen(data)
			}
		}
	}
	window := (*formats.UIComponent)(nil)
	if padNotice.screen != nil {
		window = padNotice.screen.Find("ControllerNotification")
		for _, c := range padNotice.screen.Components {
			if c.Class == "ComponentWindow" {
				window = c
				break
			}
		}
	}
	offset := formats.Vec2{Y: y - controllerNoticeRestY}
	text := "Game Controller Attached"
	size, r, g, b := 18.0, 100.0, 228.0, 100.0
	cx, cy, width := 240.1, y+33.5, 216.0
	if window != nil {
		a.drawCreditsNinePatch(screen, window, offset)
		for _, child := range window.Children {
			if child.Class != "ComponentText" {
				continue
			}
			if value := child.String("text"); value != "" {
				text = value
			}
			if value, ok := child.FontSize(); ok {
				size = value
			}
			if cr, cg, cb, _, ok := child.Colour("colour"); ok {
				r, g, b = cr, cg, cb
			}
			cx, cy = child.Anchor()
			cy += offset.Y
			if w, ok := child.Width(); ok {
				width = w
			}
		}
	}
	a.drawCreditsText(screen, text, cx, cy, width, "", size, r, g, b)
}

// analogMoveScale is the speed fraction of a movement vector. FUN_00096818 builds
// vector = 2 * stick (the stick value is in -1..1) and, only when its length exceeds 1.0
// (DAT_00096b70), divides it by its length; the velocity (+0x1c/+0x20) is that vector times a
// constant. A deflection of 0.5 or more therefore walks at full speed and anything less walks
// proportionally slower; keyboard vectors (length 1 or 1.41) always walk at full speed.
func analogMoveScale(length float64) float64 {
	return math.Min(1, 2*length)
}
