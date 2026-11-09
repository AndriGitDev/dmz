package agentgateway

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestLocalAnswerRestorationAndAssistantReplay(t *testing.T) {
	for _, protocol := range []Protocol{Responses, Messages} {
		for _, stream := range []bool{false, true} {
			for _, reset := range []bool{false, true} {
				for _, compressed := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/stream=%v/reset=%v/gzip=%v", protocol, stream, reset, compressed), func(t *testing.T) {
						const original = "pilot@example.test"
						var mu sync.Mutex
						var firstMarker string
						var observed int
						provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							body, _ := io.ReadAll(r.Body)
							if bytes.Contains(body, []byte(original)) {
								t.Error("original reached provider in replayed history")
							}
							if r.Header.Get("Content-Encoding") != "" || r.Header.Get("Digest") != "" {
								t.Error("changed request retained encoding or digest")
							}
							if r.Header.Get("Accept-Encoding") != "identity" {
								t.Error("restorable reply did not request identity encoding")
							}
							marker := issuedReplyMarker.FindString(string(body))
							if marker == "" {
								t.Error("user text was not masked")
							}
							mu.Lock()
							observed++
							if firstMarker == "" {
								firstMarker = marker
							}
							mu.Unlock()
							text := "Contact " + marker
							w.Header().Set("Digest", "synthetic-old-digest")
							if !stream {
								w.Header().Set("Content-Type", "application/json")
								json.NewEncoder(w).Encode(restorationFixtureReply(protocol, text))
								return
							}
							w.Header().Set("Content-Type", "text/event-stream")
							write := func(typ string, value map[string]any) {
								data, _ := json.Marshal(value)
								fmt.Fprintf(w, "event: %s\ndata: %s\n\n", typ, data)
							}
							cut := len(marker) / 2
							if protocol == Responses {
								item := map[string]any{"type": "message", "role": "assistant", "id": "msg_synthetic", "content": []any{}}
								write("response.output_item.added", map[string]any{"type": "response.output_item.added", "output_index": 0, "item": item})
								for _, delta := range []string{"Contact " + marker[:cut], marker[cut:]} {
									write("response.output_text.delta", map[string]any{"type": "response.output_text.delta", "item_id": "msg_synthetic", "output_index": 0, "content_index": 0, "delta": delta})
								}
								write("response.output_text.done", map[string]any{"type": "response.output_text.done", "item_id": "msg_synthetic", "output_index": 0, "content_index": 0, "text": text})
								write("response.completed", map[string]any{"type": "response.completed", "response": restorationFixtureReply(protocol, text)})
							} else {
								write("message_start", map[string]any{"type": "message_start", "message": map[string]any{"type": "message", "role": "assistant", "content": []any{}}})
								write("content_block_start", map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "text", "text": ""}})
								for _, delta := range []string{"Contact " + marker[:cut], marker[cut:]} {
									write("content_block_delta", map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": delta}})
								}
								write("content_block_stop", map[string]any{"type": "content_block_stop", "index": 0})
								write("message_stop", map[string]any{"type": "message_stop"})
							}
						}))
						defer provider.Close()
						s, err := NewServer(Config{Protocol: protocol, Upstream: provider.URL + "/v1", Token: strings.Repeat("a", 64), AllowLoopbackUpstream: true, RestoreReplies: true, SessionHeader: map[Protocol]string{Messages: "X-Claude-Code-Session-Id"}[protocol]})
						if err != nil {
							t.Fatal(err)
						}
						local := httptest.NewServer(s)
						defer local.Close()
						send := func(history []any) string {
							key, path := "input", "/v1/responses"
							if protocol == Messages {
								key, path = "messages", "/v1/messages"
							}
							body, _ := json.Marshal(map[string]any{key: history, "stream": stream})
							if compressed {
								var buf bytes.Buffer
								z := gzip.NewWriter(&buf)
								z.Write(body)
								z.Close()
								body = buf.Bytes()
							}
							r, _ := http.NewRequest("POST", local.URL+path, bytes.NewReader(body))
							r.Header.Set(TokenHeader, strings.Repeat("a", 64))
							r.Header.Set("Content-Type", "application/json")
							r.Header.Set("Digest", "synthetic-old-body")
							r.Header.Set("X-Claude-Code-Session-Id", "synthetic-session")
							if compressed {
								r.Header.Set("Content-Encoding", "gzip")
							}
							response, err := http.DefaultClient.Do(r)
							if err != nil {
								t.Fatal(err)
							}
							defer response.Body.Close()
							if response.StatusCode != 200 || response.Header.Get("Digest") != "" {
								t.Fatal("invalid restored response status or digest")
							}
							out, err := io.ReadAll(response.Body)
							if err != nil {
								t.Fatal(err)
							}
							if !stream {
								var value map[string]any
								if json.Unmarshal(out, &value) != nil {
									t.Fatal("invalid restored JSON")
								}
								if protocol == Responses {
									value = value["output"].([]any)[0].(map[string]any)
								}
								return value["content"].([]any)[0].(map[string]any)["text"].(string)
							}
							var text strings.Builder
							for _, line := range strings.Split(string(out), "\n") {
								if !strings.HasPrefix(line, "data: ") {
									continue
								}
								var value map[string]any
								if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &value) != nil {
									t.Fatal("invalid restored SSE JSON")
								}
								if value["type"] == "response.output_text.delta" {
									text.WriteString(value["delta"].(string))
								}
								if value["type"] == "content_block_delta" {
									text.WriteString(value["delta"].(map[string]any)["text"].(string))
								}
								if value["type"] == "response.output_text.done" && value["text"] != "Contact "+original {
									t.Error("done text differs from restored deltas")
								}
							}
							return text.String()
						}
						user := map[string]any{"role": "user", "content": original}
						answer := send([]any{user})
						if answer != "Contact "+original {
							t.Fatal("local answer still masked or corrupted")
						}
						if reset {
							s.ClearMappings()
						}
						partType := "output_text"
						if protocol == Messages {
							partType = "text"
						}
						assistant := map[string]any{"role": "assistant", "content": []any{map[string]any{"type": partType, "text": answer}}}
						if protocol == Responses {
							assistant["type"] = "message"
						}
						if send([]any{user, assistant, map[string]any{"role": "user", "content": "Continue the synthetic conversation."}}) != "Contact "+original {
							t.Fatal("second answer did not restore")
						}
						mu.Lock()
						count := observed
						mu.Unlock()
						if count != 2 {
							t.Fatal("fixture did not exercise both turns")
						}
						status, _ := json.Marshal(s.Snapshot())
						if bytes.Contains(status, []byte(original)) {
							t.Fatal("status exposed original")
						}
					})
				}
			}
		}
	}
}

