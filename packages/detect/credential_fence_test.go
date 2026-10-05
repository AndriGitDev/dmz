package detect

import (
	"strings"
	"testing"
)

func TestCredentialCodeFenceDistinguishesReferencesFromLiterals(t *testing.T) {
	value := "process.env.SERVICE_API_TOKEN_12345"
	for _, language := range []string{"python", "javascript", "ts", "go", "csharp", "ruby"} {
		input := "```" + language + "\napi_key = " + value + "\n```"
		if got := Detect(input); len(got) != 0 {
			t.Fatalf("code reference treated as literal in %s: %#v", language, got)
		}
		input = "```" + language + "\napi_key = \"" + value + "\"\n```"
		got := Detect(input)
		if len(got) != 1 || got[0].Kind != Secret || input[got[0].Start:got[0].End] != value {
			t.Fatalf("literal lost in %s: %#v", language, got)
		}
	}
	for _, language := range []string{"dotenv", "env", "bash", "yaml", "text", ""} {
		input := "```" + language + "\napi_key=" + value + "\n```"
		if got := Detect(input); len(got) != 1 || got[0].Kind != Secret {
			t.Fatalf("literal configuration lost in %s: %#v", language, got)
		}
	}
}

func TestCredentialFenceLengthClosureAndOffsets(t *testing.T) {
	reference := "api_key = settings.AUTHENTICATION_TOKEN_12345"
	value := "X7a9Q2b8N4c6R3d5T1e0V"
	for _, block := range []string{
		"````python\n" + reference + "\n```\n" + reference + "\n````\n",
		"   ~~~python\r\n" + reference + "\r\n```\r\n" + reference + "\r\n   ~~~~\r\n",
		"```python\n" + reference + "\n``` trailing text\n" + reference + "\n```\n",
	} {
		input := "🙂\n" + block + "api_key=" + value
		got := Detect(input)
		if len(got) != 1 || got[0].Start != strings.LastIndex(input, value) {
			t.Fatalf("incorrect fence boundary or UTF-8 offset: %#v", got)
		}
	}
	if got := Detect("```python\n" + reference); len(got) != 0 {
		t.Fatalf("unclosed code fence lost its context: %#v", got)
	}
	input := "````text\n```python\napi_key=" + value + "\n```\n````"
	if got := Detect(input); len(got) != 1 || got[0].Kind != Secret {
		t.Fatalf("nested-looking fence escaped an outer literal block: %#v", got)
	}
}

func TestCodeFenceDoesNotSuppressProviderTokenRules(t *testing.T) {
	value := "ghp_" + strings.Repeat("aB", 18)
	input := "```python\napi_key = " + value + "\n```"
	got := Detect(input)
	if len(got) != 1 || got[0].Kind != Secret || input[got[0].Start:got[0].End] != value {
		t.Fatalf("provider token lost inside code: %#v", got)
	}
}
