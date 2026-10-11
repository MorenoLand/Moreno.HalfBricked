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
)

type webSource struct {
	base string
	pf   *prefetcher
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
}

const prefetchWorkers = 4

func NewSource(root string) AssetSource {
	if root == "" {
		root = "data"
	}
	return webSource{base: strings.TrimRight(root, "/") + "/", pf: &prefetcher{data: map[string][]byte{}, pending: map[string]chan struct{}{}}}
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
	pf.mu.Unlock()
	if inflight {
		<-done
		pf.mu.Lock()
		value, ok := pf.data[path]
		delete(pf.data, path)
		pf.mu.Unlock()
		if ok {
			return io.NopCloser(bytes.NewReader(value)), nil
		}
	}
	value, err := fetchRetry(s.target(path))
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(value)), nil
}

const fetchAttempts = 6

func retryableFetch(err error) bool {
	message := err.Error()
	for _, code := range []string{"HTTP 429", "HTTP 500", "HTTP 502", "HTTP 503", "HTTP 504"} {
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
