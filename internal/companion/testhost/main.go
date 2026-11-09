//go:build linux

// SPDX-License-Identifier: GPL-3.0-only

// Command testhost runs the real DMZ companion controller for automated,
// synthetic end-to-end checks. It is a development fixture, not part of the
// shipped app: it replaces the native terminal launcher with a direct
// `dmz connect --ticket PATH` child, points sessions at loopback fake
// providers and captures every child stream in memory only.
//
// It reads one private JSON configuration file (--config) and writes one
// private JSON state file containing only counts, booleans, exit codes and
// fixed codes. It never prints child output, provider keys or local tokens.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/AndriGitDev/dmz/internal/companion"
)

type clientConfig struct {
	ID        string   `json:"id"`
	Path      string   `json:"path"`
	Arguments []string `json:"arguments"`
}

type hostConfig struct {
	DMZ         string            `json:"dmz"`
	ControlFile string            `json:"control_file"`
	StateFile   string            `json:"state_file"`
	Clients     []clientConfig    `json:"clients"`
	Upstreams   map[string]string `json:"upstreams"`
	Environment map[string]string `json:"environment"`
	// Probes are synthetic strings that must never appear in tickets or child
	// output (fake provider keys, original synthetic e-mail). FinalText must.
	Probes         []string `json:"probes"`
	StdoutProbes   []string `json:"stdout_probes"`
	FinalText      string   `json:"final_text"`
	Replay         bool     `json:"replay"`
	ConnectTimeout int      `json:"connect_timeout_seconds"`
	// TUI runs connect inside a pseudo-terminal (util-linux script) with the
	// production empty client arguments and answers known screens.
	TUI *tuiConfig `json:"tui,omitempty"`
}

type tuiRule struct {
	Code  string `json:"code"`
	Match string `json:"match"` // lowercase, whitespace-free text
	Keys  string `json:"keys"`
}

type tuiConfig struct {
	Script string    `json:"script"`
	Rules  []tuiRule `json:"rules"`
}

type tuiEvidence struct {
	ScreensAnswered []string `json:"screens_answered"`
	KeyBatches      int      `json:"key_batches"`
	PhrasesSeen     []string `json:"phrases_seen"`
}

// launchRecord holds aggregates only.
type launchRecord struct {
	Client                   string          `json:"client"`
	TicketPrivate            bool            `json:"ticket_private"`
	TicketFieldsExpected     bool            `json:"ticket_fields_expected"`
	TicketContainsProbe      bool            `json:"ticket_contains_probe"`
	TicketArgumentsFromSeam  bool            `json:"ticket_arguments_from_test_seam"`
	TicketTTLSeconds         int             `json:"ticket_ttl_seconds"`
	GatewayReachableBefore   bool            `json:"gateway_reachable_before_connect"`
	GatewayWrongTokenRefused bool            `json:"gateway_wrong_token_refused"`
	SpawnError               bool            `json:"spawn_error"`
	TicketConsumed           bool            `json:"ticket_consumed"`
	TicketDirRemoved         bool            `json:"ticket_dir_removed_after_exit"`
	ExitCode                 int             `json:"exit_code"`
	KilledByTesthost         bool            `json:"killed_by_testhost"`
	ExitedBeforeAppQuit      bool            `json:"exited_before_app_quit"`
	DurationMillis           int64           `json:"duration_ms"`
	ConnectGroupClean        bool            `json:"connect_group_clean_after_exit"`
	FinalAnswerDelivered     bool            `json:"final_answer_delivered"`
	ProbeInOutput            bool            `json:"probe_in_output"`
	StdoutProbeInOutput      bool            `json:"original_in_client_stdout"`
	LocalTokenInOutput       bool            `json:"local_token_in_output"`
	Hex64InOutput            bool            `json:"hex64_in_output"`
	AdapterActiveLine        bool            `json:"adapter_active_line"`
	SessionUnavailableLine   bool            `json:"session_unavailable_line"`
	Summary                  *[3]uint64      `json:"launcher_summary,omitempty"`
	GatewayReachableAfter    bool            `json:"gateway_reachable_after_exit"`
	ReuseRejected            *bool           `json:"reuse_same_path_rejected,omitempty"`
	Replay                   *replayEvidence `json:"replay,omitempty"`
	TUI                      *tuiEvidence    `json:"tui,omitempty"`
	pgid                     int
	done                     chan struct{}
}

