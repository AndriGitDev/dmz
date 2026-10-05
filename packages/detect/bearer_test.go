package detect

import (
	"strings"
	"testing"
)

func TestBearerCompleteOpaqueTokens(t *testing.T) {
	for _, value := range []string{
		"synthetic_token/with+padding==", "synthetic.token~with.parts", "synthetic_token==malformed_padding",
		"synthetic_token_abcdefghijklmnop/with+padding==", "synthetic_token_abcdefghijklmnop.with.parts",
	} {
		input := "Prófun: Bearer " + value + "; lokið."
		got := Detect(input)
		if len(got) != 1 || got[0].Kind != Secret || input[got[0].Start:got[0].End] != value {
			t.Fatalf("incomplete bearer token: %#v", got)
		}
	}
	for _, prefix := range []string{"Authorization: ", "proxy-authorization:\t", "\r\n  AUTHORIZATION : "} {
		for _, value := range []string{"a/b+c~d==", "synthetic.token.", "x"} {
			input := prefix + "bEaReR " + value + "\r\n"
			got := Detect(input)
			if len(got) != 1 || got[0].Kind != Secret || input[got[0].Start:got[0].End] != value {
				t.Fatalf("header token not fully protected: %#v", got)
			}
		}
	}
}

func TestBearerContextAndProseBoundaries(t *testing.T) {
	for _, input := range []string{
		"NotBearer synthetic_token_abcdefghijklmnop", "ÍBearer synthetic_token_abcdefghijklmnop",
		"Bearer short", "Bearer\nsynthetic_token_abcdefghijklmnop", "Authorization: Bearer ",
		"WWW-Authenticate: Bearer realm=\"synthetic\"", "the bearer carries something",
		"WWW-Authenticate: Bearer error_description=\"synthetic\"",
		"proxy-authenticate: Bearer error_description=\"synthetic\"",
	} {
		for _, got := range Detect(input) {
			if got.Kind == Secret {
				t.Fatalf("not a credential context: %q %#v", input, got)
			}
		}
	}
	value := "synthetic_token_abcdefghijklmnop"
	input := "Prófun Bearer " + value + "."
	got := Detect(input)
	if len(got) != 1 || strings.TrimSuffix(input[got[0].Start:got[0].End], ".") != value || got[0].End != len(input)-1 {
		t.Fatalf("prose punctuation convention changed: %#v", got)
	}
}
