package game

import (
	"fmt"
	"image"
	"math"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Credits state (HD 1.2.5 only). Evidence: Research/native/credits-cameo-skip-2026-10-10.md.
// The screen is the compiled Common0/UserInterface/screens/CreditsScreen.uiscreen with
// its controller graph (Controller/CreditsScreen.txt). The roll text, style colours,
// the seven tracks and the version string are literals of the 1.2.5 libmortargame.so
// (credits builder 0x000cc208, roll table 0x008c18c4). The SD (1.2.1) credits state
// was not decoded and keeps the previous behaviour.
const (
	creditsPage        = 7
	creditsSlideTime   = 0.5  // AnimateIn / AnimateOut key time 500 ms
	creditsScrollSpeed = 20.0 // ComponentCredits autoScrollSpeed -20 (vertical, upwards)
	creditsPageHeight  = 29.0 // SwipiePageTemplate height
	creditsVersionText = "1.2.5"
	creditsVersionNo   = 54
)

// creditsRoll is the 116-entry table the native builder walks (12-byte records:
// style, text pointer, third word not read by the builder). Style 0 white heading,
// 1 green name or blank spacer, 2 invisible spacer (alpha 0), 3 grey role. An empty
// text is the native two-space blank line.
var creditsRoll = [...]struct {
	Style int
	Text  string
}{
	{0, "Mobile Adaptation"},
	{3, "Design"},
	{1, "Michael Dobele"},
	{1, ""},
	{0, "Product Owner"},
	{1, "Ramine Darabiha"},
	{1, ""},
	{0, "Producer"},
	{1, "Bethany Ward"},
	{1, ""},
	{3, "Level Design"},
	{1, "Ryan Langley"},
	{1, ""},
	{3, "Programming"},
	{1, "Anthony Hansen"},
	{1, "James Barnes"},
	{1, "Jordan Comino"},
	{1, "Grant Peters"},
	{1, "Peter McNeill"},
	{1, "David Kilford"},
	{1, ""},
	{3, "Art"},
	{1, "Motze Asher"},
	{1, "Murry Lancashire"},
	{1, "Resa Liputra"},
	{1, "Hugh Walters"},
	{1, "Bob Jones"},
	{1, ""},
	{0, "Sound and Music"},
	{1, "Jesse Higginson"},
	{1, "Cedar Jones"},
	{1, ""},
	{3, "Marketing"},
	{1, "Justin Bowen"},
	{1, "Beverley Chen"},
	{1, "Sara Fonseca"},
	{1, "Phil Larsen"},
	{1, "Sean Ockert"},
	{1, "James Schultz"},
	{1, "Rod Wong"},
	{1, ""},
	{3, "Distribution"},
	{1, "Daniel John"},
	{1, ""},
	{3, "Quality Assurance"},
	{1, "Brent Hobson"},
	{1, "Jason Maundrell"},
	{1, "Kirby Scarfe"},
	{1, ""},
	{3, "Analytics"},
	{1, "Andrew Saul"},
	{1, "Corey Taylor"},
	{1, "John Cominos"},
	{1, ""},
	{0, "Focus Testers"},
	{1, "Aaron Green"},
	{1, "Jason Harwood"},
	{1, ""},
	{3, "HD Outsourcing"},
	{1, "SkySoul"},
	{1, ""},
	{3, "Special Thanks"},
	{1, "Alex Butterfield"},
	{1, "Tony Takoushi"},
	{1, "Shainiel Deo"},
	{1, "Will Goddard"},
	{1, "Michael Szewczyk"},
	{2, ""},
	{0, "Original Development"},
	{3, "Game Design"},
	{1, "Shainiel Deo"},
	{1, "Anthony Hansen"},
	{1, "Ryan Langley"},
	{1, "Phil Larsen"},
	{1, "Stephen Last"},
	{1, ""},
	{0, "Programming"},
	{1, "Stephen Last"},
	{1, "Paul McNab"},
	{1, ""},
	{3, "Art"},
	{1, "Motze Asher"},
	{1, "Murry Lancashire"},
	{1, ""},
	{0, "Sound and Music"},
	{1, "Jesse Higginson"},
	{1, ""},
	{3, "Level Design"},
	{1, "Ryan Langley"},
	{1, ""},
	{3, "Script"},
	{1, "Phil Larsen"},
	{1, ""},
	{0, "Marketing"},
	{1, "Phil Larsen"},
	{1, ""},
	{3, "Quality Assurance"},
	{1, "Brent Hobson"},
	{1, "Jason Maundrell"},
	{1, ""},
	{0, "Halfbrick CEO"},
	{1, "Shainiel Deo"},
	{1, ""},
	{0, "Special Thanks"},
	{1, "Steve Bennett"},
	{1, "Rose Deo"},
	{1, "Joe Gatling"},
	{1, "Todd Hunt"},
	{1, "Angela McNab"},
	{1, "DL Media"},
	{1, "Mark Milton"},
	{1, "Mr. Morowitz"},
	{1, "Chloe Pearson"},
	{1, "Xavier Yun Ho"},
	{2, ""},
	{2, ""},
}

// creditsTrackNames are the NowPlayingTrackName strings in table order; the music
// files are the menu theme followed by the six world themes.
var creditsTrackNames = [...]string{"Title Theme", "Prehistoric", "Gangster Theme", "Egyptian Theme", "Japan Theme", "Future Theme", "Western Theme"}

// creditsSlides are the AnimateIn positions of the three animated roots: from x, to x, y
// (AnimateOut runs them backwards).
var creditsSlides = map[string][3]float64{"Screen": {-238, 138, 161}, "BackBox": {666, 366, 270}, "NowPlaying": {666, 366, 179}}

type creditsPhase int

const (
	creditsIn creditsPhase = iota
	creditsIdle
	creditsOut
)

// creditsExit says where the state leaves to: the main menu, or the next story level
// (a SHOWCREDITS level whose nextLevel exists).
type creditsExit struct {
	hasNext      bool
	world, level int
}

type creditsState struct {
	exit      creditsExit
	phase     creditsPhase
	phaseTime float64
	age       float64
	focus     string
	track     int
	scroll    float64
	dragging  bool
	dragY     int
	backPage  int
}

func newCreditsState(exit creditsExit, focus string) *creditsState {
	if focus == "" {
		focus = "Back"
	}
	return &creditsState{exit: exit, focus: focus}
}

func creditsRollLength() float64 { return float64(len(creditsRoll)) * creditsPageHeight }

// slide is the animation progress: 0 fully off screen, 1 in place.
func (c *creditsState) slide() float64 {
	switch c.phase {
	case creditsIn:
		return math.Min(1, c.phaseTime/creditsSlideTime)
	case creditsOut:
		return math.Max(0, 1-c.phaseTime/creditsSlideTime)
	}
	return 1
}

// inputEnabled mirrors the SetProperty_bool inputEnabled events: false at the start of
// both animations, true at the end of AnimateIn.
func (c *creditsState) inputEnabled() bool { return c.phase == creditsIdle }

// step advances the state by dt seconds; it returns true when AnimateOut has ended.
func (c *creditsState) step(dt float64) bool {
	c.age += dt
	c.phaseTime += dt
	switch c.phase {
	case creditsIn:
		if c.phaseTime >= creditsSlideTime {
			c.phase, c.phaseTime = creditsIdle, 0
		}
	case creditsOut:
		if c.phaseTime >= creditsSlideTime {
			return true
		}
	}
	if !c.dragging {
		c.scroll = wrapCreditsScroll(c.scroll + creditsScrollSpeed*dt)
	}
	return false
}

func wrapCreditsScroll(value float64) float64 {
	length := creditsRollLength()
	value = math.Mod(value, length)
	if value < 0 {
		value += length
	}
	return value
}

// creditsStepTrack is the PrevTrack / NextTrack handler (0x000cc198 / 0x000cc1d0): the
// index wraps over the seven entries.
func creditsStepTrack(track, delta int) int {
	count := len(creditsTrackNames)
	return ((track+delta)%count + count) % count
}

// creditsNavigate walks the controller graph; the focus is unchanged when the
// direction has no target.
func creditsNavigate(controller *uiController, focus, dir string) string {
	if target, ok := controller.neighbour(focus, dir); ok {
		return target
	}
	return focus
}

func creditsVersionLine() string {
	return fmt.Sprintf("Game Version: %s (%d)", creditsVersionText, creditsVersionNo)
}

func (a *app) creditsUIScreen() *formats.UIScreen {
	if a == nil || a.pack == nil {
		return nil
	}
	if a.creditsUILoaded {
		return a.creditsUI
	}
	a.creditsUILoaded = true
	path, ok := a.pack.SourcePath("Common0/UserInterface/screens/CreditsScreen.uiscreen")
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
	if err != nil || screen.Find("Screen") == nil || screen.Find("BackBox") == nil || screen.Find("NowPlaying") == nil {
		return nil
	}
	a.creditsUI = screen
	return screen
}

// creditsAvailable reports whether the native Credits state applies: a story session
// of the HD (1.2.5) build with its credits screen present.
func (a *app) creditsAvailable() bool {
	return a != nil && a.pack != nil && a.net == nil && a.play != nil && a.play.waveBuild() == content.WaveBuild125 && a.creditsUIScreen() != nil
}

func (a *app) openCredits(info formats.LevelInfo, exit creditsExit) error {
	if err := a.recordStoryCompletion(info); err != nil {
		return err
	}
	a.stopWeaponPlayback()
	a.play.closeScript()
	focus := ""
	if controller := a.uiController("CreditsScreen"); controller != nil {
		focus = controller.def
	}
	a.credits = newCreditsState(exit, focus)
	a.credits.backPage = a.page
	a.page = creditsPage
	a.playCreditsTrack()
	return a.savePlayerProfile()
}

// playCreditsTrack starts the selected track (0x000cc0d8): the menu theme, then the
// six world themes.
func (a *app) playCreditsTrack() {
	if a.credits == nil {
		return
	}
	if a.credits.track == 0 {
		a.setMenuMusic()
		return
	}
	a.setWorldMusic(a.credits.track - 1)
}

// finishCredits leaves the Credits state once AnimateOut has finished: the final level
// goes to the main menu, a SHOWCREDITS level loads its next level.
func (a *app) finishCredits() error {
	c := a.credits
	a.credits = nil
	if c == nil {
		return nil
	}
	if !c.exit.hasNext || a.play == nil {
		a.stopWeaponPlayback()
		if a.play != nil {
			a.play.closeScript()
		}
		a.play, a.page = nil, 0
		a.setMenuMusic()
		return a.savePlayerProfile()
	}
	score, combatKills := a.play.score, a.play.combatKillCount
	a.page = c.backPage
	a.world, a.level = c.exit.world, c.exit.level
	if err := a.openPlay(); err != nil {
		return err
	}
	a.play.score, a.play.levelStartScore = score, score
	a.play.combatKillCount = combatKills
	return nil
}

// creditsRootOffset is the displacement of a sliding root from its stored position
// (the animation replaces the position property).
func (a *app) creditsRootOffset(ui *formats.UIScreen, c *creditsState, root string) formats.Vec2 {
	slide, ok := creditsSlides[root]
	component := ui.Find(root)
	if !ok || component == nil {
		return formats.Vec2{}
	}
	x := slide[0] + (slide[1]-slide[0])*c.slide()
	baseX, baseY := component.Anchor()
	return formats.Vec2{X: x - baseX, Y: slide[2] - baseY}
}

func (a *app) creditsButtonRect(ui *formats.UIScreen, c *creditsState, name string) image.Rectangle {
	component := ui.Find(name)
	if component == nil {
		return image.Rectangle{}
	}
	offset := a.creditsRootOffset(ui, c, "BackBox")
	x, y := component.Anchor()
	width, _ := component.Width()
	height, _ := component.Height()
	return image.Rect(int(x+offset.X-width/2), int(y+offset.Y-height/2), int(x+offset.X+width/2), int(y+offset.Y+height/2))
}

func (a *app) creditsRollRect(ui *formats.UIScreen, c *creditsState) image.Rectangle {
	component := ui.Find("ComponentCredits")
	if component == nil {
		return image.Rectangle{}
	}
	offset := a.creditsRootOffset(ui, c, "Screen")
	x, y := component.Anchor()
	width, _ := component.Width()
	height, _ := component.Height()
	return image.Rect(int(x+offset.X), int(y+offset.Y), int(x+offset.X+width), int(y+offset.Y+height))
}

func (a *app) creditsPress(c *creditsState, node string) {
	switch node {
	case "Back":
		a.playSound("audio/sound/sfx/menu_transition_out.ogg", .7)
		c.phase, c.phaseTime = creditsOut, 0
	case "PrevTrack", "NextTrack":
		delta := 1
		if node == "PrevTrack" {
			delta = -1
		}
		a.playSound("audio/sound/sfx/menu_move.ogg", .7)
		c.track = creditsStepTrack(c.track, delta)
		a.playCreditsTrack()
	}
}

func (a *app) updateCredits() error {
	c := a.credits
	if c.step(1.0 / 60.0) {
		return a.finishCredits()
	}
	if !c.inputEnabled() {
		return nil
	}
	controller := a.uiController("CreditsScreen")
	back := "Back"
	if controller != nil && controller.back != "" {
		back = controller.back
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		c.focus = back
		a.creditsPress(c, back)
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
		a.creditsPress(c, c.focus)
		return nil
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyLeft):
		c.focus = creditsNavigate(controller, c.focus, "Left")
	case inpututil.IsKeyJustPressed(ebiten.KeyRight):
		c.focus = creditsNavigate(controller, c.focus, "Right")
	case inpututil.IsKeyJustPressed(ebiten.KeyDown):
		c.scroll = wrapCreditsScroll(c.scroll + creditsPageHeight)
	case inpututil.IsKeyJustPressed(ebiten.KeyUp):
		c.scroll = wrapCreditsScroll(c.scroll - creditsPageHeight)
	}
	if _, wheel := ebiten.Wheel(); wheel != 0 {
		c.scroll = wrapCreditsScroll(c.scroll - wheel*creditsPageHeight)
	}
	ui := a.creditsUIScreen()
	if ui == nil {
		return nil
	}
	x, y := a.pointer()
	point := image.Pt(x, y)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		for _, name := range []string{"Back", "PrevTrack", "NextTrack"} {
			if point.In(a.creditsButtonRect(ui, c, name)) {
				c.focus = name
				a.creditsPress(c, name)
				return nil
			}
		}
		if point.In(a.creditsRollRect(ui, c)) {
			c.focus, c.dragging, c.dragY = "Credits", true, y
		}
	}
	if c.dragging {
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			c.scroll = wrapCreditsScroll(c.scroll - float64(y-c.dragY))
			c.dragY = y
		} else {
			c.dragging = false
		}
	}
	return nil
}

