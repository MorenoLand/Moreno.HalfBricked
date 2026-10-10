package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/menumotion"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/carousel"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/achievements"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/stats"
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/ui"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type app struct {
	inputHook                                    func() playerInput // headless playthrough tests inject the primary player's input
	rng                                          *weapons.NativeRNG
	pack                                         *content.Pack
	levels                                       []formats.LevelInfo
	variables                                    formats.FrontendVariables
	page                                         int
	menuSelection                                int
	options                                      optionsMenu
	statistics                                   stats.StatsData
	statsScreen                                  *statsMenu
	resultsScreen                                *resultsMenu
	achievements                                 formats.AchievementCatalog
	achievementUnlocks                           map[string]bool
	achieveSession                               achievementSession
	achievementOffset, achievementsBackPage      int
	statsClock                                   float32
	profileWritable                              bool
	world                                        int
	level                                        int
	mode                                         int
	carouselCore                                 carousel.FiniteCore
	carouselMode                                 int
	carouselReady                                bool
	selectorPointerDown, selectorTouch           bool
	selectorTouchID                              ebiten.TouchID
	selectorStartX, selectorLastX, selectorLastY int
	selectorDragging                             bool
	selectorPressIndex                           int
	weapon                                       formats.Weapon
	debug                                        bool
	mobile                                       bool
	silent                                       bool
	titleScreen                                  bool
	unlocked                                     map[string]bool
	newDismissed                                 map[string]bool
	weapons                                      formats.WeaponCatalog
	zombieWeapons                                formats.ZombieWeaponCatalog
	sprites                                      formats.SpriteCatalog
	capture                                      *engine.Capture
	captureLimit                                 int
	captureAutoDialogue                          bool
	sound                                        *engine.SoundSystem
	weaponPlayback                               weaponPlayback
	images                                       map[string]*ebiten.Image
	sources                                      map[string]image.Image
	view                                         *viewer.Viewer
	play                                         *playState
	font                                         *ui.Font
	computerFont                                 *ui.Font
	startupFrames                                int
	menuTime                                     float64
	menuSpawnTime                                float64
	menuZombies                                  []menumotion.NativeMenuZombieMotion
	titleZombie                                  menumotion.NativeMenuZombieMotion
	titleZombieActive                            bool
	achievementDrag                              bool
	selectorWheel                                float64
	resultsUI                                    *formats.UIScreen
	resultsUILoaded                              bool
	highscores                                   map[string]int32
	coopPlayers                                  int
	net                                          *netSession
	pauseOnline                                  bool
	sandboxOpen                                  bool
	lastCardClick                                cardClick
	confirm                                      *confirmDialog
	hover                                        map[string]float64
	achievementToasts                            []achievementToast
	sandboxTab, sandboxSpawned, sandboxScroll    int
	sandboxTypes                                 []formats.SpawnType
	coopDesktopPad                               bool
	barryTint                                    [3]float32
	menuMotionActive                             bool
	titleSoundElapsed                            float64
	titleSoundStage                              int
	titleCocking                                 bool
	canvas                                       *ebiten.Image
	splash                                       *ebiten.Image
	splashLoaded                                 bool
	outputWidth                                  int
	outputHeight                                 int
	frontendScaleX                               float64
	frontendScaleY                               float64
	menuClick                                    *menuClickState
	debugPanelVisible                            bool
	debugPanelDragging                           bool
	debugPanelX, debugPanelY                     float64
	debugPanelOffsetX, debugPanelOffsetY         float64
}

const logicalWidth = 480
const logicalHeight = 320

type bullet struct {
	x, y       float64
	vx, vy     float64
	life       float64
	angle      float64
	kind       string
	projectile *weapons.NativeWeaponProjectile
	origin     killOrigin
}

type explosionState struct {
	x, y, age float64
}

type portalState struct {
	x, y, age, size     float64
	rotationUnits       float64
	rotationSpeed       float64
	cellX, cellY, frame int
	animationTimer      float64
}

type bloodPop struct {
	x, y    float64
	variant int
	age     float64
}

type zombieState struct {
	x, y             float64
	speed, health    float64
	rawPoints        int
	size             formats.Vec2
	texture          string
	animation        string
	frame            float64
	angle            int
	flipX            bool
	flipY            bool
	scriptID         int
	alpha            float64
	fps              float64
	hitFlash         float64
	invulnerable     bool
	bossRage         bool
	animTimeMode     bool
	rexRageTimer     float64
	lift             float64 // guests only: the host's rex leap lift in px (net_wire.go)
	spawnAway        bool
	dying            bool
	deathAge         float64
	deathState       int     // native death state (zombie_gib.go): 0 unclassified, 1 normal, 2 gib
	deathDelay       float64 // hit timer before the death transition; 0 = zombieDeathDelay
	gibbed           bool    // a gib body presenting Disintegrate (playState.gibBodies)
	presentAge       float64 // seconds into the Disintegrate presentation
	scriptControlled bool
	collision        float64
	mirrorExploding  bool // guests only: the host says this one is an exploding zombie
	native           zombieNative // spawn-record state of wave zombies (zombie_model.go)
	grid             entityGridRegistration
}

type dialogueLine struct {
	text  string
	cameo int
}

// cardClick remembers the last level card click so a second one can play it.
type cardClick struct {
	index int
	at    float64
	valid bool
}

type playState struct {
	achievementTracking                                    achievements.AchievementTracking
	achievementMotionX, achievementMotionY                 float32
	achieve                                                achievementProgress
	combatKillCount                                        int32
	world                                                  *viewer.Viewer
	x, y, spawnX, spawnY, deathTimer                       float64
	time                                                   float64
	moving                                                 bool
	hurt                                                   float64
	vitals                                                 playerVitals // player_vitals.go
	deathStarted                                           bool
	banner                                                 waveBanner
	bannerSeen                                             int
	wavesStarted                                           bool
	gun                                                    gunState
	gunHeld                                                bool
	sawAudio                                               sawAudioState
	cheats                                                 cheatFlags
	angle                                                  int
	flipX                                                  bool
	tileSize                                               int
	radius                                                 float64
	flash                                                  float64
	stick                                                  int
	leftBaseX, leftBaseY, leftDeflectX, leftDeflectY       float64
	rightBaseX, rightBaseY, rightDeflectX, rightDeflectY   float64
	bullets                                                []bullet
	shootCooldown                                          float64
	shotSound                                              string
	spinAudio                                              weapons.WeaponAudioState
	spinEvents                                             weapons.WeaponAudioQueue
	spinEndFinished                                        bool
	pickupVoices                                           []string
	secondaryShootCooldown                                 float64
	paused                                                 bool
	shouldQuit                                             bool
	weapon                                                 formats.Weapon
	weapons                                                formats.WeaponCatalog
	sprites                                                formats.SpriteCatalog
	grenades                                               int
	secondaryType                                          string
	coop                                                   *coopState
	secondaryPlayersWait                                   bool
	input                                                  playerInput
	aimActive                                              bool
	bossScripts                                            []string
	bossDefeated, scriptPaused                             bool
	pausedControls                                         [2]bool
	sfxQueue                                               []string
	alertNoise                                             float64 // player +0x3e0 (zombie_ai.go)
	zombieWeapons                                          formats.ZombieWeaponCatalog
	zombieShots                                            []zombieShot
	sentryLoopHold                                         float64
	navs                                                   [maxCoopPlayers]navField
	secondaryWeapon                                        string
	mines                                                  []mineState
	thrown                                                 []thrownBomb
	sentries                                               []sentryState
	moveControl                                            bool
	shootControl                                           bool
	scriptAllowThumbsticks                                 bool
	scriptForceReticule                                    bool
	scriptForceThumbStick                                  [2]bool
	scriptThumbStickFree                                   [2]bool
	scriptThumbStickX, scriptThumbStickY                   [2]float64
	scriptSecondaryEnabled                                 bool
	secondaryButtonDown, secondaryButtonJustPressed        bool
	secondaryPointerDown                                   bool
	scriptCollideZombies                                   bool
	scriptZombiesActive                                    bool
	scriptRuntime                                          *scripting.Runtime
	scriptWaitRemaining                                    float64
	scriptWaitActive                                       bool
	scriptWaitStarts                                       int
	scriptWalking                                          bool
	scriptPlayerPosSet                                     bool
	scriptWalkX, scriptWalkY                               float64
	scriptWalkRange                                        float64
	scriptLevelToLoad                                      string
	scriptNextEntity                                       int
	scriptEntities                                         map[int]*scriptEntity
	scriptTextures                                         map[int]*scriptTexture
	scriptText1, scriptText2                               string
	scriptText1X, scriptText1Y, scriptText2X, scriptText2Y float64
	scriptText1Size, scriptText2Size                       float64
	scriptTextVisible                                      bool
	scriptAlpha                                            float64
	scriptFadeRemaining                                    float64
	scriptFadeDuration                                     float64
	scriptFadeBlack                                        bool
	scriptShowSkip                                         bool
	scriptCameoVisible                                     bool
	scriptCameos                                           map[int]int
	scriptZombieTargetX, scriptZombieTargetY               float64
	scriptHasZombieTarget                                  bool
	scriptCameraFollow                                     bool
	scriptCameraFollowID                                   int
	scriptCameraFollowOffsetX, scriptCameraFollowOffsetY   float64
	scriptCameraPanActive                                  bool
	scriptCameraPanStartX, scriptCameraPanStartY           float64
	scriptCameraPanTargetX, scriptCameraPanTargetY         float64
	scriptCameraPanStartZoom, scriptCameraPanTargetZoom    float64
	scriptCameraPanElapsed, scriptCameraPanDuration        float64
	shake                                                  cameraShake
	shakeOffX, shakeOffY                                   float64
	train                                                  *trainHazard
	trainSpec                                              trainSpec
	westernAchievementChoice                               float64
	westernAchievementUnlocked                             bool
	scriptAimX, scriptAimY                                 float64
	scriptHasAim                                           bool
	scriptLastCallback                                     string
	waveIndex                                              int
	wavesFinished, exitScriptStarted                       bool
	entryControlsRestored                                  bool
	levelInfo                                              formats.LevelInfo
	levelStartScore                                        int
	controls                                               optionsControls
	controlWidth, controlHeight                            int
	secondaryControlSize                                   formats.Vec2
	secondaryDensity                                       float64
	playerUnspawned                                        bool
	mobileControls                                         bool
	statistics                                             *stats.StatsData
	statsPositionX, statsPositionY                         float64
	waveElapsed                                            float64
	waveEndTimer                                           int // wave_timing.go: native end_wave_time countdown (ms)
	lookahead v7Lookahead // camera_lookahead.go: SD player look-ahead state
	waveEndStamp                                           float64
	waveEndInit                                            bool
	waveSpawned                                            []int
	survival                                               *survivalState
	rng                                                    *weapons.NativeRNG
	rex                                                    rexBossField
	zombies                                                []zombieState
	health                                                 float64
	maxHealth                                              float64
	levelKills, levelZombieTotal                           int
	progressOpacity                                        float64
	progressMirror                                         bool
	progressValue                                          float64
	score                                                  int
	lives                                                  int
	multiplier                                             int
	combo                                                  comboSystem
	portals                                                []portalState
	bloodPops                                              []bloodPop
	gibBodies                                              []zombieState // gib bodies presenting Disintegrate (zombie_gib.go)
	explosions                                             []explosionState
	zombieBlasts                                           []zombieBlast
	hudVisible                                             bool
	dialogue                                               []dialogueLine
	dialogueIndex                                          int
	dialogueAge                                            float64
}

// Native player update FUN_00096818: velocity = input (length <= 1) * 0.81 (DAT_00096bfc,
// the digital keyboard counts as a full stick) and pos += dt * velocity * base
// * walkSpeedFactor, with the base per build (playerWalkBase, vitals). The body is
// pushed out of the level by FUN_000be3c0 with radius 0.2 * the 64 px player body
// (DAT_00097498) = 12.8 px. (Earlier port values: 180 and 16, neither found in the binary.)
const playerCollisionRadius = 0.2 * 64

var barryMuzzleOffsets = [...]struct{ x, y float64 }{{-6, 24}, {4, 26}, {13, 24}, {22, 17}, {27, 11}, {30, 2}, {28, -10}, {24, -18}, {10, -22}, {-22, -22}, {-26, -14}, {-28, -1}, {-28, 7}, {-25, 15}, {-18, 21}, {-8, 25}}

const playerCollisionStep = 4.0
const zombieHitFlashDuration = .125
const zombieDeathDelay = .1
const portalRotationUnitsPerSecond = 65338.0
const portalOpenRotationSpeed = 1.2
const portalRotationLerp = .05
const zombieRenderAnchor = .35
const nativeZombieDefaultRenderSize = 48.0
const playerRenderAnchor = 25.0
const nativeWeaponFlashDuration = .16
const nativeProjectileRenderAnchor = 20.0
const nativeFireAimDistance = 96.0

func (a *app) nativeRNGForPlay() *weapons.NativeRNG {
	if a.rng == nil {
		rng := weapons.NewNativeRNG()
		a.rng = &rng
	}
	return a.rng
}

func nativeZombieRenderSize(size float64) float64 {
	if size > 0 {
		return size * 2
	}
	return nativeZombieDefaultRenderSize
}

func newApp(root string, debug, mobile, silent bool) (*app, error) {
	prepared, err := content.PrepareAssets(root)
	if err != nil {
		return nil, err
	}
	pack, err := content.NewPack(content.NewSource(prepared))
	if err != nil {
		return nil, err
	}
	weapons, err := pack.Weapons()
	if err != nil {
		return nil, err
	}
	zombieWeapons, err := pack.ZombieWeapons()
	if err != nil {
		return nil, err
	}
	sprites, err := pack.Sprites()
	if err != nil {
		return nil, err
	}
	weapon, ok := weapons.Find("PISTOL")
	if !ok {
		return nil, fmt.Errorf("default pistol is not present in the weapon catalog")
	}
	var sound *engine.SoundSystem
	if silent {
		sound = engine.NewSilentSoundSystem(pack)
	} else {
		sound = engine.NewSoundSystem(pack)
	}
	game := &app{pack: pack, levels: pack.List(), variables: pack.Variables(), debug: debug, mobile: mobile || engine.IsMobileDevice(), silent: silent, titleScreen: true, menuSelection: 1, options: newOptionsMenu(!silent, !silent), weapon: weapon, weapons: weapons, zombieWeapons: zombieWeapons, sprites: sprites, unlocked: initialUnlocks(pack.List()), sound: sound, images: map[string]*ebiten.Image{}, sources: map[string]image.Image{}, startupFrames: 45, frontendScaleX: 1, frontendScaleY: 1, debugPanelX: 8, debugPanelY: 8}
	game.font, _ = loadFont(pack)
	game.computerFont, _ = loadNamedFont(pack, "Common0/Fonts/ComputerScreen.fnt", "Common0/Fonts/ComputerScreen_0")
	game.statistics = stats.NewStatsData()
	game.options = newOptionsMenu(true, true)
	game.options.useDeviceDefaults(game.mobile)
	game.achievements, err = pack.Achievements()
	if err != nil {
		return nil, err
	}
	game.profileWritable = true
	if err := game.loadPlayerProfile(); err != nil {
		return nil, err
	}
	game.sound.MusicEnabled(!silent && game.options.music)
	return game, nil
}
func (a *app) playSound(path string, volume float64) {
	if a == nil || a.silent || !a.options.sound || a.sound == nil {
		return
	}
	a.sound.Play(path, volume)
}
func (a *app) setMenuMusic() {
	if a == nil || a.silent || a.sound == nil {
		return
	}
	_ = a.sound.SetMusic("audio/music/sound/Music_Menu.ogg", 532640)
}
func (a *app) setWorldMusic(world int) {
	if a == nil || a.silent || a.sound == nil {
		return
	}
	path, loopPoint, ok := worldMusicTrack(world)
	if ok {
		_ = a.sound.SetMusic(path, loopPoint)
	}
}
func worldMusicTrack(world int) (string, int64, bool) {
	tracks := [...]struct {
		path      string
		loopPoint int64
	}{{"audio/music/sound/Music_Caveman.ogg", 774700}, {"audio/music/sound/Music_1930s.ogg", 510000}, {"audio/music/sound/Music_Egypt.ogg", 795000}, {"audio/music/sound/Music_Japan.ogg", 599583}, {"audio/music/sound/Music_Future.ogg", 471000}, {"audio/music/sound/Music_western.ogg", 788162}}
	if world < 0 || world >= len(tracks) {
		return "", 0, false
	}
	return tracks[world].path, tracks[world].loopPoint, true
}
func (a *app) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && (ebiten.IsKeyPressed(ebiten.KeyAltLeft) || ebiten.IsKeyPressed(ebiten.KeyAltRight)) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
		return nil
	}
	if a.capture != nil && a.captureLimit > 0 && a.capture.Frames() >= uint64(a.captureLimit) {
		return ebiten.Termination
	}
	if a.startupFrames > 0 {
		a.startupFrames--
		return nil
	}
	if a.net != nil && !a.net.lobby && a.play == nil {
		a.leaveNet("game over")
	}
	a.netHostResults()
	if a.net != nil && a.net.lobby {
		return a.updateNetLobby()
	}
	a.menuTime += 1.0 / 60.0
	a.updateAchievementToasts(1.0 / 60.0)
	if handled, err := a.updateConfirm(); handled || err != nil {
		return err
	}
	a.updateStatsClock()
	if a.titleCocking {
		a.titleSoundElapsed += 1.0 / 60.0
		if a.titleSoundStage == 1 && a.titleSoundElapsed >= 8.0/60.0 {
			a.playSound("audio/sound/sfx/menu_shotgun_cock_2.ogg", .8)
			a.titleSoundStage = 2
		}
		if a.titleSoundStage == 2 && a.titleSoundElapsed >= 16.0/60.0 {
			a.playSound("audio/sound/sfx/menu_shotgun_cock_1.ogg", .8)
			a.titleSoundStage = 0
		}
		if a.titleSoundElapsed >= 32.0/60.0 {
			a.titleCocking = false
			a.titleScreen = false
			a.menuSpawnTime = 0
		}
	}
	a.updateMenuZombieMotion(float32(1.0 / 60.0))
	if inpututil.IsKeyJustPressed(ebiten.KeyF2) {
		a.debugPanelVisible = !a.debugPanelVisible
	}
	if a.debugPanelVisible && a.updateDebugPanel() {
		return nil
	}
	if a.menuClick != nil {
		return a.updateMenuClick()
	}
	if a.page == 6 {
		return a.updateAchievements()
	}
	if a.page == 4 && a.statsScreen != nil {
		if inpututil.IsKeyJustPressed(ebiten.KeyA) {
			a.openAchievements()
			return nil
		}
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			x, y := a.pointer()
			if x >= 300 && y <= 28 {
				a.openAchievements()
				return nil
			}
		}
		back := inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
		if a.statsScreen.update(1.0/60.0, back, false, false) == statsMainMenu {
			a.statsScreen = nil
			a.page = 0
			if a.play != nil {
				a.page = 2
			}
			return a.savePlayerProfile()
		}
		return nil
	}
	if a.page == 5 && a.resultsScreen != nil {
		if a.netGuest() && !a.net.lobby {
			return a.updateGuestResults()
		}
		return a.updateResultsMenu()
	}
	if a.page == 3 && !a.titleScreen && a.view == nil {
		action := optionsNone
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			action = optionsBack
		}
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			x, y := a.pointer()
			action = optionsHit(a.variables, float64(x), float64(y))
		}
		var musicEnabled func(bool)
		if a.sound != nil {
			musicEnabled = func(enabled bool) { a.sound.MusicEnabled(enabled && !a.silent) }
		}
		x, y := a.pointer()
		if center, ok := a.variables.Vec2Value("OPTIONS_SIZE_CENTER_POS_VAR"); ok {
			a.options.dragPad(float64(x), float64(y), center.X, center.Y, action == optionsPad, ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft))
		}
		if a.options.activate(action, musicEnabled) {
			a.page = 0
			if a.play != nil {
				a.page = 2
				a.play.configureControls(a.options.controls, a.outputWidth, a.outputHeight)
			}
			return a.savePlayerProfile()
		}
		return nil
	}
	if a.titleScreen {
		if a.titleCocking {
			return nil
		}
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			a.titleCocking = true
			a.titleSoundElapsed = 0
			a.titleSoundStage = 1
		}
		return nil
	}
	if a.view != nil {
		if a.view.Back() {
			a.view = nil
			return nil
		}
		a.view.SetInputSize(a.outputWidth, a.outputHeight)
		a.view.Update()
		return nil
	}
	if a.play != nil {
		statsPlay, previousKills, previousLives := a.play, a.play.levelKills, a.play.lives
		defer a.recordPlayStats(statsPlay, previousKills, previousLives)
		defer a.flushPickupVoices(statsPlay)
		defer a.flushSfxQueue(statsPlay)
		defer a.flushSpinAudio(statsPlay)
		// The camera shake advances once per logic tick, after the tick's script and camera work
		// (native: the per-camera updater runs the shake step); drawing only reads the offset.
		defer statsPlay.stepShakeTick()
		if a.netGuest() {
			return a.updateNetGuest()
		}
		if a.updateSandbox() {
			return nil
		}
		if a.netHosting() {
			a.netHostPoll()
		}
		if a.play.controlWidth != a.outputWidth || a.play.controlHeight != a.outputHeight {
			a.play.configureControls(a.play.controls, a.outputWidth, a.outputHeight)
		}
		a.play.controlWidth, a.play.controlHeight = a.outputWidth, a.outputHeight
		pointerX, pointerY := a.pointer()
		a.play.secondaryButtonDown = ebiten.IsKeyPressed(ebiten.KeyG) || ebiten.IsKeyPressed(ebiten.KeyQ) || (a.play.secondaryButtonContains(float64(pointerX), float64(pointerY)) && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft))
		a.play.secondaryButtonJustPressed = inpututil.IsKeyJustPressed(ebiten.KeyG) || inpututil.IsKeyJustPressed(ebiten.KeyQ) || (a.play.secondaryButtonContains(float64(pointerX), float64(pointerY)) && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft))
		a.play.secondaryPointerDown = a.play.secondaryButtonContains(float64(pointerX), float64(pointerY)) && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			if a.play.paused {
				a.play.paused = false
				return nil
			}
			a.play.paused = true
			return nil
		}
		if a.play.paused {
			return a.updatePauseMenu()
		}
		if a.play.storyGameOver(a.mode) {
			a.openDeathResults()
			return nil
		}
		if a.play.dialogueIndex < len(a.play.dialogue) {
			if !a.play.paused {
				if err := a.play.updateScript(); err != nil {
					return err
				}
				a.play.dialogueAge += 1.0 / 60.0
				if a.capture != nil && a.captureAutoDialogue && a.play.dialogueAge >= .5 {
					a.play.dialogueIndex++
					a.play.dialogueAge = 0
				}
			}
			if a.play.scriptRuntime == nil || a.play.scriptRuntime.Done() {
				a.play.updateWaves()
			}
			a.play.updateZombies()
			if a.play.updatePlayerDeath() {
				return a.updateGameplayAchievements(a.play)
			}
			a.play.updatePortals()
			a.play.updatePickups()
			if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				a.play.dialogueIndex++
				a.play.dialogueAge = 0
			}
			return a.updateGameplayAchievements(a.play)
		}
		x, y := a.pointer()
		if !a.play.paused {
			if err := a.play.updateScript(); err != nil {
				return err
			}
		}
		if !a.play.entryControlsRestored && !a.play.exitScriptStarted {
			a.play.entryControlsRestored = a.play.enterAfterScript()
		}
		a.play.spinEndFinished = a.weaponPlayback.player == nil || !a.weaponPlayback.player.IsPlaying()
		inputs := a.gatherPlayerInputs()
		a.play.input = inputs[0]
		if a.play.Update(x, y, ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft), inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft), a.mobile) {
			if a.play.weapon.GunType != "MINIGUN" || a.play.shotSound != a.play.weapon.SFXShoot {
				a.playWeaponSound(a.play.shotSound)
			}
		}
		if !a.play.paused {
			a.play.updateCoopPlayers(inputs[1:])
		}
		if a.netHosting() {
			a.netHostSend()
		}
		if err := a.updateGameplayAchievements(a.play); err != nil {
			return err
		}
		if a.play.weapon.GunType != "MINIGUN" && a.play.weapon.GunType != "BUZZSAW" && a.play.sentryLoopHold <= 0 && (a.play.paused || !a.play.shootControl || (a.mobile && a.play.stick != 2) || (!a.mobile && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && !ebiten.IsKeyPressed(ebiten.KeySpace))) {
			a.stopWeaponPlayback()
		}
		if a.play.shouldQuit {
			a.stopWeaponPlayback()
			a.play.closeScript()
			a.play = nil
			a.setMenuMusic()
			return nil
		}
		if err := a.updateBossScripts(); err != nil {
			return err
		}
		return a.updateLevelCompletion()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if a.page == 2 {
			a.leaveLevelSelect()
		} else if a.page > 0 {
			a.page--
		}
		return nil
	}
	if a.page == 2 {
		if handled, err := a.updateLevelSelectInput(); handled || err != nil {
			return err
		}
		if step := a.levelSelectWheelStep(); step != 0 {
			a.move(step)
			a.playSound("audio/sound/sfx/menu_move.ogg", .7)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		a.move(1)
		a.playSound("audio/sound/sfx/menu_move.ogg", .7)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		a.move(-1)
		a.playSound("audio/sound/sfx/menu_move.ogg", .7)
	}
	if a.page == 2 && inpututil.IsKeyJustPressed(ebiten.KeyC) {
		return a.activateCoop()
	}
	if a.page == 2 && inpututil.IsKeyJustPressed(ebiten.KeyH) {
		a.startNetHost()
		return nil
	}
	if a.page == 2 && inpututil.IsKeyJustPressed(ebiten.KeyJ) {
		a.startNetJoin()
		return nil
	}
	if a.page == 2 && (inpututil.IsKeyJustPressed(ebiten.KeyLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA)) {
		a.move(-1)
		a.playSound("audio/sound/sfx/menu_move.ogg", .7)
	}
	if a.page == 2 && (inpututil.IsKeyJustPressed(ebiten.KeyRight) || inpututil.IsKeyJustPressed(ebiten.KeyD)) {
		a.move(1)
		a.playSound("audio/sound/sfx/menu_move.ogg", .7)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) {
		a.playSound("audio/sound/sfx/menu_select.ogg", .8)
		if a.page == 0 {
			return a.beginMenuClick(a.menuSelection)
		}
		return a.activate()
	}
	if a.page != 2 && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		px, py := a.pointer()
		if a.page == 0 {
			if index := a.mainMenuHit(px, py); index >= 0 {
				a.menuSelection = index
				a.playSound("audio/sound/sfx/menu_select.ogg", .8)
				return a.beginMenuClick(index)
			}
		} else {
			index := (py - 96) / 28
			if index >= 0 && index < len(a.items()) {
				a.setCursor(index)
				a.playSound("audio/sound/sfx/menu_select.ogg", .8)
				return a.activate()
			}
		}
	}
	if a.page == 0 {
		px, py := a.pointer()
		if index := a.mainMenuHit(px, py); index >= 0 {
			a.menuSelection = index
		}
	} else if a.page < 3 && a.page != 2 {
		_, y := a.pointer()
		index := (y - 96) / 28
		if index >= 0 && index < len(a.items()) {
			a.setCursor(index)
		}
	}
	return nil
}
func (a *app) leaveLevelSelect() {
	a.page, a.level, a.carouselReady, a.selectorPointerDown, a.selectorDragging = 0, 0, false, false, false
}
func (a *app) syncLevelCarousel() carousel.LevelCarousel {
	levels := carousel.CatalogLevelCarousel(a.levels, a.mode)
	index := levels.Index(a.world, a.level)
	if !a.carouselReady || a.carouselMode != a.mode || a.carouselCore.Count != len(levels) {
		a.carouselCore, a.carouselMode, a.carouselReady = carousel.NewFiniteCore(len(levels), index), a.mode, true
	} else if index != a.carouselCore.Selected {
		a.carouselCore.SelectIndex(index)
	}
	if world, level, ok := levels.Selection(a.carouselCore.Selected); ok {
		a.world, a.level = world, level
	}
	return levels
}
func (a *app) levelCardX(index int) float64 {
	offset := min(max(a.carouselCore.Position-1, 0), float64(max(0, a.carouselCore.Count-3)))
	return 90 + (float64(index)-offset)*150
}
// levelSelectWheelStep turns mouse-wheel movement into one level step at a time;
// scrolling down (or right) moves to the next level.
func (a *app) levelSelectWheelStep() int {
	wheelX, wheelY := ebiten.Wheel()
	a.selectorWheel += wheelX - wheelY
	switch {
	case a.selectorWheel >= 1:
		a.selectorWheel = 0
		return 1
	case a.selectorWheel <= -1:
		a.selectorWheel = 0
		return -1
	}
	return 0
}

