package surrogates

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// These properties check byte preservation and public span contracts, not
// detector completeness. Seeds are fabricated and never sent to an issuer.
func FuzzBestEffortExactRoundTrip(f *testing.F) {
	for _, input := range []string{
		"",
		"Prófun 🙂: demo@example.invalid og 140182-8858.",
		"A\u0301sa og Ása; Pro\u0301fru\u0301n Gervido\u0301ttir.",
		"SessionToken=\"IQoJb3JpZ2luX2VjEA7b8/C9d0+E1f2G3h4I5j6==\"",
		"{\"AccessKeyId\":\"ASIAA1B2C3D4E5F6G7H8\"}",
		"(postgresql:///db?password=synthetic))",
		"Authorization: Bearer synthetic_part.+/===",
		"ghs_" + strings.Repeat("synthetic_", 12),
		"-----BEGIN PRIVATE KEY-----\r\nSYNTHETIC_NOT_A_KEY\r\n-----END PRIVATE KEY-----",
		"⟦unknown marker: demo@example.invalid⟧",
		"\"a b\"@example.test and user@[IPv6:2001:db8::1]",
		"(https://objects.example/report?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=fake%2F20261005%2Feu-west-1%2Fs3%2Faws4_request&X-Amz-Signature=" + strings.Repeat("a1", 32) + ")",
	} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) || len(input) > 16000 {
			t.Skip()
		}
		session := NewSession()
		result := session.ProcessBestEffort(input, nil)
		if !result.Allowed {
			t.Fatal("valid text rejected by best-effort processing")
		}
		if session.Restore(result.Transformed) != input {
			t.Fatal("original bytes changed during round trip")
		}
		previousEnd := 0
		for _, span := range result.DetectedSpans {
			if !result.SpansAvailable || span.Start < previousEnd || span.End <= span.Start || span.End > len(input) ||
				!utf8.ValidString(input[:span.Start]) || !utf8.ValidString(input[span.Start:span.End]) {
				t.Fatal("invalid, overlapping or unaligned final span")
			}
			previousEnd = span.End
		}
		again := session.ProcessBestEffort(result.Transformed, nil)
		if !again.Allowed || again.Transformed != result.Transformed {
			t.Fatal("issued markers are not idempotent")
		}
		repeated := session.ProcessBestEffort(input, nil)
		if !repeated.Allowed || repeated.Transformed != result.Transformed {
			t.Fatal("same input changed in a live session")
		}
		if session.Restore(repeated.Transformed) != input {
			t.Fatal("repeat changed original bytes")
		}
	})
}
