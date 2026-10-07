package main

import (
	"encoding/json"
	"fmt"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
	"log"
	"math"
)

type scriptAnalyticsEvent struct {
	Event       string `json:"event"`
	PlayerCount int    `json:"PLAYER_COUNT"`
	Level       string `json:"level"`
}
type script125UI interface {
	DisplayScreen(string) error
	PlayAnimation(string, string) error
	BoolProperty(string, string) (bool, bool, error)
	RemoveScreen(string) error
	SetupTutorialScreen() error
	ControllerEnterScreen(string) error
	ControllerLeaveScreen(string) error
}

func (h *playScriptHost) callScript125(name string, args []scripting.Value) (scripting.CallResult, error) {
	want := 1
	switch name {
	case "BrickUI_PlayAnimation":
		want = 2
	case "BrickUI_GetPropertyBool":
		want = 3
	case "BrickUI_SetupTutorialScreen":
		want = 0
	}
	if len(args) != want {
		return scripting.CallResult{}, fmt.Errorf("%s expects %d arguments, got %d", name, want, len(args))
	}
	switch name {
	case "Analytics_SendEvent":
		event, err := scriptString(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		record := scriptAnalyticsEvent{Event: event, PlayerCount: 1, Level: h.play.levelInfo.ID}
		if record.Level == "" && h.play.world != nil {
			record.Level = h.play.world.Level.Info.ID
		}
		payload, err := json.Marshal(record)
		if err != nil {
			return scripting.CallResult{}, err
		}
		h.analyticsEvents = append(h.analyticsEvents, record)
		log.Printf("script analytics offline: %s", payload)
		return scripting.CallResult{}, nil
	case "SecondaryPlayersPleaseWait":
		wait, err := scriptBool(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		h.secondaryPlayersWait = wait
		return scripting.CallResult{}, nil
	case "SetZombiesActiveDuringScripts":
		active, err := scriptNumber(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		if math.IsNaN(active) || math.IsInf(active, 0) || math.Trunc(active) != active {
			return scripting.CallResult{}, fmt.Errorf("zombie activity must be a finite integer")
		}
		h.play.scriptZombiesActive = active != 0
		return scripting.CallResult{}, nil
	}
	var screen, property string
	var fallback bool
	var err error
	if want > 0 {
		screen, err = scriptString(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
	}
	if want > 1 {
		property, err = scriptString(args, 1)
		if err != nil {
			return scripting.CallResult{}, err
		}
	}
	if want > 2 {
		fallback, err = scriptBool(args, 2)
		if err != nil {
			return scripting.CallResult{}, err
		}
	}
	if h.ui == nil {
		return scripting.CallResult{}, fmt.Errorf("%s requires the parent BrickUI screen/property/animation and controller-action host for %q", name, screen)
	}
	switch name {
	case "BrickUI_DisplayScreen":
		err = h.ui.DisplayScreen(screen)
	case "BrickUI_PlayAnimation":
		err = h.ui.PlayAnimation(screen, property)
	case "BrickUI_GetPropertyBool":
		value, found, err := h.ui.BoolProperty(screen, property)
		if err != nil {
			return scripting.CallResult{}, err
		}
		if !found {
			value = fallback
		}
		return scriptValues(value), nil
	case "BrickUI_RemoveScreen":
		err = h.ui.RemoveScreen(screen)
	case "BrickUI_SetupTutorialScreen":
		err = h.ui.SetupTutorialScreen()
	case "Controller_OnEnterScreen":
		err = h.ui.ControllerEnterScreen(screen)
	case "Controller_OnLeaveScreen":
		err = h.ui.ControllerLeaveScreen(screen)
	default:
		err = fmt.Errorf("unsupported 1.2.5 callback %q", name)
	}
	return scripting.CallResult{}, err
}
