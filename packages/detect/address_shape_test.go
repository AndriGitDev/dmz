package detect

import "testing"

// Fabricated street names exercise the fallback independently of a gazetteer.
func TestUnknownAddressShapes(t *testing.T) {
	for _, value := range []string{
		"Tilraunagata 12, 999 Sýnibær",
		"tilraunagata 12, 999 Sýnibær",
		"TILRAUNAGATA 12, 999 Sýnibær",
		"Tilraunaslóð 12 / 2, 999 Sýnibær",
		"Tilraunavegi 12 – 14, 999 Sýnibær",
		"tilraunagötu\u00a012a\u202f-\u202f14B,\u00a0999 Sýnibær",
		"Tilraunastræti 12/2 999 Sýnibær",
		"Tilraunabraut 12-14,999",
	} {
		input := "Heimilisfang: " + value + "."
		got := detectWithLexicon(input, nil, nil)
		if len(got) != 1 || got[0].Kind != Address || got[0].Confidence != Uncertain || input[got[0].Start:got[0].End] != value {
			t.Errorf("incomplete uncertain address %q: %#v", value, got)
		}
	}
	for _, value := range []string{
		"Tilraunagata 12", "tilraunagata 12 / 2", // unknown streets still require postcode
		"Tilraunagata\n12, 999 Sýnibær", "Tilraunagata 12 /\n2, 999 Sýnibær",
		"Tilraunagata 12,\n999 Sýnibær", "Tilraunagata 12 999 kr.",
		"Tilraunagata 12, 999 kr.", "Tilraunagata 12, 999.50 kr.",
		"Tilraunagata 12, 999 ISK", "Tilraunagata 12, 9999",
		"Tilraunagata 12 / 2222, 999 Sýnibær", "Tilraunagata 12, 999/2",
		"7Tilraunagata 12, 999 Sýnibær", "Tilraunagata 1234, 999 Sýnibær",
	} {
		for _, match := range detectWithLexicon(value, nil, nil) {
			if match.Kind == Address {
				t.Errorf("false address %q: %#v", value, match)
			}
		}
	}
}
