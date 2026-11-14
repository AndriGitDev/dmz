package routing

import (
	"bytes"
)

// MergeCodexSettings removes the Codex route dmz still owns from a config the
// user or Codex edited after installation. original is the pre-install file
// (nil when none existed), written is the exact TOML CodexSettings produced
// (including journals written by earlier re-encoding versions) and current is
// the file as it is now.
//
// Only two values are owned: root.model_provider and
// model_providers.dmz_background. Each is restored (or removed) only while it
// still equals the written value; a value the user changed is kept. Edits are
// made in place on current, so unrelated bytes, comments and formatting stay.
// The edited text is decoded again and must equal the intended values; a layout
// that cannot be edited that way returns ErrChanged and changes nothing. The
// result is nil only when no file existed before and nothing but whitespace
// remains, so the caller can delete the generated file. When no owned value
// remains, current is returned unchanged.
func MergeCodexSettings(original, written, current []byte) ([]byte, error) {
	if len(original) > maxConfig || len(written) > maxConfig || len(current) > maxConfig || written == nil || current == nil {
		return nil, ErrUnavailable
	}
	oldRoot := map[string]any{}
	ok := true
	if original != nil {
		if oldRoot, ok = decodeTOML(original); !ok {
			return nil, ErrUnavailable
		}
	}
	appliedRoot, ok := decodeTOML(written)
	if !ok {
		return nil, ErrUnavailable
	}
	root, ok := decodeTOML(current)
	if !ok {
		return nil, ErrUnavailable
	}

	appliedDefault, ok := appliedRoot["model_provider"]
	if !ok || appliedDefault != codexProvider {
		return nil, ErrUnavailable
	}
	appliedProviders, ok := appliedRoot["model_providers"].(map[string]any)
	if !ok {
		return nil, ErrUnavailable
	}
	appliedOwn, ok := appliedProviders[codexProvider].(map[string]any)
	if !ok {
		return nil, ErrUnavailable
	}
	var oldProviders map[string]any
	if raw, had := oldRoot["model_providers"]; had {
		if oldProviders, ok = raw.(map[string]any); !ok {
			return nil, ErrUnavailable
		}
	}

	// Decide semantically first; the text edit below must reproduce this.
	restoreDefault, removeOwn, dropProviders := false, false, false
	if value, present := root["model_provider"]; present && sameTOML(value, appliedDefault) {
		if previous, had := oldRoot["model_provider"]; had {
			root["model_provider"] = previous
		} else {
			delete(root, "model_provider")
		}
		restoreDefault = true
	}
	if raw, present := root["model_providers"]; present {
		providers, isTable := raw.(map[string]any)
		if !isTable {
			return nil, ErrUnavailable
		}
		if own, present := providers[codexProvider]; present && sameTOML(own, appliedOwn) {
			if previous, had := oldProviders[codexProvider]; had {
				providers[codexProvider] = previous
			} else {
				delete(providers, codexProvider)
			}
			removeOwn = true
		}
		if _, had := oldRoot["model_providers"]; !had && len(providers) == 0 {
			delete(root, "model_providers")
			dropProviders = true
		}
	}
	if !restoreDefault && !removeOwn && !dropProviders {
		if original == nil && len(root) == 0 && blankLine(current) {
			return nil, nil
		}
		return bytes.Clone(current), nil
	}

	result, edited := codexMergeText(original, current, restoreDefault, removeOwn, dropProviders)
	if edited {
		got, ok := decodeTOML(result)
		edited = ok && sameTOML(got, root) && len(result) <= maxConfig
	}
	if !edited {
		return nil, ErrChanged
	}
	if original == nil && len(root) == 0 && blankLine(result) {
		return nil, nil
	}
	return result, nil
}

