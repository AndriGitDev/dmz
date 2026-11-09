package companion

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

type backgroundConnection struct{ origin, token string }

func backgroundConfigForTest(t *testing.T) (BackgroundConfig, map[string][]byte) {
	t.Helper()
	dir := t.TempDir()
	settings := map[string]string{"codex": filepath.Join(dir, "codex", "config.toml"), "claude": filepath.Join(dir, "claude", "settings.json")}
	originals := map[string][]byte{"codex": []byte("# synthetic\nmodel='synthetic-model'\n[permissions]\nmode='synthetic'\n"), "claude": []byte("{\n \"model\":\"synthetic-model\",\"permissions\":{\"defaultMode\":\"default\"}\n}\n")}
	for id, path := range settings {
		os.MkdirAll(filepath.Dir(path), 0700)
		os.WriteFile(path, originals[id], 0600)
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if bytes.Contains(body, []byte("pilot@example.test")) {
			t.Error("background route leaked eligible text")
		}
		if r.Header.Get("X-Dmz-Token") != "" || r.Header.Get("Authorization") != "Bearer synthetic-native-credential" {
			t.Error("background authentication boundary changed")
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{}`)
	}))
	t.Cleanup(provider.Close)
	cfg := BackgroundConfig{StateDir: filepath.Join(dir, "dmz"), Settings: settings, Upstreams: map[string]string{"codex": provider.URL + "/v1", "claude": provider.URL + "/v1"}, Discover: func() []ClientInstallation {
		return []ClientInstallation{{ID: "codex", Name: "Codex CLI", Available: true}, {ID: "claude", Name: "Claude Code", Available: true}}
	}}
	return cfg, originals
}
func readBackgroundConnection(t *testing.T, id, path string) backgroundConnection {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("settings unavailable")
	}
	if id == "codex" {
		var root map[string]any
		if toml.Unmarshal(data, &root) != nil {
			t.Fatal("invalid TOML")
		}
		provider := root["model_providers"].(map[string]any)["dmz_background"].(map[string]any)
		return backgroundConnection{provider["base_url"].(string), provider["http_headers"].(map[string]any)["X-Dmz-Token"].(string)}
	}
	var root struct {
		Env map[string]string `json:"env"`
	}
	if json.Unmarshal(data, &root) != nil {
		t.Fatal("invalid JSON")
	}
	return backgroundConnection{root.Env["ANTHROPIC_BASE_URL"] + "/v1", strings.TrimPrefix(root.Env["ANTHROPIC_CUSTOM_HEADERS"], "X-Dmz-Token: ")}
}
func TestBackgroundNormalRequestsRestartAndRestore(t *testing.T) {
	cfg, originals := backgroundConfigForTest(t)
	m, err := newBackground(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	connections := map[string]backgroundConnection{}
	for _, view := range m.Views() {
		if view.State != "restart_required" {
			t.Fatal("detection falsely marked active")
		}
	}
	if other, err := newBackground(cfg); err == nil {
		other.Close()
		t.Fatal("second manager changed active routing")
	}
	for _, id := range []string{"codex", "claude"} {
		connection := readBackgroundConnection(t, id, cfg.Settings[id])
		connections[id] = connection
		path, body := "/responses", `{"input":"pilot@example.test"}`
		if id == "claude" {
			path, body = "/messages", `{"messages":[{"role":"user","content":"pilot@example.test"}]}`
		}
		r, _ := http.NewRequest("POST", connection.origin+path, strings.NewReader(body))
		r.Header.Set("X-Dmz-Token", connection.token)
		r.Header.Set("Authorization", "Bearer synthetic-native-credential")
		r.Header.Set("Content-Type", "application/json")
		if id == "claude" {
			r.Header.Set("X-Claude-Code-Session-Id", "synthetic-session")
		}
		response, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal("background request failed")
		}
		response.Body.Close()
		if response.StatusCode != 200 {
			t.Fatal("background request rejected")
		}
	}
	for _, view := range m.Views() {
		if view.State != "traffic_observed" || view.Requests != 1 || view.ChangedParts != 1 {
			t.Fatal("background traffic state incorrect")
		}
	}
	m.Close()
	for id, path := range cfg.Settings {
		got, _ := os.ReadFile(path)
		if !bytes.Equal(got, originals[id]) {
			t.Fatal("original settings not restored")
		}
	}
	next, err := newBackground(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	for id, previous := range connections {
		current := readBackgroundConnection(t, id, cfg.Settings[id])
		if current != previous {
			t.Fatal("background restart changed stable route credentials")
		}
	}
}
func TestBackgroundUserEditsSurviveDisconnect(t *testing.T) {
	cfg, _ := backgroundConfigForTest(t)
	m, err := newBackground(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	data, _ := os.ReadFile(cfg.Settings["claude"])
	var root map[string]any
	json.Unmarshal(data, &root)
	root["model"] = "user-edited-model"
	root["env"].(map[string]any)["USER_ADDED_OPTION"] = "retained"
	edited, _ := json.Marshal(root)
	os.WriteFile(cfg.Settings["claude"], edited, 0600)
	if err = m.SetConnected("claude", false); err != nil {
		t.Fatal("edited settings could not disconnect")
	}
	data, _ = os.ReadFile(cfg.Settings["claude"])
	json.Unmarshal(data, &root)
	if root["model"] != "user-edited-model" || root["env"].(map[string]any)["USER_ADDED_OPTION"] != "retained" {
		t.Fatal("user edit lost")
	}
	if _, exists := root["env"].(map[string]any)["ANTHROPIC_BASE_URL"]; exists {
		t.Fatal("owned route left behind")
	}
	m.Close()
	next, err := newBackground(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	for _, view := range next.Views() {
		if view.Client == "claude" && view.State != "disconnected" {
			t.Fatal("user disconnect preference ignored")
		}
	}
}
func TestCodexExternalProfilesAreVisibleAndUnmodified(t *testing.T) {
	cfg, original := backgroundConfigForTest(t)
	profile := filepath.Join(filepath.Dir(cfg.Settings["codex"]), "synthetic.config.toml")
	os.WriteFile(profile, []byte("model_provider='openai'\n"), 0600)
	m, err := newBackground(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	for _, view := range m.Views() {
		if view.Client == "codex" && (view.State != "needs_attention" || view.Issue != "codex_profiles_require_connection_review") {
			t.Fatal("profile bypass not disclosed")
		}
	}
	data, _ := os.ReadFile(cfg.Settings["codex"])
	if !bytes.Equal(data, original["codex"]) {
		t.Fatal("profile setup modified without qualification")
	}
}

func TestBackgroundDetectsLaterInstallAndAllowsDisconnectAfterRemoval(t *testing.T) {
	cfg, _ := backgroundConfigForTest(t)
	available := false
	cfg.Discover = func() []ClientInstallation {
		return []ClientInstallation{{ID: "codex", Name: "Codex CLI", Available: available}, {ID: "claude", Name: "Claude Code", Available: false}}
	}
	m, err := newBackground(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	for _, view := range m.Views() {
		if view.Routed {
			t.Fatal("undetected tool routed")
		}
	}
	available = true
	m.Refresh()
	var routed bool
	for _, view := range m.Views() {
		if view.Client == "codex" {
			routed = view.Routed
		}
	}
	if !routed {
		t.Fatal("later installation not connected")
	}
	available = false
	m.Refresh()
	if m.SetConnected("codex", false) != nil {
		t.Fatal("removed client route could not disconnect")
	}
}
