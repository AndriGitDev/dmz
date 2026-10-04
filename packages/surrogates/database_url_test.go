package surrogates

import (
	"github.com/AndriGitDev/dmz/packages/detect"
	"strings"
	"testing"
)

func TestDatabaseCredentialMaskingAndRestoration(t *testing.T) {
	for _, value := range []string{
		"postgresql://demo:placeholder@invalid.invalid/db",
		"mysql://invalid.invalid/db?password=synthetic)==",
		"postgres:///db?sslpassword=synthetic_value&sslmode=require",
	} {
		input := "Prófun 🙂: \"" + value + "\" lokið."
		s := NewSession()
		got := s.ProcessBestEffort(input, nil)
		if !got.Allowed || !got.SpansAvailable || len(got.DetectedSpans) != 1 || got.DetectedSpans[0].Kind != detect.Secret {
			t.Fatalf("missing credential span: %#v", got)
		}
		span := got.DetectedSpans[0]
		if input[span.Start:span.End] != value || strings.Contains(got.Transformed, "synthetic") || strings.Contains(got.Transformed, "placeholder") {
			t.Fatal("incomplete credential masking")
		}
		if s.Restore(got.Transformed) != input {
			t.Fatal("credential restoration changed bytes")
		}
		if NewSession().Process(input).Allowed {
			t.Fatal("strict policy allowed credential")
		}
	}
	for _, value := range []string{"postgres://invalid.invalid/db", "mysql://demo@invalid.invalid/db", "postgresql:///db?password="} {
		got := NewSession().ProcessBestEffort(value, nil)
		if !got.Allowed || got.Transformed != value || len(got.DetectedSpans) != 0 {
			t.Fatalf("non-secret connection altered: %#v", got)
		}
	}
}