func creditsCleanTexture(path string) string {
	if at := strings.IndexByte(path, 0); at >= 0 {
		path = path[:at]
	}
	return path
}

// creditsNineMargin infers a nine-patch margin from the texture pixels: the border
// length before the central uniform run. The native window segmentation (smetafile /
// mainbody) was not decoded, so this is an approximation that depends only on the
// shipped texture.
func creditsNineMargin(texture *ebiten.Image) int {
	bounds := texture.Bounds()
	margin := 1
	for _, horizontal := range []bool{true, false} {
		length := bounds.Dx()
		if !horizontal {
			length = bounds.Dy()
		}
		at := func(i int) [4]uint32 {
			var r, g, b, al uint32
			if horizontal {
				r, g, b, al = texture.At(bounds.Min.X+i, bounds.Min.Y+bounds.Dy()/2).RGBA()
			} else {
				r, g, b, al = texture.At(bounds.Min.X+bounds.Dx()/2, bounds.Min.Y+i).RGBA()
			}
			return [4]uint32{r, g, b, al}
		}
		middle := at(length / 2)
		start := length / 2
		for start > 0 && at(start-1) == middle {
			start--
		}
		margin = max(margin, start)
	}
	return min(margin, bounds.Dx()/2-1)
}

func (a *app) drawCreditsNinePatch(screen *ebiten.Image, component *formats.UIComponent, offset formats.Vec2) {
	path := creditsCleanTexture(component.Texture())
	if path == "" {
		return
	}
	name := uiTextureName(path)
	texture, err := a.Texture(name)
	if err != nil {
		return
	}
	width, okW := component.Width()
	height, okH := component.Height()
	if !okW || !okH {
		return
	}
	resolution := a.pack.TextureSourceScale(name)
	if resolution <= 0 {
		resolution = 1
	}
	if a.creditsMargins == nil {
		a.creditsMargins = map[string]int{}
	}
	margin, cached := a.creditsMargins[name]
	if !cached {
		margin = creditsNineMargin(texture)
		a.creditsMargins[name] = margin
	}
	x, y := component.Anchor()
	x, y = x+offset.X, y+offset.Y
	if component.OriginFromCenter() {
		x, y = x-width/2, y-height/2
	}
	bounds := texture.Bounds()
	edge := math.Min(float64(margin)/resolution, math.Min(width, height)/2)
	xs := [4]float64{x, x + edge, x + width - edge, x + width}
	ys := [4]float64{y, y + edge, y + height - edge, y + height}
	us := [4]int{bounds.Min.X, bounds.Min.X + margin, bounds.Max.X - margin, bounds.Max.X}
	vs := [4]int{bounds.Min.Y, bounds.Min.Y + margin, bounds.Max.Y - margin, bounds.Max.Y}
	for row := 0; row < 3; row++ {
		for column := 0; column < 3; column++ {
			source := image.Rect(us[column], vs[row], us[column+1], vs[row+1])
			targetWidth, targetHeight := xs[column+1]-xs[column], ys[row+1]-ys[row]
			if source.Empty() || targetWidth <= 0 || targetHeight <= 0 {
				continue
			}
			options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
			options.GeoM.Scale(targetWidth/float64(source.Dx()), targetHeight/float64(source.Dy()))
			options.GeoM.Translate(xs[column], ys[row])
			a.drawImage(screen, texture.SubImage(source).(*ebiten.Image), options)
		}
	}
}

