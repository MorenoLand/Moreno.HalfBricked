package netplay

import (
	"github.com/pion/webrtc/v4"
	"strings"
	"testing"
	"time"
)

func wait(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestHostAndGuestConnectWithOneCodeEachWay(t *testing.T) {
	host, err := NewHost()
	if err != nil {
		t.Skip("no network stack for WebRTC here:", err)
	}
	defer host.Link.Close()
	t.Logf("host code is %d characters", len(host.Code))
	guest, reply, err := Join(host.Code)
	if err != nil {
		t.Fatal(err)
	}
	defer guest.Close()
	t.Logf("reply code is %d characters", len(reply))
	if err := host.Accept(reply); err != nil {
		t.Fatal(err)
	}
	wait(t, "both links ready", func() bool { return host.Link.Ready() && guest.Ready() })
	host.Link.Send(true, []byte("hello guest"))
	guest.Send(false, []byte("hello host"))
	var got []string
	wait(t, "messages", func() bool {
		for _, m := range host.Link.Receive() {
			got = append(got, "host:"+string(m))
		}
		for _, m := range guest.Receive() {
			got = append(got, "guest:"+string(m))
		}
		return len(got) >= 2
	})
	joined := strings.Join(got, ",")
	if !strings.Contains(joined, "host:hello host") || !strings.Contains(joined, "guest:hello guest") {
		t.Fatalf("messages = %v", got)
	}
}

func TestBadCodesAreRejected(t *testing.T) {
	for _, code := range []string{"", "hello", "AOZ2-!!!", "AOZ2-AAAA"} {
		if _, _, err := Join(code); err == nil {
			t.Fatalf("code %q was accepted", code)
		}
	}
}

func TestCodesRoundTripBrowserStyleDescriptions(t *testing.T) {
	sdp := "v=0\r\no=- 1 2 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\na=group:BUNDLE 0\r\nm=application 9 UDP/DTLS/SCTP webrtc-datachannel\r\nc=IN IP4 0.0.0.0\r\na=ice-ufrag:abcd\r\na=ice-pwd:0123456789abcdefghijkl\r\na=fingerprint:sha-256 9E:54:84:C7:BA:8B:6D:BC:FB:11:A4:60:09:C5:6A:B0:53:26:D3:72:B8:38:5D:F6:33:73:02:3E:C6:A4:26:E5\r\na=setup:actpass\r\na=mid:0\r\na=candidate:1 1 udp 2113937151 3f2a9c1e-7a52-4c1b-9d11-2b1a4a9c0e55.local 54321 typ host generation 0\r\na=candidate:2 1 udp 1677729535 203.0.113.9 40000 typ srflx raddr 0.0.0.0 rport 0 generation 0\r\na=candidate:3 1 udp 2113937151 2001:db8::1 50000 typ host generation 0\r\n"
	code, err := packDescription(&webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: sdp})
	if err != nil {
		t.Fatal(err)
	}
	desc, err := unpackDescription(code)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"3f2a9c1e-7a52-4c1b-9d11-2b1a4a9c0e55.local 54321 typ host", "203.0.113.9 40000 typ srflx", "2001:db8::1 50000 typ host", "a=ice-ufrag:abcd", "a=setup:actpass", "9E:54:84:C7"} {
		if !strings.Contains(desc.SDP, want) {
			t.Fatalf("rebuilt SDP is missing %q:\n%s", want, desc.SDP)
		}
	}
	t.Logf("%d characters", len(code))
}
