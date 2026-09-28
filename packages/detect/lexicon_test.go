package detect

import (
	"strings"
	"testing"
)

func fixtureLexicon(t *testing.T) *Lexicon {
	t.Helper()
	l, err := LoadLexicon([]byte(`{"schema":1,"names":["Mira Demo","Mira","Prófrún Gervi","Demo A . Gervi"],"addresses":["Gervislóð","Gervislóðinni","Gervi Slóð"]}`))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestLexiconNamesUseOriginalByteSpans(t *testing.T) {
	l := fixtureLexicon(t)
	for _, value := range []string{"Mira Demo", "Prófrún Gervi", "Demo A. Gervi"} {
		input := "Í prófun: " + value + "."
		got := l.Detect(input, nil)
		if len(got) != 1 || got[0].Kind != Name || got[0].Confidence != Uncertain || input[got[0].Start:got[0].End] != value {
			t.Fatalf("bad match: %#v", got)
		}
	}
	for _, input := range []string{"XMira DemoX", "mira demo", "Mira\nDemo"} {
		for _, m := range l.Detect(input, nil) {
			if input[m.Start:m.End] == "Mira\nDemo" {
				t.Fatal("cross-line match")
			}
		}
	}
	if got := l.Detect("Mira\u200b Demo", nil); len(got) == 0 || got[0].Kind != UnicodeReview {
		t.Fatal("lexicon bypassed unicode review")
	}
}

func TestGazetteerRequiresAddressShapeAndRejectsPrices(t *testing.T) {
	l := fixtureLexicon(t)
	for _, value := range []string{"Gervislóð 99/2, 999 Sýnibær", "gervislóð 99b, 999", "Gervislóðinni 99, 999", "Gervi Slóð 99 999 Sýnibær"} {
		input := "Tilbúið: " + value + "."
		got := l.Detect(input, nil)
		if len(got) != 1 || got[0].Kind != Address || got[0].Confidence != Uncertain || input[got[0].Start:got[0].End] != value {
			t.Fatalf("bad address %q: %#v", value, got)
		}
	}
	for _, input := range []string{"Prófgata 99, 999 krónur", "Prófgata 99, 999 ISK", "Gervislóð er staður.", "Gervislóð 99", "Gervislóð 99, 999 krónur", "Gervislóð 99, 999 ISK", "Gervislóð 99, 999 kr.", "Gervislóð 99, 9999", "Gervislóð\n99, 999"} {
		if got := l.Detect(input, nil); len(got) != 0 {
			t.Fatalf("false positive %q: %#v", input, got)
		}
	}
}

func TestLexiconDoesNotSuppressSecretsOrTenantTerms(t *testing.T) {
	l := fixtureLexicon(t)
	got := l.Detect("sk-test_key_000000000000 og Verkefni Gervi", []string{"Verkefni Gervi"})
	if len(got) != 2 || got[0].Kind != Secret || got[1].Kind != TenantDictionary {
		t.Fatalf("lost baseline rules: %#v", got)
	}
}

func TestInvalidLexiconFailsWithoutEchoingEntries(t *testing.T) {
	for _, data := range []string{`{}`, `{"schema":2}`, `{"schema":1,"names":["bad_secret_123"]}`, `{"schema":1,"names":["Mira  Demo"]}`, `{"schema":1,"names":["Mira\u200b"]}`} {
		if _, err := LoadLexicon([]byte(data)); err == nil || strings.Contains(err.Error(), "bad_secret") {
			t.Fatalf("bad error for invalid pack: %v", err)
		}
	}
}

func TestLearnedNameBoundaries(t *testing.T) {
	l, err := LoadLexicon([]byte(`{"schema":1,"names":["Mira","Lira Gervi"],"name_parts":["Prófa","Mira","Lira","Gervi","Sýnisson"]}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		input string
		want  []string
	}{
		{"Í prófun: Prófa Mira Gervi.", []string{"Prófa Mira Gervi"}},
		{"Í prófun: Prófa Lira Sýnisson.", []string{"Prófa Lira Sýnisson"}},
		{"Prófa\nMira", []string{"Mira"}},
		{"Prófa, Mira", []string{"Mira"}},
		{"Prófa. Mira", []string{"Mira"}},
		{"Prófa óþekkt Mira", []string{"Mira"}},
		{"Prófa", nil},
		{"Prófa Lira Sýnisson Mira Gervi", []string{"Prófa Lira Sýnisson", "Mira Gervi"}},
	} {
		got := l.Detect(tc.input, nil)
		if len(got) != len(tc.want) {
			t.Fatalf("%q: %#v", tc.input, got)
		}
		for i, m := range got {
			if m.Kind != Name || tc.input[m.Start:m.End] != tc.want[i] || m.Confidence != Uncertain {
				t.Fatalf("%q: %#v", tc.input, got)
			}
		}
	}
	for _, part := range []string{"lower", "A", ".", "Mira Demo"} {
		if _, err := LoadLexicon([]byte(`{"schema":1,"name_parts":["` + part + `"]}`)); err == nil {
			t.Fatal("accepted invalid boundary part")
		}
	}
}

func TestNameSeedsRequireClosedBoundaries(t *testing.T) {
	l, err := LoadLexicon([]byte(`{"schema":1,"names":["Mira Gervi"],"name_seeds":["Mira"],"name_parts":["Mira","Gervi"]}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		input string
		want  []string
	}{
		{"Mira", []string{"Mira"}},
		{"til Mira.", []string{"Mira"}},
		{"próf: Mira Gervi.", []string{"Mira Gervi"}},
		{"Óþekkt Mira", nil},
		{"Mira Óþekkt", nil},
		{"X. Mira", nil},
		{"X . Mira", nil},
		{"X.\nMira", []string{"Mira"}},
		{"Óþekkt\nMira", []string{"Mira"}},
		{"Óþekkt, Mira", []string{"Mira"}},
		{"XMiraX", nil},
		// A declined seed must not erase an existing full-name match.
		{"Óþekkt Mira Gervi", []string{"Mira Gervi"}},
	} {
		got := l.Detect(tc.input, nil)
		if len(got) != len(tc.want) {
			t.Fatalf("%q: %#v", tc.input, got)
		}
		for i, m := range got {
			if m.Kind != Name || m.Confidence != Uncertain || tc.input[m.Start:m.End] != tc.want[i] {
				t.Fatalf("%q: %#v", tc.input, got)
			}
		}
	}
	for _, seed := range []string{"lower", "A", ".", "Mira Demo"} {
		if _, err := LoadLexicon([]byte(`{"schema":1,"name_seeds":["` + seed + `"]}`)); err == nil {
			t.Fatal("accepted invalid seed")
		}
	}
	got := l.Detect("Mira\u200b", nil)
	if len(got) != 1 || got[0].Kind != UnicodeReview {
		t.Fatal("seed bypassed unicode guard")
	}
	got = l.Detect("Mira", []string{"Mira"})
	if len(got) != 1 || got[0].Kind != TenantDictionary {
		t.Fatal("seed displaced explicit tenant term")
	}
}
