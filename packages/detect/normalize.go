package detect

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// normalizedDetectionText composes canonical accents without deleting them or
// folding compatibility characters. Only unambiguous original byte boundaries
// are mapped. Unresolved marks, format characters and malformed text stay guarded.
func normalizedDetectionText(input string) (string, []int, bool) {
	if !utf8.ValidString(input) || strings.IndexFunc(input, func(r rune) bool { return unicode.Is(unicode.Cf, r) }) >= 0 {
		return "", nil, false
	}
	var iter norm.Iter
	iter.InitString(norm.NFC, input)
	var out strings.Builder
	boundaries := []int{0}
	for !iter.Done() {
		start := iter.Pos()
		segment := string(iter.Next())
		end := iter.Pos()
		if end <= start || segment == "" || strings.IndexFunc(segment, func(r rune) bool {
			return unicode.IsMark(r) || unicode.Is(unicode.Cf, r)
		}) >= 0 {
			return "", nil, false
		}
		at := out.Len()
		out.WriteString(segment)
		for range len(segment) {
			boundaries = append(boundaries, -1)
		}
		boundaries[at], boundaries[out.Len()] = start, end
		if segment == input[start:end] {
			for offset := range segment {
				boundaries[at+offset] = start + offset
			}
		}
	}
	return out.String(), boundaries, true
}