func (a *app) updateLevelSelectInput() (bool, error) {
	a.syncLevelCarousel()
	x, y := a.pointer()
	down, pressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft), inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	touches := ebiten.AppendTouchIDs(nil)
	if a.selectorPointerDown && a.selectorTouch {
		down, pressed, x, y = false, false, a.selectorLastX, a.selectorLastY
		for _, id := range touches {
			if id == a.selectorTouchID {
				x, y = ebiten.TouchPosition(id)
				down = true
				break
			}
		}
	} else if !a.selectorPointerDown {
		a.selectorTouch = false
		if started := inpututil.AppendJustPressedTouchIDs(nil); len(started) > 0 {
			a.selectorTouch, a.selectorTouchID = true, started[0]
			x, y = ebiten.TouchPosition(started[0])
			down, pressed = true, true
		}
	}
	if a.selectorTouch && a.outputWidth > 0 && a.outputHeight > 0 && down {
		x, y = x*logicalWidth/a.outputWidth, y*logicalHeight/a.outputHeight
	}
	return a.updateLevelSelectPointer(x, y, pressed, down)
}
func (a *app) updateLevelSelectPointer(x, y int, pressed, down bool) (bool, error) {
	levels := a.syncLevelCarousel()
	if pressed {
		if action := a.levelActionAt(x, y); action == "back" {
			a.leaveLevelSelect()
			return true, nil
		} else if action == "play" {
			a.playSound("audio/sound/sfx/menu_select.ogg", .8)
			return true, a.activate()
		} else if action == "coop" {
			a.playSound("audio/sound/sfx/menu_select.ogg", .8)
			return true, a.activateCoop()
		}
		if mode := a.levelTabAt(x, y); mode >= 0 {
			a.mode, a.world, a.level, a.carouselReady = mode, 0, 0, false
			a.syncLevelCarousel()
			a.playSound("audio/sound/sfx/menu_select.ogg", .8)
			return true, nil
		}
		if y >= 60 && y <= 140 {
			a.selectorPointerDown, a.selectorDragging = true, false
			a.selectorStartX, a.selectorLastX, a.selectorLastY, a.selectorPressIndex = x, x, y, a.levelHit(x, y)
			return true, nil
		}
	}
	if !a.selectorPointerDown {
		return false, nil
	}
	if math.Abs(float64(x-a.selectorStartX)) > 6 {
		a.selectorDragging = true
	}
	if a.selectorDragging {
		if a.carouselCore.Scroll(float64(a.selectorLastX-x) / 150) {
			a.playSound("audio/sound/sfx/menu_move.ogg", .7)
		}
		if world, level, ok := levels.Selection(a.carouselCore.Selected); ok {
			a.world, a.level = world, level
		}
		a.selectorLastX = x
	}
	a.selectorLastY = y
	if down {
		return true, nil
	}
	a.selectorPointerDown = false
	if a.selectorDragging {
		a.carouselCore.Settle()
		a.selectorDragging = false
		return true, nil
	}
	index := a.levelHit(x, y)
	if index >= 0 && index == a.selectorPressIndex {
		// A single click only selects the card (PLAY starts the level); a second
		// click on the same card within half a second plays it.
		doubleClick := a.lastCardClick.valid && a.lastCardClick.index == index && a.menuTime-a.lastCardClick.at <= .5
		a.setCursor(index)
		if doubleClick && a.levelUnlocked(levels[index].Info) {
			a.lastCardClick.valid = false
			a.playSound("audio/sound/sfx/menu_select.ogg", .8)
			return true, a.activate()
		}
		a.lastCardClick = cardClick{index: index, at: a.menuTime, valid: true}
	}
	return true, nil
}
func (a *app) mainMenuHit(x, y int) int {
	for i, button := range mainMenuButtons {
		dx, dy := float64(x)-button.cx, float64(y)-button.cy
		if i < len(a.menuZombies) {
			motion := a.menuZombies[i]
			if layout := a.mainMenuLayout(button.action); layout != nil {
				// MainScreen.uiscreen's Button component is the native hit rectangle.
				if menuGroupHit(layout, motion, float64(x), float64(y)) {
					return i
				}
				continue
			}
			if !motion.Visible || motion.Scale <= 0 {
				continue
			}
			dx, dy = float64(x)-float64(motion.X), float64(y)-float64(motion.Y)
			angle := float64(motion.RotationDegrees) * math.Pi / 180
			cosine, sine := math.Cos(angle), math.Sin(angle)
			envelope := float64(motion.Scale / motion.BaseScale)
			dx, dy = (cosine*dx-sine*dy)/envelope, (sine*dx+cosine*dy)/envelope
		}
		cosine, sine := math.Cos(button.angle), math.Sin(button.angle)
		localX, localY := cosine*dx+sine*dy, -sine*dx+cosine*dy
		width, height := button.width, button.height
		if size, ok := a.variables.Vec2Value("MAINMENU_AOZ_BUTTON_SIZE_VAR"); ok {
			width, height = size.X, size.Y
		}
		if math.Abs(localX) <= width/2 && math.Abs(localY) <= height/2 {
			return i
		}
	}
	return -1
}
func (a *app) Draw(screen *ebiten.Image) {
	// Hide the OS cursor during play so the red reticule crosshair shows instead.
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	if a.play != nil && !a.mobile && !a.play.paused && !a.sandboxOpen && a.page != 3 && a.page != 4 && a.page != 5 && a.page != 6 && !a.play.storyGameOver(a.mode) {
		ebiten.SetCursorMode(ebiten.CursorModeHidden)
	}
	a.frontendScaleX = float64(screen.Bounds().Dx()) / logicalWidth
	a.frontendScaleY = float64(screen.Bounds().Dy()) / logicalHeight
	if a.play != nil && a.view == nil && !a.titleScreen && a.page != 3 && a.page != 4 && a.page != 5 && a.page != 6 && a.startupFrames <= 0 {
		screen.Fill(color.Black)
	} else {
		screen.Fill(colorDark)
	}
	if a.net != nil && a.net.lobby {
		a.drawNetLobby(screen)
	} else if a.startupFrames > 0 {
		a.drawStartup(screen)
	} else if a.titleScreen {
		a.drawTitle(screen)
	} else if a.view != nil {
		a.view.SetRenderScale(a.frontendScaleX, a.frontendScaleY)
		a.view.Draw(screen)
		a.drawDebugPanel(screen)
	} else if a.page == 5 && a.resultsScreen != nil {
		a.drawResultsMenu(screen, a.resultsScreen, a.menuTime)
		if a.netGuest() {
			a.drawGuestResultsOverlay(screen)
		}
	} else if a.page == 6 {
		a.drawAchievements(screen)
	} else if a.page == 3 || a.page == 4 {
		a.drawMenu(screen)
	} else if a.play != nil {
		a.play.world.SetRenderScale(a.frontendScaleX, a.frontendScaleY)
		a.drawPlay(screen)
	} else {
		a.drawMenu(screen)
	}
	a.drawConfirm(screen)
	a.drawAchievementToasts(screen)
	if a.capture != nil {
		if err := a.capture.Save(screen, a.captureState()); err != nil {
			log.Printf("capture: %v", err)
			a.capture = nil
		}
	}
	a.frontendScaleX = 1
	a.frontendScaleY = 1
}

func (a *app) drawImage(screen, source *ebiten.Image, options *ebiten.DrawImageOptions) {
	scaled := *options
	scaled.Filter = ebiten.FilterNearest
	if a.frontendScaleX != 1 || a.frontendScaleY != 1 {
		scaled.GeoM.Scale(a.frontendScaleX, a.frontendScaleY)
	}
	screen.DrawImage(source, &scaled)
}
func (a *app) renderScale() (float64, float64) {
	scaleX, scaleY := a.frontendScaleX, a.frontendScaleY
	if scaleX <= 0 {
		scaleX = 1
	}
	if scaleY <= 0 {
		scaleY = 1
	}
	return scaleX, scaleY
}
func (a *app) drawRect(screen *ebiten.Image, x, y, width, height float64, clr color.Color) {
	scaleX, scaleY := a.renderScale()
	ebitenutil.DrawRect(screen, x*scaleX, y*scaleY, width*scaleX, height*scaleY, clr)
}

func (a *app) captureState() string {
	if a.startupFrames > 0 {
		return "loading"
	}
	if a.titleScreen {
		return "title"
	}
	if a.view != nil {
		return "debug-viewer"
	}
	if a.page == 3 {
		return "options"
	}
	if a.page == 4 {
		return "stats"
	}
	if a.page == 6 {
		return "achievements"
	}
	if a.play != nil {
		return "play"
	}
	switch a.page {
	case 0:
		return "main-menu"
	case 2:
		return "level-select"
	case 3:
		return "options"
	case 4:
		return "stats"
	case 5:
		return "results"
	default:
		return "frontend"
	}
}

func (a *app) selectCaptureLevel(id string) error {
	var target formats.LevelInfo
	found := false
	for _, item := range a.levels {
		if strings.EqualFold(item.ID, strings.TrimSpace(id)) {
			target, found = item, true
			break
		}
	}
	if !found {
		return fmt.Errorf("capture level %q not found", id)
	}
	a.mode = 0
	if hasLevelFlag(target, "SURVIVAL") {
		a.mode = 1
	}
	a.world = -1
	for index, world := range a.worlds() {
		if world == target.WorldIndex {
			a.world = index
			break
		}
	}
	if a.world < 0 {
		return fmt.Errorf("capture level %q has no world entry", id)
	}
	a.level = -1
	for index, item := range a.filteredLevels() {
		if strings.EqualFold(item.ID, target.ID) {
			a.level = index
			break
		}
	}
	if a.level < 0 {
		return fmt.Errorf("capture level %q is not in the selected mode", id)
	}
	a.titleScreen, a.page = false, 2
	return nil
}

