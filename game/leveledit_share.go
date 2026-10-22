package game

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/game/platform"
)

// Sharing a level and keeping it: the level code (the whole level as one line of text, to send to someone), the custom
// folder (levels the game itself lists in level select), opening files and codes, and the autosave.

const (
	levelCodePrefix   = "AOZLEVEL1:"
	autosaveName      = "editor/autosave.json"
	autosaveEvery     = 60 * time.Second
	customLevelFolder = "custom"
)

// ---- the level code ------------------------------------------------------------------------------------------------

func encodeLevelCode(level formats.Level) (string, error) {
	data, err := json.Marshal(level)
	if err != nil {
		return "", err
	}
	var packed bytes.Buffer
	writer, _ := gzip.NewWriterLevel(&packed, gzip.BestCompression)
	if _, err := writer.Write(data); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	return levelCodePrefix + base64.RawURLEncoding.EncodeToString(packed.Bytes()), nil
}

// parseLevelData reads a level from the bytes of a file (JSON) or from a level code.
func parseLevelData(data []byte) (formats.Level, error) {
	text := strings.TrimSpace(string(data))
	var raw []byte
	if strings.HasPrefix(text, levelCodePrefix) {
		packed, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(text, levelCodePrefix))
		if err != nil {
			return formats.Level{}, fmt.Errorf("the level code is damaged (%v)", err)
		}
		reader, err := gzip.NewReader(bytes.NewReader(packed))
		if err != nil {
			return formats.Level{}, fmt.Errorf("the level code is damaged (%v)", err)
		}
		raw, err = io.ReadAll(io.LimitReader(reader, 64<<20))
		if err != nil {
			return formats.Level{}, fmt.Errorf("the level code is damaged (%v)", err)
		}
	} else {
		raw = []byte(text)
	}
	var level formats.Level
	if err := json.Unmarshal(raw, &level); err != nil {
		return formats.Level{}, fmt.Errorf("this is not a level: %v", err)
	}
	if err := level.Validate(); err != nil {
		return formats.Level{}, err
	}
	return level, nil
}

// ---- opening and pasting -------------------------------------------------------------------------------------------

// editorAccept makes a level that was read from a file or a code the one being edited.
func (a *app) editorAccept(level formats.Level, origin string) {
	e := a.editor
	if level.Info.ID == "" {
		level.Info.ID = "Custom_" + time.Now().Format("0102_1504")
	}
	if level.Info.DisplayName == "" {
		level.Info.DisplayName = level.Info.ID
	}
	level.Info.BaseFile = "custom_" + strings.ToLower(level.Info.ID)
	level.WaveBuild = a.pack.Manifest().WaveBuild
	if err := a.editorSetLevel(level, ""); err != nil {
		e.say(err.Error())
		return
	}
	e.savedPath = origin
	e.say("Opened " + level.Info.ID)
}

func (a *app) editorOpenFile() {
	e := a.editor
	if e.pickWait {
		return
	}
	e.pickWait = true
	platform.PickFileAsync("Open a level", "Level (*.json)", "*.json")
}

func (a *app) editorPasteCode() {
	e := a.editor
	e.pasteWait, e.pasteUntil = true, time.Now().Add(12*time.Second)
	platform.ClipboardRequest()
	e.say("Reading the clipboard…")
}

func (a *app) editorCopyCode() {
	e := a.editor
	code, err := encodeLevelCode(e.level)
	if err != nil {
		e.say("Could not make the code: " + err.Error())
		return
	}
	platform.ClipboardWrite(code)
	e.say(fmt.Sprintf("Level code copied (%d characters). Anyone can paste it into the editor's File menu.", len(code)))
}

// editorPoll collects a chosen file or a pasted code.
func (a *app) editorPoll() {
	e := a.editor
	if e.pickWait {
		if name, data, ok := platform.PickedFile(); ok {
			e.pickWait = false
			if level, err := parseLevelData(data); err != nil {
				e.say(err.Error())
			} else {
				a.editorAccept(level, name)
			}
		}
	}
	if e.pasteWait {
		if text, ok := platform.ClipboardTake(); ok {
			e.pasteWait = false
			level, err := parseLevelData([]byte(text))
			if err != nil {
				e.say("The clipboard does not hold a level: " + err.Error())
			} else {
				a.editorAccept(level, "")
			}
		} else if time.Now().After(e.pasteUntil) {
			e.pasteWait = false
			e.say("Nothing arrived from the clipboard")
		}
	}
}

