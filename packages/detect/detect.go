// Package detect identifies a small prototype set of Icelandic direct identifiers.
// It deliberately separates a check-digit match from a plausible kennitala shape:
// newly issued kennitölur have no calculated check digit since February 2026.
package detect

import (
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Kind string

const (
	Kennitala        Kind = "kennitala"
	Email            Kind = "email"
	IBAN             Kind = "iban"
	Card             Kind = "card"
	Phone            Kind = "phone"
	Secret           Kind = "secret"
	Name             Kind = "name_demo"
	TenantDictionary Kind = "tenant_dictionary"
)

type Confidence string

const (
	High      Confidence = "high"
	Uncertain Confidence = "uncertain"
)

// Match offsets are UTF-8 byte offsets. Value is never serialized; callers may
// extract it from the original string for local processing only.
type Match struct {
	Start      int        `json:"start"`
	End        int        `json:"end"`
	Kind       Kind       `json:"kind"`
	Confidence Confidence `json:"confidence"`
	Reason     string     `json:"reason"`
}

var (
	kennitalaRE = regexp.MustCompile(`[0-9]{6}-?[0-9]{4}`)
	emailRE     = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	ibanRE      = regexp.MustCompile(`(?i)IS(?:[ -]?[0-9]){24}`)
	cardRE      = regexp.MustCompile(`[0-9](?:[0-9 \-]{11,25})[0-9]`)
	phoneRE     = regexp.MustCompile(`(?:\+354[ \-]?)?[0-9]{3}[ \-]?[0-9]{4}`)
	secretRE    = regexp.MustCompile(`(?:sk-[A-Za-z0-9_\-]{12,}|AKIA[0-9A-Z]{16}|-----BEGIN [A-Z ]*PRIVATE KEY-----|(?:postgres|mysql)://[^\s]*[A-Za-z0-9/])`)
	jwtRE       = regexp.MustCompile(`eyJ[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{8,}`)
	bearerRE    = regexp.MustCompile(`(?i)Bearer[ \t]+([A-Za-z0-9_\-]{16,})`)
)

// Hand-authored demonstration forms; not a licensed lexicon or a general name detector.
var demoNames = []string{"Arna Jónsdóttir", "Arnu Jónsdóttur", "Bjarni Jónsson", "Bjarna Jónsson"}

func Detect(input string) []Match {
	return DetectWithTerms(input, nil)
}

// DetectWithTerms adds exact, case-sensitive tenant terms. Callers must bound
// and validate the terms before passing them in. No term list is persisted here.
func DetectWithTerms(input string, terms []string) []Match {
	var candidates []Match
	add := func(re *regexp.Regexp, kind Kind, validate func(string) (Confidence, string, bool)) {
		for _, loc := range re.FindAllStringIndex(input, -1) {
			if !bounded(input, loc[0], loc[1]) {
				continue
			}
			if kind == Phone && adjacentDigitAfterSeparator(input, loc[0], loc[1]) {
				continue
			}
			confidence, reason, ok := validate(input[loc[0]:loc[1]])
			if ok {
				candidates = append(candidates, Match{loc[0], loc[1], kind, confidence, reason})
			}
		}
	}
	add(secretRE, Secret, simple("secret-pattern"))
	add(jwtRE, Secret, simple("jwt-shape"))
	for _, loc := range bearerRE.FindAllStringSubmatchIndex(input, -1) {
		start, end := loc[2], loc[3]
		if bounded(input, start, end) {
			candidates = append(candidates, Match{start, end, Secret, High, "bearer-token"})
		}
	}
	add(ibanRE, IBAN, validateIBAN)
	add(emailRE, Email, simple("email-shape"))
	add(kennitalaRE, Kennitala, validateKennitala)
	add(cardRE, Card, validateCard)
	add(phoneRE, Phone, validatePhone)
	for _, form := range demoNames {
		for start := 0; start < len(input); {
			i := strings.Index(input[start:], form)
			if i < 0 {
				break
			}
			i += start
			end := i + len(form)
			if bounded(input, i, end) {
				candidates = append(candidates, Match{i, end, Name, High, "hand-authored-demo-form"})
			}
			start = end
		}
	}
	for _, term := range terms {
		if term == "" {
			continue
		}
		for start := 0; start < len(input); {
			i := strings.Index(input[start:], term)
			if i < 0 {
				break
			}
			i += start
			end := i + len(term)
			if bounded(input, i, end) {
				candidates = append(candidates, Match{i, end, TenantDictionary, High, "tenant-exact-term"})
			}
			start = end
		}
	}
	// Choose longest/highest-confidence span first; then return stable text order.
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.Start != b.Start {
			return a.Start < b.Start
		}
		if a.End != b.End {
			return a.End > b.End
		}
		return priority(a.Kind) > priority(b.Kind)
	})
	result := make([]Match, 0, len(candidates))
	for _, m := range candidates {
		if len(result) == 0 || m.Start >= result[len(result)-1].End {
			result = append(result, m)
		}
	}
	return result
}

