package game

import (
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/netplay"
)

func TestGuestMovesOnTheHostAndSeesIt(t *testing.T) {
	hostGame := script125CachedHost(t, "world0_level1")
	guestGame := script125CachedHost(t, "world0_level1")
	hostApp, guestApp := hostGame.app, guestGame.app
	hostPlay, guestPlay := coopTestPlay(t), coopTestPlay(t)
	hostApp.play, guestApp.play = hostPlay, guestPlay

	pending, err := netplay.NewHost()
	if err != nil {
		t.Skip("no WebRTC here:", err)
	}
	guestLink, reply, err := netplay.Join(pending.Code)
	if err != nil {
		t.Fatal(err)
	}
	if err := pending.Accept(reply); err != nil {
		t.Fatal(err)
	}
	defer pending.Link.Close()
	defer guestLink.Close()
	wait(t, "link", func() bool { return pending.Link.Ready() && guestLink.Ready() })

	peer := &netPeer{link: pending.Link, index: 1}
	hostApp.net = &netSession{isHost: true, peers: []*netPeer{peer}}
	guestApp.net = &netSession{link: guestLink, self: 1}
	hostApp.coopPlayers = 2

	hostPlay.updateCoopPlayers([]playerInput{{}})
	start := hostPlay.coop.players[0].body.x
	for frame := 0; frame < 40; frame++ {
		guestLink.Send(false, encodeWire(wireMsg{T: "in", In: &wireInput{MoveX: 1}}))
		time.Sleep(5 * time.Millisecond)
		hostApp.netHostPoll()
		inputs := hostApp.gatherPlayerInputs()
		hostPlay.updateCoopPlayers(inputs[1:])
		hostApp.netHostSend()
	}
	if hostPlay.coop.players[0].body.x <= start+20 {
		t.Fatalf("the guest's player never moved on the host: %v -> %v", start, hostPlay.coop.players[0].body.x)
	}
	wait(t, "snapshot on the guest", func() bool {
		if err := guestApp.updateNetGuest(); err != nil {
			t.Fatal(err)
		}
		return guestApp.net.gotSnap > 0
	})
	got, want := guestPlay.coop.players[0].body.x, hostPlay.coop.players[0].body.x
	if got < start+10 || got > want+.5 {
		t.Fatalf("guest sees player 2 at %v; host has %v (start %v)", got, want, start)
	}
}

func wait(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestGuestSeesTheHostsResultsAndFollowsItsReplay(t *testing.T) {
	hostGame := script125CachedHost(t, "world0_level1")
	guestGame := script125CachedHost(t, "world0_level1")
	hostApp, guestApp := hostGame.app, guestGame.app
	hostApp.play, guestApp.play = coopTestPlay(t), coopTestPlay(t)

	pending, err := netplay.NewHost()
	if err != nil {
		t.Skip("no WebRTC here:", err)
	}
	guestLink, reply, err := netplay.Join(pending.Code)
	if err != nil {
		t.Fatal(err)
	}
	if err := pending.Accept(reply); err != nil {
		t.Fatal(err)
	}
	defer pending.Link.Close()
	defer guestLink.Close()
	wait(t, "link", func() bool { return pending.Link.Ready() && guestLink.Ready() })
	hostApp.net = &netSession{isHost: true, peers: []*netPeer{{link: pending.Link, index: 1}}}
	guestApp.net = &netSession{link: guestLink, self: 1}

	data := resultsData{Dead: true, Score: 1290}
	hostApp.resultsScreen, hostApp.page = newResultsMenu(data), 5
	hostApp.netHostResults()
	wait(t, "results on the guest", func() bool {
		if err := guestApp.updateNetGuest(); err != nil {
			t.Fatal(err)
		}
		return guestApp.page == 5 && guestApp.resultsScreen != nil
	})
	if !guestApp.resultsScreen.Data.Dead || guestApp.resultsScreen.Data.Score != 1290 {
		t.Fatalf("guest results %+v", guestApp.resultsScreen.Data)
	}
}