type replayEvidence struct {
	ExitCode           int  `json:"exit_code"`
	ClientLaunched     bool `json:"client_launched"`
	SessionUnavailable bool `json:"session_unavailable_line"`
	TicketRejected     bool `json:"ticket_rejected_line"`
	TimedOut           bool `json:"timed_out"`
}

type state struct {
	Harness              string          `json:"harness"`
	ConfigValid          bool            `json:"config_valid"`
	ServeError           string          `json:"serve_error,omitempty"`
	EnvironmentLookups   int             `json:"environment_lookups"`
	EnvironmentNonEmpty  int             `json:"environment_nonempty_returns"`
	FolderPickerCalls    int             `json:"folder_picker_calls"`
	WindowOpenCalls      int             `json:"window_open_calls"`
	Launches             []*launchRecord `json:"launches"`
	ConnectsKilledAtExit int             `json:"connects_killed_at_exit"`
	TicketTempDirsLeft   int             `json:"ticket_temp_dirs_left"`
}

var (
	hex64       = regexp.MustCompile(`(?:^|[^0-9a-f])[0-9a-f]{64}(?:[^0-9a-f]|$)`)
	summaryLine = regexp.MustCompile(`(?m)^dmz: observed (\d+) model-route requests, (\d+) text parts, (\d+) changed parts\.$`)
	safeEnvName = map[string]bool{"PATH": true, "HOME": true, "USER": true, "LOGNAME": true, "LANG": true, "LC_ALL": true, "LC_CTYPE": true, "TZ": true, "SHELL": true, "TERM": true, "NO_COLOR": true, "TMPDIR": true,
		"HTTP_PROXY": true, "HTTPS_PROXY": true, "ALL_PROXY": true, "NO_PROXY": true, "http_proxy": true, "https_proxy": true, "all_proxy": true, "no_proxy": true,
		"CODEX_HOME": true, "CLAUDE_CONFIG_DIR": true, "CODEX_DISABLE_UPDATE_CHECK": true, "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": true, "DISABLE_TELEMETRY": true,
		"DISABLE_ERROR_REPORTING": true, "DISABLE_AUTOUPDATER": true, "DISABLE_BUG_COMMAND": true, "CLAUDE_CODE_DISABLE_TERMINAL_TITLE": true, "DMZ_SMOKE_RUN": true}
	proxyName = map[string]bool{"HTTP_PROXY": true, "HTTPS_PROXY": true, "ALL_PROXY": true, "http_proxy": true, "https_proxy": true, "all_proxy": true}
)

func fail(code string) int {
	fmt.Fprintf(os.Stderr, "testhost: %s\n", code)
	return 2
}

func main() { os.Exit(mainCode()) }

