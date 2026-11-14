//go:build linux

// SPDX-License-Identifier: GPL-3.0-only

// Command backgroundtesthost runs the real DMZ companion controller with the
// background integration enabled for automated, synthetic end-to-end checks.
// It is a development fixture, not part of the shipped app. It starts no
// window, no folder picker and no client: the smoke harness launches the
// installed clients itself, directly and independently, after startup.
//
// It reads one private JSON configuration file (--config) and writes one
// private JSON state file containing only counts and fixed codes. It never
// reads provider keys: the host refuses to start when its own environment
// carries provider credentials, and the configuration has no field for them.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/AndriGitDev/dmz/internal/companion"
)

type clientConfig struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type hostConfig struct {
	ControlFile string            `json:"control_file"`
	StateFile   string            `json:"state_file"`
	StateDir    string            `json:"background_state_dir"`
	Clients     []clientConfig    `json:"clients"`
	Settings    map[string]string `json:"settings"`
	Upstreams   map[string]string `json:"upstreams"`
}

type state struct {
	Harness             string `json:"harness"`
	ConfigValid         bool   `json:"config_valid"`
	ServeError          string `json:"serve_error,omitempty"`
	EnvironmentLookups  int    `json:"environment_lookups"`
	FolderPickerCalls   int    `json:"folder_picker_calls"`
	WindowOpenCalls     int    `json:"window_open_calls"`
	TerminalLaunchCalls int    `json:"terminal_launch_calls"`
}

// Credential-shaped variables the host must not inherit. Clients receive
// their own synthetic keys from the harness, never through this process.
var forbiddenEnv = []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CODEX_API_KEY", "OPENAI_BASE_URL", "ANTHROPIC_BASE_URL", "DMZ_LOCAL_TOKEN"}

func fail(code string) int {
	fmt.Fprintf(os.Stderr, "backgroundtesthost: %s\n", code)
	return 2
}

func main() { os.Exit(mainCode()) }

func mainCode() int {
	configPath := flag.String("config", "", "private JSON test configuration")
	flag.Parse()
	if *configPath == "" || flag.NArg() != 0 {
		return fail("usage")
	}
	for _, name := range forbiddenEnv {
		if _, ok := os.LookupEnv(name); ok {
			return fail("host_environment_has_provider_route_or_key")
		}
	}
	cfg, err := loadConfig(*configPath)
	if err != nil {
		return fail(err.Error())
	}
	var mu sync.Mutex
	st := &state{Harness: "dmz-background-testhost", ConfigValid: true}
	count := func(field *int) { mu.Lock(); *field++; mu.Unlock() }
	clients := make([]companion.ClientInstallation, 0, len(cfg.Clients))
	for _, c := range cfg.Clients {
		clients = append(clients, companion.ClientInstallation{ID: c.ID, Name: c.ID, Path: c.Path, Available: true})
	}
	discover := func() []companion.ClientInstallation { return append([]companion.ClientInstallation(nil), clients...) }
	app, err := companion.New(companion.Config{
		Clients: clients,
		OpenTerminal: func(string, string, string) error {
			count(&st.TerminalLaunchCalls)
			return errors.New("terminal_not_supported_in_background_testhost")
		},
		OpenWindow: func(string) error {
			count(&st.WindowOpenCalls)
			return errors.New("window_not_supported_in_background_testhost")
		},
		ChooseDirectory: func(context.Context) (string, error) {
			count(&st.FolderPickerCalls)
			return "", errors.New("folder_picker_not_supported_in_background_testhost")
		},
		Environment: func(string) string { count(&st.EnvironmentLookups); return "" },
		Background: &companion.BackgroundConfig{
			StateDir:  cfg.StateDir,
			Settings:  cfg.Settings,
			Upstreams: cfg.Upstreams,
			Discover:  discover,
		},
	})
	if err != nil {
		return fail("app_new_failed")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if serveErr := app.Serve(ctx, false, cfg.ControlFile); serveErr != nil {
		st.ServeError = fixedCode(serveErr.Error())
	}
	mu.Lock()
	defer mu.Unlock()
	if err := writeState(cfg.StateFile, st); err != nil {
		return fail("state_write_failed")
	}
	return 0
}

// fixedCode passes only short lowercase reason codes.
func fixedCode(s string) string {
	if len(s) == 0 || len(s) > 64 || strings.Trim(s, "abcdefghijklmnopqrstuvwxyz_") != "" {
		return "serve_failed"
	}
	return s
}

func writeState(name string, st *state) error {
	tmp := name + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	err = json.NewEncoder(f).Encode(st)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, name)
}

