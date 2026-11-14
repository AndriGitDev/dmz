package agentgateway

import (
	"unicode/utf8"
)

// This file contains a small, strict JSON parser that records the raw byte
// span of every value. The transformer uses the spans to splice replacement
// strings into the original request without re-encoding the rest of the
// payload, so signatures, encrypted content, tool IDs, schemas and other
// opaque values keep their exact original bytes.
//
// The parser is deliberately stricter than encoding/json:
//   - invalid UTF-8 inside strings is rejected instead of being replaced;
//   - lone UTF-16 surrogate escapes are rejected instead of being replaced
//     with U+FFFD;
//   - duplicate object keys (compared after unescaping, so "a" and "a"
//     collide) are rejected, because different parsers resolve them
//     differently;
//   - nesting deeper than maxNestingDepth is rejected;
//   - anything other than whitespace after the root value is rejected.
//
// Every rejection is reported as a fixed code; no input bytes are ever
// copied into an error.

// maxNestingDepth bounds object/array nesting. Real client requests are far
// shallower; deeper input is passed through unchanged with a warning.
const maxNestingDepth = 256

// A byte limit alone does not bound the number of nodes in a JSON tree.
const maxJSONNodes = 100000

type jsonKind uint8

const (
	kindObject jsonKind = iota + 1
	kindArray
	kindString
	kindNumber
	kindBool
	kindNull
)

type jsonNode struct {
	kind jsonKind
	// start and end delimit the raw value bytes: b[start:end]. For strings
	// the span includes both quotes.
	start, end int
	// escaped reports whether a string contains at least one backslash.
	escaped bool
	members []jsonMember // kindObject, document order
	elems   []*jsonNode  // kindArray, document order
}

type jsonMember struct {
	key string // unescaped key
	val *jsonNode
}

// get returns the value of the member with the given unescaped key, or nil.
// Keys are unique because duplicates are rejected during parsing.
func (n *jsonNode) get(key string) *jsonNode {
	if n == nil || n.kind != kindObject {
		return nil
	}
	for i := range n.members {
		if n.members[i].key == key {
			return n.members[i].val
		}
	}
	return nil
}

type parseError uint8

const (
	parseOK parseError = iota
	parseInvalid
	parseTrailing
	parseLoneSurrogate
	parseDuplicateKey
	parseTooDeep
	parseTooComplex
)

type jsonParser struct {
	b     []byte
	i     int
	err   parseError
	nodes int
}

// parseJSON parses exactly one JSON value surrounded by optional whitespace.
func parseJSON(b []byte) (*jsonNode, parseError) {
	p := &jsonParser{b: b}
	p.skipSpace()
	root := p.value(0)
	if p.err != parseOK {
		return nil, p.err
	}
	p.skipSpace()
	if p.i != len(b) {
		return nil, parseTrailing
	}
	return root, parseOK
}

func (p *jsonParser) fail(e parseError) *jsonNode {
	if p.err == parseOK {
		p.err = e
	}
	return nil
}

func (p *jsonParser) skipSpace() {
	for p.i < len(p.b) {
		switch p.b[p.i] {
		case ' ', '\t', '\n', '\r':
			p.i++
		default:
			return
		}
	}
}

func (p *jsonParser) value(depth int) *jsonNode {
	p.nodes++
	if p.nodes > maxJSONNodes {
		return p.fail(parseTooComplex)
	}
	if p.i >= len(p.b) {
		return p.fail(parseInvalid)
	}
	switch c := p.b[p.i]; {
	case c == '{':
		return p.object(depth + 1)
	case c == '[':
		return p.array(depth + 1)
	case c == '"':
		start := p.i
		end, escaped, err := scanString(p.b, p.i, nil)
		if err != parseOK {
			return p.fail(err)
		}
		p.i = end
		return &jsonNode{kind: kindString, start: start, end: end, escaped: escaped}
	case c == 't':
		return p.literal("true", kindBool)
	case c == 'f':
		return p.literal("false", kindBool)
	case c == 'n':
		return p.literal("null", kindNull)
	case c == '-' || (c >= '0' && c <= '9'):
		return p.number()
	default:
		return p.fail(parseInvalid)
	}
}

