package game

import (
	"encoding/json"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/stats"
	"os"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
)

// The original catalog is loaded from every cache that is present (the 1.2.5
// cache bin/data-cache and the older bin/web/data/data-cache); each entry is
// then driven to its threshold through the real gameplay code and must unlock.

type achievementRig struct {
	app *app
	p   *playState
}

func achievementCaches() []string {
	var roots []string
	for _, root := range []string{"bin/data-cache", "bin/web/data/data-cache"} {
		if _, err := os.Stat(root); err == nil {
			roots = append(roots, root)
		}
	}
	return roots
}

func newAchievementRig(t *testing.T, catalog formats.AchievementCatalog, weapons formats.WeaponCatalog) *achievementRig {
	t.Helper()
	a := &app{achievements: catalog, weapons: weapons, statistics: stats.NewStatsData()}
	pistol, _ := weapons.Find("PISTOL")
	p := &playState{
		world: &viewer.Viewer{Level: formats.Level{Width: 64, Height: 64, Layers: map[formats.LayerKind][]uint32{formats.LayerC: openLayer(64 * 64)}}},
		x:     100, y: 200, tileSize: 32, health: 1, maxHealth: 1, lives: 3, weapons: weapons, weapon: pistol, statistics: &a.statistics,
	}
	a.play = p
	return &achievementRig{a, p}
}

// stack puts n one-hit zombies at a point so a single projectile or blast reaches them all.
func (r *achievementRig) stack(n int, x, y float64) {
	for i := 0; i < n; i++ {
		r.p.zombies = append(r.p.zombies, zombieState{x: x, y: y, health: 1, size: formats.Vec2{X: 32, Y: 32}})
	}
}
func (r *achievementRig) tick(frames int) {
	for i := 0; i < frames; i++ {
		r.p.updateMines()
		r.p.updateThrown()
		r.p.updateZombieBlasts()
		r.p.updateSentries()
		r.p.updateBulletsAndKills()
		if err := r.app.updateGameplayAchievements(r.p); err != nil {
			panic(err)
		}
	}
}
func (r *achievementRig) clear()                  { r.p.zombies = nil }
func (r *achievementRig) unlocked(id string) bool { return r.app.achievementUnlocks[id] }

// shootUntil fires the equipped primary weapon at stacked zombies until done reports true.
func (r *achievementRig) shootUntil(t *testing.T, perShot, frames, limit int, done func() bool) {
	t.Helper()
	for i := 0; i < limit && !done(); i++ {
		r.clear()
		r.stack(perShot, r.p.x+40, r.p.y)
		r.p.spinAudio.SpinTimer = 1
		switch r.p.weapon.GunType {
		case "BUZZSAW":
			// The saw fires from its own update, not the trigger.
			r.p.tickGun(false)
			r.p.tickGun(false)
		case "DUALPISTOL":
			// The dual pistol's timer gates the shot; let it elapse.
			r.p.tickGun(false)
			r.p.gun.Timer = 1
			fallthrough
		default:
			if !r.p.fire(1, 0) {
				t.Fatalf("shot %d refused (gun %s ammo %d)", i, r.p.weapon.GunType, r.p.weapon.Ammo)
			}
		}
		r.tick(frames)
	}
}

