package companion

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/AndriGitDev/dmz/internal/agentgateway"
)

type Config struct {
	Executable      string
	Clients         []ClientInstallation
	OpenTerminal    func(string, string, string) error
	OpenWindow      func(string) error
	ChooseDirectory func(context.Context) (string, error)
	Environment     func(string) string
	// Fixture origins are a test seam, never accepted by the app HTTP API.
	FixtureUpstreams map[string]string
	ClientArguments  map[string][]string // test seam; never accepted from HTTP
	InstanceFile     string              // private app-only descriptor; no provider keys or maps
	Background       *BackgroundConfig   // native background integration; nil retains launcher mode
}

type StartRequest struct {
	Client  string `json:"client"`
	Project string `json:"project"`
	Auth    string `json:"auth"`
	APIKey  string `json:"api_key,omitempty"`
}
type SessionView struct {
	ID           string            `json:"id"`
	Client       string            `json:"client"`
	State        string            `json:"state"`
	Issue        string            `json:"issue,omitempty"`
	Started      time.Time         `json:"started"`
	Requests     uint64            `json:"requests"`
	ChangedParts uint64            `json:"changed_parts"`
	Warnings     map[string]uint64 `json:"warnings"`
}
type activeSession struct {
	view          SessionView
	finalStatus   agentgateway.Status
	handler       *agentgateway.Server
	server        *http.Server
	controlToken  string
	ticketDir     string
	done          chan struct{}
	once          sync.Once
	lastSeen      time.Time
	connectorDone chan struct{}
	connectorOnce sync.Once
	stoppedAt     time.Time
}
type App struct {
	cfg           Config
	mu            sync.Mutex
	sessions      map[string]*activeSession
	token         string
	origin        string
	quit          chan struct{}
	quitOnce      sync.Once
	closeOnce     sync.Once
	closing       bool
	windowCodes   map[string]time.Time
	helperCtx     context.Context
	cancelHelpers context.CancelFunc
	background    *backgroundManager
}

func New(cfg Config) (*App, error) {
	if cfg.Executable == "" {
		var err error
		cfg.Executable, err = os.Executable()
		if err != nil {
			return nil, errors.New("app_executable_unavailable")
		}
	}
	if cfg.Clients == nil {
		cfg.Clients = DiscoverClients()
	}
	if cfg.OpenTerminal == nil {
		cfg.OpenTerminal = OpenTerminal
	}
	if cfg.OpenWindow == nil {
		cfg.OpenWindow = OpenWindow
	}
	if cfg.ChooseDirectory == nil {
		cfg.ChooseDirectory = ChooseDirectoryContext
	}
	if cfg.Environment == nil {
		cfg.Environment = os.Getenv
	}
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	helperCtx, cancelHelpers := context.WithCancel(context.Background())
	return &App{cfg: cfg, sessions: make(map[string]*activeSession), token: token, quit: make(chan struct{}), windowCodes: make(map[string]time.Time), helperCtx: helperCtx, cancelHelpers: cancelHelpers}, nil
}

