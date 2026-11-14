package agentgateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

// All fixtures are synthetic. The only "sensitive" value is a fabricated
// address on the reserved example.test domain.
const syntheticEmail = "anna@example.test"
const syntheticMarker = "[EMAIL_1]"

func TestJSONNodeBudget(t *testing.T) {
	body := []byte(`{"input":[],"metadata":[` + strings.Repeat("null,", maxJSONNodes) + `null]}`)
	called := false
	out, report := Transform(Responses, body, func(s string) (string, error) { called = true; return s, nil })
	if !bytes.Equal(out, body) || called || !hasWarning(report, WarnJSONTooComplex) {
		t.Fatal("node budget did not fail open before masking")
	}
}

// emailMask replaces the synthetic email and records every input it sees.
type recorder struct{ seen []string }

func (r *recorder) mask(s string) (string, error) {
	r.seen = append(r.seen, s)
	return strings.ReplaceAll(s, syntheticEmail, syntheticMarker), nil
}

func hasWarning(rep TransformReport, code string) bool {
	return slices.Contains(rep.Warnings, code)
}

func mustValid(t *testing.T, b []byte) {
	t.Helper()
	if !json.Valid(b) {
		t.Fatalf("output is not valid JSON:\n%s", b)
	}
}

func TestResponsesInputString(t *testing.T) {
	body := []byte(`{"model":"m","input":"mail ` + syntheticEmail + ` please","stream":true}`)
	r := &recorder{}
	out, rep := Transform(Responses, body, r.mask)
	want := `{"model":"m","input":"mail [EMAIL_1] please","stream":true}`
	if string(out) != want {
		t.Fatalf("got %s", out)
	}
	if rep.TextParts != 1 || rep.ChangedParts != 1 || len(rep.Warnings) != 0 {
		t.Fatalf("report %+v", rep)
	}
}

func TestResponsesAllowlist(t *testing.T) {
	// Every non-user location contains the email; only the two user text
	// locations may change.
	body := []byte(`{
  "model": "m",
  "instructions": "sys ` + syntheticEmail + `",
  "metadata": {"note": "` + syntheticEmail + `"},
  "tools": [{"type":"function","name":"f","parameters":{"type":"object","description":"` + syntheticEmail + `"}}],
  "input": [
    {"type":"message","role":"developer","content":[{"type":"input_text","text":"dev ` + syntheticEmail + `"}]},
    {"role":"user","content":"first ` + syntheticEmail + `"},
    {"type":"message","role":"user","content":[
      {"type":"input_text","text":"second ` + syntheticEmail + `"},
      {"type":"input_image","image_url":"data:image/png;base64,QUJD","detail":"` + syntheticEmail + `"},
      {"type":"output_text","text":"odd ` + syntheticEmail + `"}
    ]},
    {"type":"message","role":"assistant","content":[{"type":"output_text","text":"asst ` + syntheticEmail + `"}]},
    {"type":"reasoning","id":"rs_1","encrypted_content":"gAAA` + syntheticEmail + `","summary":[]},
    {"type":"function_call","call_id":"call_1","name":"f","arguments":"{\"q\":\"` + syntheticEmail + `\"}"},
    {"type":"function_call_output","call_id":"call_1","output":"` + syntheticEmail + `"},
    {"type":"custom_tool_call_output","call_id":"call_2","output":"` + syntheticEmail + `"}
  ]
}`)
	r := &recorder{}
	out, rep := Transform(Responses, body, r.mask)
	mustValid(t, out)
	if rep.TextParts != 2 || rep.ChangedParts != 2 {
		t.Fatalf("report %+v", rep)
	}
	if got := strings.Count(string(out), syntheticEmail); got != strings.Count(string(body), syntheticEmail)-2 {
		t.Fatalf("unexpected number of remaining emails %d", got)
	}
	if !slices.Equal(r.seen, []string{"first " + syntheticEmail, "second " + syntheticEmail}) {
		t.Fatalf("mask order/scope %q", r.seen)
	}
	for _, code := range []string{WarnInstructionsUnchanged, WarnAttachmentUnchanged, WarnToolResultUnchanged, WarnUnsupportedContentUnchanged} {
		if !hasWarning(rep, code) {
			t.Errorf("missing warning %s in %v", code, rep.Warnings)
		}
	}
	// Byte-exact preservation outside the two edits.
	expected := strings.Replace(string(body), `"first `+syntheticEmail+`"`, `"first `+syntheticMarker+`"`, 1)
	expected = strings.Replace(expected, `"second `+syntheticEmail+`"`, `"second `+syntheticMarker+`"`, 1)
	if string(out) != expected {
		t.Fatalf("bytes outside user text changed:\n%s", out)
	}
}

