package agentgateway

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// All request content below is synthetic.

func gzipBytes(t *testing.T, plain []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zstdBytes(t *testing.T, plain []byte, opts ...zstd.EOption) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := zstd.NewWriter(&buf, opts...)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// largeWindowFrame is a valid zstd frame that declares an 8 MiB window
// (window descriptor exponent 13) without a content size, holding one raw block.
func largeWindowFrame(plain []byte) []byte {
	frame := []byte{0x28, 0xb5, 0x2f, 0xfd, 0x00, 13 << 3}
	header := uint32(len(plain))<<3 | 1
	frame = append(frame, byte(header), byte(header>>8), byte(header>>16))
	return append(frame, plain...)
}

func encodeAs(t *testing.T, coding string, plain []byte) []byte {
	t.Helper()
	switch coding {
	case "gzip", "x-gzip", "GZIP":
		return gzipBytes(t, plain)
	case "zstd", "ZSTD":
		return zstdBytes(t, plain)
	}
	t.Fatal("unknown test coding")
	return nil
}

type observedRequest struct {
	body          []byte
	header        http.Header
	contentLength int64
	chunked       bool
}

type upstreamRecorder struct {
	mu   sync.Mutex
	reqs []observedRequest
}

func (rec *upstreamRecorder) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error("upstream body read failed")
		}
		rec.mu.Lock()
		rec.reqs = append(rec.reqs, observedRequest{body: body, header: r.Header.Clone(), contentLength: r.ContentLength, chunked: len(r.TransferEncoding) > 0})
		rec.mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "event: synthetic.done\ndata: {}\n\n")
	})
}

func (rec *upstreamRecorder) last(t *testing.T) observedRequest {
	t.Helper()
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.reqs) == 0 {
		t.Fatal("request did not reach upstream")
	}
	return rec.reqs[len(rec.reqs)-1]
}

func postRaw(t *testing.T, local *httptest.Server, path string, wire []byte, header http.Header) *http.Response {
	t.Helper()
	r, err := http.NewRequest("POST", local.URL+path, bytes.NewReader(wire))
	if err != nil {
		t.Fatal(err)
	}
	for k, values := range header {
		for _, v := range values {
			r.Header.Add(k, v)
		}
	}
	if r.Header.Get(TokenHeader) == "" && r.Header.Get("Authorization") == "" {
		r.Header.Set(TokenHeader, syntheticToken)
	}
	r.Header.Set("Content-Type", "application/json")
	resp, err := local.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	return resp
}

func protocolPath(p Protocol) string {
	if p == Messages {
		return "/v1/messages"
	}
	return "/v1/responses"
}

func assertNoTextInStatus(t *testing.T, s *Server, needles ...string) {
	t.Helper()
	status, _ := json.Marshal(s.Snapshot())
	for _, n := range needles {
		if bytes.Contains(status, []byte(n)) {
			t.Fatal("status exposed request content")
		}
	}
}

var digestHeaders = []string{"Content-Md5", "Digest", "Content-Digest", "Repr-Digest"}

func digestHeaderSet() http.Header {
	h := http.Header{}
	h.Set("Content-MD5", "synthetic-md5")
	h.Set("Digest", "sha-256=synthetic")
	h.Set("Content-Digest", "sha-256=:c3ludGhldGlj:")
	h.Set("Repr-Digest", "sha-256=:c3ludGhldGlj:")
	return h
}

