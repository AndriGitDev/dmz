package surrogates

import (
	"strings"
	"testing"

	"github.com/AndriGitDev/dmz/packages/detect"
)

func TestCredentialAssignmentMaskingPreservesSyntaxAndRestores(t *testing.T) {
	value := "X7a9/Q2b8+N4c6~R3d5.T1e0V=="
	for _, input := range []string{"api_key=" + value, "🙂\r\n{\"client_secret\":\"" + value + "\"}"} {
		s := NewSession()
		got := s.ProcessBestEffort(input, nil)
		if !got.Allowed || !got.SpansAvailable || len(got.DetectedSpans) != 1 {
			t.Fatalf("missing assignment span: %#v", got)
		}
		span := got.DetectedSpans[0]
		if span.Kind != detect.Secret || input[span.Start:span.End] != value || strings.Contains(got.Transformed, value) {
			t.Fatal("incomplete credential masking")
		}
		if s.Restore(got.Transformed) != input {
			t.Fatal("assignment restoration changed bytes")
		}
		if NewSession().Process(input).Allowed {
			t.Fatal("strict policy allowed assignment credential")
		}
	}
}