func driveKillAchievement(t *testing.T, r *achievementRig, entry formats.Achievement) {
	t.Helper()
	met := func() bool { return r.unlocked(entry.ID) }
	pickups := map[string]struct {
		pickup  string
		perShot int
	}{"shotgun": {"p_shotgun", 6}, "smg": {"p_uzi", 2}, "flamethrower": {"p_flamer", 2}, "minigun": {"p_minigun", 2}, "buzzsaw": {"p_buzzsaw", 10}, "dualpistol": {"p_dual_pistol", 2}}
	switch spec := entry.SpecificType; spec {
	case "shotgun", "smg", "flamethrower", "minigun", "buzzsaw", "dualpistol":
		if entry.Type == "KILLS_SHIELD" {
			r.p.collectPickup("p_shield")
		}
		r.p.collectPickup(pickups[spec].pickup)
		r.shootUntil(t, pickups[spec].perShot, 6, 400, met)
	case "rifle":
		r.p.collectPickup("p_sniper")
		r.clear()
		r.stack(entry.Total, r.p.x+90, r.p.y)
		r.p.fire(1, 0)
		r.tick(60)
	case "bazooka", "grenade", "cowpat":
		r.p.collectPickup(map[string]string{"bazooka": "p_bazooka", "grenade": "p_grenade", "cowpat": "p_cow_pat"}[spec])
		if spec == "bazooka" {
			r.stack(entry.Total, r.p.x+60, r.p.y)
		}
		if !r.p.fireSecondary(1, 0) {
			t.Fatal("secondary refused")
		}
		if spec != "bazooka" {
			// A bomb bounces to its own landing point; stack the zombies where it blows.
			for i := 0; i < 600 && len(r.p.zombieBlasts) == 0; i++ {
				r.p.updateThrown()
			}
			if len(r.p.zombieBlasts) == 0 {
				t.Fatal("the thrown bomb never detonated")
			}
			r.stack(entry.Total, r.p.zombieBlasts[0].x, r.p.zombieBlasts[0].y)
		}
		r.tick(120)
	case "mine":
		r.p.collectPickup("p_mine")
		if !r.p.fireSecondary(1, 0) {
			t.Fatal("mine refused")
		}
		r.stack(entry.Total, r.p.mines[0].x, r.p.mines[0].y)
		r.tick(120)
	case "sentry":
		r.p.collectPickup("p_sentry")
		if !r.p.fireSecondary(1, 0) {
			t.Fatal("sentry refused")
		}
		sentry := r.p.sentries[0]
		for i := 0; i < 6000 && !met() && len(r.p.sentries) > 0; i++ {
			if _, ok := r.p.nearestZombie(sentry.x, sentry.y, 1); !ok {
				r.clear()
				r.stack(6, sentry.x+60, sentry.y)
			}
			r.tick(1)
		}
	case "t_rex":
		// A real rex shockwave over a stack of one-hit zombies (rex_shockwave.go).
		r.stack(entry.Total, 500, 500)
		r.p.spawnRexShockwave(500, 500, 0)
		for i := 0; i < 70 && !met(); i++ {
			r.p.updateRexShockwaves()
			r.tick(1)
		}
	case "exp_zombie":
		for i := 0; i < entry.Total; i++ {
			r.p.creditKill(killOrigin{gun: "EXPZOMBIE", shot: 1})
		}
		if err := r.app.updateGameplayAchievements(r.p); err != nil {
			t.Fatal(err)
		}
	case "train":
		// A real pass of the train: the count is submitted when the pass ends.
		r.p.spawnTrain()
		for !r.p.train.active {
			r.p.updateTrain()
		}
		for i := 0; i < entry.Total; i++ {
			r.p.zombies = append(r.p.zombies, zombieState{x: r.p.train.x + float64(i), y: 520, health: 1})
		}
		r.tick(1)
		for r.p.train.active {
			r.p.updateTrain()
		}
		if err := r.app.updateGameplayAchievements(r.p); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("no driver for %s %s", entry.Type, spec)
	}
}

func TestEveryOriginalAchievementUnlocks(t *testing.T) {
	roots := achievementCaches()
	if len(roots) == 0 {
		t.Skip("original asset cache unavailable")
	}
	for _, root := range roots {
		pack, err := content.NewPack(content.NewSource(root))
		if err != nil {
			t.Fatal(err)
		}
		catalog, err := pack.Achievements()
		if err != nil || len(catalog) != 33 {
			t.Fatalf("%s: catalog %d entries, err %v", root, len(catalog), err)
		}
		weapons, err := pack.Weapons()
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range catalog {
			entry := entry
			t.Run(root+"/"+entry.Name, func(t *testing.T) {
				r := newAchievementRig(t, formats.AchievementCatalog{entry}, weapons)
				switch {
				case entry.Type == "KILLS" || entry.Type == "KILLS_SHIELD":
					driveKillAchievement(t, r, entry)
				case entry.Type == "SPECIFIC" && entry.SpecificType == "combo":
					r.p.collectPickup("p_uzi")
					r.shootUntil(t, 1, 8, 1000, func() bool { return r.unlocked(entry.ID) })
				default:
					driveCompletionAchievement(t, r, entry)
					return
				}
				if !r.unlocked(entry.ID) {
					t.Fatalf("%s (%s %s %d) did not unlock; best=%v", entry.Name, entry.Type, entry.SpecificType, entry.Total, r.p.achieve.best)
				}
				if !achievementTracked(entry) && entry.SpecificType != "exp_zombie" && entry.SpecificType != "train" {
					t.Fatalf("%s unlocks but is displayed UNTRACKED", entry.Name)
				}
			})
		}
	}
}

