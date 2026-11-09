package companion

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWindowBootstrapIsShortLivedSingleUse(t *testing.T) {
	a, local := testApp(t, func(string, string, string) error { return nil }, http.NotFoundHandler())
	address, err := a.windowAddress()
	if err != nil || strings.Contains(address, a.token) {
		t.Fatal("window address exposed control credential")
	}
	u, _ := url.Parse(address)
	code := strings.TrimPrefix(u.Fragment, "open=")
	if len(code) != 64 {
		t.Fatal("missing window handoff")
	}
	body, _ := json.Marshal(map[string]string{"code": code})
	for i, want := range []int{200, 401} {
		resp := appRequest(t, a, local, "POST", "/api/bootstrap", string(body), nil)
		if resp.StatusCode != want {
			t.Fatalf("bootstrap attempt %d: %d", i, resp.StatusCode)
		}
		if want == 200 {
			var result map[string]string
			json.NewDecoder(resp.Body).Decode(&result)
			if result["token"] != a.token {
				t.Fatal("invalid bootstrap result")
			}
		}
	}
	a.mu.Lock()
	a.windowCodes[code] = time.Now().Add(-time.Second)
	a.mu.Unlock()
	expired := appRequest(t, a, local, "POST", "/api/bootstrap", string(body), nil)
	if expired.StatusCode != 401 {
		t.Fatal("expired handoff accepted")
	}
	address, _ = a.windowAddress()
	u, _ = url.Parse(address)
	body, _ = json.Marshal(map[string]string{"code": strings.TrimPrefix(u.Fragment, "open=")})
	rejected := appRequest(t, a, local, "POST", "/api/bootstrap", string(body), map[string]string{"Origin": "https://outside.example.test"})
	if rejected.StatusCode != 403 {
		t.Fatal("cross-origin bootstrap accepted")
	}
	accepted := appRequest(t, a, local, "POST", "/api/bootstrap", string(body), nil)
	if accepted.StatusCode != 200 {
		t.Fatal("cross-origin probe consumed valid handoff")
	}
}

func TestReopenUsesFreshHandoffAndPreservesOtherInstance(t *testing.T) {
	a, local := testApp(t, func(string, string, string) error { return nil }, http.NotFoundHandler())
	name := filepath.Join(t.TempDir(), "instance.json")
	body, _ := json.Marshal(instanceDescriptor{Origin: local.URL, Token: a.token})
	if os.WriteFile(name, body, 0600) != nil {
		t.Fatal("descriptor unavailable")
	}
	var addresses []string
	for i := 0; i < 2; i++ {
		ok, err := TryReopen(name, func(address string) error { addresses = append(addresses, address); return nil })
		if err != nil || !ok {
			t.Fatal("existing app failed to reopen")
		}
	}
	if addresses[0] == addresses[1] || strings.Contains(addresses[0], a.token) {
		t.Fatal("reopening reused handoff or exposed credential")
	}
	removeOwnInstance(name, strings.Repeat("f", 64))
	if _, err := os.Stat(name); err != nil {
		t.Fatal("old instance deleted replacement descriptor")
	}
	removeOwnInstance(name, a.token)
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		t.Fatal("owner descriptor not removed")
	}
}

func TestReopenRejectsUnsafeDescriptorAndStaleApp(t *testing.T) {
	a, local := testApp(t, func(string, string, string) error { return nil }, http.NotFoundHandler())
	name := filepath.Join(t.TempDir(), "instance.json")
	opened := false
	open := func(string) error { opened = true; return nil }
	for _, origin := range []string{"https://outside.example.test", local.URL + "/outside"} {
		body, _ := json.Marshal(instanceDescriptor{Origin: origin, Token: a.token})
		os.WriteFile(name, body, 0600)
		ok, err := TryReopen(name, open)
		if ok || err != nil || opened {
			t.Fatal("unsafe descriptor opened")
		}
	}
	body, _ := json.Marshal(instanceDescriptor{Origin: local.URL, Token: strings.Repeat("0", 64)})
	os.WriteFile(name, body, 0600)
	if ok, err := TryReopen(name, open); ok || err != nil || opened {
		t.Fatal("stale authorization opened app")
	}
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		t.Fatal("stale descriptor retained")
	}
	os.WriteFile(name, []byte(strings.Repeat(" ", 5000)), 0600)
	if ok, err := TryReopen(name, open); ok || err == nil {
		t.Fatal("oversized descriptor accepted")
	}
}

func TestWindowHandoffsBoundedWithoutLeakingCredential(t *testing.T) {
	a, local := testApp(t, func(string, string, string) error { return nil }, http.NotFoundHandler())
	for i := 0; i < 8; i++ {
		if _, err := a.windowAddress(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.windowAddress(); err == nil {
		t.Fatal("unbounded handoffs")
	}
	resp := appRequest(t, a, local, "GET", "/", "", nil)
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), a.token) {
		t.Fatal("credential in public document")
	}
	a.mu.Lock()
	for code := range a.windowCodes {
		a.windowCodes[code] = time.Now().Add(-time.Second)
	}
	a.mu.Unlock()
	if _, err := a.windowAddress(); err != nil {
		t.Fatal("expired handoffs blocked new window")
	}
}

func TestSessionClaimCannotBeReplayedFromCopiedTicket(t *testing.T) {
	var ticket Ticket
	a, local := testApp(t, func(_, name, _ string) error { var err error; ticket, err = ReadTicket(name); return err }, http.NotFoundHandler())
	view, err := a.Start(StartRequest{Client: "codex", Project: t.TempDir(), Auth: "account"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []int{204, 409} {
		resp := appRequest(t, a, local, "POST", "/api/session-control/"+view.ID, `{"state":"running"}`, map[string]string{"X-Dmz-Connect": ticket.ControlToken})
		if resp.StatusCode != want {
			t.Fatalf("claim status %d, want %d", resp.StatusCode, want)
		}
	}
}
