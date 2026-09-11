package main

import (
	"image"
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
)

func TestPortalCellMatchesNativeBounds(t *testing.T) {
	for _, test := range []struct {
		x, y         float64
		wantX, wantY int
	}{
		{-1, -1, 0, 0},
		{364, 530, 5, 8},
		{730, 660, 11, 10},
		{9999, 9999, 0x22, 0x12},
	} {
		gotX, gotY := portalCell(test.x, test.y)
		if gotX != test.wantX || gotY != test.wantY {
			t.Fatalf("portalCell(%.1f,%.1f)=(%d,%d), want (%d,%d)", test.x, test.y, gotX, gotY, test.wantX, test.wantY)
		}
	}
}

func TestButtonTextRectsUseNativeAtlasRows(t *testing.T) {
	got, ok := buttonTextRect(6)
	if !ok || got != image.Rect(0, 96, 128, 112) {
		t.Fatalf("quit text rect = %v, want (0,96)-(128,112)", got)
	}
}

func TestPortalUsesNativeCloseAndRemovalCadence(t *testing.T) {
	play := &playState{portals: []portalState{{age: 2.0, size: 140, animationTimer: 100}}}
	for frame := 0; frame < 36; frame++ {
		play.updatePortals()
	}
	if len(play.portals) != 1 || play.portals[0].size != 140 {
		t.Fatalf("portal at close threshold = count %d size %.3f, want count 1 size 140", len(play.portals), play.portals[0].size)
	}
	play.updatePortals()
	if len(play.portals) != 1 || play.portals[0].size >= 140 {
		t.Fatalf("portal after native close start = count %d size %.3f, want count 1 and shrinking size", len(play.portals), play.portals[0].size)
	}
	for len(play.portals) > 0 {
		play.updatePortals()
	}
	if len(play.portals) != 0 {
		t.Fatalf("portal slice after removal = %#v, want empty", play.portals)
	}
}

func TestPortalUsesNativeAnimationTimer(t *testing.T) {
	play := &playState{portals: []portalState{{animationTimer: 100}}}
	for frame := 0; frame < 6; frame++ {
		play.updatePortals()
	}
	if play.portals[0].frame != 0 || play.portals[0].animationTimer != -1 {
		t.Fatalf("portal after six native timer ticks = frame %d timer %.1f, want frame 0 timer -1", play.portals[0].frame, play.portals[0].animationTimer)
	}
	play.updatePortals()
	if play.portals[0].frame != 1 || play.portals[0].animationTimer != 100 {
		t.Fatalf("portal after native frame advance = frame %d timer %.1f, want frame 1 timer 100", play.portals[0].frame, play.portals[0].animationTimer)
	}
}

func TestWalkZombieToUsesNativeArrivalRange(t *testing.T) {
	play := &playState{
		world:          &viewer.Viewer{Level: formats.Level{Width: 1, Height: 1, Layers: map[formats.LayerKind][]uint32{formats.LayerC: {math.MaxUint32}}}},
		x:              0,
		y:              0,
		scriptEntities: map[int]*scriptEntity{7: {id: 7, kind: "zombie", speed: 60}},
		zombies:        []zombieState{{x: 85, y: 0, speed: 60, health: 100, size: formats.Vec2{X: 32, Y: 32}, scriptID: 7}},
	}
	host := &playScriptHost{play: play}
	if _, err := host.Call("WalkZombieTo", []scripting.Value{7, 100, 0, 20}); err != nil {
		t.Fatal(err)
	}
	entity := play.scriptEntities[7]
	if entity.targetRange != 20 || !entity.walking {
		t.Fatalf("WalkZombieTo state = range %.1f walking %t, want range 20 walking true", entity.targetRange, entity.walking)
	}
	play.updateZombies()
	if entity.walking {
		t.Fatal("WalkZombieTo remained active inside native arrival range")
	}
	if play.zombies[0].x != 85 || play.zombies[0].y != 0 {
		t.Fatalf("zombie moved inside native arrival range to (%.1f,%.1f), want unchanged (85,0)", play.zombies[0].x, play.zombies[0].y)
	}
}

