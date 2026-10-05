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

func TestResetNeverRebindsOldStreamToNewMappings(t *testing.T) {
	s := NewSession()
	r := s.NewStreamRestorer()
	s.Reset()
	marker := s.Process("demo@example.invalid").Transformed
	if got := r.Write("after reset "); got != "after reset " {
		t.Fatal("reset stream lost ordinary output")
	}
	if got := r.Write(marker) + r.Flush(); got != marker {
		t.Fatal("old stream restored a new session's private mapping")
	}
	if got := s.NewStreamRestorer().Write(marker); got != "demo@example.invalid" {
		t.Fatal("fresh stream did not use the new session")
	}
}

func TestStreamInvalidBytesDoNotRetainFollowingOutput(t *testing.T) {
	s := NewSession()
	marker := s.Process("demo@example.invalid").Transformed
	r := s.NewStreamRestorer()
	input := "\xffordinary " + marker + " end"
	if got := r.Write(input); got != "\xffordinary demo@example.invalid end" {
		t.Fatal("invalid interior byte withheld complete following output")
	}
	if len(r.buffer) != 0 {
		t.Fatal("invalid interior byte retained an unbounded buffer")
	}
	if got := r.Write("\xe2"); got != "" {
		t.Fatal("split valid rune emitted early")
	}
	if got := r.Write("X"); got != "\xe2X" {
		t.Fatal("malformed rune did not make progress")
	}
}

func FuzzStreamMatchesCompleteRestoration(f *testing.F) {
	f.Add("Á🙂", " — end", uint8(1))
	f.Add("\xff\xe2", "\xc0\x80\xe2\x82", uint8(2))
	f.Add("⟦unknown marker", "⟧", uint8(7))
	f.Fuzz(func(t *testing.T, prefix, suffix string, size uint8) {
		if len(prefix)+len(suffix) > 4096 {
			t.Skip()
		}
		s := NewSession()
		marker := s.Process("demo@example.invalid").Transformed
		input := prefix + marker + suffix
		r := s.NewStreamRestorer()
		var got strings.Builder
		step := int(size) + 1
		for start := 0; start < len(input); start += step {
			got.WriteString(r.Write(input[start:min(start+step, len(input))]))
			if len(r.buffer) > len(marker) {
				t.Fatal("buffer grew beyond a possible marker or rune suffix")
			}
		}
		got.WriteString(r.Flush())
		if got.String() != s.Restore(input) {
			t.Fatal("streamed bytes differ from complete restoration")
		}
	})
}
