package companion

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHeartbeatClosesDeadConnectorAndKeepsStopReason(t *testing.T) {
	var ticket Ticket
	a, local := testApp(t, func(_, name, _ string) error { var err error; ticket, err = ReadTicket(name); return err }, http.NotFoundHandler())
	view, err := a.Start(StartRequest{Client: "codex", Project: t.TempDir(), Auth: "account"})
	if err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{"X-Dmz-Connect": ticket.ControlToken}
	appRequest(t, a, local, "POST", "/api/session-control/"+view.ID, `{"state":"running"}`, headers)
	a.mu.Lock()
	a.sessions[view.ID].lastSeen = time.Now().Add(-time.Minute)
	a.mu.Unlock()
	heartbeat := appRequest(t, a, local, "GET", "/api/session-control/"+view.ID, "", headers)
	if heartbeat.StatusCode != 200 {
		t.Fatal("heartbeat rejected")
	}
	a.expireDisconnected(view.ID, time.Now())
	if a.Views()[0].State != "running" {
		t.Fatal("fresh connector lease expired")
	}
	a.mu.Lock()
	a.sessions[view.ID].lastSeen = time.Now().Add(-time.Minute)
	a.mu.Unlock()
	a.expireDisconnected(view.ID, time.Now())
	if v := a.Views()[0]; v.State != "stopped" || v.Issue != "connection_lost" {
		t.Fatal("dead connector retained its session")
	}
	appRequest(t, a, local, "POST", "/api/session-control/"+view.ID, `{"state":"finished","exit_code":1}`, headers)
	if a.Views()[0].Issue != "connection_lost" {
		t.Fatal("late finish overwrote stop reason")
	}
}

func TestUserStopDoesNotBecomeClientFailure(t *testing.T) {
	var ticket Ticket
	a, local := testApp(t, func(_, name, _ string) error { var err error; ticket, err = ReadTicket(name); return err }, http.NotFoundHandler())
	view, err := a.Start(StartRequest{Client: "claude", Project: t.TempDir(), Auth: "account"})
	if err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{"X-Dmz-Connect": ticket.ControlToken}
	appRequest(t, a, local, "POST", "/api/session-control/"+view.ID, `{"state":"running"}`, headers)
	a.stop(view.ID, "")
	appRequest(t, a, local, "POST", "/api/session-control/"+view.ID, `{"state":"finished","exit_code":143}`, headers)
	if a.Views()[0].Issue != "" {
		t.Fatal("normal stop shown as client failure")
	}
}

func TestStoppedHistoryIsBoundedAndKeysAreRejectedEarly(t *testing.T) {
	a, _ := testApp(t, func(string, string, string) error { return nil }, http.NotFoundHandler())
	project := t.TempDir()
	for _, key := range []string{"with space", "with\ttab", "with\x00null", "with\x7fdelete", "with-íslenska"} {
		_, err := a.Start(StartRequest{Client: "codex", Project: project, Auth: "api", APIKey: key})
		if err == nil || err.Error() != "api_key_invalid" {
			t.Fatal("invalid header credential reached launch")
		}
	}
	for i := 0; i < 60; i++ {
		view, err := a.Start(StartRequest{Client: "codex", Project: project, Auth: "account"})
		if err != nil {
			t.Fatal(err)
		}
		a.stop(view.ID, "")
	}
	if len(a.Views()) > 20 {
		t.Fatal("stopped session history unbounded")
	}
}

func TestBusyReopenDoesNotOrphanLiveInstance(t *testing.T) {
	a, local := testApp(t, func(string, string, string) error { return nil }, http.NotFoundHandler())
	for i := 0; i < 8; i++ {
		if _, err := a.windowAddress(); err != nil {
			t.Fatal(err)
		}
	}
	name := filepath.Join(t.TempDir(), "instance.json")
	body, _ := json.Marshal(instanceDescriptor{Origin: local.URL, Token: a.token})
	os.WriteFile(name, body, 0600)
	opened := false
	ok, err := TryReopen(name, func(string) error { opened = true; return nil })
	if ok || err == nil || err.Error() != "app_is_busy" || opened {
		t.Fatal("busy live app treated as stale")
	}
	if _, err := os.Stat(name); err != nil {
		t.Fatal("busy live app lost its descriptor")
	}
}

func TestConcurrentAppHostsReopenOneInstance(t *testing.T) {
	name := filepath.Join(t.TempDir(), "instance.json")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var windows []string
	cfg := Config{InstanceFile: name, Clients: []ClientInstallation{}, OpenWindow: func(address string) error { mu.Lock(); windows = append(windows, address); mu.Unlock(); return nil }}
	first, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	go func() { results <- first.Serve(ctx, true, "") }()
	go func() { results <- second.Serve(ctx, true, "") }()
	select {
	case err := <-results:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("second app host did not reopen first")
	}
	mu.Lock()
	copied := append([]string(nil), windows...)
	mu.Unlock()
	if len(copied) != 2 || strings.Split(copied[0], "/#")[0] != strings.Split(copied[1], "/#")[0] {
		t.Fatal("rapid double launch opened different app hosts")
	}
	cancel()
	select {
	case err := <-results:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("app host did not exit")
	}
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		t.Fatal("instance descriptor persisted after quit")
	}
}

func TestQuitCancelsAnOpenFolderChooser(t *testing.T) {
	a, local := testApp(t, func(string, string, string) error { return nil }, http.NotFoundHandler())
	started := make(chan struct{})
	a.cfg.ChooseDirectory = func(ctx context.Context) (string, error) { close(started); <-ctx.Done(); return "", ErrDialogCancelled }
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		resp := appRequest(t, a, local, "POST", "/api/folder", "{}", nil)
		resp.Body.Close()
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("folder helper did not start")
	}
	a.Close()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("quit orphaned folder helper")
	}
}
