package companion

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testApp(t *testing.T, terminal func(string, string, string) error, upstream http.Handler) (*App, *httptest.Server) {
	t.Helper()
	provider := httptest.NewServer(upstream)
	t.Cleanup(provider.Close)
	a, err := New(Config{Executable: "/synthetic/DMZ", Clients: []ClientInstallation{{ID: "codex", Name: "Codex", Path: "/synthetic/codex", Available: true}, {ID: "claude", Name: "Claude Code", Path: "/synthetic/claude", Available: true}}, OpenTerminal: terminal, Environment: func(string) string { return "" }, FixtureUpstreams: map[string]string{"codex": provider.URL + "/v1", "claude": provider.URL + "/v1"}})
	if err != nil {
		t.Fatal(err)
	}
	local := httptest.NewServer(a)
	a.origin = local.URL
	t.Cleanup(func() { a.Close(); local.Close() })
	return a, local
}
func appRequest(t *testing.T, a *App, local *httptest.Server, method, path, body string, headers map[string]string) *http.Response {
	t.Helper()
	r, _ := http.NewRequest(method, local.URL+path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+a.token)
	r.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	resp, err := local.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestCompanionStartMaskAndStopReleasesSession(t *testing.T) {
	var ticket Ticket
	a, local := testApp(t, func(_, name, _ string) error { var err error; ticket, err = ReadTicket(name); return err }, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if bytes.Contains(body, []byte("pilot@example.test")) {
			t.Error("app gateway leaked eligible synthetic text")
		}
		if r.Header.Get("Authorization") != "Bearer synthetic-provider-key" || r.Header.Get("X-Dmz-Token") != "" {
			t.Error("app gateway credential separation failed")
		}
		io.WriteString(w, `{"synthetic":"ok"}`)
	}))
	project := t.TempDir()
	payload, _ := json.Marshal(StartRequest{Client: "codex", Project: project, Auth: "api", APIKey: "synthetic-provider-key"})
	resp := appRequest(t, a, local, "POST", "/api/start", string(payload), nil)
	var started SessionView
	if json.NewDecoder(resp.Body).Decode(&started) != nil || started.ID == "" {
		t.Fatal("session did not start")
	}
	serialized, _ := json.Marshal(ticket)
	if bytes.Contains(serialized, []byte("synthetic-provider-key")) {
		t.Fatal("provider key persisted in launch ticket")
	}
	r, _ := http.NewRequest("POST", ticket.GatewayURL+"/responses", strings.NewReader(`{"input":"pilot@example.test"}`))
	r.Header.Set("X-Dmz-Token", ticket.GatewayToken)
	r.Header.Set("Authorization", "Bearer "+ticket.GatewayToken)
	response, err := local.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("app gateway request failed")
	}
	stopBody, _ := json.Marshal(map[string]string{"id": started.ID})
	stop := appRequest(t, a, local, "POST", "/api/stop", string(stopBody), nil)
	if stop.StatusCode != 204 {
		t.Fatal("session not stopped")
	}
	views := a.Views()
	if len(views) != 1 || views[0].State != "stopped" || views[0].Requests != 1 || views[0].ChangedParts != 1 {
		t.Fatal("stopped session lost aggregate history")
	}
	a.mu.Lock()
	s := a.sessions[started.ID]
	released := s.handler == nil && s.server == nil
	a.mu.Unlock()
	if !released {
		t.Fatal("stopped app session retained provider key or mapping owner")
	}
	state := appRequest(t, a, local, "GET", "/api/state", "", nil)
	body, _ := io.ReadAll(state.Body)
	for _, secret := range []string{"pilot@example.test", "synthetic-provider-key", ticket.GatewayToken, ticket.ControlToken} {
		if bytes.Contains(body, []byte(secret)) {
			t.Fatal("app status exposed content or credentials")
		}
	}
}

func TestCompanionAppAuthenticationAndOrigin(t *testing.T) {
	a, local := testApp(t, func(string, string, string) error { return nil }, http.NotFoundHandler())
	for _, tc := range []struct {
		path, token, origin string
		want                int
	}{{"/api/state", "", "", 401}, {"/api/state", "wrong", "", 401}, {"/api/state", a.token, "https://outside.example.test", 403}, {"/api/state", a.token, "", 200}} {
		r, _ := http.NewRequest("GET", local.URL+tc.path, nil)
		if tc.token != "" {
			r.Header.Set("Authorization", "Bearer "+tc.token)
		}
		if tc.origin != "" {
			r.Header.Set("Origin", tc.origin)
		}
		resp, err := local.Client().Do(r)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Fatalf("auth probe status %d", resp.StatusCode)
		}
	}
	asset := appRequest(t, a, local, "GET", "/", "", nil)
	body, _ := io.ReadAll(asset.Body)
	if !strings.Contains(string(body), "start-form") || strings.Contains(string(body), a.token) || asset.Header.Get("Content-Security-Policy") == "" {
		t.Fatal("app assets unsafe or missing")
	}
}