func TestMessagesAllowlist(t *testing.T) {
	body := []byte(`{"model":"m","max_tokens":8,
"system":[{"type":"text","text":"sys ` + syntheticEmail + `","cache_control":{"type":"ephemeral"}}],
"tools":[{"name":"t","input_schema":{"type":"object","description":"` + syntheticEmail + `"}}],
"messages":[
 {"role":"user","content":"plain ` + syntheticEmail + `"},
 {"role":"assistant","content":[
   {"type":"thinking","thinking":"` + syntheticEmail + `","signature":"EqQBCkYIARgC+/="},
   {"type":"text","text":"asst ` + syntheticEmail + `"},
   {"type":"tool_use","id":"toolu_01","name":"t","input":{"q":"` + syntheticEmail + `"}}]},
 {"role":"user","content":[
   {"type":"tool_result","tool_use_id":"toolu_01","content":"` + syntheticEmail + `"},
   {"type":"tool_result","tool_use_id":"toolu_02","content":[{"type":"text","text":"` + syntheticEmail + `"}]},
   {"type":"image","source":{"type":"base64","media_type":"image/png","data":"QUJD"}},
   {"type":"document","source":{"type":"text","media_type":"text/plain","data":"` + syntheticEmail + `"}},
   {"type":"text","text":"block ` + syntheticEmail + `","cache_control":{"type":"ephemeral"}}]}
]}`)
	r := &recorder{}
	out, rep := Transform(Messages, body, r.mask)
	mustValid(t, out)
	if !slices.Equal(r.seen, []string{"plain " + syntheticEmail, "block " + syntheticEmail}) {
		t.Fatalf("mask scope %q", r.seen)
	}
	if rep.TextParts != 2 || rep.ChangedParts != 2 {
		t.Fatalf("report %+v", rep)
	}
	for _, code := range []string{WarnInstructionsUnchanged, WarnToolResultUnchanged, WarnAttachmentUnchanged} {
		if !hasWarning(rep, code) {
			t.Errorf("missing warning %s in %v", code, rep.Warnings)
		}
	}
	if hasWarning(rep, WarnUnsupportedContentUnchanged) {
		t.Errorf("assistant opaque blocks must not warn: %v", rep.Warnings)
	}
	expected := strings.Replace(string(body), `"plain `+syntheticEmail+`"`, `"plain `+syntheticMarker+`"`, 1)
	expected = strings.Replace(expected, `"block `+syntheticEmail+`"`, `"block `+syntheticMarker+`"`, 1)
	if string(out) != expected {
		t.Fatalf("bytes outside user text changed:\n%s", out)
	}
	// The signature keeps its original escape spelling.
	if !bytes.Contains(out, []byte(`"EqQBCkYIARgC+/="`)) {
		t.Fatal("signature bytes changed")
	}
}

