//go:build js && wasm

package content

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/perfmark"
)

type webSource struct {
	base  string
	pf    *prefetcher
	sizes *sizeTable
}

// sizeTable holds the optional sizes.json (file path -> bytes) fetched in the background after start-up.
type sizeTable struct {
	mu    sync.Mutex
	bytes map[string]int64
}

// Size reports a file's size once sizes.json has arrived.
func (s webSource) Size(path string) (int64, bool) {
	s.sizes.mu.Lock()
	defer s.sizes.mu.Unlock()
	n, ok := s.sizes.bytes[path]
	return n, ok
}

// prefetcher fetches files in the background with a small, fixed number of requests in flight. Every Open used
// to be a blocking fetch on the game loop's goroutine, so each first sighting of a texture or sound froze a
// frame for a whole HTTP round trip (and for seconds when the server answered 503 and the retry pause ran).
// Open now takes a finished prefetch, or waits for the one in flight, before it ever starts its own request.
type prefetcher struct {
	mu      sync.Mutex
	data    map[string][]byte
	pending map[string]chan struct{}
	queue   []string
	workers int
	// Request budget. The server limits one client to about 10 requests per second after a short burst
	// (nginx limit_req, 503 beyond it), and answering 503 made every blocked request sleep through its retry
	// pause. Background fetches therefore spend tokens at a steady rate, so the file the game loop is waiting
	// for is never queued behind a rejected burst.
	tokens float64
	refill time.Time
}

const (
	prefetchWorkers = 4
	prefetchRate    = 7.0  // sustained background requests per second
	prefetchBurst   = 20.0 // tokens in a full bucket
	prefetchDebt    = -30.0
)

// take spends one token and returns how long the caller must wait before it may send its request (0 = now).
// urgent requests (the game loop is blocked on them) never wait; they only push the balance into debt so the
// background fetches slow down in turn.
func (pf *prefetcher) take(urgent bool) time.Duration {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	now := time.Now()
	if pf.refill.IsZero() {
		pf.tokens, pf.refill = prefetchBurst, now
	}
	pf.tokens = min(prefetchBurst, pf.tokens+now.Sub(pf.refill).Seconds()*prefetchRate)
	pf.refill = now
	if urgent {
		pf.tokens = max(prefetchDebt, pf.tokens-1)
		return 0
	}
	if pf.tokens >= 1 {
		pf.tokens--
		return 0
	}
	wait := time.Duration((1 - pf.tokens) / prefetchRate * float64(time.Second))
	pf.tokens--
	return wait
}

func NewSource(root string) AssetSource {
	if root == "" {
		root = "data"
	}
	if strings.HasPrefix(root, "/") {
		// An absolute path is the in-memory file system (web/memfs.js) holding the cache the page built from the
		// player's own APK; everything else is a URL below the page (the hosted data).
		return fileSource{root: root}
	}
	source := webSource{base: strings.TrimRight(root, "/") + "/", pf: &prefetcher{data: map[string][]byte{}, pending: map[string]chan struct{}{}}, sizes: &sizeTable{}}
	go func() {
		// Optional: without sizes.json the loading screen counts files only.
		value, err := fetchBytes(source.target("sizes.json"))
		if err != nil {
			return
		}
		table := map[string]int64{}
		if json.Unmarshal(value, &table) != nil {
			return
		}
		source.sizes.mu.Lock()
		source.sizes.bytes = table
		source.sizes.mu.Unlock()
	}()
	return source
}