func restorationFixtureReply(protocol Protocol, text string) map[string]any {
	partType := "output_text"
	if protocol == Messages {
		partType = "text"
	}
	item := map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": partType, "text": text}}}
	if protocol == Responses {
		return map[string]any{"object": "response", "output": []any{item}}
	}
	return item
}

func TestReplyHistoryLeavesOpaqueDataAndUnknownValues(t *testing.T) {
	for _, protocol := range []Protocol{Responses, Messages} {
		body := []byte(`{"input":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"known@example.test"}]},{"type":"function_call","arguments":"known@example.test"}],"messages":[{"role":"assistant","content":[{"type":"text","text":"known@example.test"},{"type":"thinking","thinking":"known@example.test","signature":"opaque"},{"type":"tool_use","input":{"value":"known@example.test"}}]}],"system":"known@example.test"}`)
		out, report := TransformReplyHistory(protocol, body, func(s string) (string, error) {
			return strings.ReplaceAll(s, "known@example.test", "synthetic-marker"), nil
		})
		if report.ChangedParts != 1 || bytes.Count(out, []byte("synthetic-marker")) != 1 {
			t.Fatal("wrong assistant-text scope")
		}
		if !bytes.Contains(out, []byte(`"thinking":"known@example.test","signature":"opaque"`)) || !bytes.Contains(out, []byte(`"arguments":"known@example.test"`)) {
			t.Fatal("opaque fields changed")
		}
	}
}