// drawCreditsText draws text centred on (cx, cy) in a box of the given width;
// align "CenterRight" right-aligns inside the box.
func (a *app) drawCreditsText(screen *ebiten.Image, text string, cx, cy, width float64, align string, size float64, r, g, b float64) {
	if a.font == nil || a.font.LineHeight == 0 || text == "" || size <= 0 {
		return
	}
	scale := size / float64(a.font.LineHeight)
	textWidth := a.fontTextWidth(text, scale)
	x := cx - textWidth/2
	switch align {
	case "CenterLeft":
		x = cx - width/2
	case "CenterRight":
		x = cx + width/2 - textWidth
	}
	sx, sy := a.renderScale()
	a.font.DrawScaledTinted(screen, text, x*sx, (cy-size/2)*sy, scale*sx, scale*sy, float32(r/255), float32(g/255), float32(b/255))
}

func creditsStyleColour(style int) (r, g, b float64, visible bool) {
	switch style {
	case 1:
		return 100, 228, 100, true
	case 2:
		return 255, 255, 255, false
	case 3:
		return 200, 200, 200, true
	}
	return 255, 255, 255, true
}

func (a *app) drawCreditsRoll(screen *ebiten.Image, ui *formats.UIScreen, component *formats.UIComponent, offset formats.Vec2, c *creditsState) {
	x, y := component.Anchor()
	x, y = x+offset.X, y+offset.Y
	width, _ := component.Width()
	height, _ := component.Height()
	sx, sy := a.renderScale()
	clip := image.Rect(int(math.Floor(x*sx)), int(math.Floor(y*sy)), int(math.Ceil((x+width)*sx)), int(math.Ceil((y+height)*sy))).Intersect(screen.Bounds())
	if clip.Empty() {
		return
	}
	target := screen.SubImage(clip).(*ebiten.Image)
	size := 18.0
	if template := ui.FindIn("SwipiePageTemplate", "ComponentText"); template != nil {
		if value, ok := template.FontSize(); ok {
			size = value
		}
	}
	length := creditsRollLength()
	for index, line := range creditsRoll {
		r, g, b, visible := creditsStyleColour(line.Style)
		if !visible || strings.TrimSpace(line.Text) == "" {
			continue
		}
		for _, shift := range []float64{0, length} { // the content is looped
			top := y + float64(index)*creditsPageHeight - c.scroll + shift
			if top+creditsPageHeight < y || top > y+height {
				continue
			}
			a.drawCreditsText(target, line.Text, x+width/2, top+creditsPageHeight/2, width, "", size, r, g, b)
		}
	}
}

