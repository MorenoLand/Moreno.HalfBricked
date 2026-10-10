package game

import (
	"image/color"
	"log"
	"math"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/netplay"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Online co-op over WebRTC. The host pastes nothing but one reply per guest:
// host code out, reply code back. The host runs the game; guests mirror it.

const maxNetGuests = maxCoopPlayers - 1

type netPeer struct {
	link     *netplay.Link
	index    int
	input    playerInput
	lastSec  int
	announce bool
}

type netHostResult struct {
	host *netplay.Host
	err  error
}

type netJoinResult struct {
	link  *netplay.Link
	reply string
	err   error
}

type netSession struct {
	isHost  bool
	problem string
	lobby   bool   // true until the game starts
	note    string // progress message
	reply   string // the guest's reply code
	pasted  string // last text pasted, shown in the paste box
	copied  int    // frames left on the "Copied!" flash

	// host
	peers   []*netPeer
	pending *netplay.Host
	hosting chan netHostResult

	// guest
	link    *netplay.Link
	joining chan netJoinResult
	self    int
	secSent int
	snap    *wireSnapshot
	gotSnap uint32

	seq  uint32
	tick int
	sfx  []string

	resultsSent bool
}

func (n *netSession) close() {
	for _, peer := range n.peers {
		peer.link.Send(true, encodeWire(wireMsg{T: "end"}))
		peer.link.Close()
	}
	if n.pending != nil {
		n.pending.Link.Close()
	}
	if n.link != nil {
		n.link.Send(true, encodeWire(wireMsg{T: "end"}))
		n.link.Close()
	}
}

func (a *app) netHosting() bool { return a.net != nil && a.net.isHost }
func (a *app) netGuest() bool   { return a.net != nil && !a.net.isHost }

func (a *app) startNetHost() {
	n := &netSession{isHost: true, lobby: true}
	a.net = n
	a.requestHostCode()
}

func (a *app) requestHostCode() {
	n := a.net
	n.note = "Creating your game code..."
	n.hosting = make(chan netHostResult, 1)
	go func() {
		host, err := netplay.NewHost()
		n.hosting <- netHostResult{host, err}
	}()
}

func (a *app) startNetJoin() {
	a.net = &netSession{lobby: true}
}

func (a *app) leaveNet(message string) {
	if a.net != nil {
		a.net.close()
		a.net = nil
	}
	if message != "" {
		log.Print("netplay: " + message)
	}
}

// updateNetLobby runs the connection screens. It returns true while the lobby
// owns the frame.
func (a *app) updateNetLobby() error {
	n := a.net
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		a.leaveNet("cancelled")
		return nil
	}
	if text, ok := clipboardTake(); ok {
		a.netPasted(strings.TrimSpace(text))
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyV) || (inpututil.IsKeyJustPressed(ebiten.KeyInsert) && ebiten.IsKeyPressed(ebiten.KeyShift)) {
		clipboardRequest()
	}
	if err := a.updateLobbyMouse(); err != nil || a.net == nil || !a.net.lobby {
		return err
	}
	if n.isHost {
		return a.updateHostLobby()
	}
	return a.updateGuestLobby()
}

func (a *app) netPasted(text string) {
	n := a.net
	n.pasted = text
	if n.isHost {
		if n.pending == nil {
			return
		}
		if err := n.pending.Accept(text); err != nil {
			n.problem = "That code didn't work: " + err.Error()
			return
		}
		n.peers = append(n.peers, &netPeer{link: n.pending.Link, index: len(n.peers) + 1})
		n.pending = nil
		n.problem = ""
		return
	}
	if n.link != nil || n.joining != nil {
		return
	}
	n.joining = make(chan netJoinResult, 1)
	n.note, n.problem = "Connecting...", ""
	go func() {
		link, reply, err := netplay.Join(text)
		n.joining <- netJoinResult{link, reply, err}
	}()
}