func (a *App) Start(req StartRequest) (SessionView, error) {
	if req.Client != "codex" && req.Client != "claude" {
		return SessionView{}, errors.New("client_missing")
	}
	if req.Auth != "account" && req.Auth != "api" {
		return SessionView{}, errors.New("connection_invalid")
	}
	if !filepath.IsAbs(req.Project) {
		return SessionView{}, errors.New("choose_project")
	}
	info, err := os.Stat(req.Project)
	if err != nil || !info.IsDir() {
		return SessionView{}, errors.New("choose_project")
	}
	var clientPath string
	for _, client := range a.cfg.Clients {
		if client.ID == req.Client && client.Available {
			clientPath = client.Path
		}
	}
	if clientPath == "" {
		return SessionView{}, errors.New("client_missing")
	}
	key := ""
	if req.Auth == "api" {
		key = req.APIKey
		if key == "" {
			envName := "OPENAI_API_KEY"
			if req.Client == "claude" {
				envName = "ANTHROPIC_API_KEY"
			}
			key = a.cfg.Environment(envName)
		}
		if key == "" {
			return SessionView{}, errors.New("api_key_required")
		}
		if len(key) > 8192 || !validAPIKey(key) {
			return SessionView{}, errors.New("api_key_invalid")
		}
	}
	id, err := randomToken()
	if err != nil {
		return SessionView{}, err
	}
	id = id[:24]
	gateToken, err := randomToken()
	if err != nil {
		return SessionView{}, err
	}
	controlToken, err := randomToken()
	if err != nil {
		return SessionView{}, err
	}
	protocol := agentgateway.Responses
	upstream := "https://api.openai.com/v1"
	if req.Auth == "account" {
		upstream = "https://chatgpt.com/backend-api/codex"
	}
	if req.Client == "claude" {
		protocol = agentgateway.Messages
		upstream = "https://api.anthropic.com/v1"
	}
	cfg := agentgateway.Config{Protocol: protocol, Upstream: upstream, Token: gateToken, RestoreReplies: true}
	if req.Auth == "api" {
		cfg.LocalAuth = agentgateway.AuthBearer
		cfg.ProviderKey = key
	}
	if req.Client == "claude" {
		cfg.SessionHeader = "X-Claude-Code-Session-Id"
	}
	if fixture := a.cfg.FixtureUpstreams[req.Client]; fixture != "" {
		cfg.Upstream = fixture
		cfg.AllowLoopbackUpstream = true
	}
	handler, err := agentgateway.NewServer(cfg)
	if err != nil {
		return SessionView{}, errors.New("connection_invalid")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return SessionView{}, errors.New("local_service_unavailable")
	}
	dir, err := os.MkdirTemp("", "dmz-session-")
	if err != nil {
		listener.Close()
		return SessionView{}, errors.New("session_ticket_unavailable")
	}
	view := SessionView{ID: id, Client: req.Client, State: "waiting", Started: time.Now().UTC()}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 60 * time.Second, IdleTimeout: time.Minute, ErrorLog: log.New(io.Discard, "", 0)}
	session := &activeSession{view: view, handler: handler, server: server, controlToken: controlToken, ticketDir: dir, done: make(chan struct{}), connectorDone: make(chan struct{})}
	a.mu.Lock()
	active := 0
	for _, s := range a.sessions {
		if s.view.State == "running" || s.view.State == "waiting" {
			active++
		}
	}
	if a.closing || active >= 4 {
		a.mu.Unlock()
		listener.Close()
		os.RemoveAll(dir)
		return SessionView{}, errors.New("session_limit")
	}
	a.pruneStoppedLocked()
	if len(a.sessions) >= 24 {
		a.mu.Unlock()
		listener.Close()
		os.RemoveAll(dir)
		return SessionView{}, errors.New("session_limit")
	}
	a.sessions[id] = session
	origin := a.origin
	a.mu.Unlock()
	ticket := Ticket{Version: 1, SessionID: id, Client: req.Client, ClientPath: clientPath, Project: req.Project, Auth: req.Auth, Arguments: append([]string(nil), a.cfg.ClientArguments[req.Client]...), GatewayURL: "http://" + listener.Addr().String(), GatewayToken: gateToken, ControlURL: origin, ControlToken: controlToken, ExpiresAt: time.Now().Add(90 * time.Second)}
	name := filepath.Join(dir, "launch.json")
	encoded, _ := json.Marshal(ticket)
	if err := os.WriteFile(name, encoded, 0600); err != nil {
		listener.Close()
		a.stop(id, "launch_failed")
		return SessionView{}, errors.New("session_ticket_unavailable")
	}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.stop(id, "local_service_unavailable")
		}
	}()
	if err := a.cfg.OpenTerminal(a.cfg.Executable, name, req.Project); err != nil {
		a.stop(id, "terminal_unavailable")
		return SessionView{}, errors.New("terminal_unavailable")
	}
	go func() {
		timer := time.NewTimer(90 * time.Second)
		defer timer.Stop()
		heartbeat := time.NewTicker(2 * time.Second)
		defer heartbeat.Stop()
		for {
			select {
			case <-session.done:
				return
			case now := <-heartbeat.C:
				a.expireDisconnected(id, now)
			case <-timer.C:
				a.mu.Lock()
				waiting := session.view.State == "waiting"
				a.mu.Unlock()
				if waiting {
					a.stop(id, "terminal_unavailable")
				}
			}
		}
	}()
	return view, nil
}

func (a *App) stop(id, issue string) {
	a.mu.Lock()
	s := a.sessions[id]
	if s == nil {
		a.mu.Unlock()
		return
	}
	if s.view.State != "stopped" {
		s.view.State = "stopped"
		s.stoppedAt = time.Now()
		if issue != "" {
			s.view.Issue = issue
		}
	}
	a.mu.Unlock()
	s.once.Do(func() {
		close(s.done)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.server.Shutdown(ctx)
		_ = s.server.Close()
		_ = os.RemoveAll(s.ticketDir)
		snapshot := s.handler.Snapshot()
		s.handler.ClearMappings()
		a.mu.Lock()
		s.finalStatus = snapshot
		s.view.Requests = snapshot.Requests
		s.view.ChangedParts = snapshot.ChangedParts
		s.view.Warnings = snapshot.Warnings
		s.server = nil
		s.handler = nil
		a.mu.Unlock()
	})
}
func (a *App) Close() {
	a.closeOnce.Do(a.close)
}
func (a *App) close() {
	a.cancelHelpers()
	a.mu.Lock()
	background := a.background
	a.mu.Unlock()
	if background != nil {
		background.Close()
	}
	a.mu.Lock()
	a.closing = true
	ids := make([]string, 0, len(a.sessions))
	for id := range a.sessions {
		ids = append(ids, id)
	}
	a.mu.Unlock()
	var stopped sync.WaitGroup
	for _, id := range ids {
		stopped.Add(1)
		go func() { defer stopped.Done(); a.stop(id, "") }()
	}
	stopped.Wait()
	// Keep the control endpoint alive briefly so connectors can collect their
	// final aggregate counts and acknowledge Stop before the window host exits.
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for _, id := range ids {
		a.mu.Lock()
		s := a.sessions[id]
		claimed := !s.lastSeen.IsZero()
		a.mu.Unlock()
		if claimed {
			select {
			case <-s.connectorDone:
			case <-deadline.C:
				return
			}
		}
	}
}

