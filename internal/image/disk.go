// MacXVM - Disk Preparation (MIT)
// https://github.com/shadow-x78/macxvm

package image

import (
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
)

const (
	sectorSize    = 512
	gptEntries    = 128
	gptEntrySize  = 128
	gptHeaderLen  = 92
	gptEntriesLBA = 2
)

var gptBasicData = [16]byte{
	0xA2, 0xA0, 0xD0, 0xEB, 0xE5, 0xB9, 0x33, 0x44,
	0x87, 0xC0, 0x68, 0xB6, 0xB7, 0x26, 0x99, 0xC7,
}

// PrepareDisk writes a GUID partition table on the disk of a machine, so that
// the firmware and macOS see the disk before anything is installed on it. A
// disk of QEMU starts as a file of zeros, and a system that looks for a
// partition table finds nothing to show.
func PrepareDisk(diskPath string, runner hostcheck.Runner) error {
	if runner == nil {
		runner = hostcheck.ExecRunner{}
	}
	if !hostcheck.HaveBinary("qemu-img") {
		return exitcode.New(exitcode.UnsupportedHost, "qemu-img is not in PATH (run: macxvm doctor)")
	}
	raw := filepath.Join(filepath.Dir(diskPath), "gpt.raw")
	if err := runner.Run("qemu-img", "convert", "-O", "raw", "-S", "4k", diskPath, raw); err != nil {
		return exitcode.New(exitcode.Generic, "cannot read the disk: %v", err)
	}
	if err := writeGPT(raw); err != nil {
		os.Remove(raw)
		return err
	}
	if err := runner.Run("qemu-img", "convert", "-O", "qcow2", raw, diskPath); err != nil {
		os.Remove(raw)
		return exitcode.New(exitcode.Generic, "cannot write the disk: %v", err)
	}
	if err := os.Remove(raw); err != nil {
		return exitcode.New(exitcode.Generic, "cannot remove '%s': %v", raw, err)
	}
	return nil
}

func writeGPT(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot read '%s': %v", path, err)
	}
	sectors := uint64(st.Size()) / sectorSize
	if sectors < 68 {
		return exitcode.New(exitcode.Generic, "the disk is too small for a partition table")
	}

	entries := make([]byte, gptEntries*gptEntrySize)
	copy(entries[0:16], gptBasicData[:])
	partitionGUID, err := randomGUID()
	if err != nil {
		return err
	}
	copy(entries[16:32], partitionGUID[:])
	binary.LittleEndian.PutUint64(entries[32:40], 34)
	binary.LittleEndian.PutUint64(entries[40:48], sectors-34)
	writeUTF16(entries[56:128], "macOS")

	entriesCRC := crc32.ChecksumIEEE(entries)
	diskGUID, err := randomGUID()
	if err != nil {
		return err
	}

	mbr := make([]byte, sectorSize)
	mbr[0x1BE] = 0x00
	mbr[0x1BF] = 0xFE
	mbr[0x1C0] = 0xFF
	mbr[0x1C1] = 0xFF
	mbr[0x1C2] = 0xEE
	mbr[0x1C3] = 0xFE
	mbr[0x1C4] = 0xFF
	mbr[0x1C5] = 0xFF
	binary.LittleEndian.PutUint32(mbr[0x1BE+8:], 1)
	size := sectors - 1
	if size > 0xFFFFFFFF {
		size = 0xFFFFFFFF
	}
	binary.LittleEndian.PutUint32(mbr[0x1BE+12:], uint32(size))
	mbr[0x1FE] = 0x55
	mbr[0x1FF] = 0xAA

	backupEntriesLBA := sectors - 33
	lastUsable := sectors - 34
	primary := gptHeader(1, sectors-1, gptEntriesLBA, lastUsable, diskGUID, entriesCRC)
	backup := gptHeader(sectors-1, 1, backupEntriesLBA, lastUsable, diskGUID, entriesCRC)

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot open '%s': %v", path, err)
	}
	defer f.Close()
	for _, write := range []struct {
		offset int64
		data   []byte
	}{
		{0, mbr},
		{sectorSize, primary},
		{gptEntriesLBA * sectorSize, entries},
		{int64(backupEntriesLBA) * sectorSize, entries},
		{int64(sectors-1) * sectorSize, backup},
	} {
		if _, err := f.WriteAt(write.data, write.offset); err != nil {
			return exitcode.New(exitcode.Generic, "cannot write the partition table: %v", err)
		}
	}
	return nil
}

func gptHeader(current, backup, entriesLBA, lastUsable uint64, diskGUID [16]byte, entriesCRC uint32) []byte {
	h := make([]byte, gptHeaderLen)
	copy(h[0:8], "EFI PART")
	binary.LittleEndian.PutUint32(h[8:], 0x00010000)
	binary.LittleEndian.PutUint32(h[12:], gptHeaderLen)
	binary.LittleEndian.PutUint64(h[24:], current)
	binary.LittleEndian.PutUint64(h[32:], backup)
	binary.LittleEndian.PutUint64(h[40:], 34)
	binary.LittleEndian.PutUint64(h[48:], lastUsable)
	copy(h[56:72], diskGUID[:])
	binary.LittleEndian.PutUint64(h[72:], entriesLBA)
	binary.LittleEndian.PutUint32(h[80:], gptEntries)
	binary.LittleEndian.PutUint32(h[84:], gptEntrySize)
	binary.LittleEndian.PutUint32(h[88:], entriesCRC)
	binary.LittleEndian.PutUint32(h[16:], crc32.ChecksumIEEE(h))
	return h
}

func randomGUID() ([16]byte, error) {
	var guid [16]byte
	f, err := os.Open("/dev/urandom")
	if err != nil {
		return guid, exitcode.New(exitcode.Generic, "cannot read the random source: %v", err)
	}
	defer f.Close()
	if _, err := f.Read(guid[:]); err != nil {
		return guid, exitcode.New(exitcode.Generic, "cannot read the random source: %v", err)
	}
	guid[6] = (guid[6] & 0x0F) | 0x40
	guid[8] = (guid[8] & 0x3F) | 0x80
	return guid, nil
}

func writeUTF16(dst []byte, text string) {
	for i, r := range text {
		if i*2+2 > len(dst) {
			return
		}
		binary.LittleEndian.PutUint16(dst[i*2:], uint16(r))
	}
}
