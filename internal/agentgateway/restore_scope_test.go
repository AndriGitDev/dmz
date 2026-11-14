package agentgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AndriGitDev/dmz/packages/surrogates"
)

func TestRestorationScopeResetAndUnknownMarkers(t *testing.T) {
	const original = "pilot@example.test"
	s, err := NewServer(Config{Protocol: Responses, Upstream: "https://api.openai.com/v1", Token: strings.Repeat("b", 64), RestoreReplies: true})
	if err != nil {
		t.Fatal(err)
	}
	first := &mapping{core: surrogates.NewSession()}
	second := &mapping{core: surrogates.NewSession()}
	marker := first.core.ProcessBestEffort(original, nil).Transformed
	if second.restoreAnswer(marker) != marker {
		t.Fatal("foreign scope marker restored")
	}
	if first.restoreAnswer(marker) != original {
		t.Fatal("issued marker not restored")
	}
	if got := first.remaskRestored("Contact " + original + " and " + marker); got != "Contact "+marker+" and "+marker {
		t.Fatal("exact replay masking corrupted issued markers")
	}
	if first.remaskRestored("unknown@example.test") != "unknown@example.test" {
		t.Fatal("unknown assistant content was changed")
	}
	s.maps["launch"] = first
	s.ClearMappings()
	if first.restoreAnswer(marker) != marker {
		t.Fatal("cleared mapping still restored replies")
	}
}

func TestRestorationConcurrencyBudgetAndEncoding(t *testing.T) {
	s, err := NewServer(Config{Protocol: Responses, Upstream: "https://api.openai.com/v1", Token: strings.Repeat("b", 64), RestoreReplies: true})
	if err != nil {
		t.Fatal(err)
	}
	m := &mapping{core: surrogates.NewSession()}
	response := func(encoding string) *http.Response {
		r, _ := http.NewRequest("POST", "https://api.openai.com/v1/responses", nil)
		r = r.WithContext(context.WithValue(r.Context(), replyScopeKey{}, m))
		h := http.Header{"Content-Type": []string{"application/json"}, "Digest": []string{"synthetic-digest"}, "Content-Length": []string{"2"}}
		if encoding != "" {
			h.Set("Content-Encoding", encoding)
		}
		return &http.Response{Request: r, StatusCode: 200, Header: h, ContentLength: 2, Body: io.NopCloser(strings.NewReader(`{}`))}
	}
	var bodies []io.ReadCloser
	for i := 0; i < 4; i++ {
		r := response("")
		s.restoreResponse(r)
		bodies = append(bodies, r.Body)
		if r.ContentLength != -1 || r.Header.Get("Digest") != "" {
			t.Fatal("restored body kept old framing/digest")
		}
	}
	blocked := response("")
	originalBody := blocked.Body
	s.restoreResponse(blocked)
	if blocked.Body != originalBody || blocked.Header.Get("Digest") == "" || s.Snapshot().Warnings["reply_busy_unrestored"] != 1 {
		t.Fatal("reply budget did not keep masked body unchanged")
	}
	for _, body := range bodies {
		body.Close()
		body.Close()
	}
	if len(s.replySlots) != 0 {
		t.Fatal("reply slot leaked or double-released")
	}
	encoded := response("gzip")
	originalBody = encoded.Body
	s.restoreResponse(encoded)
	if encoded.Body != originalBody || encoded.ContentLength != 2 || s.Snapshot().Warnings["reply_encoding_unrestored"] != 1 {
		t.Fatal("encoded reply was rewritten")
	}
}

func TestRestorationExpansionIsBoundedBeforeAllocating(t *testing.T) {
	m := &mapping{core: surrogates.NewSession()}
	marker := m.core.ProcessBestEffort("⟦synthetic-unknown⟧"+strings.Repeat("x", 128<<10), nil).Transformed
	text := strings.Repeat(marker, 100)
	if m.restoreAnswer(text) != text {
		t.Fatal("large repeated originals expanded without a limit")
	}
	input, _ := json.Marshal(restorationFixtureReply(Responses, "marker"))
	var warnings []string
	r := RestoreReply(io.NopCloser(bytes.NewReader(input)), "application/json", ReplyOptions{Protocol: Responses, MaxBytes: 4096, Restore: func(s string) string { return strings.ReplaceAll(s, "marker", strings.Repeat("\x00", 1000)) }, Warning: func(code string) { warnings = append(warnings, code) }})
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, input) || len(warnings) != 1 || warnings[0] != WarnReplyExpansionLimit {
		t.Fatal("JSON escaping exceeded the restoration budget")
	}
}

func TestRestorationCancellationReleasesScopeAndSlot(t *testing.T) {
	upstreamDone := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(upstreamDone)
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "event: response.output_item.added\ndata: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"type\":\"message\",\"role\":\"assistant\",\"id\":\"msg_synthetic\",\"content\":[]}}\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	}))
	defer provider.Close()
	s, err := NewServer(Config{Protocol: Responses, Upstream: provider.URL + "/v1", Token: strings.Repeat("b", 64), AllowLoopbackUpstream: true, RestoreReplies: true})
	if err != nil {
		t.Fatal(err)
	}
	local := httptest.NewServer(s)
	defer local.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	r, _ := http.NewRequestWithContext(ctx, "POST", local.URL+"/v1/responses", strings.NewReader(`{"input":"pilot@example.test","stream":true}`))
	r.Header.Set(TokenHeader, strings.Repeat("b", 64))
	r.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	if n, err := response.Body.Read(buf); n == 0 || err != nil {
		response.Body.Close()
		t.Fatal("stream did not begin before EOF")
	}
	cancel()
	response.Body.Close()
	select {
	case <-upstreamDone:
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled upstream remained open")
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		s.mu.Lock()
		inUse := s.maps["launch"].inUse
		s.mu.Unlock()
		if inUse == 0 && len(s.replySlots) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cancelled response retained mapping or restoration slot")
		}
		time.Sleep(time.Millisecond)
	}
}
