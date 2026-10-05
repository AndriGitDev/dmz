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
var fieldAssignmentRE = regexp.MustCompile(`(?m)(?:^|[{},;])[ \t]*(?:(export|const|let|var)[ \t]+)?(["']?)([A-Za-z][A-Za-z0-9_\-]*)(["']?)[ \t]*[:=][ \t]*`)

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
		"client_secret", "api_secret", "session_token", "password", "passwd", "pwd", "token", "secret",
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

func contextualCredentialShape(value string, quoted bool) bool {
	if len(value) < 20 || !utf8.ValidString(value) || strings.Contains(value, "{{") || strings.Contains(value, "}}") {
		return false
	}
	letter, digit := false, false
	for _, c := range value {
		switch {
		case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			letter = true
		case c >= '0' && c <= '9':
			digit = true
		case strings.ContainsRune("_./+=~-", c):
		case quoted && unicode.IsPrint(c) && !unicode.IsSpace(c) && !strings.ContainsRune("\"'\\$`", c):
			// A complete quoted literal supplies its own boundary. Keep escapes,
			// interpolation syntax and nonprinting characters outside this rule.
		default:
			return false // Never accept only the supported prefix of a value.
		}
	}
	return letter && digit && credentialEntropy(value) >= 3.5
}

func credentialAssignments(input string) []Match {
	var result []Match
	assignments := fieldAssignmentRE.FindAllStringSubmatchIndex(input, -1)
	if len(assignments) == 0 {
		return nil
	}
	codeRanges := credentialCodeRanges(input)
	codeIndex := 0
	for _, loc := range assignments {
		key := input[loc[6]:loc[7]]
		awsID := awsAccessKeyField(key)
		generic := credentialField(key)
		if input[loc[4]:loc[5]] != input[loc[8]:loc[9]] || (!awsID && !generic) {
			continue
		}
		start, end := loc[1], loc[1]
		quoted := false
		if start == len(input) {
			continue
		}
		if input[start] == '=' {
			continue // Equality and compound operators are not assignments here.
		}
		if quote := input[start]; quote == '\'' || quote == '"' {
			quoted = true
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
		if awsID && contextualAWSAccessKey(input[start:end]) {
			result = append(result, Match{start, end, Secret, Uncertain, "aws-access-key-field-shape"})
		} else if generic && contextualCredentialShape(input[start:end], quoted) {
			result = append(result, Match{start, end, Secret, Uncertain, "credential-assignment-entropy"})
		}
	}
	return result
}

// The broad AWS API length/alphabet envelope needs explicit field context.
// Bare provider detection retains the existing 20-uppercase-character shape.
func awsAccessKeyField(key string) bool {
	switch strings.ReplaceAll(strings.ToLower(key), "-", "_") {
	case "accesskeyid", "access_key_id", "aws_access_key_id":
		return true
	}
	return false
}

func contextualAWSAccessKey(value string) bool {
	if len(value) < 16 || len(value) > 128 || !(strings.HasPrefix(value, "AKIA") || strings.HasPrefix(value, "ASIA")) {
		return false
	}
	for i := 4; i < len(value); i++ {
		c := value[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}
