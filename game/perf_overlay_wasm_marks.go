//go:build js && wasm

package game

import (
	"runtime"
	"syscall/js"
	"time"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/perfmark"
)

// With ?perf the page's own panel (index.html) lists every blocking operation the game reports through perfmark,
// next to the browser's long-task and frame-gap measurements, so a freeze can be matched to what the game was doing.
func init() {
	if !perfOverlay {
		return
	}
	perfmark.Hook = func(label string, d time.Duration) {
		defer func() { _ = recover() }()
		if mark := js.Global().Get("aozMark"); mark.Type() == js.TypeFunction {
			mark.Invoke(label, float64(d.Microseconds())/1000)
		}
	}
	go func() {
		var last runtime.MemStats
		for {
			time.Sleep(time.Second)
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			if pause := time.Duration(ms.PauseTotalNs - last.PauseTotalNs); pause >= perfmark.Threshold {
				perfmark.Hook("garbage collection (last second, "+itoa(int(ms.NumGC-last.NumGC))+" runs)", pause)
			}
			last = ms
		}
	}()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [12]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}
