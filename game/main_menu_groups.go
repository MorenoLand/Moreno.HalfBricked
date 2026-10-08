package game

import (
	"fmt"
	"image"
	"math"
	"regexp"
	"strconv"
	"sync"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/menumotion"
	"github.com/hajimehoshi/ebiten/v2"
)

// The 1.2.5 main menu zombies are authored groups in MainScreen.uiscreen: a
// 131x151 centred group (ZombieContainer child) holding ZombieBack, Screen,
// LeftHand, RightHand, Button and Text, all centred (originFromCenter) children
// positioned relative to the group centre. The transform order the native
// applies was recovered from the ARM code (1.2.5, image base 0x10000):
//
//   - 0x004108cc (slot 0xbc) builds a component's local matrix with
//     0x0044540c(position, -rotation.z, scale): scale, then rotation about the
//     component origin (the rect is centred on it), then translation, in
//     row-vector form. The angle is NEGATED, so a positive rotation.z turns the
//     component counter-clockwise on the y-down screen (AoZLogo -25 is the
//     clockwise tilt of the title banner).
//   - 0x0040ee34 (slot 0xcc) multiplies the local matrix by the parent's world
//     matrix (0x00447094), so children are placed in the group frame and then
//     scaled/rotated/moved by the group.
//   - 0x00403ff4 (slot 0x1a8, ComponentTexture) fills the quad through
//     0x003240b0: vertex UVs are pos, pos+(size.x,0), pos+size, pos+(0,size.y)
//     with size = texCoordSize/texCoordRange, so a negative texCoordSize mirrors
//     the image in component space, before the rotation.
//
// The motion helper's anchor is the legacy board centre; the group centre is
// the Screen's local offset (1,27) away from it at the group's rest scale.
type menuGroupChild struct {
	x, y, w, h float64
	rotation   float64 // degrees, native sign (positive is counter-clockwise)
	flipX      bool
	flipY      bool
	art        int // Menu_Zombie_<art> number parsed from the texture name
}

type menuGroupLayout struct {
	back, screen, left, right, button, text menuGroupChild
	hands                                   bool
}

var menuZombieArtPattern = regexp.MustCompile(`(?i)Menu_Zombie_(\d+)`)

func menuGroupChildFrom(component *formats.UIComponent) menuGroupChild {
	if component == nil {
		return menuGroupChild{}
	}
	child := menuGroupChild{rotation: component.Rotation()}
	child.x, child.y = component.Position()
	child.w, _ = component.Width()
	child.h, _ = component.Height()
	child.flipX, child.flipY = component.TexCoordFlip()
	if match := menuZombieArtPattern.FindStringSubmatch(component.Texture()); match != nil {
		child.art, _ = strconv.Atoi(match[1])
	}
	return child
}

// menuGroupNames maps the main menu button action to its MainScreen group.
var menuGroupNames = map[int]string{0: "PlayZombie", 2: "OptionsZombie", 3: "StatsZombie", 4: "QuitZombie"}

// parseMenuGroupLayout reads one zombie group out of MainScreen.uiscreen.
func parseMenuGroupLayout(screen *formats.UIScreen, group string) *menuGroupLayout {
	if screen == nil || screen.Find(group) == nil {
		return nil
	}
	layout := &menuGroupLayout{
		back:   menuGroupChildFrom(screen.FindIn(group, "ZombieBack")),
		screen: menuGroupChildFrom(screen.FindIn(group, "Screen")),
		left:   menuGroupChildFrom(screen.FindIn(group, "LeftHand")),
		right:  menuGroupChildFrom(screen.FindIn(group, "RightHand")),
		button: menuGroupChildFrom(screen.FindIn(group, "Button")),
		text:   menuGroupChildFrom(screen.FindIn(group, "Text")),
	}
	if layout.back.w <= 0 || layout.screen.w <= 0 {
		return nil
	}
	layout.hands = layout.left.w > 0 && layout.right.w > 0
	return layout
}

var (
	mainMenuUIMu    sync.Mutex
	mainMenuUICache = map[any]*formats.UIScreen{}
)

// mainMenuLayout returns the native layout of the group for a button action, or
// nil for packs without MainScreen.uiscreen (the older SD cache has no hands and
// keeps the XML-era board layout).
func (a *app) mainMenuLayout(action int) *menuGroupLayout {
	name, ok := menuGroupNames[action]
	if !ok || a.pack == nil {
		return nil
	}
	mainMenuUIMu.Lock()
	screen, loaded := mainMenuUICache[a.pack]
	mainMenuUIMu.Unlock()
	if !loaded {
		screen = a.loadMainMenuUIScreen()
		mainMenuUIMu.Lock()
		mainMenuUICache[a.pack] = screen
		mainMenuUIMu.Unlock()
	}
	return parseMenuGroupLayout(screen, name)
}

