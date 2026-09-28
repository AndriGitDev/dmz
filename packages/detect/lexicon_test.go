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