func TestRoleDiscrimination(t *testing.T) {
	cases := []struct {
		name  string
		proto Protocol
		body  string
		want  int
	}{
		{"responses role case sensitive", Responses, `{"input":[{"role":"User","content":"x"}]}`, 0},
		{"responses role non-string", Responses, `{"input":[{"role":1,"content":"x"}]}`, 0},
		{"responses non-message type", Responses, `{"input":[{"type":"item_reference","role":"user","content":"x"}]}`, 0},
		{"responses system", Responses, `{"input":[{"role":"system","content":"x"}]}`, 0},
		{"messages assistant string", Messages, `{"messages":[{"role":"assistant","content":"x"}]}`, 0},
		{"messages escaped role key", Messages, `{"messages":[{"role":"user","content":"x"}]}`, 1},
		{"messages escaped role value", Messages, `{"messages":[{"role":"user","content":"x"}]}`, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &recorder{}
			_, rep := Transform(c.proto, []byte(c.body), r.mask)
			if len(r.seen) != c.want || rep.TextParts != c.want {
				t.Fatalf("seen %d parts %d want %d (%v)", len(r.seen), rep.TextParts, c.want, rep.Warnings)
			}
		})
	}
}

func TestUnchangedMaskPreservesRawBytes(t *testing.T) {
	body := []byte("{\"input\":\"caf\\u00e9 \\ud83d\\ude00 \\/ \\\"q\\\" Þór\"} \n\t")
	r := &recorder{}
	out, rep := Transform(Responses, body, r.mask)
	if !bytes.Equal(out, body) {
		t.Fatalf("identity mask changed bytes: %s", out)
	}
	if rep.TextParts != 1 || rep.ChangedParts != 0 || len(rep.Warnings) != 0 {
		t.Fatalf("report %+v", rep)
	}
	if r.seen[0] != "café 😀 / \"q\" Þór" {
		t.Fatalf("decoded %q", r.seen[0])
	}
}

func TestEscapesAndUnicodeRoundTrip(t *testing.T) {
	// Surrogate pair, escaped quotes around the email, a backslash before
	// the closing quote and non-ASCII text.
	body := []byte(`{"messages":[{"role":"user","content":"😀 \"` + syntheticEmail + `\" Jón \\"}],"x":"\\"}`)
	r := &recorder{}
	out, rep := Transform(Messages, body, r.mask)
	mustValid(t, out)
	if rep.ChangedParts != 1 {
		t.Fatalf("report %+v", rep)
	}
	var v struct {
		Messages []struct{ Content string }
		X        string
	}
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatal(err)
	}
	if v.Messages[0].Content != "😀 \""+syntheticMarker+"\" Jón \\" || v.X != "\\" {
		t.Fatalf("decoded %q %q", v.Messages[0].Content, v.X)
	}
	if !bytes.HasSuffix(out, []byte(`,"x":"\\"}`)) {
		t.Fatalf("trailing bytes changed: %s", out)
	}
}

func TestReplacementEscaping(t *testing.T) {
	mask := func(string) (string, error) { return "a\"b\\c\n\x01</>& é", nil }
	out, _ := Transform(Responses, []byte(`{"input":"x"}`), mask)
	if string(out) != `{"input":"a\"b\\c\n\u0001</>&`+" "+`é"}` {
		t.Fatalf("got %s", out)
	}
	mustValid(t, out)
}

