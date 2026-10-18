//go:build !js

package game

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/game/platform"
)

// resolveAssets decides where the game's content comes from at launch (desktop). The game ships without its assets:
// they are read from the player's own Age of Zombies APK, whose location is asked once and remembered
// (platform.RememberedAPK).
//
//  1. --assets was given: use it as before.
//  2. A remembered APK that still exists: use it (the cache built from it is reused while its hash is unchanged).
//  3. A ready cache already at bin/data-cache (a developer checkout): use it, no question.
//  4. A capture run never asks (it must not block waiting on a dialog).
//  5. Otherwise ask for the APK with the system file dialog, check it, remember it and use it.
//
// --choose-apk forces the question again.
func resolveAssets(value string, explicit, choose, capture bool) (string, error) {
	if explicit && !choose {
		return value, nil
	}
	if !choose {
		if apk := platform.RememberedAPK(); apk != "" {
			return apk, nil
		}
		if _, err := os.Stat(filepath.Join("bin", "data-cache", "pack.json")); err == nil || capture {
			return value, nil
		}
	}
	for {
		apk, err := platform.PickAPK()
		if err != nil {
			return "", fmt.Errorf("choose the Age of Zombies APK: %w", err)
		}
		if apk == "" {
			return "", errors.New("no APK chosen: HalfBricked needs your Age of Zombies APK (run again, or pass --assets <apk>)")
		}
		if err := content.ValidateAPK(apk); err != nil {
			log.Print(err)
			continue
		}
		if err := platform.RememberAPK(apk); err != nil {
			log.Printf("could not remember the APK location: %v", err)
		}
		return apk, nil
	}
}
