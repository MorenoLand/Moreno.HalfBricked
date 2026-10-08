package game

import (
	"fmt"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
)

// Boss fights. The native boss constructors launch a per-boss script when the
// boss spawns (scripts/dino_boss, egypt_boss, japan_boss, president_boss). The
// robot, western and gangster bosses are staged by their level entry scripts.
// A boss dying ends the level: waves stop, the remaining zombies are removed,
// the boss-end script (World 1 only) plays, and then the exit script.

// bossIntroScript names the script the native boss constructor launches.
func bossIntroScript(entityType, texture string) string {
	switch entityType {
	case "boss_rex":
		if strings.Contains(strings.ToLower(texture), "georgewashington") {
			return "president_boss.script"
		}
		return "dino_boss.script"
	case "boss_egyptian":
		return "egypt_boss.script"
	case "boss_samurai":
		return "japan_boss.script"
	}
	return ""
}

func bossEndScript(entityType, texture string) string {
	if entityType == "boss_rex" && bossIntroScript(entityType, texture) == "dino_boss.script" {
		return "dino_boss_end.script"
	}
	return ""
}

func isBossType(entityType string) bool { return strings.HasPrefix(entityType, "boss_") }

// onBossDefeated ends the wave schedule and removes the remaining zombies.
func (p *playState) onBossDefeated(entityType, texture string) {
	if p.bossDefeated {
		return
	}
	p.bossDefeated = true
	if p.world != nil {
		p.waveIndex = len(p.world.Level.Waves)
	}
	p.wavesFinished = true
	for index := range p.zombies {
		zombie := &p.zombies[index]
		if zombie.dying || zombie.health <= 0 {
			continue
		}
		if entity := p.scriptEntities[zombie.scriptID]; entity != nil && isBossType(entity.entityType) {
			continue
		}
		zombie.health, zombie.spawnAway = 0, true
	}
	if name := bossEndScript(entityType, texture); name != "" {
		p.bossScripts = append(p.bossScripts, name)
	}
}

// pauseScriptGame is PauseGame(): the simulation holds while the script keeps
// running, without opening the player's pause menu.
func (p *playState) pauseScriptGame(paused bool) {
	if paused == p.scriptPaused {
		return
	}
	p.scriptPaused = paused
	if paused {
		p.pausedControls = [2]bool{p.moveControl, p.shootControl}
		p.moveControl, p.shootControl = false, false
		return
	}
	p.moveControl, p.shootControl = p.pausedControls[0], p.pausedControls[1]
}

func (a *app) bossScriptSource(name string) (string, error) {
	suffix := "/scripts/" + strings.ToLower(name)
	for key := range a.pack.Manifest().Files {
		if strings.HasSuffix(strings.ToLower(key), suffix) {
			return a.pack.ScriptSource(key)
		}
	}
	return "", fmt.Errorf("boss script %q not found", name)
}

// updateBossScripts starts the next queued boss script once the current script
// has finished.
func (a *app) updateBossScripts() error {
	p := a.play
	if p == nil || len(p.bossScripts) == 0 || p.exitScriptStarted {
		return nil
	}
	if p.scriptRuntime != nil && !p.scriptRuntime.Done() {
		return nil
	}
	name := p.bossScripts[0]
	p.bossScripts = p.bossScripts[1:]
	source, err := a.bossScriptSource(name)
	if err != nil {
		return err
	}
	runtime, err := scripting.New(source, &playScriptHost{app: a, play: p}, scriptCallbacks)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	p.closeScript()
	p.scriptRuntime = runtime
	return nil
}

// bossPresent reports whether a live boss of this type is already on the field.
func (p *playState) bossPresent(entityType string) bool {
	for _, zombie := range p.zombies {
		entity := p.scriptEntities[zombie.scriptID]
		if entity != nil && entity.entityType == entityType && !zombie.dying && !zombie.spawnAway && zombie.health > 0 {
			return true
		}
	}
	return false
}