func TestSpawnAwayZombieUsesNativeAwayState(t *testing.T) {
	play := &playState{zombies: []zombieState{{scriptID: 7, health: 100}}, scriptEntities: map[int]*scriptEntity{7: {id: 7, kind: "zombie", walking: true}}}
	host := &playScriptHost{play: play}
	if _, err := host.Call("SpawnAwayZombie", []scripting.Value{7}); err != nil {
		t.Fatal(err)
	}
	if !play.zombies[0].spawnAway || play.zombies[0].health != 0 || play.zombies[0].dying || play.scriptEntities[7].walking {
		t.Fatalf("SpawnAwayZombie state = %#v, want away health0 non-dying and not walking", play.zombies[0])
	}
}

func TestGrenadePickupUsesConfiguredAmmo(t *testing.T) {
	play := &playState{x: 0, y: 0, weapons: formats.WeaponCatalog{{GunType: "GRENADE", Ammo: 5}}, scriptEntities: map[int]*scriptEntity{3: {id: 3, kind: "pickup", texture: "p_grenade", x: 0, y: 0}}}
	play.updatePickups()
	if play.grenades != 5 {
		t.Fatalf("grenade inventory = %d, want configured ammo 5", play.grenades)
	}
	if _, ok := play.scriptEntities[3]; ok {
		t.Fatal("collected grenade pickup remained in script entities")
	}
}

func TestLevelActionHitboxesUseXMLVariables(t *testing.T) {
	variables := formats.FrontendVariables{
		"SHOPFRONT_PLAY_ICON_POS_VAR":           {Kind: "Vec2", Vec2: formats.Vec2{X: 415, Y: 210}},
		"SHOPFRONT_PLAY_ICON_WIDTH_VAR":         {Kind: "Float", Float: 84},
		"SHOPFRONT_PLAY_ICON_HEIGHT_VAR":        {Kind: "Float", Float: 28},
		"SHOPFRONT_BACK_ICON_NO_GLOBAL_POS_VAR": {Kind: "Vec2", Vec2: formats.Vec2{X: 415, Y: 266}},
		"SHOPFRONT_BACK_ICON_WIDTH_VAR":         {Kind: "Float", Float: 84},
		"SHOPFRONT_BACK_ICON_HEIGHT_VAR":        {Kind: "Float", Float: 28},
	}
	game := &app{variables: variables}
	if got := game.levelActionAt(415, 210); got != "play" {
		t.Fatalf("play hit = %q, want play", got)
	}
	if got := game.levelActionAt(415, 266); got != "back" {
		t.Fatalf("back hit = %q, want back", got)
	}
	if got := game.levelActionAt(415, 240); got != "" {
		t.Fatalf("gap hit = %q, want empty", got)
	}
}

func TestLevelSelectBackReturnsToMainMenu(t *testing.T) {
	game := &app{page: 2, level: 4}
	game.leaveLevelSelect()
	if game.page != 0 || game.level != 0 {
		t.Fatalf("level-select back state = page %d level %d, want page 0 level 0", game.page, game.level)
	}
}

func TestBarryAimDirectionMatchesFacingColumns(t *testing.T) {
	for _, test := range []struct {
		angle int
		dx    float64
		dy    float64
	}{
		{0, 0, 1},
		{4, 1, 0},
		{8, 0, -1},
	} {
		dx, dy := barryAimDirection(test.angle, false)
		if dx < test.dx-.001 || dx > test.dx+.001 || dy < test.dy-.001 || dy > test.dy+.001 {
			t.Fatalf("barryAimDirection(%d)=(%.3f,%.3f), want (%.3f,%.3f)", test.angle, dx, dy, test.dx, test.dy)
		}
	}
}

func TestSetZombieTextureUsesLoadedNumericSlot(t *testing.T) {
	play := &playState{
		zombies:        []zombieState{{scriptID: 7, texture: "zombie"}},
		scriptEntities: map[int]*scriptEntity{7: {id: 7, kind: "zombie", texture: "zombie"}},
		scriptTextures: map[int]*scriptTexture{2: {id: 2, name: "Characters/professoridle"}},
	}
	host := &playScriptHost{play: play}
	if _, err := host.zombieProperty("SetZombieTexture", []scripting.Value{7, 2}); err != nil {
		t.Fatal(err)
	}
	if got := play.zombies[0].texture; got != "Characters/professoridle" {
		t.Fatalf("zombie texture = %q, want loaded slot texture", got)
	}
	if got := play.scriptEntities[7].texture; got != "Characters/professoridle" {
		t.Fatalf("script entity texture = %q, want loaded slot texture", got)
	}
}