func (a *app) setCaptureState(state string) error {
	a.startupFrames = 0
	normalized := strings.TrimSpace(state)
	switch strings.ToLower(normalized) {
	case "loading":
		a.startupFrames = 45
		a.titleScreen = true
	case "title":
		a.titleScreen = true
	case "title-cocking":
		a.titleScreen, a.titleCocking, a.titleSoundElapsed = true, true, 8.0/60.0
	case "main-menu":
		a.titleScreen, a.page, a.menuSelection, a.world, a.mode, a.level = false, 0, 1, 0, 0, 0
		a.menuSpawnTime, a.titleSoundStage = .2, 0
		for range 17 {
			a.updateMenuZombieMotion(float32(1.0 / 60.0))
		}
	case "main-menu-hit":
		a.titleScreen, a.page, a.menuSelection, a.world, a.mode, a.level = false, 0, 1, 0, 0, 0
		a.menuSpawnTime, a.titleSoundStage = 1, 0
		for range 17 {
			a.updateMenuZombieMotion(float32(1.0 / 60.0))
		}
		return a.beginMenuClick(1)
	case "options":
		a.titleScreen, a.page = false, 3
	case "stats":
		a.titleScreen, a.page = false, 4
		a.statsScreen = newStatsMenu(&a.statistics)
	case "achievements", "achievements-untracked":
		a.titleScreen = false
		a.openAchievements()
		if strings.EqualFold(normalized, "achievements-untracked") {
			a.achievementOffset = 5
		}
	case "play-camper-achievement":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 1, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.waveIndex = len(a.play.world.Level.Waves)
		a.play.zombies = nil
		for i := 0; i < 20; i++ {
			a.play.zombies = append(a.play.zombies, zombieState{x: a.play.x + 24, y: a.play.y, health: 100, size: formats.Vec2{X: 32, Y: 32}, texture: "girlzombiesheet", alpha: 1})
		}
		a.play.detonateGrenade(a.play.x+24, a.play.y)
		if err := a.updateGameplayAchievements(a.play); err != nil {
			return err
		}
		a.play.paused = true
		a.openAchievements()
		a.achievementOffset = 18
	case "results":
		a.titleScreen, a.page = false, 5
		kills, highscore := int32(135), int32(1210)
		a.resultsScreen = newResultsMenu(resultsData{Flags: 4, Score: 1045, Kills: &kills, Highscore: &highscore})
	case "level-select":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
	case "level-select-played":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play = nil
	case "play-paused", "play-options", "play-stats":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.paused = true
		if strings.EqualFold(normalized, "play-options") {
			return a.activatePauseMenu(1)
		}
		if strings.EqualFold(normalized, "play-stats") {
			return a.activatePauseMenu(2)
		}
	case "play":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		return a.openPlay()
	case "play-ready":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.hudVisible = true
		return nil
	case "play-device-controls":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		a.options.activate(optionsDefault, nil)
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.hudVisible = true
		return nil
	case "play-hq-dialogue":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		host := &playScriptHost{app: a, play: a.play}
		for _, item := range []struct {
			id, side int
			name     string
		}{{0, 0, "Cameos/HQcameo"}, {1, 1, "Cameos/HQcameo"}, {7, 0, "Cameos/barrycameo"}, {8, 1, "Cameos/cavezombiecameo"}} {
			for _, call := range []struct {
				name string
				args []scripting.Value
			}{{"LoadTexture", []scripting.Value{item.id, item.name}}, {"RegisterCameo", []scripting.Value{item.id, item.side}}, {"SetTextureVisible", []scripting.Value{item.id, true}}} {
				if _, err := host.Call(call.name, call.args); err != nil {
					return err
				}
			}
		}
		_, err := host.Call("StartSpeech", []scripting.Value{"tutorial intro"})
		return err
	case "play-fire", "play-fire-left", "play-shotgun", "play-uzi", "play-flamer", "play-sniper", "play-minigun", "play-buzzsaw", "play-dual":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.hudVisible = true
		direction := 1.0
		if strings.EqualFold(normalized, "play-shotgun") {
			a.play.collectPickup("p_shotgun")
		}
		if strings.EqualFold(normalized, "play-buzzsaw") {
			a.play.collectPickup("p_buzzsaw")
		}
		if strings.EqualFold(normalized, "play-dual") {
			a.play.collectPickup("p_dual_pistol")
		}
		if strings.EqualFold(normalized, "play-uzi") {
			a.play.collectPickup("p_uzi")
		}
		if strings.EqualFold(normalized, "play-flamer") {
			a.play.collectPickup("p_flamer")
		}
		if strings.EqualFold(normalized, "play-sniper") {
			a.play.collectPickup("p_sniper")
		}
		if strings.EqualFold(normalized, "play-minigun") {
			a.play.collectPickup("p_minigun")
			a.play.spinAudio.SpinTick(weaponBindings(a.play.weapon), true, float32(a.play.weapon.RateOfFire), false)
		}
		if strings.EqualFold(normalized, "play-fire-left") {
			direction = -1
		}
		a.play.angle, a.play.flipX = barryDirection(direction, 0)
		if strings.EqualFold(normalized, "play-dual") {
			// Let the dual pistol's own timer elapse so the capture shows a shot.
			a.play.tickGun(true)
			a.play.gun.Timer = 1
		}
		a.play.fire(direction, 0)
		return nil
	case "play-dead":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.health, a.play.lives, a.play.deathTimer, a.play.score = 0, 0, 0, 1234
		return nil
	case "play-sandbox", "play-sandbox-zombies", "play-sandbox-pickups":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.sandboxOpen = true
		a.sandboxTab = map[string]int{"play-sandbox": 0, "play-sandbox-zombies": 1, "play-sandbox-pickups": 2}[normalized]
		return nil
	case "achievement-toast":
		a.titleScreen = false
		if len(a.achievements) > 0 {
			delete(a.achievementUnlocks, a.achievements[7].ID)
			a.unlockAchievement(a.achievements[7])
		}
		return nil
	case "play-confirm":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.paused = true
		a.askConfirm("QUIT TO MENU?", a.quitToMenu)
		return nil
	case "play-wave-banner":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.hudVisible = true
		a.play.noteWave()
		a.play.waveIndex = 1
		return nil
	case "play-pause-hover":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.paused = true
		a.hover = map[string]float64{"pauseOPTIONS": 1}
		return nil
	case "play-train":
		if err := a.selectCaptureLevel("World5Level1"); err != nil {
			return err
		}
		a.titleScreen, a.page = false, 2
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.hudVisible = true
		a.play.spawnTrain()
		a.play.train.countdown = 0.5
		a.play.x, a.play.y = 1450, 440
		a.play.centerCamera()
		return nil
	case "net-join":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		a.startNetJoin()
		return nil
	case "net-host":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		a.startNetHost()
		return nil
	case "play-coop":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		a.coopPlayers = 3
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.hudVisible, a.play.moveControl, a.play.shootControl = true, true, true
		for _, offset := range []float64{110, 150, 190} {
			a.play.zombies = append(a.play.zombies, zombieState{x: a.play.x - 60 + offset, y: a.play.y + 90, speed: 0, health: 300, rawPoints: 100, size: formats.Vec2{X: 48, Y: 48}, collision: 15, texture: "girlzombiesheet", alpha: 1, fps: a.play.spriteFPS("girlzombiesheet", "")})
		}
		return nil
	case "play-sentry", "play-mine", "play-bazooka":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.hudVisible, a.play.moveControl, a.play.shootControl = true, true, true
		pickup := map[string]string{"play-sentry": "p_sentryUzi", "play-mine": "p_mine", "play-bazooka": "p_bazooka"}[strings.ToLower(normalized)]
		a.play.collectPickup(pickup)
		a.play.fireSecondary(1, 0)
		for _, offset := range []float64{90, 130, 170} {
			a.play.zombies = append(a.play.zombies, zombieState{x: a.play.x + 80 + offset, y: a.play.y + offset/3, speed: 0, health: 300, rawPoints: 100, size: formats.Vec2{X: 48, Y: 48}, texture: "girlzombiesheet", alpha: 1, fps: a.play.spriteFPS("girlzombiesheet", "")})
		}
		return nil
	case "play-sentry-behind", "play-sentry-front", "play-sentry-side":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.hudVisible, a.play.moveControl, a.play.shootControl = true, true, true
		a.play.collectPickup("p_sentryUzi")
		a.play.fireSecondary(1, 0)
		if len(a.play.sentries) > 0 {
			t := a.play.sentries[0]
			dy := -14.0
			if strings.EqualFold(normalized, "play-sentry-front") {
				dy = 14
			}
			a.play.x, a.play.y = t.x+4, t.y+dy
			if strings.EqualFold(normalized, "play-sentry-side") {
				a.play.x, a.play.y = t.x+110, t.y+70
			}
		}
		for _, offset := range []float64{90, 140} {
			a.play.zombies = append(a.play.zombies, zombieState{x: a.play.x + offset, y: a.play.y + 30, speed: 0, health: 3000, rawPoints: 100, size: formats.Vec2{X: 48, Y: 48}, texture: "girlzombiesheet", alpha: 1, fps: a.play.spriteFPS("girlzombiesheet", "")})
		}
		return nil
	case "play-exploding", "play-exploding-blast":
		return a.captureExplodingScene(strings.ToLower(normalized) == "play-exploding-blast")
	case "play-native-title":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.waveIndex = len(a.play.world.Level.Waves)
		host := &playScriptHost{app: a, play: a.play}
		if _, err := host.Call("DrawText1", []scripting.Value{240, 20, "SHOOT IT UP IN DINOSAUR TIMES,"}); err != nil {
			return err
		}
		_, err := host.Call("DrawText2", []scripting.Value{240, 40, "STEAKFRIES!"})
		return err
	case "play-combo":
		return a.captureComboScene()
	case "play-zoo":
		return a.captureZombieZoo()
	case "play-combat":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.hudVisible = true
		a.play.zombies = []zombieState{{x: a.play.x + 120, y: a.play.y, speed: 0, health: 100, rawPoints: 100, size: formats.Vec2{X: 29, Y: 31}, texture: "girlzombiesheet", alpha: 1}}
		a.play.angle, a.play.flipX = barryDirection(1, 0)
		a.play.fire(1, 0)
		return nil
	case "play-zombie-death":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.hudVisible = true
		a.play.waveIndex = len(a.play.world.Level.Waves)
		id := a.play.scriptNextEntity
		a.play.scriptNextEntity++
		a.play.scriptEntities[id] = &scriptEntity{id: id, kind: "zombie", entityType: "girlzombie", x: a.play.x + 96, y: a.play.y, scaleX: 1, scaleY: 1, alpha: 1, texture: "girlzombiesheet"}
		a.play.zombies = []zombieState{{x: a.play.x + 96, y: a.play.y, speed: 0, health: 100, rawPoints: 100, size: formats.Vec2{X: 32, Y: 32}, texture: "girlzombiesheet", scriptID: id, alpha: 1, fps: a.play.spriteFPS("girlzombiesheet", "")}}
		host := &playScriptHost{app: a, play: a.play}
		_, err := host.Call("KillZombie", []scripting.Value{id})
		return err
	case "play-zombie-shadow":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.hudVisible = true
		a.play.waveIndex = len(a.play.world.Level.Waves)
		a.play.zombies = []zombieState{{x: a.play.x - 48, y: a.play.y + 64, health: 100, rawPoints: 100, size: formats.Vec2{X: 48, Y: 48}, texture: "girlzombiesheet", alpha: 1}}
		return nil
	case "play-tutorial-images":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.hudVisible = false
		a.play.waveIndex = len(a.play.world.Level.Waves)
		host := &playScriptHost{app: a, play: a.play}
		for _, item := range []struct {
			id                            int
			x, y, v, height, scale, alpha float64
		}{{3, 416, 108, .5, .35, .35, 125}, {6, 64, 112, 0, .35, .36, 255}, {9, 64, 200, .36, .14, .15, 255}, {10, 416, 200, .85, .15, .15, 125}} {
			for _, call := range []struct {
				name string
				args []scripting.Value
			}{{"LoadTexture", []scripting.Value{item.id, "Tutorial_Image"}}, {"SetTexturePos", []scripting.Value{item.id, item.x, item.y}}, {"SetTextureUVs", []scripting.Value{item.id, 0, item.v, 1, item.height}}, {"SetTextureScale", []scripting.Value{item.id, 1, item.scale}}, {"SetTextureAlpha", []scripting.Value{item.id, item.alpha}}, {"SetTextureVisible", []scripting.Value{item.id, true}}} {
				if _, err := host.Call(call.name, call.args); err != nil {
					return err
				}
			}
		}
		return nil
	case "play-ground-gap":
		if err := a.selectCaptureLevel("World0Survival0"); err != nil {
			return err
		}
		a.titleScreen, a.page = false, 2
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.x, a.play.y = 912, 176
		a.play.world.CameraX, a.play.world.CameraY, a.play.world.Zoom = 672, 16, 1
		a.play.waveIndex = len(a.play.world.Level.Waves)
		a.play.hudVisible = true
		return nil
	case "play-level-exit":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.waveIndex = len(a.play.world.Level.Waves) - 1
		wave := a.play.world.Level.Waves[a.play.waveIndex]
		a.play.waveElapsed = wave.RunTime + wave.EndWaveTime
		a.play.waveEndInit, a.play.waveEndTimer, a.play.waveEndStamp = true, 1, a.play.waveElapsed
		a.play.levelKills = a.play.levelZombieTotal
		a.play.updateWaves()
		return a.updateLevelCompletion()
	case "play-grenade", "play-grenade-explosion":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 1, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.hudVisible = true
		a.play.waveIndex = len(a.play.world.Level.Waves)
		a.play.grenades = 1
		a.play.zombies = []zombieState{{x: a.play.x, y: a.play.y + 140, speed: 0, health: 100, size: formats.Vec2{X: 32, Y: 32}, texture: "girlzombiesheet", alpha: 1}}
		if strings.EqualFold(normalized, "play-grenade-explosion") {
			a.play.detonateGrenade(a.play.x, a.play.y+80)
			return nil
		}
		a.play.angle, a.play.flipX = barryDirection(0, 1)
		a.play.fireSecondary(0, 1)
		return nil
	case "play-pickup":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 1, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.hudVisible = true
		a.play.waveIndex = len(a.play.world.Level.Waves)
		a.play.spawnPickup("p_grenade", formats.Vec2{X: a.play.x + 64, Y: a.play.y})
		return nil
	case "play-pickup-collected":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 1, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.hudVisible = true
		a.play.waveIndex = len(a.play.world.Level.Waves)
		a.play.spawnPickup("p_grenade", formats.Vec2{X: a.play.x, Y: a.play.y})
		return nil
	case "play-secondary", "play-controls-visible", "play-controls-hidden":
		if strings.EqualFold(normalized, "play-controls-visible") {
			a.mobile = true
			a.options.activate(optionsVisible, nil)
		}
		if strings.EqualFold(normalized, "play-controls-hidden") {
			a.mobile = true
			a.options.activate(optionsHidden, nil)
		}
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 1, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.hudVisible = true
		a.play.waveIndex = len(a.play.world.Level.Waves)
		a.play.grenades = 1
		return nil
	case "play-portal":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 1, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.hudVisible = true
		a.play.waveIndex = len(a.play.world.Level.Waves)
		if len(a.play.world.Level.Waves) > 0 && len(a.play.world.Level.Waves[0].Spawners) > 0 {
			points := a.play.spawnPoints(a.play.world.Level.Waves[0].Spawners[0].Index)
			if len(points) > 0 {
				a.play.x, a.play.y = points[0].X, points[0].Y
				a.play.centerCamera()
				a.play.spawnZombie(a.play.world.Level.Waves[0].Spawners[0], 0)
			}
		}
		return nil
	case "play-zombie-portal":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 1, 0
		if err := a.openPlay(); err != nil {
			return err
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.hudVisible = true
		a.play.waveIndex = len(a.play.world.Level.Waves)
		id := a.play.scriptNextEntity
		a.play.scriptNextEntity++
		zombieX, zombieY := a.play.x+120, a.play.y
		a.play.scriptEntities[id] = &scriptEntity{id: id, kind: "zombie", entityType: "girlzombie", x: zombieX, y: zombieY, scaleX: 1, scaleY: 1, alpha: 1, texture: "girlzombiesheet", speed: 60}
		a.play.zombies = []zombieState{{x: zombieX, y: zombieY, speed: 60, health: 100, size: formats.Vec2{X: 32, Y: 32}, texture: "girlzombiesheet", scriptID: id, alpha: 1, fps: a.play.spriteFPS("girlzombiesheet", "")}}
		host := &playScriptHost{app: a, play: a.play}
		if _, err := host.Call("WalkZombieTo", []scripting.Value{id, a.play.x, a.play.y, 20}); err != nil {
			return err
		}
		a.play.addPortal(a.play.x, a.play.y)
		return nil
	case "play-rex-venom":
		return a.captureRexVenom()
	case "play-rex-shockwave":
		return a.captureRexShockwave()
	case "debug-viewer":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		return a.openViewer()
	default:
		const prefix = "play-level:"
		if strings.HasPrefix(strings.ToLower(normalized), prefix) {
			if err := a.selectCaptureLevel(normalized[len(prefix):]); err != nil {
				return err
			}
			return a.openPlay()
		}
		return fmt.Errorf("unknown capture state %q", state)
	}
	return nil
}
func (a *app) Layout(outsideWidth, outsideHeight int) (int, int) {
	if outsideWidth < 1 {
		outsideWidth = logicalWidth
	}
	if outsideHeight < 1 {
		outsideHeight = logicalHeight
	}
	a.outputWidth, a.outputHeight = outsideWidth, outsideHeight
	return outsideWidth, outsideHeight
}
func (a *app) pointer() (int, int) {
	x, y := ebiten.CursorPosition()
	if a.outputWidth < 1 || a.outputHeight < 1 {
		return x, y
	}
	return x * logicalWidth / a.outputWidth, y * logicalHeight / a.outputHeight
}
func (a *app) drawMenu(screen *ebiten.Image) {
	if a.page == 0 {
		a.drawBackdrop(screen)
		a.drawMainMenuBanner(screen)
		for index, button := range mainMenuButtons {
			if index >= len(a.menuZombies) || !a.menuZombies[index].Visible {
				continue
			}
			if a.menuClick != nil && a.menuClick.index == index {
				a.drawMenuClick(screen, button, *a.menuClick)
				continue
			}
			a.drawMarqueeButton(screen, button, a.menuZombies[index], index == a.menuSelection)
		}
	} else if a.page == 3 {
		a.drawBackdrop(screen)
		a.drawOptionsMenu(screen, &a.options)
	} else if a.page == 4 && a.statsScreen != nil {
		a.drawStatsMenu(screen, a.statsScreen, a.menuTime)
	} else {
		a.drawLevelSelect(screen)
	}
}

func (a *app) beginMenuClick(index int) error {
	if index < 0 || index >= len(mainMenuButtons) {
		return nil
	}
	button := mainMenuButtons[index]
	if len(a.menuZombies) != len(mainMenuButtons) {
		a.updateMenuZombieMotion(0)
	}
	for i := range a.menuZombies {
		state := menumotion.NativeMenuZombieLeaving
		if i == index {
			state = menumotion.NativeMenuZombieClicked
		}
		a.menuZombies[i].SetState(state)
	}
	a.menuSelection = index
	a.menuClick = &menuClickState{index: index, x: button.cx, y: button.cy, vx: -2, vy: -6}
	if index < len(a.menuZombies) {
		a.menuClick.x, a.menuClick.y = float64(a.menuZombies[index].X), float64(a.menuZombies[index].Y)
	}
	return nil
}

func (a *app) updateMenuClick() error {
	click := a.menuClick
	if click == nil {
		return nil
	}
	click.age += 1.0 / 60.0
	click.x += click.vx
	click.y += click.vy
	click.vx *= .97
	click.vy = (click.vy + 1.4) * .97
	if click.y > logicalHeight+64 {
		a.menuClick = nil
		return a.activate()
	}
	return nil
}

func (a *app) drawMenuClick(screen *ebiten.Image, button menuButton, click menuClickState) {
	motion := a.menuZombies[click.index]
	x, y := click.x-float64(motion.X), click.y-float64(motion.Y)
	if layout := a.mainMenuLayout(button.action); layout != nil {
		a.drawMenuClickNative(screen, layout, click, motion)
		return
	}
	if click.age < .125 {
		button.zombie = a.menuZombieArt(button)
		if zombie, err := a.Texture(fmt.Sprintf("Frontend0/Textures/menu_zombie_%d_SD", button.zombie)); err == nil {
			resolution := a.pack.TextureSourceScale(fmt.Sprintf("Frontend0/Textures/menu_zombie_%d_SD", button.zombie))
			options := menuGroupImageOptions(motion, .9/resolution, .9/resolution, float64(zombie.Bounds().Dx())/2, float64(zombie.Bounds().Dy())/2, x, y-8, 0)
			if click.age >= .04 {
				options.ColorScale.Scale(1.5, .18, .18, 1)
			}
			a.drawImage(screen, zombie, options)
		}
	}
	if click.age >= zombieHitFlashDuration {
		a.drawBloodPopSprite(screen, float64(motion.X), float64(motion.Y)-8, .9*float64(motion.Scale/motion.BaseScale), click.index%3, click.age-zombieHitFlashDuration)
	}
	flash, err := a.Texture("Common0/Textures/Button_Screen_Flash")
	if err != nil {
		return
	}
	source := flash.SubImage(image.Rect(0, 0, 140, 70)).(*ebiten.Image)
	a.drawImage(screen, source, menuGroupImageOptions(motion, .8, .8, 70, 35, x, y, button.angle))
}

func (a *app) drawTitle(screen *ebiten.Image) {
	a.drawBackdrop(screen)
	a.drawBanner(screen, "SPLASHSCREENS_AOZ_BANNER_POS_VAR", "SPLASHSCREENS_AOZ_BANNER_SIZE_VAR", .4)
	a.drawTitleBarry(screen)
	if image, err := a.Texture("Frontend0/Textures/menu_zombie_2_SD"); err == nil && a.titleZombieActive {
		resolution := a.pack.TextureSourceScale("Frontend0/Textures/menu_zombie_2_SD")
		options := menuGroupImageOptions(a.titleZombie, .5/resolution, .5/resolution, float64(image.Bounds().Dx())/2, float64(image.Bounds().Dy())/2, 0, 0, 0)
		a.drawImage(screen, image, options)
	}
	a.textCentered(screen, "Touch to Start", 290, .72)
}

type titleBarryPiece struct {
	source      image.Rectangle
	x, y, angle float64
}

var titleBarryPieces = []titleBarryPiece{
	{source: image.Rect(0, 0, 342, 512), x: 110, y: 190},                             // Barry body
	{source: image.Rect(342, 152, 512, 512), x: 164, y: 230, angle: math.Pi/2 - .25}, // shotgun
	{source: image.Rect(342, 0, 409, 152), x: 206, y: 220, angle: math.Pi/2 - .25},   // grip/pump
	{source: image.Rect(409, 0, 512, 152), x: 80, y: 250, angle: math.Pi/2 + .25},    // arm/hand
}

func (a *app) drawTitleBarry(screen *ebiten.Image) {
	texture, err := a.Texture("Frontend0/Textures/Barry")
	if err != nil {
		return
	}
	phase := 2 * math.Pi * (a.menuTime * 28000 / 65536)
	xOffset := math.Sin(phase+2*math.Pi*.5*28000/65536) * 1.5
	yOffset := math.Sin(phase) * 1.5
	for index, piece := range titleBarryPieces {
		part := texture.SubImage(piece.source).(*ebiten.Image)
		cockX, cockY := a.titleCockingOffset(index)
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Translate(-float64(piece.source.Dx())/2, -float64(piece.source.Dy())/2)
		options.GeoM.Scale(.5, .66)
		options.GeoM.Rotate(piece.angle)
		options.GeoM.Translate(piece.x+xOffset+cockX, piece.y+yOffset+cockY)
		a.drawImage(screen, part, options)
	}
}

func (a *app) titleCockingOffset(index int) (float64, float64) {
	if !a.titleCocking || index != 2 {
		return 0, 0
	}
	ticks := a.titleSoundElapsed * 60
	distance := 0.0
	switch {
	case ticks < 8:
		distance = ticks / 8 * 10
	case ticks < 16:
		distance = 10
	case ticks < 24:
		distance = (1 - (ticks-16)/8) * 10
	}
	axisX := math.Sin(titleBarryPieces[1].angle)
	axisY := math.Cos(titleBarryPieces[1].angle)
	return -axisX * distance, axisY * distance
}

func (a *app) drawBanner(screen *ebiten.Image, positionName, scaleName string, angle float64) {
	texture, err := a.Texture("Frontend0/Textures/Ageofzombies")
	if err != nil {
		return
	}
	position, ok := a.variables.Vec2Value(positionName)
	if !ok {
		return
	}
	scale, ok := a.variables.FloatValue(scaleName)
	if !ok {
		return
	}
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(texture.Bounds().Dx())/2, -float64(texture.Bounds().Dy())/2)
	options.GeoM.Scale(scale, scale)
	options.GeoM.Rotate(angle)
	options.GeoM.Translate(position.X, position.Y)
	a.drawImage(screen, texture, options)
}

func (a *app) drawLevelSelect(screen *ebiten.Image) {
	a.drawBackdrop(screen)
	a.drawLevelTabs(screen)
	levels := a.syncLevelCarousel()
	for index, entry := range levels {
		x := a.levelCardX(index)
		if x < -64 || x > logicalWidth+64 {
			continue
		}
		a.drawLevelCardAt(screen, entry.Info, x, 100, index == a.carouselCore.Selected, a.levelUnlocked(entry.Info))
	}
	if index := a.carouselCore.Selected; index >= 0 && index < len(levels) {
		a.drawLevelInfo(screen, levels[index].Info)
	}
}

func (a *app) drawLevelTabs(screen *ebiten.Image) {
	texture, err := a.Texture("ShopFront0/Textures/Shop/AOZ_StoreButtons_SD")
	if err != nil {
		return
	}
	position, positionOK := a.variables.Vec2Value("SHOPFRONT_TOPBUTTONS_POS_VAR")
	size, sizeOK := a.variables.Vec2Value("SHOPFRONT_TOPBUTTONS_SIZE")
	if !positionOK || !sizeOK {
		return
	}
	row := 0
	if a.mode == 1 {
		row = 1
	}
	resolution := a.pack.TextureSourceScale("ShopFront0/Textures/Shop/AOZ_StoreButtons_SD")
	source := texture.SubImage(resolutionRect(image.Rect(0, row*64, 256, row*64+64), resolution)).(*ebiten.Image)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-128*resolution, -32*resolution)
	options.GeoM.Scale(size.X/(256*resolution), size.Y/(64*resolution))
	options.GeoM.Translate(position.X, position.Y)
	a.drawImage(screen, source, options)
}

func (a *app) drawLevelCardAt(screen *ebiten.Image, item formats.LevelInfo, x, y float64, selected, unlocked bool) {
	if item.PostcardImage == "" {
		return
	}
	texture, err := a.Texture("ShopFront0/Textures/Shop/" + item.PostcardImage + "_SD")
	if err != nil {
		return
	}
	cardRect := image.Rect(0, 0, 128, 80)
	resolution := a.pack.TextureSourceScale("ShopFront0/Textures/Shop/" + item.PostcardImage + "_SD")
	cardRect = resolutionRect(cardRect, resolution)
	if !cardRect.In(texture.Bounds()) {
		return
	}
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-64*resolution, -40*resolution)
	options.GeoM.Scale(1/resolution, 1/resolution)
	options.GeoM.Translate(x, y)
	if !selected {
		options.ColorScale.ScaleAlpha(.42)
	}
	if !unlocked {
		options.ColorScale.ScaleAlpha(.58)
	}
	a.drawImage(screen, texture.SubImage(cardRect).(*ebiten.Image), options)
	a.drawCatalogLevelBadges(screen, item, x, y, 128, 80)
}

func (a *app) drawLevelInfo(screen *ebiten.Image, item formats.LevelInfo) {
	outerPos, ok := a.ndcBox("SHOPFRONT_TEXT_BOX_OUTER_POS_VAR", "SHOPFRONT_TEXT_BOX_OUTER_WIDTH_VAR", "SHOPFRONT_TEXT_BOX_OUTER_HEIGHT_VAR")
	if !ok {
		return
	}
	if texture, err := a.Texture("Common0/Textures/Backing_Square"); err == nil {
		drawNineSlice(a, screen, texture, image.Rect(0, 0, 64, 64), outerPos)
	}
	innerPos, innerOK := a.ndcBox("SHOPFRONT_TEXT_BOX_INNER_POS_VAR", "SHOPFRONT_TEXT_BOX_INNER_WIDTH_VAR", "SHOPFRONT_TEXT_BOX_INNER_HEIGHT_VAR")
	textX, textY := float64(outerPos.Min.X+24), float64(outerPos.Min.Y+18)
	if innerOK {
		textX, textY = float64(innerPos.Min.X+16), float64(innerPos.Min.Y+14)
	}
	if a.computerFont != nil {
		a.drawFont(screen, a.computerFont, item.DisplayName, textX, textY, .7)
	} else {
		a.text(screen, item.DisplayName, textX, textY, .6)
	}
	if item.Description != "" {
		description := strings.ReplaceAll(item.Description, "\\n", "\n")
		descriptionY := textY + 32
		if innerOK {
			descriptionY = float64(innerPos.Min.Y + 46)
		}
		if a.computerFont != nil {
			a.drawFont(screen, a.computerFont, description, textX, descriptionY, .45)
		} else {
			a.text(screen, description, textX, descriptionY, .45)
		}
	}
	playPosition, playOK := a.variables.Vec2Value("SHOPFRONT_PLAY_ICON_POS_VAR")
	playWidth, playWidthOK := a.variables.FloatValue("SHOPFRONT_PLAY_ICON_WIDTH_VAR")
	playHeight, playHeightOK := a.variables.FloatValue("SHOPFRONT_PLAY_ICON_HEIGHT_VAR")
	if playOK && playWidthOK && playHeightOK {
		a.drawShopAction(screen, "PLAY", playPosition.X, playPosition.Y, playWidth, playHeight, a.levelUnlocked(item))
	}
	if a.currentCoopLaunch().available {
		if x, y, width, height, ok := a.coopButtonRect(); ok {
			a.drawShopAction(screen, "CO-OP", x, y, width, height, a.levelUnlocked(item))
		}
	}
	a.textCentered(screen, "H  HOST ONLINE      J  JOIN ONLINE", 309, .35)
	backPosition, backOK := a.variables.Vec2Value("SHOPFRONT_BACK_ICON_NO_GLOBAL_POS_VAR")
	backWidth, backWidthOK := a.variables.FloatValue("SHOPFRONT_BACK_ICON_WIDTH_VAR")
	backHeight, backHeightOK := a.variables.FloatValue("SHOPFRONT_BACK_ICON_HEIGHT_VAR")
	if backOK && backWidthOK && backHeightOK {
		a.drawShopAction(screen, "BACK", backPosition.X, backPosition.Y, backWidth, backHeight, true)
	}
}

func (a *app) ndcBox(positionName, widthName, heightName string) (image.Rectangle, bool) {
	position, positionOK := a.variables.Vec2Value(positionName)
	width, widthOK := a.variables.FloatValue(widthName)
	height, heightOK := a.variables.FloatValue(heightName)
	if !positionOK || !widthOK || !heightOK {
		return image.Rectangle{}, false
	}
	centerX, centerY := position.X*logicalWidth, position.Y*logicalHeight
	return image.Rect(int(centerX-width*logicalWidth/2), int(centerY-height*logicalHeight/2), int(centerX+width*logicalWidth/2), int(centerY+height*logicalHeight/2)), true
}

func (a *app) drawShopAction(screen *ebiten.Image, label string, x, y, width, height float64, enabled bool) {
	if texture, err := a.Texture("Common0/Textures/Backing_Square"); err == nil {
		drawNineSlice(a, screen, texture, image.Rect(0, 0, 64, 64), image.Rect(int(x-width/2), int(y-height/2), int(x+width/2), int(y+height/2)))
	}
	if label == "CO-OP" {
		if a.font != nil && a.font.LineHeight > 0 {
			scale := 15.0 / float64(a.font.LineHeight)
			a.drawFont(screen, a.font, label, x-a.fontTextWidth(label, scale)/2, y-7.5, scale)
		}
		return
	}
	row := 0
	if label == "BACK" {
		row = 9
	}
	texture, err := a.Texture("Common0/Textures/Button_Text_SD")
	if err != nil {
		return
	}
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	textRect, ok := buttonTextRect(row)
	if !ok {
		return
	}
	textRect = resolutionRect(textRect, a.pack.TextureSourceScale("Common0/Textures/Button_Text_SD"))
	options.GeoM.Translate(-float64(textRect.Dx())/2, -float64(textRect.Dy())/2)
	options.GeoM.Scale(width/float64(textRect.Dx()), height/float64(textRect.Dy()))
	options.GeoM.Translate(x, y)
	if !enabled {
		options.ColorScale.ScaleAlpha(.4)
	}
	a.drawImage(screen, texture.SubImage(textRect).(*ebiten.Image), options)
}

func buttonTextRect(row int) (image.Rectangle, bool) {
	switch row {
	case 0:
		return image.Rect(0, 0, 128, 32), true
	case 6:
		return image.Rect(0, 92, 128, 114), true
	case 8:
		return image.Rect(0, 128, 128, 148), true
	case 9:
		return image.Rect(0, 144, 128, 176), true
	case 14:
		return image.Rect(0, 224, 128, 240), true
	default:
		return image.Rectangle{}, false
	}
}
func resolutionRect(rect image.Rectangle, scale float64) image.Rectangle {
	return image.Rect(int(float64(rect.Min.X)*scale), int(float64(rect.Min.Y)*scale), int(float64(rect.Max.X)*scale), int(float64(rect.Max.Y)*scale))
}

func drawNineSlice(a *app, screen, texture *ebiten.Image, source, target image.Rectangle) {
	const edge = 8
	sourceParts := []image.Rectangle{
		image.Rect(source.Min.X, source.Min.Y, source.Min.X+edge, source.Min.Y+edge),
		image.Rect(source.Min.X+edge, source.Min.Y, source.Max.X-edge, source.Min.Y+edge),
		image.Rect(source.Max.X-edge, source.Min.Y, source.Max.X, source.Min.Y+edge),
		image.Rect(source.Min.X, source.Min.Y+edge, source.Min.X+edge, source.Max.Y-edge),
		image.Rect(source.Min.X+edge, source.Min.Y+edge, source.Max.X-edge, source.Max.Y-edge),
		image.Rect(source.Max.X-edge, source.Min.Y+edge, source.Max.X, source.Max.Y-edge),
		image.Rect(source.Min.X, source.Max.Y-edge, source.Min.X+edge, source.Max.Y),
		image.Rect(source.Min.X+edge, source.Max.Y-edge, source.Max.X-edge, source.Max.Y),
		image.Rect(source.Max.X-edge, source.Max.Y-edge, source.Max.X, source.Max.Y),
	}
	targetParts := []image.Rectangle{
		image.Rect(target.Min.X, target.Min.Y, target.Min.X+edge, target.Min.Y+edge),
		image.Rect(target.Min.X+edge, target.Min.Y, target.Max.X-edge, target.Min.Y+edge),
		image.Rect(target.Max.X-edge, target.Min.Y, target.Max.X, target.Min.Y+edge),
		image.Rect(target.Min.X, target.Min.Y+edge, target.Min.X+edge, target.Max.Y-edge),
		image.Rect(target.Min.X+edge, target.Min.Y+edge, target.Max.X-edge, target.Max.Y-edge),
		image.Rect(target.Max.X-edge, target.Min.Y+edge, target.Max.X, target.Max.Y-edge),
		image.Rect(target.Min.X, target.Max.Y-edge, target.Min.X+edge, target.Max.Y),
		image.Rect(target.Min.X+edge, target.Max.Y-edge, target.Max.X-edge, target.Max.Y),
		image.Rect(target.Max.X-edge, target.Max.Y-edge, target.Max.X, target.Max.Y),
	}
	for index := range sourceParts {
		if sourceParts[index].Dx() <= 0 || sourceParts[index].Dy() <= 0 || targetParts[index].Dx() <= 0 || targetParts[index].Dy() <= 0 {
			continue
		}
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Scale(float64(targetParts[index].Dx())/float64(sourceParts[index].Dx()), float64(targetParts[index].Dy())/float64(sourceParts[index].Dy()))
		options.GeoM.Translate(float64(targetParts[index].Min.X), float64(targetParts[index].Min.Y))
		a.drawImage(screen, texture.SubImage(sourceParts[index]).(*ebiten.Image), options)
	}
}

type menuButton struct {
	zombie, labelRow, action     int
	cx, cy, width, height, angle float64
}

type menuClickState struct {
	index        int
	age          float64
	x, y, vx, vy float64
}

var mainMenuButtons = []menuButton{
	{zombie: 3, labelRow: 8, action: 2, cx: 76, cy: 105, width: 128, height: 64, angle: -0.10},
	{zombie: 2, labelRow: 0, action: 0, cx: 210, cy: 194, width: 128, height: 64, angle: 0.40},
	{zombie: 3, labelRow: 6, action: 4, cx: 76, cy: 274, width: 128, height: 64, angle: -0.16},
	{zombie: 2, labelRow: 14, action: 3, cx: 377, cy: 264, width: 128, height: 64, angle: 0.02},
}

