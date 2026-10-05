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

	"github.com/AndriGitDev/dmz/packages/detect"
	"github.com/AndriGitDev/dmz/packages/surrogates"
)

type entity struct {
	Kind  detect.Kind `json:"kind"`
	Value string      `json:"value"`
}
type sample struct {
	ID       string   `json:"id"`
	Stratum  string   `json:"stratum"`
	Text     string   `json:"text"`
	Entities []entity `json:"entities"`
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
		if !strings.HasPrefix(path, base+string(filepath.Separator)) || !strings.HasSuffix(path, ".jsonl") {
			return fmt.Errorf("synthetic corpus required")
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
