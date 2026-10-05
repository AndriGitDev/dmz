package surrogates

import (
	"strings"
	"testing"

	"github.com/AndriGitDev/dmz/packages/detect"
)

func TestBearerCompleteMaskingAndRestoration(t *testing.T) {
	for _, value := range []string{"synthetic_token/with+padding==", "a/b+c~d==", "synthetic.token.", "x"} {
		input := "Prófun 🙂\r\nAuthorization: Bearer " + value + "\r\nLokið."
		s := NewSession()
		got := s.ProcessBestEffort(input, nil)
		if !got.Allowed || !got.SpansAvailable || len(got.DetectedSpans) != 1 {
			t.Fatalf("missing full bearer span: %#v", got)
		}
		span := got.DetectedSpans[0]
		if span.Kind != detect.Secret || input[span.Start:span.End] != value || strings.Contains(got.Transformed, "Bearer "+value) {
			t.Fatal("incomplete bearer masking")
		}
		if s.Restore(got.Transformed) != input {
			t.Fatal("bearer restoration changed bytes")
		}
		if NewSession().Process(input).Allowed {
			t.Fatal("strict policy allowed bearer credential")
		}
	}
}
