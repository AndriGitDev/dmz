package surrogates

import "testing"

func TestExactOriginalLookup(t *testing.T) {
	s := NewSession()
	marker := s.ProcessBestEffort("pilot@example.test", nil).Transformed
	if value, ok := s.Original(marker); !ok || value != "pilot@example.test" {
		t.Fatal("issued marker lookup failed")
	}
	if _, ok := s.Original(marker + "edited"); ok {
		t.Fatal("edited marker accepted")
	}
	s.Reset()
	if _, ok := s.Original(marker); ok {
		t.Fatal("old marker remained available after reset")
	}
}