func TestFailOpenCases(t *testing.T) {
	deep := strings.Repeat("[", maxNestingDepth+1) + strings.Repeat("]", maxNestingDepth+1)
	okDeep := strings.Repeat("[", maxNestingDepth-1) + strings.Repeat("]", maxNestingDepth-1)
	cases := []struct {
		name  string
		proto Protocol
		body  string
		code  string
	}{
		{"empty", Responses, ``, WarnInvalidJSON},
		{"truncated", Responses, `{"input":"` + syntheticEmail, WarnInvalidJSON},
		{"trailing comma", Responses, `{"input":"` + syntheticEmail + `",}`, WarnInvalidJSON},
		{"raw control", Responses, "{\"input\":\"a\nb\"}", WarnInvalidJSON},
		{"bad escape", Responses, `{"input":"\x41"}`, WarnInvalidJSON},
		{"bad unicode escape", Responses, `{"input":"\u12G4"}`, WarnInvalidJSON},
		{"invalid utf8", Responses, "{\"input\":\"\xff" + syntheticEmail + "\"}", WarnInvalidJSON},
		{"encoded surrogate utf8", Responses, "{\"input\":\"\xed\xa0\x80\"}", WarnInvalidJSON},
		{"bom", Responses, "\xef\xbb\xbf{\"input\":\"x\"}", WarnInvalidJSON},
		{"leading zero", Responses, `{"input":"x","n":01}`, WarnInvalidJSON},
		{"concatenated", Responses, `{"input":"` + syntheticEmail + `"}{"input":"x"}`, WarnTrailingData},
		{"concatenated ws", Messages, `{"messages":[]} []`, WarnTrailingData},
		{"lone high", Responses, `{"input":"` + syntheticEmail + ` \ud83d"}`, WarnLoneSurrogate},
		{"lone high then text", Responses, `{"input":"\ud83dx` + syntheticEmail + `"}`, WarnLoneSurrogate},
		{"high then high", Responses, `{"input":"\ud83d\ud83d"}`, WarnLoneSurrogate},
		{"lone low", Messages, `{"messages":[{"role":"user","content":"\ude00 ` + syntheticEmail + `"}]}`, WarnLoneSurrogate},
		{"lone surrogate in key", Messages, `{"\udc00":1,"messages":[{"role":"user","content":"` + syntheticEmail + `"}]}`, WarnLoneSurrogate},
		{"duplicate key", Responses, `{"input":"` + syntheticEmail + `","input":"x"}`, WarnDuplicateKey},
		{"escaped duplicate", Responses, `{"input":"` + syntheticEmail + `","input":"x"}`, WarnDuplicateKey},
		{"nested duplicate", Messages, `{"messages":[{"role":"user","role":"assistant","content":"` + syntheticEmail + `"}]}`, WarnDuplicateKey},
		{"deep", Responses, `{"input":"` + syntheticEmail + `","x":` + deep + `}`, WarnNestingTooDeep},
		{"root array", Responses, `["` + syntheticEmail + `"]`, WarnUnsupportedShape},
		{"input object", Responses, `{"input":{"text":"` + syntheticEmail + `"}}`, WarnUnsupportedShape},
		{"messages string", Messages, `{"messages":"` + syntheticEmail + `"}`, WarnUnsupportedShape},
		{"unknown protocol", Protocol("chat"), `{"messages":[{"role":"user","content":"` + syntheticEmail + `"}]}`, WarnUnsupportedProtocol},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &recorder{}
			out, rep := Transform(c.proto, []byte(c.body), r.mask)
			if !bytes.Equal(out, []byte(c.body)) {
				t.Fatalf("body changed: %s", out)
			}
			if len(r.seen) != 0 {
				t.Fatal("mask called on fail-open input")
			}
			if !slices.Equal(rep.Warnings, []string{c.code}) || !IsFailOpenWarning(c.code) {
				t.Fatalf("warnings %v want %s", rep.Warnings, c.code)
			}
			if rep.TextParts != 0 || rep.ChangedParts != 0 {
				t.Fatalf("report %+v", rep)
			}
			for _, w := range rep.Warnings {
				if strings.Contains(w, "example") {
					t.Fatal("warning leaks content")
				}
			}
		})
	}
	t.Run("depth at limit accepted", func(t *testing.T) {
		body := `{"input":"` + syntheticEmail + `","x":` + okDeep + `}`
		out, rep := Transform(Responses, []byte(body), (&recorder{}).mask)
		if rep.ChangedParts != 1 || bytes.Equal(out, []byte(body)) {
			t.Fatalf("report %+v", rep)
		}
	})
}

func TestTrailingWhitespaceAccepted(t *testing.T) {
	body := []byte(" \r\n{\"input\":\"" + syntheticEmail + "\"}\n\t ")
	out, rep := Transform(Responses, body, (&recorder{}).mask)
	if string(out) != " \r\n{\"input\":\"[EMAIL_1]\"}\n\t " || len(rep.Warnings) != 0 {
		t.Fatalf("got %q %+v", out, rep)
	}
}

