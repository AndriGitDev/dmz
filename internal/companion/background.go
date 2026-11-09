package companion

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/AndriGitDev/dmz/internal/agentgateway"
	"github.com/AndriGitDev/dmz/internal/companion/routing"
)

// BackgroundConfig is controlled by the native application, never by web input.
// Explicit paths and upstreams are available only to isolated fixture hosts.
type BackgroundConfig struct {
	StateDir  string
	Settings  map[string]string
	Upstreams map[string]string
	Discover  func() []ClientInstallation
	Planners  map[string]func([]byte, string, string) ([]byte, error)
}
type BackgroundView struct {
	Client       string            `json:"client"`
	Name         string            `json:"name"`
	Detected     bool              `json:"detected"`
	State        string            `json:"state"`
	Routed       bool              `json:"routed"`
	Issue        string            `json:"issue,omitempty"`
	Requests     uint64            `json:"requests"`
	ChangedParts uint64            `json:"changed_parts"`
	Warnings     map[string]uint64 `json:"warnings"`
}
type backgroundRoute struct {
	view            BackgroundView
	settings, state string
	handler         *agentgateway.Server
	server          *http.Server
}
type backgroundManager struct {
	mu       sync.Mutex
	cfg      BackgroundConfig
	routes   map[string]*backgroundRoute
	ports    map[string]int
	tokens   map[string]string
	disabled map[string]bool
	lock     *os.File
	closed   bool
}

