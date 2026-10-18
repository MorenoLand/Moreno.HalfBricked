package game

import (
	"image/color"

	"github.com/MorenoLand/Moreno.HalfBricked/game/netwire"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// What the online guest sees when pressing Esc: a small menu over the running game (the host's game does not pause), with
// RESUME and LEAVE GAME, instead of leaving at once. Esc again, or RESUME, closes it.

var guestMenuLabels = [...]string{"RESUME", "LEAVE GAME"}

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

// updateGuestMenu handles the open menu and reports true when the guest chose to leave.
func (a *app) updateGuestMenu() (leave bool) {
	n := a.net
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		n.menuRow = (n.menuRow + len(guestMenuLabels) - 1) % len(guestMenuLabels)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		n.menuRow = (n.menuRow + 1) % len(guestMenuLabels)
	}
	choose := -1
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
		choose = n.menuRow
	}
	x, y := a.pointer()
	if row := pauseRowHit(x, y, len(guestMenuLabels)); row >= 0 {
		n.menuRow = row
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			choose = row
		}
	}
	switch choose {
	case 0:
		n.menuOpen = false
	case 1:
		return true
	}
	return false
}

func (a *app) drawGuestMenu(screen *ebiten.Image) {
	n := a.net
	a.drawRect(screen, 0, 0, logicalWidth, logicalHeight, color.RGBA{0, 0, 0, 150})
	a.textCentered(screen, "MENU", 78, 1)
	px, py := a.pointer()
	hovered := pauseRowHit(px, py, len(guestMenuLabels))
	for index, label := range guestMenuLabels {
		amount := a.hoverAmount("guestmenu"+label, index == hovered || index == n.menuRow)
		a.drawHoverText(screen, label, logicalWidth/2, float64(pauseMenuTop+5+index*pauseMenuStride), .5, amount)
	}
}
