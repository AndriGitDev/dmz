package surrogates

import (
	"strings"
	"testing"

	"github.com/AndriGitDev/dmz/packages/detect"
	"golang.org/x/text/unicode/norm"
)

func TestCanonicalTextMasksAndRestoresOriginalSpelling(t *testing.T) {
	for _, value := range []string{"Arna Jónsdóttir", "próf@example.invalid", "Tilraunaslóð 12, 999 Sýnibær"} {
		s := NewSession()
		raw := norm.NFD.String(value)
		input := "😀 " + raw + " og " + value
		got := s.ProcessBestEffort(input, nil)
		if !got.Allowed || !got.SpansAvailable || len(got.DetectedSpans) != 2 || strings.Count(got.Transformed, "⟦") != 2 {
			t.Fatalf("canonical equivalent forms were not individually masked: %#v", got)
		}
		if s.Restore(got.Transformed) != input {
			t.Fatalf("original decomposition lost for %q", value)
		}
		for _, span := range got.DetectedSpans {
			if span.Kind == detect.UnicodeReview {
				t.Fatal("ordinary decomposition still used whole-text fallback")
			}
		}
	}
	input := norm.NFD.String("Góður dagur.")
	got := NewSession().ProcessBestEffort(input, nil)
	if !got.Allowed || got.Transformed != input || !got.SpansAvailable || len(got.DetectedSpans) != 0 {
		t.Fatalf("benign canonical text was altered: %#v", got)
	}
}

func TestModelSpanCannotSplitCanonicalCombiningSequence(t *testing.T) {
	input := "A\u0301sa"
	for _, span := range []NameSpan{{0, 1}, {1, len(input)}} {
		got := NewSession().ProcessBestEffort(input, []NameSpan{span})
		if got.Allowed || got.SpansAvailable {
			t.Fatalf("partial combining sequence accepted: %#v", got)
		}
	}
	s := NewSession()
	got := s.ProcessBestEffort(input, []NameSpan{{0, len(input)}})
	if !got.Allowed || !got.SpansAvailable || s.Restore(got.Transformed) != input {
		t.Fatalf("valid original-byte model span rejected: %#v", got)
	}
}