// codexMergeText applies the owned-value changes to current in place.
func codexMergeText(original, current []byte, restoreDefault, removeOwn, dropProviders bool) ([]byte, bool) {
	doc, ok := scanTOML(current)
	if !ok {
		return nil, false
	}
	var old *tomlDoc
	if original != nil {
		if old, ok = scanTOML(original); !ok {
			return nil, false
		}
	}
	var edits []tomlEdit

	if restoreDefault {
		e := doc.rootKey("model_provider")
		if e == nil {
			return nil, false
		}
		var previous *tomlExpr
		if old != nil {
			previous = old.rootKey("model_provider")
		}
		if previous != nil {
			edits = append(edits, tomlEdit{start: e.valStart, end: e.valEnd, text: string(original[previous.valStart:previous.valEnd])})
		} else {
			edits = append(edits, doc.removeExpr(e))
		}
	}

	blockAtEOF := false
	if removeOwn {
		var firstHeader *tomlExpr
		for i := range doc.exprs {
			e := &doc.exprs[i]
			if !e.header && pathIs(e.path, "model_providers") {
				return nil, false // an inline table cannot be edited by line
			}
			if !hasPathPrefix(e.path, "model_providers", codexProvider) {
				continue
			}
			edits = append(edits, doc.removeExpr(e))
			blockAtEOF = e.end == len(current)
			if e.header && firstHeader == nil {
				firstHeader = e
			}
		}
		restored := ""
		if old != nil {
			if restored, ok = old.ownedBlock(); !ok {
				return nil, false
			}
		}
		switch {
		case restored != "":
			// Only reachable for a pre-existing owned table, which the planner
			// rejects; restore its exact text where the owned table was.
			if firstHeader == nil {
				return nil, false
			}
			edits = append(edits, tomlEdit{start: firstHeader.start, end: firstHeader.start, text: restored})
			blockAtEOF = false
		case firstHeader != nil && firstHeader.start > 0:
			// Remove the blank separator written before the appended table.
			prev := lineStart(current, firstHeader.start-1)
			if blankLine(current[prev:firstHeader.start]) {
				edits = append(edits, tomlEdit{start: prev, end: firstHeader.start})
			}
		}
	}

	if dropProviders {
		// An outer [model_providers] header left without entries.
		for i := range doc.exprs {
			e := &doc.exprs[i]
			if !e.header || !pathIs(e.path, "model_providers") {
				continue
			}
			empty := true
			for _, next := range doc.exprs[i+1:] {
				if next.header {
					break
				}
				empty = empty && hasPathPrefix(next.path, "model_providers", codexProvider)
			}
			if empty {
				edits = append(edits, doc.removeExpr(e))
			}
		}
	}

	result, ok := applyEdits(current, edits)
	if !ok {
		return nil, false
	}
	// The appended table ended a file that had no final newline; drop the
	// newline install had to add.
	if blockAtEOF && len(original) > 0 && original[len(original)-1] != '\n' {
		result = bytes.TrimSuffix(result, []byte(doc.eol))
	}
	// A file dmz created does not keep the separator before later tables.
	if original == nil {
		result = bytes.TrimLeft(result, "\r\n")
	}
	return result, true
}

// ownedBlock returns the exact text of an owned provider defined by table
// headers in the original, or "" when it had none.
func (d *tomlDoc) ownedBlock() (string, bool) {
	var out []byte
	for i, e := range d.exprs {
		if !hasPathPrefix(e.path, "model_providers", codexProvider) {
			continue
		}
		if !e.header {
			if inOwnedTable(d.exprs[:i]) {
				continue // included with its header's block
			}
			return "", false
		}
		end := len(d.data)
		for _, next := range d.exprs[i+1:] {
			if next.header {
				end = next.start
				break
			}
		}
		out = append(out, d.data[e.start:end]...)
		if len(out) > 0 && out[len(out)-1] != '\n' {
			out = append(out, d.eol...)
		}
	}
	return string(out), true
}

// inOwnedTable reports whether the last header before a key/value is owned.
func inOwnedTable(before []tomlExpr) bool {
	for i := len(before) - 1; i >= 0; i-- {
		if before[i].header {
			return hasPathPrefix(before[i].path, "model_providers", codexProvider)
		}
	}
	return false
}
