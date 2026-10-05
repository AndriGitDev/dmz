package detect

import (
	"fmt"
	"strings"
	"testing"
)

func TestVendorPyPILengthBounds(t *testing.T) {
	for _, n := range []int{49, 50, 51, 999, 1000, 1001, 4096} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			body := strings.Repeat("aB3dE6gH9jK2mN5pQ8sT1vW4yZ7cF0iL", n/32+1)[:n]
			value := "pypi-AgEIcHlwaS5vcmc" + body
			matches := vendorSecrets("🙂 ["+value+"].", nil)
			want := n >= 50 && n <= 1000
			if (len(matches) == 1) != want || len(matches) > 1 {
				t.Fatalf("length %d: got %d matches, accepted=%v", n, len(matches), want)
			}
			if want && (matches[0].Start != len("🙂 [") || matches[0].End != len("🙂 [")+len(value)) {
				t.Fatal("incorrect original byte span")
			}
		})
	}
}

func TestVendorSecretOriginalSpansAndPunctuation(t *testing.T) {
	value := "npm_aB3dE6gH9jK2mN5pQ8sT1vW4yZ7cF0iL3oR6"
	for _, input := range []string{
		value, "Rotate " + value + ", please.", "A\u0301ætlun 🙂: [" + value + "].",
		"before\r\n'" + value + "'\r\nafter", value + " # gitleaks:allow",
	} {
		matches := Detect(input)
		if len(matches) != 1 || matches[0].Kind != Secret || input[matches[0].Start:matches[0].End] != value {
			t.Fatalf("vendor token original span was not preserved: %#v", matches)
		}
	}
	input := value + "; " + value
	if matches := Detect(input); len(matches) != 2 || matches[0].Start == matches[1].Start {
		t.Fatalf("repeated occurrences must retain separate source spans: %#v", matches)
	}
}

func TestVendorSecretRejectsPartialTokens(t *testing.T) {
	value := "npm_aB3dE6gH9jK2mN5pQ8sT1vW4yZ7cF0iL3oR6"
	for _, input := range []string{
		"widget_" + value, value + "_counter", "á" + value, value + "ö",
		"prefix." + value, value + ".suffix", "-" + value, value + "-more",
		value + "+more", value + "/more", value + "=", "npm_" + strings.Repeat("a", 36),
		"hf_" + strings.Repeat("ſ", 34),
	} {
		for _, match := range Detect(input) {
			if match.Kind == Secret {
				t.Fatalf("partial, embedded or unsupported token accepted: %#v", match)
			}
		}
	}
}

func TestVendorSecretPreservesExistingEntities(t *testing.T) {
	value := "npm_aB3dE6gH9jK2mN5pQ8sT1vW4yZ7cF0iL3oR6"
	input := value + "@example.invalid"
	got := Detect(input)
	if len(got) != 1 || got[0].Kind != Email || got[0].Start != 0 || got[0].End != len(input) {
		t.Fatalf("vendor supplement displaced the complete email: %#v", got)
	}
	got = DetectWithTerms(value, []string{value})
	if len(got) != 1 || got[0].Kind != TenantDictionary {
		t.Fatalf("vendor supplement displaced tenant term: %#v", got)
	}
}
