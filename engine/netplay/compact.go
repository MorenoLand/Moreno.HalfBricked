package netplay

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/pion/webrtc/v4"
)

// A data-channel-only SDP is mostly boilerplate, so a code carries just the
// fields that differ between sessions: the ICE credentials, the DTLS
// fingerprint and the UDP candidates. Everything else is rebuilt on arrival.
// The result is about 200 characters instead of 800.

const codePrefix = "AOZ2-"

const (
	candHost = iota
	candSrflx
	candRelay
)

var candTypes = []string{"host", "srflx", "relay"}
var candPriority = []int{2130706431, 1694498815, 16777215}
var setupRoles = []string{"actpass", "active", "passive"}

type packedCandidate struct {
	kind int
	addr string
	port uint16
}

func putString(buf *bytes.Buffer, v string) {
	buf.WriteByte(byte(len(v)))
	buf.WriteString(v)
}

func getString(data []byte, at *int) (string, bool) {
	if *at >= len(data) {
		return "", false
	}
	n := int(data[*at])
	*at++
	if *at+n > len(data) {
		return "", false
	}
	v := string(data[*at : *at+n])
	*at += n
	return v, true
}

func packDescription(desc *webrtc.SessionDescription) (string, error) {
	var ufrag, pwd, fingerprint, setup string
	var cands []packedCandidate
	seen := map[string]bool{}
	for _, line := range strings.Split(desc.SDP, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "a=ice-ufrag:"):
			ufrag = strings.TrimPrefix(line, "a=ice-ufrag:")
		case strings.HasPrefix(line, "a=ice-pwd:"):
			pwd = strings.TrimPrefix(line, "a=ice-pwd:")
		case strings.HasPrefix(line, "a=fingerprint:sha-256 "):
			fingerprint = strings.TrimPrefix(line, "a=fingerprint:sha-256 ")
		case strings.HasPrefix(line, "a=setup:"):
			setup = strings.TrimPrefix(line, "a=setup:")
		case strings.HasPrefix(line, "a=candidate:"):
			fields := strings.Fields(strings.TrimPrefix(line, "a="))
			// candidate:<found> <component> <proto> <prio> <addr> <port> typ <type> ...
			if len(fields) < 8 || fields[1] != "1" || !strings.EqualFold(fields[2], "udp") || fields[6] != "typ" {
				continue
			}
			kind := -1
			for i, name := range candTypes {
				if fields[7] == name {
					kind = i
				}
			}
			port, err := strconv.Atoi(fields[5])
			if kind < 0 || err != nil || port < 1 || port > 65535 {
				continue
			}
			key := fields[4] + ":" + fields[5]
			if seen[key] {
				continue
			}
			seen[key] = true
			cands = append(cands, packedCandidate{kind, fields[4], uint16(port)})
		}
	}
	rawPrint, err := hex.DecodeString(strings.ReplaceAll(fingerprint, ":", ""))
	if err != nil || len(rawPrint) != 32 || ufrag == "" || pwd == "" || len(cands) == 0 || len(cands) > 40 {
		return "", errors.New("netplay: the connection description has an unexpected shape")
	}
	role := 0
	for i, name := range setupRoles {
		if setup == name {
			role = i
		}
	}
	var buf bytes.Buffer
	typ := byte(0)
	if desc.Type == webrtc.SDPTypeAnswer {
		typ = 1
	}
	buf.WriteByte(typ<<4 | byte(role))
	putString(&buf, ufrag)
	putString(&buf, pwd)
	buf.Write(rawPrint)
	buf.WriteByte(byte(len(cands)))
	for _, c := range cands {
		if ip, err := netip.ParseAddr(c.addr); err == nil {
			raw := ip.AsSlice()
			buf.WriteByte(byte(c.kind)<<4 | byte(len(raw)/4)) // 1 = IPv4, 4 = IPv6
			buf.Write(raw)
		} else {
			buf.WriteByte(byte(c.kind)<<4 | 0xF) // a hostname (browsers hide LAN addresses behind mDNS names)
			putString(&buf, c.addr)
		}
		buf.WriteByte(byte(c.port >> 8))
		buf.WriteByte(byte(c.port))
	}
	return codePrefix + base64.RawURLEncoding.EncodeToString(buf.Bytes()), nil
}

var errDamaged = errors.New("netplay: the code is damaged (copy it again)")

func unpackDescription(code string) (*webrtc.SessionDescription, error) {
	code = strings.Join(strings.Fields(code), "")
	if !strings.HasPrefix(code, codePrefix) {
		return nil, errors.New("netplay: that is not a game code")
	}
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(code, codePrefix))
	if err != nil || len(data) < 4 {
		return nil, errDamaged
	}
	typ, role := data[0]>>4, int(data[0]&0xF)
	if typ > 1 || role >= len(setupRoles) {
		return nil, errDamaged
	}
	at := 1
	ufrag, ok1 := getString(data, &at)
	pwd, ok2 := getString(data, &at)
	if !ok1 || !ok2 || at+33 > len(data) {
		return nil, errDamaged
	}
	fingerprint := strings.ToUpper(hex.EncodeToString(data[at : at+32]))
	at += 32
	count := int(data[at])
	at++
	var sb strings.Builder
	var fp []string
	for i := 0; i < len(fingerprint); i += 2 {
		fp = append(fp, fingerprint[i:i+2])
	}
	fmt.Fprintf(&sb, "v=0\r\no=- 0 0 IN IP4 0.0.0.0\r\ns=-\r\nt=0 0\r\na=msid-semantic:WMS *\r\na=fingerprint:sha-256 %s\r\na=extmap-allow-mixed\r\na=group:BUNDLE 0\r\nm=application 9 UDP/DTLS/SCTP webrtc-datachannel\r\nc=IN IP4 0.0.0.0\r\na=setup:%s\r\na=mid:0\r\na=sendrecv\r\na=sctp-port:5000\r\na=max-message-size:262144\r\na=ice-ufrag:%s\r\na=ice-pwd:%s\r\n", strings.Join(fp, ":"), setupRoles[role], ufrag, pwd)
	for i := 0; i < count; i++ {
		if at >= len(data) {
			return nil, errDamaged
		}
		kind, size := int(data[at]>>4), int(data[at]&0xF)
		at++
		if kind >= len(candTypes) {
			return nil, errDamaged
		}
		var addr string
		switch size {
		case 1, 4:
			n := size * 4
			ip, ok := netip.AddrFromSlice(data[min(at, len(data)):min(at+n, len(data))])
			if at+n > len(data) || !ok {
				return nil, errDamaged
			}
			addr = ip.String()
			at += n
		case 0xF:
			host, ok := getString(data, &at)
			if !ok || strings.ContainsAny(host, " \r\n") {
				return nil, errDamaged
			}
			addr = host
		default:
			return nil, errDamaged
		}
		if at+2 > len(data) {
			return nil, errDamaged
		}
		port := int(data[at])<<8 | int(data[at+1])
		at += 2
		fmt.Fprintf(&sb, "a=candidate:%d 1 udp %d %s %d typ %s", i+1, candPriority[kind]-i, addr, port, candTypes[kind])
		if kind != candHost {
			sb.WriteString(" raddr 0.0.0.0 rport 0")
		}
		sb.WriteString(" generation 0\r\n")
	}
	sb.WriteString("a=end-of-candidates\r\n")
	sdpType := webrtc.SDPTypeOffer
	if typ == 1 {
		sdpType = webrtc.SDPTypeAnswer
	}
	return &webrtc.SessionDescription{Type: sdpType, SDP: sb.String()}, nil
}
