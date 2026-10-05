// core-probe captures exact original-byte spans and restoration for synthetic
// corpora. It emits no input text or surrogate maps; callers pin source assets.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/AndriGitDev/dmz/packages/detect"
	"github.com/AndriGitDev/dmz/packages/surrogates"
)

type entity struct {
	Kind  detect.Kind `json:"kind"`
	Value string      `json:"value"`
}
type sample struct {
	ID       string               `json:"id"`
	Stratum  string               `json:"stratum"`
	Text     string               `json:"text"`
	Entities []entity             `json:"entities"`
	Gold     [][3]json.RawMessage `json:"gold"`
	TextSHA  string               `json:"text_sha256"`
}
type record struct {
	ID               string   `json:"id"`
	Stratum          string   `json:"stratum"`
	TextSHA          string   `json:"text_sha256"`
	Gold             [][3]any `json:"gold"`
	Predicted        [][3]any `json:"predicted"`
	Abstained        bool     `json:"abstained"`
	ExactRestoration bool     `json:"exact_restoration"`
}

func run() error {
	enc := json.NewEncoder(os.Stdout)
	base, err := filepath.Abs("bench/corpus")
	if err != nil {
		return err
	}
	for _, arg := range os.Args[1:] {
		path, err := filepath.EvalSymlinks(arg)
		if err != nil {
			return err
		}
		path, err = filepath.Abs(path)
		if err != nil {
			return err
		}
		publicProjection := !strings.HasPrefix(path, base+string(filepath.Separator))
		if !strings.HasSuffix(path, ".jsonl") {
			return fmt.Errorf("synthetic JSONL corpus required")
		}
		if publicProjection {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			h := sha256.Sum256(data)
			if hex.EncodeToString(h[:]) != "6b70350932be463af6b53fc79bee5e53be2e66cc4133ae9bd0ff672d2237ad29" {
				return fmt.Errorf("only the pinned public synthetic English projection is permitted outside bench/corpus")
			}
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 4096), 1024*1024)
		for scanner.Scan() {
			var row sample
			if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
				f.Close()
				return fmt.Errorf("invalid corpus row")
			}
			h := sha256.Sum256([]byte(row.Text))
			r := record{ID: row.ID, Stratum: row.Stratum, TextSHA: hex.EncodeToString(h[:]), Gold: make([][3]any, 0), Predicted: make([][3]any, 0)}
			if publicProjection {
				if row.TextSHA != r.TextSHA || row.Gold == nil || !utf8.ValidString(row.Text) {
					return fmt.Errorf("invalid public projection row")
				}
				previous := 0
				for _, gold := range row.Gold {
					var start, end int
					var kind detect.Kind
					if json.Unmarshal(gold[0], &start) != nil || json.Unmarshal(gold[1], &end) != nil || json.Unmarshal(gold[2], &kind) != nil || start < previous || end <= start || end > len(row.Text) || !utf8.ValidString(row.Text[start:end]) || !utf8.ValidString(row.Text[:start]) {
						return fmt.Errorf("invalid projected gold span")
					}
					switch kind {
					case detect.Name, detect.Address, detect.Phone, detect.Email, detect.Card, detect.Secret:
					default:
						return fmt.Errorf("invalid projected entity scope")
					}
					r.Gold = append(r.Gold, [3]any{start, end, kind})
					previous = end
				}
			}
			for _, e := range row.Entities {
				if e.Value == "" || strings.Count(row.Text, e.Value) != 1 {
					f.Close()
					return fmt.Errorf("ambiguous synthetic annotation")
				}
				start := strings.Index(row.Text, e.Value)
				r.Gold = append(r.Gold, [3]any{start, start + len(e.Value), e.Kind})
			}
			s := surrogates.NewSession()
			if err := s.SetTenantTerms([]string{"Verkefni Gervi"}); err != nil {
				return err
			}
			result := s.ProcessBestEffort(row.Text, nil)
			r.ExactRestoration = result.Allowed && s.Restore(result.Transformed) == row.Text
			if !r.ExactRestoration {
				f.Close()
				return fmt.Errorf("restoration failure")
			}
			r.Abstained = !result.SpansAvailable
			for _, m := range result.DetectedSpans {
				r.Predicted = append(r.Predicted, [3]any{m.Start, m.End, m.Kind})
			}
			if err := enc.Encode(r); err != nil {
				f.Close()
				return err
			}
		}
		err = scanner.Err()
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: core-probe bench/corpus/*.jsonl")
		os.Exit(2)
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
