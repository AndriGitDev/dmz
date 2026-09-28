package surrogates

import (
	"strings"
	"testing"

	"github.com/AndriGitDev/dmz/packages/detect"
)

func TestSwapAndRestoreConsistently(t *testing.T) {
	s := NewSession()
	input := "Arna Jónsdóttir, 140182-8858, demo@example.com. Sömu kt: 140182-8858."
	got := s.Process(input)
	if !got.Allowed || got.Transformed == input || strings.Contains(got.Transformed, "140182-8858") || strings.Contains(got.Transformed, "demo@example.com") {
		t.Fatalf("not safely swapped: %#v", got)
	}
	if !strings.Contains(got.Transformed, "XXXXX-XXXX") || !strings.Contains(got.Transformed, "XXXXX@XXXXX.XXX") {
		t.Fatalf("visible masks missing: %q", got.Transformed)
	}
	if s.Restore("Svar við "+got.Transformed) != "Svar við "+input {
		t.Fatalf("rehydration failed: %q", s.Restore(got.Transformed))
	}
	again := s.Process("140182-8858")
	if !strings.Contains(got.Transformed, again.Transformed) {
		t.Fatalf("same value changed across prompts: %q vs %q", got.Transformed, again.Transformed)
	}
}

func TestBlockedCasesNeverReturnOutboundText(t *testing.T) {
	for _, input := range []string{
		"0000001", // ambiguous local seven-digit number
		"sk-test_key_000000000000",
	} {
		got := NewSession().Process(input)
		if got.Allowed || got.Transformed != "" || len(got.Findings) == 0 {
			t.Fatalf("should block %q: %#v", input, got)
		}
	}
}

func TestPlausibleNewKennitalaAndCardMask(t *testing.T) {
	s := NewSession()
	got := s.Process("140182-8868 og 4111 1111 1111 1111")
	if !got.Allowed || !strings.Contains(got.Transformed, "-1: XXXXX-XXXX⟧") || !strings.Contains(got.Transformed, "-2: XXXX XXXX XXXX XXXX⟧") {
		t.Fatalf("expected visible, distinct masks: %#v", got)
	}
	if len(got.Findings) != 2 || got.Findings[0].Confidence != detect.Uncertain {
		t.Fatalf("expected uncertain kennitala finding: %#v", got.Findings)
	}
	if s.Restore(got.Transformed) != "140182-8868 og 4111 1111 1111 1111" {
		t.Fatal("rehydration failed")
	}
}

func TestResetRemovesMapping(t *testing.T) {
	s := NewSession()
	swapped := s.Process("140182-8858").Transformed
	if s.Restore(swapped) != "140182-8858" {
		t.Fatal("mapping absent before reset")
	}
	s.Reset()
	if s.Restore(swapped) != swapped {
		t.Fatal("mapping survived reset")
	}
}

func TestNameCaseForms(t *testing.T) {
	s := NewSession()
	got := s.Process("Arna Jónsdóttir og Arnu Jónsdóttur")
	if !got.Allowed || !strings.Contains(got.Transformed, "-1: XXXXX XXXXX⟧") || !strings.Contains(got.Transformed, "-2: XXXXX XXXXX⟧") {
		t.Fatalf("distinct name masks missing: %#v", got)
	}
	if s.Restore(got.Transformed) != "Arna Jónsdóttir og Arnu Jónsdóttur" {
		t.Fatal("case rehydration failed")
	}
}

func TestSessionNamespacePreventsWrongRestoration(t *testing.T) {
	s := NewSession()
	old := s.Process("140182-8858").Transformed
	s.Reset()
	current := s.Process("140182-8868").Transformed
	if old == current {
		t.Fatal("markers collided across sessions")
	}
	if s.Restore(old) != old {
		t.Fatal("old response was restored using new session mapping")
	}
	if s.Restore(current) != "140182-8868" {
		t.Fatal("new session marker failed to restore")
	}
}

func TestTenantDictionaryStaysLocalAndResetsMapping(t *testing.T) {
	s := NewSession()
	old := s.Process("140182-8858").Transformed
	if err := s.SetTenantTerms([]string{"Verkefni Gervi", "Verkefni Gervi"}); err != nil {
		t.Fatal(err)
	}
	if s.Restore(old) != old {
		t.Fatal("dictionary update kept old mapping")
	}
	result := s.Process("Ræddu Verkefni Gervi, ekki ÓVerkefni Gervi.")
	if !result.Allowed || !strings.Contains(result.Transformed, "⟦TERM-") || strings.Contains(result.Transformed, "Ræddu Verkefni Gervi,") {
		t.Fatalf("tenant term not masked: %#v", result)
	}
	if s.Restore(result.Transformed) != "Ræddu Verkefni Gervi, ekki ÓVerkefni Gervi." {
		t.Fatal("term restoration failed")
	}
	if err := s.SetTenantTerms([]string{"ab"}); err == nil {
		t.Fatal("short tenant term accepted")
	}
	if s.Restore(result.Transformed) != "Ræddu Verkefni Gervi, ekki ÓVerkefni Gervi." {
		t.Fatal("invalid update reset mapping")
	}
}
