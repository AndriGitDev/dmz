package agentgateway

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// Fixed request-decoding warning codes. They never include request bytes or
// decoder error text.
const (
	// The request uses no supported single coding (an unknown coding, a
	// coding list or repeated Content-Encoding fields).
	warnEncodingUnsupported = "encoded_body_forwarded_unchanged"
	// The encoded body is corrupt, truncated, has trailing data, needs a
	// dictionary or fails an integrity check.
	warnEncodedInvalid = "encoded_body_invalid_forwarded_unchanged"
	// Decoding would exceed the decoded-size, window or memory limit.
	warnDecodedOversized = "decoded_body_oversized_forwarded_unchanged"
)

type contentCoding int

const (
	codingIdentity contentCoding = iota
	codingGzip
	codingZstd
	codingUnsupported
)

// requestCoding accepts exactly one supported coding in exactly one header
// field. Absent, empty or "identity" values keep the previous identity path.
func requestCoding(h http.Header) contentCoding {
	values := h.Values("Content-Encoding")
	if len(values) == 0 {
		return codingIdentity
	}
	if len(values) > 1 {
		return codingUnsupported
	}
	v := strings.TrimSpace(values[0])
	switch {
	case v == "" || strings.EqualFold(v, "identity"):
		return codingIdentity
	case strings.EqualFold(v, "gzip") || strings.EqualFold(v, "x-gzip"):
		return codingGzip
	case strings.EqualFold(v, "zstd"):
		return codingZstd
	}
	return codingUnsupported
}

// decodeBody decodes an already size-bounded wire body. Output is bounded by
// max: a stream that would produce more than max bytes is rejected without
// being decoded further. On failure it returns a fixed warning code and the
// caller forwards the original encoded bytes.
func decodeBody(coding contentCoding, wire []byte, max int64) ([]byte, string) {
	if max <= 0 {
		return nil, warnDecodedOversized
	}
	switch coding {
	case codingGzip:
		// Multi-member streams are valid gzip (RFC 1952). Bytes after the
		// last member that are not another member are rejected by the reader.
		zr, err := gzip.NewReader(bytes.NewReader(wire))
		if err != nil {
			return nil, warnEncodedInvalid
		}
		defer zr.Close()
		return readBounded(zr, max)
	case codingZstd:
		window := uint64(max)
		if window < zstd.MinWindowSize {
			window = zstd.MinWindowSize
		}
		if window > zstd.MaxWindowSize {
			window = zstd.MaxWindowSize
		}
		// Concurrency 1 decodes synchronously in the calling goroutine (no
		// background decoders). Windows and declared frame sizes beyond max
		// (or the format's 1 KiB minimum window) are rejected before
		// allocation, and readBounded still enforces max on the output. No
		// dictionaries are registered, checksums are verified and buffered
		// whole-input decoding is off.
		dec, err := zstd.NewReader(bytes.NewReader(wire),
			zstd.WithDecoderConcurrency(1),
			zstd.WithDecoderLowmem(true),
			zstd.WithDecoderMaxMemory(window),
			zstd.WithDecoderMaxWindow(window),
			zstd.WithDecodeBuffersBelow(0),
		)
		if err != nil {
			return nil, warnEncodedInvalid
		}
		defer dec.Close()
		return readBounded(dec, max)
	}
	return nil, warnEncodingUnsupported
}

func readBounded(r io.Reader, max int64) ([]byte, string) {
	out, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		if errors.Is(err, zstd.ErrDecoderSizeExceeded) || errors.Is(err, zstd.ErrWindowSizeExceeded) || errors.Is(err, zstd.ErrFrameSizeExceeded) {
			return nil, warnDecodedOversized
		}
		return nil, warnEncodedInvalid
	}
	if int64(len(out)) > max {
		return nil, warnDecodedOversized
	}
	return out, ""
}
