package detect

import (
	"strings"
	"testing"
)

func TestAWSAccessKeyContextsAndCompleteSpans(t *testing.T) {
	for _, prefix := range []string{"AKIA", "ASIA"} {
		value := prefix + "A1B2C3D4E5F6G7H8"
		input := "Prófun 🙂: " + value + "."
		got := Detect(input)
		if len(got) != 1 || got[0].Kind != Secret || input[got[0].Start:got[0].End] != value {
			t.Fatalf("provider shape: %#v", got)
		}
		for _, size := range []int{16, 19, 20, 21, 128} {
			value := prefix + strings.Repeat("aB_7", 32)[:size-4]
			for _, field := range []string{"AccessKeyId", "access_key_id", "AWS_ACCESS_KEY_ID"} {
				input := field + "=\"" + value + "\""
				got := Detect(input)
				if len(got) != 1 || got[0].Kind != Secret || input[got[0].Start:got[0].End] != value {
					t.Fatalf("complete field %s length%d: %#v", field, size, got)
				}
			}
		}
	}
}

func TestAWSFieldShapeLimitsAndPublicIdentifiers(t *testing.T) {
	for _, value := range []string{"ASIA" + strings.Repeat("A", 11), "ASIA" + strings.Repeat("A", 125), "AIDA" + strings.Repeat("A", 16), "ASIAabcd/efghijkl", "asiaABCDEFGHIJKL", "ASIAabcdéefghijkl"} {
		if contextualAWSAccessKey(value) {
			t.Fatalf("invalid contextual shape: %q", value)
		}
	}
	for _, prefix := range []string{"AIDA", "AROA", "AGPA", "AIPA", "ANPA", "ANVA", "APKA", "ASCA"} {
		if got := Detect(prefix + "A1B2C3D4E5F6G7H8"); len(got) != 0 {
			t.Fatalf("public identifier treated as credential: %#v", got)
		}
	}
	for _, value := range []string{"XASIAA1B2C3D4E5F6G7H8", "ASIAA1B2C3D4E5F6G7H8Y", "ÉASIAA1B2C3D4E5F6G7H8"} {
		if got := Detect(value); len(got) != 0 {
			t.Fatalf("embedded prefix: %#v", got)
		}
	}
}

func TestSessionTokenLiteralAndCodeReference(t *testing.T) {
	value := "IQoJb3JpZ2luX2VjEA7b8/C9d0+E1f2G3h4I5j6=="
	for _, field := range []string{"SessionToken", "sessionToken", "session_token", "AWS_SESSION_TOKEN"} {
		input := field + "=\"" + value + "\""
		got := Detect(input)
		if len(got) != 1 || got[0].Kind != Secret || input[got[0].Start:got[0].End] != value {
			t.Fatalf("missing session token: %#v", got)
		}
	}
	for _, input := range []string{"public_session_token=\"" + value + "\"", "const sessionToken=config.credentials.sessionToken12345;", "```python\nSessionToken=config.credentials.sessionToken12345\n```"} {
		if got := Detect(input); len(got) != 0 {
			t.Fatalf("public or program reference masked: %#v", got)
		}
	}
}
