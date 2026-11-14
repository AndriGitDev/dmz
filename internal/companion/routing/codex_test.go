package routing

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// Synthetic values only: no real configuration, endpoint or credential.
const (
	codexOrigin = "http://127.0.0.1:43123"
	codexToken  = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

func parseCodex(t *testing.T, b []byte) map[string]any {
	t.Helper()
	m := map[string]any{}
	if err := toml.Unmarshal(b, &m); err != nil {
		t.Fatalf("generated config does not parse: %v\n%s", err, b)
	}
	return m
}

func wantDmzProvider(t *testing.T, cfg map[string]any) {
	t.Helper()
	if cfg["model_provider"] != "dmz_background" {
		t.Fatalf("model_provider = %#v", cfg["model_provider"])
	}
	providers, _ := cfg["model_providers"].(map[string]any)
	got, _ := providers["dmz_background"].(map[string]any)
	want := map[string]any{
		"name":                 "dmz background gateway",
		"base_url":             "http://127.0.0.1:43123/v1",
		"wire_api":             "responses",
		"requires_openai_auth": true,
		"supports_websockets":  false,
		"http_headers":         map[string]any{"X-Dmz-Token": codexToken},
		"env_http_headers":     map[string]any{"X-Dmz-OpenAI-Key": "OPENAI_API_KEY"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("provider = %#v, want %#v", got, want)
	}
}

func TestCodexSettingsNewConfigOwnsOnlyRoutingKeys(t *testing.T) {
	for _, original := range [][]byte{nil, {}, []byte("# comment only\n\n")} {
		out, err := CodexSettings(original, codexOrigin+"/", codexToken)
		if err != nil {
			t.Fatal(err)
		}
		cfg := parseCodex(t, out)
		wantDmzProvider(t, cfg)
		if len(cfg) != 2 || len(cfg["model_providers"].(map[string]any)) != 1 {
			t.Fatalf("unexpected extra settings: %#v", cfg)
		}
		for _, forbidden := range []string{"model =", "approval_policy", "sandbox", "features", "api_key", "experimental_bearer_token", "env_key", "Authorization"} {
			if bytes.Contains(out, []byte(forbidden)) {
				t.Fatalf("output sets %q:\n%s", forbidden, out)
			}
		}
	}
}

const codexRichFixture = `# synthetic fixture
model_provider = "openai"
model = "synthetic-model"
approval_policy = "on-request"
sandbox_mode = "workspace-write"
"" = "empty key"
"key.with.dots" = 0x2A
ratio = 1.5e3
limit = inf
since = 1979-05-27T07:32:00-08:00
local_day = 1979-05-27
local_time = 07:32:00.999
notes = """
Línur með "gæsalöppum" og \u00e6 \t tab"""
windows_literal = 'C:\Users\Jón\verkefni'
windows_basic = "C:\\Users\\Jón\\verkefni"
mixed = [1, "two", { three = 3 }, [4.0]]

[sandbox_workspace_write]
writable_roots = ["/tmp/dmz synthetic/ü", 'D:\gögn']
network_access = false

[permissions.network]
allowed_domains = ["example.invalid"]

[features]
unified_exec = true

[mcp_servers.docs]
command = "npx"
args = ["-y", "@example/synthetic-mcp"]
env = { SYNTHETIC = "1" }
startup_timeout_sec = 20

[mcp_servers."spaced name"]
url = "http://127.0.0.1:9/mcp"

[model_providers.inactive_custom]
name = "Inactive synthetic provider"
base_url = "http://127.0.0.1:9/v1"
wire_api = "responses"

[projects."/home/notandi/verk efni/ástæða.d"]
trust_level = "trusted"

[profiles.fast]
model = "synthetic-small"
approval_policy = "never"
model_reasoning_effort = "low"

[[hooks.post]]
command = ["echo", "a"]

[[hooks.post]]
command = ["echo", "b"]

[tui]
theme = "dark"
`

func TestCodexSettingsPreservesUnrelatedSemantics(t *testing.T) {
	out, err := CodexSettings([]byte(codexRichFixture), codexOrigin, codexToken)
	if err != nil {
		t.Fatal(err)
	}
	got := parseCodex(t, out)
	wantDmzProvider(t, got)
	want := parseCodex(t, []byte(codexRichFixture))

	// Remove only what dmz owns; every other value must be semantically equal.
	delete(want, "model_provider")
	delete(got, "model_provider")
	delete(got["model_providers"].(map[string]any), "dmz_background")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unrelated settings changed\n got: %#v\nwant: %#v\n%s", got, want, out)
	}

	// Spot-check values whose encoding is easy to get wrong.
	if got["windows_basic"] != `C:\Users\Jón\verkefni` || got["windows_literal"] != `C:\Users\Jón\verkefni` {
		t.Fatalf("escaped paths changed: %#v %#v", got["windows_basic"], got["windows_literal"])
	}
	if got["key.with.dots"] != int64(42) || got[""] != "empty key" || !math.IsInf(got["limit"].(float64), 1) {
		t.Fatalf("quoted keys or numbers changed")
	}
	if !strings.Contains(got["notes"].(string), "æ \t tab") {
		t.Fatalf("unicode string changed: %q", got["notes"])
	}
	if _, ok := got["since"].(time.Time); !ok {
		t.Fatalf("offset datetime type changed: %T", got["since"])
	}
	if _, ok := got["projects"].(map[string]any)["/home/notandi/verk efni/ástæða.d"]; !ok {
		t.Fatalf("quoted Unicode table key lost")
	}
	if p := got["profiles"].(map[string]any)["fast"].(map[string]any); p["approval_policy"] != "never" || p["model"] != "synthetic-small" {
		t.Fatalf("profile changed: %#v", p)
	}
	if len(got["hooks"].(map[string]any)["post"].([]any)) != 2 {
		t.Fatalf("array of tables changed")
	}
	if _, inTable := got["tui"].(map[string]any)["model_provider"]; inTable {
		t.Fatalf("model_provider was written inside the last table")
	}
	// The provider key must precede every table header in the encoded file.
	header := false
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		header = header || strings.HasPrefix(line, "[")
		if strings.HasPrefix(line, "model_provider ") {
			if header || line != `model_provider = "dmz_background"` {
				t.Fatalf("model_provider is not a top-level key before tables:\n%s", out)
			}
			break
		}
	}
}