func (a *app) drawMainMenuBanner(screen *ebiten.Image) {
	a.drawBanner(screen, "MAINMENU_AOZ_BANNER_POS_VAR", "SPLASHSCREENS_AOZ_BANNER_SIZE_VAR", .4)
}

func (a *app) updateTitleZombieMotion(dt float32) {
	if !a.titleScreen || a.startupFrames > 0 {
		a.titleZombieActive = false
		return
	}
	if !a.titleZombieActive {
		base, ok := a.variables.FloatValue("SPLASHSCREENS_ZOMBIE_BASE_DIST_VAR")
		if !ok {
			return
		}
		rng := weapons.NewNativeRNG()
		x, y := a.titleZombiePosition(base)
		a.titleZombie = menumotion.NewNativeMenuZombieMotion(float32(x), float32(y), 0, 1, weapons.NativeWeaponRandomFloat(&rng, 3))
		a.titleZombie.SetState(menumotion.NativeMenuZombieEntering)
		a.titleZombieActive = true
		return
	}
	a.titleZombie.Tick(dt)
}

// titleZombiePosition is the centre of the title zombie at rest.
func (a *app) titleZombiePosition(base float64) (float64, float64) {
	width := 0.0
	if image, err := a.Texture("Frontend0/Textures/menu_zombie_2_SD"); err == nil {
		width = float64(image.Bounds().Dx()) / a.pack.TextureSourceScale("Frontend0/Textures/menu_zombie_2_SD")
	}
	return logicalWidth - base/2 + width*.375, 178
}

func (a *app) updateMenuZombieMotion(dt float32) {
	a.updateTitleZombieMotion(dt)
	if a.titleScreen || a.page != 0 || a.play != nil || a.view != nil {
		a.menuMotionActive = false
		return
	}
	if !a.menuMotionActive {
		rng := weapons.NewNativeRNG()
		a.menuZombies = make([]menumotion.NativeMenuZombieMotion, len(mainMenuButtons))
		for i, button := range mainMenuButtons {
			scale := float32(.75)
			if i == 1 {
				scale = 1
			}
			a.menuZombies[i] = menumotion.NewNativeMenuZombieMotion(float32(button.cx), float32(button.cy), 0, scale, weapons.NativeWeaponRandomFloat(&rng, 3))
			a.menuZombies[i].SetState(menumotion.NativeMenuZombieEntering)
		}
		a.menuMotionActive = true
		return
	}
	for i := range a.menuZombies {
		a.menuZombies[i].Tick(dt)
	}
}

func menuGroupImageOptions(motion menumotion.NativeMenuZombieMotion, scaleX, scaleY, offsetX, offsetY, x, y, angle float64) *ebiten.DrawImageOptions {
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-offsetX, -offsetY)
	options.GeoM.Scale(scaleX, scaleY)
	options.GeoM.Rotate(angle)
	options.GeoM.Translate(x, y)
	envelope := float64(motion.Scale / motion.BaseScale)
	options.GeoM.Scale(envelope, envelope)
	// The native local matrix (0x0044540c via 0x004108cc) rotates by -rotation.z,
	// so a positive property value turns the group counter-clockwise on screen.
	options.GeoM.Rotate(-float64(motion.RotationDegrees) * math.Pi / 180)
	options.GeoM.Translate(float64(motion.X), float64(motion.Y))
	return options
}

// menuZombieArt picks the zombie artwork: 1.2.5's MainScreen gives Quit
// Menu_Zombie_2 (shared with Play) and Stats Menu_Zombie_1, while the older menu
// used zombie 3 for Quit and zombie 2 for Stats.
func (a *app) menuZombieArt(button menuButton) int {
	hd := a.pack.TextureSourceScale("Frontend0/Textures/menu_zombie_2_SD") > 1
	if button.action == 4 && hd {
		return 2
	}
	if button.action == 3 && hd { // 1.2.5's Stats zombie is the mummy, Menu_Zombie_1
		return 1
	}
	return button.zombie
}

// drawMarqueeButton draws a main menu zombie group: from MainScreen.uiscreen when
// the pack has it (hands, body, board, label all from the data), otherwise the
// XML-era board layout the older SD cache needs.
func (a *app) drawMarqueeButton(screen *ebiten.Image, button menuButton, motion menumotion.NativeMenuZombieMotion, selected bool) {
	if layout := a.mainMenuLayout(button.action); layout != nil {
		a.drawMarqueeButtonNative(screen, button, layout, motion, selected)
		return
	}
	boardScale := 2.0 / 3.0
	labelScale := 2.0 / 3.0
	if selected {
		boardScale = .8
		labelScale = .8
	}
	button.zombie = a.menuZombieArt(button)
	zombie, err := a.Texture(fmt.Sprintf("Frontend0/Textures/menu_zombie_%d_SD", button.zombie))
	if err == nil {
		// Older packs (no MainScreen.uiscreen, no hand art): keep the XML-era board
		// layout. The native 1.2.5 groups are drawn by drawMarqueeButtonNative.
		fit := boardScale * 128 / 101
		options := menuGroupImageOptions(motion, 116*fit/float64(zombie.Bounds().Dx()), 135*fit/float64(zombie.Bounds().Dy()), float64(zombie.Bounds().Dx())/2, float64(zombie.Bounds().Dy())/2, 4*fit, -17*fit, 0)
		a.drawImage(screen, zombie, options)
	}
	board, err := a.Texture("Common0/Textures/Button_Screen")
	boardImage := (*ebiten.Image)(nil)
	if err == nil {
		boardImage = board.SubImage(image.Rect(0, 0, 128, 64)).(*ebiten.Image)
	}
	if selected {
		if flash, flashErr := a.Texture("Common0/Textures/Button_Screen_Flash"); flashErr == nil {
			frames := [...]image.Rectangle{image.Rect(0, 73, 140, 143), image.Rect(0, 145, 140, 215)}
			boardImage = flash.SubImage(frames[int(a.menuTime*8)%len(frames)]).(*ebiten.Image)
		}
	}
	if boardImage != nil {
		a.drawImage(screen, boardImage, menuGroupImageOptions(motion, boardScale, boardScale, float64(boardImage.Bounds().Dx())/2, float64(boardImage.Bounds().Dy())/2, 0, 0, button.angle))
	}
	labels, err := a.Texture("Common0/Textures/Button_Text_SD")
	textRect, ok := buttonTextRect(button.labelRow)
	if err != nil || !ok {
		return
	}
	resolution := a.pack.TextureSourceScale("Common0/Textures/Button_Text_SD")
	textRect = resolutionRect(textRect, resolution)
	a.drawImage(screen, labels.SubImage(textRect).(*ebiten.Image), menuGroupImageOptions(motion, labelScale/resolution, labelScale/resolution, float64(textRect.Dx())/2, float64(textRect.Dy())/2, 0, 0, button.angle))
}

func (a *app) menuSpawnScale() float64 {
	scale := a.menuSpawnTime * 5
	if scale > 1 {
		return 1
	}
	return scale
}

func marqueeImageOptions(button menuButton, scale, offsetX, offsetY float64) *ebiten.DrawImageOptions {
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-offsetX, -offsetY)
	options.GeoM.Scale(scale, scale)
	options.GeoM.Rotate(button.angle)
	options.GeoM.Translate(button.cx, button.cy)
	return options
}

func (a *app) drawMenuButton(screen *ebiten.Image, x, y float64, selected bool) {
	texture, err := a.Texture("Common0/Textures/Button_Screen")
	if err != nil {
		return
	}
	frame := 0
	if selected {
		frame = 1
	}
	source := texture.SubImage(image.Rect(0, frame*64, 128, (frame+1)*64)).(*ebiten.Image)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Scale(.25, .25)
	options.GeoM.Translate(x, y)
	a.drawImage(screen, source, options)
}
func (a *app) items() []string {
	if a.page == 0 {
		return []string{"OPTIONS", "PLAY", "QUIT", "STATS"}
	}
	var result []string
	for _, item := range a.filteredLevels() {
		result = append(result, strings.ReplaceAll(item.DisplayName, "\n", " / "))
	}
	return result
}
func (a *app) worlds() []int {
	seen := map[int]bool{}
	var result []int
	for _, item := range a.levels {
		if !seen[item.WorldIndex] {
			seen[item.WorldIndex] = true
			result = append(result, item.WorldIndex)
		}
	}
	return result
}
func (a *app) filteredLevels() []formats.LevelInfo {
	worlds := a.worlds()
	if a.world >= len(worlds) {
		return nil
	}
	var result []formats.LevelInfo
	for _, item := range a.levels {
		isSurvival := false
		for _, flag := range item.Flags {
			if flag == "SURVIVAL" {
				isSurvival = true
				break
			}
		}
		if item.WorldIndex == worlds[a.world] && ((a.mode == 1) == isSurvival) {
			result = append(result, item)
		}
	}
	return result
}
func (a *app) levelHit(x, y int) int {
	if y < 60 || y > 140 || x < 0 || x >= logicalWidth {
		return -1
	}
	levels := a.syncLevelCarousel()
	for index := range levels {
		if math.Abs(float64(x)-a.levelCardX(index)) <= 64 {
			return index
		}
	}
	return -1
}
func (a *app) levelTabAt(x, y int) int {
	position, positionOK := a.variables.Vec2Value("SHOPFRONT_TOPBUTTONS_POS_VAR")
	size, sizeOK := a.variables.Vec2Value("SHOPFRONT_TOPBUTTONS_SIZE")
	if !positionOK || !sizeOK || float64(x) < position.X-size.X/2 || float64(x) > position.X+size.X/2 || float64(y) < position.Y-size.Y/2 || float64(y) > position.Y+size.Y/2 {
		return -1
	}
	if float64(x) < position.X {
		return 0
	}
	return 1
}
func (a *app) levelActionAt(x, y int) string {
	if shopActionHit(a.variables, "SHOPFRONT_PLAY_ICON_POS_VAR", "SHOPFRONT_PLAY_ICON_WIDTH_VAR", "SHOPFRONT_PLAY_ICON_HEIGHT_VAR", x, y) {
		return "play"
	}
	if shopActionHit(a.variables, "SHOPFRONT_BACK_ICON_NO_GLOBAL_POS_VAR", "SHOPFRONT_BACK_ICON_WIDTH_VAR", "SHOPFRONT_BACK_ICON_HEIGHT_VAR", x, y) {
		return "back"
	}
	if a.currentCoopLaunch().available && a.coopButtonHit(x, y) {
		return "coop"
	}
	return ""
}
func shopActionHit(variables formats.FrontendVariables, positionName, widthName, heightName string, x, y int) bool {
	position, positionOK := variables.Vec2Value(positionName)
	width, widthOK := variables.FloatValue(widthName)
	height, heightOK := variables.FloatValue(heightName)
	return positionOK && widthOK && heightOK && math.Abs(float64(x)-position.X) <= width/2 && math.Abs(float64(y)-position.Y) <= height/2
}
func (a *app) levelUnlocked(item formats.LevelInfo) bool { return a.unlocked[item.ID] }
func hasLevelFlag(item formats.LevelInfo, wanted string) bool {
	for _, flag := range item.Flags {
		if flag == wanted {
			return true
		}
	}
	return false
}
func initialUnlocks(levels []formats.LevelInfo) map[string]bool {
	result := map[string]bool{}
	predecessors := map[string]bool{}
	for _, item := range levels {
		if hasLevelFlag(item, "STORY") && item.NextLevel != "" {
			predecessors[item.NextLevel] = true
		}
	}
	for _, item := range levels {
		if hasLevelFlag(item, "STARTUNLOCKED") && !(hasLevelFlag(item, "STORY") && predecessors[item.ID]) {
			result[item.ID] = true
		}
	}
	return result
}
func (a *app) cursor() int {
	if a.page == 0 {
		return a.mainMenuIndex()
	}
	if a.page == 2 {
		return a.level
	}
	return a.mainMenuIndex()
}
func (a *app) move(delta int) {
	if a.page == 0 {
		index := clamp(a.menuSelection+delta, 0, len(mainMenuButtons)-1)
		a.menuSelection = index
		return
	}
	if a.page == 2 {
		levels := a.syncLevelCarousel()
		a.carouselCore.Move(delta)
		if world, level, ok := levels.Selection(a.carouselCore.Selected); ok {
			a.world, a.level = world, level
		}
		return
	}
	a.level = clamp(a.level+delta, 0, len(a.filteredLevels())-1)
}
func (a *app) mainMenuIndex() int {
	return clamp(a.menuSelection, 0, len(mainMenuButtons)-1)
}
func (a *app) setCursor(index int) {
	if a.page == 2 {
		levels := a.syncLevelCarousel()
		if world, level, ok := levels.Selection(index); ok {
			a.carouselCore.SelectIndex(index)
			a.world, a.level = world, level
		}
	} else if a.page == 0 {
		a.menuSelection = index
	}
}
func (a *app) activate() error {
	switch a.page {
	case 0:
		switch mainMenuButtons[a.menuSelection].action {
		case 0:
			a.mode, a.page, a.world, a.level = 0, 2, 0, 0
		case 1:
			a.mode, a.page, a.world, a.level = 1, 2, 0, 0
		case 2:
			a.page = 3
		case 3:
			a.statsScreen, a.page = newStatsMenu(&a.statistics), 4
		case 4:
			a.askConfirm("QUIT TO DESKTOP?", func() error { return ebiten.Termination })
		}
	case 2:
		levels := a.filteredLevels()
		if a.level < 0 || a.level >= len(levels) || !a.levelUnlocked(levels[a.level]) {
			return nil
		}
		if a.debug {
			return a.openViewer()
		}
		a.coopPlayers = 0
		return a.openPlay()
	}
	return nil
}

// activateCoop starts the selected level for the players the attached
// controllers allow.
func (a *app) activateCoop() error {
	launch := a.currentCoopLaunch()
	levels := a.filteredLevels()
	if !launch.available || a.level < 0 || a.level >= len(levels) || !a.levelUnlocked(levels[a.level]) {
		return nil
	}
	a.coopPlayers, a.coopDesktopPad = launch.players, launch.desktopPad
	if err := a.openPlay(); err != nil {
		a.coopPlayers = 0
		return err
	}
	return nil
}
func (a *app) drawDetails(screen *ebiten.Image) {
	if a.page == 0 {
		return
	}
	levels := a.filteredLevels()
	if a.level >= len(levels) {
		return
	}
	item := levels[a.level]
	if item.PostcardImage != "" {
		a.drawLevelCard(screen, item)
	}
	a.text(screen, item.ID, 324, 104, .5)
	a.text(screen, fmt.Sprintf("WORLD %d", item.WorldIndex+1), 324, 124, .5)
	a.text(screen, strings.ReplaceAll(item.Description, "\n", " / "), 324, 156, .5)
}
func (a *app) drawLevelCard(screen *ebiten.Image, item formats.LevelInfo) {
	image, err := a.Texture("ShopFront0/Textures/Shop/" + item.PostcardImage + "_SD")
	if err != nil {
		return
	}
	bounds := image.Bounds()
	scale := math.Min(136/float64(bounds.Dx()), 82/float64(bounds.Dy()))
	options := &ebiten.DrawImageOptions{}
	options.GeoM.Scale(scale, scale)
	options.GeoM.Translate(316+(136-float64(bounds.Dx())*scale)/2, 180+(82-float64(bounds.Dy())*scale)/2)
	a.drawImage(screen, image, options)
}
func (a *app) drawBackdrop(screen *ebiten.Image) {
	if image, err := a.Texture("Frontend0/Textures/Portal_Menu_SD"); err == nil {
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Translate(float64(-image.Bounds().Dx())/2, float64(-image.Bounds().Dy())/2)
		resolution := a.pack.TextureSourceScale("Frontend0/Textures/Portal_Menu_SD")
		options.GeoM.Scale(1.6875/resolution, 1.6875/resolution)
		options.GeoM.Rotate(a.menuTime * .3)
		options.GeoM.Translate(336, 96)
		a.drawImage(screen, image, options)
	}
}
func (a *app) drawStartup(screen *ebiten.Image) {
	screen.Fill(colorDark)
	if !a.splashLoaded {
		a.splashLoaded = true
		if source, err := a.Source("Common0/Textures/splashscreen"); err == nil {
			a.splash = ebiten.NewImageFromImage(cropSplash(source))
		}
	}
	if a.splash != nil {
		sw, sh := float64(a.splash.Bounds().Dx()), float64(a.splash.Bounds().Dy())
		ow, oh := float64(screen.Bounds().Dx()), float64(screen.Bounds().Dy())
		scale := math.Min(ow/sw, oh/sh)
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		options.GeoM.Scale(scale, scale)
		options.GeoM.Translate((ow-sw*scale)/2, (oh-sh*scale)/2)
		screen.DrawImage(a.splash, options)
	} else {
		a.text(screen, "HALFBRICKED", 160, 148, .5)
	}
}
func (a *app) drawBarryMenu(screen *ebiten.Image) {
	image, err := a.Texture("Frontend0/Textures/Barry")
	if err != nil {
		return
	}
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(image.Bounds().Dx())/2, -float64(image.Bounds().Dy())/2)
	options.GeoM.Scale(.25, .25)
	options.GeoM.Rotate(math.Sin(a.menuTime*2.2) * .015)
	options.GeoM.Translate(416+math.Sin(a.menuTime*1.7), 256+math.Sin(a.menuTime*2.2)*1.5)
	a.drawImage(screen, image, options)
}
func (a *app) drawPlay(screen *ebiten.Image) {
	a.play.shakeOffX, a.play.shakeOffY = a.play.shakeOffset()
	// The shake moves only the world: the HUD and menus stay put.
	a.play.world.CameraX += a.play.shakeOffX
	a.play.world.CameraY += a.play.shakeOffY
	defer func() {
		a.play.world.CameraX -= a.play.shakeOffX
		a.play.world.CameraY -= a.play.shakeOffY
	}()
	if !a.play.paused {
		a.play.world.AnimationTime += 1.0 / 60.0
	}
	screenX := (a.play.x-a.play.world.CameraX)*a.play.world.Zoom + a.play.world.ViewportX
	screenY := (a.play.y-a.play.world.CameraY)*a.play.world.Zoom + a.play.world.ViewportY
	frame := int(a.play.time*8) % 4
	if a.play.moving {
		frame = int(a.play.time*10) % 4
	}
	scale := a.play.world.Zoom
	if scale <= 0 {
		scale = 1
	}
	// Zombies layer by depth: lower on screen draws in front.
	drawOrder := append([]zombieState(nil), a.play.zombies...)
	sort.SliceStable(drawOrder, func(i, j int) bool { return drawOrder[i].y < drawOrder[j].y })
	a.play.world.DrawWithEntities(screen, func(target *ebiten.Image) {
		if !a.play.playerUnspawned && a.play.health > 0 {
			a.drawBarryShadow(target, screenX, screenY-24*scale, scale)
		}
		a.drawTrain(target, a.play.train) // FUN_000fde94 submits at once: under every bucketed entity
		a.drawZombieShadows(target)
		a.drawRexShockwaves(target)
		a.drawRexVenom(target)
		for _, portal := range a.play.portals {
			a.drawPortal(target, portal)
		}
		for _, pop := range a.play.bloodPops {
			if pop.y <= a.play.y {
				a.drawBloodPop(target, pop)
			}
		}
		for _, body := range a.play.gibBodies {
			if body.y <= a.play.y {
				a.drawZombie(target, body)
			}
		}
		a.drawDepthSorted(target, drawOrder, true)
		a.drawMines(target)
		a.drawThrown(target)
		if !a.play.playerUnspawned && a.play.health > 0 {
			a.barryTint = hurtTint(a.play.hurt, a.play.time, [3]float32{})
			a.drawBarry(target, screenX, screenY, scale, frame, a.play.angle, a.play.flipX)
			a.barryTint = [3]float32{}
			a.drawShield(target)
		}
		if a.play.flash > 0 && !a.play.playerUnspawned && a.play.health > 0 {
			a.drawBarryFlash(target, screenX, screenY, scale, a.play.angle, a.play.flipX)
		}
		a.drawBullets(target)
		a.drawDepthSorted(target, drawOrder, false)
		for _, pop := range a.play.bloodPops {
			if pop.y > a.play.y {
				a.drawBloodPop(target, pop)
			}
		}
		for _, body := range a.play.gibBodies {
			if body.y > a.play.y {
				a.drawZombie(target, body)
			}
		}
		for _, explosion := range a.play.explosions {
			a.drawExplosion(target, explosion)
		}
	})
	a.drawTrainMarker(screen)
	a.drawScriptFade(screen)
	a.drawScriptTextures(screen)
	a.drawScriptText(screen)
	a.drawPlayControls(screen)
	a.drawGameHUD(screen)
	if !a.play.paused && !a.sandboxOpen && !a.play.storyGameOver(a.mode) && (!a.mobile || a.play.scriptForceReticule) && (a.play.hudVisible || a.play.scriptForceReticule) {
		a.drawReticule(screen)
	}
	a.drawDialogue(screen)
	if a.play.paused {
		a.drawPauseMenu(screen)
	}
	a.drawSandbox(screen)
	a.drawDebugPanel(screen)
}

func (a *app) drawGameHUD(screen *ebiten.Image) {
	if a.play == nil {
		return
	}
	a.drawLevelProgress(screen)
	if !a.play.hudVisible {
		return
	}
	scoreX, scoreXOK := a.variables.FloatValue("HUD_SCORE_X_VAR")
	if !scoreXOK {
		scoreX = logicalWidth / 2
	}
	scoreYName, scoreYDefault := "HUD_SCORE_Y_VAR", 26.0
	if _, bar := a.play.waveRemaining(); a.mode == 0 || bar {
		// The port's survival wave bar sits where story mode's meter does, so the
		// score takes story mode's higher position instead of the native survival one.
		scoreYName, scoreYDefault = "HUD_STORYSCORE_Y_VAR", 17
	}
	scoreY, scoreYOK := a.variables.FloatValue(scoreYName)
	if !scoreYOK {
		scoreY = scoreYDefault
	}
	scoreScale := .75
	if scoreSize, ok := a.variables.FloatValue("HUD_SCORE_SIZE_VAR"); ok && a.font != nil && a.font.LineHeight > 0 {
		scoreScale = scoreSize / float64(a.font.LineHeight)
	}
	scoreText := fmt.Sprintf("%010d", a.play.score)
	if a.font != nil {
		frontendX, frontendY := a.renderScale()
		x, y, size := scoreTextGeometry(scoreX, scoreY, a.fontTextWidth(scoreText, scoreScale), float64(a.font.LineHeight)*scoreScale, scoreScale, frontendX, frontendY)
		a.font.DrawScaled(screen, scoreText, x, y, size, size)
	}
	multiplierX, multiplierXOK := a.variables.FloatValue("HUD_SCORE_MULTI_X_VAR")
	if !multiplierXOK {
		multiplierX = 305
	}
	multiplierY, multiplierYOK := a.variables.FloatValue("HUD_SCORE_MULTI_Y_VAR")
	if !multiplierYOK {
		multiplierY = 5
	}
	a.drawComboHUD(screen, multiplierX, multiplierY)
	a.drawWaveBanner(screen)
	waveX, waveXOK := a.variables.FloatValue("HUD_WAVE_TEXT_POS_X_VAR")
	if !waveXOK {
		waveX = 440
	}
	waveText := fmt.Sprintf("wave %d", a.play.waveIndex+1)
	a.text(screen, waveText, waveX-a.fontTextWidth(waveText, .4), 5, .4)
	if icon, err := a.Texture("Common0/Textures/LifeIcon"); err == nil {
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Scale(.5, .5)
		options.GeoM.Translate(394, 20)
		a.drawImage(screen, icon, options)
	}
	// Native 0x0009532c (v7) formats "x%d" with max(lives,0) right after the life
	// icon and draws it at (410,20) with size 22 every frame, so a spent lives
	// counter reads "x0" rather than disappearing.
	livesText := fmt.Sprintf("x%d", max(a.play.lives, 0))
	a.text(screen, livesText, 410, 20, 22.0/32.0)
	if a.play.coopActive() {
		a.drawCoopHUD(screen)
	} else {
		a.drawHealthHearts(screen, a.play.health)
	}
}

func (a *app) drawHUDLights(screen *ebiten.Image) {
	texture, err := a.Texture("Common0/Textures/HUD_Lights_SD")
	if err != nil {
		return
	}
	resolution := a.pack.TextureSourceScale("Common0/Textures/HUD_Lights_SD")
	records := [...]struct{ x, y, u, v, w, h int }{{20, 276, 16, 26, 16, 12}, {20, 288, 16, 26, 16, 12}, {20, 300, 0, 26, 16, 12}, {460, 280, 16, 42, 16, 14}, {460, 296, 0, 40, 16, 16}}
	for _, record := range records {
		rect := resolutionRect(image.Rect(record.u, record.v, record.u+record.w, record.v+record.h), resolution)
		source := texture.SubImage(rect).(*ebiten.Image)
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Translate(-float64(rect.Dx())/2, -float64(rect.Dy())/2)
		options.GeoM.Scale(1/resolution, 1/resolution)
		options.GeoM.Translate(float64(record.x), float64(record.y))
		a.drawImage(screen, source, options)
	}
}

