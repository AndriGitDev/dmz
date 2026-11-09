package agentgateway

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestKnownValuesDoNotSplitLargerUserEntities(t *testing.T) {
	// Synthetic identifier: century digit 8. The larger IBAN contains it;
	// detection must retain the full IBAN boundary on a later turn.
	for _, pair := range [][2]string{
		{"kt. 1401828858", "IBAN IS360159260076541401828858"},
		{"pilot@example.test", "prefixpilot@example.test"},
	} {
		for _, protocol := range []Protocol{Responses, Messages} {
			t.Run(string(protocol)+"/"+strings.Fields(pair[0])[0], func(t *testing.T) {
				var bodies [][]byte
				var mu sync.Mutex
				provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, _ := io.ReadAll(r.Body)
					mu.Lock()
					bodies = append(bodies, body)
					mu.Unlock()
					w.Header().Set("Content-Type", "application/json")
					io.WriteString(w, `{}`)
				}))
				defer provider.Close()
				s, err := NewServer(Config{Protocol: protocol, Upstream: provider.URL + "/v1", Token: strings.Repeat("a", 64), AllowLoopbackUpstream: true, RestoreReplies: true})
				if err != nil {
					t.Fatal(err)
				}
				key, path := "input", "/v1/responses"
				if protocol == Messages {
					key, path = "messages", "/v1/messages"
				}
				for _, text := range pair {
					data, _ := json.Marshal(map[string]any{key: []any{map[string]any{"role": "user", "content": text}}})
					r := httptest.NewRequest("POST", path, bytes.NewReader(data))
					r.Host = "127.0.0.1"
					r.Header.Set(TokenHeader, strings.Repeat("a", 64))
					r.Header.Set("Content-Type", "application/json")
					w := httptest.NewRecorder()
					s.ServeHTTP(w, r)
					if w.Code != 200 {
						t.Fatal("fixture request failed")
					}
				}
				mu.Lock()
				defer mu.Unlock()
				if len(bodies) != 2 {
					t.Fatal("fixture did not exercise both turns")
				}
				if strings.HasPrefix(pair[1], "IBAN ") {
					if !bytes.Contains(bodies[1], []byte("IBAN-")) || bytes.Contains(bodies[1], []byte("IS36015926007654")) {
						t.Fatal("known kennitala split the larger IBAN and exposed account digits")
					}
				} else {
					if bytes.Contains(bodies[1], []byte("prefix")) {
						t.Fatal("known email split the larger address")
					}
				}
			})
		}
	}
}