// editorArm tells the browser build where a real click may open the file chooser or read the clipboard. It is set while
// drawing (what is under the pointer) and applied at the start of the next update.
func (a *app) editorArm(fileRegion [4]float64, pasteHover bool) {
	e := a.editor
	e.armFile, e.armPaste = fileRegion, pasteHover
}

func (a *app) editorApplyArms() {
	e := a.editor
	if e.armFile[2] > e.armFile[0] {
		platform.FilePickArm(e.armFile[0]/editorW, e.armFile[1]/editorH, e.armFile[2]/editorW, e.armFile[3]/editorH)
	} else {
		platform.FilePickArm(0, 0, 0, 0)
	}
	if e.pasteWait {
		platform.ClipboardArm(false, true) // Ctrl+V works while the editor waits for a code
	} else if e.armPaste {
		platform.ClipboardArm(true, false)
	} else if e.armedPaste {
		platform.ClipboardArm(false, false)
	}
	e.armedPaste = e.armPaste || e.pasteWait
	e.armFile, e.armPaste = [4]float64{}, false
}

// ---- the custom folder ---------------------------------------------------------------------------------------------

var customIDUnsafe = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

// customLevelID makes a level id that is safe as a file name and never the id of one of the game's own levels.
func (a *app) customLevelID(raw string) string {
	id := customIDUnsafe.ReplaceAllString(strings.TrimSpace(raw), "_")
	if id == "" {
		id = "Custom_" + time.Now().Format("0102_1504")
	}
	if !a.customLevels[id] {
		for _, info := range a.levels {
			if strings.EqualFold(info.ID, id) {
				id = "Custom_" + id
				break
			}
		}
	}
	return id
}

// registerCustom lists a level in level select (and remembers it as a custom one).
func (a *app) registerCustom(level formats.Level) formats.Level {
	level.Info.ID = a.customLevelID(level.Info.ID)
	level.Info.BaseFile = "custom_" + strings.ToLower(level.Info.ID)
	if level.Info.DisplayName == "" {
		level.Info.DisplayName = level.Info.ID
	}
	level.Info.NextLevel = strings.TrimSpace(level.Info.NextLevel)
	hasFlag := func(name string) bool {
		for _, flag := range level.Info.Flags {
			if flag == name {
				return true
			}
		}
		return false
	}
	if !hasFlag("STORY") && !hasFlag("SURVIVAL") {
		level.Info.Flags = append(level.Info.Flags, "STORY")
	}
	if !hasFlag("STARTUNLOCKED") {
		level.Info.Flags = append(level.Info.Flags, "STARTUNLOCKED")
	}
	validWorld := false
	for _, world := range a.worlds() {
		if world == level.Info.WorldIndex {
			validWorld = true
		}
	}
	if !validWorld {
		level.Info.WorldIndex = 0
	}
	a.pack.RegisterLevel(level)
	replaced := false
	for index, info := range a.levels {
		if info.ID == level.Info.ID {
			a.levels[index], replaced = level.Info, true
		}
	}
	if !replaced {
		a.levels = append(a.levels, level.Info)
	}
	if a.customLevels == nil {
		a.customLevels = map[string]bool{}
	}
	a.customLevels[level.Info.ID] = true
	if a.unlocked != nil {
		a.unlocked[level.Info.ID] = true
	}
	return level
}

// loadCustomLevels lists the level files of the custom folder in level select.
func (a *app) loadCustomLevels() {
	dir := content.CustomDir()
	if dir == "" {
		return
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return
	}
	sort.Strings(files)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		level, err := parseLevelData(data)
		if err != nil {
			fmt.Fprintln(os.Stderr, "custom level", file, "skipped:", err)
			continue
		}
		if level.Info.ID == "" {
			level.Info.ID = strings.TrimSuffix(filepath.Base(file), ".json")
		}
		if _, err := a.editorTilesetFor(level); err != nil {
			fmt.Fprintln(os.Stderr, "custom level", file, "skipped:", err)
			continue
		}
		a.registerCustom(level)
	}
}

func (a *app) editorTilesetFor(level formats.Level) (formats.TileSet, error) {
	tileset, ok := a.pack.Manifest().TileSets[strings.ToLower(level.Tileset)]
	if !ok {
		return formats.TileSet{}, fmt.Errorf("tileset %q not found", level.Tileset)
	}
	return tileset, nil
}

