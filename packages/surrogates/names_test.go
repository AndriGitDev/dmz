package surrogates

import (
	"strings"
	"testing"
)

func TestTrainedNamesMaskAndRestore(t *testing.T) {
	s := NewSession()
	input := "Sigríður mætti með Anna."
	result := s.ProcessWithNames(input, []NameSpan{{0, len("Sigríður")}, {len("Sigríður mætti með "), len(input) - 1}})
	if !result.Allowed || strings.Contains(result.Transformed, "Sigríður") || strings.Contains(result.Transformed, "Anna") {
		t.Fatalf("names not masked: %+v", result)
	}
	if s.Restore(result.Transformed) != input {
		t.Fatal("restoration changed text")
	}
	again := s.ProcessWithNames(result.Transformed, nil)
	if !again.Allowed || again.Transformed != result.Transformed {
		t.Fatal("live markers not preserved")
	}
}
func TestTrainedNamesCannotBypassProtection(t *testing.T) {
	for _, input := range []string{"sk-abcdefghijklmnop", "Ólafur Sigurðsson", "a\u200bb", "⟦NAME-fake⟧"} {
		s := NewSession()
		before := s.Process(input)
		after := s.ProcessWithNames(input, []NameSpan{{0, len(input)}})
		if before.Allowed || after.Allowed || after.Transformed != "" {
			t.Fatal("existing blocker bypassed")
		}
	}
}
func TestTrainedNamesRejectInvalidSpans(t *testing.T) {
	for _, names := range [][]NameSpan{{{-1, 3}}, {{0, 1}}, {{0, 200}}, {{4, 3}}, {{0, 3}, {2, 4}}, {{0, 0}}} {
		r := NewSession().ProcessWithNames("Örn Anna", names)
		if r.Allowed || r.Transformed != "" {
			t.Fatalf("accepted invalid span %+v", names)
		}
	}
	input := "anna@example.com"
	r := NewSession().ProcessWithNames(input, []NameSpan{{0, 4}})
	if r.Allowed {
		t.Fatal("overlap with other identifier must block")
	}
}

func TestTrainedNamesCannotRewriteIssuedMarker(t *testing.T) {
	s := NewSession()
	original := s.Process("Arna Jónsdóttir")
	start := strings.Index(original.Transformed, "XXXXX")
	result := s.ProcessWithNames(original.Transformed, []NameSpan{{start, start + 5}})
	if result.Allowed || result.Transformed != "" {
		t.Fatal("model changed an issued marker")
	}
	if s.Restore(original.Transformed) != "Arna Jónsdóttir" {
		t.Fatal("blocked scan changed mapping")
	}
}
