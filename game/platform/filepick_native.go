//go:build !js

package platform

import "os"

// PickFileAsync opens the system file chooser without stopping the game; PickedFile returns the chosen file.
func PickFileAsync(title, desc, pattern string) {
	picked.mu.Lock()
	if picked.busy {
		picked.mu.Unlock()
		return
	}
	picked.busy = true
	picked.mu.Unlock()
	go func() {
		path, err := OpenFileDialog(title, desc, pattern)
		if err != nil || path == "" {
			picked.release()
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			picked.release()
			return
		}
		picked.set(path, data)
	}()
}

// FilePickArm is the browser build's hook; the desktop build opens the chooser at once.
func FilePickArm(x0, y0, x1, y1 float64) {}
