// MacXVM - VFIO Sysfs Access (MIT)
// https://github.com/shadow-x78/macxvm

package vfio

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const DefaultSysfs = "/sys"

type FS struct {
	root string
}

func NewFS(root string) *FS {
	if root == "" {
		root = DefaultSysfs
	}
	return &FS{root: root}
}

func (f *FS) Root() string { return f.root }

func (f *FS) PCIDir() string {
	return filepath.Join(f.root, "bus", "pci", "devices")
}

func (f *FS) PCIDriverDir() string {
	return filepath.Join(f.root, "bus", "pci", "drivers")
}

func (f *FS) IOMMUGroupDir() string {
	return filepath.Join(f.root, "kernel", "iommu_groups")
}

func (f *FS) DeviceDir(address string) string {
	return filepath.Join(f.PCIDir(), address)
}

type BAR struct {
	Index int    `json:"index"`
	Start uint64 `json:"start"`
	End   uint64 `json:"end"`
	Size  uint64 `json:"size"`
	Flags uint64 `json:"flags"`
	Type  string `json:"type"`
}

type Device struct {
	Address         string `json:"address"`
	Vendor          string `json:"vendor"`
	Device          string `json:"device"`
	Class           string `json:"class"`
	SubsystemVendor string `json:"subsystemVendor,omitempty"`
	SubsystemDevice string `json:"subsystemDevice,omitempty"`
	Driver          string `json:"driver,omitempty"`
	BootVGA         bool   `json:"bootVga"`
	ResetMethod     string `json:"resetMethod,omitempty"`
	DriverOverride  string `json:"driverOverride,omitempty"`
	IOMMUGroups     []int  `json:"iommuGroups,omitempty"`
	BARs            []BAR  `json:"bars,omitempty"`
}

func (d Device) IsGPU() bool {
	return strings.HasPrefix(d.Class, "0x03")
}

func (d Device) IsAudio() bool {
	return strings.HasPrefix(d.Class, "0x04")
}

func (d Device) IsUSB() bool {
	return strings.HasPrefix(d.Class, "0x0c")
}

func (d Device) VendorID() string { return normalizeID(d.Vendor) }

func (d Device) DeviceID() string { return normalizeID(d.Device) }

func normalizeID(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	value = strings.TrimPrefix(value, "0x")
	if len(value) == 4 {
		return value
	}
	return value
}

func (f *FS) Devices() ([]Device, error) {
	entries, err := os.ReadDir(f.PCIDir())
	if err != nil {
		return nil, fmt.Errorf("cannot read the PCI devices of the host: %w", err)
	}
	var out []Device
	for _, entry := range entries {
		address := entry.Name()
		if !validAddress(address) {
			continue
		}
		dev, err := f.Device(address)
		if err != nil {
			continue
		}
		out = append(out, *dev)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Address < out[j].Address })
	return out, nil
}

func (f *FS) Device(address string) (*Device, error) {
	if !validAddress(address) {
		return nil, fmt.Errorf("%q is not a PCI address of the form 0000:01:00.0", address)
	}
	dir := f.DeviceDir(address)
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("the host has no PCI device at %s", address)
	}
	dev := &Device{Address: address}
	dev.Vendor = readAttr(filepath.Join(dir, "vendor"))
	dev.Device = readAttr(filepath.Join(dir, "device"))
	dev.Class = readAttr(filepath.Join(dir, "class"))
	dev.SubsystemVendor = readAttr(filepath.Join(dir, "subsystem_vendor"))
	dev.SubsystemDevice = readAttr(filepath.Join(dir, "subsystem_device"))
	dev.ResetMethod = readAttr(filepath.Join(dir, "reset_method"))
	dev.DriverOverride = readAttr(filepath.Join(dir, "driver_override"))
	dev.BootVGA = readAttr(filepath.Join(dir, "boot_vga")) == "1"
	dev.Driver = driverOf(filepath.Join(dir, "driver"))
	groups, err := f.GroupsOf(address)
	if err != nil {
		return nil, err
	}
	dev.IOMMUGroups = groups
	dev.BARs = readBARs(filepath.Join(dir, "resource"))
	return dev, nil
}

func (f *FS) GroupsOf(address string) ([]int, error) {
	entries, err := os.ReadDir(f.IOMMUGroupDir())
	if err != nil {
		return nil, nil
	}
	var out []int
	for _, entry := range entries {
		group, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		members, err := os.ReadDir(filepath.Join(f.IOMMUGroupDir(), entry.Name(), "devices"))
		if err != nil {
			continue
		}
		for _, member := range members {
			if member.Name() == address {
				out = append(out, group)
				break
			}
		}
	}
	sort.Ints(out)
	return out, nil
}

func (f *FS) GroupMembers(group int) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(f.IOMMUGroupDir(), strconv.Itoa(group), "devices"))
	if err != nil {
		return nil, fmt.Errorf("the host has no IOMMU group %d", group)
	}
	var out []string
	for _, entry := range entries {
		if validAddress(entry.Name()) {
			out = append(out, entry.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func (f *FS) Groups() (map[int][]string, error) {
	entries, err := os.ReadDir(f.IOMMUGroupDir())
	if err != nil {
		return nil, nil
	}
	out := map[int][]string{}
	for _, entry := range entries {
		group, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		members, err := f.GroupMembers(group)
		if err != nil {
			continue
		}
		out[group] = members
	}
	return out, nil
}

func driverOf(link string) string {
	target, err := os.Readlink(link)
	if err != nil {
		return ""
	}
	return filepath.Base(target)
}

func readAttr(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func readBARs(path string) []BAR {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	const barSize = 16
	var out []BAR
	for index := 0; index < 6; index++ {
		start := index * barSize
		if start+barSize > len(raw) {
			break
		}
		low := binary.LittleEndian.Uint64(raw[start : start+8])
		high := binary.LittleEndian.Uint64(raw[start+8 : start+16])
		if low == 0 && high == 0 {
			continue
		}
		flags := high & (^uint64(0) >> 32)
		startAddr := low & 0xfffffffffffffff0
		endAddr := high & 0xfffffffffffffff0
		size := uint64(0)
		if endAddr > startAddr {
			size = endAddr - startAddr + 1
		}
		out = append(out, BAR{
			Index: index,
			Start: startAddr,
			End:   endAddr,
			Size:  size,
			Flags: flags,
			Type:  barType(flags),
		})
	}
	return out
}

func barType(flags uint64) string {
	switch flags & 0xf {
	case 0:
		return "32bit-memory"
	case 1:
		return "below-1m-memory"
	case 2:
		return "64bit-memory"
	case 4:
		return "io"
	default:
		return "reserved"
	}
}

func validAddress(address string) bool {
	parts := strings.Split(address, ":")
	if len(parts) != 3 {
		return false
	}
	if len(parts[0]) != 4 || len(parts[1]) != 2 || len(parts[2]) != 4 {
		return false
	}
	if parts[2][2] != '.' {
		return false
	}
	function := parts[2][3]
	if function < '0' || function > '7' {
		return false
	}
	for _, part := range []string{parts[0], parts[1], parts[2][:2]} {
		for _, c := range part {
			if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
				return false
			}
		}
	}
	return true
}

func ValidAddress(address string) bool { return validAddress(address) }
