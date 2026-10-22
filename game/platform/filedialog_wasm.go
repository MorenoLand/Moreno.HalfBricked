//go:build js && wasm

package platform

import (
	"errors"
	"syscall/js"
)

// OpenFileDialog is not available in the browser (a page cannot open a file without a user gesture handler).
func OpenFileDialog(title, desc, pattern string) (string, error) {
	return "", errors.New("opening a file is not supported in the browser")
}

// SaveFileAs hands the file to the browser as a download (the name is the file's default name).
func SaveFileAs(title, defaultName, desc, pattern string, data []byte) (string, error) {
	defer func() { _ = recover() }()
	array := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(array, data)
	blob := js.Global().Get("Blob").New([]any{array}, map[string]any{"type": "application/json"})
	url := js.Global().Get("URL").Call("createObjectURL", blob)
	document := js.Global().Get("document")
	link := document.Call("createElement", "a")
	link.Set("href", url)
	link.Set("download", defaultName)
	document.Get("body").Call("appendChild", link)
	link.Call("click")
	link.Call("remove")
	return "(downloaded " + defaultName + ")", nil
}
