// Package perfmark lets slow, blocking work (a download waited on by the game loop, a PNG or Ogg decode) report
// itself to whatever is watching the frame rate. The browser page's ?perf panel installs the hook; everywhere
// else the hook is nil and Since costs one comparison.
package perfmark

import "time"

// Hook receives the label and duration of each operation at least Threshold long.
var Hook func(label string, d time.Duration)

// Threshold is the shortest duration worth reporting.
const Threshold = 8 * time.Millisecond

// Since reports the work that began at start under label when a hook is installed and it took long enough.
func Since(label string, start time.Time) {
	if Hook == nil {
		return
	}
	if d := time.Since(start); d >= Threshold {
		Hook(label, d)
	}
}
