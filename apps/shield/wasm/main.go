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
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return `{"error":"expected one string"}`
		}
		encoded, err := json.Marshal(session.Process(args[0].String()))
		if err != nil {
			return `{"error":"encoding failed"}`
		}
		return string(encoded)
	})
	expose("dmzProcessWithNames", func(args []js.Value) any {
		if len(args) != 2 || args[0].Type() != js.TypeString || args[1].Type() != js.TypeString {
			return `{"error":"invalid model request"}`
		}
		var names []surrogates.NameSpan
		if err := json.Unmarshal([]byte(args[1].String()), &names); err != nil {
			return `{"error":"invalid model spans"}`
		}
		encoded, err := json.Marshal(session.ProcessWithNames(args[0].String(), names))
		if err != nil {
			return `{"error":"encoding failed"}`
		}
		return string(encoded)
	})
	expose("dmzProcessBestEffort", func(args []js.Value) any {
		if len(args) != 2 || args[0].Type() != js.TypeString || args[1].Type() != js.TypeString {
			return `{"error":"invalid best effort request"}`
		}
		var names []surrogates.NameSpan
		if err := json.Unmarshal([]byte(args[1].String()), &names); err != nil {
			return `{"error":"invalid model spans"}`
		}
		encoded, err := json.Marshal(session.ProcessBestEffort(args[0].String(), names))
		if err != nil {
			return `{"error":"encoding failed"}`
		}
		return string(encoded)
	})
	expose("dmzRestore", func(args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return ""
		}
		return session.Restore(args[0].String())
	})
	expose("dmzReset", func(_ []js.Value) any { session.Reset(); return true })
	expose("dmzSetTenantTerms", func(args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return `{"error":"expected JSON array"}`
		}
		var terms []string
		if err := json.Unmarshal([]byte(args[0].String()), &terms); err != nil {
			return `{"error":"invalid JSON array"}`
		}
		if err := session.SetTenantTerms(terms); err != nil {
			return `{"error":"invalid tenant terms"}`
		}
		return `{"ok":true}`
	})
	select {}
}