func (s webSource) target(path string) string {
	parts := strings.Split(strings.ReplaceAll(path, "\\", "/"), "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return s.base + strings.Join(parts, "/")
}

// fetchRetry: a busy or rate-limited server answers 429 / 5xx (nginx limit_req gives 503): retry with a growing
// pause instead of failing the level load. A missing file (404) is final.
func fetchRetry(target string) ([]byte, error) {
	var value []byte
	var err error
	for attempt := 0; attempt < fetchAttempts; attempt++ {
		value, err = fetchBytes(target)
		if err == nil || !retryableFetch(err) {
			break
		}
		time.Sleep(time.Duration(250*(attempt+1)) * time.Millisecond)
	}
	return value, err
}

// Prefetch queues files for background download (content.Prefetcher); paths already held or in flight are skipped.
func (s webSource) Prefetch(paths []string) {
	pf := s.pf
	pf.mu.Lock()
	for _, path := range paths {
		if _, ok := pf.data[path]; ok {
			continue
		}
		if _, ok := pf.pending[path]; ok {
			continue
		}
		pf.pending[path] = make(chan struct{})
		pf.queue = append(pf.queue, path)
	}
	for pf.workers < prefetchWorkers && len(pf.queue) > 0 {
		pf.workers++
		go s.prefetchWorker()
	}
	pf.mu.Unlock()
}

// Pending counts the paths still queued or downloading.
func (s webSource) Pending(paths []string) int {
	s.pf.mu.Lock()
	defer s.pf.mu.Unlock()
	count := 0
	for _, path := range paths {
		if _, ok := s.pf.pending[path]; ok {
			count++
		}
	}
	return count
}

// Ready reports whether a prefetched file has arrived and not been taken yet.
func (s webSource) Ready(path string) bool {
	s.pf.mu.Lock()
	defer s.pf.mu.Unlock()
	_, ok := s.pf.data[path]
	return ok
}

func (s webSource) prefetchWorker() {
	pf := s.pf
	for {
		pf.mu.Lock()
		if len(pf.queue) == 0 {
			pf.workers--
			pf.mu.Unlock()
			return
		}
		path := pf.queue[0]
		pf.queue = pf.queue[1:]
		done := pf.pending[path]
		pf.mu.Unlock()
		if wait := pf.take(false); wait > 0 {
			time.Sleep(wait)
		}
		value, err := fetchRetry(s.target(path))
		pf.mu.Lock()
		if err == nil {
			pf.data[path] = value
		}
		delete(pf.pending, path)
		pf.mu.Unlock()
		if done != nil {
			close(done)
		}
	}
}

func (s webSource) Open(path string) (io.ReadCloser, error) {
	pf := s.pf
	pf.mu.Lock()
	if value, ok := pf.data[path]; ok {
		delete(pf.data, path) // one reader per file keeps the cache from doubling the pack in memory
		pf.mu.Unlock()
		return io.NopCloser(bytes.NewReader(value)), nil
	}
	done, inflight := pf.pending[path]
	// Still waiting in the queue (no worker has it): take it out and fetch it now instead of letting the caller
	// sit behind the rest of the queue.
	queued := false
	if inflight {
		for i, queuedPath := range pf.queue {
			if queuedPath == path {
				pf.queue = append(pf.queue[:i:i], pf.queue[i+1:]...)
				queued = true
				break
			}
		}
	}
	pf.mu.Unlock()
	// From here the caller (usually the game loop) is blocked on the network; report how long.
	waitStart := time.Now()
	defer perfmark.Since("fetch "+path, waitStart)
	if inflight && !queued {
		<-done
		pf.mu.Lock()
		value, ok := pf.data[path]
		delete(pf.data, path)
		pf.mu.Unlock()
		if ok {
			return io.NopCloser(bytes.NewReader(value)), nil
		}
	}
	pf.take(true)
	value, err := fetchRetry(s.target(path))
	if queued {
		pf.mu.Lock()
		delete(pf.pending, path)
		pf.mu.Unlock()
		close(done)
	}
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(value)), nil
}

const fetchAttempts = 6

func retryableFetch(err error) bool {
	message := err.Error()
	for _, code := range []string{"HTTP 429", "HTTP 502", "HTTP 503", "HTTP 504"} {
		if strings.Contains(message, code) {
			return true
		}
	}
	return strings.Contains(message, "failed:") // network error from fetch itself
}
func (s webSource) Manifest() (PackManifest, error) {
	r, err := s.Open("pack.json")
	if err != nil {
		return PackManifest{}, err
	}
	defer r.Close()
	var manifest PackManifest
	err = json.NewDecoder(r).Decode(&manifest)
	return manifest, err
}
func fetchBytes(rawURL string) ([]byte, error) {
	result := make(chan struct {
		data []byte
		err  error
	}, 1)
	fetch := js.Global().Get("fetch").Invoke(rawURL)
	then := js.FuncOf(func(this js.Value, args []js.Value) any {
		response := args[0]
		if !response.Get("ok").Bool() {
			result <- struct {
				data []byte
				err  error
			}{err: fmt.Errorf("fetch %s: HTTP %d", rawURL, response.Get("status").Int())}
			return nil
		}
		bufferPromise := response.Call("arrayBuffer")
		bufferThen := js.FuncOf(func(this js.Value, args []js.Value) any {
			buffer := args[0]
			view := js.Global().Get("Uint8Array").New(buffer)
			data := make([]byte, view.Get("length").Int())
			js.CopyBytesToGo(data, view)
			result <- struct {
				data []byte
				err  error
			}{data: data}
			return nil
		})
		bufferPromise.Call("then", bufferThen)
		return nil
	})
	catch := js.FuncOf(func(this js.Value, args []js.Value) any {
		result <- struct {
			data []byte
			err  error
		}{err: fmt.Errorf("fetch %s failed: %v", rawURL, args[0])}
		return nil
	})
	fetch.Call("then", then).Call("catch", catch)
	value := <-result
	then.Release()
	catch.Release()
	return value.data, value.err
}
