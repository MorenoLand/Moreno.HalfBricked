package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
	"github.com/hajimehoshi/ebiten/v2"
)

var scriptCallbacks = []string{
	"LogMessage", "StartSpeech", "IsSpeechRunning", "StopSpeech", "GetCurrentDialogSpeech", "Idle", "PlaySFX", "WaitInit", "IsWaitComplete", "ShowSkip", "StartFadeBlack", "IsFading", "LoadTexture", "SetTexturePos", "SetTextureVisible", "SetTextureScale", "SetTextureUVs", "SetTextureAlpha", "UnloadTextures", "HUDSetVisible", "StartFadeNormal", "CameoShow", "RegisterCameo", "ResetExternal", "DrawText1", "DrawText2", "KillText", "SetTask", "SetLevelToLoad", "CreateEntity", "SetAnimation", "SetFrame", "StopAnimation", "SetBlack", "GetSpriteXPosition", "GetSpriteYPosition", "SetSpriteXPosition", "SetSpriteYPosition", "GetPlatform", "PauseGame", "GetDelta", "DestroyEntity", "GetPlayer", "GetPlayerX", "GetPlayerY", "PlayerLookAt", "SetEntityScale", "SetEntityRotation", "GetEntityRotation", "GetEntityXPos", "GetEntityYPos", "SetEntityPos", "SetEntityColour", "SetEntityAlpha", "SetPlayerFacing", "SetScriptAlpha", "GetScriptAlpha", "SetPlayerAnim", "SetZoom", "GetZoom", "SetCamera", "SetCameraFollow", "SetCameraPan", "CameraShake", "GetCameraX", "GetCameraY", "SpawnEntity", "SpawnZombie", "GetFirstEntityOfType", "GetZombieSpeed", "SetZombieSpeed", "SetZombieAlpha", "SetZombieAnimTime", "SetZombieTarget", "GetZombieCount", "SetZombieTexture", "SetPlayerPos", "WalkPlayerTo", "IsPlayerWalking", "IsXPlayDevice", "ZombieExists", "IsZombieWalking", "SpawnAwayZombie", "WalkZombieTo", "AddPortal", "GetCameoY", "SetEntityVFlip", "GivePlayerShootControl", "SetPlayerMoveControl",
}

func init() {
	scriptCallbacks = append(scriptCallbacks, "AimControlActive", "DefaultThumbStickFree", "DoPlayerSpawn", "FireGun", "ForceDrawReticule", "ForceDrawThumbStick", "ForceEnableSecondary", "IsSecondaryButtonDown", "KillZombie", "MoveControlActive", "NormalControlStyle", "PickupExists", "SetAllowThumbsticksDuringScripts", "SetPlayerCollideWithZombiesInScripts", "SetThumbStickCentre", "SetThumbStickFree", "SetThumbSticksToCorners", "SpawnZombiesAroundPlayer", "StopPlayerShootControl", "TriggerTutorial", "ZoomCameraOut", "AddRobotBossZombie", "AddWesternBossZombie", "GetPositionWithinRadius", "MakeRexRage", "MakeZombieInvulnerable", "MusicEnabled", "SetRobotRage", "SetWesternBossDead", "ShakeInputTriggered", "UnlockWesternBossAchievement", "Update", "ZoomCameraAndMove", "ZoomCameraIn")
}

type scriptEntity struct {
	id                      int
	kind                    string
	entityType              string
	x, y                    float64
	rotation                float64
	scaleX, scaleY, alpha   float64
	texture                 string
	flipY                   bool
	angle                   int
	animation, frame        int
	targetX, targetY, speed float64
	walking                 bool
	playing                 bool
	frameTime               float64
}

type scriptTexture struct {
	id                    int
	name                  string
	x, y                  float64
	scaleX, scaleY, alpha float64
	u1, v1, u2, v2        float64
	visible               bool
	cameo                 int
}

type playScriptHost struct {
	app  *app
	play *playState
}