func (a *app) drawMultiplier(screen *ebiten.Image, x, y float64, multiplier int) {
	texture, err := a.Texture("Common0/Textures/MultiplyFont_SD")
	if err != nil {
		return
	}
	resolution := a.pack.TextureSourceScale("Common0/Textures/MultiplyFont_SD")
	if multiplier < 0 {
		multiplier = 0
	}
	if multiplier > 9 {
		multiplier = 9
	}
	for index, glyph := range []int{10, multiplier} {
		source := texture.SubImage(resolutionRect(image.Rect(glyph*16, 0, glyph*16+16, 32), resolution)).(*ebiten.Image)
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Scale(.5/resolution, .5/resolution)
		options.GeoM.Translate(x+float64(index*8), y)
		a.drawImage(screen, source, options)
	}
}

func (a *app) drawDialogue(screen *ebiten.Image) {
	if a.play == nil || a.play.dialogueIndex >= len(a.play.dialogue) {
		return
	}
	line := a.play.dialogue[a.play.dialogueIndex]
	panelWidth, panelHeight := logicalWidth-2, logicalHeight*19/100
	panelX, panelBottom := (logicalWidth-panelWidth)/2, logicalHeight-5
	panelTop := panelBottom - panelHeight
	target := image.Rect(panelX, panelTop, panelX+panelWidth, panelBottom)
	if texture, err := a.Texture("Common0/Textures/Backing_Square"); err == nil {
		drawNineSlice(a, screen, texture, image.Rect(0, 0, 64, 64), target)
	}
	cameoResolved := false
	if line.cameo >= 0 {
		if cameo := a.dialogueCameo(line.cameo); cameo != "" {
			if texture, err := a.Texture(cameo); err == nil {
				resolution := a.pack.TextureSourceScale(cameo)
				scale := .55 / resolution
				if resolution > 1 {
					textX, _ := dialogueTextLayout(panelX, panelWidth, true)
					scale = math.Min(scale, math.Min((textX-28)/float64(texture.Bounds().Dx()), float64(panelBottom-panelTop-15)/float64(texture.Bounds().Dy())))
				}
				options := &ebiten.DrawImageOptions{}
				options.GeoM.Scale(scale, scale)
				options.GeoM.Translate(28, float64(panelTop+15))
				a.drawImage(screen, texture, options)
				cameoResolved = true
			}
		}
	}
	textX, textWidth := dialogueTextLayout(panelX, panelWidth, cameoResolved)
	a.text(screen, a.wrapDialogue(line.text, textWidth, .45), textX, float64(panelTop+8), .45)
}

func dialogueTextLayout(panelX, panelWidth int, cameoResolved bool) (float64, float64) {
	const padding, portraitReserve = 12, 87
	textX := panelX + padding
	if cameoResolved {
		textX = panelX + portraitReserve
	}
	return float64(textX), float64(panelX + panelWidth - textX - padding)
}

func (a *app) dialogueCameo(index int) string {
	if a.play == nil {
		return ""
	}
	_, ok := a.play.scriptCameos[index]
	if !ok {
		return ""
	}
	texture := a.play.scriptTextures[index]
	if texture == nil || texture.cameo < 0 || !texture.visible || texture.name == "" {
		return ""
	}
	return commonSDTexture(texture.name)
}

func (a *app) wrapDialogue(value string, maxWidth, scale float64) string {
	words := strings.Fields(value)
	if len(words) == 0 || a.font == nil {
		return value
	}
	var lines []string
	line := ""
	for _, word := range words {
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		if line != "" && a.fontTextWidth(candidate, scale) > maxWidth {
			lines = append(lines, line)
			line = word
		} else {
			line = candidate
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (a *app) fontTextWidth(value string, scale float64) float64 {
	if a.font == nil {
		return 0
	}
	width := 0.0
	for _, runeValue := range value {
		if glyph, ok := a.font.Glyphs[runeValue]; ok {
			width += float64(glyph.XAdvance) * scale
		}
	}
	return width
}

const debugPanelWidth = 278
const debugPanelHeight = 250

func (a *app) updateDebugPanel() bool {
	px, py := a.pointer()
	panel := image.Rect(int(a.debugPanelX), int(a.debugPanelY), int(a.debugPanelX)+debugPanelWidth, int(a.debugPanelY)+debugPanelHeight)
	if a.debugPanelDragging {
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			a.debugPanelDragging = false
			return true
		}
		a.debugPanelX = clampFloat(float64(px)-a.debugPanelOffsetX, 0, logicalWidth-debugPanelWidth)
		a.debugPanelY = clampFloat(float64(py)-a.debugPanelOffsetY, 0, logicalHeight-debugPanelHeight)
		return true
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && image.Pt(px, py).In(panel) {
		if py < panel.Min.Y+22 {
			a.debugPanelDragging = true
			a.debugPanelOffsetX = float64(px) - a.debugPanelX
			a.debugPanelOffsetY = float64(py) - a.debugPanelY
		}
		return true
	}
	return false
}

func (a *app) drawDebugPanel(screen *ebiten.Image) {
	if !a.debugPanelVisible || (a.play == nil && a.view == nil) {
		return
	}
	x, y := a.debugPanelX, a.debugPanelY
	a.drawRect(screen, x, y, debugPanelWidth, debugPanelHeight, color.RGBA{8, 10, 14, 235})
	a.drawRect(screen, x, y, debugPanelWidth, 22, color.RGBA{38, 48, 60, 255})
	a.text(screen, "F2 DEBUG", x+8, y+4, .32)
	outputX, outputY := ebiten.CursorPosition()
	logicalX, logicalY := a.pointer()
	lines := []string{fmt.Sprintf("mouse out %d,%d logical %d,%d", outputX, outputY, logicalX, logicalY), fmt.Sprintf("fps %.1f tps %.1f", ebiten.ActualFPS(), ebiten.ActualTPS())}
	active := a.view
	if a.play != nil {
		active = a.play.world
		waveCount := 0
		if a.play.world != nil {
			waveCount = len(a.play.world.Level.Waves)
		}
		death := fmt.Sprintf("catalog %d", len(a.sprites))
		if definition, ok := a.sprites.Find("ZombieDeaths"); ok {
			death = fmt.Sprintf("catalog %d anims %d", len(a.sprites), len(definition.Animations))
		}
		if animation, ok := zombieDeathAnimation(a.sprites, 0); ok {
			_, err := a.Texture(animation.Texture)
			death = fmt.Sprintf("%s %d@%.1f loaded %t", animation.Texture, animation.Frames, animation.FPS, err == nil)
		}
		lines = append(lines, fmt.Sprintf("wave %d/%d elapsed %.0f zombies %d portals %d pops %d", a.play.waveIndex+1, waveCount, a.play.waveElapsed, len(a.play.zombies), len(a.play.portals), len(a.play.bloodPops)), fmt.Sprintf("death0 %s", death), fmt.Sprintf("spawned %v", a.play.waveSpawned), fmt.Sprintf("player %.1f,%.1f walk %t target %.1f,%.1f dialogue %d/%d", a.play.x, a.play.y, a.play.scriptWalking, a.play.scriptWalkX, a.play.scriptWalkY, a.play.dialogueIndex, len(a.play.dialogue)), fmt.Sprintf("health %.2f lives %d weapon %s", a.play.health, a.play.lives, a.play.weapon.GunType), fmt.Sprintf("controls move %t shoot %t entered %t", a.play.moveControl, a.play.shootControl, a.play.entryControlsRestored))
		lines = append(lines, fmt.Sprintf("score %d kills %d/%d meter %.2f", a.play.score, a.play.levelKills, a.play.levelZombieTotal, a.play.progressOpacity))
		for index, portal := range a.play.portals {
			if index >= 3 {
				break
			}
			lines = append(lines, fmt.Sprintf("portal%d %.1f,%.1f age %.2f size %.1f frame %d timer %.0f", index, portal.x, portal.y, portal.age, portal.size, portal.frame, portal.animationTimer))
		}
		if len(a.play.zombies) > 0 {
			zombie := a.play.zombies[0]
			entity := a.play.scriptEntities[zombie.scriptID]
			walking := entity != nil && entity.walking
			lines = append(lines, fmt.Sprintf("zombie0 %.1f,%.1f health %.0f walk %t away %t", zombie.x, zombie.y, zombie.health, walking, zombie.spawnAway))
		}
		if a.play.scriptRuntime != nil {
			lines = append(lines, fmt.Sprintf("script status %d line %d last %s", a.play.scriptRuntime.Status(), a.play.scriptRuntime.CurrentLine(), a.play.scriptLastCallback), fmt.Sprintf("wait %.1f active %t starts %d", a.play.scriptWaitRemaining, a.play.scriptWaitActive, a.play.scriptWaitStarts))
		}
	}
	if active != nil {
		tileSize := tileSizeFor(active.TileSet)
		atlas := "none"
		if active.Atlas != nil {
			bounds := active.Atlas.Bounds()
			atlas = fmt.Sprintf("%dx%d", bounds.Dx(), bounds.Dy())
		}
		lines = append(lines, fmt.Sprintf("level %s tileset %s atlas %s", active.Level.Info.ID, active.TileSet.Name, atlas))
		worldX := (float64(logicalX)-active.ViewportX)/active.Zoom + active.CameraX
		worldY := (float64(logicalY)-active.ViewportY)/active.Zoom + active.CameraY
		tileX, tileY := int(math.Floor(worldX/float64(tileSize))), int(math.Floor(worldY/float64(tileSize)))
		pan := "off"
		if a.play != nil && a.play.scriptCameraPanActive {
			pan = fmt.Sprintf("%.2f/%.2f", a.play.scriptCameraPanElapsed, a.play.scriptCameraPanDuration)
		}
		lines = append(lines, fmt.Sprintf("world %.1f,%.1f tile %d,%d", worldX, worldY, tileX, tileY), fmt.Sprintf("camera %.1f,%.1f zoom %.2f pan %s", active.CameraX, active.CameraY, active.Zoom, pan))
		for _, kind := range []formats.LayerKind{formats.LayerG, formats.LayerD, formats.LayerHB, formats.LayerH, formats.LayerC} {
			lines = append(lines, debugTileLine(active, kind, tileX, tileY))
		}
	}
	for index, line := range lines {
		a.text(screen, line, x+8, y+27+float64(index)*13, .3)
	}
}

func debugTileLine(active *viewer.Viewer, kind formats.LayerKind, tileX, tileY int) string {
	values := active.Level.Layers[kind]
	if tileX < 0 || tileY < 0 || tileX >= active.Level.Width || tileY >= active.Level.Height || len(values) != active.Level.Width*active.Level.Height {
		return fmt.Sprintf("%s out of bounds", strings.ToUpper(string(kind)))
	}
	raw := values[tileY*active.Level.Width+tileX]
	if kind != formats.LayerG && int32(raw) < 0 {
		return fmt.Sprintf("%s %08X empty", strings.ToUpper(string(kind)), raw)
	}
	accepted := "draw"
	if kind == formats.LayerG && int32(raw) < 0 {
		accepted = "draw-repeat"
	}
	if (kind == formats.LayerD || kind == formats.LayerHB) && raw == 0 {
		accepted = "skip-zero"
	}
	flip := ""
	if raw&0x00010000 != 0 {
		flip += "X"
	}
	if raw&0x00020000 != 0 {
		flip += "Y"
	}
	if flip == "" {
		flip = "-"
	}
	if kind == formats.LayerC {
		return fmt.Sprintf("%s %08X runtime=%d %s", strings.ToUpper(string(kind)), raw, (raw+1)&0xffff, collisionLabel((raw+1)&0xffff))
	}
	atlasInfo := ""
	tileSize := tileSizeFor(active.TileSet)
	if active.Atlas != nil && tileSize > 0 {
		bounds := active.Atlas.Bounds()
		columns, rows := bounds.Dx()/tileSize, bounds.Dy()/tileSize
		tileID := int(raw & 0xffff)
		if columns > 0 && rows > 0 {
			sourceX := tileID%columns*tileSize + tileSize/2
			sourceY := (tileID/columns)%rows*tileSize + tileSize/2
			r, g, b, a := active.Atlas.At(sourceX, sourceY).RGBA()
			atlasInfo = fmt.Sprintf(" src=%d,%d rgba=%02X%02X%02X%02X", sourceX, sourceY, r>>8, g>>8, b>>8, a>>8)
		}
	}
	return fmt.Sprintf("%s %08X id=%d f=%s %s%s", strings.ToUpper(string(kind)), raw, raw&0xffff, flip, accepted, atlasInfo)
}

func collisionLabel(value uint32) string {
	switch {
	case value == 0:
		return "walkable"
	case value == 1:
		return "solid"
	case value == 2:
		return "hazard"
	case value >= 3 && value <= 9:
		return "spawn"
	case value >= 13 && value <= 16:
		return "pickup"
	default:
		return "other"
	}
}

func (a *app) drawBullets(screen *ebiten.Image) {
	if a.play == nil || (len(a.play.bullets) == 0 && len(a.play.zombieShots) == 0) {
		return
	}
	for _, shot := range a.play.zombieShots {
		a.drawWeaponProjectile(screen, shot.projectile)
	}
	zoom := a.play.world.Zoom
	for _, b := range a.play.bullets {
		if b.projectile != nil {
			if b.projectile.EntityType != 0x12 {
				a.drawWeaponProjectile(screen, *b.projectile)
			}
			continue
		}
		textureName := "Common0/Textures/bullet_SD"
		scale := zoom
		if b.kind == "grenade" {
			textureName = "Common0/Textures/grenade_SD"
			scale *= 2
		} else if b.kind == "rocket" {
			textureName = "Common0/Textures/rocket_SD"
			if _, found := a.pack.TexturePath(textureName); !found {
				textureName = "Common0/Textures/bazooka_SD"
			}
			scale *= 1.25
		}
		bulletImg, err := a.Texture(textureName)
		if err != nil {
			continue
		}
		texW, texH := float64(bulletImg.Bounds().Dx()), float64(bulletImg.Bounds().Dy())
		sx := (b.x-a.play.world.CameraX)*zoom + a.play.world.ViewportX
		sy := (b.y-nativeProjectileRenderAnchor-a.play.world.CameraY)*zoom + a.play.world.ViewportY
		scale /= a.pack.TextureSourceScale(textureName)
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Translate(-texW/2, -texH/2)
		options.GeoM.Rotate(b.angle)
		options.GeoM.Scale(scale, scale)
		options.GeoM.Translate(sx, sy)
		a.drawImage(screen, bulletImg, options)
	}
}
func (a *app) drawExplosion(screen *ebiten.Image, explosion explosionState) {
	a.drawNativeGrenadeExplosion(screen, explosion)
}
// atlasCellRect crops a cell from a sheet that packs two characters side by
// side; atlas "left" or "right" selects the half named by the sprite catalog.
func atlasCellRect(atlas string, col, frame, numCols, numRows, texW, texH int) image.Rectangle {
	switch atlas {
	case "left":
		return barryCellRect(col, frame, numCols, numRows, texW/2, texH)
	case "right":
		return barryCellRect(col, frame, numCols, numRows, texW/2, texH).Add(image.Pt(texW/2, 0))
	}
	return barryCellRect(col, frame, numCols, numRows, texW, texH)
}
func barryCellRect(col, frame, numCols, numRows, texW, texH int) image.Rectangle {
	x0 := int(math.Round(float64(col) * float64(texW) / float64(numCols)))
	x1 := int(math.Round(float64(col+1) * float64(texW) / float64(numCols)))
	y0 := int(math.Round(float64(frame) * float64(texH) / float64(numRows)))
	y1 := int(math.Round(float64(frame+1) * float64(texH) / float64(numRows)))
	return image.Rect(x0, y0, x1, y1)
}
// playerSpriteName is the body sheet set: the presidential levels dress Barry as
// Abraham Lincoln (the "AbeBarry" sprite) when the cache has it.
func (p *playState) playerSpriteName() string {
	if p.levelInfo.WorldIndex == 6 || strings.HasPrefix(strings.ToLower(p.levelInfo.ID), "president") {
		if _, ok := p.sprites.Find("AbeBarry"); ok {
			return "AbeBarry"
		}
	}
	return "Barry"
}
func (a *app) drawBarry(screen *ebiten.Image, x, y, scale float64, frame, angle int, flipX bool) {
	renderY := y - playerRenderAnchor*scale
	bodySheet := "Common0/Textures/Characters/barryidle_SD"
	bodyAnimation := "Idle"
	if a.play != nil && a.play.moving {
		bodySheet = "Common0/Textures/Characters/barryrun_SD"
		bodyAnimation = "Run"
	}
	bodyFrame := frame
	bodyColumns, bodyRows := 9, 4
	if a.play != nil {
		if sprite, ok := a.play.sprites.Find(a.play.playerSpriteName()); ok {
			if animation, ok := sprite.Animation(bodyAnimation); ok {
				bodyFrame = spriteAnimationFrame(animation, a.play.time, frame)
				bodySheet, bodyColumns, bodyRows = animation.Texture, animation.Angles, animation.Frames
			}
		}
	}
	a.drawGunFlare(screen, x, y, scale, angle, flipX, gunStageBehind)
	a.drawBarryPart(screen, bodySheet, x, renderY, scale, bodyFrame, angle, bodyColumns, bodyRows, flipX)
	a.drawGunFlare(screen, x, y, scale, angle, flipX, gunStageAfterBody)
	if a.play != nil && angle != 8 && a.play.weapon.TextureGun != "" {
		weaponFrame := bodyFrame
		weaponSheet, weaponColumns, weaponRows := commonSDTexture(a.play.weapon.TextureGun), 9, 4
		// The low-ammo blink flag (+0x20, see GunVisual) swaps the gun sprite's
		// animation (0x000a8bbc) and draws it untinted.
		animationName, blink := "Idle", a.play.gunBlinks()
		if blink {
			animationName = "Flash"
		}
		if sprite, ok := a.play.sprites.Find(a.play.weapon.TextureGun); ok {
			if animation, ok := sprite.Animation(animationName); ok {
				weaponFrame = spriteAnimationFrame(animation, a.play.time, bodyFrame)
				weaponSheet, weaponColumns, weaponRows = animation.Texture, animation.Angles, animation.Frames
			}
		}
		tint := a.barryTint
		if blink {
			a.barryTint = [3]float32{}
		}
		a.drawBarryPart(screen, weaponSheet, x, renderY, scale, weaponFrame, angle, weaponColumns, weaponRows, flipX)
		a.barryTint = tint
	}
	a.drawGunFlare(screen, x, y, scale, angle, flipX, gunStageAfterGun)
}

func spriteAnimationFrame(animation formats.SpriteAnimation, elapsed float64, fallback int) int {
	if animation.Frames <= 0 || animation.FPS <= 0 {
		return fallback
	}
	frame := int(elapsed * animation.FPS)
	if frame < 0 {
		frame = 0
	}
	if animation.Loop {
		return frame % animation.Frames
	}
	if frame >= animation.Frames {
		return animation.Frames - 1
	}
	return frame
}

func (a *app) drawZombie(screen *ebiten.Image, zombie zombieState) {
	a.drawZombieTrail(screen, zombie)
	textureName := zombie.texture
	if textureName == "" {
		textureName = "cavezombie"
	}
	animation, hasAnimation := a.spriteAnimation(textureName, zombie.animation)
	if zombie.bossRage || zombie.rexRageTimer > 0 {
		animation, hasAnimation = a.spriteAnimation(textureName, "Rage")
	}
	gibClip, gibFrame, isGibClip := a.zombieGibClip(zombie)
	if isGibClip {
		animation, hasAnimation = gibClip, true
	}
	texturePath := commonSDTexture(textureName)
	columns, rows := 5, 4
	if hasAnimation {
		texturePath = animation.Texture
		if animation.Angles > 0 {
			columns = animation.Angles
		}
		if animation.Frames > 0 {
			rows = animation.Frames
		}
	}
	texture, err := a.Texture(texturePath)
	if err != nil {
		return
	}
	angle := int(math.Round(float64(zombie.angle) * float64(columns-1) / 8))
	flipX := zombie.flipX
	if entity := a.play.scriptEntities[zombie.scriptID]; entity != nil && entity.rotationSet {
		angle, flipX = nativeSpriteDirection(entity.rotation, columns)
	}
	frame := int(math.Floor(zombie.frame)) % rows
	if isGibClip {
		frame = gibFrame
	}
	if frame < 0 {
		frame += rows
	}
	atlas := ""
	if hasAnimation {
		atlas = animation.Atlas
	}
	rect := atlasCellRect(atlas, angle, frame, columns, rows, texture.Bounds().Dx(), texture.Bounds().Dy())
	if isGibClip && gibClip.Angles <= 1 {
		// Disintegrate is a one-angle strip with its frames across (zombie_gib.go).
		rect = zombieGibStripCell(frame, gibClip.Frames, texture.Bounds())
	}
	if rect.Dx() <= 0 || rect.Dy() <= 0 {
		return
	}
	scale := a.play.world.Zoom
	if scale <= 0 {
		scale = 1
	}
	screenX := (zombie.x-a.play.world.CameraX)*scale + a.play.world.ViewportX
	renderHeight := zombie.size.Y
	if renderHeight <= 0 {
		renderHeight = 48
	}
	frontendX, frontendY := a.renderScale()
	screenY := (zombie.y-a.play.world.CameraY)*scale + a.play.world.ViewportY - renderHeight*zombieRenderAnchor*scale*frontendX/frontendY - a.play.rexLift(zombie)*scale*frontendX/frontendY
	source := texture.SubImage(rect).(*ebiten.Image)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(rect.Dx())/2, -float64(rect.Dy())/2)
	scaleX, scaleY := zombieSpriteScale(zombie.size.X, zombie.size.Y, rect, scale, frontendX, frontendY)
	scaleX, scaleY = scriptEntityRenderScale(scaleX, scaleY, flipX, zombie.flipY)
	options.GeoM.Scale(scaleX, scaleY)
	if tint := zombieTint(zombie); tint != 1 {
		options.ColorScale.Scale(tint, tint, tint, 1)
	}
	if zombie.alpha < 1 {
		options.ColorScale.ScaleAlpha(float32(math.Max(0, zombie.alpha)))
	}
	options.GeoM.Translate(screenX, screenY)
	a.drawImage(screen, source, options)
	a.drawExplodingGlow(screen, zombie)
}

func (a *app) drawBloodPop(screen *ebiten.Image, pop bloodPop) {
	if a.play == nil {
		return
	}
	zoom := a.play.world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	x := (pop.x-a.play.world.CameraX)*zoom + a.play.world.ViewportX
	y := (pop.y-a.play.world.CameraY)*zoom + a.play.world.ViewportY
	a.drawBloodPopSprite(screen, x, y, zoom, pop.variant, pop.age)
}

func zombieDeathAnimation(catalog formats.SpriteCatalog, variant int) (formats.SpriteAnimation, bool) {
	definition, ok := catalog.Find("ZombieDeaths")
	if !ok {
		return formats.SpriteAnimation{}, false
	}
	return definition.Animation(fmt.Sprintf("Pop_%d", variant))
}

func (a *app) drawPortal(screen *ebiten.Image, portal portalState) {
	texture, err := a.Texture("Common0/Textures/portal_SD")
	if err != nil {
		return
	}
	frame := portal.frame % 4
	if frame < 0 {
		frame += 4
	}
	resolution := a.pack.TextureSourceScale("Common0/Textures/portal_SD")
	source := texture.SubImage(resolutionRect(image.Rect(frame*128, 0, frame*128+128, 128), resolution)).(*ebiten.Image)
	screenX := (portal.x-a.play.world.CameraX)*a.play.world.Zoom + a.play.world.ViewportX
	screenY := (portal.y-a.play.world.CameraY)*a.play.world.Zoom + a.play.world.ViewportY
	renderSize := portal.size * .75
	if renderSize <= 0 {
		return
	}
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-64*resolution, -64*resolution)
	options.GeoM.Scale(renderSize/(128*resolution)*a.play.world.Zoom, renderSize/(128*resolution)*a.play.world.Zoom)
	options.GeoM.Rotate((portal.rotationUnits / 182) * math.Pi / 180)
	options.GeoM.Translate(screenX, screenY)
	a.drawImage(screen, source, options)
}

func commonSDTexture(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	path = strings.TrimSuffix(path, filepath.Ext(path))
	if !strings.HasSuffix(strings.ToLower(path), "_sd") {
		path += "_SD"
	}
	if !strings.HasPrefix(strings.ToLower(path), "common0/") {
		if !strings.Contains(path, "/") {
			path = "Characters/" + path
		}
		path = "Common0/" + path
	}
	return path
}
func (a *app) drawBarryPart(screen *ebiten.Image, name string, x, y, scale float64, frame, angle, columns, rows int, flipX bool) {
	texture, err := a.Texture(name)
	if err != nil {
		return
	}
	if columns <= 0 || rows <= 0 {
		return
	}
	scale /= a.pack.TextureSourceScale(name)
	if angle < 0 {
		angle = 0
	} else if angle >= columns {
		angle = columns - 1
	}
	frame = ((frame % rows) + rows) % rows
	texW, texH := texture.Bounds().Dx(), texture.Bounds().Dy()
	rect := barryCellRect(angle, frame, columns, rows, texW, texH)
	cellWidth, cellHeight := float64(rect.Dx()), float64(rect.Dy())
	if cellWidth <= 0 || cellHeight <= 0 {
		return
	}
	source := texture.SubImage(rect).(*ebiten.Image)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	if a.barryTint != ([3]float32{}) {
		options.ColorScale.Scale(a.barryTint[0], a.barryTint[1], a.barryTint[2], 1)
	}
	options.GeoM.Translate(-cellWidth/2, -cellHeight/2)
	if flipX {
		options.GeoM.Scale(-scale, scale)
	} else {
		options.GeoM.Scale(scale, scale)
	}
	options.GeoM.Translate(x, y)
	a.drawImage(screen, source, options)
}
func (a *app) drawBarryFlash(screen *ebiten.Image, x, y, scale float64, angle int, flipX bool) {
	if angle == 8 {
		return
	}
	if a.play == nil || a.play.weapon.TextureFlash == "" {
		return
	}
	if gun := a.play.weapon.GunType; gun == "BUZZSAW" || gun == "DUALPISTOL" {
		// Neither class uses the sprite's Flash animation as a muzzle flash: it is the
		// low-ammo blink frame (drawBarry); their flashes are flare quads (gun_visual.go).
		return
	}
	textureName := commonSDTexture(a.play.weapon.TextureFlash)
	columns, rows, frame := 9, 1, 0
	if sprite, ok := a.play.sprites.Find(a.play.weapon.TextureGun); ok {
		if animation, ok := sprite.Animation("Flash"); ok {
			textureName, columns, rows = animation.Texture, animation.Angles, animation.Frames
			frame = spriteAnimationFrame(animation, a.play.time, 0)
		}
	}
	texture, err := a.Texture(textureName)
	if err != nil {
		return
	}
	if columns <= 0 || rows <= 0 {
		return
	}
	renderY := y - playerRenderAnchor*scale
	scale /= a.pack.TextureSourceScale(textureName)
	if angle < 0 {
		angle = 0
	} else if angle >= columns {
		angle = columns - 1
	}
	texW, texH := texture.Bounds().Dx(), texture.Bounds().Dy()
	rect := barryCellRect(angle, frame, columns, rows, texW, texH)
	cellWidth, cellHeight := float64(rect.Dx()), float64(rect.Dy())
	if cellWidth <= 0 || cellHeight <= 0 {
		return
	}
	source := texture.SubImage(rect).(*ebiten.Image)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-cellWidth/2, -cellHeight/2)
	if flipX {
		options.GeoM.Scale(-scale, scale)
	} else {
		options.GeoM.Scale(scale, scale)
	}
	options.GeoM.Translate(x, renderY)
	a.drawImage(screen, source, options)
}
func (a *app) drawBarryShadow(screen *ebiten.Image, x, y, scale float64) {
	texture, err := a.Texture("Common0/Textures/shadow_SD")
	if err != nil {
		return
	}
	w, h := float64(texture.Bounds().Dx()), float64(texture.Bounds().Dy())
	// Draw an elliptical shadow at Barry's feet: scale narrower vertically, slightly below center.
	const shadowScale = 0.5
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-w/2, -h/2)
	options.GeoM.Scale(shadowScale*scale, shadowScale*scale)
	options.GeoM.Translate(x, y+26*scale)
	options.ColorScale.ScaleAlpha(0.55)
	a.drawImage(screen, texture, options)
}

