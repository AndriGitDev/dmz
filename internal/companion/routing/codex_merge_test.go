package routing

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

// Synthetic configurations only; no real Codex config or auth store is read.

func codexWritten(t *testing.T, original []byte) []byte {
	t.Helper()
	out, err := CodexSettings(original, codexOrigin, codexToken)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// editCodex decodes TOML, applies an edit and re-encodes it, simulating a user
// change made after installation.
func editCodex(t *testing.T, b []byte, edit func(map[string]any)) []byte {
	t.Helper()
	m := parseCodex(t, b)
	edit(m)
	out, err := toml.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func mergeCodex(t *testing.T, original, written, current []byte) map[string]any {
	t.Helper()
	out, err := MergeCodexSettings(original, written, current)
	if err != nil {
		t.Fatal(err)
	}
	if out == nil {
		t.Fatal("merge returned nil for a config that must remain")
	}
	return parseCodex(t, out)
}

func TestMergeCodexKeepsUserEditsAndRestoresOwnedKeys(t *testing.T) {
	original := []byte(`model = "synthetic-model-a"
model_provider = "openai"
approval_policy = "on-request"
sandbox_mode = "workspace-write"

[model_providers.local_other]
name = "synthetic other"
base_url = "http://127.0.0.1:9/v1"

[profiles.fast]
model = "synthetic-model-b"
`)
	written := codexWritten(t, original)
	current := editCodex(t, written, func(m map[string]any) {
		m["model"] = "synthetic-model-c"
		m["sandbox_mode"] = "read-only"
		m["mcp_servers"] = map[string]any{"docs": map[string]any{"command": "synthetic-mcp", "args": []any{"--stdio"}}}
		m["model_providers"].(map[string]any)["local_new"] = map[string]any{"name": "user added", "base_url": "http://127.0.0.1:8/v1"}
		m["model_providers"].(map[string]any)["local_other"].(map[string]any)["http_headers"] = map[string]any{"X-User": "kept"}
		m["profiles"].(map[string]any)["slow"] = map[string]any{"model": "synthetic-model-d"}
	})

	got := mergeCodex(t, original, written, current)
	want := parseCodex(t, original)
	want["model"] = "synthetic-model-c"
	want["sandbox_mode"] = "read-only"
	want["mcp_servers"] = map[string]any{"docs": map[string]any{"command": "synthetic-mcp", "args": []any{"--stdio"}}}
	want["model_providers"].(map[string]any)["local_new"] = map[string]any{"name": "user added", "base_url": "http://127.0.0.1:8/v1"}
	want["model_providers"].(map[string]any)["local_other"].(map[string]any)["http_headers"] = map[string]any{"X-User": "kept"}
	want["profiles"].(map[string]any)["slow"] = map[string]any{"model": "synthetic-model-d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merged = %#v\nwant %#v", got, want)
	}
}

func TestMergeCodexDeletesOwnedKeysAbsentFromOriginal(t *testing.T) {
	original := []byte("model = \"synthetic-model-a\"\n")
	written := codexWritten(t, original)
	current := editCodex(t, written, func(m map[string]any) { m["approval_policy"] = "never" })
	got := mergeCodex(t, original, written, current)
	want := map[string]any{"model": "synthetic-model-a", "approval_policy": "never"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merged = %#v, want %#v", got, want)
	}
}

func TestMergeCodexKeepsIntentionallyChangedProvider(t *testing.T) {
	original := []byte("model = \"synthetic-model-a\"\n")
	written := codexWritten(t, original)
	current := editCodex(t, written, func(m map[string]any) {
		m["model_provider"] = "local_new"
		m["model_providers"].(map[string]any)["local_new"] = map[string]any{"name": "user", "base_url": "http://127.0.0.1:7/v1"}
	})
	got := mergeCodex(t, original, written, current)
	want := map[string]any{
		"model":           "synthetic-model-a",
		"model_provider":  "local_new",
		"model_providers": map[string]any{"local_new": map[string]any{"name": "user", "base_url": "http://127.0.0.1:7/v1"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merged = %#v, want %#v", got, want)
	}
}

func TestMergeCodexKeepsEditedOwnedTableButRestoresDefault(t *testing.T) {
	original := []byte("model_provider = \"openai\"\n")
	written := codexWritten(t, original)
	current := editCodex(t, written, func(m map[string]any) {
		own := m["model_providers"].(map[string]any)[codexProvider].(map[string]any)
		own["http_headers"].(map[string]any)["X-User"] = "edited"
	})
	got := mergeCodex(t, original, written, current)
	if got["model_provider"] != "openai" {
		t.Fatalf("model_provider = %#v", got["model_provider"])
	}
	own, ok := got["model_providers"].(map[string]any)[codexProvider].(map[string]any)
	if !ok || own["http_headers"].(map[string]any)["X-User"] != "edited" || own["http_headers"].(map[string]any)["X-Dmz-Token"] != codexToken {
		t.Fatalf("edited owned table not kept: %#v", got["model_providers"])
	}
}

func TestMergeCodexOwnedTableDeletedByUser(t *testing.T) {
	original := []byte("model = \"synthetic-model-a\"\n")
	written := codexWritten(t, original)
	current := editCodex(t, written, func(m map[string]any) { delete(m, "model_providers") })
	got := mergeCodex(t, original, written, current)
	if want := map[string]any{"model": "synthetic-model-a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("merged = %#v, want %#v", got, want)
	}

	// Removing only the owned entry leaves an empty outer table, which is not
	// the user's and is dropped too.
	current = editCodex(t, written, func(m map[string]any) {
		delete(m["model_providers"].(map[string]any), codexProvider)
		m["model"] = "synthetic-model-b"
	})
	got = mergeCodex(t, original, written, current)
	if want := map[string]any{"model": "synthetic-model-b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("merged = %#v, want %#v", got, want)
	}
}

func TestMergeCodexKeepsEmptyOuterTableFromOriginal(t *testing.T) {
	original := []byte("model = \"a\"\n\n[model_providers]\n")
	written := codexWritten(t, original)
	current := editCodex(t, written, func(m map[string]any) { m["model"] = "b" })
	got := mergeCodex(t, original, written, current)
	if want := map[string]any{"model": "b", "model_providers": map[string]any{}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("merged = %#v, want %#v", got, want)
	}
}

func TestMergeCodexRestoresOriginalOwnedTable(t *testing.T) {
	// The planner rejects such an original, but the merge must still restore it
	// rather than delete a table that existed before.
	original := []byte("[model_providers.dmz_background]\nname = \"pre-existing\"\n")
	written := []byte(`model_provider = "dmz_background"

[model_providers.dmz_background]
name = "dmz background gateway"
`)
	current := []byte(`model = "synthetic-model-a"
model_provider = "dmz_background"

[model_providers.dmz_background]
name = "dmz background gateway"
`)
	got := mergeCodex(t, original, written, current)
	want := map[string]any{
		"model":           "synthetic-model-a",
		"model_providers": map[string]any{codexProvider: map[string]any{"name": "pre-existing"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merged = %#v, want %#v", got, want)
	}
}

func TestMergeCodexNewFileCleanup(t *testing.T) {
	written := codexWritten(t, nil)

	out, err := MergeCodexSettings(nil, written, written)
	if err != nil || out != nil {
		t.Fatalf("unchanged generated file: out=%q err=%v, want nil,nil", out, err)
	}
	out, err = MergeCodexSettings(nil, written, []byte{})
	if err != nil || out != nil {
		t.Fatalf("emptied generated file: out=%q err=%v, want nil,nil", out, err)
	}

	current := editCodex(t, written, func(m map[string]any) {
		m["model"] = "synthetic-model-a"
		m["mcp_servers"] = map[string]any{"docs": map[string]any{"command": "synthetic-mcp"}}
	})
	got := mergeCodex(t, nil, written, current)
	want := map[string]any{"model": "synthetic-model-a", "mcp_servers": map[string]any{"docs": map[string]any{"command": "synthetic-mcp"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merged = %#v, want %#v", got, want)
	}

	// An existing but empty original is a file to keep, not a file to delete.
	out, err = MergeCodexSettings([]byte{}, codexWritten(t, []byte{}), codexWritten(t, []byte{}))
	if err != nil || out == nil || len(bytes.TrimSpace(out)) != 0 {
		t.Fatalf("empty original: out=%q err=%v, want empty non-nil", out, err)
	}
}

func TestMergeCodexReturnsCurrentWhenNothingOwnedRemains(t *testing.T) {
	original := []byte("model = \"a\"\n")
	written := codexWritten(t, original)
	current := []byte("# user comment kept\nmodel = \"b\"\nmodel_provider = \"local_new\"\n")
	out, err := MergeCodexSettings(original, written, current)
	if err != nil || !bytes.Equal(out, current) {
		t.Fatalf("out=%q err=%v, want current bytes unchanged", out, err)
	}
}

func TestMergeCodexRejectsMalformedInputWithFixedError(t *testing.T) {
	original := []byte("model = \"a\"\n")
	written := codexWritten(t, original)
	secret := "synthetic-secret-value"
	cases := map[string][3][]byte{
		"malformed current":   {original, written, []byte("model = \"" + secret + "\n[")},
		"duplicate current":   {original, written, []byte("model = \"" + secret + "\"\nmodel = \"b\"\n")},
		"duplicate original":  {[]byte("model = \"" + secret + "\"\nmodel = \"b\"\n"), written, written},
		"malformed written":   {original, []byte("model_provider = "), written},
		"nil written":         {original, nil, written},
		"nil current":         {original, written, nil},
		"written not owned":   {original, []byte("model_provider = \"openai\"\n"), written},
		"providers not table": {original, written, []byte("model_provider = \"dmz_background\"\nmodel_providers = \"" + secret + "\"\n")},
		"original providers":  {[]byte("model_providers = 1\n"), written, written},
		"oversized current":   {original, written, make([]byte, maxConfig+1)},
	}
	for name, c := range cases {
		out, err := MergeCodexSettings(c[0], c[1], c[2])
		if err != ErrUnavailable || out != nil {
			t.Fatalf("%s: out=%q err=%v, want nil,ErrUnavailable", name, out, err)
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("%s: error leaks data", name)
		}
	}
}
