// Package netplay is a serverless peer-to-peer link over WebRTC data channels.
// It builds for desktop (pure Go) and for js/wasm (the browser's own
// RTCPeerConnection). Connection info travels between peers as one text code
// each way.
package netplay

import (
	"errors"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
)

// Public STUN only; strict NATs need a TURN relay, which the port does not ship.
var iceServers = []webrtc.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302", "stun:stun.cloudflare.com:3478"}}}

// Link is one peer connection with a reliable and an unreliable channel.
type Link struct {
	pc *webrtc.PeerConnection

	mu       sync.Mutex
	reliable *webrtc.DataChannel
	fast     *webrtc.DataChannel
	inbox    [][]byte
	open     int
	closed   bool

	gathered chan struct{}
}

func newLink() (*Link, error) {
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{ICEServers: iceServers})
	if err != nil {
		return nil, err
	}
	l := &Link{pc: pc, gathered: make(chan struct{})}
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed || state == webrtc.PeerConnectionStateDisconnected {
			l.mu.Lock()
			l.closed = true
			l.mu.Unlock()
		}
	})
	return l, nil
}

func (l *Link) attach(dc *webrtc.DataChannel) {
	dc.OnOpen(func() {
		l.mu.Lock()
		l.open++
		l.mu.Unlock()
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		l.mu.Lock()
		l.inbox = append(l.inbox, append([]byte(nil), msg.Data...))
		l.mu.Unlock()
	})
	l.mu.Lock()
	switch dc.Label() {
	case "reliable":
		l.reliable = dc
	case "fast":
		l.fast = dc
	}
	l.mu.Unlock()
}

// Ready is true once both data channels are open.
func (l *Link) Ready() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.open >= 2 && !l.closed
}

// Closed is true after the peer left or the connection failed.
func (l *Link) Closed() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.closed
}

// Send queues a message. Reliable messages are ordered and retransmitted; fast
// ones are unordered and may be dropped (state snapshots, inputs).
func (l *Link) Send(reliable bool, data []byte) {
	l.mu.Lock()
	dc := l.fast
	if reliable {
		dc = l.reliable
	}
	ok := dc != nil && l.open >= 2 && !l.closed
	l.mu.Unlock()
	if ok {
		_ = dc.Send(data)
	}
}

// Receive drains every message received since the last call.
func (l *Link) Receive() [][]byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := l.inbox
	l.inbox = nil
	return out
}

func (l *Link) Close() {
	l.mu.Lock()
	l.closed = true
	l.mu.Unlock()
	_ = l.pc.Close()
}

// watchGathering must run before SetLocalDescription; a nil candidate means
// gathering finished.
func (l *Link) watchGathering() {
	var once sync.Once
	l.pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			once.Do(func() { close(l.gathered) })
		}
	})
}

// gather waits for ICE gathering to finish so the code carries every candidate
// (no trickle, so one code each way is enough).
func (l *Link) gather(timeout time.Duration) error {
	select {
	case <-l.gathered:
		return nil
	case <-time.After(timeout):
		if l.pc.LocalDescription() != nil {
			return nil // use what was gathered so far
		}
		return errors.New("netplay: ICE gathering timed out")
	}
}

// Host is a pending host connection waiting for the guest's reply code.
type Host struct {
	Link *Link
	// Code is what the host gives the guest.
	Code string
}

// NewHost creates the offer code to give to a guest.
func NewHost() (*Host, error) {
	l, err := newLink()
	if err != nil {
		return nil, err
	}
	l.watchGathering()
	fast := &webrtc.DataChannelInit{Ordered: ptr(false), MaxRetransmits: ptr(uint16(0))}
	for _, spec := range []struct {
		label string
		init  *webrtc.DataChannelInit
	}{{"reliable", nil}, {"fast", fast}} {
		dc, err := l.pc.CreateDataChannel(spec.label, spec.init)
		if err != nil {
			l.Close()
			return nil, err
		}
		l.attach(dc)
	}
	offer, err := l.pc.CreateOffer(nil)
	if err != nil {
		l.Close()
		return nil, err
	}
	if err := l.pc.SetLocalDescription(offer); err != nil {
		l.Close()
		return nil, err
	}
	if err := l.gather(8 * time.Second); err != nil {
		l.Close()
		return nil, err
	}
	code, err := encode(l.pc.LocalDescription())
	if err != nil {
		l.Close()
		return nil, err
	}
	return &Host{Link: l, Code: code}, nil
}

// Accept completes the connection with the guest's reply code.
func (h *Host) Accept(reply string) error {
	desc, err := decode(reply)
	if err != nil {
		return err
	}
	if desc.Type != webrtc.SDPTypeAnswer {
		return errors.New("netplay: that is a host code, not a reply code")
	}
	return h.Link.pc.SetRemoteDescription(*desc)
}

// Join answers a host's code and returns the link plus the reply code to send back.
func Join(hostCode string) (*Link, string, error) {
	offer, err := decode(hostCode)
	if err != nil {
		return nil, "", err
	}
	if offer.Type != webrtc.SDPTypeOffer {
		return nil, "", errors.New("netplay: that is a reply code, not a host code")
	}
	l, err := newLink()
	if err != nil {
		return nil, "", err
	}
	l.watchGathering()
	l.pc.OnDataChannel(l.attach)
	if err := l.pc.SetRemoteDescription(*offer); err != nil {
		l.Close()
		return nil, "", err
	}
	answer, err := l.pc.CreateAnswer(nil)
	if err != nil {
		l.Close()
		return nil, "", err
	}
	if err := l.pc.SetLocalDescription(answer); err != nil {
		l.Close()
		return nil, "", err
	}
	if err := l.gather(8 * time.Second); err != nil {
		l.Close()
		return nil, "", err
	}
	code, err := encode(l.pc.LocalDescription())
	if err != nil {
		l.Close()
		return nil, "", err
	}
	return l, code, nil
}

func ptr[T any](v T) *T { return &v }

func encode(desc *webrtc.SessionDescription) (string, error) { return packDescription(desc) }

func decode(code string) (*webrtc.SessionDescription, error) { return unpackDescription(code) }
