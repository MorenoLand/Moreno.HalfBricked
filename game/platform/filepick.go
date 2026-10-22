package platform

import "sync"

// A file the player picked, waiting to be taken. The desktop dialog runs on its own goroutine and the browser's file chooser
// answers later, so the game polls for the result.

type pickedBox struct {
	mu    sync.Mutex
	name  string
	data  []byte
	ready bool
	busy  bool
}

var picked pickedBox

func (b *pickedBox) set(name string, data []byte) {
	b.mu.Lock()
	b.name, b.data, b.ready, b.busy = name, data, true, false
	b.mu.Unlock()
}

func (b *pickedBox) release() {
	b.mu.Lock()
	b.busy = false
	b.mu.Unlock()
}

// PickedFile returns the file chosen by PickFileAsync (or the browser's armed chooser), once.
func PickedFile() (name string, data []byte, ok bool) {
	picked.mu.Lock()
	defer picked.mu.Unlock()
	name, data, ok = picked.name, picked.data, picked.ready
	picked.name, picked.data, picked.ready = "", nil, false
	return
}
