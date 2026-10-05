package surrogates

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/AndriGitDev/dmz/packages/detect"
)

func FuzzTypedSupplementRestoration(f *testing.F) {
	f.Add("🙂 José at 18 Maple Avenue.", 13, 28, uint8(1))
	f.Add("Jose\u0301 Navarro", 0, 4, uint8(0))
	f.Add("person@example.invalid", 0, 6, uint8(2))
	f.Add("ordinary text", -1, 99, uint8(255))
	f.Fuzz(func(t *testing.T, input string, start, end int, selector uint8) {
		if len(input) > 2048 || !utf8.ValidString(input) {
			t.Skip()
		}
		kinds := []detect.Kind{detect.Name, detect.Address, detect.Phone, detect.Email,
			detect.IBAN, detect.Card, detect.Secret, detect.Kennitala, "unsupported"}
		s := NewSession()
		prior := s.ProcessBestEffort("previous@example.invalid", nil)
		mappings := len(s.reverse)
		r := s.ProcessBestEffortWithEntities(input, nil, []EntitySpan{{start, end, kinds[int(selector)%len(kinds)]}})
		if s.Restore(prior.Transformed) != "previous@example.invalid" {
			t.Fatal("supplement corrupted an existing mapping")
		}
		if r.Allowed {
			if s.Restore(r.Transformed) != input {
				t.Fatal("accepted supplemental response failed exact restoration")
			}
		} else if len(s.reverse) != mappings {
			t.Fatal("rejected supplemental response created mappings")
		}
	})
}

func TestTypedModelAddsOriginalSpansAndRestores(t *testing.T) {
	input := "Send José Navarro to 18 Maple Avenue."
	nameStart := strings.Index(input, "José")
	addressStart := strings.Index(input, "18 Maple")
	s := NewSession()
	r := s.ProcessBestEffortWithEntities(input, nil, []EntitySpan{
		{nameStart, nameStart + len("José Navarro"), detect.Name},
		{addressStart, addressStart + len("18 Maple Avenue"), detect.Address},
	})
	if !r.Allowed || !r.SpansAvailable || len(r.DetectedSpans) != 2 || s.Restore(r.Transformed) != input {
		t.Fatalf("typed masking/restoration failed: %+v", r)
	}
	for _, finding := range r.Findings {
		if finding.Reason == "local-model-entity" && finding.Confidence != detect.Uncertain {
			t.Fatal("model entity must not claim structured validation")
		}
	}
}

func TestTypedModelCannotReplaceExistingMatches(t *testing.T) {
	input := "Email: person@example.invalid; meet Emily Carter."
	a := strings.Index(input, "Emily")
	names := []NameSpan{{a, a + len("Emily Carter")}}
	expected := NewSession().ProcessBestEffort(input, names)
	for _, entities := range [][]EntitySpan{
		{{0, len(input), detect.Address}},
		{{strings.Index(input, "person"), strings.Index(input, "person") + len("person"), detect.Name}},
		{{a, a + len("Emily Carter"), detect.Address}},
	} {
		s := NewSession()
		r := s.ProcessBestEffortWithEntities(input, names, entities)
		if !r.Allowed || !reflect.DeepEqual(r.DetectedSpans, expected.DetectedSpans) || s.Restore(r.Transformed) != input {
			t.Fatalf("supplement changed protected detection: %+v", r)
		}
	}
}

func TestTypedModelRejectsInvalidResponsesWithoutMappings(t *testing.T) {
	input := "José Smith at Maple Avenue"
	cases := [][]EntitySpan{
		{{0, 4, detect.Name}}, // byte boundary inside é
		{{-1, 4, detect.Name}}, {{4, 4, detect.Name}}, {{0, len(input) + 1, detect.Name}},
		{{0, 5, detect.Kind("PERSON")}}, {{0, 5, detect.Kennitala}}, {{0, 5, detect.TenantDictionary}},
		{{0, 5, detect.UnicodeReview}}, {{0, 5, detect.Name}, {4, 10, detect.Address}},
		{{6, 11, detect.Name}, {0, 5, detect.Name}}, {{5, 6, detect.Name}},
	}
	for _, spans := range cases {
		s := NewSession()
		old := s.ProcessBestEffort("first@example.invalid", nil)
		mappings := len(s.reverse)
		r := s.ProcessBestEffortWithEntities(input, nil, spans)
		if r.Allowed || r.SpansAvailable || len(s.reverse) != mappings || s.Restore(old.Transformed) != "first@example.invalid" {
			t.Fatalf("invalid entities accepted or mutated session: %+v", spans)
		}
	}
}

func TestTypedModelGuardsCombiningSequencesAndMarkers(t *testing.T) {
	s := NewSession()
	if s.ProcessBestEffortWithEntities("Jose\u0301 Navarro", nil, []EntitySpan{{0, 4, detect.Name}}).Allowed {
		t.Fatal("split combining sequence accepted")
	}
	old := s.ProcessBestEffort("user@example.invalid", nil)
	marker := old.Transformed
	if s.ProcessBestEffortWithEntities(marker, nil, []EntitySpan{{3, len(marker) - 3, detect.Name}}).Allowed {
		t.Fatal("finding inside issued marker accepted")
	}
	plain := s.ProcessBestEffortWithEntities(marker, nil, nil)
	if !plain.Allowed || plain.Transformed != marker || s.Restore(plain.Transformed) != "user@example.invalid" {
		t.Fatal("issued marker no longer opaque")
	}
}

func TestTypedModelEmptySupplementPreservesLegacyResult(t *testing.T) {
	for _, input := range []string{"ordinary text", "a@example.invalid", "Authorization: Bearer test", "Jose\u0301 Navarro"} {
		s := NewSession()
		a := s.ProcessBestEffort(input, nil)
		b := s.ProcessBestEffortWithEntities(input, nil, nil)
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("empty supplement changed result for synthetic case: %+v / %+v", a, b)
		}
	}
}