// editorAddToGame puts the level in the custom folder and in level select, so it can be played like any other.
func (a *app) editorAddToGame() {
	e := a.editor
	if err := e.level.Validate(); err != nil {
		e.say(err.Error())
		return
	}
	level := copyLevel(e.level)
	level = a.registerCustom(level)
	e.level.Info.ID, e.level.Info.BaseFile = level.Info.ID, level.Info.BaseFile
	e.level.Info.WorldIndex, e.level.Info.Flags = level.Info.WorldIndex, level.Info.Flags
	where := fmt.Sprintf("World %d", level.Info.WorldIndex+1)
	dir := content.CustomDir()
	if dir == "" {
		e.say("Added to " + where + " for this visit (a browser cannot keep files: use Save as to keep it)")
		return
	}
	data, err := json.Marshal(level)
	if err == nil {
		err = os.MkdirAll(dir, 0o755)
	}
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, level.Info.ID+".json"), data, 0o644)
	}
	if err != nil {
		e.say("Added to the game, but the file could not be written: " + err.Error())
		return
	}
	e.say("Added to " + where + " in level select. File: " + filepath.Join(dir, level.Info.ID+".json"))
}

func (a *app) editorRemoveFromGame() {
	e := a.editor
	id := e.level.Info.ID
	if !a.customLevels[id] {
		e.say("This level is not in the game's level select")
		return
	}
	if dir := content.CustomDir(); dir != "" {
		_ = os.Remove(filepath.Join(dir, id+".json"))
	}
	a.pack.UnregisterLevel(id)
	kept := a.levels[:0]
	for _, info := range a.levels {
		if info.ID != id {
			kept = append(kept, info)
		}
	}
	a.levels = kept
	delete(a.customLevels, id)
	e.say("Removed " + id + " from level select")
}

// ---- autosave ------------------------------------------------------------------------------------------------------

// autosave keeps a copy of the level being edited, so a crash or a closed window cannot lose it. Best effort.
func (e *levelEditor) autosave() {
	data, err := json.Marshal(e.level)
	if err != nil {
		return
	}
	platform.ConfigWrite(autosaveName, data)
	e.lastAutosave = time.Now()
}

// editorAutosaveTick saves a minute after the last change, while there are unsaved changes.
func (a *app) editorAutosaveTick() {
	e := a.editor
	if e.dirty && e.view != nil && time.Since(e.lastAutosave) > autosaveEvery {
		e.autosave()
	}
}

func (a *app) editorRecoverAutosave() {
	e := a.editor
	data, ok := platform.ConfigRead(autosaveName)
	if !ok {
		e.say("There is no autosave to recover")
		return
	}
	level, err := parseLevelData(data)
	if err != nil {
		e.say("The autosave could not be read: " + err.Error())
		return
	}
	a.editorAccept(level, "")
	e.dirty = true
	e.say("Recovered the autosave of " + level.Info.ID)
}

func hasAutosave() bool {
	_, ok := platform.ConfigRead(autosaveName)
	return ok
}

// ---- the File menu -------------------------------------------------------------------------------------------------

func (a *app) openFileMenu(x float64) {
	e := a.editor
	items := []edMenuItem{
		{label: "Open a level file…", icon: "open", sub: "Ctrl+O", tip: "Open a level you saved as a file", armFile: true, do: a.editorOpenFile},
		{label: "Save as a file…", icon: "save", sub: "Ctrl+S", tip: "Save the level as a file, anywhere you like", do: a.editorSaveAs},
		{label: "Save over the cache level", icon: "stack", sub: "Ctrl+Shift+S", tip: "Overwrite this level in the game's own files; the original is kept as .orig", disabled: e.sourceID == "", do: a.editorSaveToCache},
		{label: "Share and play", header: true},
		{label: "Add to the game", icon: "play", tip: "List this level in level select so you can play it without the editor", do: a.editorAddToGame},
		{label: "Remove from the game", icon: "close", tip: "Take this level out of level select again", disabled: !a.customLevels[e.level.Info.ID], do: a.editorRemoveFromGame},
		{label: "Copy the level code", icon: "copy", tip: "One line of text with the whole level: paste it to someone", do: a.editorCopyCode},
		{label: "Paste a level code", icon: "paste", tip: "Open a level from a code on the clipboard", armPaste: true, do: a.editorPasteCode},
		{label: "Keyboard shortcuts…", icon: "details", tip: "Choose which keys do what", do: func() { e.shortcutsOpen = true }},
		{label: "Safety", header: true},
		{label: "Recover the autosave", icon: "history", tip: "Open the copy the editor kept automatically", disabled: !hasAutosave(), do: a.editorRecoverAutosave},
	}
	a.openPopup("File", x, 340, items)
}
