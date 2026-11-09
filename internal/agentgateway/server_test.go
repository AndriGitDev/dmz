package agentgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const syntheticToken = "synthetic-local-credential-for-test-only"

func fixture(t *testing.T, protocol Protocol, upstream http.Handler, adjust func(*Config)) (*Server, *httptest.Server) {
	t.Helper()
	provider := httptest.NewServer(upstream)
	t.Cleanup(provider.Close)
	cfg := Config{Protocol: protocol, Upstream: provider.URL + "/v1", Token: syntheticToken, AllowLoopbackUpstream: true}
	if adjust != nil {
		adjust(&cfg)
	}
	s, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	local := httptest.NewServer(s)
	t.Cleanup(local.Close)
	return s, local
}

func send(t *testing.T, client *http.Client, method, address, body string, headers map[string]string) *http.Response {
	t.Helper()
	r, err := http.NewRequest(method, address, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set(TokenHeader, syntheticToken)
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	response, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { response.Body.Close() })
	return response
}

func TestRequestMaskingAndCredentialBoundary(t *testing.T) {
	var observed []byte
	s, local := fixture(t, Responses, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" || r.URL.RawQuery != "synthetic=1" {
			t.Error("incorrect fixed upstream route")
		}
		if r.Header.Get(TokenHeader) != "" || r.Header.Get("X-Dmz-Extra") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Forwarded") != "" || r.Header.Get("X-Forwarded-For") != "" {
			t.Error("local or forwarding headers escaped")
		}
		if r.Header.Get("Authorization") != "Bearer synthetic-provider-key" {
			t.Error("provider authentication not preserved")
		}
		if r.Header.Get("Content-MD5") != "" {
			t.Error("stale digest forwarded")
		}
		observed, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "event: response.completed\ndata: {\"type\":\"response.completed\"}\n\n")
	}), nil)
	body := `{ "input": [{"role":"user","content":"Email pilot@example.test"}], "instructions":"keep@example.test", "tools":[{"name":"synthetic","description":"schema@example.test"}] }`
	response := send(t, local.Client(), "POST", local.URL+"/v1/responses?synthetic=1", body, map[string]string{"Authorization": "Bearer synthetic-provider-key", "X-Dmz-Extra": "local-only", "Cookie": "local-only=synthetic", "Content-MD5": "synthetic-old-digest", "Forwarded": "for=synthetic", "X-Forwarded-For": "synthetic"})
	got, _ := io.ReadAll(response.Body)
	if response.StatusCode != 200 || !strings.Contains(string(got), "response.completed") {
		t.Fatal("stream response not preserved")
	}
	if bytes.Contains(observed, []byte("pilot@example.test")) || !bytes.Contains(observed, []byte("EMAIL-")) {
		t.Fatal("supported user identifier reached upstream")
	}
	if !bytes.Contains(observed, []byte(`"instructions":"keep@example.test"`)) || !bytes.Contains(observed, []byte(`"description":"schema@example.test"`)) {
		t.Fatal("out-of-scope fields were altered")
	}
	status := s.Snapshot()
	if status.Requests != 1 || status.TextParts != 1 || status.ChangedParts != 1 {
		t.Fatalf("unexpected aggregate counters: %d/%d/%d", status.Requests, status.TextParts, status.ChangedParts)
	}
}

func TestLocalAccessChecks(t *testing.T) {
	_, local := fixture(t, Messages, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("access-rejected request reached provider") }), nil)
	for _, tc := range []struct {
		name, host, token, origin string
		want                      int
	}{
		{"missing credential", "127.0.0.1", "", "", 401},
		{"wrong credential", "127.0.0.1", "synthetic-wrong", "", 401},
		{"rebound host", "attacker.example.test", syntheticToken, "", 403},
		{"browser origin", "127.0.0.1", syntheticToken, "https://example.test", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := http.NewRequest("GET", local.URL+"/_dmz/status", nil)
			r.Host = tc.host
			if tc.token != "" {
				r.Header.Set(TokenHeader, tc.token)
			}
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			resp, err := local.Client().Do(r)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Fatalf("status %d", resp.StatusCode)
			}
		})
	}
}

