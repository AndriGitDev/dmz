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

func TestEmailLocalPartCaseRestoresSeparately(t *testing.T) {
	s := NewSession()
	input := "Demo@example.invalid og demo@EXAMPLE.INVALID"
	result := s.Process(input)
	if !result.Allowed || len(result.Findings) != 2 || result.Transformed == input {
		t.Fatalf("expected two email masks: %#v", result)
	}
	if strings.Count(result.Transformed, "⟦EMAIL-") != 2 || s.Restore(result.Transformed) != input {
		t.Fatalf("email addresses merged during restoration: %#v", result)
	}
}

func TestBlockedCasesNeverReturnOutboundText(t *testing.T) {
	for _, input := range []string{
		"0000001", // ambiguous local seven-digit number
		"sk-test_key_000000000000",
		"ghp_000000000000000000000000000000000000",
		"00354 000 0000",
		"próf@example.invalid",
		"Prófgata 99, 999",
		"Prófvegur 12 999 Reykjavík",
		"Prófrún Gerviburs",
		"Pro\u0301fru\u0301n Gervido\u0301ttir",
		"demo@exa\u200bmple.invalid",
		"demo@exam\u202eple.invalid",
		"bad\xffbytes",
	} {
		got := NewSession().Process(input)
		if got.Allowed || got.Transformed != "" || len(got.Findings) == 0 {
			t.Fatalf("should block %q: %#v", input, got)
		}
	}
}

func TestTenantTermRejectsCharactersThatAlwaysRequireReview(t *testing.T) {
	s := NewSession()
	for _, term := range []string{"Pro\u0301fverkefni", "Próf\u200bverkefni", "bad\xffbytes"} {
		if err := s.SetTenantTerms([]string{term}); err == nil {
			t.Errorf("unusable tenant term accepted: %q", term)
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

func TestUnknownPatronymicBlocksWithoutOutboundText(t *testing.T) {
	got := NewSession().Process("Ræddu Guðrún Sigurðardóttir.")
	if got.Allowed || got.Transformed != "" || len(got.Findings) != 1 || got.Findings[0].Action != "review" {
		t.Fatalf("unknown patronymic should block for review: %#v", got)
	}
}

func TestAddressShapeBlocksWithoutOutboundText(t *testing.T) {
	got := NewSession().Process("Sendu á Prófgata 99, 999 Reykjavík.")
	if got.Allowed || got.Transformed != "" || len(got.Findings) != 1 || got.Findings[0].Action != "review" {
		t.Fatalf("address candidate should block for review: %#v", got)
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
