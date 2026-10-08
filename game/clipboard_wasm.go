//go:build js && wasm

package game

import "syscall/js"

func clipboardWrite(text string) {
	defer func() { _ = recover() }()
	if board := js.Global().Get("navigator").Get("clipboard"); board.Truthy() {
		board.Call("writeText", text)
	}
}

func clipboardRequest() {
	defer func() { _ = recover() }()
	board := js.Global().Get("navigator").Get("clipboard")
	if !board.Truthy() {
		return
	}
	var then js.Func
	then = js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 0 {
			clipboardResult.set(args[0].String())
		}
		then.Release()
		return nil
	})
	board.Call("readText").Call("then", then)
}
