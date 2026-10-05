package detect

import (
	"strings"
	"testing"
)

func syntheticSignedURL() string {
	return "https://objects.example/report?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=fake%2F20261005%2Feu-west-1%2Fs3%2Faws4_request&X-Amz-Signature=" + strings.Repeat("a1", 32)
}

func TestPresignedURLWholeOriginalSpan(t *testing.T) {
	for _, value := range []string{
		syntheticSignedURL(),
		syntheticSignedURL() + "&opaque=a(b).,!",
		syntheticSignedURL() + "&opaque=can't",
		strings.Replace(syntheticSignedURL(), "/report?", "/mail/audit@example.test?", 1),
		strings.Replace(syntheticSignedURL(), "fake%2F", "ASIAA1B2C3D4E5F6G7H8%2F", 1),
	} {
		for _, wrapper := range [][2]string{{"", ""}, {"\"", "\""}, {"(", ")"}, {"[", "]"}, {"`", "`"}} {
			input := "Prófun 🙂: " + wrapper[0] + value + wrapper[1] + " end"
			got := Detect(input)
			if len(got) != 1 || got[0].Kind != Secret || got[0].Reason != "sigv4-presigned-url" || input[got[0].Start:got[0].End] != value {
				t.Fatalf("whole signed URL not retained: %#v", got)
			}
		}
	}
}

func TestPresignedURLShapeAndBoundaries(t *testing.T) {
	base := syntheticSignedURL()
	for _, value := range []string{
		strings.Replace(base, "https://objects.example", "https://", 1),
		strings.Replace(base, "20261005", "2026abcd", 1),
		strings.Replace(base, "X-Amz-Signature=", "signature=", 1),
		strings.Replace(base, "AWS4-HMAC-SHA256", "AWS4-ECDSA-P256-SHA256", 1),
		base + "&X-Amz-Signature=" + strings.Repeat("a1", 32),
		base + "&X-Amz-Credential=fake",
		strings.Replace(base, "X-Amz-Algorithm", "%ZZ", 1),
		strings.Replace(base, "a1a1", "a1%ZZ", 1),
		"https://objects.example/#" + strings.SplitN(base, "?", 2)[1],
	} {
		if sigV4URL(value) {
			t.Fatalf("unrecognized/ambiguous shape accepted: %q", value)
		}
	}
	for _, suffix := range []string{".", ",", ";", "!"} {
		got := Detect(base + suffix)
		if len(got) != 1 || got[0].End != len(base) {
			t.Fatalf("prose punctuation: %#v", got)
		}
	}
	if got := Detect("prefix" + base); len(got) != 0 {
		t.Fatalf("embedded scheme: %#v", got)
	}
}
