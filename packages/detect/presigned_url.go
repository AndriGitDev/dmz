package detect

import (
	"net/url"
	"regexp"
	"strings"
)

var presignedURLRE = regexp.MustCompile("(?i:https?)://[^\\s\\p{Z}<>\"`]+")

// Recognize a credential shape, not signature validity, issuer or expiry.
// A whole URL is sensitive even when its access-key ID has no known prefix.
func sigV4URL(value string) bool {
	u, err := url.Parse(value)
	if err != nil || u.Hostname() == "" || !(strings.EqualFold(u.Scheme, "https") || strings.EqualFold(u.Scheme, "http")) {
		return false
	}
	fields := make(map[string]string)
	for _, part := range strings.Split(u.RawQuery, "&") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		key, keyErr := url.QueryUnescape(key)
		value, valueErr := url.QueryUnescape(value)
		if keyErr != nil || valueErr != nil {
			continue
		}
		switch key {
		case "X-Amz-Algorithm", "X-Amz-Credential", "X-Amz-Signature":
			if _, duplicate := fields[key]; duplicate {
				return false // Ambiguous duplicate authentication parameters.
			}
			fields[key] = value
		}
	}
	if fields["X-Amz-Algorithm"] != "AWS4-HMAC-SHA256" {
		return false
	}
	scope := strings.Split(fields["X-Amz-Credential"], "/")
	if len(scope) != 5 || scope[0] == "" || len(scope[1]) != 8 || scope[2] == "" || scope[3] == "" || scope[4] != "aws4_request" {
		return false
	}
	for i := 0; i < len(scope[1]); i++ {
		if scope[1][i] < '0' || scope[1][i] > '9' {
			return false
		}
	}
	signature := fields["X-Amz-Signature"]
	if len(signature) != 64 {
		return false
	}
	for i := 0; i < len(signature); i++ {
		c := signature[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

func presignedURLs(input string) []Match {
	var result []Match
	for _, loc := range presignedURLRE.FindAllStringIndex(input, -1) {
		start, end := loc[0], loc[1]
		if start > 0 && input[start-1] == '\'' {
			if quote := strings.IndexByte(input[start:end], '\''); quote >= 0 {
				end = start + quote
			}
		}
		// Only remove a directly established wrapper; arbitrary query values
		// can legitimately end in parentheses or other punctuation.
		if start > 0 && end > start {
			opening, closing := input[start-1], input[end-1]
			if (opening == '(' && closing == ')' || opening == '[' && closing == ']' || opening == '{' && closing == '}') &&
				strings.Count(input[start:end], string(closing)) > strings.Count(input[start:end], string(opening)) {
				end--
			}
		}
		if !sigV4URL(input[start:end]) {
			// If the last authentication field is followed by prose punctuation,
			// remove it only when the trimmed shape becomes recognizable. Never
			// strip punctuation from an already recognized opaque query value.
			end = start + len(strings.TrimRight(input[start:end], ".,;!"))
		}
		if bounded(input, start, end) && sigV4URL(input[start:end]) {
			result = append(result, Match{start, end, Secret, High, "sigv4-presigned-url"})
		}
	}
	return result
}