func (a *app) loadMainMenuUIScreen() *formats.UIScreen {
	path, ok := a.pack.SourcePath("Common0/UserInterface/screens/MainScreen.uiscreen")
	if !ok {
		return nil
	}
	reader, err := a.pack.Open(path)
	if err != nil {
		return nil
	}
	defer reader.Close()
	data, err := ioReadAll(reader)
	if err != nil {
		return nil
	}
	screen, err := formats.ParseUIScreen(data)
	if err != nil {
		return nil
	}
	return screen
}

// menuGroupOrigin is the group centre at the motion's current orbit position:
// the legacy anchor is the Screen centre, which sits layout.screen away from the
// group centre at the group's rest scale (the position is not affected by the
// group's own scale/rotation, only the children are).
func menuGroupOrigin(layout *menuGroupLayout, motion menumotion.NativeMenuZombieMotion) (float64, float64) {
	return float64(motion.X) - layout.screen.x*float64(motion.BaseScale), float64(motion.Y) - layout.screen.y*float64(motion.BaseScale)
}

// menuGroupChildOptions places a centred child of a menu group. width/height are
// the child's authored size in group units; the image is stretched over it and
// mirrored first when the component's texCoordSize is negative.
func menuGroupChildOptions(layout *menuGroupLayout, motion menumotion.NativeMenuZombieMotion, child menuGroupChild, imageWidth, imageHeight, offsetX, offsetY float64, extraScaleX, extraScaleY float64) *ebiten.DrawImageOptions {
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-offsetX, -offsetY)
	sx, sy := child.w*extraScaleX/imageWidth, child.h*extraScaleY/imageHeight
	if child.flipX {
		sx = -sx
	}
	if child.flipY {
		sy = -sy
	}
	options.GeoM.Scale(sx, sy)
	options.GeoM.Rotate(-child.rotation * math.Pi / 180)
	options.GeoM.Translate(child.x, child.y)
	options.GeoM.Scale(float64(motion.Scale), float64(motion.Scale))
	options.GeoM.Rotate(-float64(motion.RotationDegrees) * math.Pi / 180)
	originX, originY := menuGroupOrigin(layout, motion)
	options.GeoM.Translate(originX, originY)
	return options
}

// menuGroupHit reports whether a point falls inside the group's Button
// component (the native hit rectangle), in the group's transformed frame.
func menuGroupHit(layout *menuGroupLayout, motion menumotion.NativeMenuZombieMotion, x, y float64) bool {
	if !motion.Visible || motion.Scale <= 0 || layout.button.w <= 0 || layout.button.h <= 0 {
		return false
	}
	originX, originY := menuGroupOrigin(layout, motion)
	dx, dy := x-originX, y-originY
	angle := float64(motion.RotationDegrees) * math.Pi / 180 // inverse of the drawn -rotation
	cosine, sine := math.Cos(angle), math.Sin(angle)
	localX := (cosine*dx - sine*dy) / float64(motion.Scale)
	localY := (sine*dx + cosine*dy) / float64(motion.Scale)
	return math.Abs(localX-layout.button.x) <= layout.button.w/2 && math.Abs(localY-layout.button.y) <= layout.button.h/2
}

