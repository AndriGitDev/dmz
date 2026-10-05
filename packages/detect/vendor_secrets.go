package detect

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

// Selected public token patterns from the MIT-licensed Gitleaks configuration.
// The generator records the pinned source and the boundary adaptation. This is
// not the Gitleaks runtime, and never verifies credentials over the network.
//
//go:embed vendor-secrets.json
var vendorSecretData []byte

type vendorSecretRule struct {
	id       string
	pattern  *regexp.Regexp
	entropy  float64
	keywords []string
	minBytes int
	maxBytes int
}

type vendorSecretAllowRule struct {
	pattern string
	first   byte
	once    sync.Once
	regex   *regexp.Regexp
}

func (r *vendorSecretAllowRule) matches(value string) bool {
	if r.first != 0 && (len(value) == 0 || value[0] != r.first) {
		return false
	}
	r.once.Do(func() { r.regex = regexp.MustCompile(r.pattern) })
	return r.regex.MatchString(value)
}

func vendorAllowFirstByte(pattern string) byte {
	// Only these exact pinned path expressions have this optimization. Every
	// branch requires '/', while selected vendor tokens start with a letter.
	// Do not infer it from a textual prefix: a later top-level alternation
	// could otherwise change the accepted language after an upstream update.
	switch pattern {
	case `^/Users/(?i)[a-z0-9]+/[\w .-/]+$`, `^/(?:bin|etc|home|opt|tmp|usr|var)/[\w ./-]+$`:
		return '/'
	}
	return 0
}

var vendorSecretRules struct {
	once      sync.Once
	rules     []vendorSecretRule
	allow     []*vendorSecretAllowRule
	stopwords []string
	repeated  bool
}

const repeatedVendorAllowPattern = `^(?i:a+|b+|c+|d+|e+|f+|g+|h+|i+|j+|k+|l+|m+|n+|o+|p+|q+|r+|s+|t+|u+|v+|w+|x+|y+|z+|\*+|\.+)$`

func loadVendorSecretRules() {
	vendorSecretRules.once.Do(func() {
		var data struct {
			Schema int `json:"schema"`
			Rules  []struct {
				ID       string   `json:"id"`
				Pattern  string   `json:"pattern"`
				Entropy  float64  `json:"entropy"`
				Keywords []string `json:"keywords"`
			} `json:"rules"`
			Allow     []string `json:"allow_regexes"`
			Stopwords []string `json:"allow_stopwords"`
		}
		if err := json.Unmarshal(vendorSecretData, &data); err != nil || data.Schema != 1 {
			panic("invalid bundled vendor secret rules")
		}
		for _, raw := range data.Rules {
			rule := vendorSecretRule{id: raw.ID, entropy: raw.Entropy, keywords: raw.Keywords}
			pattern := raw.Pattern
			if raw.ID == "pypi-upload-token" {
				// Expanding {50,1000} exhausts TinyGo's WASM regexp compiler
				// stack. Match the same ASCII run iteratively, then enforce its
				// length. Whole-token boundaries already reject longer runs.
				if pattern != `pypi-AgEIcHlwaS5vcmc[\w-]{50,1000}` {
					panic("unexpected bundled PyPI pattern")
				}
				pattern = `pypi-AgEIcHlwaS5vcmc[\w-]+`
				rule.minBytes = len("pypi-AgEIcHlwaS5vcmc") + 50
				rule.maxBytes = len("pypi-AgEIcHlwaS5vcmc") + 1000
			}
			rule.pattern = regexp.MustCompile(pattern)
			vendorSecretRules.rules = append(vendorSecretRules.rules, rule)
		}
		for _, pattern := range data.Allow {
			// This large alternation can exhaust the TinyGo regexp compiler's
			// stack in the rules-only build. Evaluate its ASCII language directly.
			if pattern == repeatedVendorAllowPattern {
				vendorSecretRules.repeated = true
				continue
			}
			vendorSecretRules.allow = append(vendorSecretRules.allow, &vendorSecretAllowRule{pattern: pattern, first: vendorAllowFirstByte(pattern)})
		}
		vendorSecretRules.stopwords = data.Stopwords
	})
}

func vendorSecretBounded(input string, start, end int) bool {
	if !bounded(input, start, end) {
		return false
	}
	// Do not accept the recognized prefix of a longer encoded token or a dotted
	// identifier. A terminal full stop remains ordinary sentence punctuation.
	if start > 0 {
		if strings.ContainsRune("-+/=", rune(input[start-1])) {
			return false
		}
		if input[start-1] == '.' && start > 1 {
			r, _ := utf8.DecodeLastRuneInString(input[:start-1])
			if identifierRune(r) {
				return false
			}
		}
	}
	if end < len(input) {
		if strings.ContainsRune("-+/=", rune(input[end])) {
			return false
		}
		if input[end] == '.' && end+1 < len(input) {
			r, _ := utf8.DecodeRuneInString(input[end+1:])
			if identifierRune(r) {
				return false
			}
		}
	}
	return true
}

func vendorSecretAllowed(value string) bool {
	if vendorSecretRules.repeated && repeatedVendorAllow(value) {
		return true
	}
	for _, pattern := range vendorSecretRules.allow {
		if pattern.matches(value) {
			return true
		}
	}
	lower := strings.ToLower(value)
	for _, word := range vendorSecretRules.stopwords {
		if strings.Contains(lower, strings.ToLower(word)) {
			return true
		}
	}
	return false
}

func repeatedVendorAllow(value string) bool {
	if len(value) == 0 {
		return false
	}
	fold := func(c byte) byte {
		if c >= 'A' && c <= 'Z' {
			return c + ('a' - 'A')
		}
		return c
	}
	first := fold(value[0])
	if (first < 'a' || first > 'z') && first != '*' && first != '.' {
		return false
	}
	for i := 1; i < len(value); i++ {
		if fold(value[i]) != first {
			return false
		}
	}
	return true
}

func vendorSecrets(input string, existing []Match) []Match {
	loadVendorSecretRules()
	lower := strings.ToLower(input)
	var result []Match
	for _, rule := range vendorSecretRules.rules {
		keyword := false
		for _, word := range rule.keywords {
			if strings.Contains(lower, strings.ToLower(word)) {
				keyword = true
				break
			}
		}
		if !keyword {
			continue
		}
		for _, loc := range rule.pattern.FindAllStringSubmatchIndex(input, -1) {
			start, end := loc[0], loc[1]
			// These selected rules use either the first nonempty capture group or
			// the entire expression; the generator rejects other rule semantics.
			for group := 2; group < len(loc); group += 2 {
				if loc[group] >= 0 && loc[group+1] > loc[group] {
					start, end = loc[group], loc[group+1]
					break
				}
			}
			if !vendorSecretBounded(input, start, end) {
				continue
			}
			value := input[start:end]
			if rule.minBytes > 0 && (len(value) < rule.minBytes || len(value) > rule.maxBytes) {
				continue
			}
			ascii := true
			for i := range value {
				if value[i] >= utf8.RuneSelf {
					ascii = false
					break
				}
			}
			if !ascii {
				continue
			}
			if (rule.entropy > 0 && credentialEntropy(value) <= rule.entropy) || vendorSecretAllowed(value) {
				continue
			}
			protected := false
			for _, old := range existing {
				if old.Kind != Secret && start < old.End && old.Start < end {
					protected = true
					break
				}
			}
			if !protected {
				result = append(result, Match{start, end, Secret, High, "vendor-secret-shape:" + rule.id})
			}
		}
	}
	return result
}
