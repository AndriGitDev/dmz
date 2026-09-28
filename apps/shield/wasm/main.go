//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/AndriGitDev/dmz/packages/surrogates"
)

var callbacks []js.Func

func expose(name string, fn func([]js.Value) any) {
	callback := js.FuncOf(func(_ js.Value, args []js.Value) any { return fn(args) })
	callbacks = append(callbacks, callback)
	js.Global().Set(name, callback)
}

func main() {
	session := surrogates.NewSession()
	expose("dmzProcess", func(args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString { return `{"error":"expected one string"}` }
		encoded, err := json.Marshal(session.Process(args[0].String()))
		if err != nil { return `{"error":"encoding failed"}` }
		return string(encoded)
	})
	expose("dmzRestore", func(args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString { return "" }
		return session.Restore(args[0].String())
	})
	expose("dmzReset", func(_ []js.Value) any { session.Reset(); return true })
	select {}
}
