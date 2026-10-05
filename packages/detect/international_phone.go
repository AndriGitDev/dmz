package detect

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
)

// Numbering rules are generated from Google libphonenumber v9.0.40 through
// nyaruka/phonenumbers/v2 v2.0.14. Only the build-time tool needs protobuf.
//
//go:embed phone-plans.json
var phonePlanData []byte

type phoneDescription struct {
	Pattern string `json:"pattern"`
	Lengths []int  `json:"lengths"`
}
type phonePlan struct {
	Leading string             `json:"leading"`
	General phoneDescription   `json:"general"`
	Types   []phoneDescription `json:"types"`
}
type compiledPhoneDescription struct {
	pattern *regexp.Regexp
	lengths []int
}

func (d compiledPhoneDescription) matches(number string) bool {
	if len(d.lengths) > 0 {
		found := false
		for _, length := range d.lengths {
			if len(number) == length {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return d.pattern.MatchString(number)
}
func compilePhoneDescription(d phoneDescription) compiledPhoneDescription {
	return compiledPhoneDescription{regexp.MustCompile("^(?:" + d.Pattern + ")$"), d.Lengths}
}

type compiledPhonePlan struct {
	leading *regexp.Regexp
	general compiledPhoneDescription
	types   []compiledPhoneDescription
}

func (p compiledPhonePlan) valid(number string) bool {
	if !p.general.matches(number) {
		return false
	}
	for _, kind := range p.types {
		if kind.matches(number) {
			return true
		}
	}
	return false
}

var phonePlans struct {
	once     sync.Once
	mu       sync.Mutex
	source   map[string][]phonePlan
	compiled map[string][]compiledPhonePlan
}

func plansForCallingCode(code string) []compiledPhonePlan {
	phonePlans.once.Do(func() {
		var data struct {
			Schema int                    `json:"schema"`
			Plans  map[string][]phonePlan `json:"plans"`
		}
		if err := json.Unmarshal(phonePlanData, &data); err != nil || data.Schema != 1 {
			panic("invalid bundled phone plan data")
		}
		phonePlans.source = data.Plans
		phonePlans.compiled = make(map[string][]compiledPhonePlan)
	})
	phonePlans.mu.Lock()
	defer phonePlans.mu.Unlock()
	if p, ok := phonePlans.compiled[code]; ok {
		return p
	}
	var result []compiledPhonePlan
	for _, raw := range phonePlans.source[code] {
		p := compiledPhonePlan{general: compilePhoneDescription(raw.General)}
		if raw.Leading != "" {
			p.leading = regexp.MustCompile("^(?:" + raw.Leading + ")")
		}
		for _, description := range raw.Types {
			p.types = append(p.types, compilePhoneDescription(description))
		}
		result = append(result, p)
	}
	phonePlans.compiled[code] = result
	return result
}

func validInternationalDigits(number string) bool {
	// Country calling codes have one to three digits. Plans validate the complete
	// national number against both general and actual subscriber-type patterns.
	for n := 1; n <= 3 && n < len(number); n++ {
		plans := plansForCallingCode(number[:n])
		national := number[n:]
		if len(plans) == 1 {
			return plans[0].valid(national)
		}
		for _, plan := range plans {
			if plan.leading != nil {
				if plan.leading.MatchString(national) {
					return plan.valid(national)
				}
			} else if plan.valid(national) {
				return true
			}
		}
	}

	return false
}

// Explicit country code, complete national number, optional extension. Domestic
// region inference, vanity numbers and international trunk-prefix rewriting are
// intentionally outside this adapter. Existing Icelandic recognition remains.
var internationalPhoneRE = regexp.MustCompile(`(?i)(\+[0-9](?:[() \t.\-\x{00A0}\x{202F}\x{2011}\x{2013}]*[0-9]){5,19})(?:[ \t]*(?:ext\.?|extension|x)[ \t]*[0-9]{1,20})?`)

func internationalPhones(input string, existing []Match) []Match {
	if !strings.Contains(input, "+") {
		return nil
	}
	var result []Match
	for _, loc := range internationalPhoneRE.FindAllStringSubmatchIndex(input, -1) {
		start, end := loc[0], loc[1]
		if !bounded(input, start, end) || adjacentDigitAfterSeparator(input, start, end) {
			continue
		}
		raw := input[loc[2]:loc[3]]
		// A parenthesized country prefix is part of the number; outer punctuation
		// around the entire number is not. Only borrow an immediately adjacent
		// opening parenthesis when the matched country-prefix close needs it.
		if start > 0 && input[start-1] == '(' && strings.Count(raw, ")") == strings.Count(raw, "(")+1 {
			start--
			raw = input[start:loc[3]]
		}
		if strings.Count(raw, "(") != strings.Count(raw, ")") {
			continue
		}
		if !bounded(input, start, end) || !validInternationalDigits(digits(raw)) {
			continue
		}
		protected := false
		for _, old := range existing {
			if old.Kind != Phone && start < old.End && old.Start < end {
				protected = true
				break
			}
		}
		if !protected {
			result = append(result, Match{start, end, Phone, High, "international-phone-numbering-plan"})
		}
	}
	return result
}
