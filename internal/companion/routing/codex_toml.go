package routing

import (
	"bytes"
	"math"
	"reflect"
	"sort"
	"time"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
)

// Codex settings are edited in place: every change is a span replacement on
// whole lines or on one value, so unrelated bytes, comments and formatting stay
// as they were. Semantics are always validated with the strict decoder first,
// and every edited document is decoded again and compared with the intended
// values; any mismatch is refused instead of written.

// tomlExpr is one top-level expression and the exact bytes of its line(s).
type tomlExpr struct {
	header   bool     // [table] or [[array table]]
	root     bool     // key/value before the first table header
	path     []string // header path, or table path plus the dotted key
	start    int      // first byte of the expression's first line
	first    int      // first non-blank byte: '[' or the key
	comment  int      // start of a trailing comment, or -1
	end      int      // byte after the line terminator, or len(data)
	valStart int      // key/value only: raw value span
	valEnd   int
}

type tomlDoc struct {
	data  []byte
	exprs []tomlExpr
	eol   string
}

func isBlank(c byte) bool { return c == ' ' || c == '\t' }

// scanTOML locates top-level expressions. The input must already have passed
// strict decoding; any structural surprise is reported as not editable.
func scanTOML(data []byte) (*tomlDoc, bool) {
	doc := &tomlDoc{data: data, eol: "\n"}
	if i := bytes.IndexByte(data, '\n'); i > 0 && data[i-1] == '\r' {
		doc.eol = "\r\n"
	}
	var p unstable.Parser
	p.Reset(data)
	var table []string
	headers := false
	for p.NextExpression() {
		n := p.Expression()
		if n.Kind != unstable.KeyValue && n.Kind != unstable.Table && n.Kind != unstable.ArrayTable {
			return nil, false
		}
		var keyStart, keyEnd int
		var parts []string
		for it := n.Key(); it.Next(); {
			k := it.Node()
			if len(parts) == 0 {
				keyStart = int(k.Raw.Offset)
			}
			keyEnd = int(k.Raw.Offset + k.Raw.Length)
			parts = append(parts, string(k.Data))
		}
		if len(parts) == 0 || keyEnd > len(data) {
			return nil, false
		}
		e := tomlExpr{comment: -1}
		after := keyEnd
		switch n.Kind {
		case unstable.Table, unstable.ArrayTable:
			e.header, table, headers = true, parts, true
			e.path = append([]string(nil), parts...)
		case unstable.KeyValue:
			e.root = !headers
			e.path = append(append([]string(nil), table...), parts...)
			end := int(n.Raw.Offset + n.Raw.Length)
			for end > keyEnd && isBlank(data[end-1]) {
				end--
			}
			// Arrays carry no raw range, so locate the value after '='.
			i := keyEnd
			for i < end && isBlank(data[i]) {
				i++
			}
			if i >= end || data[i] != '=' {
				return nil, false
			}
			for i++; i < end && isBlank(data[i]); i++ {
			}
			e.valStart, e.valEnd = i, end
			if e.valStart >= e.valEnd || e.valEnd > len(data) {
				return nil, false
			}
			after = end
		default:
			return nil, false
		}
		// Expressions start their own line; only indentation (and the header
		// brackets) may precede the key.
		e.start = keyStart
		for e.start > 0 && data[e.start-1] != '\n' {
			e.start--
		}
		e.first = e.start
		for e.first < keyStart && isBlank(data[e.first]) {
			e.first++
		}
		for i := e.first; i < keyStart; i++ {
			if !(e.header && (data[i] == '[' || isBlank(data[i]))) {
				return nil, false
			}
		}
		e.end = len(data)
		if i := bytes.IndexByte(data[after:], '\n'); i >= 0 {
			e.end = after + i + 1
		}
		if i := bytes.IndexByte(data[after:e.end], '#'); i >= 0 {
			e.comment = after + i
		}
		doc.exprs = append(doc.exprs, e)
	}
	if p.Error() != nil {
		return nil, false
	}
	return doc, true
}

