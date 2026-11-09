package agentgateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/AndriGitDev/dmz/packages/surrogates"
)

func TestHighlightOnlyExactInlineRestorations(t *testing.T) {
	m := &mapping{core: surrogates.NewSession()}
	original := "pilot@example.test"
	marker := m.core.ProcessBestEffort(original, nil).Transformed
	for _, tc := range []struct{ input, want string }{
		{marker, "**" + original + "**"},
		{"Hello " + marker + ".", "Hello **" + original + "**."},
		{"**" + marker + "**", "****" + original + "****"},
		{marker + " " + marker, "**" + original + "** **" + original + "**"},
		{"ordinary text", "ordinary text"},
		{"⟦EMAIL-AAAAAAAAAAAAAAAA-99: XXXXX@XXXXX.XXX⟧", "⟦EMAIL-AAAAAAAAAAAAAAAA-99: XXXXX@XXXXX.XXX⟧"},
	} {
		if got := m.restoreAnswerStyled(tc.input, true); got != tc.want {
			t.Fatal("highlight or unchanged-text boundary incorrect")
		}
	}
	m.mu.Lock()
	masked := m.remaskRestored("**" + original + "**")
	m.mu.Unlock()
	if masked != "**"+marker+"**" {
		t.Fatal("bolded value not exactly remasked")
	}
	for _, value := range []string{"multi\nline", "star*value", "code`value", " padded "} {
		if boldRestoredValue(value, true) {
			t.Fatal("unsafe Markdown value highlighted")
		}
	}
}

func TestHighlightSettingIsStableForOneReply(t *testing.T) {
	s, err := NewServer(Config{Protocol: Responses, Upstream: "https://api.openai.com/v1", Token: syntheticToken, RestoreReplies: true})
	if err != nil {
		t.Fatal(err)
	}
	m := &mapping{core: surrogates.NewSession()}
	marker := m.core.ProcessBestEffort("pilot@example.test", nil).Transformed
	reply := func() *http.Response {
		r, _ := http.NewRequest("POST", "https://api.openai.com/v1/responses", nil)
		r = r.WithContext(context.WithValue(r.Context(), replyScopeKey{}, m))
		body, _ := json.Marshal(restorationFixtureReply(Responses, marker))
		return &http.Response{Request: r, StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(body)))}
	}
	s.SetHighlightRestored(true)
	first := reply()
	s.restoreResponse(first)
	s.SetHighlightRestored(false)
	b, _ := io.ReadAll(first.Body)
	first.Body.Close()
	if !strings.Contains(string(b), "**pilot@example.test**") {
		t.Fatal("in-flight reply changed style")
	}
	next := reply()
	s.restoreResponse(next)
	b, _ = io.ReadAll(next.Body)
	next.Body.Close()
	if strings.Contains(string(b), "**") || !strings.Contains(string(b), "pilot@example.test") {
		t.Fatal("setting did not affect next reply")
	}
}

func TestHighlightStreamingWithExistingMarkdown(t *testing.T) {
	m := &mapping{core: surrogates.NewSession()}
	marker := m.core.ProcessBestEffort("pilot@example.test", nil).Transformed
	for cut := 1; cut < len(marker); cut++ {
		if !utf8.RuneStart(marker[cut]) {
			continue
		}
		stream := responsesStream([]string{"**", marker[:cut], marker[cut:], "**"}, "**"+marker+"**")
		result := runReply(t, Responses, sseType, stream, []int{1, 2, 1}, func(s string) string { return m.restoreAnswerStyled(s, true) })
		var got strings.Builder
		for _, line := range strings.Split(string(result.out), "\n") {
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var event struct {
				Type  string `json:"type"`
				Delta string `json:"delta"`
			}
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event) == nil && event.Type == "response.output_text.delta" {
				got.WriteString(event.Delta)
			}
		}
		if got.String() != "****pilot@example.test****" {
			t.Fatal("Markdown highlighting depends on delta boundaries")
		}
	}
}
