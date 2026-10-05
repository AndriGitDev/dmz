package surrogates

import (
	"strings"
	"testing"

	"github.com/AndriGitDev/dmz/packages/detect"
)

func TestGitHubTokenFullMaskAndRestore(t *testing.T) {
	values := []string{"github_pat_" + strings.Repeat("aB_", 40), "gho_" + strings.Repeat("aB", 18),
		"ghs_12345_eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9." + strings.Repeat("aB", 80) + "." + strings.Repeat("cD", 420)}
	for _, value := range values {
		input := "Prófun 🙂\r\n`" + value + "`\r\nLokið."
		s := NewSession()
		got := s.ProcessBestEffort(input, nil)
		if !got.Allowed || !got.SpansAvailable || len(got.DetectedSpans) != 1 {
			t.Fatalf("missing token span: %#v", got)
		}
		span := got.DetectedSpans[0]
		if span.Kind != detect.Secret || input[span.Start:span.End] != value || strings.Contains(got.Transformed, value) {
			t.Fatal("token not completely masked")
		}
		if s.Restore(got.Transformed) != input {
			t.Fatal("token restoration changed original bytes")
		}
		if NewSession().Process(input).Allowed {
			t.Fatal("strict policy allowed token")
		}
	}
}
