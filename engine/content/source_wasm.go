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
	value, err := fetchBytes(s.base + strings.Join(parts, "/"))
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(value)), nil
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
