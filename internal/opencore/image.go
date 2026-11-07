// MacXVM - OpenCore Image Builder (MIT)
// https://github.com/shadow-x78/macxvm

package opencore

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	ConfigPathInImage = "/EFI/OC/config.plist"
	SectorSize        = 512
)

var efiSystemPartitionGUID = []byte{
	0xc1, 0x2a, 0x73, 0x28, 0xf8, 0x1f, 0x11, 0xd2,
	0xba, 0x4b, 0x00, 0xa0, 0xc9, 0x3e, 0xc9, 0x3b,
}

type ImageBuilder struct {
	AssetsDir string
	Runner    CommandRunner
}

type CommandRunner interface {
	Run(name string, args ...string) error
	Output(name string, args ...string) (string, error)
}

type execRunner struct{}

func (execRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (execRunner) Output(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return string(out), nil
}

func (b ImageBuilder) BuildStageImage(vmDir, stage string, plist []byte) error {
	if b.AssetsDir == "" {
		return fmt.Errorf("the assets directory of the machine is missing")
	}
	asset := filepath.Join(b.AssetsDir, "OpenCore.qcow2")
	if _, err := os.Stat(asset); err != nil {
		return fmt.Errorf("the OpenCore asset %s is not available: %w", asset, err)
	}
	runner := b.Runner
	if runner == nil {
		runner = execRunner{}
	}
	work, err := os.MkdirTemp("", "macxvm-opencore-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	raw := filepath.Join(work, "opencore.raw")
	if err := runner.Run("qemu-img", "convert", "-O", "raw", asset, raw); err != nil {
		return err
	}
	offset, err := fatOffset(raw)
	if err != nil {
		return err
	}
	source := filepath.Join(work, "config.plist")
	if err := os.WriteFile(source, plist, 0o600); err != nil {
		return err
	}
	volume := raw
	if offset > 0 {
		volume = fmt.Sprintf("%s@@%d", raw, offset)
	}
	dir := pathDir(ConfigPathInImage)
	if err := runner.Run("mmd", "-i", volume, "::"+dir); err != nil {
		return err
	}
	if err := runner.Run("mcopy", "-o", "-i", volume, source, "::"+ConfigPathInImage); err != nil {
		return err
	}
	target := ImagePath(vmDir, stage)
	partial := target + ".partial"
	os.Remove(partial)
	if err := runner.Run("qemu-img", "convert", "-O", "qcow2", raw, partial); err != nil {
		return err
	}
	if err := os.Chmod(partial, 0o600); err != nil {
		return err
	}
	if err := os.Rename(partial, target); err != nil {
		return err
	}
	return nil
}

func pathDir(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "/"
	}
	return path[:idx]
}

func fatOffset(path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	header := make([]byte, SectorSize)
	if _, err := file.ReadAt(header, 0); err != nil {
		return 0, fmt.Errorf("cannot read the image %s: %w", path, err)
	}
	if isFATHeader(header) {
		return 0, nil
	}
	if string(header[0:8]) != "EFI PART" {
		return 0, fmt.Errorf("the OpenCore image holds neither an EFI volume nor a GPT disk")
	}
	partitionTable := int64(binary.LittleEndian.Uint64(header[72:80]))
	if partitionTable <= 0 {
		return 0, fmt.Errorf("the GPT of the OpenCore image has no partition table")
	}
	entries := make([]byte, SectorSize)
	if _, err := file.ReadAt(entries, partitionTable*SectorSize); err != nil {
		return 0, fmt.Errorf("cannot read the partition table of the OpenCore image: %w", err)
	}
	for offset := 0; offset+128 <= len(entries); offset += 128 {
		entry := entries[offset : offset+128]
		if !equalGUID(entry[0:16], efiSystemPartitionGUID) {
			continue
		}
		first := binary.LittleEndian.Uint64(entry[32:40])
		if first == 0 {
			continue
		}
		return int64(first) * SectorSize, nil
	}
	return 0, fmt.Errorf("the OpenCore image holds no EFI system partition")
}

func isFATHeader(header []byte) bool {
	if len(header) < 64 {
		return false
	}
	switch string(header[3:11]) {
	case "FAT12   ", "FAT16   ", "FAT32   ":
		return true
	}
	return false
}

func equalGUID(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
