package agentgateway

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AndriGitDev/dmz/packages/surrogates"
)

func TestRestoredContextualValueInUserAndToolResults(t *testing.T) {
	const token = "dmzSyntheticProbe_QxabcXYZabcXYZabcXYZabcXYZabcXYZabcXYZabcXYZ"
	initial := "Authorization: Bearer " + token
	helper := "Helper result: " + token + " done."
	if surrogates.NewSession().ProcessBestEffort(helper, nil).Transformed != helper {
		t.Fatal("fixture no longer tests a context-dependent detection")
	}
	for _, protocol := range []Protocol{Responses, Messages} {
		t.Run(string(protocol), func(t *testing.T) {
			marker := ""
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if bytes.Contains(body, []byte(token)) {
					t.Error("restored contextual value reached provider")
				}
				if candidate := issuedReplyMarker.FindString(string(body)); candidate != "" {
					marker = candidate
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(restorationFixtureReply(protocol, "Helper result: "+marker+" done."))
			}))
			defer provider.Close()
			s, err := NewServer(Config{Protocol: protocol, Upstream: provider.URL + "/v1", Token: strings.Repeat("a", 64), AllowLoopbackUpstream: true, RestoreReplies: true, MaxTextBytes: 200, MaxScopeBytes: int64(len(initial) + 1)})
			if err != nil {
				t.Fatal(err)
			}
			key, path := "input", "/v1/responses"
			if protocol == Messages {
				key, path = "messages", "/v1/messages"
			}
			send := func(items []any) {
				data, _ := json.Marshal(map[string]any{key: items})
				r := httptest.NewRequest("POST", path, bytes.NewReader(data))
				r.Host = "127.0.0.1"
				r.Header.Set(TokenHeader, strings.Repeat("a", 64))
				r.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				s.ServeHTTP(w, r)
				if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(token)) {
					t.Fatal("fixture did not restore the contextual value")
				}
			}
			send([]any{map[string]any{"role": "user", "content": initial}})
			if marker == "" {
				t.Fatal("contextual credential was not masked initially")
			}
			// No original credential context remains. This request also exceeds
			// the scope budget, and the next one exceeds the per-text budget.
			send([]any{map[string]any{"role": "user", "content": helper}})
			send([]any{map[string]any{"role": "user", "content": strings.Repeat("safe ", 100) + helper}})
			if protocol == Responses {
				send([]any{map[string]any{"type": "function_call_output", "call_id": "synthetic", "output": helper}})
				send([]any{map[string]any{"type": "custom_tool_call_output", "call_id": "synthetic", "output": []any{map[string]any{"type": "input_text", "text": helper}}}})
				send([]any{map[string]any{"type": "shell_call_output", "call_id": "synthetic", "output": []any{map[string]any{"stdout": helper, "stderr": helper}}}})
			} else {
				for _, content := range []any{helper, []any{map[string]any{"type": "text", "text": helper}}} {
					send([]any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "synthetic", "content": content}}}})
				}
			}
			status := s.Snapshot()
			if status.Warnings["scope_capacity_detection_skipped"] == 0 || status.Warnings["oversized_text_detection_skipped"] == 0 {
				t.Fatal("resource fallback paths were not exercised")
			}
		})
	}
}

func TestWholeMessageFallbackIsNotRestoredIntoClientHistory(t *testing.T) {
	const original = "Synthetic text with ⟦an-unknown-marker⟧ inside."
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if bytes.Contains(body, []byte(original)) {
			t.Error("whole original reached provider")
		}
		marker := issuedReplyMarker.FindString(string(body))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(restorationFixtureReply(Responses, marker))
	}))
	defer provider.Close()
	s, err := NewServer(Config{Protocol: Responses, Upstream: provider.URL + "/v1", Token: strings.Repeat("a", 64), AllowLoopbackUpstream: true, RestoreReplies: true})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"input": original})
	r := httptest.NewRequest("POST", "/v1/responses", bytes.NewReader(body))
	r.Host = "127.0.0.1"
	r.Header.Set(TokenHeader, strings.Repeat("a", 64))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte(original)) || !bytes.Contains(w.Body.Bytes(), []byte("VALUE-")) {
		t.Fatal("arbitrary whole-message contents were restored")
	}
	if s.Snapshot().Warnings["reply_whole_text_unrestored"] == 0 {
		t.Fatal("whole-message restoration limit not disclosed")
	}
}