func pathIs(path []string, want ...string) bool {
	return len(path) == len(want) && hasPathPrefix(path, want...)
}

func hasPathPrefix(path []string, prefix ...string) bool {
	if len(path) < len(prefix) {
		return false
	}
	for i := range prefix {
		if path[i] != prefix[i] {
			return false
		}
	}
	return true
}

// rootKey returns the root-level key/value with a single-part key.
func (d *tomlDoc) rootKey(name string) *tomlExpr {
	for i := range d.exprs {
		if e := &d.exprs[i]; e.root && pathIs(e.path, name) {
			return e
		}
	}
	return nil
}

// rootInsertAt is the line boundary where a new root key belongs: after the
// last root key/value, otherwise before the comment block that introduces the
// first table, otherwise at the end of the document.
func (d *tomlDoc) rootInsertAt() int {
	last, firstHeader := -1, -1
	for i, e := range d.exprs {
		if e.header {
			firstHeader = i
			break
		}
		last = i
	}
	if last >= 0 {
		return d.exprs[last].end
	}
	if firstHeader < 0 {
		return len(d.data)
	}
	at := d.exprs[firstHeader].start
	for at > 0 {
		prev := lineStart(d.data, at-1)
		if line := bytes.TrimLeft(d.data[prev:at], " \t"); len(line) == 0 || line[0] != '#' {
			break
		}
		at = prev
	}
	return at
}

func lineStart(data []byte, i int) int {
	for i > 0 && data[i-1] != '\n' {
		i--
	}
	return i
}

func blankLine(line []byte) bool { return len(bytes.Trim(line, " \t\r\n")) == 0 }

// removeExpr deletes an expression's line(s), keeping any trailing comment.
func (d *tomlDoc) removeExpr(e *tomlExpr) tomlEdit {
	if e.comment >= 0 {
		return tomlEdit{start: e.first, end: e.comment}
	}
	return tomlEdit{start: e.start, end: e.end}
}

// newLineAt returns the separator needed before inserting whole lines at a
// position: the end of a document without a final newline needs one.
func (d *tomlDoc) newLineAt(at int) string {
	if at == len(d.data) && at > 0 && d.data[at-1] != '\n' {
		return d.eol
	}
	return ""
}

type tomlEdit struct {
	start, end int
	text       string
}

// applyEdits applies non-overlapping edits. Insertions precede a replacement
// that starts at the same position and keep their given order.
func applyEdits(data []byte, edits []tomlEdit) ([]byte, bool) {
	sort.SliceStable(edits, func(i, j int) bool {
		a, b := edits[i], edits[j]
		return a.start < b.start || a.start == b.start && a.end == a.start && b.end > b.start
	})
	out := make([]byte, 0, len(data)+256)
	at := 0
	for _, e := range edits {
		if e.start < at || e.end < e.start || e.end > len(data) {
			return nil, false
		}
		out = append(out, data[at:e.start]...)
		out = append(out, e.text...)
		at = e.end
	}
	return append(out, data[at:]...), true
}

func decodeTOML(b []byte) (map[string]any, bool) {
	m := map[string]any{}
	if toml.Unmarshal(b, &m) != nil || m == nil {
		return nil, false
	}
	return m, true
}

// sameTOML compares decoded values. Unlike reflect.DeepEqual it treats NaN as
// equal to NaN and compares offset date-times by instant and offset.
func sameTOML(a, b any) bool {
	switch x := a.(type) {
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for k, v := range x {
			w, ok := y[k]
			if !ok || !sameTOML(v, w) {
				return false
			}
		}
		return true
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !sameTOML(x[i], y[i]) {
				return false
			}
		}
		return true
	case float64:
		y, ok := b.(float64)
		return ok && (x == y && math.Signbit(x) == math.Signbit(y) || math.IsNaN(x) && math.IsNaN(y))
	case time.Time:
		y, ok := b.(time.Time)
		if !ok || !x.Equal(y) {
			return false
		}
		_, xo := x.Zone()
		_, yo := y.Zone()
		return xo == yo
	default:
		return reflect.DeepEqual(a, b)
	}
}
