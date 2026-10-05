package surrogates

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// Exercise configuration changes, not merely a fixed detector vocabulary.
// All inputs are generated from fabricated seeds and remain local.
func FuzzTenantTermUpdatesPreserveBoundaries(f *testing.F) {
	for _, term := range []string{"Verkefni Gervi", "Áætlun Próf", "A\tB", "0 0 0", "api_key=fakeAb1Cd2Ef3Gh4Ij5Kl6Mn7", "⟦unknown⟧", "a\x00b", "a\u0301b", "  ", "demo@example.test"} {
		f.Add(term)
	}
	f.Fuzz(func(t *testing.T, term string) {
		if !utf8.ValidString(term) || len(term) > 512 {
			t.Skip()
		}
		s := NewSession()
		original := "demo@example.invalid"
		marker := s.ProcessBestEffort(original, nil).Transformed
		err := s.SetTenantTerms([]string{term})
		if err != nil {
			if s.Restore(marker) != original {
				t.Fatal("rejected update discarded mappings")
			}
			return
		}
		if s.Restore(marker) != marker {
			t.Fatal("accepted update retained previous session mapping")
		}
		term = strings.TrimSpace(term)
		if term == "" {
			return
		}
		prefix := "Input: "
		input := prefix + term + " suffix."
		result := s.ProcessBestEffort(input, nil)
		if !result.Allowed || s.Restore(result.Transformed) != input {
			t.Fatal("configured-term round trip changed bytes")
		}
		if result.SpansAvailable {
			start, end := len(prefix), len(prefix)+len(term)
			cursor := start
			for _, span := range result.DetectedSpans {
				if span.End <= cursor {
					continue
				}
				if span.Start > cursor {
					break
				}
				cursor = span.End
				if cursor >= end {
					break
				}
			}
			if cursor < end {
				t.Fatal("accepted exact tenant term was not completely masked")
			}
		}
		repeated := s.ProcessBestEffort(result.Transformed, nil)
		if !repeated.Allowed || repeated.Transformed != result.Transformed {
			t.Fatal("configured term reprocessed an issued marker")
		}
		if s.Restore(repeated.Transformed) != input {
			t.Fatal("resubmission changed configured-term bytes")
		}
	})
}
