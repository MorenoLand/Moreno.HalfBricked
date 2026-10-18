//go:build js && wasm

package platform

import (
	"sync/atomic"
	"syscall/js"
	"time"
)

// Browser clipboard. Reading is only allowed inside a real user event (Firefox shows its own "Paste" prompt, Chrome
// asks once), and the game's frame loop is not one, so the game arms two capture-phase listeners (ClipboardArm): a
// key press of Ctrl+V / Cmd+V / Shift+Insert and a pointer press while the pointer is on the PASTE button both read
// the clipboard inside the event itself. ClipboardRequest (called from the frame loop) stays as a fallback for
// browsers that accept it. Writing falls back to a hidden text area and execCommand("copy").

var (
	armPaste  atomic.Bool // the pointer is on the PASTE button
	armKeys   atomic.Bool // Ctrl+V / Shift+Insert should paste (a page that takes pasted text is open)
	lastRead  atomic.Int64
	denied    atomic.Bool
	listeners bool
)

// ClipboardArm tells the listeners what a press means right now. Call it every frame while a page that takes pasted
// text is open (and once with false, false when it closes).
func ClipboardArm(overPasteButton, keys bool) {
	armPaste.Store(overPasteButton)
	armKeys.Store(keys)
	if !listeners {
		listeners = true
		installClipboardListeners()
	}
}

// ClipboardDenied reports that the browser refused the last read (the page can then say to use Ctrl+V).
func ClipboardDenied() bool { return denied.Load() }

func installClipboardListeners() {
	defer func() { _ = recover() }()
	window := js.Global()
	pointer := js.FuncOf(func(this js.Value, args []js.Value) any {
		if armPaste.Load() {
			readClipboard()
		}
		return nil
	})
	key := js.FuncOf(func(this js.Value, args []js.Value) any {
		if !armKeys.Load() || len(args) == 0 {
			return nil
		}
		e := args[0]
		letter := e.Get("key").String()
		paste := (e.Get("ctrlKey").Bool() || e.Get("metaKey").Bool()) && (letter == "v" || letter == "V")
		insert := e.Get("shiftKey").Bool() && e.Get("key").String() == "Insert"
		if paste || insert {
			readClipboard()
		}
		return nil
	})
	window.Call("addEventListener", "pointerdown", pointer, true)
	window.Call("addEventListener", "keydown", key, true)
}

func readClipboard() {
	defer func() { _ = recover() }()
	board := js.Global().Get("navigator").Get("clipboard")
	if !board.Truthy() || board.Get("readText").IsUndefined() {
		denied.Store(true)
		return
	}
	lastRead.Store(time.Now().UnixMilli())
	var then, failed js.Func
	then = js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 0 {
			denied.Store(false)
			clipboardResult.set(args[0].String())
		}
		then.Release()
		failed.Release()
		return nil
	})
	failed = js.FuncOf(func(this js.Value, args []js.Value) any {
		denied.Store(true)
		then.Release()
		failed.Release()
		return nil
	})
	board.Call("readText").Call("then", then, failed)
}

// ClipboardRequest is the frame-loop fallback: skipped when an event handler has just read.
func ClipboardRequest() {
	if time.Now().UnixMilli()-lastRead.Load() < 1500 {
		return
	}
	readClipboard()
}

func ClipboardWrite(text string) {
	defer func() { _ = recover() }()
	board := js.Global().Get("navigator").Get("clipboard")
	if board.Truthy() && !board.Get("writeText").IsUndefined() {
		var failed js.Func
		failed = js.FuncOf(func(this js.Value, args []js.Value) any {
			copyWithTextArea(text)
			failed.Release()
			return nil
		})
		board.Call("writeText", text).Call("catch", failed)
		return
	}
	copyWithTextArea(text)
}

func copyWithTextArea(text string) {
	defer func() { _ = recover() }()
	document := js.Global().Get("document")
	area := document.Call("createElement", "textarea")
	area.Set("value", text)
	area.Get("style").Set("position", "fixed")
	area.Get("style").Set("opacity", "0")
	document.Get("body").Call("appendChild", area)
	area.Call("focus")
	area.Call("select")
	document.Call("execCommand", "copy")
	document.Get("body").Call("removeChild", area)
}
