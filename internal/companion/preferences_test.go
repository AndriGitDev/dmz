package companion

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestHighlightPreferenceDefaultPersistenceAndValidation(t *testing.T) {
	cfg, _ := backgroundConfigForTest(t)
	m, err := newBackground(cfg)
	if err != nil {
		t.Fatal(err)
	}
	a, local := testApp(t, func(string, string, string) error { return nil }, http.NotFoundHandler())
	a.mu.Lock()
	a.background = m
	a.mu.Unlock()
	if a.highlightRestored() {
		t.Fatal("highlight should default off")
	}
	for _, body := range []string{`{}`, `{"highlight_restored":null}`, `{"highlight_restored":"yes"}`, `{"highlight_restored":true,"upstream":"unexpected"}`, `{"highlight_restored":true} {}`} {
		if appRequest(t, a, local, "POST", "/api/preferences", body, nil).StatusCode != 400 {
			t.Fatal("invalid preference accepted")
		}
	}
	if appRequest(t, a, local, "POST", "/api/preferences", `{"highlight_restored":true}`, map[string]string{"Authorization": "wrong"}).StatusCode != 401 {
		t.Fatal("unauthenticated setting changed")
	}
	if appRequest(t, a, local, "POST", "/api/preferences", `{"highlight_restored":true}`, map[string]string{"Origin": "https://outside.example.test"}).StatusCode != 403 {
		t.Fatal("cross-origin setting changed")
	}
	if appRequest(t, a, local, "POST", "/api/preferences", `{"highlight_restored":true}`, nil).StatusCode != 204 || !a.highlightRestored() {
		t.Fatal("preference not applied")
	}
	path := filepath.Join(cfg.StateDir, "preferences.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var stored map[string]any
	if json.Unmarshal(b, &stored) != nil || len(stored) != 1 || stored["highlight_restored"] != true {
		t.Fatal("unexpected persisted settings")
	}
	if info, err := os.Stat(path); err != nil || runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		t.Fatal("preference file not private")
	}
	m.Close()
	next, err := newBackground(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	a.mu.Lock()
	a.background = next
	a.mu.Unlock()
	if !a.highlightRestored() {
		t.Fatal("preference not restored after restart")
	}
	if appRequest(t, a, local, "POST", "/api/preferences", `{"highlight_restored":false}`, nil).StatusCode != 204 || a.highlightRestored() {
		t.Fatal("cannot turn preference off")
	}
}