func (a *app) updateHostLobby() error {
	n := a.net
	select {
	case result := <-n.hosting:
		n.hosting = nil
		if result.err != nil {
			n.problem = "Could not create a code: " + result.err.Error()
			break
		}
		n.pending = result.host
		n.note, n.problem = "", ""
		clipboardWrite(result.host.Code)
	default:
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyH) && n.pending == nil && n.hosting == nil && len(n.peers) < maxNetGuests {
		a.requestHostCode()
	}
	ready := 0
	for _, peer := range n.peers {
		if peer.link.Ready() {
			ready++
		}
	}
	if ready > 0 && (inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter)) {
		return a.startNetHostGame()
	}
	return nil
}

func (a *app) startNetHostGame() error {
	n := a.net
	kept := n.peers[:0]
	for _, peer := range n.peers {
		if peer.link.Ready() {
			kept = append(kept, peer)
		} else {
			peer.link.Close()
		}
	}
	n.peers = kept
	if n.pending != nil {
		n.pending.Link.Close()
		n.pending = nil
	}
	for i, peer := range n.peers {
		peer.index = i + 1
	}
	if p := a.play; p != nil {
		return a.joinNetPlayersInPlace(p)
	}
	levels := a.filteredLevels()
	if a.level < 0 || a.level >= len(levels) || !a.levelUnlocked(levels[a.level]) {
		n.problem = "Pick an unlocked level first."
		return nil
	}
	a.coopPlayers, a.coopDesktopPad = len(n.peers)+1, false
	if err := a.openPlay(); err != nil {
		a.coopPlayers = 0
		return err
	}
	n.lobby = false
	for _, peer := range n.peers {
		peer.link.Send(true, encodeWire(wireMsg{T: "start", Level: levels[a.level].ID, Mode: a.mode, Self: peer.index, Players: len(n.peers) + 1}))
	}
	return nil
}

// joinNetPlayersInPlace adds the connected guests to the level already being
// played (hosting from the pause menu); guests load the same level and mirror it.
func (a *app) joinNetPlayersInPlace(p *playState) error {
	n := a.net
	base := 1
	if p.coopActive() {
		base += len(p.coop.players)
	}
	if room := maxCoopPlayers - base; len(n.peers) > room {
		for _, peer := range n.peers[max(room, 0):] {
			peer.link.Close()
		}
		n.peers = n.peers[:max(room, 0)]
	}
	if len(n.peers) == 0 {
		n.problem = "The game is full."
		return nil
	}
	total := base + len(n.peers)
	p.ensureCoopPlayers(total)
	a.coopPlayers = total
	for i, peer := range n.peers {
		peer.index = base + i
		peer.link.Send(true, encodeWire(wireMsg{T: "start", Level: p.levelInfo.ID, Mode: a.mode, Self: peer.index, Players: total}))
	}
	n.lobby = false
	p.paused = false
	return nil
}

func (a *app) updateGuestLobby() error {
	n := a.net
	select {
	case result := <-n.joining:
		n.joining = nil
		if result.err != nil {
			n.note, n.problem = "", "That code didn't work: "+result.err.Error()
			return nil
		}
		n.link = result.link
		clipboardWrite(result.reply)
		n.reply, n.note = result.reply, "Reply code copied. Send it to the host and wait for them to start."
	default:
	}
	if n.link == nil {
		return nil
	}
	if n.link.Closed() {
		n.link.Close()
		n.link = nil
		n.reply, n.note, n.problem = "", "", "The connection dropped. Paste the host's code again."
		return nil
	}
	for _, raw := range n.link.Receive() {
		msg, ok := decodeWire(raw)
		if !ok {
			continue
		}
		if msg.T == "start" {
			return a.startNetGuestGame(msg)
		}
	}
	return nil
}

// selectLevelByID points the level selector at a level by its ID.
func (a *app) selectLevelByID(id string, mode int) bool {
	a.mode = mode
	for world := range a.worlds() {
		a.world = world
		for index, item := range a.filteredLevels() {
			if item.ID == id {
				a.level = index
				return true
			}
		}
	}
	return false
}

func (a *app) startNetGuestGame(msg wireMsg) error {
	n := a.net
	if !a.selectLevelByID(msg.Level, msg.Mode) {
		n.problem = "The host picked a level this copy doesn't have."
		return nil
	}
	n.self = msg.Self
	if a.play != nil {
		a.stopWeaponPlayback()
		a.play.closeScript()
	}
	a.coopPlayers, a.coopDesktopPad = msg.Players, false
	if err := a.openPlay(); err != nil {
		a.coopPlayers = 0
		return err
	}
	n.lobby = false
	return nil
}

