package main

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
	"testing"
)

func TestNativeScriptTitlePair(t *testing.T) {
	p := &playState{}
	h := &playScriptHost{play: p}
	for _, call := range []struct {
		name string
		args []scripting.Value
	}{{"DrawText1", []scripting.Value{240, 20, "SHOOT IT UP IN DINOSAUR TIMES,"}}, {"DrawText2", []scripting.Value{240, 40, "STEAKFRIES!"}}} {
		if _, err := h.Call(call.name, call.args); err != nil {
			t.Fatal(err)
		}
	}
	if p.scriptText1Y != 120 || p.scriptText2Y != 151 || p.scriptText1Size != 30 || p.scriptText2Size != 30 {
		t.Fatalf("native title pair: y=%v,%v sizes=%v,%v", p.scriptText1Y, p.scriptText2Y, p.scriptText1Size, p.scriptText2Size)
	}
}
func TestNativeScriptSmallSecondLineUsesFirstBaseline(t *testing.T) {
	p := &playState{}
	h := &playScriptHost{play: p}
	if _, err := h.Call("DrawText1", []scripting.Value{240, 20, "ONE EVIL DUDE", false}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Call("DrawText2", []scripting.Value{240, 999, "PROFESSOR BRAINS", false}); err != nil {
		t.Fatal(err)
	}
	if p.scriptText2Y != 44 || p.scriptText2Size != 24 {
		t.Fatalf("second line y=%v size=%v", p.scriptText2Y, p.scriptText2Size)
	}
}
func TestNativeInactiveScriptRestoresGameplayControls(t *testing.T) {
	p := &playState{moveControl: false, shootControl: false}
	if !p.enterAfterScript() || !p.moveControl || !p.shootControl {
		t.Fatal("inactive script must permit movement and aiming")
	}
}
func TestResetExternalDoesNotInventMoveControl(t *testing.T) {
	p := &playState{moveControl: false, shootControl: false, scriptTextVisible: true, scriptText1: "title"}
	h := &playScriptHost{play: p}
	if _, err := h.Call("ResetExternal", nil); err != nil {
		t.Fatal(err)
	}
	if p.moveControl || p.shootControl || p.scriptTextVisible {
		t.Fatal("reset must reset display without enabling controls")
	}
}