func adjacentDigitAfterSeparator(s string, start, end int) bool {
	if start >= 2 && (s[start-1] == '-' || s[start-1] == ' ') && s[start-2] >= '0' && s[start-2] <= '9' {
		return true
	}
	if end+1 < len(s) && (s[end] == '-' || s[end] == ' ') && s[end+1] >= '0' && s[end+1] <= '9' {
		return true
	}
	return false
}

func simple(reason string) func(string) (Confidence, string, bool) {
	return func(string) (Confidence, string, bool) { return High, reason, true }
}

func priority(k Kind) int {
	switch k {
	case Secret:
		return 7
	case IBAN:
		return 6
	case Email:
		return 5
	case Kennitala:
		return 4
	case Card:
		return 3
	case Phone:
		return 2
	case TenantDictionary:
		return 1
	default:
		return 0
	}
}

func bounded(s string, start, end int) bool {
	if start > 0 {
		r, _ := utf8.DecodeLastRuneInString(s[:start])
		if identifierRune(r) {
			return false
		}
	}
	if end < len(s) {
		r, _ := utf8.DecodeRuneInString(s[end:])
		if identifierRune(r) {
			return false
		}
	}
	return true
}

func identifierRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func validateKennitala(raw string) (Confidence, string, bool) {
	d := digits(raw)
	if len(d) != 10 {
		return "", "", false
	}
	day := int(d[0]-'0')*10 + int(d[1]-'0')
	legal := day > 40
	if legal {
		day -= 40
	}
	month := int(d[2]-'0')*10 + int(d[3]-'0')
	year := int(d[4]-'0')*10 + int(d[5]-'0')
	century := map[byte]int{'8': 1800, '9': 1900, '0': 2000, '1': 2100}[d[9]]
	if century == 0 || month < 1 || month > 12 || day < 1 || day > 31 {
		return "", "", false
	}
	date := time.Date(century+year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if date.Year() != century+year || int(date.Month()) != month || date.Day() != day {
		return "", "", false
	}
	weights := [...]int{3, 2, 7, 6, 5, 4, 3, 2}
	sum := 0
	for i, w := range weights {
		sum += int(d[i]-'0') * w
	}
	check := (11 - sum%11) % 11
	if check != 10 && check == int(d[8]-'0') {
		if legal {
			return High, "legacy-entity-check", true
		}
		return High, "legacy-person-check", true
	}
	// A date-shaped number can be a post-2026 kennitala. It must not be silently
	// dismissed as invalid; the policy layer blocks or asks for confirmation.
	return Uncertain, "date-shape-no-check", true
}

func validateIBAN(raw string) (Confidence, string, bool) {
	s := strings.ToUpper(strings.NewReplacer(" ", "", "-", "").Replace(raw))
	if len(s) != 26 || !strings.HasPrefix(s, "IS") {
		return "", "", false
	}
	rearranged := s[4:] + "1828" + s[2:4]
	mod := 0
	for i := 0; i < len(rearranged); i++ {
		if rearranged[i] < '0' || rearranged[i] > '9' {
			return "", "", false
		}
		mod = (mod*10 + int(rearranged[i]-'0')) % 97
	}
	if mod != 1 {
		return "", "", false
	}
	return High, "iban-mod97", true
}

func validateCard(raw string) (Confidence, string, bool) {
	d := digits(raw)
	if len(d) < 13 || len(d) > 19 {
		return "", "", false
	}
	sum, double := 0, false
	for i := len(d) - 1; i >= 0; i-- {
		n := int(d[i] - '0')
		if double {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		double = !double
	}
	if sum%10 != 0 {
		return "", "", false
	}
	return High, "card-luhn", true
}

func validatePhone(raw string) (Confidence, string, bool) {
	d := digits(raw)
	if strings.HasPrefix(raw, "+354") && len(d) == 10 {
		return High, "country-code-shape", true
	}
	if len(d) == 7 {
		return Uncertain, "local-number-shape", true
	}
	return "", "", false
}

// Canonical returns the per-session identity key for a match.
func Canonical(kind Kind, raw string) string {
	switch kind {
	case Kennitala, Card, Phone:
		return digits(raw)
	case IBAN:
		return strings.ToUpper(strings.NewReplacer(" ", "", "-", "").Replace(raw))
	case Email:
		return strings.ToLower(raw)
	default:
		return raw
	}
}
