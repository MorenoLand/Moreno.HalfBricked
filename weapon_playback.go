package main

import "github.com/hajimehoshi/ebiten/v2/audio"

type weaponPlayback struct {
	player *audio.Player
	sound  string
}

func (a *app) stopWeaponPlayback() {
	if a.weaponPlayback.player != nil {
		_ = a.weaponPlayback.player.Close()
	}
	a.weaponPlayback = weaponPlayback{}
}
func (a *app) playWeaponSound(symbol string) {
	if a.silent || !a.options.sound || a.sound == nil {
		return
	}
	point := nativeWeaponSoundLoopSamples(symbol)
	if point < 0 && symbol != "SFX_FLAMETHROWER" {
		a.playSound(a.scriptSoundPath(symbol), .8)
		return
	}
	if a.weaponPlayback.sound == symbol && a.weaponPlayback.player != nil && (symbol == "SFX_FLAMETHROWER" || a.weaponPlayback.player.IsPlaying()) {
		return
	}
	a.stopWeaponPlayback()
	player, err := a.sound.PlayTracked(a.scriptSoundPath(symbol), .8, point)
	if err == nil {
		a.weaponPlayback = weaponPlayback{player: player, sound: symbol}
	}
}
func (a *app) flushPickupVoices(p *playState) {
	for _, sound := range p.pickupVoices {
		a.playSound(sound, .8)
	}
	p.pickupVoices = nil
}
