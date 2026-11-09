// SPDX-License-Identifier: GPL-3.0-only

// This build helper combines the two owned Go Mach-O executables without
// changing either slice. The result is checked with Go's independent Mach-O
// reader. It is not a signing, notarization or native-runtime qualification.
package main

import (
	"bytes"
	"debug/macho"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) != 4 || buildUniversal(os.Args[1], os.Args[2:]) != nil {
		fmt.Fprintln(os.Stderr, "universal_macho_build_failed")
		os.Exit(1)
	}
	fmt.Println("PASS universal Mach-O contains byte-identical Intel and Apple Silicon slices")
}

func buildUniversal(output string, paths []string) error {
	if len(paths) != 2 {
		return errors.New("two_slices_required")
	}
	type image struct {
		header macho.FatArchHeader
		body   []byte
	}
	images := make([]image, 0, 2)
	seen := map[macho.Cpu]bool{}
	offset := uint32(16384)
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 64<<20 {
			return errors.New("invalid_macho_input")
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		parsed, err := macho.NewFile(bytes.NewReader(body))
		if err != nil {
			return err
		}
		if parsed.Magic != macho.Magic64 || parsed.Type != macho.TypeExec || (parsed.Cpu != macho.CpuAmd64 && parsed.Cpu != macho.CpuArm64) || seen[parsed.Cpu] {
			return errors.New("invalid_macho_architecture")
		}
		seen[parsed.Cpu] = true
		images = append(images, image{macho.FatArchHeader{Cpu: parsed.Cpu, SubCpu: parsed.SubCpu, Offset: offset, Size: uint32(len(body)), Align: 14}, body})
		offset = (offset + uint32(len(body)) + 16383) &^ 16383
	}
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0755)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		file.Close()
		if !complete {
			os.Remove(output)
		}
	}()
	if err = binary.Write(file, binary.BigEndian, []uint32{macho.MagicFat, 2}); err != nil {
		return err
	}
	for _, image := range images {
		if err = binary.Write(file, binary.BigEndian, image.header); err != nil {
			return err
		}
	}
	for _, image := range images {
		if n, err := file.WriteAt(image.body, int64(image.header.Offset)); err != nil {
			return err
		} else if n != len(image.body) {
			return io.ErrShortWrite
		}
	}
	if err = file.Close(); err != nil {
		return err
	}
	fat, err := macho.OpenFat(output)
	if err != nil {
		return err
	}
	defer fat.Close()
	if len(fat.Arches) != 2 {
		return errors.New("missing_macho_slice")
	}
	combined, err := os.ReadFile(output)
	if err != nil {
		return err
	}
	for i, arch := range fat.Arches {
		want := images[i]
		if arch.Cpu != want.header.Cpu || arch.SubCpu != want.header.SubCpu || arch.Offset%16384 != 0 || arch.Size != uint32(len(want.body)) || !bytes.Equal(combined[arch.Offset:arch.Offset+arch.Size], want.body) {
			return errors.New("macho_slice_mismatch")
		}
	}
	complete = true
	return nil
}
