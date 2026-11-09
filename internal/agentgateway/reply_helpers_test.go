package agentgateway

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// All values are synthetic. Markers follow the issued format; only mEmail
// and mName are "issued" for the test scope.
const (
	mEmail   = "⟦EMAIL-t1-1: XXXXX@XXXXX.XXX⟧"
	mName    = "⟦NAME-t1-2: XXXXX XXXXX⟧"
	mUnknown = "⟦EMAIL-t1-9: XXXXX@XXXXX.XXX⟧"
	oEmail   = "pilot@example.test"
	oName    = "Test Pilot"
)

var testRestorer = strings.NewReplacer(mEmail, oEmail, mName, oName)

func testRestore(s string) string { return testRestorer.Replace(s) }

// chunkBody yields data in the given chunk sizes, then err (default EOF).
type chunkBody struct {
	data   []byte
	sizes  []int
	err    error
	closed int
}

func (b *chunkBody) Read(p []byte) (int, error) {
	if len(b.data) == 0 {
		if b.err != nil {
			return 0, b.err
		}
		return 0, io.EOF
	}
	n := len(b.data)
	if len(b.sizes) > 0 {
		n = min(n, b.sizes[0])
		b.sizes = b.sizes[1:]
	}
	n = copy(p, b.data[:n])
	b.data = b.data[n:]
	return n, nil
}

func (b *chunkBody) Close() error { b.closed++; return nil }

type replyRun struct {
	out      []byte
	err      error
	warnings []string
	body     *chunkBody
}

func runReply(t *testing.T, proto Protocol, contentType string, in []byte, sizes []int, restore func(string) string) replyRun {
	t.Helper()
	body := &chunkBody{data: bytes.Clone(in), sizes: sizes}
	var warnings []string
	rc := RestoreReply(body, contentType, ReplyOptions{
		Protocol: proto,
		Restore:  restore,
		Warning:  func(code string) { warnings = append(warnings, code) },
	})
	out, err := io.ReadAll(rc)
	if cerr := rc.Close(); cerr != nil {
		t.Fatalf("close: %v", cerr)
	}
	if body.closed != 1 {
		t.Fatalf("upstream closed %d times", body.closed)
	}
	return replyRun{out: out, err: err, warnings: warnings, body: body}
}

// sse renders one frame with an event line and compact JSON data.
func sse(event string, v map[string]any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		panic(err)
	}
	data := strings.TrimSuffix(buf.String(), "\n")
	if event == "" {
		return "data: " + data + "\n\n"
	}
	return "event: " + event + "\ndata: " + data + "\n\n"
}

func ev(typ string, kv ...any) string {
	m := map[string]any{"type": typ}
	for i := 0; i < len(kv); i += 2 {
		m[kv[i].(string)] = kv[i+1]
	}
	return sse(typ, m)
}

type obj = map[string]any
type arr = []any

// splitFrames splits an LF-framed stream into frames, keeping terminators.
func splitFrames(b []byte) []string {
	var frames []string
	for len(b) > 0 {
		i := bytes.Index(b, []byte("\n\n"))
		if i < 0 {
			frames = append(frames, string(b))
			break
		}
		frames = append(frames, string(b[:i+2]))
		b = b[i+2:]
	}
	return frames
}

func frameJSON(t *testing.T, frame string) (string, map[string]any) {
	t.Helper()
	event, data, lines := parseSSEFrame([]byte(frame))
	if len(lines) == 0 {
		return event, nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return event, nil
	}
	return event, m
}

// textFields lists the JSON paths of fields the engine may rewrite.
var textFields = map[string][]string{
	"response.output_text.delta":  {"delta"},
	"response.output_text.done":   {"text"},
	"response.content_part.added": {"part", "text"},
	"response.content_part.done":  {"part", "text"},
	"content_block_start":         {"content_block", "text"},
	"content_block_delta":         {"delta", "text"},
}

func dropPath(m map[string]any, path []string) {
	for i, k := range path {
		if i == len(path)-1 {
			delete(m, k)
			return
		}
		next, ok := m[k].(map[string]any)
		if !ok {
			return
		}
		m = next
	}
}

// assertSameFrames checks that out has the same frames as in, in order,
// and that every changed frame differs only in its text field. With
// otherChecked, frames without a streamed text field may differ (the caller
// compares them against an exact expectation).
func assertSameFrames(t *testing.T, in, out []byte, otherChecked ...bool) {
	t.Helper()
	fi, fo := splitFrames(in), splitFrames(out)
	if len(fi) != len(fo) {
		t.Fatalf("frame count %d, want %d\nout=%q", len(fo), len(fi), out)
	}
	for i := range fi {
		if fi[i] == fo[i] {
			continue
		}
		ei, mi := frameJSON(t, fi[i])
		eo, mo := frameJSON(t, fo[i])
		if ei != eo || mi == nil || mo == nil {
			t.Fatalf("frame %d changed outside JSON text:\n in=%q\nout=%q", i, fi[i], fo[i])
		}
		typ, _ := mi["type"].(string)
		path, ok := textFields[typ]
		if !ok && len(otherChecked) > 0 && otherChecked[0] {
			continue
		}
		if !ok {
			t.Fatalf("frame %d (%s) must be byte-identical:\n in=%q\nout=%q", i, typ, fi[i], fo[i])
		}
		dropPath(mi, path)
		dropPath(mo, path)
		if !reflect.DeepEqual(mi, mo) {
			t.Fatalf("frame %d changed outside %v:\n in=%q\nout=%q", i, path, fi[i], fo[i])
		}
		// Untouched bytes before the text field must be preserved exactly.
		if !strings.HasPrefix(fo[i], fi[i][:strings.Index(fi[i], `"`+path[len(path)-1]+`":`)]) {
			t.Fatalf("frame %d prefix bytes changed:\n in=%q\nout=%q", i, fi[i], fo[i])
		}
	}
}

// channelText concatenates streamed text per channel.
func channelText(t *testing.T, b []byte) map[string]string {
	t.Helper()
	got := map[string]string{}
	for _, f := range splitFrames(b) {
		if !strings.Contains(f, "output_text.delta") && !strings.Contains(f, "content_block_") {
			continue
		}
		_, m := frameJSON(t, f)
		if m == nil {
			continue
		}
		switch m["type"] {
		case "response.output_text.delta":
			k := jsonKey(m["output_index"]) + ":" + jsonKey(m["content_index"])
			got[k] += m["delta"].(string)
		case "content_block_start":
			if cb, ok := m["content_block"].(map[string]any); ok && cb["type"] == "text" {
				got[jsonKey(m["index"])] += cb["text"].(string)
			}
		case "content_block_delta":
			if d, ok := m["delta"].(map[string]any); ok && d["type"] == "text_delta" {
				got[jsonKey(m["index"])] += d["text"].(string)
			}
		}
	}
	return got
}

func jsonKey(v any) string { b, _ := json.Marshal(v); return string(b) }

func runeCuts(s string) []int {
	var cuts []int
	for i := range s {
		cuts = append(cuts, i)
	}
	cuts = append(cuts, len(s))
	return cuts
}

func assertWarnings(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("warnings %q, want %q", got, want)
	}
}

func mustValidUTF8(t *testing.T, b []byte) {
	t.Helper()
	if !utf8.Valid(b) {
		t.Fatal("output is not valid UTF-8")
	}
}