func (h *playScriptHost) Call(name string, args []scripting.Value) (scripting.CallResult, error) {
	h.play.scriptLastCallback = name
	switch name {
	case "LogMessage":
		if len(args) > 0 {
			log.Printf("script: %v", args[0])
		}
		return scripting.CallResult{}, nil
	case "IsXPlayDevice":
		return scriptValues(h.app.mobile), nil
	case "Idle":
		return scripting.CallResult{Yield: true}, nil
	case "WaitInit":
		amount, err := scriptNumber(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		h.play.scriptWaitRemaining = math.Max(0, amount)
		h.play.scriptWaitActive = true
		h.play.scriptWaitStarts++
		return scripting.CallResult{}, nil
	case "IsWaitComplete":
		if !h.play.scriptWaitActive || h.play.scriptWaitRemaining <= 0 {
			h.play.scriptWaitActive = false
			return scriptValues(1), nil
		}
		return scriptValues(0), nil
	case "PlaySFX":
		name, err := scriptString(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		volume := 1.0
		if len(args) > 1 {
			volume, err = scriptNumber(args, 1)
			if err != nil {
				return scripting.CallResult{}, err
			}
		}
		if path := h.app.scriptSoundPath(name); path != "" {
			h.app.sound.Play(path, volume)
		}
		return scripting.CallResult{}, nil
	case "MusicEnabled":
		enabled, err := scriptBool(args, 0)
		h.app.sound.MusicEnabled(enabled)
		return scripting.CallResult{}, err
	case "HUDSetVisible":
		visible, err := scriptBool(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		h.play.hudVisible = visible
		return scripting.CallResult{}, nil
	case "SetLevelToLoad":
		level, err := scriptString(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		h.play.scriptLevelToLoad = level
		if err := h.loadScriptLevel(level); err != nil {
			return scripting.CallResult{}, err
		}
		return scripting.CallResult{}, nil
	case "SetCamera":
		x, err := scriptNumber(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		y, err := scriptNumber(args, 1)
		if err != nil {
			return scripting.CallResult{}, err
		}
		h.play.setScriptCamera(x, y)
		return scripting.CallResult{}, nil
	case "SetZoom":
		zoom, err := scriptNumber(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		if zoom <= 0 {
			return scripting.CallResult{}, fmt.Errorf("zoom must be positive")
		}
		h.play.world.Zoom = zoom
		return scripting.CallResult{}, nil
	case "GetZoom":
		return scriptValues(h.play.world.Zoom), nil
	case "SetCameraPan":
		zoom, err := scriptNumber(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		x, err := scriptNumber(args, 1)
		if err != nil {
			return scripting.CallResult{}, err
		}
		y, err := scriptNumber(args, 2)
		if err != nil {
			return scripting.CallResult{}, err
		}
		if zoom <= 0 {
			return scripting.CallResult{}, fmt.Errorf("zoom must be positive")
		}
		h.play.world.Zoom = zoom
		h.play.setScriptCamera(x, y)
		return scripting.CallResult{}, nil
	case "GetCameraX":
		return scriptValues(h.play.scriptCameraCenterX()), nil
	case "GetCameraY":
		return scriptValues(h.play.scriptCameraCenterY()), nil
	case "SetCameraFollow":
		follow, err := scriptNumber(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		h.play.scriptCameraFollow = follow != 0
		return scripting.CallResult{}, nil
	case "GetDelta":
		return scriptValues(1.0 / 60.0), nil
	case "SetPlayerPos":
		x, err := scriptNumber(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		y, err := scriptNumber(args, 1)
		if err != nil {
			return scripting.CallResult{}, err
		}
		h.play.x, h.play.y = x, y
		return scripting.CallResult{}, nil
	case "GetPlayerX":
		return scriptValues(h.play.x), nil
	case "GetPlayerY":
		return scriptValues(h.play.y), nil
	case "GetPlayer":
		return scriptValues(1), nil
	case "SetPlayerFacing":
		angle, err := scriptNumber(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		h.play.setScriptFacing(angle)
		return scripting.CallResult{}, nil
	case "PlayerLookAt":
		var x, y float64
		var err error
		if len(args) == 1 {
			id, idErr := scriptID(args, 0)
			if idErr != nil {
				return scripting.CallResult{}, idErr
			}
			entity := h.findEntity(id)
			if entity == nil {
				return scripting.CallResult{}, fmt.Errorf("entity %d not found", id)
			}
			x, y = entity.x, entity.y
		} else {
			x, err = scriptNumber(args, 0)
			if err != nil {
				return scripting.CallResult{}, err
			}
			y, err = scriptNumber(args, 1)
			if err != nil {
				return scripting.CallResult{}, err
			}
		}
		h.play.scriptAimX, h.play.scriptAimY, h.play.scriptHasAim = x, y, true
		h.play.angle, h.play.flipX = barryDirection(x-h.play.x, y-h.play.y)
		return scripting.CallResult{}, nil
	case "SetPlayerMoveControl":
		value, err := scriptBool(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		h.play.moveControl = value
		return scripting.CallResult{}, nil
	case "GivePlayerShootControl":
		h.play.shootControl = true
		return scripting.CallResult{}, nil
	case "WalkPlayerTo":
		x, err := scriptNumber(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		y, err := scriptNumber(args, 1)
		if err != nil {
			return scripting.CallResult{}, err
		}
		h.play.scriptWalkX, h.play.scriptWalkY, h.play.scriptWalking = x, y, true
		return scripting.CallResult{}, nil
	case "IsPlayerWalking":
		if h.play.scriptWalking {
			return scriptValues(1), nil
		}
		return scriptValues(0), nil
	case "SpawnZombie":
		return h.spawnZombie(args)
	case "ZombieExists":
		id, err := scriptID(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		if entity := h.findEntity(id); entity != nil && entity.kind == "zombie" {
			if zombie := h.findZombie(id); zombie != nil && zombie.health > 0 && !zombie.dying {
				return scriptValues(1), nil
			}
			return scriptValues(0), nil
		}
		if h.findEntity(id) != nil {
			return scriptValues(1), nil
		}
		return scriptValues(0), nil
	case "IsZombieWalking":
		id, err := scriptID(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		entity := h.findEntity(id)
		if entity != nil && entity.walking {
			return scriptValues(1), nil
		}
		return scriptValues(0), nil
	case "WalkZombieTo":
		id, err := scriptID(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		x, err := scriptNumber(args, 1)
		if err != nil {
			return scripting.CallResult{}, err
		}
		y, err := scriptNumber(args, 2)
		if err != nil {
			return scripting.CallResult{}, err
		}
		speed, err := scriptNumber(args, 3)
		if err != nil {
			return scripting.CallResult{}, err
		}
		entity := h.findEntity(id)
		if entity == nil {
			return scripting.CallResult{}, fmt.Errorf("entity %d not found", id)
		}
		entity.targetX, entity.targetY, entity.speed, entity.walking = x, y, speed, true
		return scripting.CallResult{}, nil
	case "SpawnAwayZombie":
		id, err := scriptID(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		entity := h.findEntity(id)
		if entity == nil {
			return scripting.CallResult{}, fmt.Errorf("entity %d not found", id)
		}
		entity.walking = false
		return scripting.CallResult{}, nil
	case "SetZombieTarget":
		x, err := scriptNumber(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		y, err := scriptNumber(args, 1)
		if err != nil {
			return scripting.CallResult{}, err
		}
		h.play.scriptZombieTargetX, h.play.scriptZombieTargetY, h.play.scriptHasZombieTarget = x, y, true
		return scripting.CallResult{}, nil
	case "GetZombieCount":
		count := 0
		for _, zombie := range h.play.zombies {
			if !zombie.dying && zombie.health > 0 {
				count++
			}
		}
		return scriptValues(count), nil
	case "GetFirstEntityOfType":
		typeName, err := scriptString(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		ids := make([]int, 0, len(h.play.scriptEntities))
		for id := range h.play.scriptEntities {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		for _, id := range ids {
			entity := h.play.scriptEntities[id]
			if entity == nil || (entity.kind != "zombie" && entity.kind != "sprite") || (!strings.EqualFold(entity.entityType, typeName) && !strings.EqualFold(entity.texture, typeName)) {
				continue
			}
			if entity.kind == "zombie" {
				zombie := h.findZombie(id)
				if zombie == nil || zombie.health <= 0 || zombie.dying {
					continue
				}
			}
			return scriptValues(id), nil
		}
		return scripting.CallResult{}, fmt.Errorf("entity type %q not found", typeName)
	case "GetZombieSpeed", "SetZombieSpeed", "SetZombieAlpha", "SetZombieAnimTime", "SetZombieTexture":
		return h.zombieProperty(name, args)
	case "AddPortal":
		x, err := scriptNumber(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		y, err := scriptNumber(args, 1)
		if err != nil {
			return scripting.CallResult{}, err
		}
		h.play.addPortal(x, y)
		return scripting.CallResult{}, nil
	case "CreateEntity":
		return h.createEntity(args)
	case "SpawnEntity":
		return h.spawnEntity(args)
	case "DestroyEntity":
		id, err := scriptID(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		if _, ok := h.play.scriptEntities[id]; !ok {
			return scripting.CallResult{}, fmt.Errorf("entity %d not found", id)
		}
		delete(h.play.scriptEntities, id)
		return scripting.CallResult{}, nil
	case "GetEntityXPos", "GetSpriteXPosition":
		entity, err := h.entityArg(args)
		if err != nil {
			return scripting.CallResult{}, err
		}
		return scriptValues(entity.x), nil
	case "GetEntityYPos", "GetSpriteYPosition":
		entity, err := h.entityArg(args)
		if err != nil {
			return scripting.CallResult{}, err
		}
		return scriptValues(entity.y), nil
	case "SetEntityPos", "SetSpriteXPosition", "SetSpriteYPosition":
		return h.setEntityPosition(name, args)
	case "GetEntityRotation":
		entity, err := h.entityArg(args)
		if err != nil {
			return scripting.CallResult{}, err
		}
		return scriptValues(entity.rotation), nil
	case "SetEntityRotation":
		entity, err := h.entityArg(args)
		if err != nil {
			return scripting.CallResult{}, err
		}
		rotation, err := scriptNumber(args, 1)
		if err != nil {
			return scripting.CallResult{}, err
		}
		entity.rotation = rotation
		entity.angle, _ = barryDirection(math.Cos(rotation*math.Pi/180), math.Sin(rotation*math.Pi/180))
		if entity.kind == "zombie" {
			if zombie := h.findZombie(entity.id); zombie != nil {
				zombie.angle, zombie.flipX = barryDirection(math.Cos(rotation*math.Pi/180), math.Sin(rotation*math.Pi/180))
			}
		}
		return scripting.CallResult{}, nil
	case "SetEntityVFlip":
		entity, err := h.entityArg(args)
		if err != nil {
			return scripting.CallResult{}, err
		}
		entity.flipY, err = scriptBool(args, 1)
		if entity.kind == "zombie" {
			if zombie := h.findZombie(entity.id); zombie != nil {
				zombie.flipY = entity.flipY
			}
		}
		return scripting.CallResult{}, err
	case "SetEntityScale":
		entity, err := h.entityArg(args)
		if err != nil {
			return scripting.CallResult{}, err
		}
		entity.scaleX, err = scriptNumber(args, 1)
		if err != nil {
			return scripting.CallResult{}, err
		}
		entity.scaleY, err = scriptNumber(args, 2)
		if err == nil && entity.kind == "zombie" {
			if zombie := h.findZombie(entity.id); zombie != nil {
				zombie.size = formats.Vec2{X: entity.scaleX, Y: entity.scaleY}
			}
		}
		return scripting.CallResult{}, err
	case "SetEntityAlpha":
		entity, err := h.entityArg(args)
		if err != nil {
			return scripting.CallResult{}, err
		}
		entity.alpha, err = scriptAlpha(args, 1)
		if err == nil && entity.kind == "zombie" {
			if zombie := h.findZombie(entity.id); zombie != nil {
				zombie.alpha = entity.alpha
			}
		}
		return scripting.CallResult{}, err
	case "SetEntityColour", "SetAnimation", "SetFrame", "StopAnimation":
		return h.entityProperty(name, args)
	case "SetPlayerAnim":
		return scripting.CallResult{}, nil
	case "SetTexturePos":
		texture, err := h.textureArg(args)
		if err != nil {
			return scripting.CallResult{}, err
		}
		texture.x, err = scriptNumber(args, 1)
		if err != nil {
			return scripting.CallResult{}, err
		}
		texture.y, err = scriptNumber(args, 2)
		return scripting.CallResult{}, err
	case "SetTextureVisible":
		texture, err := h.textureArg(args)
		if err != nil {
			return scripting.CallResult{}, err
		}
		texture.visible, err = scriptBool(args, 1)
		return scripting.CallResult{}, err
	case "SetTextureScale":
		texture, err := h.textureArg(args)
		if err != nil {
			return scripting.CallResult{}, err
		}
		texture.scaleX, err = scriptNumber(args, 1)
		if err != nil {
			return scripting.CallResult{}, err
		}
		texture.scaleY, err = scriptNumber(args, 2)
		return scripting.CallResult{}, err
	case "SetTextureUVs":
		texture, err := h.textureArg(args)
		if err != nil {
			return scripting.CallResult{}, err
		}
		values := []*float64{&texture.u1, &texture.v1, &texture.u2, &texture.v2}
		for index, value := range values {
			*value, err = scriptNumber(args, index+1)
			if err != nil {
				return scripting.CallResult{}, err
			}
		}
		return scripting.CallResult{}, nil
	case "SetTextureAlpha":
		texture, err := h.textureArg(args)
		if err != nil {
			return scripting.CallResult{}, err
		}
		texture.alpha, err = scriptAlpha(args, 1)
		return scripting.CallResult{}, err
	case "UnloadTextures":
		h.play.scriptTextures = map[int]*scriptTexture{}
		return scripting.CallResult{}, nil
	case "LoadTexture":
		id, err := scriptID(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		name, err := scriptString(args, 1)
		if err != nil {
			return scripting.CallResult{}, err
		}
		h.play.scriptTextures[id] = &scriptTexture{id: id, name: name, scaleX: 1, scaleY: 1, alpha: 1, cameo: -1}
		return scripting.CallResult{}, nil
	case "RegisterCameo":
		texture, err := h.textureArg(args)
		if err != nil {
			return scripting.CallResult{}, err
		}
		texture.cameo, err = scriptID(args, 1)
		return scripting.CallResult{}, err
	case "CameoShow":
		show, err := scriptBool(args, 0)
		h.play.scriptCameoVisible = show
		return scripting.CallResult{}, err
	case "GetCameoY":
		return scriptValues(0), nil
	case "DrawText1":
		return h.drawScriptText(args, false)
	case "DrawText2":
		return h.drawScriptText(args, true)
	case "KillText":
		h.play.scriptText1, h.play.scriptText2, h.play.scriptTextVisible = "", "", false
		return scripting.CallResult{}, nil
	case "ShowSkip":
		show, err := scriptBool(args, 0)
		h.play.scriptShowSkip = show
		return scripting.CallResult{}, err
	case "StartSpeech":
		name, err := scriptString(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		conversation, err := h.app.pack.Conversation(h.play.world.Level.Info.WorldIndex, name)
		if err != nil {
			return scripting.CallResult{}, err
		}
		h.play.dialogue = conversationLines(conversation)
		h.play.dialogueIndex = 0
		return scripting.CallResult{}, nil
	case "IsSpeechRunning":
		if h.play.dialogueIndex < len(h.play.dialogue) {
			return scriptValues(1), nil
		}
		return scriptValues(0), nil
	case "StopSpeech":
		h.play.dialogueIndex = len(h.play.dialogue)
		return scripting.CallResult{}, nil
	case "GetCurrentDialogSpeech":
		return scriptValues(h.play.dialogueIndex), nil
	case "StartFadeNormal", "StartFadeBlack":
		duration, err := scriptNumber(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		h.play.scriptFadeRemaining = math.Max(0, duration)
		h.play.scriptFadeDuration = math.Max(0, duration)
		h.play.scriptFadeBlack = name == "StartFadeBlack"
		return scripting.CallResult{}, nil
	case "IsFading":
		if h.play.scriptFadeRemaining > 0 {
			return scriptValues(1), nil
		}
		return scriptValues(0), nil
	case "PauseGame":
		paused, err := scriptBool(args, 0)
		h.play.paused = paused
		return scripting.CallResult{}, err
	case "SetBlack":
		h.play.scriptFadeBlack = true
		h.play.scriptFadeRemaining = 0
		h.play.scriptFadeDuration = 0
		return scripting.CallResult{}, nil
	case "SetScriptAlpha":
		alpha, err := scriptAlpha(args, 0)
		h.play.scriptAlpha = alpha
		return scripting.CallResult{}, err
	case "GetScriptAlpha":
		return scriptValues(h.play.scriptAlpha), nil
	case "SetTask":
		return scripting.CallResult{}, nil
	case "GetPlatform":
		if h.app.mobile {
			return scriptValues("android"), nil
		}
		return scriptValues("windows"), nil
	case "ResetExternal":
		h.play.scriptText1, h.play.scriptText2, h.play.scriptTextVisible = "", "", false
		return scripting.CallResult{}, nil
	case "CameraShake":
		return scripting.CallResult{}, fmt.Errorf("CameraShake is not implemented")
	case "AimControlActive":
		if h.play.shootControl {
			return scriptValues(true), nil
		}
		return scriptValues(false), nil
	case "MoveControlActive":
		if h.play.moveControl {
			return scriptValues(true), nil
		}
		return scriptValues(false), nil
	case "NormalControlStyle":
		return scriptValues(true), nil
	case "DoPlayerSpawn":
		layer := h.play.world.Level.Layers[formats.LayerC]
		for y := 0; y < h.play.world.Level.Height; y++ {
			for x := 0; x < h.play.world.Level.Width; x++ {
				if layer[y*h.play.world.Level.Width+x] == 2 {
					h.play.x = float64(x*h.play.tileSize + h.play.tileSize/2)
					h.play.y = float64(y*h.play.tileSize + h.play.tileSize/2)
					h.play.scriptWalking = false
					return scripting.CallResult{}, nil
				}
			}
		}
		return scripting.CallResult{}, fmt.Errorf("player spawn marker (collision value 3) not found")
	case "StopPlayerShootControl":
		h.play.shootControl = false
		return scripting.CallResult{}, nil
	case "ForceDrawReticule":
		show, err := scriptBool(args, 0)
		h.play.scriptForceReticule = show
		return scripting.CallResult{}, err
	case "ForceDrawThumbStick":
		index, err := scriptID(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		show, err := scriptBool(args, 1)
		if err != nil {
			return scripting.CallResult{}, err
		}
		if index < 0 || index >= len(h.play.scriptForceThumbStick) {
			return scripting.CallResult{}, fmt.Errorf("thumbstick %d is out of range", index)
		}
		h.play.scriptForceThumbStick[index] = show
		return scripting.CallResult{}, nil
	case "ForceEnableSecondary":
		enabled, err := scriptBool(args, 0)
		h.play.scriptSecondaryEnabled = enabled
		return scripting.CallResult{}, err
	case "IsSecondaryButtonDown":
		return scriptValues(ebiten.IsKeyPressed(ebiten.KeyQ)), nil
	case "SetAllowThumbsticksDuringScripts":
		allowed, err := scriptBool(args, 0)
		h.play.scriptAllowThumbsticks = allowed
		return scripting.CallResult{}, err
	case "SetPlayerCollideWithZombiesInScripts":
		allowed, err := scriptBool(args, 0)
		h.play.scriptCollideZombies = allowed
		return scripting.CallResult{}, err
	case "SetThumbStickFree":
		index, err := scriptID(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		free, err := scriptBool(args, 1)
		if err != nil {
			return scripting.CallResult{}, err
		}
		if index < 0 || index >= len(h.play.scriptThumbStickFree) {
			return scripting.CallResult{}, fmt.Errorf("thumbstick %d is out of range", index)
		}
		h.play.scriptThumbStickFree[index] = free
		return scripting.CallResult{}, nil
	case "DefaultThumbStickFree":
		h.play.scriptThumbStickFree = [2]bool{true, true}
		return scripting.CallResult{}, nil
	case "SetThumbSticksToCorners":
		h.play.scriptThumbStickX = [2]float64{64, 416}
		h.play.scriptThumbStickY = [2]float64{256, 256}
		return scripting.CallResult{}, nil
	case "SetThumbStickCentre":
		index, err := scriptID(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		if index < 0 || index >= len(h.play.scriptThumbStickX) {
			return scripting.CallResult{}, fmt.Errorf("thumbstick %d is out of range", index)
		}
		h.play.scriptThumbStickX[index], err = scriptNumber(args, 1)
		if err != nil {
			return scripting.CallResult{}, err
		}
		h.play.scriptThumbStickY[index], err = scriptNumber(args, 2)
		return scripting.CallResult{}, err
	case "PickupExists":
		id, err := scriptID(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		if entity := h.findEntity(id); entity != nil && entity.kind == "pickup" {
			return scriptValues(true), nil
		}
		return scriptValues(false), nil
	case "KillZombie":
		id, err := scriptID(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		zombie := h.findZombie(id)
		if zombie == nil {
			if entity := h.findEntity(id); entity != nil && entity.kind == "zombie" {
				delete(h.play.scriptEntities, id)
				return scripting.CallResult{}, nil
			}
			return scripting.CallResult{}, fmt.Errorf("zombie %d not found", id)
		}
		zombie.health = 0
		zombie.dying = true
		zombie.deathAge = 0
		return scripting.CallResult{}, nil
	case "FireGun":
		if len(args) > 1 {
			return scripting.CallResult{}, fmt.Errorf("FireGun secondary behavior is unresolved")
		}
		dx, dy := math.Cos(float64(h.play.angle)*math.Pi/4), math.Sin(float64(h.play.angle)*math.Pi/4)
		if h.play.scriptHasAim {
			dx, dy = h.play.scriptAimX-h.play.x, h.play.scriptAimY-h.play.y
		}
		if h.play.fire(dx, dy) {
			if path := h.app.scriptSoundPath(h.play.weapon.SFXShoot); path != "" {
				h.app.sound.Play(path, .8)
			}
		}
		return scripting.CallResult{}, nil
	case "UpdateCamera":
		h.play.updateCamera()
		return scripting.CallResult{Yield: true}, nil
	case "ZoomCameraOut":
		zoom, err := scriptNumber(args, 0)
		if err != nil {
			return scripting.CallResult{}, err
		}
		x, err := scriptNumber(args, 1)
		if err != nil {
			return scripting.CallResult{}, err
		}
		y, err := scriptNumber(args, 2)
		if err != nil {
			return scripting.CallResult{}, err
		}
		h.play.world.Zoom = zoom
		h.play.setScriptCamera(x, y)
		return scripting.CallResult{}, nil
	case "SpawnZombiesAroundPlayer":
		return scripting.CallResult{}, fmt.Errorf("SpawnZombiesAroundPlayer call shape is unresolved")
	case "TriggerTutorial":
		return scripting.CallResult{}, fmt.Errorf("TriggerTutorial is not present in the active entry flow")
	default:
		return scripting.CallResult{}, fmt.Errorf("unsupported callback")
	}
}

func (h *playScriptHost) spawnZombie(args []scripting.Value) (scripting.CallResult, error) {
	x, err := scriptNumber(args, 0)
	if err != nil {
		return scripting.CallResult{}, err
	}
	y, err := scriptNumber(args, 1)
	if err != nil {
		return scripting.CallResult{}, err
	}
	size, err := scriptNumber(args, 2)
	if err != nil {
		return scripting.CallResult{}, err
	}
	speed := 70.0
	if len(args) > 3 {
		speed, err = scriptNumber(args, 3)
		if err != nil {
			return scripting.CallResult{}, err
		}
	}
	if speed <= 0 {
		speed = 70
	}
	id := h.play.scriptNextEntity
	h.play.scriptNextEntity++
	h.play.scriptEntities[id] = &scriptEntity{id: id, kind: "zombie", entityType: "zombie", x: x, y: y, scaleX: 1, scaleY: 1, alpha: 1, texture: "cavezombie", speed: speed}
	h.play.zombies = append(h.play.zombies, zombieState{x: x, y: y, speed: speed, health: 100, size: formats.Vec2{X: size, Y: size}, texture: "cavezombie", scriptID: id, alpha: 1, fps: h.play.spriteFPS("cavezombie", "")})
	return scriptValues(id), nil
}

func (h *playScriptHost) createEntity(args []scripting.Value) (scripting.CallResult, error) {
	x, err := scriptNumber(args, 0)
	if err != nil {
		return scripting.CallResult{}, err
	}
	y, err := scriptNumber(args, 1)
	if err != nil {
		return scripting.CallResult{}, err
	}
	texture, err := scriptString(args, 2)
	if err != nil {
		return scripting.CallResult{}, err
	}
	id := h.play.scriptNextEntity
	h.play.scriptNextEntity++
	h.play.scriptEntities[id] = &scriptEntity{id: id, kind: "sprite", entityType: texture, x: x, y: y, scaleX: 1, scaleY: 1, alpha: 1, texture: texture, playing: true}
	return scriptValues(id), nil
}

func (h *playScriptHost) spawnEntity(args []scripting.Value) (scripting.CallResult, error) {
	texture, err := scriptString(args, 0)
	if err != nil {
		return scripting.CallResult{}, err
	}
	x, err := scriptNumber(args, 1)
	if err != nil {
		return scripting.CallResult{}, err
	}
	y, err := scriptNumber(args, 2)
	if err != nil {
		return scripting.CallResult{}, err
	}
	id := h.play.scriptNextEntity
	h.play.scriptNextEntity++
	h.play.scriptEntities[id] = &scriptEntity{id: id, kind: "pickup", entityType: texture, x: x, y: y, scaleX: 1, scaleY: 1, alpha: 1, texture: texture}
	return scriptValues(id), nil
}

func (h *playScriptHost) findEntity(id int) *scriptEntity {
	return h.play.scriptEntities[id]
}

func (h *playScriptHost) loadScriptLevel(name string) error {
	level, err := h.app.pack.LoadScriptLevel(name, h.play.world.Level.Info.WorldIndex)
	if err != nil {
		return err
	}
	tileset, ok := h.app.pack.Manifest().TileSets[strings.ToLower(level.Tileset)]
	if !ok {
		return fmt.Errorf("tileset %q not found", level.Tileset)
	}
	atlas, err := h.app.Texture(tileset.Texture)
	if err != nil {
		return err
	}
	zoom := h.play.world.Zoom
	world := viewer.New(level, tileset, atlas, h.app)
	world.Zoom = zoom
	world.Layers[formats.LayerH] = true
	h.play.world = world
	h.play.tileSize = tileSizeFor(tileset)
	h.play.zombies = nil
	h.play.portals = nil
	h.play.bloodPops = nil
	h.play.waveIndex = 0
	h.play.waveElapsed = 0
	h.play.waveSpawned = nil
	h.play.scriptEntities = map[int]*scriptEntity{}
	h.play.scriptNextEntity = 1
	return nil
}

func (h *playScriptHost) findZombie(id int) *zombieState {
	for index := range h.play.zombies {
		if h.play.zombies[index].scriptID == id {
			return &h.play.zombies[index]
		}
	}
	return nil
}

func (h *playScriptHost) entityArg(args []scripting.Value) (*scriptEntity, error) {
	id, err := scriptID(args, 0)
	if err != nil {
		return nil, err
	}
	entity := h.findEntity(id)
	if entity == nil {
		return nil, fmt.Errorf("entity %d not found", id)
	}
	return entity, nil
}

func (h *playScriptHost) textureArg(args []scripting.Value) (*scriptTexture, error) {
	id, err := scriptID(args, 0)
	if err != nil {
		return nil, err
	}
	texture := h.play.scriptTextures[id]
	if texture == nil {
		texture = &scriptTexture{id: id, scaleX: 1, scaleY: 1, alpha: 1, cameo: -1}
		h.play.scriptTextures[id] = texture
	}
	return texture, nil
}

func (h *playScriptHost) setEntityPosition(name string, args []scripting.Value) (scripting.CallResult, error) {
	entity, err := h.entityArg(args)
	if err != nil {
		return scripting.CallResult{}, err
	}
	value, err := scriptNumber(args, 1)
	if err != nil {
		return scripting.CallResult{}, err
	}
	switch name {
	case "SetSpriteXPosition":
		entity.x = value
	case "SetSpriteYPosition":
		entity.y = value
	default:
		entity.x = value
		entity.y, err = scriptNumber(args, 2)
	}
	if entity.kind == "zombie" {
		if zombie := h.findZombie(entity.id); zombie != nil {
			zombie.x, zombie.y = entity.x, entity.y
		}
	}
	return scripting.CallResult{}, err
}

func (h *playScriptHost) entityProperty(name string, args []scripting.Value) (scripting.CallResult, error) {
	entity, err := h.entityArg(args)
	if err != nil {
		return scripting.CallResult{}, err
	}
	switch name {
	case "SetAnimation":
		entity.animation, err = scriptID(args, 1)
		entity.playing = true
		entity.frame, entity.frameTime = 0, 0
	case "SetFrame":
		entity.frame, err = scriptID(args, 1)
		entity.frameTime = 0
	case "StopAnimation":
		entity.walking, entity.playing = false, false
	case "SetEntityColour":
		if len(args) < 4 {
			return scripting.CallResult{}, fmt.Errorf("SetEntityColour needs four values")
		}
	}
	return scripting.CallResult{}, err
}

func (h *playScriptHost) zombieProperty(name string, args []scripting.Value) (scripting.CallResult, error) {
	id, err := scriptID(args, 0)
	if err != nil {
		return scripting.CallResult{}, err
	}
	zombie := h.findZombie(id)
	if zombie == nil {
		return scripting.CallResult{}, fmt.Errorf("zombie %d not found", id)
	}
	switch name {
	case "GetZombieSpeed":
		return scriptValues(zombie.speed), nil
	case "SetZombieSpeed":
		zombie.speed, err = scriptNumber(args, 1)
	case "SetZombieAlpha":
		zombie.alpha, err = scriptAlpha(args, 1)
	case "SetZombieAnimTime":
		zombie.frame, err = scriptNumber(args, 1)
	case "SetZombieTexture":
		zombie.texture, err = scriptString(args, 1)
	}
	return scripting.CallResult{}, err
}

func (h *playScriptHost) drawScriptText(args []scripting.Value, second bool) (scripting.CallResult, error) {
	x, err := scriptNumber(args, 0)
	if err != nil {
		return scripting.CallResult{}, err
	}
	y, err := scriptNumber(args, 1)
	if err != nil {
		return scripting.CallResult{}, err
	}
	text, err := scriptString(args, 2)
	if err != nil {
		return scripting.CallResult{}, err
	}
	if second {
		h.play.scriptText2, h.play.scriptText2X, h.play.scriptText2Y = text, x, y
	} else {
		h.play.scriptText1, h.play.scriptText1X, h.play.scriptText1Y = text, x, y
	}
	h.play.scriptTextVisible = true
	return scripting.CallResult{}, nil
}

func (p *playState) updateScript() error {
	if p.scriptRuntime == nil {
		return nil
	}
	if p.scriptRuntime.Done() {
		return p.scriptRuntime.Err()
	}
	const dt = 1000.0 / 60.0
	if p.scriptWaitActive {
		p.scriptWaitRemaining = math.Max(0, p.scriptWaitRemaining-dt)
	}
	if p.scriptFadeRemaining > 0 {
		p.scriptFadeRemaining = math.Max(0, p.scriptFadeRemaining-dt)
		if p.scriptFadeRemaining == 0 && p.scriptFadeBlack {
			p.scriptFadeDuration = 0
		}
	}
	p.updateScriptWalk()
	p.updatePickups()
	p.updateScriptEntities()
	return p.scriptRuntime.Step()
}

func (p *playState) closeScript() {
	if p == nil || p.scriptRuntime == nil {
		return
	}
	p.scriptRuntime.Close()
	p.scriptRuntime = nil
}

func (p *playState) updateScriptWalk() {
	if !p.scriptWalking {
		return
	}
	dx, dy := p.scriptWalkX-p.x, p.scriptWalkY-p.y
	distance := math.Hypot(dx, dy)
	step := playerBaseSpeed / 60
	if distance <= step {
		p.x, p.y, p.scriptWalking = p.scriptWalkX, p.scriptWalkY, false
		return
	}
	candidateX := p.x + dx/distance*step
	candidateY := p.y + dy/distance*step
	for resolve := 0; resolve < 4; resolve++ {
		pushX, pushY, hit := p.collisionDisplacement(candidateX, candidateY, p.radius, p.tileSize)
		if !hit {
			break
		}
		candidateX += pushX
		candidateY += pushY
	}
	p.x, p.y = candidateX, candidateY
	p.angle, p.flipX = barryDirection(dx, dy)
}

func (p *playState) updatePickups() {
	for id, entity := range p.scriptEntities {
		if entity == nil || entity.kind != "pickup" || math.Hypot(p.x-entity.x, p.y-entity.y) > playerCollisionRadius+12 {
			continue
		}
		p.collectPickup(entity.texture)
		delete(p.scriptEntities, id)
	}
}

func (p *playState) updateScriptEntities() {
	for _, entity := range p.scriptEntities {
		if entity == nil || !entity.playing || entity.kind == "pickup" {
			continue
		}
		fps, frames := 8.0, 4
		if animation, ok := findSpriteAnimationByIndex(p.sprites, entity.texture, entity.animation); ok {
			if animation.FPS > 0 {
				fps = animation.FPS
			}
			if animation.Frames > 0 {
				frames = animation.Frames
			}
		}
		entity.frameTime += 1.0 / 60.0
		for entity.frameTime >= 1.0/fps {
			entity.frameTime -= 1.0 / fps
			entity.frame = (entity.frame + 1) % frames
		}
	}
}

func (p *playState) spawnPickup(name string, point formats.Vec2) {
	if p.scriptEntities == nil {
		p.scriptEntities = map[int]*scriptEntity{}
	}
	if p.scriptNextEntity <= 0 {
		p.scriptNextEntity = 1
	}
	id := p.scriptNextEntity
	p.scriptNextEntity++
	p.scriptEntities[id] = &scriptEntity{id: id, kind: "pickup", entityType: name, x: point.X, y: point.Y, scaleX: 1, scaleY: 1, alpha: 1, texture: name}
}

func (p *playState) collectPickup(name string) {
	name = strings.ToUpper(strings.TrimSpace(name))
	switch name {
	case "P_GRENADE":
		p.grenades++
	case "P_HEALTH":
		p.health = p.maxHealth
	default:
		if strings.HasPrefix(name, "P_") {
			if weapon, ok := p.weapons.Find(strings.TrimPrefix(name, "P_")); ok {
				p.weapon = weapon
			}
		}
	}
}

func (p *playState) setScriptCamera(x, y float64) {
	zoom := p.world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	worldWidth := float64(p.world.Level.Width * p.tileSize)
	worldHeight := float64(p.world.Level.Height * p.tileSize)
	maxX := math.Max(0, worldWidth-float64(logicalWidth)/zoom)
	maxY := math.Max(0, worldHeight-float64(logicalHeight)/zoom)
	p.world.CameraX = math.Max(0, math.Min(maxX, x-float64(logicalWidth)/(2*zoom)))
	p.world.CameraY = math.Max(0, math.Min(maxY, y-float64(logicalHeight)/(2*zoom)))
}

func (p *playState) scriptCameraCenterX() float64 {
	zoom := p.world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	return p.world.CameraX + float64(logicalWidth)/(2*zoom)
}

func (p *playState) scriptCameraCenterY() float64 {
	zoom := p.world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	return p.world.CameraY + float64(logicalHeight)/(2*zoom)
}

func (p *playState) setScriptFacing(degrees float64) {
	p.angle, p.flipX = barryDirection(math.Cos(degrees*math.Pi/180), math.Sin(degrees*math.Pi/180))
}

func (a *app) drawScriptEntities(screen *ebiten.Image, behind bool) {
	if a.play == nil || len(a.play.scriptEntities) == 0 {
		return
	}
	ids := make([]int, 0, len(a.play.scriptEntities))
	for id := range a.play.scriptEntities {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		entity := a.play.scriptEntities[id]
		if entity == nil || entity.kind == "zombie" || behind != (entity.y <= a.play.y) {
			continue
		}
		a.drawScriptEntity(screen, entity)
	}
}

func (a *app) spriteAnimation(name, preferred string) (formats.SpriteAnimation, bool) {
	return findSpriteAnimation(a.sprites, name, preferred)
}

func (a *app) spriteAnimationByIndex(name string, index int) (formats.SpriteAnimation, bool) {
	return findSpriteAnimationByIndex(a.sprites, name, index)
}

func findSpriteAnimation(catalog formats.SpriteCatalog, name, preferred string) (formats.SpriteAnimation, bool) {
	candidates := []string{name}
	if !strings.Contains(name, "/") {
		candidates = append(candidates, "Characters/"+name)
	}
	for _, candidate := range candidates {
		definition, ok := catalog.Find(candidate)
		if !ok {
			continue
		}
		if preferred != "" {
			if animation, ok := definition.Animation(preferred); ok {
				return animation, true
			}
		}
		for _, fallback := range []string{"Idle", "Run", "Death"} {
			if animation, ok := definition.Animation(fallback); ok {
				return animation, true
			}
		}
		keys := make([]string, 0, len(definition.Animations))
		for key := range definition.Animations {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if len(keys) > 0 {
			return definition.Animations[keys[0]], true
		}
	}
	return formats.SpriteAnimation{}, false
}

func findSpriteAnimationByIndex(catalog formats.SpriteCatalog, name string, index int) (formats.SpriteAnimation, bool) {
	candidates := []string{name}
	if !strings.Contains(name, "/") {
		candidates = append(candidates, "Characters/"+name)
	}
	for _, candidate := range candidates {
		definition, ok := catalog.Find(candidate)
		if !ok {
			continue
		}
		if animation, ok := definition.AnimationByIndex(index); ok {
			return animation, true
		}
	}
	return findSpriteAnimation(catalog, name, "")
}

func (p *playState) spriteFPS(name, preferred string) float64 {
	if animation, ok := findSpriteAnimation(p.sprites, name, preferred); ok && animation.FPS > 0 {
		return animation.FPS
	}
	return 8
}

func (a *app) drawScriptEntity(screen *ebiten.Image, entity *scriptEntity) {
	animation, hasAnimation := a.spriteAnimationByIndex(entity.texture, entity.animation)
	texturePath := commonSDTexture(entity.texture)
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
	col, frame := entity.angle, entity.frame
	if entity.kind == "pickup" {
		columns, rows, col, frame = 1, 1, 0, 0
	}
	if col < 0 {
		col = 0
	} else if col >= columns {
		col = columns - 1
	}
	if frame < 0 {
		frame = 0
	} else if frame >= rows {
		frame = rows - 1
	}
	rect := barryCellRect(col, frame, columns, rows, texture.Bounds().Dx(), texture.Bounds().Dy())
	if rect.Dx() <= 0 || rect.Dy() <= 0 {
		return
	}
	zoom := a.play.world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	screenX := (entity.x-a.play.world.CameraX)*zoom + a.play.world.ViewportX
	screenY := (entity.y-a.play.world.CameraY)*zoom + a.play.world.ViewportY
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(rect.Dx())/2, -float64(rect.Dy())/2)
	scaleX, scaleY := entity.scaleX*zoom, entity.scaleY*zoom
	if scaleX == 0 {
		scaleX = zoom
	}
	if scaleY == 0 {
		scaleY = zoom
	}
	if entity.flipY {
		scaleY = -scaleY
	}
	options.GeoM.Scale(scaleX, scaleY)
	if entity.alpha < 1 {
		options.ColorScale.ScaleAlpha(float32(math.Max(0, entity.alpha)))
	}
	options.GeoM.Translate(screenX, screenY)
	a.drawImage(screen, texture.SubImage(rect).(*ebiten.Image), options)
}

func (a *app) drawScriptTextures(screen *ebiten.Image) {
	if a.play == nil {
		return
	}
	ids := make([]int, 0, len(a.play.scriptTextures))
	for id := range a.play.scriptTextures {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		textureState := a.play.scriptTextures[id]
		if textureState == nil || !textureState.visible || (textureState.cameo >= 0 && !a.play.scriptCameoVisible) {
			continue
		}
		texture, err := a.Texture(commonSDTexture(textureState.name))
		if err != nil {
			continue
		}
		source := texture
		if textureState.u2 > textureState.u1 && textureState.v2 > textureState.v1 {
			bounds := texture.Bounds()
			x0 := int(math.Round(textureState.u1 * float64(bounds.Dx())))
			y0 := int(math.Round(textureState.v1 * float64(bounds.Dy())))
			x1 := int(math.Round(textureState.u2 * float64(bounds.Dx())))
			y1 := int(math.Round(textureState.v2 * float64(bounds.Dy())))
			x0, y0 = max(0, x0), max(0, y0)
			x1, y1 = min(bounds.Dx(), x1), min(bounds.Dy(), y1)
			if x1 > x0 && y1 > y0 {
				source = texture.SubImage(image.Rect(x0, y0, x1, y1)).(*ebiten.Image)
			}
		}
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		bounds := source.Bounds()
		options.GeoM.Translate(-float64(bounds.Dx())/2, -float64(bounds.Dy())/2)
		scaleX, scaleY := textureState.scaleX, textureState.scaleY
		if scaleX == 0 {
			scaleX = 1
		}
		if scaleY == 0 {
			scaleY = 1
		}
		options.GeoM.Scale(scaleX, scaleY)
		if textureState.alpha < 1 {
			options.ColorScale.ScaleAlpha(float32(math.Max(0, textureState.alpha)))
		}
		options.GeoM.Translate(textureState.x, textureState.y)
		a.drawImage(screen, source, options)
	}
}

func (a *app) drawScriptText(screen *ebiten.Image) {
	if a.play == nil || !a.play.scriptTextVisible {
		return
	}
	for _, item := range []struct {
		value string
		x, y  float64
	}{
		{a.play.scriptText1, a.play.scriptText1X, a.play.scriptText1Y},
		{a.play.scriptText2, a.play.scriptText2X, a.play.scriptText2Y},
	} {
		if item.value == "" {
			continue
		}
		scale := .5
		a.text(screen, item.value, item.x-a.fontTextWidth(item.value, scale)/2, item.y, scale)
	}
}

func (a *app) drawScriptFade(screen *ebiten.Image) {
	if a.play == nil {
		return
	}
	if a.play.scriptFadeDuration <= 0 {
		if a.play.scriptFadeBlack {
			a.drawRect(screen, 0, 0, logicalWidth, logicalHeight, color.Black)
		}
		return
	}
	if a.play.scriptFadeRemaining <= 0 {
		return
	}
	progress := a.play.scriptFadeRemaining / a.play.scriptFadeDuration
	alpha := progress
	if a.play.scriptFadeBlack {
		alpha = 1 - progress
	}
	a.drawRect(screen, 0, 0, logicalWidth, logicalHeight, color.RGBA{A: uint8(math.Max(0, math.Min(1, alpha)) * 255)})
}

func scriptValues(values ...scripting.Value) scripting.CallResult {
	return scripting.CallResult{Values: values}
}

func scriptID(args []scripting.Value, index int) (int, error) {
	value, err := scriptNumber(args, index)
	if err != nil {
		return 0, err
	}
	return int(math.Round(value)), nil
}

func scriptNumber(args []scripting.Value, index int) (float64, error) {
	if index < 0 || index >= len(args) {
		return 0, fmt.Errorf("missing argument %d", index+1)
	}
	switch value := args[index].(type) {
	case float64:
		return value, nil
	case float32:
		return float64(value), nil
	case int:
		return float64(value), nil
	case int64:
		return float64(value), nil
	case uint:
		return float64(value), nil
	case uint64:
		return float64(value), nil
	default:
		return 0, fmt.Errorf("argument %d is not a number", index+1)
	}
}

func scriptString(args []scripting.Value, index int) (string, error) {
	if index < 0 || index >= len(args) {
		return "", fmt.Errorf("missing argument %d", index+1)
	}
	value, ok := args[index].(string)
	if !ok {
		return "", fmt.Errorf("argument %d is not a string", index+1)
	}
	return value, nil
}

func scriptBool(args []scripting.Value, index int) (bool, error) {
	if index < 0 || index >= len(args) {
		return false, fmt.Errorf("missing argument %d", index+1)
	}
	switch value := args[index].(type) {
	case bool:
		return value, nil
	case float64:
		return value != 0, nil
	default:
		return false, fmt.Errorf("argument %d is not boolean", index+1)
	}
}

func scriptAlpha(args []scripting.Value, index int) (float64, error) {
	value, err := scriptNumber(args, index)
	if err != nil {
		return 0, err
	}
	if value > 1 {
		value /= 255
	}
	return math.Max(0, math.Min(1, value)), nil
}

func (a *app) scriptSoundPath(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "0" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(name), "audio/") {
		return filepath.ToSlash(name)
	}
	wanted := strings.ToLower(strings.TrimSuffix(filepath.Base(filepath.ToSlash(name)), filepath.Ext(name)))
	wanted = strings.TrimPrefix(wanted, "sfx_")
	for key, path := range a.pack.Manifest().Files {
		if !strings.EqualFold(filepath.Ext(key), ".ogg") {
			continue
		}
		base := strings.ToLower(strings.TrimSuffix(filepath.Base(filepath.ToSlash(key)), filepath.Ext(key)))
		base = strings.TrimPrefix(base, "sfx_")
		if base == wanted {
			return path
		}
	}
	return filepath.ToSlash(filepath.Join("audio", "sound", "sfx", name+".ogg"))
}