func TestCompanionRejectsUnsupportedStartFields(t *testing.T) {
	a, local := testApp(t, func(string, string, string) error { t.Error("invalid start invoked terminal"); return nil }, http.NotFoundHandler())
	for _, body := range []string{`{"client":"codex","upstream":"https://outside.example.test"}`, `{"client":"codex"}` + strings.Repeat(" ", 17000) + `{}`, `{"client":"codex"} {}`} {
		resp := appRequest(t, a, local, "POST", "/api/start", body, nil)
		if resp.StatusCode != 400 {
			t.Fatal("unsupported app API payload accepted")
		}
	}
	for _, req := range []StartRequest{{Client: "unknown", Project: t.TempDir(), Auth: "api"}, {Client: "codex", Project: "relative", Auth: "api"}, {Client: "codex", Project: t.TempDir(), Auth: "api"}} {
		if _, err := a.Start(req); err == nil {
			t.Fatal("invalid app start accepted")
		}
	}
}

func TestLaunchTicketIsOneUseAndPrivate(t *testing.T) {
	var name string
	a, _ := testApp(t, func(_, path, _ string) error { name = path; return nil }, http.NotFoundHandler())
	view, err := a.Start(StartRequest{Client: "claude", Project: t.TempDir(), Auth: "account"})
	if err != nil {
		t.Fatal(err)
	}
	ticket, err := ReadTicket(name)
	if err != nil {
		t.Fatal(err)
	}
	if ticket.SessionID != view.ID || ticket.Auth != "account" || ticket.GatewayToken == ticket.ControlToken {
		t.Fatal("invalid scoped ticket")
	}
	if _, err := ReadTicket(name); err == nil {
		t.Fatal("launch ticket could be replayed")
	}
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		t.Fatal("consumed launch ticket persisted")
	}
	a.stop(view.ID, "")
}

func TestStoppedSessionCannotBecomeRunningAgain(t *testing.T) {
	var ticket Ticket
	a, local := testApp(t, func(_, name, _ string) error { var err error; ticket, err = ReadTicket(name); return err }, http.NotFoundHandler())
	view, err := a.Start(StartRequest{Client: "codex", Project: t.TempDir(), Auth: "account"})
	if err != nil {
		t.Fatal(err)
	}
	a.stop(view.ID, "")
	resp := appRequest(t, a, local, "POST", "/api/session-control/"+view.ID, `{"state":"running"}`, map[string]string{"X-Dmz-Connect": ticket.ControlToken})
	if resp.StatusCode != 410 {
		t.Fatal("stopped session accepted a late launch")
	}
}

func TestCloseRacesWithSessionStarts(t *testing.T) {
	a, _ := testApp(t, func(string, string, string) error { return nil }, http.NotFoundHandler())
	project := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = a.Start(StartRequest{Client: "codex", Project: project, Auth: "account"})
		}()
	}
	a.Close()
	wg.Wait()
	a.Close()
	for _, view := range a.Views() {
		if view.State != "stopped" {
			t.Fatal("close left an active session")
		}
	}
}

func TestInvalidAndExpiredTicketsAreNotConsumed(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "ticket.json")
	encoded, _ := json.Marshal(Ticket{Version: 1, Client: "codex", Auth: "account", SessionID: "synthetic", GatewayURL: "http://127.0.0.1:1", ControlURL: "http://127.0.0.1:2", GatewayToken: strings.Repeat("a", 64), ControlToken: strings.Repeat("b", 64), ExpiresAt: time.Now().Add(-time.Minute)})
	if os.WriteFile(name, encoded, 0600) != nil {
		t.Fatal("ticket fixture unavailable")
	}
	if _, err := ReadTicket(name); err == nil {
		t.Fatal("expired ticket accepted")
	}
	if _, err := os.Stat(name); err != nil {
		t.Fatal("invalid input file unexpectedly deleted")
	}
}

func TestCancelledFolderPickerIsNotAnError(t *testing.T) {
	a, local := testApp(t, func(string, string, string) error { return nil }, http.NotFoundHandler())
	a.cfg.ChooseDirectory = func(context.Context) (string, error) { return "", ErrDialogCancelled }
	response := appRequest(t, a, local, "POST", "/api/folder", `{}`, nil)
	var result map[string]any
	if response.StatusCode != 200 || json.NewDecoder(response.Body).Decode(&result) != nil || result["cancelled"] != true || result["error"] != nil {
		t.Fatal("picker cancellation reported failure")
	}
}
