package main

import (
	"bytes"
	"debug/macho"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func thinFixture(t *testing.T, dir, name string, cpu macho.Cpu) string {
	t.Helper()
	var body bytes.Buffer
	for _, value := range []uint32{macho.Magic64, uint32(cpu), 0, uint32(macho.TypeExec), 0, 0, 0, 0} {
		binary.Write(&body, binary.LittleEndian, value)
	}
	body.WriteString("synthetic-independent-slice-" + name)
	path := filepath.Join(dir, name)
	if os.WriteFile(path, body.Bytes(), 0755) != nil {
		t.Fatal("fixture unavailable")
	}
	return path
}
func TestUniversalRoundTripAndNoOverwrite(t *testing.T) {
	dir := t.TempDir()
	intel := thinFixture(t, dir, "intel", macho.CpuAmd64)
	arm := thinFixture(t, dir, "arm", macho.CpuArm64)
	out := filepath.Join(dir, "universal")
	if err := buildUniversal(out, []string{intel, arm}); err != nil {
		t.Fatal(err)
	}
	fat, err := macho.OpenFat(out)
	if err != nil {
		t.Fatal(err)
	}
	defer fat.Close()
	if len(fat.Arches) != 2 || fat.Arches[0].Cpu != macho.CpuAmd64 || fat.Arches[1].Cpu != macho.CpuArm64 {
		t.Fatal("wrong architecture slices")
	}
	before, _ := os.ReadFile(out)
	if buildUniversal(out, []string{intel, arm}) == nil {
		t.Fatal("existing output overwritten")
	}
	after, _ := os.ReadFile(out)
	if !bytes.Equal(before, after) {
		t.Fatal("existing output changed")
	}
	if buildUniversal(filepath.Join(dir, "duplicate"), []string{intel, intel}) == nil {
		t.Fatal("duplicate architecture accepted")
	}
	os.WriteFile(arm, []byte("invalid"), 0600)
	if buildUniversal(filepath.Join(dir, "invalid"), []string{intel, arm}) == nil {
		t.Fatal("invalid Mach-O accepted")
	}
}
