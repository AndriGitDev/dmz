package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AndriGitDev/dmz/packages/detect"
)

func TestRecallFloorRejectsLoweredOrMissingKind(t *testing.T) {
	result := report{PerKind: map[string]score{
		"email": {Recall: 0.8},
	}}
	path := filepath.Join(t.TempDir(), "floor.json")
	for _, tc := range []struct {
		name, json string
		fail       bool
	}{
		{"passes", `{"email":0.8}`, false},
		{"regression", `{"email":0.9}`, true},
		{"missing kind", `{"name_demo":0.8}`, true},
		{"invalid floor", `{"email":1.1}`, true},
		{"empty floor", `{}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.json), 0600); err != nil {
				t.Fatal(err)
			}
			err := enforceRecallFloor(result, path)
			if (err != nil) != tc.fail {
				t.Fatalf("got error %v; want failure %t", err, tc.fail)
			}
		})
	}
}

func TestExposureCapRejectsRegression(t *testing.T) {
	result := exposureReport{PerKind: map[string]exposureCount{
		"email":     {Exposed: 1},
		"name_demo": {Exposed: 1},
	}}
	if err := enforceExposureCap(result, 2); err != nil {
		t.Fatal(err)
	}
	if err := enforceExposureCap(result, 1); err == nil {
		t.Fatal("increased outbound exposure passed the cap")
	}
}

func TestExposureSeparatesBenignReviewAndExposedIdentifiers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corpus.jsonl")
	data := `{"id":"plain","text":"Hello world.","entities":[]}
{"id":"review","text":"x\u0301 dagur.","entities":[]}
{"id":"altered","text":"demo@example.invalid","entities":[]}
{"id":"miss","text":"Mira Demo","entities":[{"kind":"name_demo","value":"Mira Demo"}]}
`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := evaluateExposure(path)
	if err != nil {
		t.Fatal(err)
	}
	if r.BenignPrompts != 3 || len(r.BlockedBenign) != 1 || r.BlockedBenign[0] != "review" || len(r.AlteredBenign) != 1 || r.AlteredBenign[0] != "altered" || len(r.ExposedCases) != 1 || r.ExposedCases[0] != "miss" {
		t.Fatalf("wrong case accounting: %#v", r)
	}
}

func TestFormatCorpusHasNoMissesOrFalseAlarms(t *testing.T) {
	path := "../corpus/format-v2.jsonl"
	r, err := evaluate(path, "dmz", detect.Detect)
	if err != nil {
		t.Fatal(err)
	}
	for kind, score := range r.PerKind {
		if score.FalseNegative != 0 || score.FalsePositive != 0 {
			t.Errorf("format regression for %s: %#v", kind, score)
		}
	}
	exposure, err := evaluateExposure(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := enforceExposureCap(exposure, 0); err != nil {
		t.Fatal(err)
	}
	if len(exposure.BlockedBenign) != 0 || len(exposure.AlteredBenign) != 0 {
		t.Fatalf("benign format examples changed: %#v", exposure)
	}
}
