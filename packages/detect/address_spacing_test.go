package detect

import "testing"

// Fabricated vocabulary only. These cases are development regressions, not a
// representative or independently annotated address evaluation.
func TestGazetteerAddressSpacing(t *testing.T) {
	l := fixtureLexicon(t)
	for _, value := range []string{
		"Gervi\u00a0Slóð 99", "Gervi\u202fSlóð 99",
		"Gervislóð 97 - 99", "Gervislóð 97 – 99",
		"Gervislóð 97\u00a0–\u00a099", "Gervislóð 99 / 2",
		"Gervislóð 97 - 99, 999 Sýnibær",
	} {
		got := l.Detect(value, nil)
		if len(got) != 1 || got[0].Kind != Address || got[0].Start != 0 || got[0].End != len(value) {
			t.Errorf("incomplete address %q: %#v", value, got)
		}
	}
	for _, value := range []string{
		"Gervislóð 99\u00a0kr.", "Gervislóð 99\u202fISK",
		"Gervislóð 99,\u00a050 kr.", "Gervislóð 99.\u202f50 kr.",
		"Gervislóð 99 / 9999", "Gervislóð 99 – 9999",
		"Gervi\nSlóð 99", "Gervislóð 97\n– 99",
	} {
		// A line break can leave a legitimate first address, but must not join
		// the range across lines. The remaining cases must not match at all.
		got := l.Detect(value, nil)
		if value == "Gervislóð 97\n– 99" {
			if len(got) != 1 || value[got[0].Start:got[0].End] != "Gervislóð 97" {
				t.Errorf("cross-line range handling %q: %#v", value, got)
			}
		} else if len(got) != 0 {
			t.Errorf("false address %q: %#v", value, got)
		}
	}
}