// drawCreditsBackdrop is the state's own render (0x000cbd18): portal_menu at
// (0.7 W, 0.3 H), 864 square, turning at game time / 3.
func (a *app) drawCreditsBackdrop(screen *ebiten.Image) {
	for _, name := range []string{"Common0/Textures/portal_menu_SD", "Frontend0/Textures/Portal_Menu_SD"} {
		texture, err := a.Texture(name)
		if err != nil {
			continue
		}
		bounds := texture.Bounds()
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Translate(-float64(bounds.Dx())/2, -float64(bounds.Dy())/2)
		options.GeoM.Scale(864/float64(bounds.Dx()), 864/float64(bounds.Dy()))
		options.GeoM.Rotate(a.menuTime / 3)
		options.GeoM.Translate(.7*logicalWidth, .3*logicalHeight)
		a.drawImage(screen, texture, options)
		return
	}
}

var creditsNineNames = map[string]bool{"Backing": true, "Box": true, "Back": true, "PrevTrack": true, "NextTrack": true}

func (a *app) drawCredits(screen *ebiten.Image) {
	c := a.credits
	ui := a.creditsUIScreen()
	a.drawCreditsBackdrop(screen)
	if c == nil || ui == nil {
		return
	}
	for _, rootName := range []string{"Screen", "BackBox", "NowPlaying"} {
		root := ui.Find(rootName)
		if root == nil {
			continue
		}
		offset := a.creditsRootOffset(ui, c, rootName)
		var walk func(node *formats.UIComponent)
		walk = func(node *formats.UIComponent) {
			for _, child := range node.Children {
				if !child.Enabled() {
					continue
				}
				switch {
				case child.Name == "ComponentCredits":
					a.drawCreditsRoll(screen, ui, child, offset, c)
					continue
				case strings.HasPrefix(child.Name, "ComponentAnimatedHudDecor"):
					a.drawUIDecor(screen, child, offset, c.age)
				case child.Texture() != "" && creditsNineNames[child.Name]:
					a.drawCreditsNinePatch(screen, child, offset)
				case child.Texture() != "":
					a.drawUITexture(screen, child, offset)
				}
				a.drawCreditsComponentText(screen, child, offset, c)
				walk(child)
			}
		}
		walk(root)
	}
}

func (a *app) drawCreditsComponentText(screen *ebiten.Image, child *formats.UIComponent, offset formats.Vec2, c *creditsState) {
	text := child.String("text")
	switch child.Name {
	case "NowPlayingTrackName":
		text = creditsTrackNames[c.track]
	case "VersionString":
		text = creditsVersionLine()
	}
	if text == "" || text == "LineOfCredits" {
		return
	}
	size, ok := child.FontSize()
	if !ok {
		return
	}
	r, g, b := 255.0, 255.0, 255.0
	if cr, cg, cb, _, found := child.Colour("textColour"); found && child.Name == "Back" {
		r, g, b = cr, cg, cb
	} else if cr, cg, cb, _, found := child.Colour("colour"); found {
		r, g, b = cr, cg, cb
	}
	x, y := child.Anchor()
	width, _ := child.Width()
	a.drawCreditsText(screen, text, x+offset.X, y+offset.Y, width, child.String("alignment"), size, r, g, b)
}