func TestUpstreamAllowlist(t *testing.T) {
	for _, tc := range []struct {
		url      string
		protocol Protocol
		allow    bool
		valid    bool
	}{
		{"https://api.openai.com/v1", Responses, false, true},
		{"https://chatgpt.com/backend-api/codex", Responses, false, true},
		{"https://api.anthropic.com/v1", Messages, false, true},
		{"http://api.openai.com/v1", Responses, false, false},
		{"https://api.openai.com.attacker.example.test/v1", Responses, false, false},
		{"https://api.anthropic.com/v1", Responses, false, false},
		{"https://synthetic:synthetic@api.openai.com/v1", Responses, false, false},
		{"https://api.openai.com/v1?redirect=synthetic", Responses, false, false},
		{"http://127.0.0.1:1234/v1", Responses, false, false},
		{"http://127.0.0.1:1234/v1", Responses, true, true},
		{"http://localhost:1234/v1", Responses, true, false},
		{"http://10.0.0.1:1234/v1", Responses, true, false},
	} {
		_, err := NewServer(Config{Protocol: tc.protocol, Upstream: tc.url, Token: syntheticToken, AllowLoopbackUpstream: tc.allow})
		if (err == nil) != tc.valid {
			t.Errorf("upstream validation mismatch for %s", tc.url)
		}
	}
}

func TestSessionIsolationAndStableReplay(t *testing.T) {
	_, local := fixture(t, Messages, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.Copy(w, r.Body) }), func(c *Config) { c.SessionHeader = "X-Claude-Code-Session-Id" })
	mask := func(session string) string {
		resp := send(t, local.Client(), "POST", local.URL+"/v1/messages", `{"messages":[{"role":"user","content":"pilot@example.test"}]}`, map[string]string{"X-Claude-Code-Session-Id": session})
		body, _ := io.ReadAll(resp.Body)
		return string(body)
	}
	a := mask("synthetic-root-a")
	b := mask("synthetic-root-b")
	again := mask("synthetic-root-a")
	if a == b || a != again || strings.Contains(a, "pilot@example.test") {
		t.Fatal("mapping isolation or stability failed")
	}
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := mask("synthetic-root-a"); got != a {
				t.Error("concurrent mapping changed")
			}
		}()
	}
	wg.Wait()
}

func TestCapacityAndMissingIdentityWarnAndForward(t *testing.T) {
	for _, tc := range []struct {
		name    string
		adjust  func(*Config)
		headers map[string]string
		warning string
	}{
		{"body size", func(c *Config) { c.MaxBodyBytes = 8 }, nil, "oversized_body_forwarded_unchanged"},
		{"scope size", func(c *Config) { c.MaxScopeBytes = 8 }, nil, "scope_capacity_forwarded_unchanged"},
		{"identity", func(c *Config) { c.SessionHeader = "X-Claude-Code-Session-Id" }, nil, "missing_session_identity_forwarded_unchanged"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, local := fixture(t, Messages, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.Copy(w, r.Body) }), tc.adjust)
			body := `{"messages":[{"role":"user","content":"pilot@example.test"}]}`
			resp := send(t, local.Client(), "POST", local.URL+"/v1/messages", body, tc.headers)
			got, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != 200 || string(got) != body || s.Snapshot().Warnings[tc.warning] != 1 {
				t.Fatal("capacity/identity fallback did not preserve original request with warning")
			}
		})
	}
}

func TestOpaqueEncodedBodyPassesUnchanged(t *testing.T) {
	s, local := fixture(t, Responses, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.Copy(w, r.Body) }), nil)
	body := "synthetic encoded bytes"
	resp := send(t, local.Client(), "POST", local.URL+"/responses", body, map[string]string{"Content-Encoding": "synthetic-encoding"})
	got, _ := io.ReadAll(resp.Body)
	if string(got) != body || s.Snapshot().Warnings["encoded_body_forwarded_unchanged"] != 1 {
		t.Fatal("encoded body was changed")
	}
}

