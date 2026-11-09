package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudePrivateRoutingSettings(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Chdir(t.TempDir())
	args, cleanup, err := claudeSettings([]string{"--settings", `{"model":"synthetic-model","env":{"SYNTHETIC_OPTION":"retained"}}`, "--bare", "--", "synthetic prompt"}, "http://127.0.0.1:1234", "X-Dmz-Token: synthetic-local-token")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if len(args) != 5 || args[0] != "--settings" || args[2] != "--bare" || args[3] != "--" || args[4] != "synthetic prompt" {
		t.Fatal("client arguments were not preserved")
	}
	body, err := os.ReadFile(args[1])
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Model string            `json:"model"`
		Env   map[string]string `json:"env"`
	}
	if json.Unmarshal(body, &settings) != nil {
		t.Fatal("temporary settings invalid")
	}
	if settings.Model != "synthetic-model" || settings.Env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:1234" || settings.Env["SYNTHETIC_OPTION"] != "retained" || settings.Env["ANTHROPIC_CUSTOM_HEADERS"] != "X-Dmz-Token: synthetic-local-token" {
		t.Fatal("settings merge lost user options or routing")
	}
	for _, arg := range args {
		if strings.Contains(arg, "synthetic-local-token") {
			t.Fatal("local token appeared in arguments")
		}
	}
	if info, err := os.Stat(args[1]); err != nil || info.Mode().Perm()&0077 != 0 {
		t.Fatal("temporary file is not private")
	}
	if info, err := os.Stat(filepath.Dir(args[1])); err != nil || info.Mode().Perm()&0077 != 0 {
		t.Fatal("temporary directory is not private")
	}
	cleanup()
	if _, err := os.Stat(args[1]); !os.IsNotExist(err) {
		t.Fatal("temporary settings not removed")
	}
}

func TestClaudeSettingsRejectIncompatibleRoutes(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Chdir(t.TempDir())
	for _, value := range []string{`{"env":{"ANTHROPIC_BASE_URL":"https://gateway.example.test"}}`, `{"env":{"CLAUDE_CODE_USE_BEDROCK":"1"}}`, `{"env":{"ANTHROPIC_CUSTOM_HEADERS":"X-Synthetic: value"}}`, ""} {
		_, cleanup, err := claudeSettings([]string{"--settings", value}, "http://127.0.0.1:1234", "synthetic")
		if cleanup != nil {
			cleanup()
		}
		if err == nil {
			t.Fatal("incompatible explicit settings accepted")
		}
	}
	name := filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "settings.json")
	if err := os.WriteFile(name, []byte(`{"env":{"ANTHROPIC_BASE_URL":"https://gateway.example.test"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	_, cleanup, err := claudeSettings(nil, "http://127.0.0.1:1234", "synthetic")
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		t.Fatal("incompatible user route accepted")
	}
}

func TestCompanionClaudeConnectionChoicePinsOnlyLocalCredentials(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Chdir(t.TempDir())
	for _, bearer := range []string{"", "synthetic-local-bearer"} {
		args, cleanup, err := claudeSettings([]string{"--settings", `{"env":{"ANTHROPIC_API_KEY":"synthetic-user-key","ANTHROPIC_AUTH_TOKEN":"synthetic-user-token","CLAUDE_CODE_OAUTH_TOKEN":"synthetic-oauth"}}`}, "http://127.0.0.1:1234", "X-Dmz-Token: synthetic-local", claudeLaunchSettings{PinAuth: true, LocalBearer: bearer})
		if err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(args[1])
		cleanup()
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Env map[string]string `json:"env"`
		}
		if json.Unmarshal(body, &result) != nil {
			t.Fatal("invalid private auth settings")
		}
		if result.Env["ANTHROPIC_API_KEY"] != "" || result.Env["ANTHROPIC_AUTH_TOKEN"] != bearer {
			t.Fatal("explicit app connection could be replaced by ordinary settings")
		}
		if bearer != "" && result.Env["CLAUDE_CODE_OAUTH_TOKEN"] != "" {
			t.Fatal("API connection kept an overriding OAuth setting")
		}
		if strings.Contains(string(body), "synthetic-user-key") || strings.Contains(string(body), "synthetic-user-token") {
			t.Fatal("private local routing copied provider credentials")
		}
	}
}