func (a *app) drawReticule(screen *ebiten.Image) {
	texture, err := a.Texture("Common0/Textures/Reticule_SD")
	if err != nil {
		return
	}
	px, py := a.pointer()
	w, h := float64(texture.Bounds().Dx()), float64(texture.Bounds().Dy())
	scale := 2 / a.pack.TextureSourceScale("Common0/Textures/Reticule_SD")
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Scale(scale, scale)
	options.GeoM.Translate(float64(px)-w*scale/2, float64(py)-h*scale/2)
	a.drawImage(screen, texture, options)
}
func (a *app) drawPlayControls(screen *ebiten.Image) {
	if a.play == nil || (!a.play.hudVisible && !a.play.scriptAllowThumbsticks && !a.play.scriptForceReticule && !a.play.scriptForceThumbStick[0] && !a.play.scriptForceThumbStick[1]) {
		return
	}
	a.drawGrenadeButton(screen)
	if !a.play.coopActive() {
		a.drawPrimaryWeaponButton(screen)
	}
	showScriptSticks := a.play.scriptRuntime == nil || a.play.scriptRuntime.Done() || a.play.scriptAllowThumbsticks
	showLeftStick := a.play.controls.Visible && (showScriptSticks || a.play.scriptForceThumbStick[0])
	showRightStick := a.play.controls.Visible && (showScriptSticks || a.play.scriptForceThumbStick[1])
	if showLeftStick {
		if a.play.stick == 1 {
			a.drawStick(screen, "Common0/Textures/Analog_Nub_Move_SD", a.play.leftBaseX, a.play.leftBaseY, a.play.leftDeflectX, a.play.leftDeflectY)
		} else {
			baseX, baseY := a.play.scriptThumbStickX[0], a.play.scriptThumbStickY[0]
			if baseX == 0 {
				baseX = a.play.leftBaseX
			}
			if baseY == 0 {
				baseY = a.play.leftBaseY
			}
			a.drawStick(screen, "Common0/Textures/Analog_Nub_Move_SD", baseX, baseY, 0, 0)
		}
	}
	if showRightStick {
		if a.play.stick == 2 {
			a.drawStick(screen, "Common0/Textures/Analog_Nub_Gun_SD", a.play.rightBaseX, a.play.rightBaseY, a.play.rightDeflectX, a.play.rightDeflectY)
		} else {
			baseX, baseY := a.play.rightStickAnchor()
			a.drawStick(screen, "Common0/Textures/Analog_Nub_Gun_SD", baseX, baseY, 0, 0)
		}
	}
	if image, err := a.Texture("Common0/Textures/Pause_Large_SD"); err == nil {
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		resolution := a.pack.TextureSourceScale("Common0/Textures/Pause_Large_SD")
		options.GeoM.Scale(.5/resolution, .5/resolution)
		options.GeoM.Translate(448, 8)
		a.drawImage(screen, image, options)
	}
}

func (a *app) drawGrenadeButton(screen *ebiten.Image) {
	if a.play == nil || a.play.grenades <= 0 {
		return
	}
	x, y, width, height := a.play.secondaryButtonGeometry()
	a.drawControlButtonPlate(screen, x, y, width, height)
	a.drawButtonReadout(screen, x, y, "Common0/Textures/Weapons_Secondary_SD", secondaryIconCell(a.play.secondaryType), fmt.Sprintf("x%d", a.play.grenades))
}
func secondaryIconPosition(x, y float64, offset formats.Vec2, mobile bool) (float64, float64) {
	if mobile {
		return x + offset.X, y + offset.Y
	}
	return x, y
}
func (p *playState) secondaryButtonContains(x, y float64) bool {
	if p.grenades <= 0 {
		return false
	}
	buttonX, buttonY, width, height := p.secondaryButtonGeometry()
	return x >= buttonX-width/2 && x <= buttonX+width/2 && y >= buttonY-height/2 && y <= buttonY+height/2
}
func (a *app) drawStick(screen *ebiten.Image, name string, baseX, baseY, deflectX, deflectY float64) {
	padRadius := float64(a.play.controls.PadRadius)
	sx, sy := a.renderScale()
	nubRadius := 32.0
	if !a.mobile {
		padRadius /= sx
		nubRadius /= sx
	}
	if image, err := a.Texture("Common0/Textures/Analog_Back_SD"); err == nil {
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Translate(-float64(image.Bounds().Dx())/2, -float64(image.Bounds().Dy())/2)
		options.GeoM.Scale(padRadius*2/float64(image.Bounds().Dx()), padRadius*2/float64(image.Bounds().Dy())*sx/sy)
		options.GeoM.Translate(baseX, baseY)
		a.drawImage(screen, image, options)
	}
	if image, err := a.Texture(name); err == nil {
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Translate(-float64(image.Bounds().Dx())/2, -float64(image.Bounds().Dy())/2)
		options.GeoM.Scale(nubRadius*2/float64(image.Bounds().Dx()), nubRadius*2/float64(image.Bounds().Dy())*sx/sy)
		dx, dy := deflectX*padRadius*1.3, deflectY*padRadius*1.3
		limit := math.Max(0, padRadius-nubRadius)
		if distance := math.Hypot(dx, dy); distance > limit && distance > 0 {
			dx, dy = dx/distance*limit, dy/distance*limit
		}
		options.GeoM.Translate(baseX+dx, baseY+dy*sx/sy)
		a.drawImage(screen, image, options)
	}
}
func cropSplash(source image.Image) image.Image {
	bounds := source.Bounds()
	top, bottom := bounds.Max.Y, bounds.Min.Y
	br, bg, bb, _ := source.At(bounds.Min.X, bounds.Min.Y).RGBA()
	bar := [3]uint32{br, bg, bb}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		if splashRowHasContent(source, y, bounds, bar) {
			top = y
			break
		}
	}
	for y := bounds.Max.Y - 1; y >= bounds.Min.Y; y-- {
		if splashRowHasContent(source, y, bounds, bar) {
			bottom = y + 1
			break
		}
	}
	if top >= bottom {
		return source
	}
	cropped := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bottom-top))
	draw.Draw(cropped, cropped.Bounds(), source, image.Point{X: bounds.Min.X, Y: top}, draw.Src)
	return cropped
}
func splashRowHasContent(source image.Image, y int, bounds image.Rectangle, bar [3]uint32) bool {
	diff := func(a, b uint32) uint32 {
		if a > b {
			return a - b
		}
		return b - a
	}
	for x := bounds.Min.X; x < bounds.Max.X; x += 8 {
		r, g, b, a := source.At(x, y).RGBA()
		if a > 0 && diff(r, bar[0])+diff(g, bar[1])+diff(b, bar[2]) > 24*257 {
			return true
		}
	}
	return false
}
func (a *app) text(screen *ebiten.Image, value string, x, y, scale float64) {
	if a.font != nil {
		a.drawFont(screen, a.font, value, x, y, scale)
		return
	}
	if a.frontendScaleX != 1 || a.frontendScaleY != 1 {
		x *= a.frontendScaleX
		y *= a.frontendScaleY
	}
	ebitenutil.DebugPrintAt(screen, value, int(x), int(y))
}
func (a *app) textCentered(screen *ebiten.Image, value string, y, scale float64) {
	width := 0.0
	if a.font != nil {
		for _, runeValue := range value {
			if glyph, ok := a.font.Glyphs[runeValue]; ok {
				width += float64(glyph.XAdvance) * scale
			}
		}
	}
	a.text(screen, value, (logicalWidth-width)/2, y, scale)
}
func (a *app) drawFont(screen *ebiten.Image, font *ui.Font, value string, x, y, scale float64) {
	if font == nil {
		return
	}
	if a.frontendScaleX == 1 && a.frontendScaleY == 1 {
		font.Draw(screen, value, x, y, scale)
		return
	}
	font.DrawScaled(screen, value, x*a.frontendScaleX, y*a.frontendScaleY, scale*a.frontendScaleX, scale*a.frontendScaleY)
}
func loadFont(pack *content.Pack) (*ui.Font, error) {
	return loadNamedFont(pack, "Common0/Fonts/font.fnt", "Common0/Fonts/font_0")
}
func loadNamedFont(pack *content.Pack, metadataName, textureName string) (*ui.Font, error) {
	metadataPath, ok := pack.SourcePath(metadataName)
	if !ok {
		return nil, fmt.Errorf("font metadata not found")
	}
	atlasPath, ok := pack.TexturePath(textureName)
	if !ok {
		return nil, fmt.Errorf("font atlas not found")
	}
	metadata, err := pack.Open(metadataPath)
	if err != nil {
		return nil, err
	}
	defer metadata.Close()
	atlasReader, err := pack.Open(atlasPath)
	if err != nil {
		return nil, err
	}
	defer atlasReader.Close()
	data, err := ioReadAll(atlasReader)
	if err != nil {
		return nil, err
	}
	source, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return ui.LoadFont(metadata, ebiten.NewImageFromImage(source))
}
func (a *app) drawTexture(screen *ebiten.Image, name string, x, y, scale float64) {
	image, err := a.Texture(name)
	if err != nil {
		return
	}
	options := &ebiten.DrawImageOptions{}
	options.GeoM.Scale(scale, scale)
	options.GeoM.Translate(x, y)
	a.drawImage(screen, image, options)
}
func (a *app) openViewer() error {
	level, tileset, atlas, err := a.selectedLevel()
	if err != nil {
		return err
	}
	a.view = viewer.New(level, tileset, atlas, a)
	a.view.Debug = true
	return nil
}
func (a *app) openPlay() error {
	level, tileset, atlas, err := a.selectedLevel()
	if err != nil {
		return err
	}
	world := viewer.New(level, tileset, atlas, a)
	world.Zoom = 1.0
	world.ViewportX = 0
	world.ViewportY = 0
	world.Layers[formats.LayerH] = true
	tileSize := tileSizeFor(tileset)
	spawnX, spawnY := spawnPosition(level, tileSize)
	play := &playState{world: world, x: spawnX, y: spawnY, spawnX: spawnX, spawnY: spawnY, tileSize: tileSize, radius: playerCollisionRadius, weapon: a.weapon, weapons: a.weapons, zombieWeapons: a.zombieWeapons, sprites: a.sprites, health: 1, maxHealth: 1, lives: 3, multiplier: 1, hudVisible: true, moveControl: a.mode != 0, shootControl: a.mode != 0, scriptNextEntity: 1, scriptEntities: map[int]*scriptEntity{}, scriptTextures: map[int]*scriptTexture{}, scriptAlpha: 1, scriptPlayerPosSet: false, rng: a.nativeRNGForPlay()}
	play.levelZombieTotal = levelZombieCount(level.Waves, a.mode == 1)
	play.levelInfo = level.Info
	play.trainSpec = a.loadTrainSpec()
	play.configureControls(a.options.controls, a.outputWidth, a.outputHeight)
	play.controlWidth, play.controlHeight, play.statistics = a.outputWidth, a.outputHeight, &a.statistics
	play.secondaryControlSize, _ = a.variables.Vec2Value("HUD_SECOND_BUTTON_SIZE_VAR")
	play.secondaryDensity = a.pack.TextureSourceScale("Common0/Textures/SecondaryButton_SD")
	play.mobileControls = a.mobile
	play.statsPositionX, play.statsPositionY = spawnX, spawnY
	if a.mode == 0 && !a.netGuest() {
		source, err := a.pack.ScriptSource(entryScriptPath(level.Info))
		switch {
		case err != nil && strings.Contains(err.Error(), "not found"):
			// Some levels (for example president_story_2) have no entry script and just begin.
			log.Printf("no entry script for %s; starting directly", level.Info.ID)
		case err != nil:
			return err
		default:
			host := &playScriptHost{app: a, play: play}
			play.playerUnspawned = strings.Contains(source, "DoPlayerSpawn()") && !strings.Contains(source, "SetLevelToLoad")
			play.scriptRuntime, err = scripting.New(source, host, scriptCallbacks)
			if err != nil {
				return fmt.Errorf("%s: %w", entryScriptPath(level.Info), err)
			}
		}
	}
	if a.mode == 1 && !a.netGuest() {
		// Survival opens with scripts/survival_tute.script (GET READY... / HERE THEY COME!): the native survival game
		// state loads it from virtual slot 16 (1.2.5 FUN_000995a8), the sibling of the story state's entry-script
		// loader FUN_00098a1c, and the waves only start once it has finished.
		source, err := a.pack.ScriptSource(survivalIntroScript)
		switch {
		case err != nil && strings.Contains(err.Error(), "not found"):
			log.Printf("no %s in this data set; survival starts directly", survivalIntroScript)
		case err != nil:
			return err
		default:
			play.scriptRuntime, err = scripting.New(source, &playScriptHost{app: a, play: play}, scriptCallbacks)
			if err != nil {
				return fmt.Errorf("%s: %w", survivalIntroScript, err)
			}
		}
	}
	if a.coopPlayers > 1 {
		play.startCoop(a.coopPlayers, false)
	}
	a.stopWeaponPlayback()
	a.noteAchievementLevelEntry(a.play, level.Info)
	a.play = play
	a.setLevelMusic(level.Info)
	a.play.centerCamera()
	if a.newDismissed == nil {
		a.newDismissed = map[string]bool{}
	}
	a.newDismissed[level.Info.ID] = true
	a.netBroadcastStart()
	return a.savePlayerProfile()
}

// survivalIntroScript is the script every survival level starts with (see openPlay).
const survivalIntroScript = "Common0/Scripts/Survival_Tute.script"

func entryScriptPath(info formats.LevelInfo) string {
	parts := strings.Split(filepath.ToSlash(info.SourceXML), "/")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	return parts[0] + "/Scripts/" + info.BaseFile + "_entry.script"
}
func conversationLines(conversation formats.Conversation) []dialogueLine {
	var result []dialogueLine
	for _, speech := range conversation.Speeches {
		for _, text := range speech.Text {
			if strings.TrimSpace(text) != "" {
				result = append(result, dialogueLine{text: strings.ReplaceAll(text, "#", ""), cameo: speech.Cameo})
			}
		}
	}
	return result
}
func (a *app) selectedLevel() (formats.Level, formats.TileSet, *ebiten.Image, error) {
	levels := a.filteredLevels()
	if a.level >= len(levels) {
		return formats.Level{}, formats.TileSet{}, nil, fmt.Errorf("selected level is unavailable")
	}
	level, err := a.pack.Load(levels[a.level].ID)
	if err != nil {
		return formats.Level{}, formats.TileSet{}, nil, err
	}
	tileset, ok := a.pack.Manifest().TileSets[strings.ToLower(level.Tileset)]
	if !ok {
		return formats.Level{}, formats.TileSet{}, nil, fmt.Errorf("tileset %q not found", level.Tileset)
	}
	atlas, err := a.Texture(tileset.Texture)
	if err != nil {
		return formats.Level{}, formats.TileSet{}, nil, err
	}
	return level, tileset, atlas, nil
}
func tileSizeFor(tileset formats.TileSet) int {
	return 32
}
func spawnPosition(level formats.Level, tileSize int) (float64, float64) {
	if level.Layers[formats.LayerC] != nil {
		for y := 0; y < level.Height; y++ {
			for x := 0; x < level.Width; x++ {
				if level.Layers[formats.LayerC][y*level.Width+x] == 2 {
					return float64(x*tileSize + tileSize/2), float64(y*tileSize + tileSize/2)
				}
			}
		}
	}
	return float64(level.Width*tileSize) / 2, float64(level.Height*tileSize) / 2
}
func (p *playState) Update(pointerX, pointerY int, pointerDown, pointerJustPressed, mobile bool) bool {
	if mobile {
		p.aimActive = pointerDown && p.stick == 2
	} else {
		p.aimActive = pointerDown || ebiten.IsKeyPressed(ebiten.KeySpace) || p.input.aiming()
	}
	tileSize := p.tileSize
	if tileSize <= 0 {
		tileSize = 32
	}
	radius := p.radius
	if radius <= 0 {
		radius = playerCollisionRadius
	}
	inPauseBtn := pointerX >= 440 && pointerX <= 480 && pointerY >= 0 && pointerY <= 48
	if pointerJustPressed && inPauseBtn {
		p.paused = !p.paused
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		p.paused = !p.paused
		return false
	}
	if p.paused {
		if pointerJustPressed {
			if pointerX >= 180 && pointerX <= 300 && pointerY >= 150 && pointerY <= 175 {
				p.paused = false
				return false
			}
			if pointerX >= 170 && pointerX <= 310 && pointerY >= 180 && pointerY <= 205 {
				p.shouldQuit = true
				return false
			}
		}
		return false
	}
	p.updateProgressOpacity(1.0 / 60.0)
	p.flash = math.Max(0, p.flash-1.0/60.0)
	p.shootCooldown = math.Max(0, p.shootCooldown-1.0/60.0)
	p.secondaryShootCooldown = math.Max(0, p.secondaryShootCooldown-1.0/60.0)
	fired := false
	scriptFacingLocked := p.scriptWalking || (p.dialogueIndex >= 0 && p.dialogueIndex < len(p.dialogue))
	p.updateZombies()
	if p.updatePlayerDeath() {
		// Keep effects (including the player's own blood pop) animating while dead.
		p.updatePortals()
		p.updateBloodPops()
		p.updateExplosions()
		if !p.coopActive() {
			p.updateCombo(1.0 / 60.0)
		}
		if p.coopActive() {
			// Teammates keep playing: projectiles, pickups, waves and the camera continue.
			p.updateMines()
			p.updateSentries()
			p.updatePickups()
			p.updateBulletsAndKills()
			if p.scriptRuntime == nil || p.scriptRuntime.Done() {
				p.updateWaves()
			}
			p.time += 1.0 / 60.0
			p.updateCamera()
		}
		return false
	}
	p.updatePortals()
	p.updateBloodPops()
	p.updateExplosions()
	p.updateMines()
	p.updateSentries()
	p.updatePickups()

	p.updateBulletsAndKills()

	if !mobile {
		p.stick = 0
		p.leftDeflectX, p.leftDeflectY, p.rightDeflectX, p.rightDeflectY = 0, 0, 0, 0
	} else if !pointerDown {
		p.stick = 0
		p.leftDeflectX, p.leftDeflectY, p.rightDeflectX, p.rightDeflectY = 0, 0, 0, 0
	} else if p.stick == 0 && pointerJustPressed && !inPauseBtn && !p.secondaryPointerDown {
		p.startControlTouch(float64(pointerX), float64(pointerY))
	}
	if mobile && pointerDown && p.stick == 1 {
		p.updateControlTouch(float64(pointerX), float64(pointerY), p.controlWidth, p.controlHeight)
	}
	if mobile && pointerDown && p.stick == 2 {
		p.updateControlTouch(float64(pointerX), float64(pointerY), p.controlWidth, p.controlHeight)
	}
	dx, dy := 0.0, 0.0
	if ebiten.IsKeyPressed(ebiten.KeyLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
		dx--
	}
	if ebiten.IsKeyPressed(ebiten.KeyRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
		dx++
	}
	if ebiten.IsKeyPressed(ebiten.KeyUp) || ebiten.IsKeyPressed(ebiten.KeyW) {
		dy--
	}
	if ebiten.IsKeyPressed(ebiten.KeyDown) || ebiten.IsKeyPressed(ebiten.KeyS) {
		dy++
	}
	if mobile && p.stick == 1 {
		dx, dy = p.leftDeflectX, p.leftDeflectY
	}
	dx, dy = dx+p.input.moveX, dy+p.input.moveY
	if p.weapon.GunType == "MINIGUN" {
		trigger := p.shootControl && (p.input.aiming() || (mobile && p.stick == 2 && math.Hypot(p.rightDeflectX, p.rightDeflectY) > .5) || (!mobile && (pointerDown || ebiten.IsKeyPressed(ebiten.KeySpace)) && !inPauseBtn && !p.secondaryPointerDown))
		p.spinEvents.Add(p.spinAudio.SpinTick(weaponBindings(p.weapon), trigger, 1.0/60.0, p.spinEndFinished))
	}
	if mobile && p.stick == 2 && p.shootControl && math.Hypot(p.rightDeflectX, p.rightDeflectY) > .5 {
		if !scriptFacingLocked {
			p.angle, p.flipX = barryDirection(p.rightDeflectX, p.rightDeflectY)
		}
		if p.fireGateOpen() {
			fired = p.fire(p.rightDeflectX, p.rightDeflectY)
		}
	}
	if !mobile && p.shootControl && !p.secondaryButtonContains(float64(pointerX), float64(pointerY)) {
		worldX := (float64(pointerX)-p.world.ViewportX)/p.world.Zoom + p.world.CameraX
		worldY := (float64(pointerY)-p.world.ViewportY)/p.world.Zoom + p.world.CameraY
		aimDX := worldX - p.x
		aimDY := worldY - p.y
		if math.Hypot(aimDX, aimDY) > .001 && !scriptFacingLocked {
			p.angle, p.flipX = barryDirection(aimDX, aimDY)
		}
		firing := (pointerDown || ebiten.IsKeyPressed(ebiten.KeySpace)) && !inPauseBtn && !p.secondaryPointerDown
		if p.shootControl && firing && p.fireGateOpen() {
			fired = p.fire(aimDX, aimDY)
		}
	}
	if p.input.aiming() && p.shootControl {
		if !scriptFacingLocked {
			p.angle, p.flipX = barryDirection(p.input.aimX, p.input.aimY)
		}
		if p.fireGateOpen() {
			if p.fire(p.input.aimX, p.input.aimY) {
				fired = true
			}
		}
	} else if (p.input.moveX != 0 || p.input.moveY != 0) && !scriptFacingLocked && !mobile {
		p.angle, p.flipX = barryDirection(p.input.moveX, p.input.moveY)
	}
	if p.shootControl && p.grenades > 0 && (p.secondaryButtonJustPressed || p.input.secondaryHit) && p.secondaryShootCooldown <= 0 {
		dx, dy := barryAimDirection(p.angle, p.flipX)
		fired = p.fireSecondary(dx, dy)
	}
	p.tickGun(true)
	if !p.moveControl {
		dx, dy = 0, 0
	}
	p.stepBody(dx, dy, mobile, scriptFacingLocked, true)
	if p.coopActive() {
		p.clampToView(&p.x, &p.y, 12)
	}
	p.time += 1.0 / 60.0
	p.updateCamera()
	if p.scriptRuntime == nil || p.scriptRuntime.Done() {
		p.updateWaves()
	}
	return fired
}