func TestErrorsAndRedirects(t *testing.T) {
	_, local := fixture(t, Responses, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery == "redirect=1" {
			w.Header().Set("Location", "https://elsewhere.example.test/")
			w.WriteHeader(307)
			return
		}
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(429)
		io.WriteString(w, `{"error":{"message":"synthetic rate limit"}}`)
	}), nil)
	resp := send(t, local.Client(), "POST", local.URL+"/responses", `{"input":"benign"}`, nil)
	if resp.StatusCode != 429 || resp.Header.Get("Retry-After") != "2" {
		t.Fatal("upstream error changed")
	}
	redirect := send(t, local.Client(), "POST", local.URL+"/responses?redirect=1", `{"input":"benign"}`, nil)
	if redirect.StatusCode != 502 || redirect.Header.Get("Location") != "" {
		t.Fatal("unsafe redirect propagated")
	}
}

func TestStreamingFlushAndCancellation(t *testing.T) {
	upstreamStopped := make(chan struct{})
	_, local := fixture(t, Responses, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "event: response.created\ndata: {}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(upstreamStopped)
	}), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	r, _ := http.NewRequestWithContext(ctx, "POST", local.URL+"/responses", strings.NewReader(`{"input":"benign"}`))
	r.Header.Set(TokenHeader, syntheticToken)
	resp, err := local.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	first := make([]byte, len("event: response.created\ndata: {}\n\n"))
	if _, err := io.ReadFull(resp.Body, first); err != nil {
		t.Fatal("stream was buffered", err)
	}
	cancel()
	resp.Body.Close()
	select {
	case <-upstreamStopped:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream did not observe cancellation")
	}
}

func TestStatusResetContainsNoOriginals(t *testing.T) {
	s, local := fixture(t, Responses, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.Copy(w, r.Body) }), nil)
	resp := send(t, local.Client(), "POST", local.URL+"/responses", `{"input":"pilot@example.test"}`, nil)
	io.Copy(io.Discard, resp.Body)
	status := send(t, local.Client(), "GET", local.URL+"/_dmz/status", "", nil)
	raw, _ := io.ReadAll(status.Body)
	var data Status
	if json.Unmarshal(raw, &data) != nil || data.Sessions != 1 {
		t.Fatal("status invalid")
	}
	if bytes.Contains(raw, []byte("pilot")) || bytes.Contains(raw, []byte("EMAIL-")) || bytes.Contains(raw, []byte(syntheticToken)) {
		t.Fatal("sensitive status data")
	}
	reset := send(t, local.Client(), "POST", local.URL+"/_dmz/reset", "", nil)
	if reset.StatusCode != 204 || s.Snapshot().Sessions != 0 || s.Snapshot().Warnings["mapping_reset_history_may_degrade"] != 1 {
		t.Fatal("reset not applied")
	}
}

func TestDesktopAuthenticationConsumesLocalKey(t *testing.T) {
	for _, tc := range []struct {
		protocol             Protocol
		auth, header, prefix string
	}{
		{Responses, AuthBearer, "Authorization", "Bearer "},
		{Responses, AuthAPIKey, "X-Api-Key", ""},
		{Messages, AuthBearer, "Authorization", "Bearer "},
		{Messages, AuthAPIKey, "X-Api-Key", ""},
	} {
		t.Run(string(tc.protocol)+"/"+tc.auth, func(t *testing.T) {
			_, local := fixture(t, tc.protocol, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for _, values := range r.Header {
					for _, value := range values {
						if strings.Contains(value, syntheticToken) {
							t.Error("local credential reached provider")
						}
					}
				}
				if tc.protocol == Messages {
					if r.Header.Get("X-Api-Key") != "synthetic-provider-key" || r.Header.Get("Authorization") != "" {
						t.Error("incorrect Anthropic provider authentication")
					}
				} else {
					if r.Header.Get("Authorization") != "Bearer synthetic-provider-key" || r.Header.Get("X-Api-Key") != "" {
						t.Error("incorrect Responses provider authentication")
					}
				}
				io.WriteString(w, `{"synthetic":"ok"}`)
			}), func(c *Config) { c.LocalAuth = tc.auth; c.ProviderKey = "synthetic-provider-key" })
			r, _ := http.NewRequest("POST", local.URL+"/v1/"+map[Protocol]string{Responses: "responses", Messages: "messages"}[tc.protocol], strings.NewReader(`{"input":"benign","messages":[]}`))
			r.Header.Set(tc.header, tc.prefix+syntheticToken)
			resp, err := local.Client().Do(r)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("desktop authentication status %d", resp.StatusCode)
			}
		})
	}
}