// drawMarqueeButtonNative draws one zombie group from MainScreen.uiscreen data:
// ZombieBack, Screen (top half of Button_Screen), the two hands, then the label.
// The Screen sits on the group's own rotation; the Text child has rotation 0.
func (a *app) drawMarqueeButtonNative(screen *ebiten.Image, button menuButton, layout *menuGroupLayout, motion menumotion.NativeMenuZombieMotion, selected bool) {
	art := layout.back.art
	if art == 0 {
		art = button.zombie
	}
	if zombie, err := a.Texture(fmt.Sprintf("Frontend0/Textures/menu_zombie_%d_SD", art)); err == nil {
		bounds := zombie.Bounds()
		a.drawImage(screen, zombie, menuGroupChildOptions(layout, motion, layout.back, float64(bounds.Dx()), float64(bounds.Dy()), float64(bounds.Dx())/2, float64(bounds.Dy())/2, 1, 1))
	}
	if board, err := a.Texture("Common0/Textures/Button_Screen"); err == nil {
		// texCoordSize (1,.5): the top 128x64 frame stretched over the 101x67 Screen.
		frame := board.SubImage(image.Rect(0, 0, 128, 64)).(*ebiten.Image)
		extraX, extraY := 1.0, 1.0
		if selected {
			// The 1.2.5 binary never references Button_Screen_Flash and the
			// zombie records carry no hover animation; the lit frame is only the
			// port's selection marker, drawn at the same board size (the flash
			// art is 140x70 around the 128x64 body).
			if flash, flashErr := a.Texture("Common0/Textures/Button_Screen_Flash"); flashErr == nil {
				frames := [...]image.Rectangle{image.Rect(0, 73, 140, 143), image.Rect(0, 145, 140, 215)}
				frame = flash.SubImage(frames[int(a.menuTime*8)%len(frames)]).(*ebiten.Image)
				extraX, extraY = 140.0/128.0, 70.0/64.0
			}
		}
		bounds := frame.Bounds()
		a.drawImage(screen, frame, menuGroupChildOptions(layout, motion, layout.screen, float64(bounds.Dx()), float64(bounds.Dy()), float64(bounds.Dx())/2, float64(bounds.Dy())/2, extraX, extraY))
	}
	if layout.hands {
		if hands, err := a.Texture(fmt.Sprintf("Frontend0/Textures/menu_zombie_%d_Hand_SD", art)); err == nil {
			bounds := hands.Bounds()
			for _, hand := range []menuGroupChild{layout.left, layout.right} {
				a.drawImage(screen, hands, menuGroupChildOptions(layout, motion, hand, float64(bounds.Dx()), float64(bounds.Dy()), float64(bounds.Dx())/2, float64(bounds.Dy())/2, 1, 1))
			}
		}
	}
	labels, err := a.Texture("Common0/Textures/Button_Text_SD")
	textRect, ok := buttonTextRect(button.labelRow)
	if err != nil || !ok {
		return
	}
	resolution := a.pack.TextureSourceScale("Common0/Textures/Button_Text_SD")
	textRect = resolutionRect(textRect, resolution)
	// The label atlas is the port's stand-in for the Text component
	// (AoZ_Terminal.ttf, 79x23 centred at layout.text, rotation 0). Its pixels
	// keep their size relative to the Screen, as in the older layout.
	label := menuGroupChild{x: layout.text.x, y: layout.text.y, w: float64(textRect.Dx()) * layout.screen.w / 128 / resolution, h: float64(textRect.Dy()) * layout.screen.w / 128 / resolution}
	a.drawImage(screen, labels.SubImage(textRect).(*ebiten.Image), menuGroupChildOptions(layout, motion, label, float64(textRect.Dx()), float64(textRect.Dy()), float64(textRect.Dx())/2, float64(textRect.Dy())/2, 1, 1))
}

// drawMenuClickNative is the clicked-state counterpart for native groups. The
// 1.2.5 state setter 0x000dacb0 (case 1) moves the Screen's texture window to
// (0, .5) -- DAT_000db09c = 0.5 -- the bloodied lower half of Button_Screen, and
// launches it with a +/-45 x velocity; the port keeps its own flight path and
// brief red body flash, drawn here at the native board/body size.
func (a *app) drawMenuClickNative(screen *ebiten.Image, layout *menuGroupLayout, click menuClickState, motion menumotion.NativeMenuZombieMotion) {
	scale := float64(motion.Scale)
	if scale <= 0 {
		return
	}
	dx, dy := (click.x-float64(motion.X))/scale, (click.y-float64(motion.Y))/scale
	if click.age < .125 {
		art := layout.back.art
		if zombie, err := a.Texture(fmt.Sprintf("Frontend0/Textures/menu_zombie_%d_SD", art)); err == nil {
			bounds := zombie.Bounds()
			options := menuGroupChildOptions(layout, motion, layout.back, float64(bounds.Dx()), float64(bounds.Dy()), float64(bounds.Dx())/2, float64(bounds.Dy())/2, 1, 1)
			if click.age >= .04 {
				options.ColorScale.Scale(1.5, .18, .18, 1)
			}
			a.drawImage(screen, zombie, options)
		}
	}
	if click.age >= zombieHitFlashDuration {
		pop := menuGroupChildOptions(layout, motion, menuGroupChild{x: 7, y: -2, w: 1, h: 1}, 1, 1, .5, .5, 1, 1)
		popX, popY := pop.GeoM.Apply(.5, .5)
		a.drawBloodPopSprite(screen, popX, popY, .9*float64(motion.Scale/motion.BaseScale), click.index%3, click.age-zombieHitFlashDuration)
	}
	board, err := a.Texture("Common0/Textures/Button_Screen")
	if err != nil {
		return
	}
	flying := layout.screen
	flying.x, flying.y = flying.x+dx, flying.y+dy
	frame := board.SubImage(image.Rect(0, 64, 128, 128)).(*ebiten.Image)
	a.drawImage(screen, frame, menuGroupChildOptions(layout, motion, flying, 128, 64, 64, 32, 1, 1))
}
