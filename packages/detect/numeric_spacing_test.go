package detect

import (
	"strings"
	"testing"
)

func TestHorizontalNumericTypography(t *testing.T) {
	for _, sep := range []string{"\t", "\u00a0", "\u202f", "\u2011", "\u2013"} {
		for _, tc := range []struct {
			kind Kind
			raw  string
		}{
			{Kennitala, "140182" + sep + "8858"},
			{Phone, "+354" + sep + "000" + sep + "0002"},
			{Card, strings.Join([]string{"5555", "5555", "5555", "4444"}, sep)},
			{IBAN, strings.Join([]string{"IS46", "0000", "0000", "0000", "0000", "0000", "00"}, sep)},
		} {
			got := Detect(tc.raw)
			if len(got) != 1 || got[0].Kind != tc.kind || got[0].Start != 0 || got[0].End != len(tc.raw) {
				t.Errorf("incomplete synthetic %s %q: %#v", tc.kind, tc.raw, got)
			}
		}
	}
}

func TestHorizontalNumericTypographyRejectsBadAndCrossLineShapes(t *testing.T) {
	for _, value := range []string{
		"140182\t8858X", "X140182–8858", "000\t0002\t4",
		"5555\t5555\t5555\t4445", "IS47\t0000\t0000\t0000\t0000\t0000\t00",
		"140182\n8858", "140182\r8858", "140182\u20288858",
		"5555 5555\n5555 4444", "29\t02\t1800", "12–05–2026",
	} {
		for _, match := range Detect(value) {
			if match.Kind == Kennitala || match.Kind == Card || match.Kind == IBAN || match.Kind == Phone {
				t.Errorf("false numeric match for %q: %#v", value, match)
			}
		}
	}
}

func TestIBANHorizontalFormattingHasStableCanonicalKey(t *testing.T) {
	want := "IS46" + strings.Repeat("0", 22)
	for _, sep := range []string{"\t", "\u00a0", "\u202f", "\u2011", "\u2013"} {
		value := strings.Join([]string{"is46", "0000", "0000", "0000", "0000", "0000", "00"}, sep)
		if Canonical(IBAN, value) != want {
			t.Error("IBAN formatting changed session identity")
		}
	}
}
