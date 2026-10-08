package game

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
		if a.mode == 1 && a.play != nil && a.play.health <= 0 && a.play.lives <= 0 && a.play.allPlayersDown() {
			a.openLevelResults()
		}
		return nil
	}
	if a.play.exitScriptStarted {
		if a.play.scriptRuntime != nil && a.play.scriptRuntime.Done() {
			if err := a.recordStoryCompletion(a.play.levelInfo); err != nil {
				return err
			}
			if resultsVisible(false, nativeCatalogLevelFlags(a.play.levelInfo.Flags)) {
				a.openLevelResults()
				return nil
			}
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
	host := &playScriptHost{app: a, play: a.play}
	var runtime *scripting.Runtime
	if a.play.scriptRuntime != nil {
		// The native VM is shared: exit scripts call helpers the entry script defined.
		runtime, err = a.play.scriptRuntime.Successor(source, host, scriptCallbacks)
		if err == nil {
			a.play.scriptRuntime = nil
		}
	} else {
		runtime, err = scripting.New(source, host, scriptCallbacks)
	}
	if err != nil {
		return fmt.Errorf("level exit %s: %w", path, err)
	}
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
	if err := a.recordStoryCompletion(info); err != nil {
		return err
	}
	score := a.play.score
	combatKills := a.play.combatKillCount
	a.play.closeScript()
	a.world, a.level = world, level
	if err := a.openPlay(); err != nil {
		return err
	}
	a.play.score, a.play.levelStartScore = score, score
	a.play.combatKillCount = combatKills
	return nil
}
func (a *app) recordStoryCompletion(info formats.LevelInfo) error {
	a.awardLocalAchievements(&info)
	if a.unlocked == nil {
		a.unlocked = map[string]bool{}
	}
	if info.NextLevel != "" {
		a.unlocked[info.NextLevel] = true
	}
	for _, id := range info.UnlockLevels {
		if id != "" {
			a.unlocked[id] = true
		}
	}
	return a.savePlayerProfile()
}
