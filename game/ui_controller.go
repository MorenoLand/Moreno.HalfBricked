package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"strings"
	"sync"
)

// uiController is a parsed Common0/UserInterface/Controller/<Screen>.txt: the
// focus graph the native front end walks with the d-pad / arrow keys. Lines are
// either `Alias=Component.Path;` or `Node Up:Target Down:Target Left:Target
// Right:Target [IsDefault] [IsBack]`. IsDefault is the initially focused node,
// IsBack the node that is pressed by the back key.
type uiController struct {
	aliases map[string]string
	links   map[string]map[string]string
	def     string
	back    string
}

func parseUIController(text string) *uiController {
	c := &uiController{aliases: map[string]string{}, links: map[string]map[string]string{}}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if at := strings.Index(line, "="); at > 0 && !strings.Contains(line[:at], " ") {
			c.aliases[line[:at]] = strings.TrimSuffix(strings.TrimSpace(line[at+1:]), ";")
			continue
		}
		fields := strings.Fields(line)
		name := fields[0]
		if c.links[name] == nil {
			c.links[name] = map[string]string{}
		}
		for _, field := range fields[1:] {
			field = strings.TrimSuffix(field, ";") // the shipped files end node lines with ';'
			switch field {
			case "IsDefault":
				c.def = name
			case "IsBack":
				c.back = name
			default:
				if at := strings.Index(field, ":"); at > 0 {
					c.links[name][field[:at]] = field[at+1:]
				}
			}
		}
	}
	return c
}

// neighbour returns the node reached from `from` in direction dir ("Up",
// "Down", "Left", "Right").
func (c *uiController) neighbour(from, dir string) (string, bool) {
	if c == nil {
		return "", false
	}
	target, ok := c.links[from][dir]
	return target, ok && target != ""
}

var (
	uiControllerMu    sync.Mutex
	uiControllerCache = map[any]map[string]*uiController{}
)

// uiController loads a shipped controller file by screen name (nil when the
// pack has none, e.g. the older SD cache).
func (a *app) uiController(screen string) *uiController {
	if a == nil || a.pack == nil {
		return nil
	}
	uiControllerMu.Lock()
	defer uiControllerMu.Unlock()
	if cached, ok := uiControllerCache[a.pack][screen]; ok {
		return cached
	}
	var result *uiController
	if path, ok := a.pack.SourcePath("Common0/UserInterface/Controller/" + screen + ".txt"); ok {
		if reader, err := a.pack.Open(path); err == nil {
			data, _ := ioReadAll(reader)
			reader.Close()
			result = parseUIController(string(data))
		}
	}
	if uiControllerCache[a.pack] == nil {
		uiControllerCache[a.pack] = map[string]*uiController{}
	}
	uiControllerCache[a.pack][screen] = result
	return result
}

// mainMenuNodes maps MainScreen.txt button aliases to the port's main-menu
// actions (mainMenuButtons[i].action). Aliases with no port button (Achievements,
// MoreGames, News, Privacy, Credits) are absent: focus does not move onto them.
var mainMenuNodes = map[string]int{"PlayButton": 0, "OptionsButton": 2, "StatsButton": 3, "QuitButton": 4}

// mainMenuNavigate returns the main-menu button index reached from index in dir
// along MainScreen.txt's graph, or -1 when the native graph has no present
// target (or the controller file is missing).
func (a *app) mainMenuNavigate(index int, dir string) int {
	return mainMenuTarget(a.uiController("MainScreen"), index, dir)
}

func mainMenuTarget(controller *uiController, index int, dir string) int {
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
		target, ok := controller.neighbour(name, dir)
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
	if controller == nil || controller.back == "" {
		return -1
	}
	want, ok := mainMenuNodes[controller.back]
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
	case inpututil.IsKeyJustPressed(ebiten.KeyLeft):
		return "Left"
	case inpututil.IsKeyJustPressed(ebiten.KeyRight):
		return "Right"
	case inpututil.IsKeyJustPressed(ebiten.KeyUp):
		return "Up"
	case inpututil.IsKeyJustPressed(ebiten.KeyDown):
		return "Down"
	}
	return ""
}