// stepBody moves the active body by the input direction with tile and zombie
// collision. primary is false for co-op players (no achievement bookkeeping, no
// script walking).
func (p *playState) stepBody(dx, dy float64, mobile, scriptFacingLocked, primary bool) {
	p.hurt = math.Max(0, p.hurt-1.0/60.0)
	p.stepVitals(1.0 / 60.0)
	if p.cheats.infiniteAmmo {
		p.restockAmmo()
	}
	tileSize := p.tileSize
	if tileSize <= 0 {
		tileSize = 32
	}
	radius := p.radius
	if radius <= 0 {
		radius = playerCollisionRadius
	}
	if primary {
		p.achievementMotionX, p.achievementMotionY = achievements.NativeAchievementMotion(float32(dx), float32(dy))
	}
	p.moving = dx != 0 || dy != 0
	if p.moving {
		length := math.Sqrt(dx*dx + dy*dy)
		walk := playerWalkBase(p.waveBuild()) * p.vitals.walkSpeedFactor()
		moveX, moveY := dx/length*walk/60, dy/length*walk/60
		stepLength := math.Sqrt(moveX*moveX + moveY*moveY)
		steps := int(math.Ceil(stepLength / playerCollisionStep))
		if steps < 1 {
			steps = 1
		}
		for step := 0; step < steps; step++ {
			candidateX, candidateY := p.x+moveX/float64(steps), p.y+moveY/float64(steps)
			for resolve := 0; resolve < 4; resolve++ {
				pushX, pushY, hit := p.playerTileDisplacement(candidateX, candidateY, radius, tileSize)
				if !hit {
					break
				}
				candidateX += pushX
				candidateY += pushY
			}
			zombiePushX, zombiePushY, zombieHit := p.zombieCollisionDisplacement(candidateX, candidateY, radius)
			if zombieHit {
				candidateX += zombiePushX
				candidateY += zombiePushY
			}
			for resolve := 0; resolve < 4; resolve++ {
				pushX, pushY, hit := p.playerTileDisplacement(candidateX, candidateY, radius, tileSize)
				if !hit {
					break
				}
				candidateX += pushX
				candidateY += pushY
			}
			p.x, p.y = candidateX, candidateY
		}
		if mobile && !scriptFacingLocked && (p.stick != 2 || math.Hypot(p.rightDeflectX, p.rightDeflectY) <= .5) {
			p.angle, p.flipX = barryDirection(dx, dy)
		}
	}
	if primary && p.scriptWalking {
		p.moving = true
	}
	for resolve := 0; resolve < 4 && !(primary && p.scriptWalking); resolve++ {
		// A scripted walk (WalkPlayerTo) skips tile collision natively (FUN_000f3280: the walk-flag branch
		// adds the step straight to the position), so it must not be pushed back out of a wall either.
		pushX, pushY, hit := p.playerTileDisplacement(p.x, p.y, radius, tileSize)
		if !hit {
			break
		}
		p.x += pushX
		p.y += pushY
	}
	if pushX, pushY, hit := p.zombieCollisionDisplacement(p.x, p.y, radius); hit {
		p.x += pushX
		p.y += pushY
		for resolve := 0; resolve < 4; resolve++ {
			pushX, pushY, tileHit := p.playerTileDisplacement(p.x, p.y, radius, tileSize)
			if !tileHit {
				break
			}
			p.x += pushX
			p.y += pushY
		}
	}
	maxX, maxY := float64(p.world.Level.Width*tileSize), float64(p.world.Level.Height*tileSize)
	if maxX < radius*2 {
		p.x = maxX / 2
	} else {
		p.x = math.Max(radius, math.Min(maxX-radius, p.x))
	}
	if maxY < radius*2 {
		p.y = maxY / 2
	} else {
		p.y = math.Max(radius, math.Min(maxY-radius, p.y))
	}
}

// updateBulletsAndKills moves every projectile, applies hits, and removes dead
// zombies with their score and blood pops.
func (p *playState) updateBulletsAndKills() {
	const dt = 1.0 / 60.0
	p.banner.advance(dt)
	p.updateCombo(dt)
	p.achieve.tickShield(dt)
	p.updateTrain()
	activeBullets := p.bullets[:0]
	var childBullets []bullet
	for _, b := range p.bullets {
		if b.projectile != nil && b.projectile.EntityType == sawBladeKind {
			if p.stepSawBlade(&b) {
				activeBullets = append(activeBullets, b)
			}
			continue
		}
		if b.projectile != nil && b.projectile.EntityType == 0x12 {
			remaining, spawn, remove := weapons.NativeWeaponMultiTick(b.projectile.Penetration)
			b.projectile.Penetration = remaining
			if remove {
				continue
			}
			if spawn {
				child := weapons.NativeWeaponMultiChild(*b.projectile)
				childBullets = append(childBullets, bullet{x: child.X, y: child.Y, vx: child.VX, vy: child.VY, life: child.Life, projectile: &child, origin: b.origin})
			}
		}
		b.x += b.vx * dt
		b.y += b.vy * dt
		b.life -= dt
		if b.projectile != nil {
			b.projectile.X, b.projectile.Y, b.projectile.Life = b.x, b.y, b.life
			b.projectile.Age += dt
		}
		if b.life <= 0 || p.bulletInWall(&b) {
			if b.explodes() {
				p.detonateFrom(b.x, b.y, b.explosionSound(), b.origin)
			}
			continue
		}
		hit := false
		if b.projectile != nil && b.projectile.EntityType == 0x12 {
			activeBullets = append(activeBullets, b)
			continue
		}
		for index := range p.zombies {
			if p.zombies[index].health <= 0 {
				continue
			}
			if !p.bulletReachesZombie(b, &p.zombies[index]) {
				continue
			}
			p.markProjectileHit(b.projectile, b.origin)
			if b.explodes() {
				p.rocketContactDamage(b, &p.zombies[index])
				p.classifyZombieDeath(&p.zombies[index], 0x13, b.x, b.y)
				p.detonateFrom(b.x, b.y, b.explosionSound(), b.origin)
				hit = true
				break
			}
			if p.zombies[index].invulnerable {
				hit = true
				if bulletStopsAtFirstTarget(b) {
					break
				}
				continue
			}
			if b.projectile != nil && b.projectile.EntityType == 0x16 {
				b.vx = float64(float32(b.vx) * float32(.85))
				b.vy = float64(float32(b.vy) * float32(.85))
				b.projectile.VX, b.projectile.VY = b.vx, b.vy
			}
			// FUN_000a521c / FUN_000a4bbc: fixed damage per hit (101 / 1) against the
			// zombie's real health (zombie_model.go).
			if p.damageFromBullet(b, &p.zombies[index]) {
				p.creditKill(b.origin)
				p.zombies[index].dying = true
				p.zombies[index].deathAge = 0
				p.classifyZombieDeath(&p.zombies[index], bulletNativeKind(b), b.x, b.y)
			}
			hit = true
			if bulletStopsAtFirstTarget(b) {
				break
			}
		}
		if b.projectile != nil && (b.projectile.EntityType == 0x11 || b.projectile.EntityType == 0x16) {
			// Multi-bullet children and flame particles pass through targets (saw
			// blades have their own pass, stepSawBlade).
			hit = false
		}
		if !hit {
			activeBullets = append(activeBullets, b)
		}
	}
	p.bullets = append(activeBullets, childBullets...)
	alive := p.zombies[:0]
	for _, zombie := range p.zombies {
		if zombie.dying && zombie.deathAge < p.zombieDeathDelayFor(zombie) {
			alive = append(alive, zombie)
			continue
		}
		if !zombie.dying && zombie.health > 0 {
			alive = append(alive, zombie)
			continue
		}
		if entity := p.scriptEntities[zombie.scriptID]; entity != nil && isBossType(entity.entityType) && zombie.dying {
			p.onBossDefeated(entity.entityType, zombie.texture)
		}
		p.explodeZombie(zombie)
		p.levelKills = int(int32(p.levelKills) + 1)
		if p.hudVisible {
			points := int32(zombie.rawPoints) / 10 * 10
			award := (points / 20) * int32(p.multiplier)
			p.score = int(int32(p.score) + award)
		}
		if !zombie.spawnAway {
			if zombie.dying && zombie.deathState == zombieDeathGib {
				// A gib makes no blood pop: the body plays Disintegrate (zombie_gib.go).
				p.addZombieGibBody(zombie)
				if !p.isExplodingZombie(zombie) {
					p.queueZombieDeathSound()
				}
				continue
			}
				pop := bloodPop{x: zombie.x, y: zombie.y, variant: len(p.bloodPops) % 3}
			if zombie.native.kind != 0 {
				pop.variant, pop.x = p.zombieDeathPop(zombie)
			}
			p.bloodPops = append(p.bloodPops, pop)
			if zombie.dying && !p.isExplodingZombie(zombie) {
				p.queueZombieDeathSound()
			}
		}
	}
	p.zombies = alive
}

func (p *playState) updateWaves() {
	if p.cheats.freezeWaves {
		return
	}
	if p.world == nil || len(p.world.Level.Waves) == 0 || p.waveIndex >= len(p.world.Level.Waves) {
		return
	}
	p.noteWave()
	// The wave block (FUN_000c0cec / FUN_00123180) runs only while the living enemy count is below the build's
	// limit. The gate reads the count before this frame's spawns, and the spawners, the timer and the advance
	// all sit behind it, so they pause together (waveZombieLimit).
	if p.livingZombies() >= p.waveGateLimit() {
		return
	}
	wave := p.waveNow()
	if len(p.waveSpawned) != len(wave.Spawners) {
		p.waveSpawned = make([]int, len(wave.Spawners))
	}
	p.waveElapsed += 1000.0 / 60.0
	allSpawned := true
	for index, spawner := range wave.Spawners {
		count := spawner.Count
		if count <= 0 {
			// FUN_000bf120 drops a spawner whose count is below 1 right after its first FUN_000bec48 call, and that
			// call still spawns once when the delay timer is already due: delay 0 / count 0 (the president boss,
			// a few bonus crates) spawns one, a delayed count 0 spawner never does.
			count = 0
			if spawner.DelayTime <= 1000.0/60 {
				count = 1
			}
		}
		if spawner.Index < 1 || spawner.Index > 13 || count <= 0 || len(spawner.Types) == 0 {
			continue
		}
		interval := waveSpawnerInterval(wave.RunTime, spawner.DelayTime, count)
		for p.waveSpawned[index] < count && p.waveElapsed >= spawner.DelayTime+float64(p.waveSpawned[index])*interval {
			p.spawnZombieFor(spawner, p.waveTurns(index), p.waveSpawned[index])
			p.waveSpawned[index]++
		}
		if p.waveSpawned[index] < count {
			allSpawned = false
		}
	}
	alive := p.livingZombies()
	if waveEndRule == waveEndAllDead {
		alive = p.waveAliveForEnd()
	}
	if p.waveEndStep(wave, !allSpawned, alive) {
		// The next wave is next_wave, or the one after this when next_wave is below 1. Past the last wave none
		// follows and the level ends. The banner moves with the index on this same frame.
		next := wave.NextWave
		if next < 1 {
			next = p.waveIndex + 1
		}
		if next < len(p.world.Level.Waves) {
			p.survivalAdvance(next)
			p.waveIndex = next
			p.waveElapsed = 0
			p.waveSpawned = nil
		} else {
			p.waveIndex, p.wavesFinished = next, true
		}
		p.noteWave()
	}
}
func waveSpawnerInterval(runTime, delayTime float64, count int) float64 {
	interval := (runTime - delayTime) / float64(count)
	if interval <= 0 {
		return 500
	}
	return interval
}

// spawnZombieAt creates one zombie of the given type, opening a portal there.
func (p *playState) spawnZombieAt(entry formats.SpawnType, point formats.Vec2) {
	p.spawnZombieAtTurn(entry, point, nil)
}

// spawnZombieAtTurn is spawnZombieAt. A non-nil turn is the native turnSpeed range of a survival type (survival.go).
func (p *playState) spawnZombieAtTurn(entry formats.SpawnType, point formats.Vec2, turn *turnRange) {
	native := nativeZombieTypes[entry.Name]
	waveZombie := native >= zombieKindPlain && native <= zombieKindProspect
	var record zombieSpawnRecord
	speed := entry.Speed.X
	if entry.Speed.Y > 0 {
		speed = (entry.Speed.X + entry.Speed.Y) / 2
	}
	if speed <= 0 {
		speed = 70
	}
	health, rawPoints := 100.0, 0
	if entry.Strength >= 0 {
		rawPoints = int(entry.Strength)
		health = float64(rawPoints)
	}
	if waveZombie {
		// Types 2..9 come from the native spawn record (zombie_model.go): rolled speed,
		// size and strength instead of the averages the port used before.
		if turn != nil {
			record = rollSurvivalSpawnRecord(entry, *turn, p.rng)
		} else {
			record = rollZombieSpawnRecord(entry, p.rng)
		}
		speed = float64(record.Speed)
		if record.Speed < 0 {
			speed = 30 + float64(zombieRandom(p.rng, 30))
		}
	}
	texture := entry.Texture
	if texture == "" {
		texture = "cavezombie"
	}
	p.addPortal(point.X, point.Y)
	if p.scriptEntities == nil {
		p.scriptEntities = map[int]*scriptEntity{}
	}
	if p.scriptNextEntity <= 0 {
		p.scriptNextEntity = 1
	}
	id := p.scriptNextEntity
	p.scriptNextEntity++
	p.scriptEntities[id] = &scriptEntity{id: id, kind: "zombie", entityType: entry.Name, x: point.X, y: point.Y, scaleX: 1, scaleY: 1, alpha: 1, texture: texture, speed: speed}
	p.markRexSpawn(id, entry.Name)
	if name := bossIntroScript(entry.Name, texture); name != "" {
		p.bossScripts = append(p.bossScripts, name)
	}
	if waveZombie {
		z := newWaveZombie(record, p.rng, p.zombieWeapons, point, speed, texture, id)
		z.fps = p.spriteFPS(texture, "")
		p.zombies = append(p.zombies, z)
		return
	}
	renderSize := nativeSpawnRenderSize(entry.Size)
	collision := nativeSpawnCollisionRadius(entry.Size)
	p.zombies = append(p.zombies, zombieState{x: point.X, y: point.Y, speed: speed, health: health, rawPoints: rawPoints, size: formats.Vec2{X: renderSize, Y: renderSize}, collision: collision, texture: texture, scriptID: id, alpha: 1, fps: p.spriteFPS(texture, "")})
}

func (p *playState) spawnZombie(spawner formats.Spawner, ordinal int) {
	p.spawnZombieFor(spawner, nil, ordinal)
}

// spawnZombieFor is spawnZombie for one spawner of the running wave. turns are the spawner's native turn ranges after
// a survival advance (survival.go); nil keeps each type's single turnSpeed.
func (p *playState) spawnZombieFor(spawner formats.Spawner, turns []turnRange, ordinal int) {
	points := p.spawnPoints(spawner.Index)
	if len(points) == 0 {
		return
	}
	index, ok := chooseSpawnIndex(spawner.Types, p.rng)
	if !ok {
		return
	}
	entry := spawner.Types[index]
	point := p.nativeSpawnPoint(points) // native: random tile and random point inside it (wave_timing.go)
	if entry.Name == "train" {
		p.spawnTrain()
		return
	}
	if strings.HasPrefix(strings.ToLower(entry.Name), "p_") {
		p.spawnPickup(entry.Name, point)
		return
	}
	if isBossType(entry.Name) && p.bossPresent(entry.Name) {
		// A script (AddRobotBossZombie, AddWesternBossZombie) already staged this boss.
		return
	}
	if turns != nil && index < len(turns) {
		turn := turns[index]
		p.spawnZombieAtTurn(entry, point, &turn)
		return
	}
	p.spawnZombieAt(entry, point)
}

// nativeSpawnCollisionRadius is the body radius from the spawner's declared size,
// kept separate from the larger render size so zombies fit through narrow gaps.
func nativeSpawnCollisionRadius(declared formats.Vec2) float64 {
	size := math.Max(declared.X, declared.Y)
	if size <= 0 {
		return 0
	}
	return size / 2
}

// nativeSpawnRenderSize scales the default zombie size by the spawner's declared
// size against the standard 29x31 zombie, so bigger types spawn larger.
func nativeSpawnRenderSize(declared formats.Vec2) float64 {
	size := declared.X
	if declared.Y > 0 {
		size = (declared.X + declared.Y) / 2
	}
	const standard = 30.0
	if size <= 0 {
		return nativeZombieDefaultRenderSize
	}
	return nativeZombieDefaultRenderSize * math.Max(.5, math.Min(3, size/standard))
}

func (p *playState) addPortal(x, y float64) {
	cellX, cellY := portalCell(x, y)
	for index := range p.portals {
		if p.portals[index].cellX != cellX || p.portals[index].cellY != cellY {
			continue
		}
		p.portals[index].age = 0
		return
	}
	p.portals = append(p.portals, portalState{x: x, y: y, cellX: cellX, cellY: cellY, animationTimer: 100})
}

func portalCell(x, y float64) (int, int) {
	cellX, cellY := int(x*0.015625), int(y*0.015625)
	if cellX < 0 {
		cellX = 0
	} else if cellX > 0x22 {
		cellX = 0x22
	}
	if cellY < 0 {
		cellY = 0
	} else if cellY > 0x12 {
		cellY = 0x12
	}
	return cellX, cellY
}

func bulletHitsZombie(previousX, previousY, x, y float64, zombie zombieState) bool {
	left := math.Min(previousX, x) - 4
	right := math.Max(previousX, x) + 4
	top := math.Min(previousY, y) - 8
	bottom := math.Max(previousY, y) + 8
	halfWidth, halfHeight := zombie.size.X/2, zombie.size.Y/2
	if halfWidth <= 0 {
		halfWidth = 16
	}
	if halfHeight <= 0 {
		halfHeight = 16
	}
	return zombie.x >= left-halfWidth && zombie.x <= right+halfWidth && zombie.y >= top-halfHeight && zombie.y <= bottom+halfHeight
}

// chooseSpawnType is the weighted type roll of a spawner (see chooseSpawnIndex).
func chooseSpawnType(types []formats.SpawnType, rng *weapons.NativeRNG) (formats.SpawnType, bool) {
	index, ok := chooseSpawnIndex(types, rng)
	if !ok {
		return formats.SpawnType{}, false
	}
	return types[index], true
}

// chooseSpawnIndex is the weighted type roll: rnd(total chance), then the first type whose running chance covers it.
func chooseSpawnIndex(types []formats.SpawnType, rng *weapons.NativeRNG) (int, bool) {
	total := uint32(0)
	for _, entry := range types {
		if entry.Chance > 0 {
			total += uint32(int64(entry.Chance))
		}
	}
	if total == 0 {
		return 0, false
	}
	value := rng.Bounded(total)
	for index, entry := range types {
		if entry.Chance <= 0 {
			continue
		}
		chance := uint32(int64(entry.Chance))
		if value < chance {
			return index, true
		}
		value -= chance
	}
	return 0, false
}

func (p *playState) spawnPoints(index int) []formats.Vec2 {
	if p.world == nil {
		return nil
	}
	layer := p.world.Level.Layers[formats.LayerC]
	marker := uint32(index + 2)
	points := make([]formats.Vec2, 0)
	for y := 0; y < p.world.Level.Height; y++ {
		for x := 0; x < p.world.Level.Width; x++ {
			if layer[y*p.world.Level.Width+x] == marker {
				points = append(points, formats.Vec2{X: float64(x*p.tileSize + p.tileSize/2), Y: float64(y*p.tileSize + p.tileSize/2)})
			}
		}
	}
	return points
}

func (p *playState) updateZombies() {
	if p.world == nil {
		return
	}
	const dt = 1.0 / 60.0
	p.refreshNav()
	p.updateAlertNoise(dt)
	p.updateZombieGibBodies(dt)
	p.updateZombieShots(dt)
	for index := range p.zombies {
		zombie := &p.zombies[index]
		zombie.hitFlash = math.Max(0, zombie.hitFlash-dt)
		zombie.rexRageTimer = math.Max(0, zombie.rexRageTimer-1000*dt)
		if zombie.dying {
			zombie.deathAge += dt
			continue
		}
		if zombie.spawnAway || zombie.health <= 0 || p.cheats.freezeZombies {
			continue
		}
		prey := p.nearestPlayer(zombie.x, zombie.y)
		targetX, targetY := prey.x, prey.y
		var entity *scriptEntity
		if zombie.scriptID != 0 {
			entity = p.scriptEntities[zombie.scriptID]
			if entity != nil && entity.walking {
				targetX, targetY = entity.targetX, entity.targetY
			} else if p.scriptHasZombieTarget && p.scriptRuntime != nil && !p.scriptRuntime.Done() {
				targetX, targetY = p.scriptZombieTargetX, p.scriptZombieTargetY
			}
		}
		dx, dy := targetX-zombie.x, targetY-zombie.y
		distance := math.Hypot(dx, dy)
		zombieRadius := zombieCollisionRadius(*zombie)
		speed := zombie.speed
		if p.rexHeld(zombie) {
			speed = 0
		}
		speed *= p.rexSpeedFactor(zombie)
		scriptWalk := entity != nil && entity.walking
		stopDistance := playerCollisionRadius + zombieRadius
		if scriptWalk {
			stopDistance = entity.targetRange
		}
		moved := false
		unstickIntent, unstickAlert := 0.0, false
		scriptIdle := p.scriptRuntime == nil || p.scriptRuntime.Done() || p.scriptZombiesActive
		switch {
		case scriptWalk && distance < stopDistance:
			// native arrival test (FUN_001013c4): squared distance strictly below the squared range
			entity.walking = false
			if entity.stopOnArrival {
				zombie.speed, entity.speed = 0, 0
			}
		case scriptWalk && p.scriptWalkGivesUp(entity, distance, dt):
			// Port watchdog, not in the native code: a scripted zombie that cannot get any closer for
			// scriptWalkPatience seconds (a wall corner between its spawn ring point and the target, a script that
			// zeroed its speed) would hold the level-complete cutscene in its IsZombieWalking loop forever.
			entity.walking = false
			if entity.stopOnArrival {
				zombie.speed, entity.speed = 0, 0
			}
		case zombie.native.kind != 0 && !scriptWalk:
			// Spawn-record zombies run the native update (zombie_ai.go).
			aimX, aimY := dx, dy
			if zombie.native.kind == zombieKindSmart && !p.lineClear(zombie.x, zombie.y, targetX, targetY, zombieRadius*.8) {
				if navX, navY, ok := p.navDirection(zombie.x, zombie.y, prey.index); ok {
					aimX, aimY = navX*math.Max(distance, 2), navY*math.Max(distance, 2)
				}
			}
			forceX, forceY, forced := p.zombieUnstickDirection(zombie, prey.index) // port addition (zombie_unstick.go)
			if forced {
				aimX, aimY = forceX*math.Max(distance, 2), forceY*math.Max(distance, 2)
			}
			moveX, moveY := p.stepZombieAI(zombie, dx, dy, aimX, aimY, dt)
			if forced {
				zombieUnstickAssist(zombie, forceX, forceY)
			}
			if scriptIdle && speed > 0 {
				zombie.x += moveX
				zombie.y += moveY
				moved = moveX != 0 || moveY != 0
				unstickIntent, unstickAlert = math.Hypot(moveX, moveY), true
			}
		case scriptWalk || scriptIdle:
			// Staged zombies (scripts, capture scenes, bosses): straight chase.
			if distance > 0 && (distance > stopDistance || scriptWalk) && speed > 0 {
				step := math.Min(speed*dt, distance-stopDistance)
				if scriptWalk {
					// Native steps the full speed*dt toward the target (the clamp above landed exactly on the range edge,
					// where float rounding then kept the strict `<` arrival test false forever).
					step = speed * dt
				}
				dirX, dirY := p.rexSteer(zombie, prey.index, dx/distance, dy/distance)
				zombie.x += dirX * step
				zombie.y += dirY * step
				moved = step > 0
				if p.isRexBoss(zombie) {
					unstickIntent = step // port addition (zombie_unstick.go)
				}
			}
		}
		// FUN_000a17cc: after every move the body is pushed out of blocking tiles.
		if moved {
			pushX, pushY := p.nativeTilePush(zombie.x, zombie.y, zombieBodyScale*zombie.size.X)
			zombie.x += pushX
			zombie.y += pushY
			p.zombieUnstickObserve(zombie, distance, unstickIntent, pushX, pushY, dt, unstickAlert)
		} else {
			p.zombieUnstickObserve(zombie, distance, 0, 0, 0, dt, unstickAlert)
		}
		p.separateZombie(index, dt)
		if zombie.native.kind == zombieKindSpeedy {
			stepZombieTrail(zombie)
		}
		if !scriptWalk && (!zombie.scriptControlled || p.scriptCollideZombies) && !p.isRexBoss(zombie) {
			p.zombieContact(zombie, prey, dt)
		}
		if entity != nil {
			entity.x, entity.y = zombie.x, zombie.y
		}
		if entity == nil || !entity.rotationSet {
			if zombie.native.kind != 0 {
				zombie.angle, zombie.flipX = barryDirection(cosU16(zombie.native.facing), sinU16(zombie.native.facing))
			} else {
				zombie.angle, zombie.flipX = barryDirection(dx, dy)
			}
		}
		fps := zombie.fps
		if fps <= 0 {
			fps = p.spriteFPS(zombie.texture, "")
		}
		if zombie.native.kind != 0 {
			fps *= zombie.native.ai.speedFactor
		}
		zombie.frame += dt * fps * p.rexAnimationFactor(zombie)
	}
	p.updateRexBoss()
}

