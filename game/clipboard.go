package game

import "sync"

type clipboardBox struct {
	mu    sync.Mutex
	text  string
	ready bool
}

func (c *clipboardBox) set(text string) {
	c.mu.Lock()
	c.text, c.ready = text, true
	c.mu.Unlock()
}

var clipboardResult clipboardBox

// clipboardTake returns text from a finished clipboardRequest, once.
func clipboardTake() (string, bool) {
	clipboardResult.mu.Lock()
	defer clipboardResult.mu.Unlock()
	text, ok := clipboardResult.text, clipboardResult.ready
	clipboardResult.text, clipboardResult.ready = "", false
	return text, ok
}