// Changed decoded JSON is forwarded as identity JSON; untouched decoded bytes
// (spacing, opaque signatures, other fields) are preserved exactly.
func TestCompressedRequestMasking(t *testing.T) {
	bodies := map[Protocol]string{
		Responses: `{ "instructions" :"keep@example.test", "input": [{"role":"user","content":[{"type":"input_text","text":"Email pilot@example.test now"}]}, {"type":"reasoning","encrypted_content":"synthetic-opaque=="}] , "stream":true}`,
		Messages:  `{"system":"keep@example.test",  "messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"pilot@example.test","signature":"synthetic-signature/+=="}]},{"role":"user","content":[{"type":"text","text":"Email pilot@example.test now"}]}],"stream":true}`,
	}
	for _, protocol := range []Protocol{Responses, Messages} {
		for _, coding := range []string{"gzip", "x-gzip", "zstd", "ZSTD"} {
			t.Run(string(protocol)+"/"+coding, func(t *testing.T) {
				rec := &upstreamRecorder{}
				s, local := fixture(t, protocol, rec.handler(t), nil)
				plain := []byte(bodies[protocol])
				wire := encodeAs(t, coding, plain)
				h := digestHeaderSet()
				h.Set("Content-Encoding", " "+coding+" ")
				h.Set("Authorization", "Bearer synthetic-provider-key")
				h.Set(TokenHeader, syntheticToken)
				postRaw(t, local, protocolPath(protocol), wire, h)
				got := rec.last(t)
				if got.header.Get("Content-Encoding") != "" {
					t.Fatal("changed body still declared an encoding")
				}
				for _, d := range digestHeaders {
					if got.header.Get(d) != "" {
						t.Fatal("stale entity digest forwarded")
					}
				}
				if got.contentLength != int64(len(got.body)) || got.chunked {
					t.Fatal("content length not recomputed")
				}
				if got.header.Get(TokenHeader) != "" || got.header.Get("Authorization") != "Bearer synthetic-provider-key" {
					t.Fatal("credential boundary changed")
				}
				if !json.Valid(got.body) {
					t.Fatal("transformed body is not JSON")
				}
				// Exactly one occurrence (the user text) was replaced; all other
				// bytes match the decoded original.
				i := bytes.Index(plain, []byte("Email pilot@example.test now"))
				prefix, suffix := plain[:i], plain[i+len("Email pilot@example.test now"):]
				if !bytes.HasPrefix(got.body, prefix) || !bytes.HasSuffix(got.body, suffix) {
					t.Fatal("untouched decoded bytes changed")
				}
				middle := got.body[len(prefix) : len(got.body)-len(suffix)]
				if bytes.Contains(middle, []byte("pilot@example.test")) || !bytes.Contains(middle, []byte("EMAIL-")) {
					t.Fatal("supported user text was not masked")
				}
				st := s.Snapshot()
				if st.ChangedParts != 1 || st.Warnings[warnEncodedInvalid] != 0 || st.Warnings[warnDecodedOversized] != 0 || st.Warnings[warnEncodingUnsupported] != 0 {
					t.Fatal("unexpected counters")
				}
				assertNoTextInStatus(t, s, "pilot", "keep@", "synthetic-signature")
			})
		}
	}
}

// With no change, or when Transform fails open, the original encoded bytes and
// all original headers go upstream exactly.
func TestCompressedUnchangedForwardsOriginalBytes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		protocol Protocol
		plain    string
		warning  string
	}{
		{"no user text", Responses, `{"instructions":"keep@example.test","input":[]}`, WarnNoUserText},
		{"benign user text", Messages, `{"messages":[{"role":"user","content":"benign"}]}`, ""},
		{"signature only", Messages, `{"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"pilot@example.test","signature":"synthetic-signature"}]}]}`, WarnNoUserText},
		{"invalid json", Responses, `{"input":"pilot@example.test"`, WarnInvalidJSON},
		{"trailing json", Messages, `{"messages":[{"role":"user","content":"pilot@example.test"}]} {}`, WarnTrailingData},
		{"duplicate key", Responses, `{"input":"pilot@example.test","input":"x"}`, WarnDuplicateKey},
		{"empty decoded body", Responses, ``, WarnInvalidJSON},
	} {
		for _, coding := range []string{"gzip", "zstd"} {
			t.Run(tc.name+"/"+coding, func(t *testing.T) {
				rec := &upstreamRecorder{}
				s, local := fixture(t, tc.protocol, rec.handler(t), nil)
				wire := encodeAs(t, coding, []byte(tc.plain))
				h := digestHeaderSet()
				h.Set("Content-Encoding", coding)
				postRaw(t, local, protocolPath(tc.protocol), wire, h)
				got := rec.last(t)
				if !bytes.Equal(got.body, wire) || got.contentLength != int64(len(wire)) {
					t.Fatal("original encoded bytes not forwarded")
				}
				if got.header.Get("Content-Encoding") != coding {
					t.Fatal("encoding header changed")
				}
				for _, d := range digestHeaders {
					if got.header.Get(d) != h.Get(d) {
						t.Fatal("digest header changed on unchanged body")
					}
				}
				st := s.Snapshot()
				if st.ChangedParts != 0 || (tc.warning != "" && st.Warnings[tc.warning] != 1) {
					t.Fatal("unexpected counters")
				}
				assertNoTextInStatus(t, s, "pilot", "keep@", "benign")
			})
		}
	}
}

