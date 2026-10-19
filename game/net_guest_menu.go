package game

import (
	"image/color"

	"github.com/MorenoLand/Moreno.HalfBricked/game/netwire"
	"github.com/MorenoLand/Moreno.HalfBricked/game/platform"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// What the online guest sees when pressing Esc: a small menu over the running game (the host's game does not pause), with
// RESUME and LEAVE GAME, instead of leaving at once. Esc again, or RESUME, closes it.

// guestMenuActions are the rows of the guest's menu, as pause menu action indices (pause_menu.go): the host's menu without
// JOIN / HOST and with LEAVE GAME as QUIT TO MENU. Options, stats and the sound toggles are the guest's own.
var guestMenuActions = [...]int{0, 1, 2, 5, 3, 4}

// maxForwardedSounds caps the sounds passed to guests in one snapshot (a minigun is many shots).
const maxForwardedSounds = 24

// forwardSoundToGuests is the SoundSystem hook: the online host records every one-shot sound it starts so the next
// snapshot can carry it to the guests.
func (a *app) forwardSoundToGuests(path string, volume float64) {
	n := a.net
	if n == nil || !n.isHost || n.lobby || len(n.played) >= maxForwardedSounds {
		return
	}
	same := 0
	for _, played := range n.played {
		if played.P == path {
			same++
		}
	}
	if same >= 4 {
		return
	}
	n.played = append(n.played, netwire.Play{P: path, V: volume})
}

// updateGuestMenu handles the open menu: the same rows as the pause menu, but the host's game keeps running, so every
// page it opens (options, stats, achievements) leaves the link running (the guest pump in Update).
func (a *app) updateGuestMenu() error {
	n := a.net
	if handled, err := a.updatePauseToggles(); handled || err != nil {
		return err
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		n.menuRow = (n.menuRow + len(guestMenuActions) - 1) % len(guestMenuActions)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		n.menuRow = (n.menuRow + 1) % len(guestMenuActions)
	}
	choose := -1
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
		choose = n.menuRow
	}
	x, y := a.pointer()
	if row := pauseRowHit(x, y, len(guestMenuActions)); row >= 0 {
		n.menuRow = row
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			choose = row
		}
	}
	if choose < 0 {
		return nil
	}
	switch guestMenuActions[choose] {
	case 0:
		n.menuOpen = false
	case 1:
		a.page = 3
	case 2:
		a.statsScreen, a.page = newStatsMenuBuild(&a.statistics, a.statsWestern()), 4
	case 3:
		a.askConfirm("QUIT TO MENU?", func() error {
			a.leaveNet("left the game")
			return a.quitToMenu()
		})
	case 4:
		a.askNativeQuit(platform.QuitGame)
	case 5:
		a.openAchievements()
	}
	return nil
}

func (a *app) drawGuestMenu(screen *ebiten.Image) {
	n := a.net
	a.drawRect(screen, 0, 0, logicalWidth, logicalHeight, color.RGBA{0, 0, 0, 160})
	pointerX, pointerY := a.pointer()
	hoveredToggle := 0
	if a.confirm == nil {
		hoveredToggle = a.pauseToggleAt(float64(pointerX), float64(pointerY))
	}
	a.drawPauseFixture(screen, hoveredToggle, pausePadFocus.toggle)
	a.textCentered(screen, "MENU", 78, 1)
	hovered := pauseRowHit(pointerX, pointerY, len(guestMenuActions))
	for index, action := range guestMenuActions {
		label := pauseMenuLabels[action]
		amount := a.hoverAmount("guestmenu"+label, (index == hovered || index == n.menuRow) && a.confirm == nil)
		a.drawHoverText(screen, label, logicalWidth/2, float64(pauseMenuTop+5+index*pauseMenuStride), .5, amount)
	}
}

// drawHostPausedNotice tells the guest why the world stands still: the host has the game paused.
func (a *app) drawHostPausedNotice(screen *ebiten.Image) {
	a.drawRect(screen, 0, 0, logicalWidth, logicalHeight, color.RGBA{0, 0, 0, 120})
	a.textCentered(screen, "PAUSED", 120, 1)
	a.textCentered(screen, "THE HOST PAUSED THE GAME", 156, .42)
}