// netHostPoll reads guest controls before the frame's simulation.
func (a *app) netHostPoll() {
	n := a.net
	for _, peer := range n.peers {
		peer.input.secondaryHit = false
		for _, raw := range peer.link.Receive() {
			msg, ok := decodeWire(raw)
			if !ok {
				continue
			}
			switch msg.T {
			case "in":
				if msg.In == nil {
					continue
				}
				in := msg.In
				peer.input = playerInput{moveX: in.MoveX, moveY: in.MoveY, aimX: in.AimX, aimY: in.AimY, secondary: in.Secondary}
				if in.SecondaryCount > peer.lastSec {
					peer.input.secondaryHit = true
				}
				peer.lastSec = in.SecondaryCount
			case "end":
				peer.input = playerInput{}
			}
		}
		if peer.link.Closed() {
			peer.input = playerInput{}
		}
	}
}

// netHostSend ships a snapshot to every guest (30 per second).
func (a *app) netHostSend() {
	n := a.net
	n.sfx = append(n.sfx, a.play.sfxQueue...)
	n.tick++
	if n.tick%2 != 0 {
		return
	}
	n.seq++
	snap := a.play.snapshot(n.seq)
	snap.Sfx = n.sfx
	n.sfx = nil
	raw := encodeWire(wireMsg{T: "snap", Snap: snap})
	for _, peer := range n.peers {
		peer.link.Send(false, raw)
	}
}

// guestInput reads this machine's controls for the guest's own player.
func (a *app) guestInput() (playerInput, bool) {
	p := a.play
	var in playerInput
	if ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyLeft) {
		in.moveX--
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) || ebiten.IsKeyPressed(ebiten.KeyRight) {
		in.moveX++
	}
	if ebiten.IsKeyPressed(ebiten.KeyW) || ebiten.IsKeyPressed(ebiten.KeyUp) {
		in.moveY--
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) || ebiten.IsKeyPressed(ebiten.KeyDown) {
		in.moveY++
	}
	if length := math.Hypot(in.moveX, in.moveY); length > 1 {
		in.moveX, in.moveY = in.moveX/length, in.moveY/length
	}
	hit := inpututil.IsKeyJustPressed(ebiten.KeyG) || inpututil.IsKeyJustPressed(ebiten.KeyQ) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) || ebiten.IsKeyPressed(ebiten.KeySpace) {
		pointerX, pointerY := a.pointer()
		zoom := p.world.Zoom
		if zoom <= 0 {
			zoom = 1
		}
		if p.coopActive() && a.net.self >= 1 && a.net.self-1 < len(p.coop.players) {
			me := p.coop.players[a.net.self-1].body
			dx := float64(pointerX) - ((me.x-p.world.CameraX)*zoom + p.world.ViewportX)
			dy := float64(pointerY) - ((me.y-p.world.CameraY)*zoom + p.world.ViewportY)
			if length := math.Hypot(dx, dy); length > 1 {
				in.aimX, in.aimY = dx/length, dy/length
			}
		}
	}
	if pads := attachedGamepads(); len(pads) > 0 {
		pad := gamepadInput(pads[0])
		if pad.moveX != 0 || pad.moveY != 0 {
			in.moveX, in.moveY = pad.moveX, pad.moveY
		}
		if pad.aiming() {
			in.aimX, in.aimY = pad.aimX, pad.aimY
		}
		hit = hit || pad.secondaryHit
		in.secondary = pad.secondary
	}
	return in, hit
}

