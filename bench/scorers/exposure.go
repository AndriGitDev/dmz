package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/AndriGitDev/dmz/packages/surrogates"
)

type exposureCount struct {
	Masked  int `json:"masked"`
	Blocked int `json:"blocked"`
	Exposed int `json:"exposed"`
}

type exposureReport struct {
	Corpus         string                   `json:"corpus"`
	Mode           string                   `json:"mode"`
	Samples        int                      `json:"samples"`
	AllowedPrompts int                      `json:"allowed_prompts"`
	BlockedPrompts int                      `json:"blocked_prompts"`
	TenantTerms    []string                 `json:"tenant_terms"`
	Method         string                   `json:"method"`
	PerKind        map[string]exposureCount `json:"per_kind"`
}

// evaluateExposure counts direct-identifier literals in the text that the
// prototype would allow out. No model is called, and no prompt is printed.
func evaluateExposure(path string) (exposureReport, error) {
	f, err := os.Open(path)
	if err != nil {
		return exposureReport{}, err
	}
	defer f.Close()
	terms := []string{"Verkefni Gervi"}
	r := exposureReport{Corpus: path, Mode: "outbound-exposure", TenantTerms: terms,
		Method:  "per annotated direct-identifier literal: masked, blocked with prompt, or still present in allowed text",
		PerKind: map[string]exposureCount{}}
	seen := map[string]bool{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		r.Samples++
		var item sample
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			return exposureReport{}, fmt.Errorf("line %d: %w", r.Samples, err)
		}
		if item.ID == "" || item.Text == "" || seen[item.ID] {
			return exposureReport{}, fmt.Errorf("line %d: missing text/id or duplicate id", r.Samples)
		}
		seen[item.ID] = true
		session := surrogates.NewSession()
		if err := session.SetTenantTerms(terms); err != nil {
			return exposureReport{}, err
		}
		out := session.Process(item.Text)
		if out.Allowed {
			r.AllowedPrompts++
		} else {
			r.BlockedPrompts++
		}
		for _, e := range item.Entities {
			if e.Kind == "" || e.Value == "" || strings.Count(item.Text, e.Value) != 1 {
				return exposureReport{}, fmt.Errorf("line %d: invalid annotation", r.Samples)
			}
			count := r.PerKind[string(e.Kind)]
			switch {
			case !out.Allowed:
				count.Blocked++
			case strings.Contains(out.Transformed, e.Value):
				count.Exposed++
			default:
				count.Masked++
			}
			r.PerKind[string(e.Kind)] = count
		}
	}
	if err := scanner.Err(); err != nil {
		return exposureReport{}, err
	}
	if r.Samples == 0 {
		return exposureReport{}, fmt.Errorf("empty corpus")
	}
	return r, nil
}