func TestCompressedFailuresForwardOriginal(t *testing.T) {
	user := []byte(`{"messages":[{"role":"user","content":"Email pilot@example.test"}]}`)
	gz := gzipBytes(t, user)
	zs := zstdBytes(t, user)
	corrupt := func(b []byte, i int) []byte { c := bytes.Clone(b); c[i] ^= 0xff; return c }
	big := bytes.Repeat([]byte(" "), 1<<20)
	bigJSON := append(append([]byte(`{"messages":[{"role":"user","content":"pilot@example.test"}]`), big...), '}')
	for _, tc := range []struct {
		name     string
		wire     []byte
		encoding []string
		adjust   func(*Config)
		warning  string
	}{
		{"gzip corrupted crc", corrupt(gz, len(gz)-6), []string{"gzip"}, nil, warnEncodedInvalid},
		{"gzip corrupted header", corrupt(gz, 0), []string{"gzip"}, nil, warnEncodedInvalid},
		{"gzip truncated", gz[:len(gz)-4], []string{"gzip"}, nil, warnEncodedInvalid},
		{"gzip truncated deflate", gz[:len(gz)/2], []string{"gzip"}, nil, warnEncodedInvalid},
		{"gzip trailing garbage", append(bytes.Clone(gz), "synthetic"...), []string{"gzip"}, nil, warnEncodedInvalid},
		{"gzip trailing zeros", append(bytes.Clone(gz), 0, 0, 0, 0), []string{"gzip"}, nil, warnEncodedInvalid},
		{"gzip empty", nil, []string{"gzip"}, nil, warnEncodedInvalid},
		{"zstd corrupted checksum", corrupt(zs, len(zs)-1), []string{"zstd"}, nil, warnEncodedInvalid},
		{"zstd corrupted magic", corrupt(zs, 0), []string{"zstd"}, nil, warnEncodedInvalid},
		{"zstd truncated", zs[:len(zs)-3], []string{"zstd"}, nil, warnEncodedInvalid},
		{"zstd trailing garbage", append(bytes.Clone(zs), "synthetic"...), []string{"zstd"}, nil, warnEncodedInvalid},
		{"gzip labelled zstd", gz, []string{"zstd"}, nil, warnEncodedInvalid},
		{"zstd labelled gzip", zs, []string{"gzip"}, nil, warnEncodedInvalid},
		{"unsupported coding", gz, []string{"br"}, nil, warnEncodingUnsupported},
		{"coding list", gz, []string{"gzip, identity"}, nil, warnEncodingUnsupported},
		{"double coding", gz, []string{"identity, gzip"}, nil, warnEncodingUnsupported},
		{"repeated fields", gz, []string{"gzip", "gzip"}, nil, warnEncodingUnsupported},
		{"gzip expansion", gzipBytes(t, bigJSON), []string{"gzip"}, func(c *Config) { c.MaxBodyBytes = 64 << 10 }, warnDecodedOversized},
		{"zstd expansion streamed", zstdBytes(t, bigJSON), []string{"zstd"}, func(c *Config) { c.MaxBodyBytes = 64 << 10 }, warnDecodedOversized},
		{"zstd expansion declared size", zstdBytes(t, bigJSON, zstd.WithEncoderLevel(zstd.SpeedFastest), zstd.WithZeroFrames(true)), []string{"zstd"}, func(c *Config) { c.MaxBodyBytes = 64 << 10 }, warnDecodedOversized},
		{"zstd large window", largeWindowFrame(user), []string{"zstd"}, func(c *Config) { c.MaxBodyBytes = 64 << 10 }, warnDecodedOversized},
		{"excess wire body", gzipBytes(t, bigJSON), []string{"gzip"}, func(c *Config) { c.MaxBodyBytes = 512 }, "oversized_body_forwarded_unchanged"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &upstreamRecorder{}
			s, local := fixture(t, Messages, rec.handler(t), tc.adjust)
			h := digestHeaderSet()
			for _, e := range tc.encoding {
				h.Add("Content-Encoding", e)
			}
			postRaw(t, local, "/v1/messages", tc.wire, h)
			got := rec.last(t)
			if !bytes.Equal(got.body, tc.wire) {
				t.Fatal("original encoded bytes not forwarded")
			}
			if strings.Join(got.header.Values("Content-Encoding"), "|") != strings.Join(tc.encoding, "|") || got.header.Get("Content-Digest") != h.Get("Content-Digest") {
				t.Fatal("headers changed on fail-open")
			}
			st := s.Snapshot()
			if st.Warnings[tc.warning] != 1 || st.ChangedParts != 0 || st.TextParts != 0 {
				t.Fatalf("unexpected warnings %v", st.Warnings)
			}
			assertNoTextInStatus(t, s, "pilot", "synthetic-md5")
		})
	}
}

