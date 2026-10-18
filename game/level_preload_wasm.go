//go:build js && wasm

package game

import (
	"image"
	"time"
)

// The browser prefetches over HTTP instead (level_prefetch.go).
type levelPreload struct{}

func (a *app) preloadSelectedLevel()                  {}
func (a *app) takePredecoded(path string) image.Image { return nil }
func (a *app) stepPreload(budget time.Duration)       {}
