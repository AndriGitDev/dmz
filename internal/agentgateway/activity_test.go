package agentgateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestActivityCountsNewMappingsNotReplayedHistory(t *testing.T) {
	for _, protocol := range []Protocol{Responses, Messages} {
		t.Run(string(protocol), func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{}`))
			}))
			defer provider.Close()
			s, err := NewServer(Config{Protocol: protocol, Upstream: provider.URL + "/v1", Token: syntheticToken, AllowLoopbackUpstream: true, RestoreReplies: true})
			if err != nil {
				t.Fatal(err)
			}
			send := func(input string, raw bool) {
				var b []byte
				path := "/v1/responses"
				if protocol == Messages {
					path = "/v1/messages"
				}
				if raw {
					b = []byte(input)
				} else if protocol == Responses {
					b, _ = json.Marshal(map[string]any{"input": input})
				} else {
					path = "/v1/messages"
					b, _ = json.Marshal(map[string]any{"messages": []any{map[string]any{"role": "user", "content": input}}})
				}
				r := httptest.NewRequest("POST", path, strings.NewReader(string(b)))
				r.Host = "127.0.0.1:1234"
				r.Header.Set(TokenHeader, syntheticToken)
				r.Header.Set("Content-Type", "application/json")
				s.ServeHTTP(httptest.NewRecorder(), r)
			}
			input := "Contact pilot@example.test and helper@example.test"
			send(input, false)
			first := s.Snapshot()
			if first.NewRedactions != 2 || len(first.Activity) != 1 || first.Activity[0].NewRedactions != 2 || first.Activity[0].Outcome != "masked" {
				t.Fatal("new values not counted")
			}
			send(input, false)
			status := s.Snapshot()
			if status.NewRedactions != 2 || status.Activity[1].NewRedactions != 0 || status.Activity[1].ChangedParts != 1 {
				t.Fatal("repeated history inflated new-value counter")
			}
			first.Activity[0].Outcome = "mutated"
			if s.Snapshot().Activity[0].Outcome == "mutated" {
				t.Fatal("snapshot aliases activity")
			}
			for range 25 {
				send("ordinary text", false)
			}
			status = s.Snapshot()
			if len(status.Activity) != 20 || status.Activity[19].Outcome != "no_changes" {
				t.Fatal("activity limit or no-change outcome incorrect")
			}
			send(`{"invalid":`, true)
			if s.Snapshot().Activity[19].Outcome != "processing_skipped" {
				t.Fatal("parse failure shown as ordinary processing")
			}
			s.ClearMappings()
			send(input, false)
			status = s.Snapshot()
			if status.NewRedactions != 4 {
				t.Fatal("new mapping generation not counted")
			}
			wire, _ := json.Marshal(status)
			for _, secret := range []string{"pilot@example.test", "helper@example.test", syntheticToken, "⟦", "forward", "reverse"} {
				if strings.Contains(string(wire), secret) {
					t.Fatal("activity/status exposed sensitive content")
				}
			}
		})
	}
}
