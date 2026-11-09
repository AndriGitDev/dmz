//go:build linux

// SPDX-License-Identifier: GPL-3.0-only

// Command nativetesthost is a Linux-only development fixture. It uses the
// production terminal launcher with isolated fake client state and loopback
// providers. It is never included in a DMZ app package.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/AndriGitDev/dmz/internal/companion"
)

type config struct {
	Executable  string   `json:"executable"`
	ControlFile string   `json:"control_file"`
	Client      string   `json:"client"`
	ClientPath  string   `json:"client_path"`
	Arguments   []string `json:"arguments"`
	Upstream    string   `json:"upstream"`
	OpenWindow  bool     `json:"open_window,omitempty"`
}

func readConfig(name string) (config, error) {
	var cfg config
	file, err := os.Open(name)
	if err != nil {
		return cfg, errors.New("native_fixture_config_unavailable")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 16384 {
		return cfg, errors.New("native_fixture_config_not_private")
	}
	d := json.NewDecoder(io.LimitReader(file, 16385))
	d.DisallowUnknownFields()
	if d.Decode(&cfg) != nil {
		return cfg, errors.New("native_fixture_config_invalid")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return cfg, errors.New("native_fixture_config_invalid")
	}
	u, err := url.Parse(cfg.Upstream)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.Path != "/v1" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return cfg, errors.New("native_fixture_upstream_not_loopback")
	}
	if (cfg.Client != "codex" && cfg.Client != "claude") || !filepath.IsAbs(cfg.Executable) || !filepath.IsAbs(cfg.ClientPath) || !filepath.IsAbs(cfg.ControlFile) || len(cfg.Arguments) > 128 {
		return cfg, errors.New("native_fixture_config_invalid")
	}
	for _, key := range []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN"} {
		if os.Getenv(key) != "" {
			return cfg, errors.New("native_fixture_parent_credential_present")
		}
	}
	return cfg, nil
}

func main() { os.Exit(run()) }
func run() int {
	name := flag.String("config", "", "private synthetic configuration")
	flag.Parse()
	if *name == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "native_fixture_usage")
		return 2
	}
	cfg, err := readConfig(*name)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	app, err := companion.New(companion.Config{
		Executable:       cfg.Executable,
		Clients:          []companion.ClientInstallation{{ID: cfg.Client, Name: cfg.Client, Path: cfg.ClientPath, Available: true}},
		ClientArguments:  map[string][]string{cfg.Client: cfg.Arguments},
		FixtureUpstreams: map[string]string{cfg.Client: cfg.Upstream},
		Environment:      func(string) string { return "" },
		InstanceFile:     cfg.ControlFile,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "native_fixture_app_failed")
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	if err := app.Serve(ctx, cfg.OpenWindow, cfg.ControlFile); err != nil {
		fmt.Fprintln(os.Stderr, "native_fixture_serve_failed")
		return 1
	}
	return 0
}