// Keep a small recent history. A just-stopped connector retains its control
// record long enough to read final counts and acknowledge shutdown.
func (a *App) pruneStoppedLocked() {
	var old []*activeSession
	for _, s := range a.sessions {
		if s.view.State == "stopped" && s.handler == nil {
			old = append(old, s)
		}
	}
	sort.Slice(old, func(i, j int) bool { return old[i].stoppedAt.Before(old[j].stoppedAt) })
	for _, s := range old {
		if len(a.sessions) < 20 {
			return
		}
		acknowledged := false
		select {
		case <-s.connectorDone:
			acknowledged = true
		default:
		}
		if acknowledged || s.lastSeen.IsZero() || time.Since(s.stoppedAt) > 30*time.Second {
			delete(a.sessions, s.view.ID)
		}
	}
}

func validAPIKey(key string) bool {
	for _, b := range []byte(key) {
		if b < 33 || b > 126 {
			return false
		}
	}
	return true
}

func (a *App) expireDisconnected(id string, now time.Time) {
	a.mu.Lock()
	s := a.sessions[id]
	lost := s != nil && s.view.State == "running" && now.Sub(s.lastSeen) > 15*time.Second
	a.mu.Unlock()
	if lost {
		a.stop(id, "connection_lost")
	}
}
func (a *App) Views() []SessionView {
	a.mu.Lock()
	defer a.mu.Unlock()
	views := make([]SessionView, 0, len(a.sessions))
	for _, s := range a.sessions {
		v := s.view
		if s.handler != nil {
			status := s.handler.Snapshot()
			v.Requests = status.Requests
			v.ChangedParts = status.ChangedParts
			v.Warnings = status.Warnings
		}
		views = append(views, v)
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Started.After(views[j].Started) })
	return views
}

func (a *App) sessionControl(w http.ResponseWriter, r *http.Request) bool {
	const prefix = "/api/session-control/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return false
	}
	if r.Header.Get("Origin") != "" {
		http.Error(w, "not_allowed", 403)
		return true
	}
	id := strings.TrimPrefix(r.URL.Path, prefix)
	a.mu.Lock()
	s := a.sessions[id]
	if s == nil {
		a.mu.Unlock()
		http.Error(w, "session_missing", 404)
		return true
	}
	token := s.controlToken
	a.mu.Unlock()
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Dmz-Connect")), []byte(token)) != 1 {
		http.Error(w, "not_authorized", 401)
		return true
	}
	if r.Method == "GET" {
		a.mu.Lock()
		s.lastSeen = time.Now()
		status := s.finalStatus
		if s.handler != nil {
			status = s.handler.Snapshot()
		}
		a.mu.Unlock()
		select {
		case <-s.done:
			writeJSON(w, map[string]any{"stop": true, "status": status})
		default:
			writeJSON(w, map[string]any{"stop": false, "status": status})
		}
		return true
	}
	if r.Method != "POST" {
		http.Error(w, "method_not_allowed", 405)
		return true
	}
	var event struct {
		State    string `json:"state"`
		ExitCode int    `json:"exit_code"`
	}
	if decode(r, &event) != nil {
		http.Error(w, "invalid_request", 400)
		return true
	}
	switch event.State {
	case "running":
		a.mu.Lock()
		if s.view.State == "stopped" {
			a.mu.Unlock()
			http.Error(w, "session_stopped", 410)
			return true
		}
		if s.view.State != "waiting" {
			a.mu.Unlock()
			http.Error(w, "session_already_claimed", 409)
			return true
		}
		s.view.State = "running"
		s.lastSeen = time.Now()
		a.mu.Unlock()
	case "finished":
		issue := ""
		if event.ExitCode != 0 {
			issue = "client_exited"
		}
		a.stop(id, issue)
		s.connectorOnce.Do(func() { close(s.connectorDone) })
	default:
		http.Error(w, "invalid_request", 400)
		return true
	}
	w.WriteHeader(204)
	return true
}
