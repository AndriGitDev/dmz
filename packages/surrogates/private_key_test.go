package surrogates

import (
	"strings"
	"testing"

	"github.com/AndriGitDev/dmz/packages/detect"
)

func TestCompletePrivateKeyBlockHasExactSpanAndRestores(t *testing.T) {
	// Deliberately nonfunctional synthetic payloads, never real keys.
	for _, label := range []string{"PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY", "ENCRYPTED PRIVATE KEY", "OPENSSH PRIVATE KEY"} {
		for _, newline := range []string{"\n", "\r\n", "\r"} {
			block := "-----BEGIN " + label + "-----" + newline + "SYNTHETIC_NOT_A_KEY" + newline + "-----END " + label + "-----"
			prefix, suffix := "Prófun 🙂: ", " lokið."
			input := prefix + block + suffix
			s := NewSession()
			got := s.ProcessBestEffort(input, nil)
			if !got.Allowed || !got.SpansAvailable || len(got.DetectedSpans) != 1 {
				t.Fatalf("missing precise key-block detection: %#v", got)
			}
			span := got.DetectedSpans[0]
			if span.Kind != detect.Secret || span.Start != len(prefix) || span.End != len(prefix)+len(block) {
				t.Fatalf("wrong span: %#v", span)
			}
			if !strings.HasPrefix(got.Transformed, prefix) || !strings.HasSuffix(got.Transformed, suffix) || strings.Contains(got.Transformed, "SYNTHETIC") {
				t.Fatal("key body exposed or surrounding text lost")
			}
			if s.Restore(got.Transformed) != input {
				t.Fatal("restoration changed original bytes")
			}
			if NewSession().Process(input).Allowed {
				t.Fatal("strict mode allowed private key")
			}
		}
	}
}

func TestIncompletePrivateKeyKeepsWholeTextFallback(t *testing.T) {
	for _, block := range []string{
		"-----BEGIN PRIVATE KEY-----\nSYNTHETIC",
		"-----BEGIN RSA PRIVATE KEY-----\nSYNTHETIC\n-----END EC PRIVATE KEY-----",
		"-----BEGIN PRIVATE KEY-----\n-----BEGIN RSA PRIVATE KEY-----\nSYNTHETIC\n-----END RSA PRIVATE KEY-----\n-----END PRIVATE KEY-----",
		"-----BEGIN PRIVATE KEY-----\nSYNTHETIC\n-----END RSA PRIVATE KEY-----\n-----END PRIVATE KEY-----",
	} {
		input := "Before " + block + " after"
		s := NewSession()
		got := s.ProcessBestEffort(input, nil)
		if !got.Allowed || got.SpansAvailable || strings.Contains(got.Transformed, "Before") || strings.Contains(got.Transformed, "SYNTHETIC") {
			t.Fatalf("unsafe partial private-key handling: %#v", got)
		}
		if s.Restore(got.Transformed) != input {
			t.Fatal("fallback restoration mismatch")
		}
	}
}

func TestSeparateKeyBlocksAndPublicMaterial(t *testing.T) {
	one := "-----BEGIN PRIVATE KEY-----\nSYNTHETIC_ONE\n-----END PRIVATE KEY-----"
	two := "-----BEGIN RSA PRIVATE KEY-----\nSYNTHETIC_TWO\n-----END RSA PRIVATE KEY-----"
	input := one + " ordinary prose " + two
	s := NewSession()
	got := s.ProcessBestEffort(input, nil)
	if !got.SpansAvailable || len(got.DetectedSpans) != 2 || !strings.Contains(got.Transformed, " ordinary prose ") || strings.Contains(got.Transformed, "SYNTHETIC") {
		t.Fatalf("separate blocks merged or exposed: %#v", got)
	}
	if s.Restore(got.Transformed) != input {
		t.Fatal("multiple block restoration mismatch")
	}
	for _, label := range []string{"PUBLIC KEY", "CERTIFICATE", "CERTIFICATE REQUEST"} {
		input := "-----BEGIN " + label + "-----\nSYNTHETIC_PUBLIC_DATA\n-----END " + label + "-----"
		got := NewSession().ProcessBestEffort(input, nil)
		if !got.Allowed || !got.SpansAvailable || len(got.DetectedSpans) != 0 || got.Transformed != input {
			t.Fatalf("public material masked as private key: %#v", got)
		}
	}
}