func TestSpawnZombieUsesNativeSpriteIndex(t *testing.T) {
	for _, test := range []struct {
		index scripting.Value
		want  string
	}{
		{2, "Characters/professor"},
		{3, "Characters/princeworker"},
	} {
		play := &playState{scriptNextEntity: 1, scriptEntities: map[int]*scriptEntity{}}
		host := &playScriptHost{play: play}
		if _, err := host.spawnZombie([]scripting.Value{100, 120, 32, 0, test.index, 0, 0}); err != nil {
			t.Fatal(err)
		}
		if got := play.zombies[0].texture; got != test.want {
			t.Fatalf("spawn sprite index %v = %q, want %q", test.index, got, test.want)
		}
		if got := play.scriptEntities[1].texture; got != test.want {
			t.Fatalf("script sprite index %v = %q, want %q", test.index, got, test.want)
		}
	}
}

func TestScriptAnimationStopsOnNonLoopingXMLAnimation(t *testing.T) {
	play := &playState{
		sprites:        formats.SpriteCatalog{"test": {Name: "test", Animations: map[string]formats.SpriteAnimation{"idle": {Name: "Idle", Frames: 2, FPS: 60, Loop: false}}, AnimationOrder: []string{"idle"}}},
		scriptEntities: map[int]*scriptEntity{1: {id: 1, kind: "sprite", texture: "test", playing: true}},
	}
	play.updateScriptEntities()
	play.updateScriptEntities()
	entity := play.scriptEntities[1]
	if entity.frame != 1 || entity.playing {
		t.Fatalf("non-looping animation = frame %d playing %t, want frame 1 playing false", entity.frame, entity.playing)
	}
}

func TestSetEntityRotationPreservesScriptFacing(t *testing.T) {
	play := &playState{
		zombies:        []zombieState{{scriptID: 7}},
		scriptEntities: map[int]*scriptEntity{7: {id: 7, kind: "zombie"}},
	}
	host := &playScriptHost{play: play}
	if _, err := host.Call("SetEntityRotation", []scripting.Value{7, 180}); err != nil {
		t.Fatal(err)
	}
	entity := play.scriptEntities[7]
	if !entity.rotationSet || entity.angle != 4 || !entity.flipX {
		t.Fatalf("script rotation state = set:%t angle:%d flipX:%t, want set:true angle:4 flipX:true", entity.rotationSet, entity.angle, entity.flipX)
	}
	if got := play.zombies[0]; got.angle != 4 || !got.flipX {
		t.Fatalf("zombie rotation state = angle:%d flipX:%t, want angle:4 flipX:true", got.angle, got.flipX)
	}
}

func TestMakeZombieInvulnerableSetsDamageGate(t *testing.T) {
	play := &playState{
		zombies:        []zombieState{{scriptID: 7, health: 100}},
		scriptEntities: map[int]*scriptEntity{7: {id: 7, kind: "zombie"}},
	}
	host := &playScriptHost{play: play}
	if _, err := host.Call("MakeZombieInvulnerable", []scripting.Value{7, true}); err != nil {
		t.Fatal(err)
	}
	if !play.zombies[0].invulnerable {
		t.Fatal("zombie invulnerability was not enabled")
	}
}

func TestInvulnerableZombieIgnoresGrenadeDamage(t *testing.T) {
	play := &playState{zombies: []zombieState{{x: 0, y: 0, health: 100, invulnerable: true}, {x: 64, y: 0, health: 100}}}
	play.detonateGrenade(0, 0)
	if got := play.zombies[0].health; got != 100 {
		t.Fatalf("invulnerable zombie health = %.1f, want 100", got)
	}
	if got := play.zombies[1].health; got >= 100 {
		t.Fatalf("normal zombie health = %.1f, want damage", got)
	}
}

