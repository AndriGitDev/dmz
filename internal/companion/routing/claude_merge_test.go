package routing

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestClaudeMergeKeepsNewHeadersAndUserRoute(t *testing.T) {
	original := []byte(`{"model":"original","env":{"ANTHROPIC_CUSTOM_HEADERS":"X-Existing: kept"}}`)
	written, err := ClaudeSettings(original, "http://127.0.0.1:12345", strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	var current map[string]any
	json.Unmarshal(written, &current)
	env := current["env"].(map[string]any)
	env["ANTHROPIC_CUSTOM_HEADERS"] = env["ANTHROPIC_CUSTOM_HEADERS"].(string) + "\nX-User-Added: kept"
	env["ANTHROPIC_BASE_URL"] = "https://gateway.example.test"
	current["model"] = "user-edited"
	changed, _ := json.Marshal(current)
	result, err := MergeClaudeSettings(original, written, changed)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal(result, &got)
	mergedEnv := got["env"].(map[string]any)
	if got["model"] != "user-edited" || mergedEnv["ANTHROPIC_BASE_URL"] != "https://gateway.example.test" || mergedEnv["ANTHROPIC_CUSTOM_HEADERS"] != "X-Existing: kept\nX-User-Added: kept" {
		t.Fatal("selective Claude restore lost user settings")
	}
}
func TestClaudeMergeRemovesOnlyOwnedNewSettings(t *testing.T) {
	written, err := ClaudeSettings(nil, "http://127.0.0.1:12345", strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	result, err := MergeClaudeSettings(nil, written, written)
	if err != nil || result != nil {
		t.Fatal("new owned file not removable")
	}
	var root map[string]any
	json.Unmarshal(written, &root)
	root["model"] = "new-user-model"
	changed, _ := json.Marshal(root)
	result, err = MergeClaudeSettings(nil, written, changed)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal(result, &got)
	if len(got) != 1 || got["model"] != "new-user-model" {
		t.Fatal("new user settings lost")
	}
}
