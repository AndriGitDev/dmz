package detect

import (
	"strings"
	"testing"
)

func TestGitHubTokenFamiliesAndLongInstallationTokens(t *testing.T) {
	for _, prefix := range []string{"ghp_", "gho_", "ghu_", "ghs_", "ghr_", "github_pat_"} {
		for _, body := range []string{strings.Repeat("aB", 18), strings.Repeat("aB", 18) + "a", strings.Repeat("aB", 64), strings.Repeat("aB_", 40)} {
			value := prefix + body
			input := "Prófun 🙂: `" + value + "`."
			got := Detect(input)
			if len(got) != 1 || got[0].Kind != Secret || input[got[0].Start:got[0].End] != value {
				t.Fatalf("incomplete %s credential shape (%d bytes): %#v", prefix, len(value), got)
			}
		}
	}
	value := "ghs_12345_eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9." + strings.Repeat("aB", 80) + "." + strings.Repeat("cD", 420)
	input := "Prófun: " + value + "."
	got := Detect(input)
	if len(got) != 1 || got[0].Kind != Secret || input[got[0].Start:got[0].End] != value {
		t.Fatalf("stateless installation token truncated: %#v", got)
	}
}

func TestGitHubTokenContextBoundaries(t *testing.T) {
	for _, prefix := range []string{"ghp_", "gho_", "ghu_", "ghs_", "ghr_", "github_pat_"} {
		value := prefix + strings.Repeat("aB", 18)
		for _, input := range []string{
			prefix, prefix + strings.Repeat("aB", 17) + "a.",
			"x" + value, "Í" + value, value + "ö", value + "Qö",
			prefix + strings.Repeat("aB", 17) + "\naB",
		} {
			for _, m := range Detect(input) {
				if m.Kind == Secret {
					t.Fatalf("incorrect embedded, short or split token: %#v", m)
				}
			}
		}
	}
}
