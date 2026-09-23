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
func TestPortalCellReuseResetsNativeLifetime(t *testing.T) {
	play := &playState{portals: []portalState{{cellX: 1, cellY: 1, age: 1.5}}}
	play.addPortal(120, 120)
	if len(play.portals) != 1 || play.portals[0].age != 0 {
		t.Fatalf("reused portal state = %#v, want one portal with age 0", play.portals)
	}
}

func TestButtonTextRectsUseNativeAtlasRows(t *testing.T) {
	got, ok := buttonTextRect(6)
	if !ok || got != image.Rect(0, 96, 128, 112) {
		t.Fatalf("quit text rect = %v, want (0,96)-(128,112)", got)
	}
}
func TestMainMenuHitUsesNativeButtonSize(t *testing.T) {
	app := &app{variables: formats.FrontendVariables{"MAINMENU_AOZ_BUTTON_SIZE_VAR": {Kind: "Vec2", Vec2: formats.Vec2{X: 128, Y: 128}}}}
	if got := app.mainMenuHit(76, 150); got != 0 {
		t.Fatalf("native-sized main-menu hit = %d, want options button 0", got)
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

func TestZombieDeathAnimationUsesXMLCatalog(t *testing.T) {
	catalog := formats.SpriteCatalog{"zombiedeaths": {Name: "ZombieDeaths", Animations: map[string]formats.SpriteAnimation{"pop_1": {Name: "Pop_1", Texture: "Textures/ZombiePop_1_SD", Frames: 4, FPS: 8, Loop: false}}}}
	animation, ok := zombieDeathAnimation(catalog, 1)
	if !ok || animation.Texture != "Textures/ZombiePop_1_SD" || animation.Frames != 4 || animation.FPS != 8 || animation.Loop {
		t.Fatalf("death animation = %#v, found %t, want XML Pop_1 metadata", animation, ok)
	}
}

func TestBulletUpdateAppliesNativeDamageAndStartsZombieDeath(t *testing.T) {
	collision := make([]uint32, 64)
	for i := range collision {
		collision[i] = math.MaxUint32
	}
	play := &playState{
		world:     &viewer.Viewer{Level: formats.Level{Width: 8, Height: 8, Layers: map[formats.LayerKind][]uint32{formats.LayerC: collision}}, Zoom: 1},
		tileSize:  32,
		health:    1,
		maxHealth: 1,
		zombies:   []zombieState{{x: 96, y: 96, health: 100, size: formats.Vec2{X: 32, Y: 32}}},
		bullets:   []bullet{{x: 96, y: 96, life: 1}},
	}
	if play.isSolid(96, 96) || !bulletHitsZombie(96, 96, 96, 96, play.zombies[0]) {
		t.Fatal("bullet fixture must be in a non-solid tile and overlap the zombie")
	}
	play.Update(0, 0, false, false, false)
	if len(play.bullets) != 0 {
		t.Fatalf("overlapping bullet count = %d, want consumed hit", len(play.bullets))
	}
	if zombie := play.zombies[0]; zombie.health != -400 || !zombie.dying || zombie.deathAge != 0 {
		t.Fatalf("zombie after native 500 hit = health %.1f dying %t deathAge %.3f, want -400/true/0", zombie.health, zombie.dying, zombie.deathAge)
	}
}

func TestBulletUpdateContinuesPastInvulnerableZombie(t *testing.T) {
	collision := make([]uint32, 64)
	for i := range collision {
		collision[i] = math.MaxUint32
	}
	play := &playState{
		world:     &viewer.Viewer{Level: formats.Level{Width: 8, Height: 8, Layers: map[formats.LayerKind][]uint32{formats.LayerC: collision}}, Zoom: 1},
		tileSize:  32,
		health:    1,
		maxHealth: 1,
		zombies: []zombieState{
			{x: 96, y: 96, health: 100, invulnerable: true, size: formats.Vec2{X: 32, Y: 32}},
			{x: 96, y: 96, health: 100, size: formats.Vec2{X: 32, Y: 32}},
		},
		bullets: []bullet{{x: 96, y: 96, life: 1}},
	}
	for _, zombie := range play.zombies {
		if !bulletHitsZombie(96, 96, 96, 96, zombie) {
			t.Fatal("bullet fixture must overlap both zombies")
		}
	}
	play.Update(0, 0, false, false, false)
	if len(play.bullets) != 0 {
		t.Fatalf("overlapping bullet count = %d, want consumed hit", len(play.bullets))
	}
	if zombie := play.zombies[0]; zombie.health != 100 || zombie.dying {
		t.Fatalf("invulnerable first target = health %.1f dying %t, want 100/false", zombie.health, zombie.dying)
	}
	if zombie := play.zombies[1]; zombie.health != -400 || !zombie.dying || zombie.deathAge != 0 {
		t.Fatalf("vulnerable second target = health %.1f dying %t deathAge %.3f, want -400/true/0", zombie.health, zombie.dying, zombie.deathAge)
	}
}

func TestSpawnEntityDistinguishesNativePickupNames(t *testing.T) {
	play := &playState{scriptNextEntity: 1, scriptEntities: map[int]*scriptEntity{}}
	host := &playScriptHost{play: play}
	if _, err := host.spawnEntity([]scripting.Value{"mine", 10, 20}); err != nil {
		t.Fatal(err)
	}
	if entity := play.scriptEntities[1]; entity == nil || entity.kind != "sprite" || !entity.playing {
		t.Fatalf("mine entity = %#v, want active sprite", play.scriptEntities[1])
	}
	if _, err := host.spawnEntity([]scripting.Value{"p_grenade", 30, 40}); err != nil {
		t.Fatal(err)
	}
	if entity := play.scriptEntities[2]; entity == nil || entity.kind != "pickup" || entity.playing {
		t.Fatalf("p_grenade entity = %#v, want inactive pickup", play.scriptEntities[2])
	}
}

func TestBaseRenderLayersUseNativeOrder(t *testing.T) {
	want := []formats.LayerKind{formats.LayerG, formats.LayerHB, formats.LayerD}
	if len(formats.BaseRenderLayerKinds) != len(want) {
		t.Fatalf("base render layers = %#v, want %#v", formats.BaseRenderLayerKinds, want)
	}
	for index := range want {
		if formats.BaseRenderLayerKinds[index] != want[index] {
			t.Fatalf("base render layer %d = %q, want %q", index, formats.BaseRenderLayerKinds[index], want[index])
		}
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

func TestWalkZombieToStopsMovementWhenNativeFlagIsSet(t *testing.T) {
	play := &playState{
		world:          &viewer.Viewer{Level: formats.Level{Width: 1, Height: 1, Layers: map[formats.LayerKind][]uint32{formats.LayerC: {math.MaxUint32}}}},
		x:              0,
		y:              0,
		scriptEntities: map[int]*scriptEntity{7: {id: 7, kind: "zombie", speed: 60}},
		zombies:        []zombieState{{x: 85, y: 0, speed: 60, health: 100, size: formats.Vec2{X: 32, Y: 32}, scriptID: 7}},
	}
	host := &playScriptHost{play: play}
	if _, err := host.Call("WalkZombieTo", []scripting.Value{7, 100, 0, 20, true}); err != nil {
		t.Fatal(err)
	}
	play.updateZombies()
	entity := play.scriptEntities[7]
	if entity.walking || !entity.stopOnArrival || play.zombies[0].speed != 0 {
		t.Fatalf("flagged WalkZombieTo state = walking %t stop %t speed %.1f, want false true 0", entity.walking, entity.stopOnArrival, play.zombies[0].speed)
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

func TestSecondaryButtonBoundsMatchVisibleGrenadeControl(t *testing.T) {
	play := &playState{grenades: 1}
	if !play.secondaryButtonContains(416, 208) || play.secondaryButtonContains(350, 208) {
		t.Fatal("grenade button hitbox does not match its visible bounds")
	}
	play.grenades = 0
	if play.secondaryButtonContains(416, 208) {
		t.Fatal("hidden grenade button still consumes the aiming pointer")
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
		if got := play.zombies[0].size; got.X != 64 || got.Y != 64 {
			t.Fatalf("script sprite index %v render size = %#v, want 64x64", test.index, got)
		}
	}
}

func TestSpawnZombieUsesNativeDefaultRenderSize(t *testing.T) {
	play := &playState{scriptNextEntity: 1, scriptEntities: map[int]*scriptEntity{}}
	host := &playScriptHost{play: play}
	if _, err := host.spawnZombie([]scripting.Value{100, 120, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if got := play.zombies[0].size; got.X != 48 || got.Y != 48 {
		t.Fatalf("default zombie render size = %#v, want 48x48", got)
	}
}

func TestNativeZombieRenderSizeUsesConstructorDefault(t *testing.T) {
	if got := nativeZombieRenderSize(0); got != nativeZombieDefaultRenderSize {
		t.Fatalf("native default zombie render size = %.1f, want %.1f", got, nativeZombieDefaultRenderSize)
	}
	if got := nativeZombieRenderSize(29); got != 58 {
		t.Fatalf("positive zombie render size = %.1f, want 58", got)
	}
}

func TestZombieRenderAnchorUsesNativeFactor(t *testing.T) {
	if zombieRenderAnchor != .35 {
		t.Fatalf("zombie render anchor = %.2f, want native .35", zombieRenderAnchor)
	}
}

func TestPlayerRenderAnchorUsesNativeOffset(t *testing.T) {
	if playerRenderAnchor != 25 {
		t.Fatalf("player render anchor = %.1f, want native 25", playerRenderAnchor)
	}
}

func TestMuzzleTransformHorizontalOffsetsAreSymmetric(t *testing.T) {
	x, y, _, ok := muzzleTransform(1, 0)
	if !ok || x != 22 || y != 0 {
		t.Fatalf("right-horizontal muzzle offset = (%.1f, %.1f), %t; want (22, 0), true", x, y, ok)
	}
	x, y, _, ok = muzzleTransform(1, .1)
	if !ok || x != 22 || y != 12 {
		t.Fatalf("slightly-down-right muzzle offset = (%.1f, %.1f), %t; want unchanged (22, 12), true", x, y, ok)
	}
	x, y, _, ok = muzzleTransform(-1, 0)
	if !ok || x != -22 || y != 0 {
		t.Fatalf("left-horizontal muzzle offset = (%.1f, %.1f), %t; want symmetric (-22, 0), true", x, y, ok)
	}
	x, y, _, ok = muzzleTransform(-1, .1)
	if !ok || x != -22 || y != 12 {
		t.Fatalf("slightly-down-left muzzle offset = (%.1f, %.1f), %t; want unchanged (-22, 12), true", x, y, ok)
	}
}

func TestPlayerFlashDurationUsesNativeWeaponTimer(t *testing.T) {
	if nativePlayerFlashDuration != 0.4 {
		t.Fatalf("player flash duration = %.2f, want native 0.4", nativePlayerFlashDuration)
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
func TestNativeSpriteDirectionFoldsNativeRotation(t *testing.T) {
	for _, test := range []struct {
		degrees       float64
		columns, want int
		flip          bool
	}{{0, 9, 4, false}, {90, 9, 0, false}, {180, 9, 4, true}, {270, 9, 8, false}, {0, 5, 2, false}} {
		got, flip := nativeSpriteDirection(test.degrees, test.columns)
		if got != test.want || flip != test.flip {
			t.Fatalf("nativeSpriteDirection(%.1f,%d)=(%d,%t), want (%d,%t)", test.degrees, test.columns, got, flip, test.want, test.flip)
		}
	}
}
func TestPickupCrateTextureUsesNativeSpecialTypes(t *testing.T) {
	for _, name := range []string{"p_cow_pat", "p_bazooka", "p_sentry", "p_rand_1", "p_rand_2"} {
		if got := pickupCrateTexture(name); got != "Common0/Textures/Special_Crate" {
			t.Fatalf("pickupCrateTexture(%q) = %q, want special crate", name, got)
		}
	}
	if got := pickupCrateTexture("p_grenade"); got != "Common0/Textures/crate_SD" {
		t.Fatalf("pickupCrateTexture(p_grenade) = %q, want normal crate", got)
	}
}
func TestWaveSpawnerIntervalUsesNativeDelayAndCount(t *testing.T) {
	if got := waveSpawnerInterval(1000, 100, 3); got != 300 {
		t.Fatalf("waveSpawnerInterval = %.1f, want 300", got)
	}
	if got := waveSpawnerInterval(100, 200, 1); got != 500 {
		t.Fatalf("invalid waveSpawnerInterval = %.1f, want fallback 500", got)
	}
}
func TestWaveSpawnedZombieAndPortalDoNotAdvanceOnCreationUpdate(t *testing.T) {
	collision := make([]uint32, 32)
	for index := range collision {
		collision[index] = math.MaxUint32
	}
	collision[9] = 3
	rng := newNativeRNG()
	play := &playState{
		world: &viewer.Viewer{Level: formats.Level{Width: 8, Height: 4, Layers: map[formats.LayerKind][]uint32{formats.LayerC: collision}}, Zoom: 1},
		x:     240, y: 112, health: 1, maxHealth: 1, tileSize: 32, radius: playerCollisionRadius, rng: &rng,
	}
	play.world.Level.Waves = []formats.Wave{{RunTime: 1000, EndWaveZombies: 10, Spawners: []formats.Spawner{{Count: 1, Index: 1, Types: []formats.SpawnType{{Name: "zombie", Chance: 1, Speed: formats.Vec2{X: 60}, Strength: 100}}}}}}

	play.Update(0, 0, false, false, false)

	if len(play.zombies) != 1 || len(play.portals) != 1 {
		t.Fatalf("wave update spawned %d zombies and %d portals, want one each", len(play.zombies), len(play.portals))
	}
	if play.zombies[0].x != 48 || play.zombies[0].y != 48 {
		t.Errorf("new zombie advanced on its creation update to (%.3f, %.3f), want spawn point (48, 48)", play.zombies[0].x, play.zombies[0].y)
	}
	if play.portals[0].age != 0 || play.portals[0].size != 0 || play.portals[0].animationTimer != 100 {
		t.Errorf("new portal advanced on its creation update: age %.3f size %.3f timer %.1f, want 0/0/100", play.portals[0].age, play.portals[0].size, play.portals[0].animationTimer)
	}
}
func TestNativeRNGSeedAndBoundedSequence(t *testing.T) {
	rng := newNativeRNG()
	for _, test := range []struct{ bound, want uint32 }{{100, 1}, {10, 6}, {3, 0}, {524287, 243767}} {
		if got := rng.bounded(test.bound); got != test.want {
			t.Fatalf("native RNG bounded(%d) = %d, want %d", test.bound, got, test.want)
		}
	}
}
func TestPlayStateOpeningsShareNativeRNG(t *testing.T) {
	a := &app{}
	first := &playState{rng: a.nativeRNGForPlay()}
	if got := first.rng.bounded(100); got != 1 {
		t.Fatalf("first play-state RNG bounded(100) = %d, want 1", got)
	}
	second := &playState{rng: a.nativeRNGForPlay()}
	if first.rng != second.rng {
		t.Fatal("play-state openings received different RNG state")
	}
	if got := second.rng.bounded(10); got != 6 {
		t.Fatalf("second play-state RNG bounded(10) = %d, want continued stream value 6", got)
	}
}
func TestChooseSpawnTypeUsesNativeChanceWeights(t *testing.T) {
	rng := newNativeRNG()
	types := []formats.SpawnType{{Name: "first", Chance: 1}, {Name: "second", Chance: 9}}
	for _, want := range []string{"first", "second", "second", "second"} {
		got, ok := chooseSpawnType(types, &rng)
		if !ok || got.Name != want {
			t.Fatalf("chooseSpawnType = %#v, %t, want %q", got, ok, want)
		}
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

func TestGetPositionWithinRadiusReturnsNativeClearedPair(t *testing.T) {
	host := &playScriptHost{play: &playState{}}
	result, err := host.Call("GetPositionWithinRadius", []scripting.Value{240, 160, 50, 170})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Values) != 2 || result.Values[0] != 0 || result.Values[1] != 0 {
		t.Fatalf("GetPositionWithinRadius() = %#v, want cleared pair (0, 0)", result.Values)
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

func TestRegisteredDialoguePortraitSurvivesHiddenScriptCameos(t *testing.T) {
	play := &playState{scriptTextures: map[int]*scriptTexture{}}
	host := &playScriptHost{play: play}
	for _, texture := range []struct {
		id   float64
		name string
	}{{0, "Cameos/wrongcameo"}, {7, "Cameos/barrycameo"}} {
		if _, err := host.Call("LoadTexture", []scripting.Value{texture.id, texture.name}); err != nil {
			t.Fatal(err)
		}
		if _, err := host.Call("SetTextureVisible", []scripting.Value{texture.id, true}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := host.Call("RegisterCameo", []scripting.Value{float64(0), float64(7)}); err != nil {
		t.Fatal(err)
	}
	if play.scriptTextures[0].cameo != -1 || play.scriptTextures[7].cameo != 0 || play.scriptCameos[0] != 7 {
		t.Fatalf("RegisterCameo(0, 7) registered textures %#v and mapping %#v, want cameo 0 -> texture 7", play.scriptTextures, play.scriptCameos)
	}
	if _, err := host.Call("CameoShow", []scripting.Value{false}); err != nil {
		t.Fatal(err)
	}
	if got, want := (&app{play: play}).dialogueCameo(0), commonSDTexture("Cameos/barrycameo"); got != want {
		t.Fatalf("dialogue cameo while script cameos are hidden = %q, want registered portrait %q", got, want)
	}
	if _, err := host.Call("CameoShow", []scripting.Value{true}); err != nil {
		t.Fatal(err)
	}
	if got, want := (&app{play: play}).dialogueCameo(0), commonSDTexture("Cameos/barrycameo"); got != want {
		t.Fatalf("dialogue cameo = %q, want registered texture %q", got, want)
	}
}

func TestDialogueTextLayoutKeepsPanelPaddingWithoutCameo(t *testing.T) {
	textX, textWidth := dialogueTextLayout(12, 456, false)
	if textX != 24 || textWidth != 432 {
		t.Fatalf("no-cameo layout = x %.0f width %.0f, want x 24 width 432", textX, textWidth)
	}
}

func TestDialogueTextLayoutReservesPortraitSpaceForResolvedCameo(t *testing.T) {
	textX, textWidth := dialogueTextLayout(12, 456, true)
	if textX != 99 || textWidth != 357 {
		t.Fatalf("resolved-cameo layout = x %.0f width %.0f, want x 99 width 357", textX, textWidth)
	}
}

func TestActiveDialoguePreventsManualPlayerRotation(t *testing.T) {
	play := &playState{world: &viewer.Viewer{Level: formats.Level{Width: 20, Height: 20, Layers: map[formats.LayerKind][]uint32{formats.LayerC: make([]uint32, 400)}}, Zoom: 1}, x: 100, y: 100, health: 1, maxHealth: 1, tileSize: 32, shootControl: true, angle: 0, dialogue: []dialogueLine{{text: "cutscene"}}}
	play.Update(400, 100, false, false, false)
	if play.angle != 0 {
		t.Fatalf("active dialogue changed facing to angle %d, want scripted angle 0", play.angle)
	}
}

func TestSpriteAnimationFrameUsesAnimationFPSAndFrameCount(t *testing.T) {
	idle := formats.SpriteAnimation{Frames: 4, FPS: 8, Loop: true}
	run := formats.SpriteAnimation{Frames: 4, FPS: 10, Loop: true}
	if got := spriteAnimationFrame(idle, .3, -1); got != 2 {
		t.Fatalf("idle frame = %d, want 2 at 8 fps", got)
	}
	if got := spriteAnimationFrame(run, .3, -1); got != 3 {
		t.Fatalf("run frame = %d, want 3 at 10 fps", got)
	}
	if got := spriteAnimationFrame(idle, .5, -1); got != 0 {
		t.Fatalf("looped idle frame = %d, want 0", got)
	}
}

func TestDoPlayerSpawnPreservesExplicitScriptPosition(t *testing.T) {
	play := &playState{world: &viewer.Viewer{Level: formats.Level{Width: 2, Height: 2, Layers: map[formats.LayerKind][]uint32{formats.LayerC: {0, 0, 0, 2}}}}, tileSize: 32, scriptWalking: true}
	host := &playScriptHost{play: play}
	if _, err := host.Call("SetPlayerPos", []scripting.Value{float64(530), float64(431)}); err != nil {
		t.Fatal(err)
	}
	if !play.scriptPlayerPosSet {
		t.Fatal("SetPlayerPos did not mark the explicit spawn position")
	}
	if _, err := host.Call("DoPlayerSpawn", nil); err != nil {
		t.Fatal(err)
	}
	if play.x != 530 || play.y != 431 || play.spawnX != 530 || play.spawnY != 431 || play.scriptWalking || play.scriptPlayerPosSet {
		t.Fatalf("spawn state = player (%.1f,%.1f), spawn (%.1f,%.1f), walking %t, explicit %t; want (530,431), stopped, flag cleared", play.x, play.y, play.spawnX, play.spawnY, play.scriptWalking, play.scriptPlayerPosSet)
	}
}

func TestDoPlayerSpawnUsesCollisionMarkerWithoutExplicitPosition(t *testing.T) {
	play := &playState{world: &viewer.Viewer{Level: formats.Level{Width: 2, Height: 2, Layers: map[formats.LayerKind][]uint32{formats.LayerC: {0, 0, 2, 0}}}}, tileSize: 32, x: 530, y: 431, scriptWalking: true}
	if _, err := (&playScriptHost{play: play}).Call("DoPlayerSpawn", nil); err != nil {
		t.Fatal(err)
	}
	if play.x != 16 || play.y != 48 || play.spawnX != 16 || play.spawnY != 48 || play.scriptWalking || play.scriptPlayerPosSet {
		t.Fatalf("spawn state = player (%.1f,%.1f), spawn (%.1f,%.1f), walking %t, explicit %t; want marker center (16,48), stopped, flag clear", play.x, play.y, play.spawnX, play.spawnY, play.scriptWalking, play.scriptPlayerPosSet)
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

func TestZombieDeathDelayUsesNativeTransitionTimer(t *testing.T) {
	if zombieDeathDelay != .1 {
		t.Fatalf("zombie death delay = %.3f, want native .1", zombieDeathDelay)
	}
}

func TestPortalRotationUsesNativeAngleUnits(t *testing.T) {
	play := &playState{portals: []portalState{{}}}
	play.updatePortals()
	if play.portals[0].rotationSpeed != portalOpenRotationSpeed*portalRotationLerp {
		t.Fatalf("portal rotation speed = %.3f, want %.3f", play.portals[0].rotationSpeed, portalOpenRotationSpeed*portalRotationLerp)
	}
	play.updatePortals()
	if play.portals[0].rotationUnits >= 0 {
		t.Fatalf("portal rotation units = %.3f, want negative native clockwise step", play.portals[0].rotationUnits)
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

func TestSetCameraPanInterpolatesNativeDuration(t *testing.T) {
	play := &playState{world: &viewer.Viewer{Level: formats.Level{Width: 20, Height: 20}, Zoom: 1}, tileSize: 32}
	host := &playScriptHost{play: play}
	if _, err := host.Call("SetCameraPan", []scripting.Value{.5, 300, 300, 1}); err != nil {
		t.Fatal(err)
	}
	if !play.scriptCameraPanActive || play.world.Zoom != 1 {
		t.Fatalf("camera pan start = active %t zoom %.3f, want active true zoom 1", play.scriptCameraPanActive, play.world.Zoom)
	}
	play.updateCamera()
	if play.world.Zoom >= 1 || !play.scriptCameraPanActive {
		t.Fatalf("camera pan first step = active %t zoom %.3f, want active true and zoom below 1", play.scriptCameraPanActive, play.world.Zoom)
	}
	for i := 0; i < 59; i++ {
		play.updateCamera()
	}
	if play.scriptCameraPanActive || math.Abs(play.world.Zoom-.5) > .0001 {
		t.Fatalf("camera pan final = active %t zoom %.3f, want active false zoom .5", play.scriptCameraPanActive, play.world.Zoom)
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