func newBackground(cfg BackgroundConfig) (*backgroundManager, error) {
	if cfg.StateDir == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return nil, errors.New("background_state_unavailable")
		}
		cfg.StateDir = filepath.Join(dir, "dmz", "background")
	}
	if !filepath.IsAbs(cfg.StateDir) || os.MkdirAll(cfg.StateDir, 0700) != nil || os.Chmod(cfg.StateDir, 0700) != nil {
		return nil, errors.New("background_state_unavailable")
	}
	lock, err := os.OpenFile(filepath.Join(cfg.StateDir, "routing.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, errors.New("background_state_unavailable")
	}
	if lockBackground(lock) != nil {
		lock.Close()
		return nil, errors.New("background_already_running")
	}
	if cfg.Discover == nil {
		cfg.Discover = DiscoverClients
	}
	if cfg.Planners == nil {
		cfg.Planners = map[string]func([]byte, string, string) ([]byte, error){"codex": routing.CodexSettings, "claude": routing.ClaudeSettings}
	}
	m := &backgroundManager{cfg: cfg, routes: map[string]*backgroundRoute{}, ports: map[string]int{}, tokens: map[string]string{}, disabled: map[string]bool{}, lock: lock}
	readStateJSON(filepath.Join(cfg.StateDir, "ports.json"), &m.ports)
	readStateJSON(filepath.Join(cfg.StateDir, "disabled.json"), &m.disabled)
	readStateJSON(filepath.Join(cfg.StateDir, "tokens.json"), &m.tokens)
	if m.ports == nil {
		m.ports = map[string]int{}
	}
	if m.tokens == nil {
		m.tokens = map[string]string{}
	}
	if m.disabled == nil {
		m.disabled = map[string]bool{}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		lock.Close()
		return nil, errors.New("background_state_unavailable")
	}
	if cfg.Settings == nil {
		codex := os.Getenv("CODEX_HOME")
		if codex == "" {
			codex = filepath.Join(home, ".codex")
		}
		claude := os.Getenv("CLAUDE_CONFIG_DIR")
		if claude == "" {
			claude = filepath.Join(home, ".claude")
		}
		cfg.Settings = map[string]string{"codex": filepath.Join(codex, "config.toml"), "claude": filepath.Join(claude, "settings.json")}
		m.cfg.Settings = cfg.Settings
	}
	for _, client := range cfg.Discover() {
		if client.ID != "codex" && client.ID != "claude" {
			continue
		}
		r := &backgroundRoute{view: BackgroundView{Client: client.ID, Name: client.Name, Detected: client.Available, State: "not_detected"}, settings: cfg.Settings[client.ID], state: filepath.Join(cfg.StateDir, client.ID+"-routing.json")}
		m.routes[client.ID] = r
		if _, err := os.Lstat(r.state); err == nil {
			if err = m.restoreRoute(r); err != nil {
				r.view.State = "needs_attention"
				r.view.Issue = "routing_recovery_conflict"
				continue
			}
		}
		if !client.Available {
			continue
		}
		if m.disabled[client.ID] {
			r.view.State = "disconnected"
			continue
		}
		m.connectLocked(r)
	}
	return m, nil
}
func readStateJSON(path string, dest any) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	_ = json.NewDecoder(io.LimitReader(file, 16384)).Decode(dest)
}
func writeStateJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".dmz-background-")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if temp.Chmod(0600) != nil {
		temp.Close()
		return errors.New("background_state_unavailable")
	}
	if _, err = temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	syncErr := temp.Sync()
	closeErr := temp.Close()
	if syncErr != nil || closeErr != nil {
		return errors.New("background_state_unavailable")
	}
	return os.Rename(temp.Name(), path)
}
func (m *backgroundManager) connectLocked(r *backgroundRoute) {
	if m.closed || r.handler != nil {
		return
	}
	r.view.Issue = ""
	fail := func(code string) { r.view.State = "needs_attention"; r.view.Issue = code }
	if !filepath.IsAbs(r.settings) {
		fail("routing_settings_unavailable")
		return
	}
	if r.view.Client == "codex" {
		profiles, err := filepath.Glob(filepath.Join(filepath.Dir(r.settings), "*.config.toml"))
		if err != nil || len(profiles) > 0 {
			fail("codex_profiles_require_connection_review")
			return
		}
	}
	plan := m.cfg.Planners[r.view.Client]
	if plan == nil {
		fail("routing_adapter_unavailable")
		return
	}
	var original []byte
	info, err := os.Lstat(r.settings)
	if err == nil {
		if !info.Mode().IsRegular() || info.Size() > 4<<20 {
			fail("routing_settings_unavailable")
			return
		}
		original, err = os.ReadFile(r.settings)
	} else if os.IsNotExist(err) {
		err = nil
	}
	if err != nil {
		fail("routing_settings_unavailable")
		return
	}
	token := m.tokens[r.view.Client]
	decoded, tokenErr := hex.DecodeString(token)
	if tokenErr != nil || len(decoded) != 32 {
		var err error
		token, err = randomToken()
		if err != nil {
			fail("local_service_unavailable")
			return
		}
		m.tokens[r.view.Client] = token
		if writeStateJSON(filepath.Join(m.cfg.StateDir, "tokens.json"), m.tokens) != nil {
			fail("background_state_unavailable")
			return
		}
	}
	protocol := agentgateway.Responses
	upstream := "https://api.openai.com/v1"
	if r.view.Client == "claude" {
		protocol = agentgateway.Messages
		upstream = "https://api.anthropic.com/v1"
	}
	gatewayCfg := agentgateway.Config{Protocol: protocol, Upstream: upstream, Token: token, NativeOpenAIAuth: r.view.Client == "codex"}
	if r.view.Client == "claude" {
		gatewayCfg.SessionHeader = "X-Claude-Code-Session-Id"
	}
	if fixture := m.cfg.Upstreams[r.view.Client]; fixture != "" {
		gatewayCfg.Upstream = fixture
		gatewayCfg.AllowLoopbackUpstream = true
	}
	handler, err := agentgateway.NewServer(gatewayCfg)
	if err != nil {
		fail("connection_invalid")
		return
	}
	port := m.ports[r.view.Client]
	if port < 1 || port > 65535 {
		port = 0
	}
	listener, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil && port != 0 {
		listener, err = net.Listen("tcp4", "127.0.0.1:0")
	}
	if err != nil {
		fail("local_service_unavailable")
		return
	}
	origin := "http://" + listener.Addr().String()
	replacement, err := plan(original, origin, token)
	if err != nil {
		listener.Close()
		if errors.Is(err, routing.ErrCustomRoute) {
			fail("routing_custom_provider_present")
		} else {
			fail("routing_settings_unavailable")
		}
		return
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 60 * time.Second, IdleTimeout: time.Minute, ErrorLog: log.New(io.Discard, "", 0)}
	// Serve before publishing the route to a client configuration.
	go func() { _ = server.Serve(listener) }()
	m.ports[r.view.Client] = listener.Addr().(*net.TCPAddr).Port
	if writeStateJSON(filepath.Join(m.cfg.StateDir, "ports.json"), m.ports) != nil {
		_ = server.Close()
		fail("background_state_unavailable")
		return
	}
	if err = routing.Install(r.settings, r.state, original, replacement); err != nil {
		_ = server.Close()
		if restoreErr := m.restoreRoute(r); restoreErr != nil && !os.IsNotExist(restoreErr) {
			fail("routing_recovery_conflict")
		} else {
			fail("routing_settings_changed")
		}
		return
	}
	r.handler = handler
	r.server = server
	r.view.State = "restart_required"
}
func (m *backgroundManager) Views() []BackgroundView {
	m.mu.Lock()
	defer m.mu.Unlock()
	views := make([]BackgroundView, 0, len(m.routes))
	for _, r := range m.routes {
		view := r.view
		view.Routed = r.handler != nil
		if r.handler != nil {
			status := r.handler.Snapshot()
			view.Requests = status.ModelRequests
			view.ChangedParts = status.ChangedParts
			view.Warnings = status.Warnings
			if status.NativeCredentialMissing {
				view.Issue = "codex_authentication_required"
			}
			needsAttention := view.Issue != ""
			for code, count := range status.Warnings {
				if count > 0 && code != "instructions_unchanged" && code != "tool_result_unchanged" && code != "attachment_unchanged" && code != "no_user_text" {
					needsAttention = true
				}
			}
			if needsAttention {
				view.State = "needs_attention"
			} else if status.ModelRequests > 0 {
				view.State = "traffic_observed"
			}
		}
		views = append(views, view)
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Client < views[j].Client })
	return views
}
func (m *backgroundManager) disconnectLocked(r *backgroundRoute) error {
	if r.handler == nil {
		return nil
	}
	err := m.restoreRoute(r)
	if err != nil {
		r.view.Issue = "routing_restore_conflict"
		r.view.State = "needs_attention"
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = r.server.Shutdown(ctx)
	_ = r.server.Close()
	r.handler.ClearMappings()
	status := r.handler.Snapshot()
	r.view.Requests = status.ModelRequests
	r.view.ChangedParts = status.ChangedParts
	r.view.Warnings = status.Warnings
	r.handler = nil
	r.server = nil
	r.view.State = "disconnected"
	r.view.Issue = ""
	return nil
}
func (m *backgroundManager) SetConnected(client string, connected bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.routes[client]
	if r == nil || m.closed || (connected && !r.view.Detected) {
		return errors.New("client_missing")
	}
	if connected {
		m.disabled[client] = false
		if _, err := os.Lstat(r.state); err == nil && r.handler == nil {
			if m.restoreRoute(r) != nil {
				return errors.New("routing_recovery_conflict")
			}
		}
		m.connectLocked(r)
		if r.handler == nil {
			return errors.New(r.view.Issue)
		}
	} else {
		if err := m.disconnectLocked(r); err != nil {
			return errors.New("routing_restore_conflict")
		}
		m.disabled[client] = true
	}
	if writeStateJSON(filepath.Join(m.cfg.StateDir, "disabled.json"), m.disabled) != nil {
		return errors.New("background_state_unavailable")
	}
	return nil
}
func (m *backgroundManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	m.closed = true
	for _, r := range m.routes {
		if m.disconnectLocked(r) != nil && r.server != nil {
			_ = r.server.Close()
			r.handler.ClearMappings()
			r.handler = nil
			r.server = nil
		}
	}
	if m.lock != nil {
		_ = m.lock.Close()
		m.lock = nil
	}
}

// Refresh detects clients installed after DMZ started. It never reads process
// command lines or client authentication stores.
func (m *backgroundManager) Refresh() {
	clients := m.cfg.Discover()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	for _, client := range clients {
		r := m.routes[client.ID]
		if r == nil {
			continue
		}
		r.view.Detected = client.Available
		if client.Available && r.handler == nil && !m.disabled[client.ID] && r.view.Issue != "routing_recovery_conflict" {
			m.connectLocked(r)
		}
	}
}

func (a *App) BackgroundViews() []BackgroundView {
	a.mu.Lock()
	manager := a.background
	a.mu.Unlock()
	if manager == nil {
		return []BackgroundView{}
	}
	return manager.Views()
}

func (m *backgroundManager) restoreRoute(r *backgroundRoute) error {
	switch r.view.Client {
	case "codex":
		return routing.RestoreMerge(r.state, routing.MergeCodexSettings)
	case "claude":
		return routing.RestoreMerge(r.state, routing.MergeClaudeSettings)
	default:
		return routing.ErrUnavailable
	}
}
