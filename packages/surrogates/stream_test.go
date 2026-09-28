package surrogates

import (
	"strings"
	"testing"
)

func TestStreamRestoresEveryByteBoundary(t *testing.T) {
	s := NewSession()
	masked := s.Process("Arna Jónsdóttir og 140182-8858, demo@example.com").Transformed
	input := "Svar: " + masked + " — endir."
	want := "Svar: Arna Jónsdóttir og 140182-8858, demo@example.com — endir."
	for size := 1; size <= len(input); size++ {
		r := s.NewStreamRestorer()
		var got strings.Builder
		for start := 0; start < len(input); start += size {
			end := min(start+size, len(input))
			got.WriteString(r.Write(input[start:end]))
		}
		got.WriteString(r.Flush())
		if got.String() != want {
			t.Fatalf("chunk size %d: got %q", size, got.String())
		}
	}
}

func TestStreamDoesNotGuessChangedMarker(t *testing.T) {
	s := NewSession()
	masked := s.Process("140182-8858").Transformed
	changed := strings.Replace(masked, "XXXXX", "XXXXY", 1)
	r := s.NewStreamRestorer()
	got := r.Write(changed[:3]) + r.Write(changed[3:]) + r.Flush()
	if got != changed {
		t.Fatalf("changed marker was incorrectly restored: %q", got)
	}
}

func TestResetStopsOldStreamRestoration(t *testing.T) {
	s := NewSession()
	masked := s.Process("140182-8858").Transformed
	r := s.NewStreamRestorer()
	first := r.Write(masked[:5])
	s.Reset()
	got := first + r.Write(masked[5:]) + r.Flush()
	if got != masked {
		t.Fatalf("old mapping survived reset: %q", got)
	}
}
