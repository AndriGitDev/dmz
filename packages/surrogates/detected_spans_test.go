package surrogates

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/AndriGitDev/dmz/packages/detect"
)

func TestFinalDetectedSpansPreserveContainedEmailAndByteOffsets(t *testing.T) {
	s := NewSession()
	input := "Ása demo@example.invalid"
	got := s.ProcessBestEffort(input, []NameSpan{{0, len("Ása")}, {len("Ása "), len("Ása demo")}})
	want := []EntitySpan{{0, len("Ása"), detect.Name}, {len("Ása "), len(input), detect.Email}}
	if !got.Allowed || !got.SpansAvailable || !reflect.DeepEqual(got.DetectedSpans, want) {
		t.Fatalf("incorrect final typed spans: %#v", got)
	}
	if s.Restore(got.Transformed) != input {
		t.Fatal("span reporting changed restoration")
	}
	encoded, err := json.Marshal(got)
	if err != nil || strings.Contains(string(encoded), "Ása") || strings.Contains(string(encoded), "demo@example") {
		t.Fatal("span metadata exposed original values")
	}
}

func TestFinalDetectedSpansReflectMergedMaskRatherThanEarlierFindings(t *testing.T) {
	input := "Ása demo@example.invalid"
	s := NewSession()
	got := s.ProcessBestEffort(input, []NameSpan{{0, len("Ása de")}})
	want := []EntitySpan{{0, len(input), detect.Name}}
	if !got.SpansAvailable || !reflect.DeepEqual(got.DetectedSpans, want) || strings.Count(got.Transformed, "⟦") != 1 {
		t.Fatalf("reported stale pre-merge spans: %#v", got)
	}
	if s.Restore(got.Transformed) != input {
		t.Fatal("merged mask failed restoration")
	}
}

func TestWholeTextFallbackIsNotAnEmptyTypedDetection(t *testing.T) {
	for _, input := range []string{"pro\u0301fun", "⟦unknown marker⟧", "-----BEGIN PRIVATE KEY-----\nfabricated"} {
		got := NewSession().ProcessBestEffort(input, nil)
		if !got.Allowed || got.SpansAvailable || got.DetectedSpans != nil {
			t.Fatalf("fallback claimed typed detections: %#v", got)
		}
	}
	got := NewSession().ProcessBestEffort("Fabricated prose.", []NameSpan{{1, 999}})
	if got.Allowed || got.SpansAvailable || got.DetectedSpans != nil {
		t.Fatalf("invalid model spans claimed typed detections: %#v", got)
	}
	got = NewSession().ProcessBestEffort("Ordinary prose.", nil)
	if !got.Allowed || !got.SpansAvailable || got.DetectedSpans == nil || len(got.DetectedSpans) != 0 {
		t.Fatalf("ordinary negative is not an available empty set: %#v", got)
	}
}

func TestStrictBlockCanStillReportDetectedSecret(t *testing.T) {
	input := "sk-test_key_000000000000"
	got := NewSession().Process(input)
	if got.Allowed || !got.SpansAvailable || !reflect.DeepEqual(got.DetectedSpans, []EntitySpan{{0, len(input), detect.Secret}}) {
		t.Fatalf("strict policy and typed detection were conflated: %#v", got)
	}
}
