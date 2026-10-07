package main

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
	"github.com/hajimehoshi/ebiten/v2"
	"os"
	"strings"
	"testing"
)

func TestScript125EntryRecordsAnalyticsAndWaits(t *testing.T) {
	source, err := os.ReadFile("bin/data-cache/source/World0/Scripts/world0_level0_entry.script")
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Split(string(source), "\n"); len(lines) < 103 || strings.TrimSpace(lines[6]) != "local g_isControllerAttached = HasControllerAttached()" || !strings.Contains(lines[102], "Analytics_SendEvent(\"tutorialStart\")") {
		t.Fatal("cache does not contain the expected 1.2.5 entry script")
	}
	host := script125CachedHost(t, "world0_level0")
	runtime := host.play.scriptRuntime
	err = host.play.updateScript()
	if err != nil || runtime.Status() != scripting.StatusSuspended || host.play.scriptLastCallback != "Idle" || !host.play.scriptWaitActive || len(host.analyticsEvents) != 1 || host.analyticsEvents[0].Event != "tutorialStart" || host.analyticsEvents[0].PlayerCount != 1 {
		t.Fatalf("entry error=%v, last callback=%s", err, host.play.scriptLastCallback)
	}
}
func TestScript125SinglePlayerGetters(t *testing.T) {
	host := &playScriptHost{app: &app{mobile: true}, play: &playState{x: 200, y: 224}}
	attached := len(ebiten.AppendGamepadIDs(nil)) > 0
	for _, test := range []struct {
		name string
		want scripting.Value
	}{{"HasControllerAttached", attached}, {"IsXPlayDevice", attached}, {"GetPlayer", 1}, {"GetPlayerX", float64(200)}, {"GetPlayerY", float64(224)}} {
		t.Run(test.name, func(t *testing.T) {
			result, err := host.Call(test.name, nil)
			if err != nil || result.Yield || len(result.Values) != 1 || result.Values[0] != test.want {
				t.Fatalf("result=%+v, error=%v, want=%v", result, err, test.want)
			}
			for _, arg := range []scripting.Value{nil, false, "1", 0, 1, 2} {
				if _, err := host.Call(test.name, []scripting.Value{arg}); err == nil {
					t.Fatalf("accepted unexpected argument %v", arg)
				}
			}
		})
	}
	runtime, err := scripting.New(`assert(type(HasControllerAttached()) == "boolean"); assert(HasControllerAttached() == IsXPlayDevice()); assert(GetPlayer() == 1); assert(GetPlayerX() == 200); assert(GetPlayerY() == 224)`, host, scriptCallbacks)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if err := runtime.Step(); err != nil || runtime.Status() != scripting.StatusComplete {
		t.Fatalf("getter Lua status=%v, error=%v", runtime.Status(), err)
	}
}
