//go:build !js

package game

import (
	"fmt"
	"image/color"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/game/platform"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// The desktop program opens its window first. While the game's files are made ready from the APK (the first launch, or
// after a new APK) the window shows what is happening; only then is the game itself built and handed the window.
// F2 picks a different APK at any point (switchAPK).

type launchOptions struct {
	assets                string
	explicit, chooseAPK   bool
	debug, mobile, silent bool
}

type launcher struct {
	options launchOptions
	setup   func(*app) error // what Run does to the app once it exists

	mu          sync.Mutex
	stage       string
	done, total int

	frames   int
	root     string
	phase    int // 0 choosing, 1 preparing, 2 building, 3 running
	prepared chan error
	game     *app
	buildErr error
}

// newLauncher is the game the window runs on the desktop, and the function that closes it down afterwards.
func newLauncher(options launchOptions, setup func(*app) error) (ebiten.Game, func()) {
	l := &launcher{options: options, setup: setup, stage: "Starting"}
	return l, l.finish
}

func (l *launcher) progress(stage string, done, total int) {
	l.mu.Lock()
	l.stage, l.done, l.total = stage, done, total
	l.mu.Unlock()
}

func (l *launcher) Layout(w, h int) (int, int) {
	if l.game != nil {
		return l.game.Layout(w, h)
	}
	if w < 1 || h < 1 {
		return logicalWidth, logicalHeight
	}
	return w, h
}

func (l *launcher) Update() error {
	if l.game != nil {
		return l.game.Update()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF2) && l.phase != 0 {
		return switchAPK()
	}
	switch l.phase {
	case 0:
		l.frames++
		if l.frames < 3 { // let the window appear before a dialog or any work
			return nil
		}
		root, err := resolveAssets(l.options.assets, l.options.explicit, l.options.chooseAPK, false)
		if err != nil {
			fatal(err)
		}
		l.root = root
		l.progress("Looking at the game files", 0, 0)
		content.ImportStarted = func() { l.progress("Preparing the game files from your APK", 0, 0) }
		content.ImportFinished = nil
		content.ImportProgress = l.progress
		l.prepared = make(chan error, 1)
		go func() {
			_, err := content.PrepareAssets(root)
			l.prepared <- err
		}()
		l.phase = 1
	case 1:
		select {
		case err := <-l.prepared:
			if err != nil {
				fatal(err)
			}
			l.progress("Starting the game", 0, 0)
			l.phase = 2
		default:
		}
	case 2:
		// One frame has shown "Starting the game"; building the app takes a moment.
		game, err := newApp(l.root, l.options.debug, l.options.mobile, l.options.silent)
		if err != nil {
			fatal(err)
		}
		if err := l.setup(game); err != nil {
			fatal(err)
		}
		l.game = game
		l.phase = 3
	}
	return nil
}

func (l *launcher) Draw(screen *ebiten.Image) {
	if l.game != nil {
		l.game.Draw(screen)
		return
	}
	w, h := float64(screen.Bounds().Dx()), float64(screen.Bounds().Dy())
	screen.Fill(color.RGBA{11, 14, 22, 255})
	unit := h / 540
	l.mu.Lock()
	stage, done, total := l.stage, l.done, l.total
	l.mu.Unlock()
	drawBootText(screen, "Age of Zombies", w/2, h*.36, 34*unit, true, color.RGBA{232, 235, 244, 255})
	line := stage
	if total > 0 {
		line = fmt.Sprintf("%s (%d of %d)", stage, done, total)
	}
	drawBootText(screen, line, w/2, h*.5, 18*unit, false, color.RGBA{200, 205, 220, 255})
	barW, barH := 420*unit, 10*unit
	x, y := w/2-barW/2, h*.5+34*unit
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(barW), float32(barH), color.RGBA{31, 36, 51, 255}, false)
	if total > 0 {
		vector.DrawFilledRect(screen, float32(x), float32(y), float32(barW*float64(done)/float64(total)), float32(barH), color.RGBA{197, 61, 43, 255}, false)
	} else {
		// Nothing countable yet: a bar that sweeps.
		sweep := float64(l.frames%90) / 90
		vector.DrawFilledRect(screen, float32(x+barW*sweep*.8), float32(y), float32(barW*.2), float32(barH), color.RGBA{197, 61, 43, 255}, false)
		l.frames++
	}
	drawBootText(screen, "This happens the first time, or after you choose a different APK.", w/2, y+barH+30*unit, 14*unit, false, color.RGBA{141, 148, 168, 255})
	drawBootText(screen, "F2: choose a different APK", w/2, h-28*unit, 14*unit, false, color.RGBA{141, 148, 168, 255})
}

// drawBootText draws s centred on (cx, cy) in device pixels with the Go font.
func drawBootText(screen *ebiten.Image, s string, cx, cy, size float64, bold bool, c color.Color) {
	source := nativeFaceSource(bold)
	if source == nil || s == "" || size <= 0 {
		return
	}
	face := &text.GoTextFace{Source: source, Size: size}
	width, height := text.Measure(s, face, 0)
	options := &text.DrawOptions{}
	options.GeoM.Translate(cx-width/2, cy-height/2)
	options.ColorScale.ScaleWithColor(c)
	text.Draw(screen, s, face, options)
}

// finish is what Run does after the loop: close the sound and save the profile of the game that ran.
func (l *launcher) finish() {
	if l.game == nil {
		return
	}
	if l.game.sound != nil {
		l.game.sound.Close()
	}
	if err := l.game.savePlayerProfile(); err != nil {
		log.Printf("profile: %v", err)
	}
}

// switchAPK lets the player choose another APK (F2): the dialog, a check, the new location remembered, and the program
// started again so the files are made from it. It ends the running program when it did restart it.
func switchAPK() error {
	apk, err := platform.PickAPK()
	if err != nil {
		platform.ShowError("HalfBricked", err.Error())
		return nil
	}
	if apk == "" {
		return nil // cancelled
	}
	if err := content.ValidateAPK(apk); err != nil {
		platform.ShowError("HalfBricked", err.Error())
		return nil
	}
	if err := platform.RememberAPK(apk); err != nil {
		log.Printf("could not remember the APK location: %v", err)
	}
	if err := platform.StartSelf(relaunchArgs(os.Args[1:])); err != nil {
		platform.ShowError("HalfBricked", "The APK is remembered, but the game could not restart itself: "+err.Error()+"\nStart it again.")
	}
	return ebiten.Termination
}

// relaunchArgs are the program's own arguments without the ones that would override the new APK.
func relaunchArgs(args []string) []string {
	var kept []string
	for i := 0; i < len(args); i++ {
		arg := strings.TrimLeft(args[i], "-")
		switch {
		case arg == "choose-apk" || strings.HasPrefix(arg, "choose-apk="):
		case arg == "assets":
			i++ // its value
		case strings.HasPrefix(arg, "assets="):
		default:
			kept = append(kept, args[i])
		}
	}
	return kept
}
