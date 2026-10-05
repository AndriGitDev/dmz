package detect

import (
	"strings"
	"testing"
)

func TestAddressTypographicHyphens(t *testing.T) {
	lexicon, err := LoadLexicon([]byte(`{"schema":1,"names":["Gervi-Mira"],"addresses":["Gervi-Slóð"]}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, hyphen := range []string{"-", "‐", "‑"} {
		for _, value := range []string{
			"Gervi" + hyphen + "Slóð 97" + hyphen + "99",
			"GERVI" + hyphen + "SLÓÐ\u00a097 " + hyphen + " 99, 999 Sýnibær",
			"Prófgata 97" + hyphen + "99, 999 Sýnibær",
		} {
			input := "🙂 Sendu á " + value + "."
			got := lexicon.Detect(input, nil)
			if len(got) != 1 || got[0].Kind != Address || input[got[0].Start:got[0].End] != value {
				t.Fatalf("incomplete typed address for %q: %#v", value, got)
			}
		}
		for _, suffix := range []string{"97" + hyphen + "9999", "97" + hyphen + "99 kr.", "97" + hyphen + "99 / 2", "97" + hyphen + "\n99"} {
			input := "Gervi" + hyphen + "Slóð " + suffix
			for _, match := range lexicon.Detect(input, nil) {
				if match.Kind == Address && (strings.Contains(suffix, "\n") && strings.Contains(input[match.Start:match.End], "\n") || !strings.Contains(suffix, "\n")) {
					t.Fatalf("invalid address tail %q: %#v", input, match)
				}
			}
		}
	}
	// Address typography must not broaden byte-exact name vocabulary matching.
	if got := lexicon.Detect("Gervi‑Mira", nil); len(got) != 0 {
		t.Fatalf("name matching changed: %#v", got)
	}
}