func (p *playState) updatePlayerDeath() bool {
	if p.health > 0 {
		p.deathStarted = false
		return false
	}
	if !p.deathStarted {
		// One death, one blood pop: with no lives left (co-op, survival) the timer
		// stays at zero and must not start another death every two seconds.
		p.deathStarted = true
		p.deathTimer = 2
		p.achieve.died = true
		p.resetMultiplier()
		if p.coopActive() {
			if p.lives > 0 {
				p.lives--
			}
		} else {
			p.lives-- // native FUN_00094c6c: +0x3ec -= 1; only a negative counter ends the run
		}
		p.vitals.speedBoost = 0
		// FUN_00094c6c stores the respawn spot (+0x48) at the moment of death: the least
		// crowded start marker (FUN_000bdf7c, player_vitals.go).
		p.spawnX, p.spawnY = p.respawnPoint()
		// FUN_00094c6c on death: FUN_0009458c(player, 0, 0) puts the pistol back in
		// the primary slot (the secondary slot reset, FUN_0009458c(player, 8, 0), is
		// UNRESOLVED: the attach ammo of the type-8 weapon was not decoded).
		p.equipWeapon(p.pistolWeapon())
		p.bloodPops = append(p.bloodPops, bloodPop{x: p.x, y: p.y, variant: len(p.bloodPops) % 3})
		p.scriptWalking = false
		p.moveControl, p.shootControl = false, false
	}
	p.deathTimer = math.Max(0, p.deathTimer-1.0/60.0)
	if p.deathTimer > 0 || p.outOfLives() {
		return true
	}
	p.x, p.y = p.spawnX, p.spawnY
	p.health = p.maxHealth
	p.startRespawnGrace()
	p.achievementTracking.Reset()
	p.achievementMotionX, p.achievementMotionY = 0, 0
	p.deathTimer = 0
	p.moveControl, p.shootControl = true, true
	return false
}

func (p *playState) updatePortals() {
	const portalLifetime = 2.0
	const portalCloseRemaining = -0.6
	const portalRemoveRemaining = -1.0
	active := p.portals[:0]
	for _, portal := range p.portals {
		portal.age += 1.0 / 60.0
		portal.rotationUnits -= 1.0 / 60.0 * portal.rotationSpeed * portalRotationUnitsPerSecond
		if portal.animationTimer < 1 {
			portal.frame = (portal.frame + 1) % 4
			portal.animationTimer = 100
		} else {
			portal.animationTimer = math.Trunc(portal.animationTimer - 1000.0/60.0)
		}
		remaining := portalLifetime - portal.age
		targetSize := 140.0
		if remaining < portalCloseRemaining {
			targetSize = 0
		}
		portal.size += (targetSize - portal.size) * .1
		targetRotationSpeed := portalOpenRotationSpeed
		if remaining < portalCloseRemaining {
			targetRotationSpeed = 0
		}
		portal.rotationSpeed += (targetRotationSpeed - portal.rotationSpeed) * portalRotationLerp
		if remaining >= portalRemoveRemaining {
			active = append(active, portal)
		}
	}
	p.portals = active
}

func (p *playState) updateBloodPops() {
	active := p.bloodPops[:0]
	for _, pop := range p.bloodPops {
		pop.age += 1.0 / 60.0
		duration := .5
		if animation, ok := zombieDeathAnimation(p.sprites, pop.variant); ok && animation.FPS > 0 && animation.Frames > 0 {
			duration = float64(animation.Frames) / animation.FPS
		}
		if pop.age < duration {
			active = append(active, pop)
		}
	}
	p.bloodPops = active
}
func (p *playState) updateExplosions() {
	p.updateThrown()
	p.updateZombieBlasts()
	active := p.explosions[:0]
	for _, explosion := range p.explosions {
		explosion.age += 1.0 / 60.0
		if explosion.age <= weapons.NativeGrenadeExplosionDuration {
			active = append(active, explosion)
		}
	}
	p.explosions = active
}

func zombieCollisionRadius(zombie zombieState) float64 {
	if zombie.collision > 0 {
		return zombie.collision
	}
	radius := math.Max(zombie.size.X, zombie.size.Y) / 2
	if radius <= 0 {
		return 16
	}
	return radius
}

func (p *playState) zombieCollisionDisplacement(x, y, radius float64) (float64, float64, bool) {
	if !playerBlockedByZombies {
		return 0, 0, false
	}
	if p.scriptRuntime != nil && !p.scriptRuntime.Done() && !p.scriptCollideZombies {
		return 0, 0, false
	}
	bestX, bestY, bestPenetration := 0.0, 0.0, 0.0
	for _, zombie := range p.zombies {
		if zombie.spawnAway {
			continue
		}
		if zombie.dying {
			continue
		}
		dx, dy := x-zombie.x, y-zombie.y
		minimumDistance := radius + zombieCollisionRadius(zombie)
		distance := math.Hypot(dx, dy)
		if distance >= minimumDistance {
			continue
		}
		if distance < .0001 {
			dx, dy = x-p.x, y-p.y
			distance = math.Hypot(dx, dy)
			if distance < .0001 {
				dx, dy, distance = 1, 0, 1
			}
		}
		penetration := minimumDistance - distance
		if penetration > bestPenetration {
			bestX = dx / distance * penetration
			bestY = dy / distance * penetration
			bestPenetration = penetration
		}
	}
	return bestX, bestY, bestPenetration > 0
}

func (p *playState) equipWeapon(weapon formats.Weapon) {
	if p.weapon.GunType == "MINIGUN" && weapons.WeaponAudioValid(p.spinAudio.Current) {
		p.spinEvents.Add([]weapons.WeaponAudioEvent{{Sound: p.spinAudio.Current, Stop: true}})
	}
	p.weapon, p.shootCooldown, p.flash = weapon, 0, 0
	p.gun.stale = true // a new weapon object: tickGun attaches it (0x000a9eb8 / 0x000abf44)
	p.achieve.pickupSerial++
	if weapon.GunType == "MINIGUN" {
		p.spinAudio.SpinAttach()
	}
}
func (p *playState) fire(dx, dy float64) bool {
	switch p.weapon.GunType {
	case "BUZZSAW":
		// The saw never fires from the trigger: its update spawns the blades every
		// frame (tickGun, saw_blade.go).
		return false
	case "DUALPISTOL":
		// The dual pistol's own timer gates the shot (0x000a8f98 / 0x000aadac).
		p.gunHeld = true
		if !p.gun.DualReady(float32(p.weapon.RateOfFire)) {
			return false
		}
	}
	dist := math.Hypot(dx, dy)
	if dist < 0.0001 {
		return false
	}
	if p.weapon.Speed <= 0 || p.weapon.Life <= 0 || p.weapon.RateOfFire <= 0 {
		return false
	}
	if p.weapon.GunType == "MINIGUN" && p.spinAudio.SpinTimer < float32(p.weapon.RateOfFire) {
		return false
	}
	dirX, dirY := dx/dist, dy/dist
	dirX0, dirY0 := dirX, dirY // the facing column indexes the native tables
	offsetX, offsetY, _, _ := muzzleTransform(dirX, dirY)
	bx := p.x + offsetX
	by := p.y + offsetY
	dirX, dirY = p.projectileDirection(dirX, dirY, offsetX, offsetY)
	p.shotSound = p.weapon.SFXShoot
	p.addFireNoise()
	if weapons.NativeVolleyWeapon(p.weapon.GunType) {
		if p.weapon.Ammo <= 0 {
			if pistol, ok := p.weapons.Find("PISTOL"); ok {
				p.equipWeapon(pistol)
				return p.fire(dx, dy)
			}
			return false
		}
		if p.rng == nil {
			rng := weapons.NewNativeRNG()
			p.rng = &rng
		}
		if p.weapon.GunType == "SHOTGUN" || p.weapon.GunType == "SNIPER" {
			p.shotSound = weapons.NativeWeaponSoundRange(p.weapon.SFXStart, p.weapon.SFXEnd, p.rng.Bounded(0))
		}
		volley, err := weapons.NativePrimaryVolley(p.weapon, weapons.NativeWeaponDirection(dirX, dirY), p.rng)
		if err != nil {
			return false
		}
		spawnX, spawnY := bx, by
		if p.weapon.GunType == "DUALPISTOL" {
			// 0x000aa174: the bullet leaves from the weapon position plus the firing
			// hand's per-direction offset (tables 0x005d9cd8 / 0x005d9c18), not the
			// generic muzzle point; the hand flag toggles every shot.
			handX, handY := p.gun.DualShot(muzzleColumn(dirX0, dirY0))
			spawnX, spawnY = p.x+float64(handX), p.y+float64(handY)
		}
		origin := p.primaryOrigin(p.newShot())
		p.achieve.shots++
		p.achieve.nonPistol = true
		for _, shot := range volley.Shots {
			projectile, ok := weapons.NewNativeWeaponProjectile(shot, p.weapon.BulletType, spawnX, spawnY, p.rng)
			if !ok {
				return false
			}
			p.bullets = append(p.bullets, bullet{x: projectile.X, y: projectile.Y, vx: projectile.VX, vy: projectile.VY, life: projectile.Life, projectile: &projectile, origin: origin})
		}
		p.weapon.Ammo -= volley.AmmoConsumed
		if p.weapon.GunType == "MINIGUN" {
			p.spinAudio.SpinShot()
		}
		if p.weapon.GunType != "DUALPISTOL" {
			p.flash, p.shootCooldown = nativeWeaponFlashDuration, p.weapon.RateOfFire
		}
		if p.statistics != nil {
			p.statistics.ShotsFired++
			p.statistics.Available["Shots Fired"] = true
		}
		return len(volley.Shots) > 0
	}
	bvx := dirX * p.weapon.Speed
	bvy := dirY * p.weapon.Speed
	bAngle := math.Atan2(dirY, dirX) + math.Pi/2
	p.bullets = append(p.bullets, bullet{
		x:      bx,
		y:      by,
		vx:     bvx,
		vy:     bvy,
		life:   p.weapon.Life,
		angle:  bAngle,
		origin: p.primaryOrigin(p.newShot()),
	})
	p.achieve.shots++
	if p.weapon.GunType != "PISTOL" {
		p.achieve.nonPistol = true
	}
	p.flash = nativeWeaponFlashDuration
	p.shootCooldown = p.weapon.RateOfFire
	if p.statistics != nil {
		p.statistics.ShotsFired++
		p.statistics.Available["Shots Fired"] = true
	}
	return true
}
func (p *playState) fireSecondary(dx, dy float64) bool {
	if p.secondaryType != "" && p.secondaryType != secondaryGrenade {
		return p.fireSecondaryOther(dx, dy)
	}
	weapon, ok := p.weapons.Find("GRENADE")
	if !ok || p.grenades <= 0 || weapon.Speed <= 0 || weapon.Life <= 0 || weapon.RateOfFire <= 0 {
		return false
	}
	dist := math.Hypot(dx, dy)
	if dist < .0001 {
		return false
	}
	dirX, dirY := dx/dist, dy/dist
	offsetX, offsetY, _, _ := muzzleTransform(dirX, dirY)
	dirX, dirY = p.projectileDirection(dirX, dirY, offsetX, offsetY)
	// FUN_000adbe8: a bouncing bomb thrown at lift 20 (thrown_bomb.go).
	p.throwBomb(p.x+offsetX, p.y+offsetY, dirX, dirY, weapon.Speed, weapon.Life, false, killOrigin{gun: "GRENADE", shot: p.newShot()})
	p.achieve.nonPistol = true
	p.grenades--
	p.shotSound = weapon.SFXShoot
	if p.statistics != nil {
		p.statistics.GrenadesTossed++
		p.statistics.Available["Grenades Tossed"] = true
	}
	p.secondaryShootCooldown = weapon.RateOfFire
	return true
}
func (p *playState) detonateGrenade(x, y float64) {
	p.detonate(x, y, "SFX_GRENADE_EXPLODE")
}

// detonate blasts the area and queues the matching explosion sound.
func (p *playState) detonate(x, y float64, sound string) {
	p.detonateFrom(x, y, sound, killOrigin{})
}

// detonateFrom is detonate with the origin credited for the blast's kills.
func (p *playState) detonateFrom(x, y float64, sound string, origin killOrigin) {
	// The native blast is a damaging projectile that lives .75 s (blast.go), not
	// an instant area hit: spawnBlast queues the sound and animation and
	// updateZombieBlasts applies 5 damage per overlapping tick.
	p.spawnBlast(x, y, sound, origin, true)
}

func muzzleTransform(dx, dy float64) (float64, float64, float64, bool) {
	column, flipX := barryDirection(dx, dy)
	if flipX && column != 0 {
		column = 16 - column
	}
	if column >= len(barryMuzzleOffsets) || math.Hypot(dx, dy) < .0001 {
		return 0, 0, 0, false
	}
	offset := barryMuzzleOffsets[column]
	return offset.x, offset.y, math.Atan2(dy, dx), true
}
func (p *playState) projectileDirection(dx, dy, offsetX, offsetY float64) (float64, float64) {
	if p.stick != 2 {
		dx, dy = dx*nativeFireAimDistance-offsetX, dy*nativeFireAimDistance-offsetY
	}
	length := math.Hypot(dx, dy)
	return dx / length, dy / length
}

func (p *playState) isSolid(x, y float64) bool {
	tileSize := p.tileSize
	if tileSize <= 0 {
		tileSize = 32
	}
	tileX := int(math.Floor(x / float64(tileSize)))
	tileY := int(math.Floor(y / float64(tileSize)))
	return playerCollisionBlocks(p.collisionValue(tileX, tileY))
}
func stickDeflection(x, y, baseX, baseY float64) (float64, float64) {
	dx, dy := x-baseX, y-baseY
	length := math.Hypot(dx, dy)
	if length > 32 {
		dx, dy = dx/length*32, dy/length*32
	}
	return dx / 32, dy / 32
}
func barryDirection(dx, dy float64) (int, bool) {
	flipX := dx < -0.001
	absX := math.Abs(dx)
	if absX < 0.0001 && math.Abs(dy) < 0.0001 {
		return 0, false
	}
	angleRad := math.Atan2(dy, absX)
	t := (math.Pi/2 - angleRad) / math.Pi
	col := int(math.Round(t * 8.0))
	if col < 0 {
		col = 0
	} else if col > 8 {
		col = 8
	}
	return col, flipX
}
func nativeSpriteDirection(degrees float64, columns int) (int, bool) {
	if columns <= 1 {
		return 0, false
	}
	nativeUnits := float32(degrees) * 182.0
	var raw uint16
	if nativeUnits > 0 {
		raw = uint16(int16(int(nativeUnits)))
	}
	nativeDegrees := float64(raw) / 182.04167175
	directionCount := columns*2 - 1
	direction := int(math.Floor(math.Ceil(450-nativeDegrees)/(360/float64(directionCount)))) % directionCount
	if direction < 0 {
		direction += directionCount
	}
	if direction >= columns {
		return directionCount - 1 - direction, true
	}
	return direction, false
}
func barryAimDirection(angle int, flipX bool) (float64, float64) {
	if angle < 0 {
		angle = 0
	} else if angle > 8 {
		angle = 8
	}
	radians := float64(4-angle) * math.Pi / 8
	dx, dy := math.Cos(radians), math.Sin(radians)
	if flipX {
		dx = -dx
	}
	return dx, dy
}
func (p *playState) centerCamera() {
	tileSize := p.tileSize
	if tileSize <= 0 {
		tileSize = 32
	}
	zoom := p.world.Zoom
	worldWidth := float64(p.world.Level.Width * tileSize)
	worldHeight := float64(p.world.Level.Height * tileSize)
	maxX := math.Max(0, worldWidth-float64(logicalWidth)/zoom)
	maxY := math.Max(0, worldHeight-float64(logicalHeight)/zoom)
	p.world.CameraX = math.Max(0, math.Min(maxX, p.x-float64(logicalWidth)/(2*zoom)))
	p.world.CameraY = math.Max(0, math.Min(maxY, p.y-float64(logicalHeight)/(2*zoom)))
	p.world.ViewportX, p.world.ViewportY = 0, 0
}
func (p *playState) updateCamera() {
	if p.scriptCameraPanActive {
		p.scriptCameraPanElapsed += 1.0 / 60.0
		progress := p.scriptCameraPanElapsed / p.scriptCameraPanDuration
		if progress >= 1 {
			progress = 1
		}
		zoom := p.scriptCameraPanStartZoom + (p.scriptCameraPanTargetZoom-p.scriptCameraPanStartZoom)*progress
		p.world.SetZoom(portZoomFromNative(zoom))
		p.setScriptCamera(p.scriptCameraPanStartX+(p.scriptCameraPanTargetX-p.scriptCameraPanStartX)*progress, p.scriptCameraPanStartY+(p.scriptCameraPanTargetY-p.scriptCameraPanStartY)*progress)
		if progress >= 1 {
			p.scriptCameraPanActive = false
		}
		return
	}
	if p.scriptRuntime != nil && !p.scriptRuntime.Done() && !p.scriptCameraFollow {
		return
	}
	tileSize := p.tileSize
	if tileSize <= 0 {
		tileSize = 32
	}
	zoom := p.world.Zoom
	worldWidth := float64(p.world.Level.Width * tileSize)
	worldHeight := float64(p.world.Level.Height * tileSize)
	maxX := math.Max(0, worldWidth-float64(logicalWidth)/zoom)
	maxY := math.Max(0, worldHeight-float64(logicalHeight)/zoom)
	targetX, targetY := p.cameraFocus()
	offsetX, offsetY := 0.0, 0.0
	if p.scriptCameraFollow {
		offsetX, offsetY = p.scriptCameraFollowOffsetX, p.scriptCameraFollowOffsetY
	}
	if p.scriptCameraFollow && p.scriptCameraFollowID > 1 {
		entity := p.scriptEntities[p.scriptCameraFollowID]
		if entity == nil {
			return
		}
		targetX, targetY = entity.x, entity.y
	}
	// SD look-ahead (native FUN_00096818 block, camera_lookahead.go): the centre leads the player by the eased offset.
	if cameraIsV7(p.waveBuild()) && !p.scriptCameraFollow && !p.coopActive() {
		lookX, lookY := p.cameraLookaheadTick(1.0 / 60.0)
		targetX += lookX
		targetY += lookY
	}
	targetX = math.Max(0, math.Min(maxX, targetX+offsetX-float64(logicalWidth)/(2*zoom)))
	targetY = math.Max(0, math.Min(maxY, targetY+offsetY-float64(logicalHeight)/(2*zoom)))
	ease := p.playCameraEase()
	p.world.CameraX = math.Max(0, math.Min(maxX, p.world.CameraX+(targetX-p.world.CameraX)*ease))
	p.world.CameraY = math.Max(0, math.Min(maxY, p.world.CameraY+(targetY-p.world.CameraY)*ease))
	p.world.ViewportX, p.world.ViewportY = 0, 0
}
func (p *playState) collisionDisplacement(x, y, radius float64, tileSize int) (float64, float64, bool) {
	minX := int(math.Floor((x - radius) / float64(tileSize)))
	maxX := int(math.Floor((x + radius) / float64(tileSize)))
	minY := int(math.Floor((y - radius) / float64(tileSize)))
	maxY := int(math.Floor((y + radius) / float64(tileSize)))
	bestX, bestY, bestPen := 0.0, 0.0, math.Inf(1)
	for tileY := minY; tileY <= maxY; tileY++ {
		for tileX := minX; tileX <= maxX; tileX++ {
			if !playerCollisionBlocks(p.collisionValue(tileX, tileY)) {
				continue
			}
			centerX := (float64(tileX) + .5) * float64(tileSize)
			centerY := (float64(tileY) + .5) * float64(tileSize)
			penX := float64(tileSize)/2 + radius - math.Abs(x-centerX)
			penY := float64(tileSize)/2 + radius - math.Abs(y-centerY)
			if penX <= 0 || penY <= 0 {
				continue
			}
			if penX < penY && penX < bestPen {
				pushX := -penX
				if x > centerX {
					pushX = penX
				}
				bestX, bestY, bestPen = pushX, 0, penX
			} else if penY < bestPen {
				pushY := -penY
				if y > centerY {
					pushY = penY
				}
				bestX, bestY, bestPen = 0, pushY, penY
			}
		}
	}
	return bestX, bestY, bestPen != math.Inf(1)
}
func playerCollisionBlocks(value uint32) bool {
	return value == 1 || value == 2
}
func (p *playState) collisionValue(tileX, tileY int) uint32 {
	if tileX < 0 || tileX >= p.world.Level.Width || tileY < 0 || tileY >= p.world.Level.Height {
		return 1
	}
	raw := p.world.Level.Layers[formats.LayerC][tileY*p.world.Level.Width+tileX]
	if raw == ^uint32(0) {
		return 0
	}
	return (raw + 1) & 0xffff
}
// TextureSourceScale lets the map viewer convert SD-authored prop UVs for HD art.
func (a *app) TextureSourceScale(name string) float64 { return a.pack.TextureSourceScale(name) }

func (a *app) Texture(name string) (*ebiten.Image, error) {
	key := strings.ToLower(strings.TrimSuffix(name, ".tex"))
	if image, ok := a.images[key]; ok {
		return image, nil
	}
	path, ok := a.pack.TexturePath(name)
	if !ok {
		return nil, fmt.Errorf("texture %q not found", name)
	}
	reader, err := a.pack.Open(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := ioReadAll(reader)
	if err != nil {
		return nil, err
	}
	source, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	image := ebiten.NewImageFromImage(source)
	a.sources[key] = source
	a.images[key] = image
	return image, nil
}
func (a *app) Source(name string) (image.Image, error) {
	key := strings.ToLower(strings.TrimSuffix(name, ".tex"))
	if source, ok := a.sources[key]; ok {
		return source, nil
	}
	if _, err := a.Texture(name); err != nil {
		return nil, err
	}
	source, ok := a.sources[key]
	if !ok {
		return nil, fmt.Errorf("texture source %q not found", name)
	}
	return source, nil
}
func ioReadAll(reader interface{ Read([]byte) (int, error) }) ([]byte, error) {
	var data bytes.Buffer
	buffer := make([]byte, 32768)
	for {
		count, err := reader.Read(buffer)
		if count > 0 {
			_, _ = data.Write(buffer[:count])
		}
		if err != nil {
			if err.Error() == "EOF" {
				return data.Bytes(), nil
			}
			return nil, err
		}
	}
}
func clamp(value, low, high int) int {
	if high < low {
		return low
	}
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
func clampFloat(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

var colorDark = color.RGBA{10, 12, 18, 255}

// IconPNGs holds the encoded window icons; the entry point embeds and assigns them.
var IconPNGs [][]byte

func loadAppIcons() []image.Image {
	var icons []image.Image
	for _, b := range IconPNGs {
		if len(b) == 0 {
			continue
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err == nil {
			icons = append(icons, img)
		}
	}
	return icons
}

// Run starts the game.
func Run() {
	assets := flag.String("assets", "data", "generated cache, content directory, or APK")
	debug := flag.Bool("debug", false, "enable the diagnostic map viewer and its controls")
	mobile := flag.Bool("mobile", false, "enable the mobile virtual-stick HUD")
	silent := flag.Bool("silent", false, "disable music and sound effects")
	captureDir := flag.String("capture-dir", "", "write rendered state screenshots to this directory")
	captureEvery := flag.Int("capture-every", 0, "capture every N frames; zero captures only state changes")
	captureState := flag.String("capture-state", "", "start a capture probe at loading, title, main-menu, main-menu-hit, level-select, play, play-ready, play-fire, play-fire-left, play-combat, play-zombie-death, play-zombie-shadow, play-tutorial-images, play-pickup, play-pickup-collected, play-portal, play-zombie-portal, play-level:<manifest-id>, or debug-viewer")
	captureFrames := flag.Int("capture-frames", 0, "terminate after this many rendered frames when capturing")
	captureAutoDialogue := flag.Bool("capture-auto-dialogue", false, "advance scripted dialogue during capture probes")
	captureSelection := flag.Int("capture-selection", -1, "select a main-menu item by index for a bounded capture probe")
	waveEnd := flag.String("wave-end", "all-dead", "wave end rule: all-dead (port option: every spawner done and no zombie alive) or native (FUN_000bf120 timer and alive limit)")
	flag.Parse()
	switch *waveEnd {
	case "all-dead":
		waveEndRule = waveEndAllDead
	case "native":
		waveEndRule = waveEndNative
	default:
		log.Fatalf("-wave-end must be all-dead or native, not %q", *waveEnd)
	}
	game, err := newApp(*assets, *debug, *mobile, *silent)
	if err != nil {
		log.Fatal(err)
	}
	game.capture, err = engine.NewCapture(*captureDir, *captureEvery)
	if err != nil {
		log.Fatal(err)
	}
	game.captureLimit = *captureFrames
	if *captureDir != "" {
		game.profileWritable = false
	}
	game.captureAutoDialogue = *captureAutoDialogue
	if *captureState != "" {
		if err := game.setCaptureState(*captureState); err != nil {
			log.Fatal(err)
		}
		if game.debug {
			game.debugPanelVisible = true
		}
	}
	if *captureSelection >= 0 && *captureSelection < len(mainMenuButtons) {
		game.menuSelection = *captureSelection
	}
	defer game.sound.Close()
	defer func() {
		if err := game.savePlayerProfile(); err != nil {
			log.Printf("profile: %v", err)
		}
	}()
	ebiten.SetWindowSize(960, 540)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("HalfBricked")
	if icons := loadAppIcons(); len(icons) > 0 {
		ebiten.SetWindowIcon(icons)
	}
	if err := ebiten.RunGame(game); err != nil && err != ebiten.Termination {
		log.Fatal(err)
	}
}
