package formats

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"regexp"
)

// UIScreen is a decoded compiled BrickUI screen (.uiscreen). Only the layout
// properties the game needs are decoded: component tree, local position, size,
// rotation, texture, text, decor value and texture coordinates. Animations and
// events are not decoded.
type UIScreen struct {
	Components []*UIComponent
	byName     map[string][]*UIComponent
}

type UIComponent struct {
	ID, Parent int
	Name       string
	Class      string
	Children   []*UIComponent
	parent     *UIComponent
	props      map[string][]byte
	anim       []byte
}

var uiClassPattern = regexp.MustCompile(`(Component[A-Za-z]+|UIModifier[A-Za-z]+)\x00`)

// Property names decoded from component records.
var uiPropertyNames = []string{"originFromCenter", "position", "rotation", "width", "height", "texture", "text", "currentValue", "texCoordPos", "texCoordSize", "alignment", "fontSize", "enabled", "drawninepatch", "colour", "textColour", "visible"}

func ParseUIScreen(data []byte) (*UIScreen, error) {
	if len(data) < 64 || string(data[:4]) != "FSIU" {
		return nil, fmt.Errorf("not a uiscreen file")
	}
	head := data
	if len(head) > 400 {
		head = head[:400]
	}
	classes := uiClassTable(data)
	if classes == nil {
		for _, match := range uiClassPattern.FindAllSubmatch(head, -1) {
			classes = append(classes, string(match[1]))
		}
	}
	var starts []int
	for offset := 0; ; {
		index := bytes.Index(data[offset:], []byte("HCIU"))
		if index < 0 {
			break
		}
		starts = append(starts, offset+index)
		offset += index + 4
	}
	if len(starts) == 0 {
		return nil, fmt.Errorf("uiscreen has no components")
	}
	screen := &UIScreen{byName: map[string][]*UIComponent{}}
	byID := map[int]*UIComponent{}
	for i, start := range starts {
		end := len(data)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		if start+24 > len(data) {
			break
		}
		id := int(binary.LittleEndian.Uint32(data[start+8:]))
		nameLength := int(binary.LittleEndian.Uint32(data[start+16:]))
		nameEnd := start + 20 + nameLength
		if nameLength <= 0 || nameLength > 128 || nameEnd+8 > end {
			return nil, fmt.Errorf("uiscreen component %d has a malformed header", i)
		}
		classIndex := int(binary.LittleEndian.Uint32(data[nameEnd+4:]))
		component := &UIComponent{ID: id, Parent: int(int32(binary.LittleEndian.Uint32(data[nameEnd:]))), Name: string(bytes.TrimRight(data[start+20:nameEnd], "\x00")), props: map[string][]byte{}}
		if classIndex < len(classes) {
			component.Class = classes[classIndex]
		}
		body := data[nameEnd+8 : end]
		if limit := bytes.Index(body, []byte("MAIU")); limit >= 0 {
			component.anim = body[limit:]
			body = body[:limit]
		}
		for _, name := range uiPropertyNames {
			needle := append(binary.LittleEndian.AppendUint32(nil, uint32(len(name)+1)), append([]byte(name), 0)...)
			at := bytes.Index(body, needle)
			if at < 0 {
				continue
			}
			rest := body[at+len(needle):]
			window := rest
			if len(window) > 64 {
				window = window[:64]
			}
			marker := bytes.Index(window, []byte{1, 0, 0, 0, 0, 0, 0, 0})
			if marker < 0 {
				continue
			}
			component.props[name] = rest[marker+8:]
		}
		screen.Components = append(screen.Components, component)
		byID[component.ID] = component
		screen.byName[component.Name] = append(screen.byName[component.Name], component)
	}
	for _, component := range screen.Components {
		if parent, ok := byID[component.Parent]; ok && parent != component {
			component.parent = parent
			parent.Children = append(parent.Children, component)
		}
	}
	return screen, nil
}

// FindIn returns the first component called name that sits anywhere below the
// component called parent.
func (s *UIScreen) FindIn(parent, name string) *UIComponent {
	for _, root := range s.byName[parent] {
		var search func(node *UIComponent) *UIComponent
		search = func(node *UIComponent) *UIComponent {
			for _, child := range node.Children {
				if child.Name == name {
					return child
				}
				if found := search(child); found != nil {
					return found
				}
			}
			return nil
		}
		if found := search(root); found != nil {
			return found
		}
	}
	return nil
}