func mainCode() int {
	configPath := flag.String("config", "", "private JSON test configuration")
	flag.Parse()
	if *configPath == "" || flag.NArg() != 0 {
		return fail("usage")
	}
	cfg, err := loadConfig(*configPath)
	if err != nil {
		return fail(err.Error())
	}
	h := &host{cfg: cfg, st: &state{Harness: "dmz-companion-testhost", ConfigValid: true}, quitAt: make(chan struct{})}
	clients := make([]companion.ClientInstallation, 0, len(cfg.Clients))
	args := make(map[string][]string)
	for _, c := range cfg.Clients {
		clients = append(clients, companion.ClientInstallation{ID: c.ID, Name: c.ID, Path: c.Path, Available: true})
		args[c.ID] = append([]string(nil), c.Arguments...)
	}
	app, err := companion.New(companion.Config{
		Executable:   cfg.DMZ,
		Clients:      clients,
		OpenTerminal: h.openTerminal,
		OpenWindow: func(string) error {
			h.mu.Lock()
			h.st.WindowOpenCalls++
			h.mu.Unlock()
			return errors.New("window_not_supported_in_testhost")
		},
		ChooseDirectory: func(context.Context) (string, error) {
			h.mu.Lock()
			h.st.FolderPickerCalls++
			h.mu.Unlock()
			return "", errors.New("folder_picker_not_supported_in_testhost")
		},
		Environment:      h.environment,
		FixtureUpstreams: cfg.Upstreams,
		ClientArguments:  args,
	})
	if err != nil {
		return fail("app_new_failed")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveErr := app.Serve(ctx, false, cfg.ControlFile)
	close(h.quitAt)
	if serveErr != nil {
		h.st.ServeError = "serve_failed"
	}
	h.drain()
	h.st.TicketTempDirsLeft = ticketDirsLeft()
	if err := writeState(cfg.StateFile, h.snapshot()); err != nil {
		return fail("state_write_failed")
	}
	return 0
}

type host struct {
	cfg    hostConfig
	mu     sync.Mutex
	st     *state
	quitAt chan struct{}
	wg     sync.WaitGroup
}

// environment never returns provider credentials: the app flow under test
// must receive the synthetic key through POST /api/start.
func (h *host) environment(name string) string {
	h.mu.Lock()
	h.st.EnvironmentLookups++
	h.mu.Unlock()
	return ""
}

func (h *host) childEnv() []string {
	env := make([]string, 0, len(h.cfg.Environment))
	for k, v := range h.cfg.Environment {
		env = append(env, k+"="+v)
	}
	return env
}

func (h *host) openTerminal(executable, ticketPath, projectDir string) error {
	rec := &launchRecord{done: make(chan struct{}), ExitCode: -1}
	raw, err := os.ReadFile(ticketPath)
	info, statErr := os.Stat(ticketPath)
	if err != nil || statErr != nil {
		return errors.New("ticket_unreadable")
	}
	rec.TicketPrivate = info.Mode().IsRegular() && info.Mode().Perm()&0077 == 0 && dirPrivate(filepath.Dir(ticketPath))
	for _, p := range h.cfg.Probes {
		if p != "" && bytes.Contains(raw, []byte(p)) {
			rec.TicketContainsProbe = true
		}
	}
	var fields map[string]json.RawMessage
	var ticket companion.Ticket
	if json.Unmarshal(raw, &fields) == nil && json.Unmarshal(raw, &ticket) == nil {
		expected := map[string]bool{"version": true, "session_id": true, "client": true, "client_path": true, "project": true, "auth": true, "arguments": true, "gateway_url": true, "gateway_token": true, "control_url": true, "control_token": true, "expires_at": true}
		ok := len(fields) <= len(expected)
		for k := range fields {
			if !expected[k] {
				ok = false
			}
		}
		rec.TicketFieldsExpected = ok && ticket.Project == projectDir && executable == h.cfg.DMZ
		rec.Client = ticket.Client
		rec.TicketTTLSeconds = int(time.Until(ticket.ExpiresAt).Round(time.Second) / time.Second)
		for _, c := range h.cfg.Clients {
			if c.ID == ticket.Client {
				rec.TicketArgumentsFromSeam = equalStrings(c.Arguments, ticket.Arguments) && ticket.ClientPath == c.Path
			}
		}
	}
	rec.GatewayReachableBefore = probeGateway(ticket.GatewayURL, ticket.GatewayToken) == 200
	rec.GatewayWrongTokenRefused = probeGateway(ticket.GatewayURL, strings.Repeat("0", 64)) == 401
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(executable, "connect", "--ticket", ticketPath)
	outBuf := &limitedBuffer{buf: &stdout, max: 4 << 20}
	var keys io.WriteCloser
	if h.cfg.TUI != nil {
		line := "stty cols 120 rows 40; exec " + shellQuote(executable) + " connect --ticket " + shellQuote(ticketPath)
		cmd = exec.Command(h.cfg.TUI.Script, "-qefc", line, "/dev/null")
		keys, _ = cmd.StdinPipe()
		rec.TUI = &tuiEvidence{ScreensAnswered: []string{}}
	}
	cmd.Dir = projectDir
	cmd.Env = h.childEnv()
	cmd.Stdout = outBuf
	cmd.Stderr = &limitedBuffer{buf: &stderr, max: 4 << 20}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	h.mu.Lock()
	h.st.Launches = append(h.st.Launches, rec)
	h.mu.Unlock()
	start := time.Now()
	if err := cmd.Start(); err != nil {
		rec.SpawnError = true
		close(rec.done)
		return errors.New("connect_spawn_failed")
	}
	rec.pgid = cmd.Process.Pid
	if keys != nil {
		go h.answerScreens(rec, outBuf, keys)
	}
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		defer close(rec.done)
		timeout := time.Duration(h.cfg.ConnectTimeout) * time.Second
		waitErr := make(chan error, 1)
		go func() { waitErr <- cmd.Wait() }()
		var killed bool
		select {
		case <-waitErr:
		case <-time.After(timeout):
			killed = true
			_ = syscall.Kill(-rec.pgid, syscall.SIGKILL)
			<-waitErr
		}
		select {
		case <-h.quitAt:
		default:
			rec.ExitedBeforeAppQuit = true
		}
		rec.DurationMillis = time.Since(start).Milliseconds()
		rec.KilledByTesthost = killed
		rec.ExitCode = cmd.ProcessState.ExitCode()
		rec.ConnectGroupClean = syscall.Kill(-rec.pgid, 0) != nil
		_ = syscall.Kill(-rec.pgid, syscall.SIGKILL)
		_, err := os.Stat(ticketPath)
		rec.TicketConsumed = errors.Is(err, os.ErrNotExist)
		_, err = os.Stat(filepath.Dir(ticketPath))
		rec.TicketDirRemoved = errors.Is(err, os.ErrNotExist)
		rec.GatewayReachableAfter = probeGateway(ticket.GatewayURL, ticket.GatewayToken) != 0
		if keys != nil {
			keys.Close()
		}
		outText := outBuf.String()
		if rec.TUI != nil {
			whole := normalize(outText)
			rec.TUI.PhrasesSeen = []string{}
			for _, rule := range h.cfg.TUI.Rules {
				if rule.Keys == "" && strings.Contains(whole, rule.Match) {
					rec.TUI.PhrasesSeen = append(rec.TUI.PhrasesSeen, rule.Code)
				}
			}
			// A terminal screen interleaves streams and control sequences.
			outText = normalize(outText)
		}
		h.inspectOutput(rec, outText, stderr.String()+"\n"+outBuf.String(), ticket)
		stdout.Reset()
		stderr.Reset()
		if rec.TicketConsumed {
			// The same path cannot be used again.
			code, _, _ := h.runConnect(executable, ticketPath, projectDir, 10*time.Second)
			rejected := code != 0
			rec.ReuseRejected = &rejected
		}
		if h.cfg.Replay {
			rec.Replay = h.replay(executable, raw, projectDir)
		}
		for i := range raw {
			raw[i] = 0
		}
	}()
	return nil
}