func TestMaskFailuresKeepFieldAndContinue(t *testing.T) {
	body := []byte(`{"input":[{"role":"user","content":"a ` + syntheticEmail + `"},{"role":"user","content":"b ` + syntheticEmail + `"},{"role":"user","content":"c ` + syntheticEmail + `"},{"role":"user","content":"d ` + syntheticEmail + `"}]}`)
	secret := errors.New("detector failed on " + syntheticEmail)
	calls := 0
	mask := func(s string) (string, error) {
		calls++
		switch s[0] {
		case 'a':
			return "", secret
		case 'b':
			panic("boom " + s)
		case 'c':
			return "\xff", nil
		}
		return strings.ReplaceAll(s, syntheticEmail, syntheticMarker), nil
	}
	out, rep := Transform(Responses, body, mask)
	mustValid(t, out)
	if calls != 4 || rep.TextParts != 4 || rep.ChangedParts != 1 {
		t.Fatalf("calls %d report %+v", calls, rep)
	}
	if strings.Count(string(out), syntheticEmail) != 3 || !strings.Contains(string(out), "d "+syntheticMarker) {
		t.Fatalf("got %s", out)
	}
	if !slices.Equal(rep.Warnings, []string{WarnMaskError, WarnMaskPanic, WarnMaskInvalidOutput}) {
		t.Fatalf("warnings %v", rep.Warnings)
	}
	for _, w := range rep.Warnings {
		if IsFailOpenWarning(w) || strings.Contains(w, "example") || strings.Contains(w, "boom") {
			t.Fatalf("bad warning %q", w)
		}
	}
}

func TestNilMask(t *testing.T) {
	body := []byte(`{"input":"` + syntheticEmail + `"}`)
	out, rep := Transform(Responses, body, nil)
	if !bytes.Equal(out, body) || !hasWarning(rep, WarnMaskError) {
		t.Fatalf("got %s %+v", out, rep)
	}
}

func TestMarkerShapedTextIsStillMasked(t *testing.T) {
	// Text that already looks like a marker must still reach the mask
	// function so its own unknown-marker guard can run.
	r := &recorder{}
	Transform(Messages, []byte(`{"messages":[{"role":"user","content":"[EMAIL_9] `+syntheticEmail+`"}]}`), r.mask)
	if len(r.seen) != 1 {
		t.Fatalf("seen %q", r.seen)
	}
}

func TestNoUserTextWarnings(t *testing.T) {
	cases := []struct {
		proto Protocol
		body  string
		extra string
	}{
		{Responses, `{"model":"m"}`, ""},
		{Responses, `{"input":null,"previous_response_id":"resp_1"}`, ""},
		{Responses, `{"input":[{"type":"function_call_output","call_id":"c","output":"x"}]}`, WarnToolResultUnchanged},
		{Responses, `{"input":[{"role":"user","content":[{"type":"input_file","file_id":"file_1"}]}]}`, WarnAttachmentUnchanged},
		{Messages, `{"messages":[]}`, ""},
		{Messages, `{"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"t","content":"x"}]}]}`, WarnToolResultUnchanged},
	}
	for _, c := range cases {
		r := &recorder{}
		out, rep := Transform(c.proto, []byte(c.body), r.mask)
		if string(out) != c.body || len(r.seen) != 0 || !hasWarning(rep, WarnNoUserText) {
			t.Errorf("%s: got %s %+v", c.body, out, rep)
		}
		if c.extra != "" && !hasWarning(rep, c.extra) {
			t.Errorf("%s: missing %s in %v", c.body, c.extra, rep.Warnings)
		}
	}
	// Empty text counts as a text part, is not passed to the mask and does
	// not produce no_user_text.
	r := &recorder{}
	_, rep := Transform(Messages, []byte(`{"messages":[{"role":"user","content":""}]}`), r.mask)
	if rep.TextParts != 1 || len(r.seen) != 0 || len(rep.Warnings) != 0 {
		t.Fatalf("empty text report %+v", rep)
	}
}

