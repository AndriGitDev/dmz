package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AndriGitDev/dmz/packages/detect"
)

func TestCompleteSpanClassification(t *testing.T) {
	text := "Gervislóð 97‑99"
	for _, tc := range []struct {
		negative bool
		matches  []detect.Match
		want     string
	}{
		{false, []detect.Match{{Start: 0, End: len(text), Kind: detect.Address}}, "passed"},
		{false, []detect.Match{{Start: 0, End: len("Gervislóð 97"), Kind: detect.Address}}, "partial"},
		{false, []detect.Match{{Start: 0, End: len(text), Kind: detect.Name}}, "missed"},
		{true, nil, "passed"},
		{true, []detect.Match{{Start: 0, End: len(text), Kind: detect.Address}}, "false_address"},
	} {
		if got := classify(text, tc.negative, tc.matches); got != tc.want {
			t.Fatalf("got %s want %s", got, tc.want)
		}
	}
}

func TestExternalPathsAndOutputPreservation(t *testing.T) {
	if _, err := external("main.go", true); err == nil {
		t.Fatal("accepted checkout source")
	}
	if _, err := external("new-report.json", false); err == nil {
		t.Fatal("accepted checkout output")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "pack.json")
	output := filepath.Join(dir, "report.json")
	if err := os.WriteFile(source, []byte(`{"schema":1,"addresses":["Gervislóð"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	sentinel := []byte("preserve existing report")
	if err := os.WriteFile(output, sentinel, 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(source, output, "", 0); err == nil {
		t.Fatal("overwrote existing output")
	}
	got, err := os.ReadFile(output)
	if err != nil || string(got) != string(sentinel) {
		t.Fatal("existing output changed")
	}
	link := filepath.Join(dir, "checkout")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(cwd, link); err != nil {
		t.Fatal(err)
	}
	if _, err = external(filepath.Join(link, "report.json"), false); err == nil {
		t.Fatal("accepted symlink into checkout")
	}
}