func TestDecodeBodyBounds(t *testing.T) {
	plain := bytes.Repeat([]byte("a"), 4096)
	for _, coding := range []contentCoding{codingGzip, codingZstd} {
		var wire []byte
		if coding == codingGzip {
			wire = gzipBytes(t, plain)
		} else {
			wire = zstdBytes(t, plain)
		}
		if out, code := decodeBody(coding, wire, int64(len(plain))); code != "" || !bytes.Equal(out, plain) {
			t.Fatal("exact-limit body rejected")
		}
		if _, code := decodeBody(coding, wire, int64(len(plain))-1); code != warnDecodedOversized {
			t.Fatal("over-limit body accepted")
		}
	}
	// Concatenated members/frames are valid single codings.
	two := append(gzipBytes(t, []byte(`{"input":`)), gzipBytes(t, []byte(`"x"}`))...)
	if out, code := decodeBody(codingGzip, two, 64); code != "" || string(out) != `{"input":"x"}` {
		t.Fatal("multi-member gzip rejected")
	}
	twoZ := append(zstdBytes(t, []byte(`{"input":`)), zstdBytes(t, []byte(`"x"}`))...)
	if out, code := decodeBody(codingZstd, twoZ, 64); code != "" || string(out) != `{"input":"x"}` {
		t.Fatal("multi-frame zstd rejected")
	}
	if _, code := decodeBody(codingUnsupported, plain, 1<<20); code != warnEncodingUnsupported {
		t.Fatal("unsupported coding decoded")
	}
}

func TestRequestCodingParsing(t *testing.T) {
	for _, tc := range []struct {
		values []string
		want   contentCoding
	}{
		{nil, codingIdentity},
		{[]string{""}, codingIdentity},
		{[]string{" Identity "}, codingIdentity},
		{[]string{"gzip"}, codingGzip},
		{[]string{"X-GZIP"}, codingGzip},
		{[]string{"zstd"}, codingZstd},
		{[]string{"zstd, gzip"}, codingUnsupported},
		{[]string{"gzip,"}, codingUnsupported},
		{[]string{"gzip", "zstd"}, codingUnsupported},
		{[]string{"identity", "identity"}, codingUnsupported},
		{[]string{"deflate"}, codingUnsupported},
		{[]string{"br"}, codingUnsupported},
		{[]string{"compress"}, codingUnsupported},
	} {
		h := http.Header{}
		for _, v := range tc.values {
			h.Add("Content-Encoding", v)
		}
		if got := requestCoding(h); got != tc.want {
			t.Fatalf("%q: got %d", tc.values, got)
		}
	}
}

