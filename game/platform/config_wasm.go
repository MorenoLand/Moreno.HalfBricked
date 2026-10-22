//go:build js && wasm

package platform

import (
	"sort"
	"strings"
	"syscall/js"
)

const configKey = "halfbricked.cfg."

func storage() js.Value { return js.Global().Get("localStorage") }

func ConfigRead(name string) (data []byte, ok bool) {
	defer func() {
		if recover() != nil {
			data, ok = nil, false
		}
	}()
	value := storage().Call("getItem", configKey+name)
	if value.IsNull() || value.IsUndefined() {
		return nil, false
	}
	return []byte(value.String()), true
}

func ConfigWrite(name string, data []byte) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	storage().Call("setItem", configKey+name, string(data))
	return true
}

func ConfigDelete(name string) {
	defer func() { _ = recover() }()
	storage().Call("removeItem", configKey+name)
}

func ConfigList(prefix string) []string {
	defer func() { _ = recover() }()
	var names []string
	store := storage()
	for index := 0; index < store.Get("length").Int(); index++ {
		key := store.Call("key", index).String()
		if strings.HasPrefix(key, configKey+strings.TrimSuffix(prefix, "/")+"/") {
			names = append(names, strings.TrimPrefix(key, configKey))
		}
	}
	sort.Strings(names)
	return names
}