func TestCodexSettingsRejectsAmbiguousRoutes(t *testing.T) {
	custom := map[string]string{
		"active custom provider":   "model_provider = \"corp\"\n[model_providers.corp]\nname = \"c\"\nbase_url = \"https://example.invalid/v1\"\n",
		"active built-in oss":      "model_provider = \"ollama\"\n",
		"empty provider":           "model_provider = \"\"\n",
		"openai base override":     "openai_base_url = \"https://example.invalid/v1\"\n",
		"chatgpt base override":    "chatgpt_base_url = \"https://example.invalid/backend-api/\"\n",
		"existing dmz table":       "[model_providers.dmz_background]\nname = \"old\"\n",
		"profile openai provider":  "[profiles.p]\nmodel_provider = \"openai\"\n",
		"profile custom provider":  "[profiles.p]\nmodel_provider = \"corp\"\n",
		"profile endpoint":         "[profiles.p]\nopenai_base_url = \"https://example.invalid/v1\"\n",
		"profile chatgpt endpoint": "[profiles.p]\nchatgpt_base_url = \"https://example.invalid/\"\n",
	}
	for name, cfg := range custom {
		if out, err := CodexSettings([]byte(cfg), codexOrigin, codexToken); err != ErrCustomRoute || out != nil {
			t.Errorf("%s: got %v, want %v", name, err, ErrCustomRoute)
		}
	}
	malformed := map[string]string{
		"syntax":              "model = \n",
		"duplicate key":       "model = \"a\"\nmodel = \"b\"\n",
		"duplicate table":     "[tui]\na = 1\n[tui]\nb = 2\n",
		"redefined table":     "a = { b = 1 }\n[a]\nc = 2\n",
		"invalid utf-8":       "model = \"\xff\"\n",
		"non-string provider": "model_provider = 1\n",
		"providers not table": "model_providers = \"x\"\n",
		"profiles not table":  "profiles = 3\n",
		"profile not table":   "profiles = { p = 1 }\n",
		"profile bad type":    "[profiles.p]\nmodel_provider = true\n",
		"deep nesting":        "a = " + strings.Repeat("[", 100000) + strings.Repeat("]", 100000) + "\n",
	}
	for name, cfg := range malformed {
		if out, err := CodexSettings([]byte(cfg), codexOrigin, codexToken); err != ErrUnavailable || out != nil {
			t.Errorf("%s: got %v, want %v", name, err, ErrUnavailable)
		}
	}
	if _, err := CodexSettings(make([]byte, maxConfig+1), codexOrigin, codexToken); err != ErrUnavailable {
		t.Errorf("oversized: %v", err)
	}
	// An installed config is not silently re-wrapped; Restore must run first.
	out, err := CodexSettings(nil, codexOrigin, codexToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CodexSettings(out, codexOrigin, codexToken); err != ErrCustomRoute {
		t.Errorf("reinstall: %v", err)
	}
}

func TestCodexSettingsValidatesOriginAndToken(t *testing.T) {
	for _, origin := range []string{
		"", "https://127.0.0.1:43123", "http://localhost:43123", "http://127.0.0.2:43123", "http://[::1]:43123",
		"http://127.0.0.1", "http://127.0.0.1:", "http://127.0.0.1:0", "http://127.0.0.1:65536", "http://127.0.0.1:043123",
		"http://127.0.0.1:43123/v1", "http://127.0.0.1:43123?x=1", "http://127.0.0.1:43123/?", "http://127.0.0.1:43123#f",
		"http://u@127.0.0.1:43123", "http:127.0.0.1:43123", "http://127.0.0.1:43123//", "http://127.0.0.1:43123/%2F",
	} {
		if _, err := CodexSettings(nil, origin, codexToken); err != ErrUnavailable {
			t.Errorf("origin %q: %v", origin, err)
		}
	}
	for _, token := range []string{"", codexToken[:63], codexToken + "0", strings.Repeat("g", 64), strings.Repeat("é", 32)} {
		if _, err := CodexSettings(nil, codexOrigin, token); err != ErrUnavailable {
			t.Errorf("token %q: %v", token, err)
		}
	}
	// Validation happens before parsing, so bad input never echoes config data.
	_, err := CodexSettings([]byte("model = \"synthetic-secret-marker\"\nmodel = 1\n"), codexOrigin, codexToken)
	if !errors.Is(err, ErrUnavailable) || strings.Contains(err.Error(), "synthetic") {
		t.Fatalf("error leaks input: %v", err)
	}
}

func TestCodexSettingsInstallRestoreIsByteExact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "codex", "config.toml")
	state := filepath.Join(dir, "state", "codex.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	original := []byte(codexRichFixture)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	replacement, err := CodexSettings(original, codexOrigin, codexToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := Install(path, state, original, replacement); err != nil {
		t.Fatal(err)
	}
	if err := Restore(state); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, original) {
		t.Fatal("restore did not return the original bytes, including comments")
	}
}