func (s *UIScreen) Find(name string) *UIComponent {
	if list := s.byName[name]; len(list) > 0 {
		return list[0]
	}
	return nil
}

func (c *UIComponent) float(name string, index int) (float64, bool) {
	raw, ok := c.props[name]
	if !ok || len(raw) < 4*(index+1) {
		return 0, false
	}
	value := math.Float32frombits(binary.LittleEndian.Uint32(raw[4*index:]))
	if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
		return 0, false
	}
	return float64(value), true
}

func (c *UIComponent) Position() (x, y float64) {
	x, _ = c.float("position", 0)
	y, _ = c.float("position", 1)
	return
}

func (c *UIComponent) Rotation() float64 {
	z, _ := c.float("rotation", 2)
	return z
}

func (c *UIComponent) Width() (float64, bool)  { return c.float("width", 0) }
func (c *UIComponent) Height() (float64, bool) { return c.float("height", 0) }

func (c *UIComponent) OriginFromCenter() bool {
	raw, ok := c.props["originFromCenter"]
	return ok && len(raw) > 0 && raw[0] != 0
}

func (c *UIComponent) Enabled() bool {
	raw, ok := c.props["enabled"]
	return !ok || len(raw) == 0 || raw[0] != 0
}

func (c *UIComponent) String(name string) string {
	raw, ok := c.props[name]
	if !ok || len(raw) < 4 {
		return ""
	}
	length := int(binary.LittleEndian.Uint32(raw))
	if length < 0 || 4+length > len(raw) {
		return ""
	}
	return string(bytes.TrimRight(raw[4:4+length], "\x00"))
}

func (c *UIComponent) Texture() string { return c.String("texture") }

func (c *UIComponent) CurrentValue() int {
	raw, ok := c.props["currentValue"]
	if !ok || len(raw) < 4 {
		return 0
	}
	return int(int32(binary.LittleEndian.Uint32(raw)))
}

// TexCoords returns the texture window as position and size fractions
// (default is the whole texture).
func (c *UIComponent) TexCoords() (x, y, w, h float64) {
	w, h = 1, 1
	if _, ok := c.props["texCoordPos"]; ok {
		x, _ = c.float("texCoordPos", 0)
		y, _ = c.float("texCoordPos", 1)
	}
	if _, ok := c.props["texCoordSize"]; ok {
		if value, ok := c.float("texCoordSize", 0); ok && value > 0 {
			w = value
		}
		if value, ok := c.float("texCoordSize", 1); ok && value > 0 {
			h = value
		}
	}
	return
}

// TexCoordFlip reports whether the component mirrors its texture. The native
// quad builder (0x003240b0 via 0x00403ff4) writes the four vertex UVs as
// pos, pos+(size.x,0), pos+size, pos+(0,size.y) on an axis-aligned rectangle, so
// a negative texCoordSize component mirrors the image in the component's own
// space, before the rotation/scale/translation matrix is applied.
func (c *UIComponent) TexCoordFlip() (flipX, flipY bool) {
	if value, ok := c.float("texCoordSize", 0); ok && value < 0 {
		flipX = true
	}
	if value, ok := c.float("texCoordSize", 1); ok && value < 0 {
		flipY = true
	}
	return
}

// Anchor returns the component's anchor point in screen space: its centre when
// it originates from its centre, otherwise its top-left corner. A child's local
// position is relative to its parent's anchor.
func (c *UIComponent) Anchor() (x, y float64) {
	if c.parent != nil {
		x, y = c.parent.Anchor()
	}
	localX, localY := c.Position()
	return x + localX, y + localY
}

// FontSize returns the component's fontSize property.
func (c *UIComponent) FontSize() (float64, bool) { return c.float("fontSize", 0) }