func TestDocumentOrder(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"1"},{"type":"text","text":"2"}]},{"role":"user","content":"3"}],"model":"m"}`)
	r := &recorder{}
	Transform(Messages, body, r.mask)
	if !slices.Equal(r.seen, []string{"1", "2", "3"}) {
		t.Fatalf("order %q", r.seen)
	}
	// Key order inside the object does not affect selection.
	body = []byte(`{"content":"x","role":"user"}`)
	r = &recorder{}
	Transform(Messages, []byte(`{"messages":[`+string(body)+`]}`), r.mask)
	if len(r.seen) != 1 {
		t.Fatal("content before role not selected")
	}
}

func TestOpaqueResponsesItemsDoNotWarn(t *testing.T) {
	body := []byte(`{"input":[{"type":"reasoning","encrypted_content":"x"},{"type":"function_call","call_id":"c","arguments":"{}"},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"x"}]},{"role":"user","content":"hi"}]}`)
	_, rep := Transform(Responses, body, (&recorder{}).mask)
	if len(rep.Warnings) != 0 || rep.TextParts != 1 {
		t.Fatalf("report %+v", rep)
	}
}

func TestParserAgreesWithEncodingJSON(t *testing.T) {
	cases := []string{
		`0`, `-0.5e+10`, `1E5`, `true`, `null`, `[]`, `{}`, `{"a":[1,{"b":"c"}]}`,
		`"é😀\/\b\f\n\r\t"`, ` [ 1 , 2 ] `,
		`-`, `1.`, `.5`, `1e`, `+1`, `[1,]`, `{"a"}`, `{"a":1,}`, `tru`, `nul`, `"\`, `[`, `{"a":1 "b":2}`,
	}
	for _, c := range cases {
		_, perr := parseJSON([]byte(c))
		if (perr == parseOK) != json.Valid([]byte(c)) {
			t.Errorf("%q: parse %v json.Valid %v", c, perr, json.Valid([]byte(c)))
		}
	}
}

func FuzzTransform(f *testing.F) {
	seeds := []string{
		`{"input":"x ` + syntheticEmail + `"}`,
		`{"input":[{"role":"user","content":[{"type":"input_text","text":"😀 ` + syntheticEmail + `"}]}]}`,
		`{"messages":[{"role":"user","content":"\"` + syntheticEmail + `\\"}]}`,
		`{"messages":[{"role":"user","content":"\ud83d"}]}`,
		`{"input":"a","input":"b"}`,
		`{"input":"a"} {}`,
	}
	for _, s := range seeds {
		f.Add([]byte(s), true)
	}
	upper := func(s string) (string, error) { return strings.ToUpper(s) + "\"\\", nil }
	f.Fuzz(func(t *testing.T, body []byte, msgs bool) {
		proto := Responses
		if msgs {
			proto = Messages
		}
		identity := func(s string) (string, error) { return s, nil }
		out, rep := Transform(proto, body, identity)
		if !bytes.Equal(out, body) {
			t.Fatal("identity mask changed bytes")
		}
		failOpen := slices.ContainsFunc(rep.Warnings, IsFailOpenWarning)
		if !failOpen && !json.Valid(body) {
			t.Fatal("accepted input that encoding/json rejects")
		}
		out, rep = Transform(proto, body, upper)
		if slices.ContainsFunc(rep.Warnings, IsFailOpenWarning) {
			if !bytes.Equal(out, body) {
				t.Fatal("fail-open changed bytes")
			}
			return
		}
		if !json.Valid(out) {
			t.Fatalf("invalid output %q", out)
		}
		if _, perr := parseJSON(out); perr != parseOK {
			t.Fatalf("output rejected by own parser: %v", perr)
		}
		if rep.ChangedParts > rep.TextParts {
			t.Fatal("changed > text parts")
		}
	})
}