func TestGetPlatformMatchesNativeValue(t *testing.T) {
	host := &playScriptHost{play: &playState{}}
	result, err := host.Call("GetPlatform", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Values) != 1 || result.Values[0] != 5 {
		t.Fatalf("GetPlatform() = %#v, want numeric 5", result.Values)
	}
}

func TestDrawScriptTextUsesNativeFlags(t *testing.T) {
	play := &playState{}
	host := &playScriptHost{play: play}
	if _, err := host.drawScriptText([]scripting.Value{240, 20, "large"}, false); err != nil {
		t.Fatal(err)
	}
	if play.scriptText1Y != 136 || play.scriptText1Size != 30 {
		t.Fatalf("default DrawText1 state = y %.1f size %.1f, want y 136 size 30", play.scriptText1Y, play.scriptText1Size)
	}
	if _, err := host.drawScriptText([]scripting.Value{240, 123, "large", true, true}, false); err != nil {
		t.Fatal(err)
	}
	if play.scriptText1Y != 123 || play.scriptText1Size != 30 {
		t.Fatalf("large flagged DrawText1 state = y %.1f size %.1f, want y 123 size 30", play.scriptText1Y, play.scriptText1Size)
	}
	if _, err := host.drawScriptText([]scripting.Value{240, 149, "small", false}, true); err != nil {
		t.Fatal(err)
	}
	if play.scriptText2Y != 149 || play.scriptText2Size != 24 {
		t.Fatalf("regular DrawText2 state = y %.1f size %.1f, want y 149 size 24", play.scriptText2Y, play.scriptText2Size)
	}
}

func TestWalkPlayerToUsesNativeRangeCheck(t *testing.T) {
	play := &playState{x: 70, y: 0, scriptEntities: map[int]*scriptEntity{}}
	host := &playScriptHost{play: play}
	if _, err := host.Call("WalkPlayerTo", []scripting.Value{100, 0, 32}); err != nil {
		t.Fatal(err)
	}
	if play.scriptWalkRange != 32 || !play.scriptWalking {
		t.Fatalf("WalkPlayerTo state = range %.1f walking %t, want range 32 walking true", play.scriptWalkRange, play.scriptWalking)
	}
	play.updateScriptWalk()
	if play.scriptWalking {
		t.Fatal("WalkPlayerTo remained active inside native range check")
	}
	if play.x != 70 || play.y != 0 {
		t.Fatalf("WalkPlayerTo moved player inside range to (%.1f,%.1f), want unchanged (70,0)", play.x, play.y)
	}
}

func TestWalkPlayerToUsesNativeDefaultRangeCheck(t *testing.T) {
	play := &playState{scriptEntities: map[int]*scriptEntity{}}
	host := &playScriptHost{play: play}
	if _, err := host.Call("WalkPlayerTo", []scripting.Value{100, 0}); err != nil {
		t.Fatal(err)
	}
	if play.scriptWalkRange != 4 || !play.scriptWalking {
		t.Fatalf("WalkPlayerTo default state = range %.1f walking %t, want range 4 walking true", play.scriptWalkRange, play.scriptWalking)
	}
}

func TestKillZombieIgnoresNonPositiveHealth(t *testing.T) {
	play := &playState{zombies: []zombieState{{scriptID: 7, health: 0, dying: true, deathAge: .25}}, scriptEntities: map[int]*scriptEntity{7: {id: 7, kind: "zombie"}}}
	host := &playScriptHost{play: play}
	if _, err := host.Call("KillZombie", []scripting.Value{7}); err != nil {
		t.Fatal(err)
	}
	if !play.zombies[0].dying || play.zombies[0].deathAge != .25 {
		t.Fatalf("KillZombie reset dead state = %#v", play.zombies[0])
	}
}

func TestCameraShakeStoresNativeCallbackArguments(t *testing.T) {
	play := &playState{}
	host := &playScriptHost{play: play}
	if result, err := host.Call("CameraShake", []scripting.Value{12, 24, 1.5}); err != nil {
		t.Fatal(err)
	} else if len(result.Values) != 1 || result.Values[0] != 1 {
		t.Fatalf("CameraShake result = %#v, want 1", result.Values)
	}
	if play.shakeX != 12 || play.shakeY != 24 || play.shakeAmount != 1.5 || play.shakeDuration != 1 || !play.shakeActive {
		t.Fatalf("CameraShake state = %#v", play)
	}
	if _, err := host.Call("CameraShake", []scripting.Value{12, 24, 1.5, 2.2}); err != nil {
		t.Fatal(err)
	}
	if play.shakeDuration != 2.2 {
		t.Fatalf("CameraShake duration = %.1f, want 2.2", play.shakeDuration)
	}
}

