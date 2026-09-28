package detect

import (
	"strings"
	"testing"
)

func TestKennitalaLegacyAndNewShape(t *testing.T) {
	cases := []struct {
		text       string
		wantReason string
		want       Confidence
	}{
		{"140182-8858", "legacy-person-check", High},
		{"1401828858", "legacy-person-check", High},
		{"140182-8868", "date-shape-no-check", Uncertain},
		{"540182-8898", "date-shape-no-check", Uncertain},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			got := Detect(tc.text)
			if len(got) != 1 || got[0].Kind != Kennitala || got[0].Confidence != tc.want || got[0].Reason != tc.wantReason {
				t.Fatalf("unexpected detection: %#v", got)
			}
		})
	}
}

func TestKennitalaInvalidDateAndBoundaries(t *testing.T) {
	for _, s := range []string{"310282-8858", "001382-8858", "1140182-8858", "140182-88581"} {
		for _, match := range Detect(s) {
			if match.Kind == Kennitala {
				t.Errorf("%q should not be a kennitala, got %#v", s, match)
			}
		}
	}
}

func TestEmailAndUnicodeByteOffsets(t *testing.T) {
	input := "Halló demo@example.com"
	got := Detect(input)
	if len(got) != 1 || got[0].Kind != Email || input[got[0].Start:got[0].End] != "demo@example.com" {
		t.Fatalf("unexpected spans: %#v", got)
	}
}

func TestIBANFormattingAndChecksum(t *testing.T) {
	for _, value := range []string{
		"IS460000000000000000000000",
		"IS46 0000 0000 0000 0000 0000 00",
		"IS46-0000-0000-0000-0000-0000-00",
	} {
		matches := Detect(value)
		if len(matches) != 1 || matches[0].Kind != IBAN || matches[0].Start != 0 || matches[0].End != len(value) {
			t.Errorf("%q: %#v", value, matches)
		}
		if got := Canonical(IBAN, value); got != "IS460000000000000000000000" {
			t.Errorf("canonical %q = %q", value, got)
		}
	}
	for _, value := range []string{"IS990000000000000000000000", "IS46-0000-0000-0000-0000-0000-01"} {
		if matches := Detect(value); len(matches) != 0 {
			t.Errorf("invalid IBAN %q: %#v", value, matches)
		}
	}
}

func TestPhoneConfidence(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  Confidence
	}{{"+354 0000001", High}, {"0000001", Uncertain}} {
		got := Detect(tc.input)
		if len(got) != 1 || got[0].Kind != Phone || got[0].Confidence != tc.want {
			t.Errorf("%q: %#v", tc.input, got)
		}
	}
}

func TestCardAndSecret(t *testing.T) {
	input := "4111 1111 1111 1111 and sk-test_key_000000000000"
	got := Detect(input)
	if len(got) != 2 || got[0].Kind != Card || got[1].Kind != Secret {
		t.Fatalf("unexpected detection: %#v", got)
	}
	if got := Detect(strings.Replace(input, "4111 1111 1111 1111", "4111 1111 1111 1112", 1)); len(got) != 1 || got[0].Kind != Secret {
		t.Fatalf("bad Luhn number should not match: %#v", got)
	}
}

func TestHandAuthoredNameForms(t *testing.T) {
	input := "Arna Jónsdóttir hitti Arnu Jónsdóttur."
	got := Detect(input)
	if len(got) != 2 || got[0].Kind != Name || got[1].Kind != Name {
		t.Fatalf("unexpected forms: %#v", got)
	}
}
