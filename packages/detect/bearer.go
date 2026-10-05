package detect

import (
	"regexp"
	"strings"
)

// RFC 6750 section 2.1 permits dots, tildes, pluses, slashes and trailing
// padding. Retain even misplaced padding rather than masking a credential prefix.
var bearerRE = regexp.MustCompile(`(?i)Bearer[ \t]+([A-Za-z0-9._~+/=\-]+)`)
var authorizationPrefixRE = regexp.MustCompile(`(?i)^(?:proxy-)?authorization[ \t]*:[ \t]*$`)
var challengePrefixRE = regexp.MustCompile(`(?i)^(?:www|proxy)-authenticate[ \t]*:[ \t]*$`)

func bearerTokens(input string) []Match {
	var result []Match
	for _, loc := range bearerRE.FindAllStringSubmatchIndex(input, -1) {
		start, end := loc[2], loc[3]
		if !bounded(input, loc[0], end) {
			continue
		}
		lineStart := strings.LastIndexAny(input[:loc[0]], "\r\n") + 1
		prefix := strings.TrimLeft(input[lineStart:loc[0]], " \t")
		if challengePrefixRE.MatchString(prefix) {
			continue // Response challenge parameters are not a submitted credential.
		}
		header := authorizationPrefixRE.MatchString(prefix)
		if !header {
			// Free prose retains the existing minimum length and sentence-period
			// convention. Actual header lines preserve all token punctuation and
			// accept short opaque values; token length is not proof of validity.
			end = start + len(strings.TrimRight(input[start:end], "."))
			if end-start < 16 {
				continue
			}
		}
		result = append(result, Match{start, end, Secret, High, "bearer-token"})
	}
	return result
}