func TestCompressedSessionScopeAndReplay(t *testing.T) {
	rec := &upstreamRecorder{}
	s, local := fixture(t, Messages, rec.handler(t), func(c *Config) { c.SessionHeader = "X-Claude-Code-Session-Id" })
	plain := []byte(`{"messages":[{"role":"user","content":"pilot@example.test"}]}`)
	maskWith := func(coding, session string) []byte {
		h := http.Header{}
		h.Set("Content-Encoding", coding)
		if session != "" {
			h.Set("X-Claude-Code-Session-Id", session)
		}
		postRaw(t, local, "/v1/messages", encodeAs(t, coding, plain), h)
		return rec.last(t).body
	}
	a := maskWith("zstd", "synthetic-root-a")
	again := maskWith("gzip", "synthetic-root-a")
	b := maskWith("zstd", "synthetic-root-b")
	if !bytes.Equal(a, again) || bytes.Equal(a, b) || bytes.Contains(a, []byte("pilot@example.test")) {
		t.Fatal("compressed requests broke mapping scope")
	}
	wire := encodeAs(t, "zstd", plain)
	h := http.Header{}
	h.Set("Content-Encoding", "zstd")
	postRaw(t, local, "/v1/messages", wire, h)
	got := rec.last(t)
	if !bytes.Equal(got.body, wire) || got.header.Get("Content-Encoding") != "zstd" || s.Snapshot().Warnings["missing_session_identity_forwarded_unchanged"] != 1 {
		t.Fatal("missing identity did not forward original encoded bytes")
	}
}

func TestCompressedDesktopCredentialBoundary(t *testing.T) {
	for _, protocol := range []Protocol{Responses, Messages} {
		rec := &upstreamRecorder{}
		_, local := fixture(t, protocol, rec.handler(t), func(c *Config) { c.LocalAuth = AuthBearer; c.ProviderKey = "synthetic-provider-key" })
		plain := map[Protocol]string{Responses: `{"input":"pilot@example.test"}`, Messages: `{"messages":[{"role":"user","content":"pilot@example.test"}]}`}[protocol]
		h := http.Header{}
		h.Set("Authorization", "Bearer "+syntheticToken)
		h.Set("Content-Encoding", "zstd")
		postRaw(t, local, protocolPath(protocol), zstdBytes(t, []byte(plain)), h)
		got := rec.last(t)
		for _, values := range got.header {
			for _, v := range values {
				if strings.Contains(v, syntheticToken) {
					t.Fatal("local credential reached provider")
				}
			}
		}
		if bytes.Contains(got.body, []byte("pilot@example.test")) || got.header.Get("Content-Encoding") != "" || got.contentLength != int64(len(got.body)) {
			t.Fatal("compressed desktop request not masked as identity JSON")
		}
	}
}

func TestCancelledCompressedQueueDoesNotInvokeUpstream(t *testing.T) {
	s, err := NewServer(Config{Protocol: Responses, Upstream: "https://api.openai.com/v1", Token: syntheticToken})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < cap(s.workers); i++ {
		s.workers <- struct{}{}
	}
	s.proxy.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("cancelled draft invoked upstream transport")
		return nil, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest("POST", "/responses", bytes.NewReader(zstdBytes(t, []byte(`{"input":"pilot@example.test"}`)))).WithContext(ctx)
	r.Host = "127.0.0.1"
	r.Header.Set(TokenHeader, syntheticToken)
	r.Header.Set("Content-Encoding", "zstd")
	s.ServeHTTP(httptest.NewRecorder(), r)
	if s.Snapshot().ChangedParts != 0 {
		t.Fatal("cancelled queued draft unexpectedly transformed")
	}
}

func TestLargeWindowFrameDecodesWhenAllowed(t *testing.T) {
	// The crafted frame is otherwise valid: it decodes when the limit allows
	// its window, so the oversized failure above is the window bound.
	plain := []byte(`{"input":"x"}`)
	if out, code := decodeBody(codingZstd, largeWindowFrame(plain), 8<<20); code != "" || !bytes.Equal(out, plain) {
		t.Fatal("valid large-window frame rejected at the default limit")
	}
	if _, code := decodeBody(codingZstd, largeWindowFrame(plain), 64<<10); code != warnDecodedOversized {
		t.Fatal("large window accepted beyond the limit")
	}
}
