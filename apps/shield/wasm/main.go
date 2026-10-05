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
	// Test the original JS value before .String() converts it to UTF-8. That
	// conversion replaces lone UTF-16 surrogates and would lose original data.
	unpaired := js.Global().Get("RegExp").New(`[\uD800-\uDFFF]`, "u")
	validText := func(value js.Value) bool { return !unpaired.Call("test", value).Bool() }
	expose("dmzProcess", func(args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return `{"error":"expected one string"}`
		}
		if !validText(args[0]) {
			return `{"error":"invalid Unicode input"}`
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
		if !validText(args[0]) {
			return `{"error":"invalid Unicode input"}`
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
		if !validText(args[0]) {
			return `{"error":"invalid Unicode input"}`
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
		if !validText(args[0]) {
			return args[0] // Lossless pass-through; no guessed restoration.
		}
		return session.Restore(args[0].String())
	})
	expose("dmzReset", func(_ []js.Value) any { session.Reset(); return true })
	expose("dmzSetTenantTerms", func(args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return `{"error":"expected JSON array"}`
		}
		if !validText(args[0]) {
			return `{"error":"invalid tenant terms"}`
		}
		var terms []string
		if err := json.Unmarshal([]byte(args[0].String()), &terms); err != nil {
			return `{"error":"invalid JSON array"}`
		}
		if len(terms) > 32 {
			return `{"error":"invalid tenant terms"}`
		}
		// JSON escapes can contain lone surrogates even when the JSON source is
		// ASCII. Inspect parsed JS values before Go's decoder replacement is used.
		if len(terms) > 0 {
			values := js.Global().Get("JSON").Call("parse", args[0])
			for i := range terms {
				if !validText(values.Index(i)) {
					return `{"error":"invalid tenant terms"}`
				}
			}
		}
		if err := session.SetTenantTerms(terms); err != nil {
			return `{"error":"invalid tenant terms"}`
		}
		return `{"ok":true}`
	})
	select {}
}