// updateNetGuest replaces the simulation on a guest: read the host's snapshot,
// send this player's controls, and keep the camera and clock moving.
func (a *app) updateNetGuest() error {
	n, p := a.net, a.play
	if n.link.Closed() {
		a.leaveNet("host left")
		a.play = nil
		a.setMenuMusic()
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		a.leaveNet("left the game")
		a.play = nil
		a.setMenuMusic()
		return nil
	}
	for _, raw := range n.link.Receive() {
		msg, ok := decodeWire(raw)
		if !ok {
			continue
		}
		switch msg.T {
		case "snap":
			if msg.Snap != nil && msg.Snap.Seq > n.gotSnap {
				n.gotSnap = msg.Snap.Seq
				p.applySnapshot(msg.Snap)
			}
		case "results":
			if msg.Res != nil {
				a.resultsScreen = newResultsMenu(msg.Res.data())
				a.page = 5
				a.stopWeaponPlayback()
				return nil
			}
		case "start":
			return a.startNetGuestGame(msg)
		case "end":
			a.leaveNet("host ended the game")
			a.play = nil
			a.setMenuMusic()
			return nil
		}
	}
	in, hit := a.guestInput()
	if hit {
		n.secSent++
	}
	n.link.Send(false, encodeWire(wireMsg{T: "in", In: &wireInput{MoveX: in.moveX, MoveY: in.moveY, AimX: in.aimX, AimY: in.aimY, Secondary: in.secondary, SecondaryCount: n.secSent}}))
	p.updateCamera()
	p.updateProgressOpacity(1.0 / 60.0)
	p.banner.advance(1.0 / 60.0)
	return nil
}

// netHostResults tells the guests once when the host's results screen opens.
func (a *app) netHostResults() {
	n := a.net
	if n == nil || !n.isHost || n.lobby {
		return
	}
	if a.page != 5 || a.resultsScreen == nil {
		n.resultsSent = false
		return
	}
	if n.resultsSent {
		return
	}
	n.resultsSent = true
	raw := encodeWire(wireMsg{T: "results", Res: a.wireResultsFrom(a.resultsScreen)})
	for _, peer := range n.peers {
		peer.link.Send(true, raw)
	}
}

// netBroadcastStart sends the freshly opened level to the guests (Replay, or the
// next story level), so the host alone decides what happens after a results screen.
func (a *app) netBroadcastStart() {
	n := a.net
	if n == nil || !n.isHost || n.lobby || a.play == nil {
		return
	}
	n.resultsSent = false
	total := len(n.peers) + 1
	if a.coopPlayers > total {
		total = a.coopPlayers
	}
	for _, peer := range n.peers {
		peer.link.Send(true, encodeWire(wireMsg{T: "start", Level: a.play.levelInfo.ID, Mode: a.mode, Self: peer.index, Players: total}))
	}
}

// updateGuestResults runs the guest's results screen: the buttons are the
// host's, so the guest only watches and waits for the next level or the end.
func (a *app) updateGuestResults() error {
	n := a.net
	leave := func() {
		a.leaveNet("host ended the game")
		a.resultsScreen = nil
		a.play, a.page = nil, 0
		a.setMenuMusic()
	}
	if n.link.Closed() || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		leave()
		return nil
	}
	for _, raw := range n.link.Receive() {
		msg, ok := decodeWire(raw)
		if !ok {
			continue
		}
		switch msg.T {
		case "start":
			a.resultsScreen = nil
			a.page = 2
			return a.startNetGuestGame(msg)
		case "end":
			leave()
			return nil
		}
	}
	if a.resultsScreen != nil {
		// Port addition (results_count.go): guests see the count-up too; a press finishes it.
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			a.resultsScreen.finishCount()
		}
		a.resultsScreen.update(1.0/60.0, false)
		a.playResultsTick(a.resultsScreen)
	}
	return nil
}

// drawGuestResultsOverlay greys out the buttons the guest cannot press.
func (a *app) drawGuestResultsOverlay(screen *ebiten.Image) {
	for _, name := range []string{"MENU", "REPLAY"} {
		position, posOK := a.variables.Vec2Value("ENDSCREEN_" + name + "_BOX_OUTER_POS_VAR")
		size, sizeOK := a.variables.Vec2Value("ENDSCREEN_" + name + "_BOX_OUTER_SIZE_VAR")
		if !posOK || !sizeOK || a.resultsScreen == nil {
			continue
		}
		offset := a.resultsScreen.offset(a.variables, "MAIN")
		a.drawRect(screen, position.X+offset.X-size.X/2, position.Y+offset.Y-size.Y/2, size.X, size.Y, color.RGBA{0, 0, 0, 130})
	}
	a.textCentered(screen, "WAITING FOR THE HOST TO REPLAY OR QUIT", 300, .4)
}
