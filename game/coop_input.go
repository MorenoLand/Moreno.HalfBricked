package game

import (
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const gamepadDeadzone = .25

func radialDeadzone(x, y float64) (float64, float64) {
	length := math.Hypot(x, y)
	if length < gamepadDeadzone {
		return 0, 0
	}
	scale := (math.Min(length, 1) - gamepadDeadzone) / (1 - gamepadDeadzone) / length
	return x * scale, y * scale
}

// gamepadInput reads one gamepad: left stick moves, right stick aims and fires,
// and a shoulder button or trigger (or the south face button) throws the
// secondary weapon.
func gamepadInput(id ebiten.GamepadID) playerInput {
	var input playerInput
	if ebiten.IsStandardGamepadLayoutAvailable(id) {
		input.moveX, input.moveY = radialDeadzone(ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal), ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical))
		input.aimX, input.aimY = radialDeadzone(ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisRightStickHorizontal), ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisRightStickVertical))
		for _, button := range []ebiten.StandardGamepadButton{ebiten.StandardGamepadButtonFrontTopRight, ebiten.StandardGamepadButtonFrontBottomRight, ebiten.StandardGamepadButtonRightBottom} {
			if ebiten.IsStandardGamepadButtonPressed(id, button) {
				input.secondary = true
			}
			if inpututil.IsStandardGamepadButtonJustPressed(id, button) {
				input.secondaryHit = true
			}
		}
		return input
	}
	if ebiten.GamepadAxisCount(id) >= 4 {
		input.moveX, input.moveY = radialDeadzone(ebiten.GamepadAxisValue(id, 0), ebiten.GamepadAxisValue(id, 1))
		input.aimX, input.aimY = radialDeadzone(ebiten.GamepadAxisValue(id, 2), ebiten.GamepadAxisValue(id, 3))
	}
	return input
}

func attachedGamepads() []ebiten.GamepadID {
	ids := ebiten.AppendGamepadIDs(nil)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) > maxCoopPlayers {
		ids = ids[:maxCoopPlayers]
	}
	return ids
}

// coopLaunch describes the co-op session the Co-op button would start.
type coopLaunch struct {
	available, split, desktopPad bool
	players                      int
}

// currentCoopLaunch applies the native availability test to the attached
// gamepads. Touch co-op (split screen) is not implemented, so touch points are
// not offered.
func (a *app) currentCoopLaunch() coopLaunch {
	pads := len(attachedGamepads())
	desktopPad := !a.mobile
	launch := coopLaunch{desktopPad: desktopPad && pads == 1}
	launch.available = coopAvailable(pads, 0, desktopPad)
	if launch.available {
		launch.players, launch.split = coopRequest(pads, 0, desktopPad)
	}
	return launch
}

// gatherPlayerInputs maps gamepads to players: with two or more pads, pad i
// drives player i (player 0 also keeps keyboard and mouse); with one pad on
// desktop, that pad drives player 1 while player 0 stays on keyboard and mouse.
func (a *app) gatherPlayerInputs() [maxCoopPlayers]playerInput {
	var inputs [maxCoopPlayers]playerInput
	if a.inputHook != nil {
		inputs[0] = a.inputHook()
		return inputs
	}
	pads := attachedGamepads()
	if a.netHosting() && a.play != nil {
		if len(pads) > 0 {
			inputs[0] = gamepadInput(pads[0])
		}
		for _, peer := range a.net.peers {
			if peer.index > 0 && peer.index < maxCoopPlayers {
				inputs[peer.index] = peer.input
			}
		}
		return inputs
	}
	if a.play != nil && a.play.coopActive() && a.coopDesktopPad {
		for index := 1; index < maxCoopPlayers && index-1 < len(pads); index++ {
			inputs[index] = gamepadInput(pads[index-1])
		}
		return inputs
	}
	for index := 0; index < maxCoopPlayers && index < len(pads); index++ {
		inputs[index] = gamepadInput(pads[index])
	}
	return inputs
}

// The Co-op button sits between Play and Back on the level select screen
// (native navigation order Play -> Coop -> Back).
func (a *app) coopButtonRect() (x, y, width, height float64, ok bool) {
	play, playOK := a.variables.Vec2Value("SHOPFRONT_PLAY_ICON_POS_VAR")
	back, backOK := a.variables.Vec2Value("SHOPFRONT_BACK_ICON_NO_GLOBAL_POS_VAR")
	width, widthOK := a.variables.FloatValue("SHOPFRONT_PLAY_ICON_WIDTH_VAR")
	height, heightOK := a.variables.FloatValue("SHOPFRONT_PLAY_ICON_HEIGHT_VAR")
	if !playOK || !backOK || !widthOK || !heightOK {
		return 0, 0, 0, 0, false
	}
	return (play.X + back.X) / 2, (play.Y + back.Y) / 2, width, height, true
}

func (a *app) coopButtonHit(x, y int) bool {
	cx, cy, width, height, ok := a.coopButtonRect()
	return ok && math.Abs(float64(x)-cx) <= width/2 && math.Abs(float64(y)-cy) <= height/2
}
