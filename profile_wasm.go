//go:build js && wasm

package main

import (
	"fmt"
	"syscall/js"
)

func readPlayerProfile() (data []byte, err error) {
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("profile storage: %v", value)
		}
	}()
	value := js.Global().Get("localStorage").Call("getItem", "halfbricked.profile")
	if value.IsNull() || value.IsUndefined() {
		return nil, nil
	}
	return []byte(value.String()), nil
}
func writePlayerProfile(data []byte) (err error) {
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("profile storage: %v", value)
		}
	}()
	js.Global().Get("localStorage").Call("setItem", "halfbricked.profile", string(data))
	return nil
}
