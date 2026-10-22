package formats

import "fmt"

type LayerKind string

const (
	LayerG  LayerKind = "g"
	LayerD  LayerKind = "d"
	LayerH  LayerKind = "h"
	LayerHB LayerKind = "hb"
	LayerC  LayerKind = "c"
)

var LayerKinds = []LayerKind{LayerG, LayerD, LayerH, LayerHB, LayerC}
var RenderLayerKinds = []LayerKind{LayerG, LayerHB, LayerD}
// BaseRenderLayerKinds are the layers drawn before the y-bucket list: g then d (native FUN_000bff80). The hb layer is
// drawn inside the bucket list between bucket 9 and bucket 10 (viewer.DrawWithEntityBands) and h is drawn as
// per-row objects in it (Research/native/prop-entity-draw-order-2026-10-12.md).
var BaseRenderLayerKinds = []LayerKind{LayerG, LayerD}

type LevelInfo struct {
	ID               string   `json:"id"`
	DisplayName      string   `json:"displayName"`
	BaseFile         string   `json:"baseFile"`
	NextLevel        string   `json:"nextLevel"`
	UnlockLevels     []string `json:"unlockLevels"`
	Music            string   `json:"music"`
	VoiceoverPrefix  string   `json:"voiceoverPrefix"`
	ConversationXMLs []string `json:"conversationXmls"`
	LeaderboardID    string   `json:"leaderboardId"`
	LeaderboardIDSD  string   `json:"leaderboardIdSD"`
	LeaderboardIDHD  string   `json:"leaderboardIdHD"`
	WorldIndex       int      `json:"worldIndex"`
	Flags            []string `json:"flags"`
	Description      string   `json:"description"`
	PostcardImage    string   `json:"postcardImage"`
	SourceXML        string   `json:"sourceXML"`
}

type Level struct {
	Info          LevelInfo              `json:"info"`
	Width         int                    `json:"width"`
	Height        int                    `json:"height"`
	Tileset       string                 `json:"tileset"`
	Layers        map[LayerKind][]uint32 `json:"layers"`
	Props         []Prop                 `json:"props"`
	AnimatedProps []AnimatedProp         `json:"animatedProps"`
	Waves         []Wave                 `json:"waves"`
	WaveBuild     string                 `json:"-"`
	// Port additions, written only by levels made in the level editor (the original levels never have them).
	Intro   []IntroLine   `json:"intro,omitempty"`
	Pickups []LevelPickup `json:"pickups,omitempty"`
}

// IntroLine is one line of the conversation a custom level opens with. Cameo is a portrait texture name ("" for none).
type IntroLine struct {
	Cameo string `json:"cameo,omitempty"`
	Text  string `json:"text"`
}

// LevelPickup is a crate a custom level puts down at a fixed place: when the level starts (Wave 0 or 1) or when the given
// wave (counting from 1) begins.
type LevelPickup struct {
	Name string  `json:"name"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Wave int     `json:"wave,omitempty"`
}

type Wave struct {
	NextWave       int       `json:"nextWave"`
	RunTime        float64   `json:"runTime"`
	EndWaveTime    float64   `json:"endWaveTime"`
	EndWaveZombies int       `json:"endWaveZombies"`
	Spawners       []Spawner `json:"spawners"`
}

type Spawner struct {
	DelayTime float64     `json:"delayTime"`
	Count     int         `json:"count"`
	Index     int         `json:"index"`
	Types     []SpawnType `json:"types"`
}

type SpawnType struct {
	Name              string  `json:"name"`
	Chance            float64 `json:"chance"`
	Speed             Vec2    `json:"speed"`
	Strength          float64 `json:"strength"`
	Size              Vec2    `json:"size"`
	TurnSpeed         float64 `json:"turnSpeed"`
	Texture           string  `json:"texture"`
	Weapon            string  `json:"weapon"`
	AlertRadius       float64 `json:"alertRadius"`
	DeviateCycleSpeed float64 `json:"deviateCycleSpeed"`
	DeviateAmount     float64 `json:"deviateAmount"`
}

type Prop struct {
	Texture string  `json:"texture"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Height  float64 `json:"height"`
	ScaleX  float64 `json:"scaleX"`
	ScaleY  float64 `json:"scaleY"`
	UV1X    float64 `json:"uv1X"`
	UV1Y    float64 `json:"uv1Y"`
	UV2X    float64 `json:"uv2X"`
	UV2Y    float64 `json:"uv2Y"`
}

type AnimatedProp struct {
	Texture   string  `json:"texture"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Height    float64 `json:"height"`
	ScaleX    float64 `json:"scaleX"`
	ScaleY    float64 `json:"scaleY"`
	XFrames   int     `json:"xFrames"`
	YFrames   int     `json:"yFrames"`
	FrameTime float64 `json:"frameTime"`
}

type TileSet struct {
	Name      string  `json:"name"`
	Texture   string  `json:"texture"`
	TileSize  int     `json:"tileSize"`
	TileShift int     `json:"tileShift"`
	UVOffset  float64 `json:"uvOffset"`
}

func (l Level) Validate() error {
	if l.Width <= 0 || l.Height <= 0 {
		return fmt.Errorf("invalid level dimensions %dx%d", l.Width, l.Height)
	}
	for _, kind := range LayerKinds {
		values, ok := l.Layers[kind]
		if !ok || len(values) != l.Width*l.Height {
			return fmt.Errorf("layer %q has %d values, want %d", kind, len(values), l.Width*l.Height)
		}
	}
	return nil
}
