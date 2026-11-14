package agentgateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Exercise the production native-credential route, including its backend path,
// rather than replacing NativeOpenAIAuth with a loopback fixture configuration.
func TestRestorationOnNativeCredentialRoutes(t *testing.T) {
	for _, account := range []bool{false, true} {
		for _, stream := range []bool{false, true} {
			for _, label := range []string{"correct", "", "application/octet-stream", "text/plain", "text/event-stream, text/event-stream"} {
				t.Run(fmt.Sprintf("account=%v/stream=%v/label=%s", account, stream, label), func(t *testing.T) {
					const original = "pilot@example.test"
					s, err := NewServer(Config{Protocol: Responses, Upstream: "https://api.openai.com/v1", Token: syntheticToken, NativeOpenAIAuth: true, RestoreReplies: true})
					if err != nil {
						t.Fatal(err)
					}
					s.proxy.Transport = nativeAuthTransport(func(r *http.Request) (*http.Response, error) {
						wantHost, wantPath := "api.openai.com", "/v1/responses"
						if account {
							wantHost, wantPath = "chatgpt.com", "/backend-api/codex/responses"
						}
						if r.URL.Host != wantHost || r.URL.Path != wantPath {
							t.Error("unexpected native route")
						}
						raw, _ := io.ReadAll(r.Body)
						if bytes.Contains(raw, []byte(original)) {
							t.Error("unmasked request")
						}
						marker := issuedReplyMarker.FindString(string(raw))
						if marker == "" {
							t.Fatal("no issued marker")
						}
						var body strings.Builder
						media := "application/json"
						if stream {
							media = "text/event-stream; charset=utf-8"
							emit := func(value any) { b, _ := json.Marshal(value); fmt.Fprintf(&body, "data: %s\n\n", b) }
							emit(map[string]any{"type": "response.output_item.added", "output_index": 1, "item": map[string]any{"type": "message", "role": "assistant", "id": "msg_fixture", "content": []any{}}})
							for _, piece := range []string{marker[:len(marker)/2], marker[len(marker)/2:]} {
								emit(map[string]any{"type": "response.output_text.delta", "output_index": 1, "content_index": 0, "item_id": "msg_fixture", "delta": piece})
							}
							emit(map[string]any{"type": "response.completed", "response": restorationFixtureReply(Responses, marker)})
						} else {
							b, _ := json.Marshal(restorationFixtureReply(Responses, marker))
							body.Write(b)
						}
						if label != "correct" {
							media = label
						}
						return &http.Response{Request: r, StatusCode: 200, Header: http.Header{"Content-Type": []string{media}}, Body: io.NopCloser(strings.NewReader(body.String()))}, nil
					})
					r := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"input":"pilot@example.test"}`))
					r.Host = "127.0.0.1:1234"
					r.Header.Set(TokenHeader, syntheticToken)
					r.Header.Set("Content-Type", "application/json")
					r.Header.Set("Authorization", "Bearer synthetic-native-credential")
					if account {
						r.Header.Set("ChatGPT-Account-ID", "synthetic-account")
					}
					w := httptest.NewRecorder()
					s.ServeHTTP(w, r)
					if w.Code != 200 || !strings.Contains(w.Body.String(), original) || strings.Contains(w.Body.String(), "⟦") {
						t.Fatal("native-route answer not restored")
					}
					if len(s.replySlots) != 0 {
						t.Fatal("reply slot leaked")
					}
					if label != "correct" {
						code := "reply_content_type_inferred_json"
						if stream {
							code = "reply_content_type_inferred_sse"
						}
						status := s.Snapshot()
						if status.Warnings[code] != 1 || len(status.RecentWarnings) != 0 {
							t.Fatal("inference not recorded as informational")
						}
					}
				})
			}
		}
	}
}
