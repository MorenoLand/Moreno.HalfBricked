//go:build js && wasm

package content

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"syscall/js"
	"time"
)

type webSource struct{ base string }

func NewSource(root string) AssetSource {
	if root == "" {
		root = "data"
	}
	return webSource{base: strings.TrimRight(root, "/") + "/"}
}
func (s webSource) Open(path string) (io.ReadCloser, error) {
	parts := strings.Split(strings.ReplaceAll(path, "\\", "/"), "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	target := s.base + strings.Join(parts, "/")
	// A busy or rate-limited server answers 429 / 5xx (nginx limit_req gives 503): retry with a growing pause
	// instead of failing the level load. A missing file (404) is final.
	var value []byte
	var err error
	for attempt := 0; attempt < fetchAttempts; attempt++ {
		value, err = fetchBytes(target)
		if err == nil || !retryableFetch(err) {
			break
		}
		time.Sleep(time.Duration(250*(attempt+1)) * time.Millisecond)
	}
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
