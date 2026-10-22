//go:build js && wasm

package platform

import "syscall/js"

// A page cannot open a file chooser from a frame callback, only from a real click. So the game tells this file which part
// of the canvas (as a share of its width and height) is the "open file" control, and a real pointer press inside it opens
// the chooser. The chosen file is read in the browser and handed over through PickedFile.

var (
	pickRegion    [4]float64
	pickArmed     bool
	pickInstalled bool
)

// PickFileAsync does nothing in the browser (the armed click opens the chooser); it exists so the game code is the same.
func PickFileAsync(title, desc, pattern string) {}

// FilePickArm marks the canvas region (0..1 on both axes) where a click opens the file chooser; call it every frame the
// control is visible, with an empty region when it is not.
func FilePickArm(x0, y0, x1, y1 float64) {
	pickRegion = [4]float64{x0, y0, x1, y1}
	pickArmed = x1 > x0 && y1 > y0
	if pickArmed && !pickInstalled {
		pickInstalled = true
		installFilePicker()
	}
}

func installFilePicker() {
	defer func() { _ = recover() }()
	window := js.Global()
	document := window.Get("document")
	input := document.Call("createElement", "input")
	input.Set("type", "file")
	input.Set("accept", ".json,application/json")
	input.Get("style").Set("display", "none")
	document.Get("body").Call("appendChild", input)
	change := js.FuncOf(func(this js.Value, args []js.Value) any {
		files := input.Get("files")
		if files.Length() == 0 {
			return nil
		}
		file := files.Index(0)
		name := file.Get("name").String()
		reader := window.Get("FileReader").New()
		load := js.FuncOf(func(this js.Value, args []js.Value) any {
			buffer := window.Get("Uint8Array").New(reader.Get("result"))
			data := make([]byte, buffer.Get("length").Int())
			js.CopyBytesToGo(data, buffer)
			picked.set(name, data)
			return nil
		})
		reader.Set("onload", load)
		reader.Call("readAsArrayBuffer", file)
		input.Set("value", "")
		return nil
	})
	input.Call("addEventListener", "change", change)
	press := js.FuncOf(func(this js.Value, args []js.Value) any {
		if !pickArmed || len(args) == 0 {
			return nil
		}
		event := args[0]
		canvas := document.Call("querySelector", "canvas")
		if !canvas.Truthy() {
			return nil
		}
		rect := canvas.Call("getBoundingClientRect")
		x := (event.Get("clientX").Float() - rect.Get("left").Float()) / rect.Get("width").Float()
		y := (event.Get("clientY").Float() - rect.Get("top").Float()) / rect.Get("height").Float()
		if x >= pickRegion[0] && x < pickRegion[2] && y >= pickRegion[1] && y < pickRegion[3] {
			input.Call("click")
		}
		return nil
	})
	window.Call("addEventListener", "pointerdown", press, true)
}
