package detect

import (
	"regexp"
	"strings"
)

var privateKeyHeaderRE = regexp.MustCompile(`-----BEGIN ([A-Z ]*PRIVATE KEY)-----`)

// Cover complete key-shaped blocks without parsing or validating key material.
// Incomplete, nested or mismatched fences retain the header-only signal, which
// the best-effort session handles with whole-text coverage.
func privateKeyBlocks(input string) []Match {
	var result []Match
	for _, header := range privateKeyHeaderRE.FindAllStringSubmatchIndex(input, -1) {
		footer := "-----END " + input[header[2]:header[3]] + "-----"
		relativeEnd := strings.Index(input[header[1]:], footer)
		if relativeEnd < 0 {
			continue
		}
		body := input[header[1] : header[1]+relativeEnd]
		if strings.Contains(body, "-----BEGIN ") || strings.Contains(body, "-----END ") {
			continue
		}
		end := header[1] + relativeEnd + len(footer)
		if bounded(input, header[0], end) {
			result = append(result, Match{header[0], end, Secret, High, "private-key-block"})
		}
	}
	return result
}
