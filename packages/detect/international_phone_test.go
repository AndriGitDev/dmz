package detect

import (
	"strings"
	"testing"
)

func TestInternationalPhoneOriginalByteSpan(t *testing.T) {
	for _, raw := range []string{"+1 202-555-0107", "(+1) 202-555-0107", "(+44) 20 7946 0123", "+12025550114", "+1 (202) 555-0138", "+1.202.555.0148", "+1 202-555-0162 ext. 45", "+44 20 7946 0123", "+44 (20) 7946 0789", "+1\u00a0202\u00a0555\u00a00107"} {
		for _, prefix := range []string{"Phone: ", "Þú: ", "Jose\u0301: "} {
			input := prefix + "(" + raw + ")."
			matches := Detect(input)
			found := false
			for _, m := range matches {
				if m.Kind == Phone {
					if m.Start != len(prefix)+1 || m.End != len(prefix)+1+len(raw) {
						t.Fatalf("wrong original phone boundary: %+v", m)
					}
					found = true
				}
			}
			if !found {
				t.Fatalf("missed reserved synthetic phone form %q", raw)
			}
		}
	}
}

func TestInternationalPhonePreservesOtherEntityTypes(t *testing.T) {
	for _, test := range []struct {
		text string
		kind Kind
	}{
		{"Email +12025550107@example.invalid", Email},
		{"password=\"+12025550107AbC9zX8qW7\"", Secret},
		{"Test card 4111 1111 1111 1111", Card},
		{"IBAN IS140159260076545510730339", IBAN},
	} {
		found := false
		for _, m := range Detect(test.text) {
			if m.Kind == Phone {
				t.Fatal("phone replaced or split a protected entity")
			}
			if m.Kind == test.kind {
				found = true
			}
		}
		if !found {
			t.Fatalf("lost %s", test.kind)
		}
	}
}

func TestInternationalPhoneDoesNotAssumeDomesticRegion(t *testing.T) {
	for _, text := range []string{"(202) 555-0114", "020 7946 0123", "Version 1.202.555.0107", "Compute +1 + 202 + 555 + 107.", "ABC+12025550107XYZ", "+120255501071234567890"} {
		if len(internationalPhones(text, nil)) != 0 {
			t.Fatalf("unjustified international phone in %q", text)
		}
	}
}

func TestInternationalPhoneAfterManyUnrelatedNumbers(t *testing.T) {
	prefix := strings.Repeat("12 34 56. ", 1500)
	phone := "+1 202-555-0107"
	matches := internationalPhones(prefix+phone, nil)
	if len(matches) != 1 || matches[0].Start != len(prefix) || matches[0].End != len(prefix)+len(phone) {
		t.Fatal("numeric noise exhausted recognition before the phone")
	}
}

func BenchmarkInternationalPhoneNoise(b *testing.B) {
	input := strings.Repeat("+1 + 23 + 456 + 78; ", 1000) + "Call +1 202-555-0107"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		internationalPhones(input, nil)
	}
}
