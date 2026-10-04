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
	for _, tc := range []struct {
		input, value string
		confidence   Confidence
	}{
		{"Halló demo@example.com", "demo@example.com", High},
		{"Alþjóðlegt próffang próf@example.invalid", "próf@example.invalid", Uncertain},
	} {
		got := Detect(tc.input)
		if len(got) != 1 || got[0].Kind != Email || got[0].Confidence != tc.confidence || tc.input[got[0].Start:got[0].End] != tc.value {
			t.Errorf("unexpected email span for %q: %#v", tc.input, got)
		}
	}
}

func TestEmailCanonicalPreservesLocalPartCase(t *testing.T) {
	if got := Canonical(Email, "Demo@EXAMPLE.INVALID"); got != "Demo@example.invalid" {
		t.Fatalf("canonical email: %q", got)
	}
	if Canonical(Email, "Demo@example.invalid") == Canonical(Email, "demo@example.invalid") {
		t.Fatal("distinct email local parts merged")
	}
}

func TestUnicodeObfuscationRequiresReview(t *testing.T) {
	for _, input := range []string{
		"x\u0301", // no composed character; unresolved mark remains guarded
		"demo@exa\u200bmple.invalid",
		"demo@exam\u202eple.invalid",
		"bad\xffbytes",
	} {
		matches := Detect(input)
		found := false
		for _, match := range matches {
			if match.Kind == UnicodeReview && match.Confidence == Uncertain {
				found = true
			}
		}
		if !found {
			t.Errorf("expected Unicode review for %q: %#v", input, matches)
		}
	}
	if matches := Detect("Prófrún Gervidóttir"); len(matches) != 1 || matches[0].Kind != Name {
		t.Fatalf("precomposed Icelandic name changed: %#v", matches)
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
	}{{"+354 0000001", High}, {"00354 000 0001", Uncertain}, {"0000001", Uncertain}} {
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

func TestJWTAndBearerTokenBlockSpans(t *testing.T) {
	for _, tc := range []struct{ text, value, reason string }{
		{"JWT eyJ0ZXN0Ijp0cnVlfQ.eyJhdWQiOiJ0ZXN0In0.c2lnbmF0dXJl.", "eyJ0ZXN0Ijp0cnVlfQ.eyJhdWQiOiJ0ZXN0In0.c2lnbmF0dXJl", "jwt-shape"},
		{"Authorization: Bearer test_token_abcdefghijklmnop.", "test_token_abcdefghijklmnop", "bearer-token"},
	} {
		matches := Detect(tc.text)
		if len(matches) != 1 || matches[0].Kind != Secret || matches[0].Reason != tc.reason || tc.text[matches[0].Start:matches[0].End] != tc.value {
			t.Errorf("%q: %#v", tc.text, matches)
		}
	}
}

func TestGitHubStyleSecretShape(t *testing.T) {
	value := "ghp_000000000000000000000000000000000000"
	input := "Tilbúinn lykill " + value + "."
	matches := Detect(input)
	if len(matches) != 1 || matches[0].Kind != Secret || input[matches[0].Start:matches[0].End] != value {
		t.Fatalf("GitHub-style token: %#v", matches)
	}
	for _, value := range []string{
		"ghp_00000000000000000000000000000000000",
		"ghp_0000000000000000000000000000000000000",
	} {
		if matches := Detect(value); len(matches) != 0 {
			t.Errorf("invalid token length %q: %#v", value, matches)
		}
	}
}

func TestHandAuthoredNameForms(t *testing.T) {
	input := "Arna Jónsdóttir hitti Arnu Jónsdóttur."
	got := Detect(input)
	if len(got) != 2 || got[0].Kind != Name || got[1].Kind != Name {
		t.Fatalf("unexpected forms: %#v", got)
	}
}

func TestUnlistedPatronymicRequiresReview(t *testing.T) {
	for _, name := range []string{"Guðrún Sigurðardóttir", "Elín Björnsdóttir", "Jón Jónssonar", "Prófrún Gerviburs"} {
		matches := Detect("Hitti " + name + ".")
		if len(matches) != 1 || matches[0].Kind != Name || matches[0].Confidence != Uncertain || matches[0].Reason != "patronymic-shape-without-lexicon" {
			t.Errorf("%q: %#v", name, matches)
		}
	}
	for _, text := range []string{"Project Johnson", "Stafrænt Ísland", "Guðrún SigurðardóttirX"} {
		if matches := Detect(text); len(matches) != 0 {
			t.Errorf("false patronymic match %q: %#v", text, matches)
		}
	}
}

func TestAddressShapeRequiresReview(t *testing.T) {
	for _, address := range []string{"Prófgata 99, 999 Reykjavík", "Laugavegur 10, 101 Reykjavík", "Prófgata 99, 999", "Prófgata 99 999 Reykjavík"} {
		matches := Detect(address)
		if len(matches) != 1 || matches[0].Kind != Address || matches[0].Confidence != Uncertain || address[matches[0].Start:matches[0].End] != address {
			t.Errorf("%q: %#v", address, matches)
		}
	}
	for _, text := range []string{"Invoice 99, 999 Items", "Prófgata 99 999", "Prófgata 99, 99 Reykjavík"} {
		if matches := Detect(text); len(matches) != 0 {
			t.Errorf("false address match %q: %#v", text, matches)
		}
	}
}

func TestTenantTermsExactAndBounded(t *testing.T) {
	input := "Verkefni Gervi var rætt, ekki ÓVerkefni Gervi eða verkefni gervi."
	got := DetectWithTerms(input, []string{"Verkefni Gervi"})
	if len(got) != 1 || got[0].Kind != TenantDictionary || input[got[0].Start:got[0].End] != "Verkefni Gervi" {
		t.Fatalf("unexpected tenant match: %#v", got)
	}
}

func TestFormattingKeepsExactUTF8Spans(t *testing.T) {
	for _, tc := range []struct {
		value      string
		kind       Kind
		confidence Confidence
	}{
		{"140182.8858", Kennitala, High},
		{"140182\u00a08858", Kennitala, High},
		{"140182\u202f8868", Kennitala, Uncertain},
		{"4111.1111.1111.1111", Card, High},
		{"4111  1111  1111  1111", Card, High},
		{"4111\u202f1111\u202f1111\u202f1111", Card, High},
		{"(+354) 000 0001", Phone, High},
		{"+354\u00a0000.0001", Phone, High},
		{"00354 000.0001", Phone, Uncertain},
		{"000.0001", Phone, Uncertain},
		{"demo@dæmi.invalid", Email, Uncertain},
	} {
		t.Run(tc.value, func(t *testing.T) {
			input := "Í prófun: " + tc.value + "."
			got := Detect(input)
			if len(got) != 1 || got[0].Kind != tc.kind || got[0].Confidence != tc.confidence || input[got[0].Start:got[0].End] != tc.value {
				t.Fatalf("unexpected match: %#v", got)
			}
		})
	}
}

func TestFormattingHardNegatives(t *testing.T) {
	for _, input := range []string{
		"14.01.1882", "192.0.2.1", "123.456,78 kr.",
		"310282.8858", "141382 8858", "X140182.8858Y",
		"4111.1111.1111.1112", "X4111.1111.1111.1111Y",
		"12.000.0001.34", "12\u00a0000\u00a00001\u00a034",
		"demo@example..invalid", "demo@-example.invalid", "demo@example-.invalid",
		".demo@example.invalid", "demo..test@example.invalid",
	} {
		if got := Detect(input); len(got) != 0 {
			t.Errorf("false detection for %q: %#v", input, got)
		}
	}
}
