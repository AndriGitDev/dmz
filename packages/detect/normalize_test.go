package detect

import (
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

func TestCanonicalCompositionUsesOriginalByteOffsets(t *testing.T) {
	for _, example := range []struct {
		value string
		kind  Kind
	}{
		{"Arna Jónsdóttir", Name},
		{"Prófrún Gervidóttir", Name},
		{"próf@example.invalid", Email},
		{"Tilraunaslóð 12 / 2, 999 Sýnibær", Address},
	} {
		raw := norm.NFD.String(example.value)
		prefix := "😀 Í: "
		input := norm.NFD.String(prefix) + raw + "."
		got := detectWithLexicon(input, nil, nil)
		start := len(norm.NFD.String(prefix))
		if len(got) != 1 || got[0].Kind != example.kind || got[0].Start != start || got[0].End != start+len(raw) {
			t.Errorf("normalization lost original span for %q: %#v", example.value, got)
		}
	}
	for _, value := range []string{"Góður dagur.", "ÁÉÍÓÚÝÖ áéíóúýö.", "Ångström élan."} {
		if got := detectWithLexicon(norm.NFD.String(value), nil, nil); len(got) != 0 {
			t.Errorf("canonical prose unnecessarily guarded: %#v", got)
		}
	}
}

func TestNormalizedBoundariesRemainValidAndReversible(t *testing.T) {
	for _, text := range []string{"😀 Ása Örn", "ÁÉÍÓÚÝÖ áéíóúýö", "Ångström\nélan", "가나다 Ása", "Æ Ð Þ"} {
		raw := norm.NFD.String(text)
		normalized, boundaries, ok := normalizedDetectionText(raw)
		if !ok || normalized != norm.NFC.String(raw) || len(boundaries) != len(normalized)+1 {
			t.Fatalf("composition failed for %q", text)
		}
		previous := 0
		for offset, original := range boundaries {
			if original < 0 {
				continue
			}
			if original < previous || original > len(raw) || !utf8.ValidString(raw[:original]) || !utf8.ValidString(normalized[:offset]) {
				t.Fatalf("invalid source boundary for %q", text)
			}
			if norm.NFC.String(raw[:original]) != normalized[:offset] {
				t.Fatalf("mapped prefix changed meaning for %q", text)
			}
			previous = original
		}
		if boundaries[0] != 0 || boundaries[len(normalized)] != len(raw) {
			t.Fatal("normalization omitted original bytes")
		}
	}
}

func TestNormalizationKeepsUnsupportedSequencesGuarded(t *testing.T) {
	for _, input := range []string{"x\u0301", "\u0301a", "a\u0301\u200b", "bad\xffbytes", "a" + strings.Repeat("\u0301", 64)} {
		if _, _, ok := normalizedDetectionText(input); ok {
			t.Errorf("unsupported sequence accepted: %q", input)
		}
		got := detectWithLexicon(input, nil, nil)
		if len(got) == 0 || got[0].Kind != UnicodeReview {
			t.Errorf("unsupported sequence lost review guard: %#v", got)
		}
	}
}

func TestNormalizationPreservesByteExactTenantTerms(t *testing.T) {
	for _, term := range []string{"Prófverkefni", "Ångström"} {
		input := norm.NFD.String(term) + " og " + term + " á."
		got := detectWithLexicon(input, []string{term}, nil)
		start := strings.LastIndex(input, term)
		if len(got) != 1 || got[0].Kind != TenantDictionary || got[0].Start != start || got[0].End != start+len(term) {
			t.Fatalf("canonical composition widened exact dictionary: %#v", got)
		}
	}
}

func TestCanonicalCompositionUsesGazetteer(t *testing.T) {
	l := fixtureLexicon(t)
	input := norm.NFD.String("Gervislóð 99 / 2")
	got := l.Detect(input, nil)
	if len(got) != 1 || got[0].Kind != Address || got[0].Start != 0 || got[0].End != len(input) {
		t.Fatalf("decomposed gazetteer address missed: %#v", got)
	}
}