func TestLaunchTokenCannotBecomeProviderCredential(t *testing.T) {
	s, local := fixture(t, Responses, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-Api-Key") != "" {
			t.Error("local credential escaped through provider auth")
		}
	}), nil)
	resp := send(t, local.Client(), "POST", local.URL+"/responses", `{"input":"benign"}`, map[string]string{"Authorization": "Bearer " + syntheticToken, "X-Api-Key": syntheticToken})
	io.Copy(io.Discard, resp.Body)
	if s.Snapshot().Warnings["local_credential_in_provider_field_removed"] != 2 {
		t.Fatal("credential misconfiguration warning missing")
	}
}

func TestBusyForwardsOriginalWithWarning(t *testing.T) {
	s, local := fixture(t, Responses, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.Copy(w, r.Body) }), func(c *Config) { c.WorkerWait = 5 * time.Millisecond })
	for i := 0; i < cap(s.workers); i++ {
		s.workers <- struct{}{}
	}
	defer func() {
		for len(s.workers) > 0 {
			<-s.workers
		}
	}()
	body := `{"input":"pilot@example.test"}`
	resp := send(t, local.Client(), "POST", local.URL+"/responses", body, nil)
	got, _ := io.ReadAll(resp.Body)
	if string(got) != body || s.Snapshot().Warnings["masking_busy_forwarded_unchanged"] != 1 {
		t.Fatal("busy fallback did not warn and preserve original")
	}
}

func TestReplayDoesNotExhaustUniqueTextBudget(t *testing.T) {
	s, local := fixture(t, Responses, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.Copy(w, r.Body) }), func(c *Config) { c.MaxScopeBytes = int64(len("pilot@example.test")) })
	for i := 0; i < 8; i++ {
		resp := send(t, local.Client(), "POST", local.URL+"/responses", `{"input":"pilot@example.test"}`, nil)
		got, _ := io.ReadAll(resp.Body)
		if bytes.Contains(got, []byte("pilot@example.test")) {
			t.Fatal("replayed text lost masking")
		}
	}
	if s.Snapshot().Warnings["scope_capacity_forwarded_unchanged"] != 0 {
		t.Fatal("history replay consumed the scope budget")
	}
}

func TestUnknownMarkerDoesNotBypassMasking(t *testing.T) {
	s, local := fixture(t, Responses, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.Copy(w, r.Body) }), nil)
	body := `{"input":"⟦EMAIL-STALE-1: XXXXX@XXXXX.XXX⟧ plus pilot@example.test"}`
	resp := send(t, local.Client(), "POST", local.URL+"/responses", body, nil)
	got, _ := io.ReadAll(resp.Body)
	if bytes.Contains(got, []byte("pilot@example.test")) || bytes.Contains(got, []byte("STALE")) || !bytes.Contains(got, []byte("⟦VALUE-")) {
		t.Fatal("unknown-marker guard weakened")
	}
	if s.Snapshot().Warnings["whole_text_masked_context_may_degrade"] != 1 {
		t.Fatal("whole-field context degradation warning missing")
	}
}

func TestBurstWaitsForMaskingSlot(t *testing.T) {
	s, local := fixture(t, Responses, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.Copy(w, r.Body) }), func(c *Config) { c.WorkerWait = time.Second })
	for i := 0; i < cap(s.workers); i++ {
		s.workers <- struct{}{}
	}
	// Release a slot after a short burst, well before the bounded timeout.
	timer := time.AfterFunc(20*time.Millisecond, func() { <-s.workers })
	defer timer.Stop()
	resp := send(t, local.Client(), "POST", local.URL+"/responses", `{"input":"pilot@example.test"}`, nil)
	got, _ := io.ReadAll(resp.Body)
	if bytes.Contains(got, []byte("pilot@example.test")) || s.Snapshot().Warnings["masking_busy_forwarded_unchanged"] != 0 {
		t.Fatal("short contention bypassed masking")
	}
	for len(s.workers) > 0 {
		<-s.workers
	}
}