func TestAddRobotBossZombieUsesNativeSpawnState(t *testing.T) {
	play := &playState{scriptEntities: map[int]*scriptEntity{}, scriptNextEntity: 1}
	host := &playScriptHost{play: play}
	result, err := host.Call("AddRobotBossZombie", []scripting.Value{120, 80})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Values) != 1 || result.Values[0] != 1 {
		t.Fatalf("AddRobotBossZombie result = %#v, want handle 1", result.Values)
	}
	if len(play.zombies) != 1 {
		t.Fatalf("zombie count = %d, want 1", len(play.zombies))
	}
	zombie := play.zombies[0]
	if zombie.texture != "bigboss" || zombie.size.X != 70 || zombie.size.Y != 70 || zombie.health != 40000 || zombie.speed != -1 || zombie.scriptID != 1 {
		t.Fatalf("boss state = %#v", zombie)
	}
	if entity := play.scriptEntities[1]; entity == nil || entity.entityType != "boss_robot" || entity.texture != "bigboss" {
		t.Fatalf("boss entity = %#v", play.scriptEntities[1])
	}
}

func TestSetRobotRageUpdatesRobotBossState(t *testing.T) {
	play := &playState{zombies: []zombieState{{scriptID: 1}}, scriptEntities: map[int]*scriptEntity{1: {id: 1, kind: "zombie", entityType: "boss_robot"}}}
	host := &playScriptHost{play: play}
	if _, err := host.Call("SetRobotRage", []scripting.Value{true}); err != nil {
		t.Fatal(err)
	}
	if !play.zombies[0].bossRage {
		t.Fatal("robot rage state was not enabled")
	}
	if _, err := host.Call("SetRobotRage", []scripting.Value{false}); err != nil {
		t.Fatal(err)
	}
	if play.zombies[0].bossRage {
		t.Fatal("robot rage state was not disabled")
	}
}

func TestAddWesternBossZombieUsesNativeSpawnState(t *testing.T) {
	play := &playState{scriptEntities: map[int]*scriptEntity{}, scriptNextEntity: 1}
	host := &playScriptHost{play: play}
	result, err := host.Call("AddWesternBossZombie", []scripting.Value{120, 80})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Values) != 1 || result.Values[0] != 1 {
		t.Fatalf("AddWesternBossZombie result = %#v, want handle 1", result.Values)
	}
	if len(play.zombies) != 1 {
		t.Fatalf("zombie count = %d, want 1", len(play.zombies))
	}
	zombie := play.zombies[0]
	if zombie.texture != "maddog" || zombie.size.X != 60 || zombie.size.Y != 60 || zombie.health != 40000 || zombie.speed != 0 || zombie.scriptID != 1 {
		t.Fatalf("boss state = %#v", zombie)
	}
	if entity := play.scriptEntities[1]; entity == nil || entity.entityType != "boss_west" || entity.texture != "maddog" {
		t.Fatalf("boss entity = %#v", play.scriptEntities[1])
	}
}

func TestSetWesternBossDeadUsesMaddogDeathAnimation(t *testing.T) {
	play := &playState{zombies: []zombieState{{scriptID: 1, speed: 90}}, scriptEntities: map[int]*scriptEntity{1: {id: 1, kind: "zombie", entityType: "boss_west"}}}
	host := &playScriptHost{play: play}
	if _, err := host.Call("SetWesternBossDead", nil); err != nil {
		t.Fatal(err)
	}
	if play.zombies[0].animation != "Dead" || play.zombies[0].speed != 0 {
		t.Fatalf("western boss state = %#v", play.zombies[0])
	}
}

