package game

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"sort"
	"testing"
	"time"
)

func TestPerfProbe(t *testing.T) {
	level := os.Getenv("AOZ_PERF_LEVEL")
	if level == "" {
		t.Skip("set AOZ_PERF_LEVEL")
	}
	root := "bin/data-cache"
	a := newHeadlessApp(t, root)
	a.resultsScreen, a.page = nil, 2
	if err := a.selectCaptureLevel(level); err != nil {
		t.Fatal(err)
	}
	if err := a.openPlay(); err != nil {
		t.Fatal(err)
	}
	p := a.play
	p.cheats.god = true
	p.cheats.infiniteAmmo = true
	bot := &ptBot{a: a}
	a.inputHook = bot.input
	f, _ := os.Create(os.Getenv("AOZ_PERF_PROF"))
	if f != nil {
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	var times []time.Duration
	var ms0 runtime.MemStats
	runtime.ReadMemStats(&ms0)
	for frame := 0; frame < 60*90; frame++ {
		if a.play == nil {
			break
		}
		p = a.play
		if p.dialogueIndex >= 0 && p.dialogueIndex < len(p.dialogue) && frame%25 == 0 {
			p.dialogueIndex++
			p.dialogueAge = 0
		}
		start := time.Now()
		if err := a.Update(); err != nil {
			t.Fatal(err)
		}
		times = append(times, time.Since(start))
	}
	var ms1 runtime.MemStats
	runtime.ReadMemStats(&ms1)
	fmt.Printf("alloc per frame %d bytes, %d mallocs, GCs %d\n", (ms1.TotalAlloc-ms0.TotalAlloc)/uint64(len(times)), (ms1.Mallocs-ms0.Mallocs)/uint64(len(times)), ms1.NumGC-ms0.NumGC)
	sorted := append([]time.Duration(nil), times...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	n := len(sorted)
	if n == 0 {
		return
	}
	var total time.Duration
	for _, d := range times {
		total += d
	}
	zombies := 0
	if a.play != nil {
		zombies = len(a.play.zombies)
	}
	fmt.Printf("%s frames %d avg %v p50 %v p99 %v max %v zombies-now %d\n", level, n, total/time.Duration(n), sorted[n/2], sorted[n*99/100], sorted[n-1], zombies)
	over := 0
	for _, d := range times {
		if d > 8*time.Millisecond {
			over++
		}
	}
	fmt.Printf("frames over 8 ms: %d\n", over)
}
