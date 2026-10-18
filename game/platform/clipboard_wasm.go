//go:build js && wasm

package platform

import (
	"strings"
	"sync/atomic"
	"syscall/js"
	"time"
)

// Browser clipboard.
//
// Reading the clipboard from a page is the part browsers guard: Chrome asks once for permission, Safari shows a "Paste"
// callout, Firefox shows a "Paste" bubble that has to be clicked, and the game's canvas swallows Ctrl+V (Ebitengine
// prevents the default of every key press), so the browser never sends a paste event. What works everywhere is a real
// text field: the player pastes into it with the normal shortcut or menu and the browser hands the text over without
// any prompt. So:
//
//   - PASTE button or Ctrl+V (armed through ClipboardArm so the press is seen inside the real event): Chrome-like
//     browsers read the clipboard directly; if that is refused, or the browser is Firefox, a small paste box opens
//     over the page (showPasteBox) and takes the text from there.
//   - Writing uses the clipboard API and falls back to a hidden text area and execCommand("copy").

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
	if !keys {
		removePasteBox()
	}
}

// ClipboardDenied reports that the browser gave no way to paste at all (not even the paste box could open).
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
		insert := e.Get("shiftKey").Bool() && letter == "Insert"
		if paste || insert {
			readClipboard()
		}
		return nil
	})
	window.Call("addEventListener", "pointerdown", pointer, true)
	window.Call("addEventListener", "keydown", key, true)
}

func isFirefox() bool {
	return strings.Contains(js.Global().Get("navigator").Get("userAgent").String(), "Firefox")
}

// readClipboard tries the browser's own read and falls back to the paste box.
func readClipboard() {
	defer func() { _ = recover() }()
	board := js.Global().Get("navigator").Get("clipboard")
	if isFirefox() || !board.Truthy() || board.Get("readText").IsUndefined() {
		showPasteBox()
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
		showPasteBox()
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

var pasteBoxFuncs []js.Func

func removePasteBox() {
	defer func() { _ = recover() }()
	if box := js.Global().Get("document").Call("getElementById", "aoz-paste"); box.Truthy() {
		box.Call("remove")
	}
	for _, f := range pasteBoxFuncs {
		f.Release()
	}
	pasteBoxFuncs = nil
}

// showPasteBox opens a text field over the page: paste into it (Ctrl+V or the menu) and the text is taken; Esc or a
// click outside closes it.
func showPasteBox() {
	defer func() {
		if recover() != nil {
			denied.Store(true)
		}
	}()
	document := js.Global().Get("document")
	if existing := document.Call("getElementById", "aoz-paste-input"); existing.Truthy() {
		existing.Call("focus")
		return
	}
	box := document.Call("createElement", "div")
	box.Set("id", "aoz-paste")
	box.Get("style").Set("cssText", "position:fixed;left:50%;bottom:14%;transform:translateX(-50%);z-index:100002;background:#0b0e16;color:#d8dbe5;"+
		"font:15px system-ui,sans-serif;padding:14px 18px;border:1px solid #c53d2b;border-radius:8px;text-align:center;box-shadow:0 6px 24px rgba(0,0,0,.6)")
	label := document.Call("createElement", "div")
	label.Set("textContent", "Paste the code here (Ctrl+V), it is taken at once. Esc cancels.")
	label.Get("style").Set("marginBottom", "8px")
	input := document.Call("createElement", "input")
	input.Set("id", "aoz-paste-input")
	input.Set("type", "text")
	input.Set("placeholder", "paste here")
	input.Get("style").Set("cssText", "width:min(420px,70vw);padding:6px 8px;font:14px ui-monospace,Consolas,monospace;background:#1b2030;color:#fff;border:1px solid #444;border-radius:4px")
	var onPaste, onInput, onKey js.Func
	finish := func(text string) {
		text = strings.TrimSpace(text)
		if text != "" {
			denied.Store(false)
			clipboardResult.set(text)
		}
		go removePasteBox() // after this handler has returned (it releases the functions it runs from)
	}
	onPaste = js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 0 {
			if data := args[0].Get("clipboardData"); data.Truthy() {
				finish(data.Call("getData", "text").String())
				args[0].Call("preventDefault")
			}
		}
		return nil
	})
	onInput = js.FuncOf(func(this js.Value, args []js.Value) any { // a paste from a menu, or typed in
		if value := input.Get("value").String(); len(value) > 24 {
			finish(value)
		}
		return nil
	})
	onKey = js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 0 && args[0].Get("key").String() == "Escape" {
			go removePasteBox()
		}
		return nil
	})
	pasteBoxFuncs = []js.Func{onPaste, onInput, onKey}
	input.Call("addEventListener", "paste", onPaste)
	input.Call("addEventListener", "input", onInput)
	input.Call("addEventListener", "keydown", onKey)
	box.Call("appendChild", label)
	box.Call("appendChild", input)
	document.Get("body").Call("appendChild", box)
	input.Call("focus")
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
