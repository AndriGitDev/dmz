package detect

import (
	"net/netip"
	"strings"
	"unicode"
	"unicode/utf8"
)

// emailMatches recognises mailbox-shaped spans in prose, not deliverability.
// It does not unfold headers, resolve DNS, or accept obsolete comment syntax.
func emailMatches(input string) []Match {
	var matches []Match
	assignments := emailAssignmentValues(input)
	for cursor := 0; cursor < len(input); {
		offset := strings.IndexByte(input[cursor:], '@')
		if offset < 0 {
			break
		}
		at := cursor + offset
		cursor = at + 1
		start, ok := emailLocalStart(input, at)
		if !ok {
			continue
		}
		end, ok := emailDomainEnd(input, at+1)
		if !ok {
			continue
		}
		for _, value := range assignments {
			if start < value.start && value.start < at && (value.quote == 0 || end < len(input) && input[end] == value.quote) {
				start = value.start
			}
		}
		// Paired prose/code delimiters are outside the mailbox. Internal atext
		// punctuation (including apostrophes and backticks) remains part of it.
		if end < len(input) && start < at {
			first, last := input[start], input[end]
			if (first == '\'' && last == '\'') || (first == '`' && last == '`') || (first == '{' && last == '}') {
				start++
			}
		}
		start, suppress, authority := emailURIContext(input, start, at, end)
		if suppress || start == at || !bounded(input, start, end) || (start > 0 && input[start-1] == '@') || (end < len(input) && input[end] == '@' && !authority) {
			continue
		}
		confidence, reason := High, "email-shape"
		for _, r := range input[start:end] {
			if r > unicode.MaxASCII {
				confidence, reason = Uncertain, "internationalized-email-shape"
				break
			}
		}
		matches = append(matches, Match{start, end, Email, confidence, reason})
		cursor = end
	}
	return matches
}

func emailAtomRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSymbol(r) && r > unicode.MaxASCII || strings.ContainsRune("!#$%&'*+-/=?^_`{|}~.", r)
}

func emailLocalStart(input string, at int) (int, bool) {
	if at == 0 {
		return 0, false
	}
	if input[at-1] == '"' {
		// Locate the opening quote; backslash escapes are interpreted only to
		// establish boundaries. The complete original spelling stays in the span.
		for start := at - 2; start >= 0; start-- {
			if input[start] == '\n' || input[start] == '\r' {
				return 0, false
			}
			if input[start] != '"' {
				continue
			}
			slashes := 0
			for i := start - 1; i >= 0 && input[i] == '\\'; i-- {
				slashes++
			}
			if slashes%2 != 0 {
				continue
			}
			value := input[start+1 : at-1]
			if value == "" {
				return 0, false
			}
			escaped := false
			for _, r := range value {
				if unicode.IsControl(r) {
					return 0, false
				}
				if escaped {
					escaped = false
					continue
				}
				if r == '\\' {
					escaped = true
				} else if r == '"' {
					return 0, false
				}
			}
			return start, !escaped
		}
		return 0, false
	}
	start := at
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(input[:start])
		if !emailAtomRune(r) {
			break
		}
		start -= size
	}
	local := input[start:at]
	return start, local != "" && !strings.HasPrefix(local, ".") && !strings.HasSuffix(local, ".") && !strings.Contains(local, "..")
}

func emailDomainEnd(input string, start int) (int, bool) {
	if start >= len(input) {
		return start, false
	}
	if input[start] == '[' {
		close := strings.IndexByte(input[start:], ']')
		if close < 0 {
			return start, false
		}
		end := start + close + 1
		value := input[start+1 : end-1]
		ipv6 := strings.HasPrefix(value, "IPv6:")
		if ipv6 {
			value = strings.TrimPrefix(value, "IPv6:")
		}
		address, err := netip.ParseAddr(value)
		return end, err == nil && address.Zone() == "" && (ipv6 && address.Is6() || !ipv6 && address.Is4())
	}
	end := start
	for end < len(input) {
		r, size := utf8.DecodeRuneInString(input[end:])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '.' && r != '_' {
			break
		}
		end += size
	}
	// Sentence punctuation is outside the address. Interior empty labels fail.
	for end > start && input[end-1] == '.' {
		end--
	}
	labels := strings.Split(input[start:end], ".")
	if len(labels) < 2 {
		return end, false
	}
	for _, label := range labels {
		if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") || strings.Contains(label, "_") {
			return end, false
		}
	}
	tld := labels[len(labels)-1]
	if strings.HasPrefix(strings.ToLower(tld), "xn--") {
		return end, len(tld) > 4
	}
	if utf8.RuneCountInString(tld) < 2 {
		return end, false
	}
	for _, r := range tld {
		if !unicode.IsLetter(r) {
			return end, false
		}
	}
	return end, true
}

// URI delimiters have structural meaning only within an explicit URI token.
// A bare authority username is not an email; a mailbox embedded in a more
// complex username remains eligible, as do path/query mailboxes.
func emailURIContext(input string, start, at, end int) (int, bool, bool) {
	sep := strings.LastIndex(input[:at], "://")
	if sep < 0 {
		return start, false, false
	}
	schemeStart := sep
	for schemeStart > 0 {
		c := input[schemeStart-1]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.') {
			break
		}
		schemeStart--
	}
	if schemeStart == sep {
		return start, false, false
	}
	for _, r := range input[sep+3 : at] {
		if unicode.IsSpace(r) || strings.ContainsRune("<>\"`", r) {
			return start, false, false
		}
	}
	authorityStart := sep + 3
	boundary := strings.IndexAny(input[authorityStart:at], "/?#")
	if boundary >= 0 {
		// In a URI query, keep the parameter's value; in a path keep its last
		// segment. Percent-encoded forms are not decoded or claimed as mailboxes.
		for i := authorityStart + boundary; i < at; i++ {
			if strings.ContainsRune("/?#&=", rune(input[i])) && start <= i {
				start = i + 1
			}
		}
		return start, false, false
	}
	if start < authorityStart {
		start = authorityStart
	}
	if end < len(input) && input[end] == '@' {
		return start, false, true
	}
	switch strings.ToLower(input[schemeStart:sep]) {
	case "http", "https", "ftp", "ftps", "ssh", "postgres", "postgresql", "mysql":
		return start, true, true
	}
	return start, false, true
}

// Assignment punctuation is context only at an explicit field/declaration
// boundary. A free-standing team=one mailbox remains a complete local part.
type emailAssignmentValue struct {
	start int
	quote byte
}

func emailAssignmentValues(input string) []emailAssignmentValue {
	var result []emailAssignmentValue
	for _, loc := range fieldAssignmentRE.FindAllStringSubmatchIndex(input, -1) {
		if input[loc[4]:loc[5]] != input[loc[8]:loc[9]] {
			continue
		}
		key := input[loc[6]:loc[7]]
		if len(key) > 64 {
			continue
		}
		key = strings.ReplaceAll(strings.ToLower(key), "-", "_")
		namedEmail := key == "email" || key == "email_address" || key == "emailaddress" || key == "mailbox" || strings.HasSuffix(key, "_email") || strings.HasSuffix(key, "_email_address")
		declared := loc[2] >= 0 && input[loc[2]:loc[3]] != "export"
		start := loc[1]
		if start >= len(input) {
			continue
		}
		quote := byte(0)
		if input[start] == '\'' || input[start] == '"' {
			quote = input[start]
			start++
		}
		if !namedEmail && !(declared && quote != 0) {
			continue
		}
		result = append(result, emailAssignmentValue{start, quote})
	}
	return result
}
