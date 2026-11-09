package agentgateway

import (
	"testing"
	"time"
)

func TestAttentionExpiresWithoutLosingWarningHistory(t *testing.T) {
	s, err := NewServer(Config{Protocol: Responses, Upstream: "https://api.openai.com/v1", Token: "synthetic-local-token-with-at-least-32-characters"})
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"instructions_unchanged", "attachment_unchanged", "tool_result_unchanged", "mapping_evicted_history_may_degrade"} {
		s.warn(code)
	}
	if len(s.Snapshot().RecentWarnings) != 0 {
		t.Fatal("routine coverage notice requires attention")
	}
	s.warn("upstream_unavailable")
	status := s.Snapshot()
	if len(status.RecentWarnings) != 1 || status.RecentWarnings[0] != "upstream_unavailable" {
		t.Fatal("current failure not visible")
	}
	s.mu.Lock()
	s.warningTimes["upstream_unavailable"] = time.Now().Add(-3 * time.Minute)
	s.mu.Unlock()
	status = s.Snapshot()
	if len(status.RecentWarnings) != 0 || status.Warnings["upstream_unavailable"] != 1 {
		t.Fatal("old attention persisted or history lost")
	}
	s.warn("upstream_unavailable")
	if s.Snapshot().Warnings["upstream_unavailable"] != 2 || len(s.Snapshot().RecentWarnings) != 1 {
		t.Fatal("recurring failure not visible")
	}
}
