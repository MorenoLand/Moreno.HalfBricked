package main

import (
	"fmt"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
	"path/filepath"
	"strings"
)

func exitScriptPath(info formats.LevelInfo) string {
	parts := strings.Split(filepath.ToSlash(info.SourceXML), "/")
	if len(parts) == 0 || parts[0] == "" || info.BaseFile == "" {
		return ""
	}
	return parts[0] + "/Scripts/" + info.BaseFile + "_exit.script"
}
func (p *playState) readyForExitScript() bool {
	return p != nil && p.health > 0 && p.wavesFinished && !p.exitScriptStarted && len(p.zombies) == 0 && (p.scriptRuntime == nil || p.scriptRuntime.Done())
}
func (a *app) updateLevelCompletion() error {
	if a.mode != 0 || a.play == nil {
		return nil
	}
	if a.play.exitScriptStarted {
		if a.play.scriptRuntime != nil && a.play.scriptRuntime.Done() && !hasLevelFlag(a.play.levelInfo, "ENDWORLD") && !hasLevelFlag(a.play.levelInfo, "ENDSTORY") {
			return a.continueStoryLevel(a.play.levelInfo)
		}
		return nil
	}
	if !a.play.readyForExitScript() {
		return nil
	}
	path := exitScriptPath(a.play.levelInfo)
	source, err := a.pack.ScriptSource(path)
	if err != nil {
		return fmt.Errorf("level exit %s: %w", path, err)
	}
	runtime, err := scripting.New(source, &playScriptHost{app: a, play: a.play}, scriptCallbacks)
	if err != nil {
		return fmt.Errorf("level exit %s: %w", path, err)
	}
	a.play.closeScript()
	a.play.scriptRuntime, a.play.exitScriptStarted = runtime, true
	return nil
}
func (a *app) storyLevelSelection(id string) (int, int, error) {
	for _, target := range a.levels {
		if !strings.EqualFold(target.ID, id) || hasLevelFlag(target, "SURVIVAL") {
			continue
		}
		worldIndex := -1
		for index, world := range a.worlds() {
			if world == target.WorldIndex {
				worldIndex = index
				break
			}
		}
		if worldIndex < 0 {
			break
		}
		levelIndex := 0
		for _, item := range a.levels {
			if item.WorldIndex != target.WorldIndex || hasLevelFlag(item, "SURVIVAL") {
				continue
			}
			if strings.EqualFold(item.ID, target.ID) {
				return worldIndex, levelIndex, nil
			}
			levelIndex++
		}
	}
	return 0, 0, fmt.Errorf("story continuation level %q not found", id)
}
func (a *app) continueStoryLevel(info formats.LevelInfo) error {
	world, level, err := a.storyLevelSelection(info.NextLevel)
	if err != nil {
		return err
	}
	score := a.play.score
	a.play.closeScript()
	a.world, a.level = world, level
	if err := a.openPlay(); err != nil {
		return err
	}
	a.play.score, a.play.levelStartScore = score, score
	if a.unlocked == nil {
		a.unlocked = map[string]bool{}
	}
	a.unlocked[info.NextLevel] = true
	for _, id := range info.UnlockLevels {
		a.unlocked[id] = true
	}
	return nil
}
