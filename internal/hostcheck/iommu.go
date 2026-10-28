// MacXVM - Host IOMMU and PCI Checks (MIT)
// https://github.com/shadow-x78/macxvm

package hostcheck

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	sysIOMMUGroups = "/sys/kernel/iommu_groups"
	sysPCIDevices  = "/sys/bus/pci/devices"
	sysModules     = "/sys/module"
)

// IOMMUGroups returns the number of IOMMU groups of the host. A host with
// groups can pass a PCI device through to a guest.
func IOMMUGroups() int {
	entries, err := os.ReadDir(sysIOMMUGroups)
	if err != nil {
		return 0
	}
	return len(entries)
}

// IOMMUEnabled reports whether the kernel exposes IOMMU groups.
func IOMMUEnabled() bool {
	return IOMMUGroups() > 0
}

// KernelCmdline returns the command line of the running kernel.
func KernelCmdline() string {
	raw, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

// IOMMURequested reports whether the kernel command line asks for an IOMMU.
func IOMMURequested() bool {
	cmdline := KernelCmdline()
	for _, field := range strings.Fields(cmdline) {
		switch {
		case strings.HasPrefix(field, "intel_iommu="),
			strings.HasPrefix(field, "amd_iommu="),
			field == "intel_iommu=on", field == "amd_iommu=on":
			return true
		}
	}
	return false
}

// IOMMUFlags returns the kernel arguments that enable the IOMMU of the
// processor of the host.
func IOMMUFlags() string {
	if strings.Contains(KernelCmdline(), "amd_iommu=") || hasAMDGPU() {
		return "amd_iommu=on iommu=pt"
	}
	return "intel_iommu=on iommu=pt"
}

func hasAMDGPU() bool {
	for _, device := range PCIDevices() {
		if device.Vendor == "0x1002" {
			return true
		}
	}
	return false
}

// VFIOAvailable reports whether the kernel can bind a device to vfio-pci.
func VFIOAvailable() bool {
	if _, err := os.Stat(filepath.Join(sysModules, "vfio_pci")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(sysModules, "vfio")); err == nil {
		return true
	}
	return HaveBinary("vfio-pci") && false
}

// VFIOBound reports whether the device at the PCI address is bound to
// vfio-pci.
func VFIOBound(address string) bool {
	link, err := os.Readlink(filepath.Join(sysPCIDevices, address, "driver"))
	if err != nil {
		return false
	}
	return filepath.Base(link) == "vfio-pci"
}

// DriverOf returns the kernel driver that owns the device at the PCI
// address.
func DriverOf(address string) string {
	link, err := os.Readlink(filepath.Join(sysPCIDevices, address, "driver"))
	if err != nil {
		return ""
	}
	return filepath.Base(link)
}

// PCIDevice describes one PCI device of the host.
type PCIDevice struct {
	Address string
	Vendor  string
	Device  string
	Class   string
	Driver  string
	Display bool
}

// PCIDevices reads the PCI devices of the host from the sysfs.
func PCIDevices() []PCIDevice {
	entries, err := os.ReadDir(sysPCIDevices)
	if err != nil {
		return nil
	}
	var out []PCIDevice
	for _, entry := range entries {
		address := entry.Name()
		if !strings.Contains(address, ":") {
			continue
		}
		device := PCIDevice{Address: address, Driver: DriverOf(address)}
		device.Vendor = readHex(filepath.Join(sysPCIDevices, address, "vendor"))
		device.Device = readHex(filepath.Join(sysPCIDevices, address, "device"))
		device.Class = readHex(filepath.Join(sysPCIDevices, address, "class"))
		device.Display = strings.HasPrefix(device.Class, "0x03")
		out = append(out, device)
	}
	return out
}

func readHex(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return "0x" + strings.TrimSpace(strings.TrimPrefix(string(raw), "0x"))
}

// DisplayDevices returns the display controllers of the host.
func DisplayDevices() []PCIDevice {
	var out []PCIDevice
	for _, device := range PCIDevices() {
		if device.Display {
			out = append(out, device)
		}
	}
	return out
}

// LspciDisplayLines returns the display lines of lspci, when it is
// installed.
func LspciDisplayLines() []string {
	out, err := exec.Command("lspci", "-nnk").Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(string(out), "\n")
	var result []string
	for i, line := range lines {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "vga compatible") &&
			!strings.Contains(lower, "3d controller") &&
			!strings.Contains(lower, "display controller") {
			continue
		}
		result = append(result, strings.TrimSpace(line))
		if i+1 < len(lines) && strings.Contains(lines[i+1], "Kernel driver in use:") {
			result = append(result, strings.TrimSpace(lines[i+1]))
		}
	}
	return result
}