func TestIdleScopeEvictionKeepsNewSessionsMasked(t *testing.T) {
	s, local := fixture(t, Messages, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.Copy(w, r.Body) }), func(c *Config) { c.SessionHeader = "X-Claude-Code-Session-Id"; c.MaxSessions = 1 })
	for _, scope := range []string{"synthetic-root-a", "synthetic-root-b", "synthetic-root-c"} {
		resp := send(t, local.Client(), "POST", local.URL+"/messages", `{"messages":[{"role":"user","content":"pilot@example.test"}]}`, map[string]string{"X-Claude-Code-Session-Id": scope})
		got, _ := io.ReadAll(resp.Body)
		if bytes.Contains(got, []byte("pilot@example.test")) {
			t.Fatal("new session forwarded unmasked at capacity")
		}
	}
	status := s.Snapshot()
	if status.Sessions != 1 || status.Warnings["mapping_evicted_history_may_degrade"] != 2 {
		t.Fatal("scope eviction warning or bound incorrect")
	}
}

func TestLocalCredentialCaseAndDuplicateScrub(t *testing.T) {
	_, local := fixture(t, Responses, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.Header.Values("Authorization")) > 0 || len(r.Header.Values("X-Api-Key")) > 0 {
			t.Error("variant local credential escaped")
		}
	}), nil)
	r, _ := http.NewRequest("POST", local.URL+"/responses", strings.NewReader(`{"input":"benign"}`))
	r.Header.Set(TokenHeader, syntheticToken)
	r.Header.Add("Authorization", "Bearer synthetic-provider-key")
	r.Header.Add("Authorization", "bearer   "+syntheticToken)
	r.Header.Add("Authorization", syntheticToken)
	r.Header.Add("X-Api-Key", "synthetic-provider-key")
	r.Header.Add("X-Api-Key", " "+syntheticToken+" ")
	r.Header.Add("X-Api-Key", "Bearer "+syntheticToken)
	resp, err := local.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestProviderKeyModeRejectsChatGPTBackend(t *testing.T) {
	_, err := NewServer(Config{Protocol: Responses, Upstream: "https://chatgpt.com/backend-api/codex", Token: syntheticToken, LocalAuth: AuthBearer, ProviderKey: "synthetic-provider-key"})
	if err == nil {
		t.Fatal("API key replacement accepted the ChatGPT backend")
	}
}

func TestCompactionMasksUserTextAndPreservesOpaqueState(t *testing.T) {
	responseBody := `{"object":"response.compaction","output":[{"type":"compaction","encrypted_content":"synthetic-opaque-state"}]}`
	_, local := fixture(t, Responses, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses/compact" {
			t.Error("incorrect compaction endpoint")
		}
		body, _ := io.ReadAll(r.Body)
		if bytes.Contains(body, []byte("pilot@example.test")) {
			t.Error("compaction leaked eligible text")
		}
		if !bytes.Contains(body, []byte(`"encrypted_content" : "synthetic-opaque-state"`)) || !bytes.Contains(body, []byte(`"previous_response_id":"synthetic-response"`)) {
			t.Error("opaque compaction state changed")
		}
		io.WriteString(w, responseBody)
	}), nil)
	resp := send(t, local.Client(), "POST", local.URL+"/v1/responses/compact", `{"input":[{"role":"user","content":"pilot@example.test"},{"type":"compaction","encrypted_content" : "synthetic-opaque-state"}],"previous_response_id":"synthetic-response"}`, nil)
	got, _ := io.ReadAll(resp.Body)
	if string(got) != responseBody {
		t.Fatal("compaction output changed")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCancelledQueueDoesNotInvokeUpstreamTransport(t *testing.T) {
	s, err := NewServer(Config{Protocol: Responses, Upstream: "https://api.openai.com/v1", Token: syntheticToken})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < cap(s.workers); i++ {
		s.workers <- struct{}{}
	}
	s.proxy.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("cancelled draft invoked upstream transport")
		return nil, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest("POST", "/responses", strings.NewReader(`{"input":"pilot@example.test"}`)).WithContext(ctx)
	r.Host = "127.0.0.1"
	r.Header.Set(TokenHeader, syntheticToken)
	s.ServeHTTP(httptest.NewRecorder(), r)
	if s.Snapshot().Requests != 1 || s.Snapshot().ChangedParts != 0 {
		t.Fatal("cancelled queued draft unexpectedly transformed")
	}
}
