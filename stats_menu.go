package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image"
)

type statsRecord struct {
	Kind  int
	Label string
	Value *int32
	Delay int
}
type statsNode struct {
	Label, Value string
	X, Y         float32
	Numeric      bool
}
type statsDestination uint8

const (
	statsStay         statsDestination = 0
	statsMainMenu     statsDestination = 4
	statsLevelLoad    statsDestination = 9
	statsInputChannel                  = 3
)

type statsMenu struct {
	Records          []statsRecord
	Nodes            []statsNode
	Index, Delay     int
	Waiting, Closing bool
	Alpha            uint8
	Data             *statsData
}

func statsRecords(d *statsData) []statsRecord {
	return []statsRecord{
		{0, "PERSONAL BESTS", nil, 64},
		{1, "Best Story Level Score", &d.BestStoryScore, 55}, {1, "Best Survival Level Score", &d.BestSurvivalScore, 55},
		{1, "Highest Survival Wave", &d.HighestSurvivalWave, 55}, {1, "Highest Multiplier", &d.HighestMultiplier, 55}, {4, "", nil, 55},
		{1, "Kills with single Shotgun", &d.ShotgunKills, 55}, {1, "Kills with single Uzi", &d.UziKills, 55},
		{1, "Kills with single Flamer", &d.FlamerKills, 55}, {1, "Kills with single Minigun", &d.MinigunKills, 55},
		{1, "Kills with single Rifle", &d.RifleKills, 55}, {1, "Kills with single Buzzsaw", &d.BuzzsawKills, 55},
		{1, "Kills with single Grenade", &d.GrenadeKills, 55}, {1, "Kills with single Mine", &d.MineKills, 55}, {4, "", nil, 55},
		{2, "Best T-Rex time", &d.BestTRexTime, 55}, {2, "Best Gangster Boss Time", &d.BestGangsterTime, 55},
		{2, "Best Egypt Boss Time", &d.BestEgyptTime, 55}, {2, "Best Japan Boss Time", &d.BestJapanTime, 55},
		{2, "Best Future Boss Time", &d.BestFutureTime, 55}, {3, "Total", nil, 55}, {4, "", nil, 80},
		{0, "TOTALS", nil, 64}, {2, "Total Time Played", &d.TimePlayed, 55}, {1, "Number of times played", &d.TimesPlayed, 55},
		{1, "Zombies Killed", &d.ZombiesKilled, 55}, {1, "Zombies Ignited", &d.ZombiesIgnited, 55}, {1, "Lives Lost", &d.LivesLost, 55},
		{1, "Story Levels Played", &d.StoryLevelsPlayed, 55}, {1, "Survival Levels Played", &d.SurvivalLevelsPlayed, 55},
		{1, "Distance Travelled", &d.DistanceTravelled, 55}, {1, "Grenades Tossed", &d.GrenadesTossed, 55},
		{1, "Sentry Guns Used", &d.SentryGunsUsed, 55}, {1, "Sentry Gun Kills", &d.SentryGunKills, 55},
		{1, "Shots Fired", &d.ShotsFired, 55}, {1, "Bosses Killed", &d.BossesKilled, 55},
		{1, "Fruit Sliced", &d.FruitSliced, 55}, {1, "Stats screen views", &d.ScreenViews, 55}, {4, "", nil, 80}, {4, "", nil, 400},
	}
}
func newStatsMenu(data *statsData) *statsMenu {
	data.ScreenViews++
	if data.Available == nil {
		data.Available = map[string]bool{}
	}
	data.Available["Stats screen views"] = true
	return &statsMenu{Records: statsRecords(data), Alpha: 255, Data: data}
}
func (menu *statsMenu) update(dt float32, back, levelActive, animationActive bool) statsDestination {
	nodes := menu.Nodes[:0]
	for _, node := range menu.Nodes {
		node.Y -= dt * 75
		if node.Y >= -60 {
			nodes = append(nodes, node)
		}
	}
	menu.Nodes = nodes
	if menu.Closing {
		if menu.Alpha > 25 {
			menu.Alpha -= 25
		} else {
			menu.Alpha = 0
		}
		if menu.Alpha != 0 {
			return statsStay
		}
		if !levelActive {
			return statsMainMenu
		}
		if !animationActive {
			return statsLevelLoad
		}
		return statsStay
	}
	if back && !animationActive {
		menu.Closing = true
	}
	if menu.Waiting {
		menu.Delay--
		if menu.Delay < 0 {
			menu.Waiting = false
		}
		return statsStay
	}
	record := menu.Records[menu.Index]
	menu.Delay, menu.Waiting = record.Delay, true
	menu.Index = (menu.Index + 1) % len(menu.Records)
	if record.Kind == 4 {
		return statsStay
	}
	node := statsNode{Label: record.Label, X: 10, Y: 320}
	if record.Kind >= 1 && record.Kind <= 3 {
		node.X, node.Numeric = 25, true
		if record.Kind == 3 {
			total := menu.Data.bossTotal()
			if total == 0 {
				node.Value = "Not Set"
			} else {
				node.Value = statsTime(total)
			}
		} else if !menu.Data.Available[record.Label] {
			node.Value = "-"
		} else if record.Kind == 2 {
			node.Value = statsTime(*record.Value)
		} else {
			node.Value = statsInteger(*record.Value)
		}
	}
	menu.Nodes = append(menu.Nodes, node)
	return statsStay
}
func (a *app) drawStatsMenu(screen *ebiten.Image, menu *statsMenu, gameTime float64) {
	if texture, err := a.Texture("Common0/Textures/portal_menu_SD"); err == nil {
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		bounds := texture.Bounds()
		op.GeoM.Translate(-float64(bounds.Dx())/2, -float64(bounds.Dy())/2)
		op.GeoM.Scale(864/float64(bounds.Dx()), 864/float64(bounds.Dy()))
		op.GeoM.Rotate(gameTime / 3)
		op.GeoM.Translate(336, 96)
		a.drawImage(screen, texture, op)
	}
	if a.font == nil || a.font.LineHeight == 0 {
		return
	}
	sx, sy := a.frontendScaleX, a.frontendScaleY
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	scale := 24 / float64(a.font.LineHeight) * sx
	for _, node := range menu.Nodes {
		for _, part := range []struct {
			Text string
			X    float64
		}{{node.Label, float64(node.X)}, {node.Value, 470}} {
			if part.Text == "" {
				continue
			}
			x := part.X * sx
			if part.X >= 240 {
				x -= a.fontTextWidth(part.Text, scale)
			}
			y := float64(node.Y) * sy
			for _, r := range part.Text {
				glyph, ok := a.font.Glyphs[r]
				if !ok {
					continue
				}
				if glyph.Width > 0 && glyph.Height > 0 {
					op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
					op.GeoM.Scale(scale, scale)
					op.GeoM.Translate(x+float64(glyph.XOffset)*scale, y+float64(glyph.YOffset)*scale)
					if node.Numeric {
						op.ColorScale.Scale(0, 1, 0, float32(menu.Alpha)/255)
					} else {
						op.ColorScale.ScaleAlpha(float32(menu.Alpha) / 255)
					}
					screen.DrawImage(a.font.Atlas.SubImage(image.Rect(glyph.X, glyph.Y, glyph.X+glyph.Width, glyph.Y+glyph.Height)).(*ebiten.Image), op)
				}
				x += float64(glyph.XAdvance) * scale
			}
		}
	}
}
