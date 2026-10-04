package detect

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// This identifies credential-shaped values without connecting to or validating
// any database. Quotes/markup delimit the URI; percent-encoded bytes stay raw.
var databaseURLRE = regexp.MustCompile("(?i:postgres(?:ql)?|mysql)://[^\\s\\p{Z}<>\"`]+")

func databaseURLHasCredential(value string) bool {
	_, rest, ok := strings.Cut(value, "://")
	if !ok {
		return false
	}
	if fragment := strings.IndexByte(rest, '#'); fragment >= 0 {
		rest = rest[:fragment]
	}
	authority := rest
	if end := strings.IndexAny(authority, "/?#"); end >= 0 {
		authority = authority[:end]
	}
	if at := strings.LastIndexByte(authority, '@'); at >= 0 {
		if _, password, ok := strings.Cut(authority[:at], ":"); ok && password != "" {
			return true
		}
	}
	_, query, ok := strings.Cut(rest, "?")
	if !ok {
		return false
	}
	for _, parameter := range strings.Split(query, "&") {
		key, value, ok := strings.Cut(parameter, "=")
		if !ok || value == "" {
			continue
		}
		key, err := url.QueryUnescape(key)
		if err != nil {
			continue
		}
		switch strings.ToLower(key) {
		case "password", "sslpassword", "passwd", "pwd", "access_token", "token", "api_key":
			return true
		}
	}
	return false
}

func databaseURLs(input string) []Match {
	var result []Match
	for _, loc := range databaseURLRE.FindAllStringIndex(input, -1) {
		start, end := loc[0], loc[1]
		// A surrounding single quote is a delimiter, not part of the URI value.
		if start > 0 && input[start-1] == '\'' {
			if quote := strings.IndexByte(input[start:end], '\''); quote >= 0 {
				end = start + quote
			}
		}
		value := input[start:end]
		// Preserve query-value punctuation: it can be part of the credential.
		// Path-only URLs retain the existing sentence punctuation convention.
		if !strings.Contains(value, "?") {
			end = start + len(strings.TrimRight(value, ".,;!"))
		}
		// Remove unmatched prose delimiters while retaining IPv6 and balanced values.
		for end > start {
			closing := input[end-1]
			opening := byte(0)
			switch closing {
			case ')':
				opening = '('
			case ']':
				opening = '['
			case '}':
				opening = '{'
			}
			if strings.Contains(input[start:end], "?") && (start == 0 || input[start-1] != opening) {
				break
			}
			if opening == 0 || strings.Count(input[start:end], string(closing)) <= strings.Count(input[start:end], string(opening)) {
				break
			}
			end--
			if strings.Contains(input[start:end], "?") {
				// Only one directly surrounding delimiter is established. More
				// closing punctuation may belong to the credential itself.
				break
			}
		}
		if bounded(input, start, end) && databaseURLHasCredential(input[start:end]) {
			result = append(result, Match{start, end, Secret, High, "database-url-credential"})
		}
	}
	return result
}

// A username@host authority is not an email address. An email inside a query
// value or inside a more complex username is not suppressed by this rule.
func databaseAuthorityEmail(input string, start, end int) bool {
	if end < len(input) {
		next, _ := utf8.DecodeRuneInString(input[end:])
		if !unicode.IsSpace(next) && !strings.ContainsRune("/?#:.,;!)]}\"'`", next) {
			return false
		}
	}
	for _, prefix := range []string{"postgres://", "postgresql://", "mysql://"} {
		if start >= len(prefix) && strings.EqualFold(input[start-len(prefix):start], prefix) && bounded(input, start-len(prefix), end) {
			return true
		}
	}
	return false
}
