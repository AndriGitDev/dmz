package routing

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallRestoreAndDrift(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	state := filepath.Join(dir, "private", "routing.json")
	original := []byte("{\n  \"model\": \"synthetic\", \"permissions\":{\"defaultMode\":\"default\"}\n}\n")
	os.WriteFile(path, original, 0640)
	next, err := ClaudeSettings(original, "http://127.0.0.1:43128", strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	if err = Install(path, state, original, next); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal(next, &got)
	if got["model"] != "synthetic" || got["permissions"].(map[string]any)["defaultMode"] != "default" {
		t.Fatal("unrelated settings changed")
	}
	changed := append(append([]byte{}, next...), '\n')
	os.WriteFile(path, changed, 0600)
	if !errors.Is(Restore(state), ErrChanged) {
		t.Fatal("drift overwritten")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(after, changed) {
		t.Fatal("external edit lost")
	}
	os.WriteFile(path, next, 0600)
	if err = Restore(state); err != nil {
		t.Fatal(err)
	}
	after, _ = os.ReadFile(path)
	if !bytes.Equal(after, original) {
		t.Fatal("original bytes not restored")
	}
}
func TestNewSettingsAreRemovedOnRestore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	state := filepath.Join(dir, "private", "routing.json")
	next, err := ClaudeSettings(nil, "http://127.0.0.1:43128", strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	if err = Install(path, state, nil, next); err != nil {
		t.Fatal(err)
	}
	if err = Restore(state); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("new settings left behind")
	}
}
func TestCustomRouteAndStaleEditAreRejected(t *testing.T) {
	if _, err := ClaudeSettings([]byte(`{"env":{"ANTHROPIC_BASE_URL":"https://gateway.example.test"}}`), "http://127.0.0.1:43128", strings.Repeat("a", 64)); !errors.Is(err, ErrCustomRoute) {
		t.Fatal("custom route replaced")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	os.WriteFile(path, []byte(`{}`), 0600)
	if !errors.Is(Install(path, filepath.Join(dir, "private", "state"), []byte(`{"model":"stale"}`), []byte(`{}`)), ErrChanged) {
		t.Fatal("stale edit installed")
	}
}

func TestPreparedJournalRecoversWithoutChangingOriginal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	state := filepath.Join(dir, "journal.json")
	original := []byte("{\"model\":\"synthetic\"}\n")
	os.WriteFile(path, original, 0600)
	prepared, _ := json.Marshal(journal{Path: path, Existed: true, Mode: 0600, Original: original, Applied: digest([]byte("replacement"))})
	os.WriteFile(state, prepared, 0600)
	if err := Restore(state); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, original) {
		t.Fatal("prepared transaction changed original")
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatal("prepared transaction not cleared")
	}
}
func TestSymlinkSettingsAreNotReplaced(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	path := filepath.Join(dir, "settings.json")
	os.WriteFile(target, []byte(`{}`), 0600)
	if err := os.Symlink(target, path); err != nil {
		t.Skip("symlinks unavailable")
	}
	if !errors.Is(Install(path, filepath.Join(dir, "private", "journal"), []byte(`{}`), []byte(`{"new":true}`)), ErrUnavailable) {
		t.Fatal("symlink settings replaced")
	}
	got, _ := os.ReadFile(target)
	if string(got) != `{}` {
		t.Fatal("symlink target changed")
	}
}
