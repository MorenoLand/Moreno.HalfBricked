package game

import (
	"math"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
)

// 1.2.5 (HD) title screen UI layer: Common0/UserInterface/screens/SplashScreen.uiscreen holds the Age of Zombies
// logo (group "AoZLogo") and Abe Steakfries (group "Abe", texture AbeSteakfries). The title class (init 0xe6ea8,
// update 0xe7310) looks the screen up by name ("SplashScreen" at 0x85a637), shows it with SetVisible(1)
// (FUN_003358dc) and plays "AnimateIn" when the 2.0 s splash ends (0xe73dc-0xe7444, state 2), and plays
// "AnimateOut" on it at the click (0xe7520-0xe7574, state 3). The class exit (0xe6bfc) removes the screen. The
// title draw (0xe7bd8) contains no UI draw call, so the screen is drawn by the UI manager outside it; it is
// drawn here after the Barry quads (draw order against them is not proven, see Research notes).
//
// AnimateOut tracks decoded from the file (see titleSplashAbeKeys and the logo constants below):
//   - Abe: position (354.8,175) held until 0.417 s, then linear to (574.8,175) at 0.817 s (slides off to the right).
//   - AoZLogo: position (249,91) (the group rests at (245,93)), rotation z = 2 (single key at t=0), colour 255 ->
//     alpha 0 over 0.3 s.
//
// AbeSteakfries_Hat.png is shipped but nothing in the 1.2.5 binary or any uiscreen references it.
//
// UIModifierSineWave update (0x59204c): t += dt; value = min + (max-min) * sinf(fmod(t*frequency, 2*pi)), written
// to the modifier's output; how the output is applied to the owner (virtual 0x198 on the parent) was not traced,
// so it is applied here as a position offset along the modifier's axis ("y" when unset)  (port assumption).
const (
	titleSplashScreenName = "SplashScreen"
	titleLogoOutDX        = 4.0 // AnimateOut logo position (249,91) minus the resting group position (245,93)
	titleLogoOutDY        = -2.0
	titleLogoOutRotation  = 2.0 // degrees, z of the single rotation key
	titleLogoFadeSeconds  = 0.3
)

// drawTitleSplashUI draws the SplashScreen layer; false when the pack has no such screen (SD cache), in which
// case the caller keeps its previous layout.
func (a *app) drawTitleSplashUI(screen *ebiten.Image) bool {
	if !a.titleNative() {
		return false
	}
	ui := a.nativeScreen(titleSplashScreenName)
	if ui == nil || ui.Find("Abe") == nil || ui.Find("AoZLogo") == nil {
		return false
	}
	elapsed := -1.0 // seconds since the click (AnimateOut not started)
	if a.titleCocking {
		elapsed = a.titleSoundElapsed
	}
	// Logo.
	logo := ui.Find("AoZLogo")
	dx, dy, rotation, alpha := 0.0, 0.0, 0.0, 1.0
	if elapsed >= 0 {
		dx, dy, rotation = titleLogoOutDX, titleLogoOutDY, titleLogoOutRotation
		alpha = math.Max(0, 1-elapsed/titleLogoFadeSeconds)
	}
	gx, gy := logo.Anchor()
	for _, child := range logo.Children {
		if child.Texture() != "" {
			a.drawTitleSplashTexture(screen, child, gx+dx, gy+dy, rotation, alpha, 0, 0)
		}
	}
	// Abe: AnimateOut position track plus the two sine-wave modifiers of his texture.
	abe := ui.Find("Abe")
	ox, oy := 0.0, 0.0
	if elapsed >= 0 {
		progress := 1.0
		if end := titleAnimEnd(abe); end > 0 {
			progress = math.Min(1, elapsed/end)
		}
		if x, y, ok := titleAnimatePosition(abe, progress); ok {
			baseX, baseY := abe.Position()
			ox, oy = x-baseX, y-baseY
		}
	}
	ax, ay := abe.Anchor()
	sx, sy := titleSineOffsets(abe, a.menuTime) // the modifiers are children of the Abe group (ids 3 and 4 under 1)
	for _, child := range abe.Children {
		if child.Texture() != "" {
			a.drawTitleSplashTexture(screen, child, ax+ox, ay+oy, 0, 1, sx, sy)
		}
	}
	return true
}

// titleSineOffsets sums the UIModifierSineWave children of the group c per axis.
func titleSineOffsets(c *formats.UIComponent, t float64) (x, y float64) {
	for _, modifier := range c.Children {
		if modifier.Class != "UIModifierSineWave" {
			continue
		}
		frequency, _ := modifier.Float("frequency", 0)
		low, _ := modifier.Float("minAmplitude", 0)
		high, _ := modifier.Float("maxAmplitude", 0)
		value := low + (high-low)*math.Sin(math.Mod(t*frequency, 2*math.Pi))
		if modifier.String("axis") == "x" {
			x += value
		} else {
			y += value
		}
	}
	return
}

// titleAnimEnd is the time of the last key of the AnimateOut position track.
func titleAnimEnd(c *formats.UIComponent) float64 {
	keys := c.AnimationPosition("AnimateOut")
	if len(keys) < 2 {
		return 0
	}
	return keys[len(keys)-1].Time
}

// titleAnimatePosition samples the AnimateOut position track at progress (0..1 of its last key time).
func titleAnimatePosition(c *formats.UIComponent, progress float64) (x, y float64, ok bool) {
	keys := c.AnimationPosition("AnimateOut")
	if len(keys) == 0 {
		return 0, 0, false
	}
	t := progress * keys[len(keys)-1].Time
	if t <= keys[0].Time {
		return keys[0].X, keys[0].Y, true
	}
	for i := 0; i+1 < len(keys); i++ {
		k0, k1 := keys[i], keys[i+1]
		if t <= k1.Time {
			f := (t - k0.Time) / (k1.Time - k0.Time)
			return k0.X + (k1.X-k0.X)*f, k0.Y + (k1.Y-k0.Y)*f, true
		}
	}
	last := keys[len(keys)-1]
	return last.X, last.Y, true
}

// drawTitleSplashTexture draws a ComponentTexture of the SplashScreen: the screen's Frontend0 art, the node's
// size, centred on its anchor (the parent group at (gx,gy) after the animation, plus the node's own offset),
// turned by `degrees` (counter-clockwise, rotating the node about its parent's anchor) with alpha.
func (a *app) drawTitleSplashTexture(screen *ebiten.Image, node *formats.UIComponent, gx, gy, degrees, alpha, offX, offY float64) {
	name := "Frontend0/Textures/Ageofzombies"
	if strings.Contains(node.Texture(), "Abe") {
		name = "Frontend0/Textures/AbeSteakfries"
	}
	texture, err := a.Texture(name)
	if err != nil {
		return
	}
	width, okW := node.Width()
	height, okH := node.Height()
	bounds := texture.Bounds()
	if !okW || !okH {
		width, height = float64(bounds.Dx()), float64(bounds.Dy())
	}
	localX, localY := node.Position()
	radians := -degrees * math.Pi / 180 // rotation.z is counter-clockwise on the y-down screen (native_ui.go)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(bounds.Dx())/2, -float64(bounds.Dy())/2)
	options.GeoM.Scale(width/float64(bounds.Dx()), height/float64(bounds.Dy()))
	options.GeoM.Translate(localX, localY) // child centre relative to the group anchor
	options.GeoM.Rotate(radians)
	options.GeoM.Translate(gx+offX, gy+offY)
	options.ColorScale.ScaleAlpha(float32(alpha))
	a.drawImage(screen, texture, options)
}