func (h *host) inspectOutput(rec *launchRecord, out, errOut string, t companion.Ticket) {
	all := out + "\n" + errOut
	final := h.cfg.FinalText
	if rec.TUI != nil {
		final = normalize(final)
	}
	rec.FinalAnswerDelivered = final != "" && strings.Contains(out, final)
	for _, p := range h.cfg.Probes {
		if p != "" && strings.Contains(all, p) {
			rec.ProbeInOutput = true
		}
	}
	for _, p := range h.cfg.StdoutProbes {
		if rec.TUI != nil {
			p = normalize(p)
		}
		if p != "" && strings.Contains(out, p) {
			rec.StdoutProbeInOutput = true
		}
	}
	rec.LocalTokenInOutput = (t.GatewayToken != "" && strings.Contains(all, t.GatewayToken)) || (t.ControlToken != "" && strings.Contains(all, t.ControlToken))
	rec.Hex64InOutput = hex64.MatchString(all)
	rec.AdapterActiveLine = strings.Contains(errOut, "dmz: local adapter active")
	rec.SessionUnavailableLine = strings.Contains(errOut, "dmz: the app session")
	if m := summaryLine.FindStringSubmatch(errOut); m != nil {
		var s [3]uint64
		for i := 0; i < 3; i++ {
			fmt.Sscan(m[i+1], &s[i])
		}
		rec.Summary = &s
	}
}