// TestCodexSettingsLoadsInInstalledCodex is opt-in: set DMZ_CODEX_BIN to a
// Codex CLI. It uses a temporary CODEX_HOME with synthetic settings only, so no
// real configuration or authentication store is read, and no request is made.
func TestCodexSettingsLoadsInInstalledCodex(t *testing.T) {
	bin := os.Getenv("DMZ_CODEX_BIN")
	if bin == "" {
		t.Skip("set DMZ_CODEX_BIN to check generated settings with an installed Codex CLI")
	}
	load := func(config []byte) ([]byte, error) {
		home := t.TempDir()
		if err := os.WriteFile(filepath.Join(home, "config.toml"), config, 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, bin, "features", "list")
		cmd.Dir = home
		cmd.Env = []string{"CODEX_HOME=" + home, "HOME=" + home, "PATH=" + os.Getenv("PATH"), "NO_COLOR=1"}
		return cmd.CombinedOutput()
	}
	// Negative control: the command really validates the provider route.
	if out, err := load([]byte("model_provider = \"missing\"\n")); err == nil && !bytes.Contains(out, []byte("failed to load")) {
		t.Fatalf("Codex accepted an unknown provider; check is not validating:\n%s", out)
	}
	for _, original := range [][]byte{nil, []byte(codexRichFixture)} {
		generated, err := CodexSettings(original, codexOrigin, codexToken)
		if err != nil {
			t.Fatal(err)
		}
		if out, err := load(generated); err != nil || bytes.Contains(out, []byte("failed to load")) {
			t.Fatalf("Codex rejected generated settings: %v\n%s\n--- config ---\n%s", err, out, generated)
		}
	}
}
