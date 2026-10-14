package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"sync"

	"github.com/MorenoLand/Moreno.HalfBricked/game/focusgraph"
)

var (
	uiControllerMu    sync.Mutex
	uiControllerCache = map[any]map[string]*focusgraph.Graph{}
)

// uiController loads a shipped controller file by screen name (nil when the
// pack has none, e.g. the older SD cache).
func (a *app) uiController(screen string) *focusgraph.Graph {
	if a == nil || a.pack == nil {
		return nil
	}
	uiControllerMu.Lock()
	defer uiControllerMu.Unlock()
	if cached, ok := uiControllerCache[a.pack][screen]; ok {
		return cached
	}
	var result *focusgraph.Graph
	if path, ok := a.pack.SourcePath("Common0/UserInterface/Controller/" + screen + ".txt"); ok {
		if reader, err := a.pack.Open(path); err == nil {
			data, _ := ioReadAll(reader)
			reader.Close()
			result = focusgraph.Parse(string(data))
		}
	}
	if uiControllerCache[a.pack] == nil {
		uiControllerCache[a.pack] = map[string]*focusgraph.Graph{}
	}
	uiControllerCache[a.pack][screen] = result
	return result
}

// mainMenuNodes maps MainScreen.txt button aliases to the port's main-menu
// actions (mainMenuButtons[i].action). Aliases with no port button (Achievements,
// MoreGames, News, Privacy, Credits) are absent: focus does not move onto them.
var mainMenuNodes = map[string]int{"PlayButton": 0, "OptionsButton": 2, "StatsButton": 3, "QuitButton": 4, "AchievementsButton": menuActionAchievements}

// mainMenuNavigate returns the main-menu button index reached from index in dir
// along MainScreen.txt's graph, or -1 when the native graph has no present
// target (or the controller file is missing).
func (a *app) mainMenuNavigate(index int, dir string) int {
	target := mainMenuTarget(a.uiController("MainScreen"), index, dir)
	if target >= 0 && !a.mainMenuButtonAvailable(target) {
		return -1 // a group the loaded pack does not have
	}
	return target
}

func mainMenuTarget(controller *focusgraph.Graph, index int, dir string) int {
	if index < 0 || index >= len(mainMenuButtons) {
		return -1
	}
	if controller == nil {
		return -1
	}
	for name, action := range mainMenuNodes {
		if action != mainMenuButtons[index].action {
			continue
		}
		target, ok := controller.Neighbour(name, dir)
		if !ok {
			return -1
		}
		want, present := mainMenuNodes[target]
		if !present {
			return -1
		}
		for i, button := range mainMenuButtons {
			if button.action == want {
				return i
			}
		}
	}
	return -1
}

// mainMenuBackIndex is the button the back key presses (QuitButton IsBack).
func (a *app) mainMenuBackIndex() int {
	controller := a.uiController("MainScreen")
	if controller == nil || controller.Back == "" {
		return -1
	}
	want, ok := mainMenuNodes[controller.Back]
	if !ok {
		return -1
	}
	for i, button := range mainMenuButtons {
		if button.action == want {
			return i
		}
	}
	return -1
}

// mainMenuNavKey reports the arrow key pressed this frame as a controller
// direction. Up/Down with no present native target fall through to the port's
// linear Up/Down stepping (a convenience; the native graph has no such move).
func mainMenuNavKey() string {
	switch {
	case uiKeyJustPressed(ebiten.KeyLeft):
		return "Left"
	case uiKeyJustPressed(ebiten.KeyRight):
		return "Right"
	case uiKeyJustPressed(ebiten.KeyUp):
		return "Up"
	case uiKeyJustPressed(ebiten.KeyDown):
		return "Down"
	}
	return ""
}