// replay rewrites a captured ticket after the session finished. A stopped
// session must not launch the client again.
func (h *host) replay(executable string, raw []byte, projectDir string) *replayEvidence {
	dir, err := os.MkdirTemp("", "dmz-testhost-replay-")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(dir)
	name := filepath.Join(dir, "launch.json")
	if os.WriteFile(name, raw, 0600) != nil {
		return nil
	}
	code, errOut, timedOut := h.runConnect(executable, name, projectDir, 20*time.Second)
	return &replayEvidence{ExitCode: code, ClientLaunched: strings.Contains(errOut, "dmz: local adapter active"), TimedOut: timedOut,
		SessionUnavailable: strings.Contains(errOut, "session is no longer available"), TicketRejected: strings.Contains(errOut, "could not be opened")}
}

func (h *host) runConnect(executable, ticketPath, projectDir string, timeout time.Duration) (int, string, bool) {
	var out, errOut bytes.Buffer
	cmd := exec.Command(executable, "connect", "--ticket", ticketPath)
	cmd.Dir = projectDir
	cmd.Env = h.childEnv()
	cmd.Stdout = &limitedBuffer{buf: &out, max: 1 << 20}
	cmd.Stderr = &limitedBuffer{buf: &errOut, max: 1 << 20}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if cmd.Start() != nil {
		return -1, "", false
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	timedOut := false
	select {
	case <-done:
	case <-time.After(timeout):
		timedOut = true
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-done
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	return cmd.ProcessState.ExitCode(), errOut.String(), timedOut
}

// drain waits for connect children after the app has quit, then kills any
// survivors so no process outlives the fixture.
func (h *host) drain() {
	h.mu.Lock()
	launches := append([]*launchRecord(nil), h.st.Launches...)
	h.mu.Unlock()
	deadline := time.After(25 * time.Second)
	for _, rec := range launches {
		select {
		case <-rec.done:
		case <-deadline:
			if rec.pgid > 0 {
				_ = syscall.Kill(-rec.pgid, syscall.SIGKILL)
				h.st.ConnectsKilledAtExit++
			}
		}
	}
	h.wg.Wait()
}

func (h *host) snapshot() *state {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.st
}

func probeGateway(base, token string) int {
	if base == "" {
		return 0
	}
	r, err := http.NewRequest("GET", base+"/_dmz/status", nil)
	if err != nil {
		return 0
	}
	r.Header.Set("X-Dmz-Token", token)
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}}
	resp, err := client.Do(r)
	if err != nil {
		return 0
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 65536))
	resp.Body.Close()
	return resp.StatusCode
}

func ticketDirsLeft() int {
	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "dmz-session-*"))
	return len(matches)
}

