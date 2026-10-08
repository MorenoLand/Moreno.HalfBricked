package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/stats"
	"testing"
)

func TestStatsNativeSequence(t *testing.T) {
	d := stats.NewStatsData()
	m := newStatsMenu(&d)
	if len(m.Records) != 40 || m.Records[22].Label != "TOTALS" || m.Records[34].Label != "Shots Fired" || m.Records[39].Delay != 400 {
		t.Fatal("definition sequence")
	}
	m.update(0, false, false, false)
	if len(m.Nodes) != 1 || m.Nodes[0].Y != 320 || m.Nodes[0].X != 10 {
		t.Fatal(m.Nodes)
	}
	m.update(1, false, false, false)
	if m.Nodes[0].Y != 245 {
		t.Fatal(m.Nodes)
	}
	for i := 0; i < 65; i++ {
		m.update(0, false, false, false)
	}
	if len(m.Nodes) != 2 || m.Nodes[1].X != 25 {
		t.Fatal(m.Nodes)
	}
	for i := 0; i < 4000; i++ {
		m.update(1.0/60, false, false, false)
	}
	if m.Index < 0 || m.Index >= 40 {
		t.Fatal(m.Index)
	}
}
func TestStatsBackDestinations(t *testing.T) {
	for _, active := range []bool{false, true} {
		d := stats.NewStatsData()
		m := newStatsMenu(&d)
		m.update(0, true, active, false)
		var destination statsDestination
		for i := 0; i < 11; i++ {
			destination = m.update(0, false, active, false)
		}
		want := statsMainMenu
		if active {
			want = statsLevelLoad
		}
		if destination != want {
			t.Fatal(destination, want)
		}
	}
}
func TestStatsUnavailableDoesNotInventCounts(t *testing.T) {
	d := stats.NewStatsData()
	m := newStatsMenu(&d)
	m.Index = 25
	m.update(0, false, false, false)
	if m.Nodes[0].Value != "-" {
		t.Fatal(m.Nodes)
	}
	d.Available["Zombies Killed"] = true
	d.ZombiesKilled = 1234
	m.Waiting = false
	m.Index = 25
	m.update(0, false, false, false)
	if m.Nodes[1].Value != "1,234" {
		t.Fatal(m.Nodes)
	}
}
