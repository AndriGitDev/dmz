package detect

import (
	"math"
	"strings"
	"testing"
)

func TestCredentialAssignmentWholeValue(t *testing.T) {
	value := "X7a9/Q2b8+N4c6~R3d5.T1e0V=="
	for _, input := range []string{
		"api_key=" + value, "export SERVICE_TOKEN=" + value,
		"{\"clientSecret\":\"" + value + "\"}",
		"const apiKey = '" + value + "';",
		"Prófun 🙂\r\n  AWS_SECRET_ACCESS_KEY = \"" + value + "\"",
	} {
		got := Detect(input)
		if len(got) != 1 || got[0].Kind != Secret || input[got[0].Start:got[0].End] != value {
			t.Fatalf("missing complete assignment credential: %#v", got)
		}
	}
}

func TestCredentialAssignmentDoesNotMaskMetadataOrExpressions(t *testing.T) {
	value := "X7a9Q2b8N4c6R3d5T1e0V"
	for _, input := range []string{
		"PUBLIC_API_KEY=" + value, "monkey=" + value, "api_key_name=" + value,
		"const apiKey = " + value + ";", "api_key=\"${SERVICE_TOKEN}\"",
		"'api_key\"=\"" + value + "\"", "api_key=\"YOUR_API_KEY_HERE\"",
		"api_key==" + value, "api_key:=\"" + value + "\"",
		"const apiKey = \"" + value + "\" + suffix;",
	} {
		for _, m := range Detect(input) {
			if m.Kind == Secret {
				t.Fatalf("metadata or expression treated as credential: %#v", m)
			}
		}
	}
}

func TestCredentialEntropyKnownDistributions(t *testing.T) {
	for _, tc := range []struct {
		value   string
		entropy float64
	}{
		{"", 0}, {"aaaaaaaa", 0}, {"aaaabbbb", 1},
		{"aaab", 0.8112781244591328}, {"0123456789abcdef", 4},
	} {
		if got := credentialEntropy(tc.value); math.Abs(got-tc.entropy) > 1e-12 {
			t.Fatalf("incorrect empirical entropy: got %.12f want %.12f", got, tc.entropy)
		}
	}
}

func TestCredentialUnsupportedSuffixDoesNotEmitPartialSecret(t *testing.T) {
	for _, suffix := range []string{"!suffix", "#suffix", "é", "\\\"suffix"} {
		input := "api_key=\"X7a9Q2b8N4c6R3d5T1e0V" + suffix + "\""
		for _, m := range Detect(input) {
			if m.Kind == Secret {
				t.Fatalf("unsupported full value became a partial credential: %#v", m)
			}
		}
	}
}

func TestCredentialAssignmentMultipleValuesAndOriginalOffsets(t *testing.T) {
	a, b := "X7a9Q2b8N4c6R3d5T1e0V", "Z8f0W3g9M5h7S4j6U2k1Y"
	input := "🙂\n{\"api_key\":\"" + a + "\", \"client_secret\":\"" + b + "\"}"
	got := Detect(input)
	if len(got) != 2 {
		t.Fatalf("expected both independent fields: %#v", got)
	}
	for i, value := range []string{a, b} {
		if got[i].Kind != Secret || got[i].Start != strings.Index(input, value) || input[got[i].Start:got[i].End] != value {
			t.Fatalf("assignment offset changed: %#v", got[i])
		}
	}
}

func TestCredentialFieldLengthUsesWholeKey(t *testing.T) {
	for _, length := range []int{64, 65, 1000} {
		key := strings.Repeat("A", length-len("_api_key")) + "_api_key"
		got := Detect(key + "=X7a9Q2b8N4c6R3d5T1e0V")
		if (length == 64 && (len(got) != 1 || got[0].Kind != Secret)) || (length > 64 && len(got) != 0) {
			t.Fatalf("whole %d-byte key boundary not respected: %#v", length, got)
		}
	}
}