// Colour returns a colour property ("colour" or "textColour") as 0..255 channels
// (the compiled screen stores four floats).
func (c *UIComponent) Colour(name string) (r, g, b, a float64, ok bool) {
	raw, present := c.props[name]
	if !present || len(raw) < 16 {
		return 0, 0, 0, 0, false
	}
	values := [4]float64{}
	for index := range values {
		value := math.Float32frombits(binary.LittleEndian.Uint32(raw[4*index:]))
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return 0, 0, 0, 0, false
		}
		values[index] = float64(value)
	}
	return values[0], values[1], values[2], values[3], true
}

// uiClassTable reads the screen's component class table: after the "~PC" tag come
// a count and that many length-prefixed names, closed by "PMOC". It is nil when the
// table is not laid out that way (the caller then scans for class names). Screens
// such as AchievementsScreen list a bare "Component" class that a name scan misses,
// which would shift every later class index.
func uiClassTable(data []byte) []string {
	at := bytes.Index(data, []byte("~PC\x00"))
	if at < 0 {
		return nil
	}
	// Some screens (1.4.0 LevelSelectScreen) put a "~default" name before the count.
	if classes := uiReadClassTable(data, at+4); classes != nil {
		return classes
	}
	if at+8 <= len(data) {
		skip := int(binary.LittleEndian.Uint32(data[at+4:]))
		if skip > 0 && skip < 32 {
			return uiReadClassTable(data, at+8+skip)
		}
	}
	return nil
}

func uiReadClassTable(data []byte, position int) []string {
	if position+4 > len(data) {
		return nil
	}
	count := int(binary.LittleEndian.Uint32(data[position:]))
	position += 4
	if count <= 0 || count > 64 {
		return nil
	}
	classes := make([]string, 0, count)
	for i := 0; i < count; i++ {
		if position+4 > len(data) {
			return nil
		}
		length := int(binary.LittleEndian.Uint32(data[position:]))
		position += 4
		if length <= 0 || length > 64 || position+length > len(data) {
			return nil
		}
		classes = append(classes, string(bytes.TrimRight(data[position:position+length], "\x00")))
		position += length
	}
	if position+8 > len(data) || !bytes.Contains(data[position:position+8], []byte("PMOC")) {
		return nil
	}
	return classes
}

// Visible is the component's visible flag (true when the screen does not set it).
func (c *UIComponent) Visible() bool {
	raw, ok := c.props["visible"]
	return !ok || len(raw) == 0 || raw[0] != 0
}

// UIKeyframe is one key of an animation track: time in seconds, the interpolation
// mode id the screen stores, and the vector value.
type UIKeyframe struct {
	Time    float64
	Ease    int
	X, Y, Z float64
}

// AnimationPosition returns the position track of the named animation (AnimateIn,
// AnimateOut, ...) of this component, nil when it has none. Each key is
// "FKIU", time in milliseconds, mode, then three float32 (24 bytes in all).
func (c *UIComponent) AnimationPosition(animation string) []UIKeyframe {
	named := func(prefix string, name string) []byte {
		out := append([]byte(prefix), binary.LittleEndian.AppendUint32(nil, uint32(len(name)+1))...)
		return append(append(out, name...), 0)
	}
	start := bytes.Index(c.anim, named("MASU", animation))
	if start < 0 {
		return nil
	}
	segment := c.anim[start+len(named("MASU", animation)):]
	if next := bytes.Index(segment, []byte("MASU")); next >= 0 {
		segment = segment[:next]
	}
	track := named("TAIU", "position")
	at := bytes.Index(segment, track)
	if at < 0 || at+len(track)+4 > len(segment) {
		return nil
	}
	position := at + len(track)
	count := int(binary.LittleEndian.Uint32(segment[position:]))
	position += 4
	var keys []UIKeyframe
	for i := 0; i < count && position+24 <= len(segment) && string(segment[position:position+4]) == "FKIU"; i++ {
		f := func(offset int) float64 {
			return float64(math.Float32frombits(binary.LittleEndian.Uint32(segment[position+offset:])))
		}
		keys = append(keys, UIKeyframe{Time: float64(binary.LittleEndian.Uint32(segment[position+4:])) / 1000, Ease: int(binary.LittleEndian.Uint32(segment[position+8:])), X: f(12), Y: f(16), Z: f(20)})
		position += 24
	}
	return keys
}