func (p *jsonParser) literal(word string, k jsonKind) *jsonNode {
	if len(p.b)-p.i < len(word) || string(p.b[p.i:p.i+len(word)]) != word {
		return p.fail(parseInvalid)
	}
	start := p.i
	p.i += len(word)
	return &jsonNode{kind: k, start: start, end: p.i}
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func (p *jsonParser) number() *jsonNode {
	b, start := p.b, p.i
	i := p.i
	if i < len(b) && b[i] == '-' {
		i++
	}
	switch {
	case i < len(b) && b[i] == '0':
		i++
	case i < len(b) && b[i] >= '1' && b[i] <= '9':
		for i < len(b) && isDigit(b[i]) {
			i++
		}
	default:
		return p.fail(parseInvalid)
	}
	if i < len(b) && b[i] == '.' {
		i++
		if i >= len(b) || !isDigit(b[i]) {
			return p.fail(parseInvalid)
		}
		for i < len(b) && isDigit(b[i]) {
			i++
		}
	}
	if i < len(b) && (b[i] == 'e' || b[i] == 'E') {
		i++
		if i < len(b) && (b[i] == '+' || b[i] == '-') {
			i++
		}
		if i >= len(b) || !isDigit(b[i]) {
			return p.fail(parseInvalid)
		}
		for i < len(b) && isDigit(b[i]) {
			i++
		}
	}
	p.i = i
	return &jsonNode{kind: kindNumber, start: start, end: i}
}

func (p *jsonParser) object(depth int) *jsonNode {
	if depth > maxNestingDepth {
		return p.fail(parseTooDeep)
	}
	n := &jsonNode{kind: kindObject, start: p.i}
	p.i++ // '{'
	p.skipSpace()
	if p.i < len(p.b) && p.b[p.i] == '}' {
		p.i++
		n.end = p.i
		return n
	}
	var seen map[string]struct{}
	for {
		if p.i >= len(p.b) || p.b[p.i] != '"' {
			return p.fail(parseInvalid)
		}
		var buf []byte
		end, _, err := scanString(p.b, p.i, &buf)
		if err != parseOK {
			return p.fail(err)
		}
		p.i = end
		key := string(buf)
		if seen == nil {
			seen = make(map[string]struct{})
		}
		if _, dup := seen[key]; dup {
			return p.fail(parseDuplicateKey)
		}
		seen[key] = struct{}{}
		p.skipSpace()
		if p.i >= len(p.b) || p.b[p.i] != ':' {
			return p.fail(parseInvalid)
		}
		p.i++
		p.skipSpace()
		v := p.value(depth)
		if v == nil {
			return nil
		}
		n.members = append(n.members, jsonMember{key: key, val: v})
		p.skipSpace()
		if p.i >= len(p.b) {
			return p.fail(parseInvalid)
		}
		switch p.b[p.i] {
		case ',':
			p.i++
			p.skipSpace()
		case '}':
			p.i++
			n.end = p.i
			return n
		default:
			return p.fail(parseInvalid)
		}
	}
}

func (p *jsonParser) array(depth int) *jsonNode {
	if depth > maxNestingDepth {
		return p.fail(parseTooDeep)
	}
	n := &jsonNode{kind: kindArray, start: p.i}
	p.i++ // '['
	p.skipSpace()
	if p.i < len(p.b) && p.b[p.i] == ']' {
		p.i++
		n.end = p.i
		return n
	}
	for {
		v := p.value(depth)
		if v == nil {
			return nil
		}
		n.elems = append(n.elems, v)
		p.skipSpace()
		if p.i >= len(p.b) {
			return p.fail(parseInvalid)
		}
		switch p.b[p.i] {
		case ',':
			p.i++
			p.skipSpace()
		case ']':
			p.i++
			n.end = p.i
			return n
		default:
			return p.fail(parseInvalid)
		}
	}
}

func hexVal(c byte) (rune, bool) {
	switch {
	case c >= '0' && c <= '9':
		return rune(c - '0'), true
	case c >= 'a' && c <= 'f':
		return rune(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return rune(c-'A') + 10, true
	}
	return 0, false
}

// readU4 decodes the four hex digits following "\u" at b[i] == '\\'.
func readU4(b []byte, i int) (rune, bool) {
	if len(b)-i < 6 || b[i] != '\\' || b[i+1] != 'u' {
		return 0, false
	}
	var r rune
	for _, c := range b[i+2 : i+6] {
		v, ok := hexVal(c)
		if !ok {
			return 0, false
		}
		r = r<<4 | v
	}
	return r, true
}

// scanString validates the JSON string starting at b[start] == '"' and
// returns the offset just past the closing quote. When buf is non-nil the
// unescaped UTF-8 value is appended to it.
func scanString(b []byte, start int, buf *[]byte) (end int, escaped bool, err parseError) {
	i := start + 1
	for {
		if i >= len(b) {
			return 0, false, parseInvalid
		}
		c := b[i]
		switch {
		case c == '"':
			return i + 1, escaped, parseOK
		case c == '\\':
			escaped = true
			if i+1 >= len(b) {
				return 0, false, parseInvalid
			}
			var r rune
			switch b[i+1] {
			case '"':
				r = '"'
			case '\\':
				r = '\\'
			case '/':
				r = '/'
			case 'b':
				r = '\b'
			case 'f':
				r = '\f'
			case 'n':
				r = '\n'
			case 'r':
				r = '\r'
			case 't':
				r = '\t'
			case 'u':
				u, ok := readU4(b, i)
				if !ok {
					return 0, false, parseInvalid
				}
				switch {
				case u >= 0xD800 && u <= 0xDBFF:
					lo, ok := readU4(b, i+6)
					if !ok {
						// Distinguish a malformed escape from a lone surrogate.
						if len(b)-(i+6) >= 2 && b[i+6] == '\\' && b[i+7] == 'u' {
							return 0, false, parseInvalid
						}
						return 0, false, parseLoneSurrogate
					}
					if lo < 0xDC00 || lo > 0xDFFF {
						return 0, false, parseLoneSurrogate
					}
					r = 0x10000 + (u-0xD800)<<10 + (lo - 0xDC00)
					i += 6
				case u >= 0xDC00 && u <= 0xDFFF:
					return 0, false, parseLoneSurrogate
				default:
					r = u
				}
				i += 6
				if buf != nil {
					*buf = utf8.AppendRune(*buf, r)
				}
				continue
			default:
				return 0, false, parseInvalid
			}
			if buf != nil {
				*buf = append(*buf, byte(r))
			}
			i += 2
		case c < 0x20:
			return 0, false, parseInvalid
		case c < utf8.RuneSelf:
			if buf != nil {
				*buf = append(*buf, c)
			}
			i++
		default:
			r, size := utf8.DecodeRune(b[i:])
			if r == utf8.RuneError && size == 1 {
				return 0, false, parseInvalid
			}
			if buf != nil {
				*buf = append(*buf, b[i:i+size]...)
			}
			i += size
		}
	}
}

// stringValue returns the unescaped value of an already validated string
// node.
func stringValue(b []byte, n *jsonNode) string {
	if !n.escaped {
		return string(b[n.start+1 : n.end-1])
	}
	buf := make([]byte, 0, n.end-n.start)
	scanString(b, n.start, &buf)
	return string(buf)
}

// appendJSONString appends s as a JSON string literal. Non-ASCII text is
// kept as raw UTF-8; only the characters JSON requires are escaped. HTML
// characters are not escaped. s must be valid UTF-8.
func appendJSONString(dst []byte, s string) []byte {
	const hex = "0123456789abcdef"
	dst = append(dst, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			dst = append(dst, '\\', '"')
		case c == '\\':
			dst = append(dst, '\\', '\\')
		case c == '\n':
			dst = append(dst, '\\', 'n')
		case c == '\r':
			dst = append(dst, '\\', 'r')
		case c == '\t':
			dst = append(dst, '\\', 't')
		case c < 0x20:
			dst = append(dst, '\\', 'u', '0', '0', hex[c>>4], hex[c&0xF])
		default:
			dst = append(dst, c)
		}
	}
	return append(dst, '"')
}
