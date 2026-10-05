package detect

import (
	"math"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Delimiters establish a config/property assignment rather than a keyword in
// prose. Optional key quotes are checked for equality outside the regexp.
var credentialAssignmentRE = regexp.MustCompile(`(?m)(?:^|[{},;])[ \t]*(?:(export|const|let|var)[ \t]+)?(["']?)([A-Za-z][A-Za-z0-9_\-]*)(["']?)[ \t]*[:=][ \t]*`)

func credentialField(key string) bool {
	// Enforce this here instead of expanding a counted regexp repetition in
	// TinyGo's runtime regexp compiler. Consume the whole key before rejecting.
	if len(key) > 64 {
		return false
	}
	key = strings.ReplaceAll(strings.ToLower(key), "-", "_")
	for _, part := range strings.Split(key, "_") {
		if part == "public" {
			return false
		}
	}
	for _, field := range []string{
		"api_key", "access_key", "secret_key", "secret_access_key", "private_key",
		"access_token", "auth_token", "bearer_token", "refresh_token",
		"client_secret", "api_secret", "password", "passwd", "pwd", "token", "secret",
	} {
		if key == field || key == strings.ReplaceAll(field, "_", "") || strings.HasSuffix(key, "_"+field) {
			return true
		}
	}
	return false
}

func credentialEntropy(value string) float64 {
	var counts [256]int
	for i := 0; i < len(value); i++ {
		counts[value[i]]++
	}
	var entropy float64
	for _, count := range counts {
		if count > 0 {
			p := float64(count) / float64(len(value))
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

func contextualCredentialShape(value string) bool {
	if len(value) < 20 {
		return false
	}
	letter, digit := false, false
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			letter = true
		case c >= '0' && c <= '9':
			digit = true
		case strings.ContainsRune("_./+=~-", rune(c)):
		default:
			return false // Never accept only the supported prefix of a value.
		}
	}
	return letter && digit && credentialEntropy(value) >= 3.5
}

func credentialAssignments(input string) []Match {
	var result []Match
	assignments := credentialAssignmentRE.FindAllStringSubmatchIndex(input, -1)
	if len(assignments) == 0 {
		return nil
	}
	codeRanges := credentialCodeRanges(input)
	codeIndex := 0
	for _, loc := range assignments {
		if input[loc[4]:loc[5]] != input[loc[8]:loc[9]] || !credentialField(input[loc[6]:loc[7]]) {
			continue
		}
		start, end := loc[1], loc[1]
		if start == len(input) {
			continue
		}
		if input[start] == '=' {
			continue // Equality and compound operators are not assignments here.
		}
		if quote := input[start]; quote == '\'' || quote == '"' {
			start++
			end = start
			for end < len(input) && input[end] != quote && input[end] != '\r' && input[end] != '\n' {
				end++
			}
			if end == len(input) || input[end] != quote {
				continue
			}
			tail := input[end+1:]
			rest := strings.TrimLeft(tail, " \t")
			if len(rest) > 0 && !strings.ContainsRune("\r\n,;}])", rune(rest[0])) &&
				!(len(rest) < len(tail) && (strings.HasPrefix(rest, "#") || strings.HasPrefix(rest, "//"))) {
				continue // Do not call one literal part a complete computed value.
			}
		} else {
			for codeIndex < len(codeRanges) && codeRanges[codeIndex][1] <= start {
				codeIndex++
			}
			if codeIndex < len(codeRanges) && codeRanges[codeIndex][0] <= start {
				continue // In explicitly labelled program code, require a literal.
			}
			if loc[2] >= 0 && input[loc[2]:loc[3]] != "export" {
				continue // A JS binding needs a literal, not an identifier/expression.
			}
			for end < len(input) {
				r, size := utf8.DecodeRuneInString(input[end:])
				if unicode.IsSpace(r) || strings.ContainsRune(",;}])", r) {
					break
				}
				end += size
			}
		}
		if contextualCredentialShape(input[start:end]) {
			result = append(result, Match{start, end, Secret, Uncertain, "credential-assignment-entropy"})
		}
	}
	return result
}
