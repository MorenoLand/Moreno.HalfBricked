//go:build !js

package game

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Desktop: starting a level used to decode every Common0 and world texture and every sound effect on the one frame
// that starts it (warmTextures), which is the pause after pressing Play. The same work is now done early and in
// parallel: as soon as a level is selected, worker goroutines decode the PNGs and Ogg sounds the level will use
// while the player reads the level screen; the game loop only uploads the finished textures to the graphics card, a
// few per frame (stepPreload), and the slide-out of the level screen after Play gives it more time. When the level
// then starts, warmTextures finds everything in place. Nothing here changes what is loaded, only when and on which
// thread.

type levelPreload struct {
	mu     sync.Mutex
	world  string
	images map[string]image.Image // texture path -> decoded PNG, until Texture takes it
	sounds map[string][]byte      // sound path -> decoded PCM, until the game loop stores it
	names  []string               // the world's texture names still to upload
	next   int
}

// preloadSelectedLevel is called every frame the level screen is up: it starts the workers when the selected level is
// in a world not started yet, and spends a little of the frame on uploads.
func (a *app) preloadSelectedLevel() {
	if a.silent || a.pack == nil {
		return
	}
	levels := a.filteredLevels()
	if a.level < 0 || a.level >= len(levels) {
		return
	}
	world := ""
	if parts := strings.Split(filepath.ToSlash(levels[a.level].SourceXML), "/"); len(parts) > 0 {
		world = strings.ToLower(parts[0])
	}
	if a.preload.world != world {
		a.startPreload(world)
	}
	a.stepPreload(3 * time.Millisecond)
}

func (a *app) startPreload(world string) {
	p := &a.preload
	p.mu.Lock()
	p.world = world
	if p.images == nil {
		p.images = map[string]image.Image{}
		p.sounds = map[string][]byte{}
	}
	p.mu.Unlock()

	// What this level shows: the same filter warmTextures uses. The list is made here, on the game loop's goroutine,
	// because it reads the texture maps the workers must not touch.
	type job struct{ name, path string }
	var jobs []job
	var names []string
	for name, path := range a.pack.TexturePaths() {
		lower := strings.ToLower(filepath.ToSlash(path))
		if !(strings.Contains(lower, "/common0/") || (world != "" && strings.Contains(lower, "/"+world+"/"))) {
			continue
		}
		names = append(names, name)
		if _, loaded := a.imageByPath[path]; !loaded {
			jobs = append(jobs, job{name, path})
		}
	}
	sort.Strings(names)
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].path < jobs[j].path })
	p.mu.Lock()
	p.names, p.next = names, 0
	p.mu.Unlock()

	var soundPaths []string
	if a.sound != nil {
		for _, path := range a.pack.SFXPaths() {
			if !a.sound.HasShort(path) {
				soundPaths = append(soundPaths, path)
			}
		}
	}
	workers := runtime.NumCPU() - 1
	if workers < 1 {
		workers = 1
	}
	if workers > 6 {
		workers = 6
	}
	pack, sound := a.pack, a.sound
	textureJobs := make(chan job, len(jobs))
	for _, j := range jobs {
		textureJobs <- j
	}
	close(textureJobs)
	soundJobs := make(chan string, len(soundPaths))
	for _, path := range soundPaths {
		soundJobs <- path
	}
	close(soundJobs)
	for w := 0; w < workers; w++ {
		go func() {
			for j := range textureJobs {
				reader, err := pack.Open(j.path)
				if err != nil {
					continue
				}
				data, err := io.ReadAll(reader)
				reader.Close()
				if err != nil {
					continue
				}
				img, err := png.Decode(bytes.NewReader(data))
				if err != nil {
					continue
				}
				p.mu.Lock()
				if p.world == world { // a world the player has moved on from is not wanted any more
					p.images[j.path] = img
				}
				p.mu.Unlock()
			}
			for path := range soundJobs {
				if pcm, ok := sound.DecodeShort(path); ok {
					p.mu.Lock()
					p.sounds[path] = pcm
					p.mu.Unlock()
				}
			}
		}()
	}
}

// takePredecoded hands over (once) a texture a worker has decoded.
func (a *app) takePredecoded(path string) image.Image {
	p := &a.preload
	p.mu.Lock()
	defer p.mu.Unlock()
	img := p.images[path]
	delete(p.images, path)
	return img
}

// stepPreload spends up to budget on the game loop's share of the work: storing decoded sounds and uploading decoded
// textures (Texture takes them from the workers' results). With a zero budget it only stores the sounds.
func (a *app) stepPreload(budget time.Duration) {
	p := &a.preload
	deadline := time.Now().Add(budget)
	p.mu.Lock()
	sounds := p.sounds
	p.sounds = map[string][]byte{}
	p.mu.Unlock()
	if a.sound != nil {
		for path, pcm := range sounds {
			a.sound.StoreShort(path, pcm)
		}
	}
	for time.Now().Before(deadline) {
		p.mu.Lock()
		if p.next >= len(p.names) {
			p.mu.Unlock()
			return
		}
		name := p.names[p.next]
		path, _ := a.pack.TexturePath(name)
		_, decoded := p.images[path]
		if _, loaded := a.imageByPath[path]; !decoded && !loaded {
			p.mu.Unlock() // a worker has not finished it: wait for it rather than decode it on this frame
			return
		}
		p.next++
		p.mu.Unlock()
		_, _ = a.Texture(name)
	}
}