func storyInfo(id string, world int, flags ...string) formats.LevelInfo {
	return formats.LevelInfo{ID: id, WorldIndex: world, Flags: append([]string{"STORY"}, flags...)}
}

func driveCompletionAchievement(t *testing.T, r *achievementRig, entry formats.Achievement) {
	t.Helper()
	a, p := r.app, r.p
	check := func(want bool) {
		t.Helper()
		if a.achievementUnlocks[entry.ID] != want {
			t.Fatalf("%s unlocked=%v want %v", entry.Name, a.achievementUnlocks[entry.ID], want)
		}
	}
	switch entry.Type + "/" + entry.SpecificType {
	case "STORY/":
		a.awardLocalAchievements(&formats.LevelInfo{WorldIndex: entry.Total, Flags: []string{"STORY"}})
		check(false)
		a.awardLocalAchievements(&formats.LevelInfo{WorldIndex: entry.Total, Flags: []string{"STORY", "ENDWORLD"}})
		check(true)
	case "SPECIFIC/total":
		p.levelKills = entry.Total
		a.recordPlayStats(p, 0, p.lives)
		check(true)
	case "SPECIFIC/wave":
		// A two-wave looping survival table: the wave index never passes 1, the native wave counter keeps counting.
		a.mode, p.levelInfo.Flags = 1, []string{"SURVIVAL"}
		p.world.Level.Waves = []formats.Wave{{EndWaveTime: 100}, {EndWaveTime: 100, NextWave: 1}}
		for frame := 0; frame < 60*60 && !a.achievementUnlocks[entry.ID]; frame++ {
			if int(p.achieve.waveAdvances) == entry.Total-1 {
				check(false)
			}
			p.updateWaves()
			a.updateGameplayAchievements(p)
		}
		if p.waveIndex > 1 {
			t.Fatalf("looping table left index 1: %d", p.waveIndex)
		}
		if int(p.achieve.waveAdvances) != entry.Total {
			t.Fatalf("unlocked at wave counter %d, want %d", p.achieve.waveAdvances, entry.Total)
		}
		check(true)
	case "SPECIFIC/no_move":
		for i := 0; i < entry.Total; i++ {
			p.creditKill(killOrigin{gun: "PISTOL"})
		}
		a.updateGameplayAchievements(p)
		check(true)
	case "SPECIFIC/no_kills":
		for i := 0; i < (entry.Total+1)*60 && !a.achievementUnlocks[entry.ID]; i++ {
			a.updateGameplayAchievements(p)
		}
		check(true)
	case "SPECIFIC/pistol_only":
		r.clear()
		r.stack(1, p.x+40, p.y)
		p.fire(1, 0)
		r.tick(10)
		a.awardLocalAchievements(&formats.LevelInfo{ID: "l", Flags: []string{"STORY"}})
		check(true)
	case "SPECIFIC/accuracy":
		// Two chapter levels, three of four shots connect overall: exactly 75%.
		shoot := func(hits, misses int) {
			for shot := 0; shot < hits+misses; shot++ {
				r.clear()
				if shot < hits {
					r.stack(1, r.p.x+40, r.p.y)
				}
				r.p.fire(1, 0)
				r.tick(20)
			}
		}
		shoot(2, 0)
		a.awardLocalAchievements(&formats.LevelInfo{ID: "l1", WorldIndex: 0, Flags: []string{"STORY"}})
		check(false)
		r.nextLevel()
		shoot(1, 1)
		a.awardLocalAchievements(&formats.LevelInfo{ID: "l2", WorldIndex: 0, Flags: []string{"STORY", "ENDWORLD"}})
		check(true)
	case "SPECIFIC/continue":
		levels := []formats.LevelInfo{storyInfo("w0l0", 0, "BEGINSTORY"), storyInfo("w0l1", 0), storyInfo("w4l0", 4, "ENDWORLD", "SHOWCREDITS")}
		var prev *playState
		for _, level := range levels {
			a.noteAchievementLevelEntry(prev, level)
			prev = &playState{levelInfo: level}
		}
		a.play = prev
		a.awardLocalAchievements(&levels[2])
		check(true)
	case "SPECIFIC/death":
		a.levels = []formats.LevelInfo{storyInfo("a", 0), storyInfo("b", 1), storyInfo("s", 0, "SURVIVAL")}
		for _, level := range a.levels[:2] {
			a.play = &playState{levelInfo: level}
			a.awardLocalAchievements(&level)
			if level.ID == "a" {
				check(false)
			}
		}
		check(true)
	case "SPECIFIC/maddog":
		host := &playScriptHost{app: a, play: p}
		other := 1 - entry.Total
		if _, err := host.Call("UnlockWesternBossAchievement", []scripting.Value{float64(other)}); err != nil {
			t.Fatal(err)
		}
		check(false)
		if _, err := host.Call("UnlockWesternBossAchievement", []scripting.Value{float64(entry.Total)}); err != nil {
			t.Fatal(err)
		}
		check(true)
	default:
		t.Fatalf("no driver for %s/%s", entry.Type, entry.SpecificType)
	}
	if !achievementTracked(entry) {
		t.Fatalf("%s unlocks but is displayed UNTRACKED", entry.Name)
	}
}

