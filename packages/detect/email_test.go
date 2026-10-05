package detect

import (
	"strings"
	"testing"
)

func TestCompleteEmailLocalPartsAndContexts(t *testing.T) {
	for _, local := range []string{"o'connor", "team!#$%&'*+-/=?^_`{|}~.one", `"team one"`, `"team\"one"`, `"team@one"`, "📮", "próf"} {
		value := local + "@example.invalid"
		for _, wrapper := range [][2]string{{"Contact ", " today."}, {"<", ">"}, {"(", ")"}, {"\"", "\""}} {
			input := wrapper[0] + value + wrapper[1]
			matches := emailMatches(input)
			if len(matches) != 1 || input[matches[0].Start:matches[0].End] != value {
				t.Fatalf("incomplete span for %q: %#v", input, matches)
			}
		}
	}
	for _, input := range []string{"https://example.invalid?email=demo@example.invalid", "postgresql:///db?user=demo@example.invalid", "https://example.invalid/people/demo@example.invalid", "https://demo@example.invalid@host.invalid/", "mailto:demo@example.invalid"} {
		got := emailMatches(input)
		if len(got) != 1 || input[got[0].Start:got[0].End] != "demo@example.invalid" {
			t.Fatalf("URI mailbox: %q %#v", input, got)
		}
	}
}

func TestEmailMalformedAndLiteralBoundaries(t *testing.T) {
	for _, input := range []string{"a..b@example.invalid", ".a@example.invalid", "a.@example.invalid", "a@@example.invalid", "a@bad_domain.invalid", "a@-host.invalid", "a@host-.invalid", "a@host..invalid", "a@host.123", "a@[999.0.0.1]", "a@[IPv6:bad]", "a@[IPv6:fe80::1%eth0]", "a@[2001:db8::1]", "a@[192.0.2.1]suffix", "\"a\nb\"@example.invalid", "\"a\\\"@example.invalid", "https://demo@example.invalid/path"} {
		if got := emailMatches(input); len(got) != 0 {
			t.Fatalf("malformed/non-email matched: %q %#v", input, got)
		}
	}
	for _, value := range []string{"a@[192.0.2.1]", "a@[IPv6:2001:db8::1]", "a@example.xn--p1ai"} {
		got := emailMatches(value + ".")
		if len(got) != 1 || got[0].End != len(value) {
			t.Fatalf("literal/TLD: %q %#v", value, got)
		}
	}
	// Multibyte prefixes never change the original byte offsets.
	input := "Halló: o'connor@example.invalid"
	got := emailMatches(input)
	if len(got) != 1 || got[0].Start != strings.Index(input, "o'connor") {
		t.Fatal(got)
	}
}

func TestEmailAssignmentBoundaries(t *testing.T) {
	for _, input := range []string{"EMAIL=demo@example.invalid", "export SUPPORT_EMAIL=demo@example.invalid", "email_address='demo@example.invalid'", "const recipient='demo@example.invalid';", "{email:demo@example.invalid}", "EMAIL=\"demo@example.invalid\""} {
		got := emailMatches(input)
		if len(got) != 1 || input[got[0].Start:got[0].End] != "demo@example.invalid" {
			t.Fatalf("assignment included in mailbox: %q %#v", input, got)
		}
	}
	for _, value := range []string{"team=one@example.invalid", "email=demo@example.invalid"} {
		input := "Contact " + value + " today."
		got := emailMatches(input)
		if len(got) != 1 || input[got[0].Start:got[0].End] != value {
			t.Fatalf("literal punctuation discarded: %q %#v", input, got)
		}
	}
}
