package main

import (
	"os"
	"path/filepath"
	"testing"
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