func testRig(t *testing.T) *achievementRig {
	t.Helper()
	roots := achievementCaches()
	if len(roots) == 0 {
		t.Skip("original asset cache unavailable")
	}
	pack, err := content.NewPack(content.NewSource(roots[0]))
	if err != nil {
		t.Fatal(err)
	}
	weapons, err := pack.Weapons()
	if err != nil {
		t.Fatal(err)
	}
	return newAchievementRig(t, nil, weapons)
}

func TestPickupKillsDoNotCarryToNextPickup(t *testing.T) {
	r := testRig(t)
	r.p.collectPickup("p_shotgun")
	r.shootUntil(t, 6, 6, 4, func() bool { return false })
	first := r.p.achieve.best["shotgun"]
	if first < 20 || r.p.achieve.best["combo"] < int32(first) {
		t.Fatalf("first pickup best=%d combo=%d", first, r.p.achieve.best["combo"])
	}
	r.p.collectPickup("p_shotgun")
	r.shootUntil(t, 1, 6, 1, func() bool { return false })
	if r.p.achieve.best["shotgun"] != first || r.p.achieve.counts[scopeKey{"shotgun", r.p.achieve.pickupSerial}] != 1 {
		t.Fatal("new pickup continued the old counter")
	}
	if r.app.statistics.ShotgunKills != first || !r.app.statistics.Available["Kills with single Shotgun"] {
		t.Fatalf("stats best = %d", r.app.statistics.ShotgunKills)
	}
}

func TestSentryKillsAreNotPlayerKills(t *testing.T) {
	r := testRig(t)
	r.p.collectPickup("p_sentry")
	r.p.fireSecondary(1, 0)
	s := r.p.sentries[0]
	r.stack(1, s.x+60, s.y)
	// The turret needs .75 s to raise its head and then to turn toward the zombie.
	for i := 0; i < 600 && r.p.achieve.best["sentry"] == 0; i++ {
		r.tick(1)
	}
	if r.p.achieve.best["sentry"] != 1 || r.app.statistics.SentryGunKills != 1 || r.app.statistics.SentryGunsUsed != 1 || r.p.achieve.best["smg"] != 0 {
		t.Fatalf("sentry attribution: best=%v stats=%+v", r.p.achieve.best, r.app.statistics)
	}
}

