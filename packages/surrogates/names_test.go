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

func TestTrainedNameInsideEmailUsesCompleteEmailMask(t *testing.T) {
	input := "anna@example.com"
	for _, spans := range [][]NameSpan{{{0, 4}}, {{5, 12}}, {{0, len(input)}}, {{0, 4}, {5, 12}}} {
		s := NewSession()
		result := s.ProcessBestEffort(input, spans)
		if !result.Allowed || strings.Contains(result.Transformed, "anna") || strings.Contains(result.Transformed, "example.com") || !strings.Contains(result.Transformed, "⟦EMAIL-") || strings.Contains(result.Transformed, "⟦NAME-") {
			t.Fatalf("email not fully protected: %+v", result)
		}
		if s.Restore(result.Transformed) != input {
			t.Fatal("email restoration changed")
		}
	}
}

func TestBestEffortMasksUncertainAndSecrets(t *testing.T) {
	for _, input := range []string{"-----BEGIN PRIVATE KEY-----\nSYNTHETIC_KEY_BODY\n-----END PRIVATE KEY-----", "sk-test_key_000000000000", "Ólafur Sigurðsson", "jón@example.com", "Laugavegur 10, 101 Reykjavík", "a\u200bb anna@example.com", "⟦NAME-fake⟧ anna@example.com"} {
		s := NewSession()
		r := s.ProcessBestEffort(input, nil)
		if !r.Allowed || r.Transformed == input || r.Transformed == "" {
			t.Fatalf("best effort did not mask: %+v", r)
		}
		if s.Restore(r.Transformed) != input {
			t.Fatal("restoration mismatch")
		}
		for _, finding := range r.Findings {
			if finding.Action != "mask" {
				t.Fatal("content blocker in best effort")
			}
		}
	}
}
func TestBestEffortPartialOverlapCoversUnion(t *testing.T) {
	input := "Anna anna@example.com"
	s := NewSession()
	r := s.ProcessBestEffort(input, []NameSpan{{0, 9}})
	if !r.Allowed || strings.Contains(r.Transformed, "Anna") || strings.Contains(r.Transformed, "example") || s.Restore(r.Transformed) != input {
		t.Fatalf("union not covered: %+v", r)
	}
}
