package game

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
)

func TestFOVScaleKeepsTheOriginalViewAtNinety(t *testing.T) {
	if (extraSettings{}).fovZoomScale() != 1 || (extraSettings{FOV: 90}).fovZoomScale() != 1 {
		t.Fatal("90 (and an unset profile value) must be the original zoom")
	}
	if got := (extraSettings{FOV: 120}).fovZoomScale(); got != .75 {
		t.Fatalf("FOV 120 zoom scale %v, want .75 (more of the level)", got)
	}
	if got := (extraSettings{FOV: 10}).fov(); got != fovMin {
		t.Fatalf("FOV clamps to %d, got %d", fovMin, got)
	}
}

func TestFOVScaleSeparatesTheGameZoomFromTheSetting(t *testing.T) {
	v := &viewer.Viewer{Zoom: 1}
	v.SetZoomScale(.75)
	if v.Zoom != .75 || v.BaseZoom() != 1 {
		t.Fatalf("zoom %v base %v", v.Zoom, v.BaseZoom())
	}
	v.SetZoom(2) // a script zoom keeps the player's FOV on top
	if v.Zoom != 1.5 || v.BaseZoom() != 2 {
		t.Fatalf("after a script zoom: zoom %v base %v", v.Zoom, v.BaseZoom())
	}
	v.SetZoomScale(1)
	if v.Zoom != 2 {
		t.Fatalf("back at 90: zoom %v", v.Zoom)
	}
}

func TestExtrasSurviveTheProfile(t *testing.T) {
	data, err := json.Marshal(playerProfile{Extras: &extraSettings{FOV: 105, DiscordRPC: false}})
	if err != nil {
		t.Fatal(err)
	}
	var profile playerProfile
	if err := json.Unmarshal(data, &profile); err != nil || profile.Extras == nil || profile.Extras.FOV != 105 || profile.Extras.DiscordRPC {
		t.Fatalf("round trip %+v err %v", profile.Extras, err)
	}
}

func TestDiscordFramesUseTheIPCLayout(t *testing.T) {
	var buf bytes.Buffer
	if err := discordWriteFrame(&buf, 1, map[string]any{"cmd": "SET_ACTIVITY"}); err != nil {
		t.Fatal(err)
	}
	raw := buf.Bytes()
	if binary.LittleEndian.Uint32(raw[0:]) != 1 || int(binary.LittleEndian.Uint32(raw[4:])) != len(raw)-8 {
		t.Fatalf("bad header % x", raw[:8])
	}
	opcode, payload, err := discordReadFrame(bytes.NewReader(raw))
	if err != nil || opcode != 1 || !bytes.Contains(payload, []byte("SET_ACTIVITY")) {
		t.Fatalf("read back %d %s %v", opcode, payload, err)
	}
	var out bytes.Buffer
	if err := discordSetActivity(&rwc{r: bytes.NewReader(raw), w: &out}, "Level", "Score 5", time.Unix(1700000000, 0)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"details":"Level"`)) || !bytes.Contains(out.Bytes(), []byte(`"name":"Age of Zombies"`)) || !bytes.Contains(out.Bytes(), []byte(`"start":1700000000000`)) {
		t.Fatalf("activity frame %s", out.Bytes())
	}
}

type rwc struct {
	r io.Reader
	w *bytes.Buffer
}

func (c *rwc) Read(p []byte) (int, error)  { return c.r.Read(p) }
func (c *rwc) Write(p []byte) (int, error) { return c.w.Write(p) }
func (c *rwc) Close() error                { return nil }