func TestBuzzsawShieldKillsNeedShield(t *testing.T) {
	r := testRig(t)
	r.p.collectPickup("p_buzzsaw")
	r.shootUntil(t, 10, 6, 3, func() bool { return false })
	if r.p.achieve.shieldKills["buzzsaw"] != 0 || r.app.statistics.BuzzsawKills == 0 {
		t.Fatalf("unshielded kills counted: %v", r.p.achieve.shieldKills)
	}
	r.p.collectPickup("p_shield")
	r.shootUntil(t, 10, 6, 2, func() bool { return false })
	if r.p.achieve.shieldKills["buzzsaw"] == 0 || r.app.statistics.BuzzsawShieldKills == 0 {
		t.Fatal("shielded kills not counted")
	}
	r.p.damagePlayer(0, .5)
	if r.p.health != 1 {
		t.Fatal("shield did not block damage")
	}
	r.tick(int(shieldDuration*60) + 5)
	r.p.damagePlayer(0, .5)
	if r.p.health != .5 {
		t.Fatal("shield never expired")
	}
}

func openLayer(n int) []uint32 {
	layer := make([]uint32, n)
	for i := range layer {
		layer[i] = ^uint32(0)
	}
	return layer
}

// nextLevel replaces the play with a fresh one, as opening the next level does.
func (r *achievementRig) nextLevel() {
	old := r.p
	r.p = &playState{world: old.world, x: old.x, y: old.y, tileSize: 32, health: 1, maxHealth: 1, lives: 3, weapons: old.weapons, weapon: old.weapons[0], statistics: old.statistics}
	r.app.play = r.p
}

func TestSessionAchievementsRejectBrokenRuns(t *testing.T) {
	r := testRig(t)
	a := r.app
	a.achievements = formats.AchievementCatalog{
		{ID: "pistol", Type: "SPECIFIC", Check: "e", SpecificType: "pistol_only", Total: 1},
		{ID: "continue", Type: "SPECIFIC", Check: "e", SpecificType: "continue"},
		{ID: "death", Type: "SPECIFIC", Check: "e", SpecificType: "death"},
	}
	// A non-pistol shot spoils PEASHOOTER; a death spoils the level's death-free credit.
	r.p.collectPickup("p_shotgun")
	r.shootUntil(t, 1, 3, 1, func() bool { return false })
	r.p.achieve.died = true
	a.levels = []formats.LevelInfo{storyInfo("only", 0)}
	info := storyInfo("only", 0)
	a.awardLocalAchievements(&info)
	if len(a.achievementUnlocks) != 0 {
		t.Fatalf("spoiled run unlocked %v", a.achievementUnlocks)
	}
	// Replaying the level without dying earns the death-free credit (replays are allowed).
	r.nextLevel()
	a.awardLocalAchievements(&info)
	if !a.achievementUnlocks["death"] || a.achievementUnlocks["pistol"] {
		t.Fatalf("replay unlocks = %v", a.achievementUnlocks)
	}
	// Retrying or returning through the main menu breaks the one-sitting story run.
	begin, end := storyInfo("a", 0, "BEGINSTORY"), storyInfo("z", 4, "ENDWORLD", "SHOWCREDITS")
	for name, entry := range map[string]func(){
		"retry":     func() { a.noteAchievementLevelEntry(&playState{levelInfo: begin}, begin) },
		"main menu": func() { a.noteAchievementLevelEntry(nil, end) },
	} {
		a.noteAchievementLevelEntry(nil, begin)
		entry()
		a.play = &playState{levelInfo: end}
		a.awardLocalAchievements(&end)
		if a.achievementUnlocks["continue"] {
			t.Fatalf("%s still awarded UNTOUCHABLE", name)
		}
	}
}

func TestDeathFreeLevelsPersist(t *testing.T) {
	a := &app{achieveSession: achievementSession{deathFree: map[string]bool{"w0l0": true}}}
	data, err := json.Marshal(playerProfile{DeathFreeLevels: a.achieveSession.deathFree})
	if err != nil {
		t.Fatal(err)
	}
	var restored playerProfile
	if err := json.Unmarshal(data, &restored); err != nil || !restored.DeathFreeLevels["w0l0"] {
		t.Fatal("death-free ledger lost")
	}
}