func TestMakeRexRageUsesNativeTimerGate(t *testing.T) {
	play := &playState{zombies: []zombieState{{scriptID: 1, rexRageTimer: 100}, {scriptID: 2, rexRageTimer: 700}}, scriptEntities: map[int]*scriptEntity{1: {id: 1, kind: "zombie", entityType: "boss_rex"}, 2: {id: 2, kind: "zombie", entityType: "boss_rex"}}}
	host := &playScriptHost{play: play}
	if _, err := host.Call("MakeRexRage", nil); err != nil {
		t.Fatal(err)
	}
	if play.zombies[0].rexRageTimer != 1000 || play.zombies[1].rexRageTimer != 700 {
		t.Fatalf("T-Rex rage timers = %.0f, %.0f, want 1000, 700", play.zombies[0].rexRageTimer, play.zombies[1].rexRageTimer)
	}
}

func TestSetZombieAnimTimeUsesNativeModeAndDefault(t *testing.T) {
	play := &playState{zombies: []zombieState{{scriptID: 1, frame: 9}}, scriptEntities: map[int]*scriptEntity{1: {id: 1, kind: "zombie"}}}
	host := &playScriptHost{play: play}
	if _, err := host.Call("SetZombieAnimTime", []scripting.Value{1, true, 0}); err != nil {
		t.Fatal(err)
	}
	if !play.zombies[0].animTimeMode || play.zombies[0].frame != 0 {
		t.Fatalf("animated zombie state = mode:%t frame:%.1f, want true/0", play.zombies[0].animTimeMode, play.zombies[0].frame)
	}
	if _, err := host.Call("SetZombieAnimTime", []scripting.Value{1, false}); err != nil {
		t.Fatal(err)
	}
	if play.zombies[0].animTimeMode || play.zombies[0].frame != 1 {
		t.Fatalf("default animated zombie state = mode:%t frame:%.1f, want false/1", play.zombies[0].animTimeMode, play.zombies[0].frame)
	}
}

func TestUnlockWesternBossAchievementStoresChoice(t *testing.T) {
	play := &playState{}
	host := &playScriptHost{play: play}
	if _, err := host.Call("UnlockWesternBossAchievement", []scripting.Value{1}); err != nil {
		t.Fatal(err)
	}
	if play.westernAchievementChoice != 1 || !play.westernAchievementUnlocked {
		t.Fatalf("achievement state = choice %.1f unlocked %t, want 1 true", play.westernAchievementChoice, play.westernAchievementUnlocked)
	}
}

func TestStationaryZombieDamagesOnlyAtContact(t *testing.T) {
	play := &playState{world: &viewer.Viewer{Level: formats.Level{Width: 20, Height: 20, Layers: map[formats.LayerKind][]uint32{formats.LayerC: make([]uint32, 400)}}}, x: 32, y: 32, health: 1, scriptCollideZombies: true, scriptEntities: map[int]*scriptEntity{1: {id: 1, kind: "zombie", entityType: "zombie"}}, zombies: []zombieState{{x: 320, y: 320, health: 100, size: formats.Vec2{X: 32, Y: 32}, scriptID: 1, scriptControlled: true}}}
	play.updateZombies()
	if play.health != 1 {
		t.Fatalf("distant zombie changed health to %.3f", play.health)
	}
	play.zombies[0].x, play.zombies[0].y = 32, 32
	play.updateZombies()
	if play.health >= 1 {
		t.Fatal("contact zombie did not damage player")
	}
}

func TestPlayerDeathUsesNativeTimerAndRespawn(t *testing.T) {
	play := &playState{health: 0, maxHealth: 1, lives: 3, spawnX: 100, spawnY: 120, x: 40, y: 50}
	if !play.updatePlayerDeath() || play.lives != 2 || play.deathTimer <= 0 {
		t.Fatalf("initial death state = health %.1f lives %d timer %.3f", play.health, play.lives, play.deathTimer)
	}
	for i := 0; i < 120; i++ {
		play.updatePlayerDeath()
	}
	if play.health != 1 || play.lives != 2 || play.x != 100 || play.y != 120 || play.deathTimer != 0 {
		t.Fatalf("respawn state = health %.1f lives %d pos %.1f,%.1f timer %.3f", play.health, play.lives, play.x, play.y, play.deathTimer)
	}
}