func dirPrivate(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir() && info.Mode().Perm()&0077 == 0
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var ansi = regexp.MustCompile(`\x1b(\[[0-9;?<>=]*[ -/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\)|[()][0-9A-Za-z]|[@-Z\\-_])`)

// normalize removes control sequences and whitespace so rendered screens can
// be matched against fixed lowercase phrases.
func normalize(s string) string {
	s = ansi.ReplaceAllString(s, "")
	var b strings.Builder
	for _, r := range s {
		if r > ' ' && r != 0x7f {
			b.WriteRune(r)
		}
	}
	return strings.ToLower(b.String())
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'" }

// answerScreens sends fixed keys when a known screen appears. Only the codes
// of answered screens are recorded.
func (h *host) answerScreens(rec *launchRecord, out *limitedBuffer, keys io.Writer) {
	fired := make([]bool, len(h.cfg.TUI.Rules))
	cursor := 0
	for {
		select {
		case <-rec.done:
			return
		case <-time.After(300 * time.Millisecond):
		}
		text := out.String()
		if cursor > len(text) {
			cursor = len(text)
		}
		fresh := normalize(text[cursor:])
		for i, rule := range h.cfg.TUI.Rules {
			if fired[i] || rule.Keys == "" || !strings.Contains(fresh, rule.Match) {
				continue
			}
			time.Sleep(700 * time.Millisecond)
			for _, chunk := range strings.SplitAfter(rule.Keys, "\r") {
				if chunk == "" {
					continue
				}
				// Typed text and its Enter are separate writes so clients
				// with paste detection treat Enter as submit.
				if body := strings.TrimSuffix(chunk, "\r"); body != chunk && body != "" {
					if _, err := io.WriteString(keys, body); err != nil {
						return
					}
					time.Sleep(900 * time.Millisecond)
					chunk = "\r"
				}
				if _, err := io.WriteString(keys, chunk); err != nil {
					return
				}
				time.Sleep(250 * time.Millisecond)
			}
			fired[i] = true
			h.mu.Lock()
			rec.TUI.ScreensAnswered = append(rec.TUI.ScreensAnswered, rule.Code)
			rec.TUI.KeyBatches++
			h.mu.Unlock()
			cursor = len(text)
			break
		}
	}
}

type limitedBuffer struct {
	mu  sync.Mutex
	buf *bytes.Buffer
	max int
}

func (l *limitedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if room := l.max - l.buf.Len(); room > 0 {
		if len(p) > room {
			l.buf.Write(p[:room])
		} else {
			l.buf.Write(p)
		}
	}
	return len(p), nil
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
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 65536 {
		return cfg, errors.New("config_not_private")
	}
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if d.Decode(&cfg) != nil {
		return cfg, errors.New("config_invalid")
	}
	return cfg, validate(cfg)
}

func validate(cfg hostConfig) error {
	if !executableFile(cfg.DMZ) {
		return errors.New("config_dmz_invalid")
	}
	for _, p := range []string{cfg.ControlFile, cfg.StateFile} {
		if !filepath.IsAbs(p) || !dirPrivate(filepath.Dir(p)) {
			return errors.New("config_private_path_invalid")
		}
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
		if cfg.Upstreams[c.ID] == "" {
			return errors.New("config_upstream_missing")
		}
	}
	for id, u := range cfg.Upstreams {
		if !seen[id] || !loopbackV1(u) {
			return errors.New("config_upstream_not_loopback_v1")
		}
	}
	for k, v := range cfg.Environment {
		if !safeEnvName[k] || strings.ContainsAny(v, "\x00\r\n") {
			return errors.New("config_environment_name_not_allowed")
		}
		if proxyName[k] && !loopbackOrigin(v) {
			return errors.New("config_proxy_not_loopback")
		}
	}
	if cfg.Environment["DMZ_SMOKE_RUN"] == "" || cfg.Environment["TMPDIR"] == "" {
		return errors.New("config_environment_incomplete")
	}
	if cfg.TUI != nil {
		if !executableFile(cfg.TUI.Script) || len(cfg.TUI.Rules) == 0 || len(cfg.TUI.Rules) > 128 {
			return errors.New("config_tui_invalid")
		}
		for _, r := range cfg.TUI.Rules {
			if r.Code == "" || r.Match == "" || r.Match != strings.ToLower(r.Match) || len(r.Keys) > 4096 {
				return errors.New("config_tui_rule_invalid")
			}
		}
	}
	if cfg.ConnectTimeout < 5 || cfg.ConnectTimeout > 600 {
		return errors.New("config_timeout_invalid")
	}
	return nil
}

func executableFile(p string) bool {
	if !filepath.IsAbs(p) {
		return false
	}
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0
}

func loopbackOrigin(v string) bool {
	u, err := url.Parse(v)
	return err == nil && u.Scheme == "http" && u.Hostname() == "127.0.0.1" && u.Port() != "" && u.User == nil && (u.Path == "" || u.Path == "/") && u.RawQuery == ""
}

func loopbackV1(v string) bool {
	u, err := url.Parse(v)
	return err == nil && u.Scheme == "http" && u.Hostname() == "127.0.0.1" && u.Port() != "" && u.User == nil && u.Path == "/v1" && u.RawQuery == "" && u.Fragment == ""
}
