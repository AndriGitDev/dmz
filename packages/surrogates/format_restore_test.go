package surrogates

import (
	"strings"
	"testing"
)

func TestFormattingVariantsRestoreExactlyWithinSession(t *testing.T) {
	groups := [][]string{
		{"140182-8858", "140182 8858", "140182\u00a08858", "1401828858"},
		{"5555 5555 5555 4444", "5555-5555-5555-4444", "5555555555554444"},
		{"IS46 0000 0000 0000 0000 0000 00", "is46" + strings.Repeat("0", 22)},
		{"demo@example.invalid", "demo@EXAMPLE.INVALID"},
		{"+354 000 0002", "+354\u202f000\u202f0002", "+3540000002"},
	}
	for _, forms := range groups {
		s := NewSession()
		input := strings.Join(forms, " og ")
		got := s.ProcessBestEffort(input, nil)
		if !got.Allowed || strings.Count(got.Transformed, "⟦") != len(forms) {
			t.Fatalf("expected all formatting variants masked: %#v", got)
		}
		if restored := s.Restore(got.Transformed); restored != input {
			t.Errorf("formatting was lost: got %q, want %q", restored, input)
		}
		stream := s.NewStreamRestorer()
		var restored strings.Builder
		for i := range len(got.Transformed) {
			restored.WriteString(stream.Write(got.Transformed[i : i+1]))
		}
		restored.WriteString(stream.Flush())
		if restored.String() != input {
			t.Errorf("streamed formatting was lost: got %q, want %q", restored.String(), input)
		}
		seen := map[string]bool{}
		for _, form := range forms {
			one := s.ProcessBestEffort(form, nil)
			if !one.Allowed || s.Restore(one.Transformed) != form || seen[one.Transformed] {
				t.Errorf("variant reused another spelling's marker: %q", form)
			}
			seen[one.Transformed] = true
			if !strings.Contains(got.Transformed, one.Transformed) {
				t.Errorf("same exact form changed its marker: %q", form)
			}
			if again := s.ProcessBestEffort(one.Transformed, nil); !again.Allowed || again.Transformed != one.Transformed {
				t.Errorf("issued marker failed resubmission: %#v", again)
			}
		}
	}
}