func loadConfig(name string) (hostConfig, error) {
	var cfg hostConfig
	f, err := os.Open(name)
	if err != nil {
		return cfg, errors.New("config_unavailable")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 65536 || !dirPrivate(filepath.Dir(name)) {
		return cfg, errors.New("config_not_private")
	}
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if d.Decode(&cfg) != nil || d.More() {
		return cfg, errors.New("config_invalid")
	}
	return cfg, validate(cfg)
}

func validate(cfg hostConfig) error {
	for _, p := range []string{cfg.ControlFile, cfg.StateFile} {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p || !dirPrivate(filepath.Dir(p)) {
			return errors.New("config_private_path_invalid")
		}
		if _, err := os.Lstat(p); err == nil {
			return errors.New("config_private_path_exists")
		}
	}
	if !filepath.IsAbs(cfg.StateDir) || filepath.Clean(cfg.StateDir) != cfg.StateDir || !dirPrivate(filepath.Dir(cfg.StateDir)) {
		return errors.New("config_state_dir_invalid")
	}
	if info, err := os.Lstat(cfg.StateDir); err == nil && (!info.IsDir() || info.Mode().Perm()&0077 != 0) {
		return errors.New("config_state_dir_invalid")
	}
	if len(cfg.Clients) == 0 || len(cfg.Clients) > 2 {
		return errors.New("config_clients_invalid")
	}
	seen := map[string]bool{}
	for _, c := range cfg.Clients {
		if (c.ID != "codex" && c.ID != "claude") || seen[c.ID] || !executableFile(c.Path) {
			return errors.New("config_client_invalid")
		}
		seen[c.ID] = true
		if !loopbackV1(cfg.Upstreams[c.ID]) {
			return errors.New("config_upstream_not_loopback_v1")
		}
		want := map[string]string{"codex": "config.toml", "claude": "settings.json"}[c.ID]
		p := cfg.Settings[c.ID]
		if !filepath.IsAbs(p) || filepath.Clean(p) != p || filepath.Base(p) != want || !dirPrivate(filepath.Dir(p)) {
			return errors.New("config_settings_path_invalid")
		}
		// Never touch the caller's own client configuration.
		if home, err := os.UserHomeDir(); err == nil {
			for _, own := range []string{filepath.Join(home, ".codex"), filepath.Join(home, ".claude")} {
				if strings.HasPrefix(p, own+string(filepath.Separator)) {
					return errors.New("config_settings_path_is_user_config")
				}
			}
		}
	}
	if len(cfg.Upstreams) != len(seen) || len(cfg.Settings) != len(seen) {
		return errors.New("config_unexpected_client_entry")
	}
	return nil
}

func dirPrivate(dir string) bool {
	info, err := os.Lstat(dir)
	return err == nil && info.IsDir() && info.Mode().Perm()&0077 == 0
}

func executableFile(p string) bool {
	if !filepath.IsAbs(p) {
		return false
	}
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0
}

func loopbackV1(v string) bool {
	u, err := url.Parse(v)
	return err == nil && u.Scheme == "http" && u.Hostname() == "127.0.0.1" && u.Port() != "" && u.User == nil && u.Path == "/v1" && u.RawQuery == "" && u.Fragment == ""
}
