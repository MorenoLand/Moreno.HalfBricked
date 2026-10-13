//go:build js && wasm

package platform

import (
	"fmt"
	"syscall/js"
)

func ReadProfile() (data []byte, err error) {
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
func WriteProfile(data []byte) (err error) {
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("profile storage: %v", value)
		}
	}()
	js.Global().Get("localStorage").Call("setItem", "halfbricked.profile", string(data))
	return nil
}
